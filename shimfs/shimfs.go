package shimfs

import (
	"../minfs"
	"errors"
	"fmt"
	"os"
	"path"
	"time"
)

//Plan: all in-memory data stored in a Map[string path]shimFS.fileInf
//shimFS.fileInf implements os.FileInfo, but also has fields for tempPath-cached data and in-memory-cached data
//Locking done at the shimFS.fileInf-level (to prevent clusterF*s of async stuff)
//Async stuff is taken care of by other goroutines

//Acts as a buffer for reads and writes to another MinFS filesystem
type shimFS struct {
	tempPath  string        //path to use as a temporary buffer
	tempSize  int           //max total space to use for buffer
	mfs       minfs.MinFS   //Filesystem being shimmed
	timeout   time.Duration //timeout if asynced
	filecache map[string]*fileInfo
}

func New(tempPath string, tempSize int, mfs minfs.MinFS) (minfs.MinFS, error) {
	//Check tempPath is usable
	return &shimFS{tempPath, tempSize, mfs, 5 * time.Second, make(map[string]*fileInfo)}, nil
}

type swath struct {
	off   int64
	array []byte
}

//Stuff to comply with minfs.MinFS
func (f *shimFS) ReadFile(name string, b []byte, off int64) (int, error) {
	file, err := f.interStat(name)
	if err != nil {
		return 0, err
	}

	unfulfilled := make(chan swath, 100)
	unfulfilled <- swath{off, b[0:len(b)]}
	var chunks []fileChunk

	if file.chunks != nil {
		chunks = *file.chunks
	} else {
		chunks = make([]fileChunk, 0)
	}

	//iterate through the chunks, fill in where we can
	for _, chunk := range chunks {
		chend := chunk.offset + chunk.size
		nunf := make(chan swath, 100)

		//compare against everything we have yet to fullfil
		for 0 < len(unfulfilled) {
			//pop out the next swath and work it
			sw := <-unfulfilled
			swend := sw.off + int64(len(sw.array))

			//What can happen: ABCDE

			//A: No overlap
			//                      |sw.off-swend|
			//  |chunk.offset-chend|              |chunk.offset-chend|
			//
			//B: Complete overlap
			//     |sw.off-swend|
			//  |chunk.offset-chend|
			//
			//C: Overlap the front
			//          |sw.off-swend|
			//  |chunk.offset-chend|
			//
			//D: Overlap the end
			//  |sw.off-swend|
			//     |chunk.offset---chend|
			//
			//E: Only inside
			//  |sw.off-----------swend|
			//    |chunk.offset-chend|

			//A: covers nothing, save need for next chunk and continue
			if sw.off > chend || swend <= chunk.offset {
				//fmt.Println("A", sw.off, chend, swend, chunk.offset)
				nunf <- sw
				continue
			}
			//B: covers everything, fullfil and continue
			if chunk.offset <= sw.off && swend <= chend {
				//fmt.Println("B", chunk.offset, sw.off, swend, chend)
				copy(sw.array, chunk.memback[sw.off-chunk.offset:swend-chunk.offset])
				continue
			}
			//C: covers the beginning, fill that and save what's needed
			if chunk.offset <= sw.off {
				//fmt.Println("C", chunk.offset, sw.off)
				copy(sw.array[:chend-sw.off], chunk.memback[sw.off-chunk.offset:])
				nunf <- swath{chend, sw.array[chend-sw.off:]}
				continue
			}
			//D: covers the end, fill that and save what's needed
			if swend <= chend {
				//fmt.Println("D", swend, chend)
				copy(sw.array[chunk.offset-sw.off:], chunk.memback[:swend-chunk.offset])
				nunf <- swath{sw.off, sw.array[:chunk.offset-sw.off]}
				continue
			}
			//E: covers the middle, fill, and split need in two
			//fmt.Println("E", sw.off, chend, swend, chunk.offset)
			copy(sw.array[chunk.offset-sw.off:chend-sw.off], chunk.memback)
			nunf <- swath{sw.off, sw.array[:chunk.offset-sw.off]}
			nunf <- swath{chend, sw.array[chend-sw.off:]}
		}
		close(unfulfilled)
		unfulfilled = nunf
	}

	//Any swaths left to fullfil gets served from the source

	//totRead := 0 //debugging cruft
	for len(unfulfilled) > 0 {
		job := <-unfulfilled
		if len(job.array) > 0 {
			//fmt.Println("shim passed through a read", file.fullpath, "at", job.off, "for", len(job.array))
			nread, errread := f.mfs.ReadFile(file.fullpath, job.array, job.off)
			if errread != nil {
				err = errread
			}
			if nread > 0 {
				//totRead += nread
				//totRead++
				tA := make([]byte, nread)
				copy(tA, job.array[:nread])

				chunks = append(chunks, fileChunk{offset: job.off, memback: tA, size: int64(len(tA))})
				file.chunks = &chunks
			}
		}
	}
	//return totRead, err

	retCount := int64(len(b))
	if file.Size()-off < retCount {
		retCount = file.Size() - off
	}
	if retCount < 0 {
		retCount = 0
	}
	return int(retCount), err
}

func (f *shimFS) WriteFile(name string, b []byte, off int64) (int, error) {
	//cache written data
	// send data to mfs
	fmt.Println("shim passed through a write")
	return f.mfs.WriteFile(name, b, off)
}

func (f *shimFS) CreateFile(name string) error {
	fmt.Println("shim passed through a createfile")
	return f.mfs.CreateFile(name)
}

func (f *shimFS) CreateDirectory(name string) error {
	fmt.Println("shim passed through a createdir")
	return f.mfs.CreateDirectory(name)
}

func (f *shimFS) Move(oldpath string, newpath string) error {
	fmt.Println("shim passed through a move")
	return f.mfs.Move(oldpath, newpath)
}

func (f *shimFS) Remove(name string, recursive bool) error {
	fmt.Println("shim passed through a remove")
	return f.mfs.Remove(name, recursive)
}

func (f *shimFS) ReadDirectory(dirpath string) ([]os.FileInfo, error) {
	dirpath = path.Clean(dirpath)
	val, ok := f.filecache[dirpath]
	var err error
	if !ok { //we've never done a stat on this path
		val, err = f.interStat(dirpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.ReadDirectory Stat: " + err.Error())
		}
	}

	//check if cache too old
	if time.Now().After(val.dirage.Add(f.timeout)) {
		//fmt.Println("shim passed through a readdir:", dirpath)
		fia, err := f.mfs.ReadDirectory(dirpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.ReadDirectory read of path: " + err.Error())
		}
		val.dirage = time.Now()
		val.diritems = make([]string, len(fia))

		for i, entry := range fia {
			val.diritems[i] = entry.Name()

			npath := path.Clean(dirpath + "/" + entry.Name())
			eval, ok := f.filecache[npath]
			if !ok {
				f.filecache[npath] = &fileInfo{fullpath: npath}
				eval = f.filecache[npath]
			}

			eval.Update(entry)
		}
	}

	retval := make([]os.FileInfo, len(val.diritems))
	for i, name := range val.diritems {
		retval[i] = f.filecache[dirpath+"/"+name]
	}
	return retval, nil
}

func (f *shimFS) GetAttribute(path string, attribute string) (interface{}, error) {

	switch attribute {
	case "modtime":
		fi, err := f.interStat(path)
		if err != nil {
			return nil, errors.New("Error shimFS.GetAtt Stat: " + err.Error())
		}
		return fi.ModTime(), nil
	case "size":
		fi, err := f.interStat(path)
		if err != nil {
			return nil, errors.New("Error shimFS.GetAtt Stat: " + err.Error())
		}
		return fi.Size(), nil
	default:
		fmt.Println("shim passed through a getattr")
		return f.mfs.GetAttribute(path, attribute)
	}
}

func (f *shimFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	fmt.Println("shim passed through a setattr")
	switch attribute {
	case "modtime":
		return f.mfs.SetAttribute(path, attribute, newvalue)
	case "size":
		return f.mfs.SetAttribute(path, attribute, newvalue)
	}
	return errors.New("SetAttribute Error: Unsupported attribute " + attribute)
}

func (f *shimFS) interStat(fpath string) (*fileInfo, error) {
	fpath = path.Clean(fpath)
	val, ok := f.filecache[fpath]

	//if we've never done a stat on this path, make a new entry
	if !ok {
		val = &fileInfo{fullpath: fpath}
	}

	//if entry too old, refresh
	if time.Now().After(val.fiage.Add(f.timeout)) {
		//fmt.Println("shim passed through a stat:", fpath, "fiage:", val.fiage, "now:", time.Now())
		fi, err := f.mfs.Stat(fpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.Stat: " + err.Error())
		}
		val.Update(fi)
	}

	//if we've never done a stat before, add new entry to cache
	if !ok {
		f.filecache[fpath] = val
	}
	return val, nil
}

func (f *shimFS) Stat(fpath string) (os.FileInfo, error) {
	return f.interStat(fpath)
}

func (f *shimFS) String() string { return "shimFS( " + f.mfs.String() + " )" }

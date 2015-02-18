package shimfs

import (
	"../minfs"
	"errors"
	//"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

//Plan: all in-memory data stored in a Map[string path]shimFS.fileInf
//shimFS.fileInf implements os.FileInfo, but also has fields for tempPath-cached data and in-memory-cached data
//Locking done at the shimFS.fileInf-level (to prevent clusterF*s of async stuff)
//Async stuff is taken care of by other goroutines

//Acts as a buffer for reads and writes to another MinFS filesystem
type shimFS struct {
	tempPath  string        //path to use as a temporary buffer
	tempSize      int64         //max total space to use for buffer
	mfs       minfs.MinFS   //Filesystem being shimmed
	timeout   time.Duration //timeout if asynced
	filecache     map[string]*shimFI
	filecacheLock *sync.RWMutex
	giudCounter   chan int
	closed		bool       //is usually false, unless the shimfs has been closed
}

func New(tempPath string, tempSize int64, mfs minfs.MinFS) (minfs.MinFS, error) {
	//Clean out any residuals from previous runs
	qw, _ := os.Open(tempPath)
	qa, _ := qw.Readdirnames(-1)
	for _, qd := range qa {
		if strings.HasSuffix(qd, ".shimfs") {
			os.Remove(path.Clean(tempPath + "/" + qd))
		}
	}
	qw.Close()

	//Check tempPath is usable
	file, err := os.Create(path.Clean(tempPath + "/test.shimfs"))
	if err != nil {
		return nil, errors.New("Couldn't create in cache directory:" + tempPath + " Error:" + err.Error())
	}
	_, err = file.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	if err != nil {
		return nil, errors.New("Couldn't write to cache directory:" + tempPath + " Error:" + err.Error())
	}
	file.Close()
	err = os.Remove(path.Clean(tempPath + "/test.shimfs"))
	if err != nil {
		return nil, errors.New("Couldn't delete in cache directory:" + tempPath + " Error:" + err.Error())
	}

	guidchan := make(chan int, 8)
	go func() {
		count := 0
		for {
			guidchan <- count
			count++
		}
	}()

	return &shimFS{tempPath, tempSize, mfs, 5 * time.Second, make(map[string]*shimFI), new(sync.RWMutex), guidchan, false}, nil
}

func (f *shimFS) addFI(fpath string, fi os.FileInfo) *shimFI {
	eval := newShimFI(fpath, fi, <-f.giudCounter)
	f.filecacheLock.Lock()
	f.filecache[fpath] = eval
	f.filecacheLock.Unlock()
	return eval
}

func (f *shimFS) interStat(fpath string) (*shimFI, error) {
	fpath = path.Clean(fpath)
	f.filecacheLock.RLock()
	val, ok := f.filecache[fpath]
	f.filecacheLock.RUnlock()

	//if we've never done a stat on this path, make a new entry
	if !ok {
		fi, err := f.mfs.Stat(fpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.Stat: " + err.Error())
		}

		val = f.addFI(fpath, fi)
	}

	//if entry too old, refresh
	if time.Now().After(val.fiAge().Add(f.timeout)) {
		fi, err := f.mfs.Stat(fpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.Stat: " + err.Error())
		}
		val.updateFi(fi)
	} else {
		//fmt.Println("Saved a stat by switching to shimfs!")
	}
	return val, nil
}

//Stuff to comply with minfs.MinFS
func (f *shimFS) Close() error {
	if f.closed {
		return errors.New("ShimFS: Close: Already Closed")
	}
	f.closed = true
	f.filecacheLock.Lock()
	for key,_ := range f.filecache {
		delete(f.filecache, key)
	}
	f.filecacheLock.Unlock()

	return f.mfs.Close()
}
	
func (f *shimFS) ReadFile(name string, b []byte, off int64) (int, error) {
	if f.closed {
		return 0, errors.New("ShimFS: ReadFile: Already Closed")
	}
	if len(b) == 0 {
		return 0, nil
	}
	if off < 0 {
		return 0, errors.New("ShimFS: ReadFile: Negative offset")
	}
	file, err := f.interStat(name)
	if err != nil {
		return 0, err
	}
	if file.IsDir() {
		return 0, errors.New("ShinFS: ReadFile called on Directory: " + name)
	}

	unfulfilled, bytesFromCache := file.read(swath{off, b[0:len(b)]})

	if unfulfilled == nil { //file was recently deleted
		return 0, os.ErrNotExist
	}
	if bytesFromCache > 0 {
		//fmt.Println("Saved", bytesFromCache, "bytes by switching to shimfs!")
	}

	//Any swaths left to fullfill gets served from the source
	bytesFromSource := 0
	for _, job := range unfulfilled {
			nread, errread := f.mfs.ReadFile(file.path(), job.array, job.off)
			if errread != nil {
				err = errread
			}
			if nread > 0 {
				bytesFromSource += nread
				file.cache(swath{off: job.off, array: job.array[:nread]})
			}
		}

	return int(bytesFromCache) + bytesFromSource, err
}

func (f *shimFS) WriteFile(name string, b []byte, offset int64) (int, error) {
	if f.closed {
		return 0, errors.New("ShimFS: WriteFile: Already Closed")
	}
	if len(b) == 0 {
		return 0, nil
	}

	if offset < 0 {
		return 0, errors.New("ShimFS: WriteFile: Negative offset")
	}
	file, err := f.interStat(name)
	if err != nil {
		return 0, err
	}

	//TODO: check permission to see if we're allowed to write first (also do that for all the other operations)
	
	if file.IsDir() {
		return 0, errors.New("ShinFS: WriteFile called on Directory: " + name)
	}

	nFC := file.write(swath{off: offset, array: b}, false)
	if nFC == nil { //file was deleted recently
		return 0, os.ErrNotExist
	}
	return f.mfs.WriteFile(name, b, offset)
}

func (f *shimFS) CreateFile(name string) error {
	if f.closed {
		return errors.New("ShimFS: CreateFile: Already Closed")
	}
	return f.mfs.CreateFile(name)
}

func (f *shimFS) CreateDirectory(name string) error {
	if f.closed {
		return errors.New("ShimFS: CreateDirectory: Already Closed")
	}
	return f.mfs.CreateDirectory(name)
}

func (f *shimFS) Move(oldpath string, newpath string) error {
	if f.closed {
		return errors.New("ShimFS: Move: Already Closed")
	}

	oldpath = path.Clean(oldpath)
	newpath = path.Clean(newpath)

	file, err := f.interStat(oldpath)
	if err != nil {
		return err
	}
	err = f.mfs.Move(oldpath, newpath)

	if err == nil {
		//update file
		file.changePath(newpath)

		f.filecacheLock.Lock()
		//Update the f.filecache
		f.filecache[newpath] = file
		delete(f.filecache, oldpath)

		//force parent to update dirlist next readdir
		parent, ok := f.filecache[(path.Dir(oldpath))]
		if ok {
			parent.invalidateDirAge()
		}
		f.filecacheLock.Unlock()

		//update the FI
		tFI, _ := f.mfs.Stat(newpath)
		file.updateFi(tFI)

		//if is a dir, handle it's children as well
		if file.IsDir() {
			if !strings.HasSuffix(oldpath, "/") {
				oldpath += "/"
			}
			if !strings.HasSuffix(newpath, "/") {
				newpath += "/"
			}
			trimLength := len(oldpath)
			f.filecacheLock.Lock()
			for opath, sfi := range f.filecache {
				if strings.HasPrefix(opath, oldpath) {
					npath := newpath + opath[trimLength:]
					sfi.changePath(npath)
					f.filecache[npath] = sfi
					delete(f.filecache, opath)
				}
			}
			f.filecacheLock.Unlock()
		}
	}
	return err
}

func (f *shimFS) Remove(name string, recursive bool) error {
	if f.closed {
		return errors.New("ShimFS: Remove: Already Closed")
	}
	name = path.Clean(name)
	file, err := f.interStat(name)
	if err != nil {
		return err
	}
	err = f.mfs.Remove(name, recursive)
	if err != nil {
		return err
	}
	f.filecacheLock.Lock()
	delete(f.filecache, file.path())

	if file.IsDir() && recursive {
		dirpath := file.path()
		if !strings.HasSuffix(dirpath, "/") {
			dirpath += "/"
		}

		for path, sfi := range f.filecache {
			if strings.HasPrefix(path, dirpath) {
				delete(f.filecache, path)
				sfi.delete()
			}
		}
	}
	f.filecacheLock.Unlock()
	file.delete()

	//force parent to update dirlist next readdir
	parent, err := f.interStat(path.Dir(name))
	parent.invalidateDirAge()
	return nil
}

func (f *shimFS) ReadDirectory(dirpath string) ([]os.FileInfo, error) {
	if f.closed {
		return nil, errors.New("ShimFS: ReadDirectory: Already Closed")
	}
	dirpath = path.Clean(dirpath)
	f.filecacheLock.RLock()
	val, ok := f.filecache[dirpath]
	f.filecacheLock.RUnlock()
	var err error
	if !ok { //we've never done a stat on this path
		val, err = f.interStat(dirpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.ReadDirectory Stat: " + err.Error())
		}
	}

	//check if cache too old
	if time.Now().After(val.dirAge().Add(f.timeout)) {
		fia, err := f.mfs.ReadDirectory(dirpath)
		if err != nil {
			return nil, errors.New("Error on shimFS.ReadDirectory read of path: " + err.Error())
		}
		diritems := make([]string, len(fia))

		for i, entry := range fia {
			diritems[i] = entry.Name()

			npath := path.Clean(dirpath + "/" + entry.Name())
			f.filecacheLock.RLock()
			eval, ok := f.filecache[npath]
			f.filecacheLock.RUnlock()

			if ok {
				eval.updateFi(entry)
			} else {
				f.addFI(npath, entry)
			}

		}
		val.updateDir(diritems)
		return fia, nil
	} else {
		//fmt.Println("Saved a readdir by switching to shimfs!")
	}

	diritems := val.dirItems()
	retval := make([]os.FileInfo, len(diritems))
	if dirpath == "/" { //are we at root?
		dirpath = "" //Since we'll be adding a slash later, make dirpath empty
	}
	f.filecacheLock.RLock()
	for i, name := range diritems {
		retval[i] = f.filecache[dirpath+"/"+name]
	}
	f.filecacheLock.RUnlock()
	return retval, nil
}

func (f *shimFS) GetAttribute(path string, attribute string) (interface{}, error) {
	if f.closed {
		return nil, errors.New("ShimFS: GetAttribute: Already Closed")
	}
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
		return f.mfs.GetAttribute(path, attribute)
	}
}

func (f *shimFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	if f.closed {
		return errors.New("ShimFS: SetAttribute: Already Closed")
	}
	
	switch attribute {
	case "modtime":
		return f.mfs.SetAttribute(path, attribute, newvalue)
	case "size":
		return f.mfs.SetAttribute(path, attribute, newvalue)
	}
	return errors.New("SetAttribute Error: Unsupported attribute " + attribute)
}

func (f *shimFS) Stat(fpath string) (os.FileInfo, error) {
	if f.closed {
		return nil, errors.New("ShimFS: Stat: Already Closed")
	}
	return f.interStat(fpath)
}

func (f *shimFS) String() string {
	retVal :=  "shimFS( " + f.mfs.String() + " )"
	if f.closed {
		retVal += " (closed)"
	}
	return retVal
}

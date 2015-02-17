package shimfs

import (
	"os"
	"sync"
	"time"
)

const maxChunkSize = 1024 * 1024 * 4

type shimFI struct {
	fpath string
	uid   int
	size  int64
	isdir bool

	modtime  time.Time
	fi       os.FileInfo //original fileinfo
	fiage    time.Time   //age of original fileinfo
	fileLock sync.RWMutex

	//if a file
	writechunks []*fileChunk //written fileChunks, in approximate age order
	cachechunks []*fileChunk //cached fileChunks, in approximate age order

	//if a directory
	dirage   time.Time //age of last readdir, if directory
	diritems []string  //items in, if directory
}

func newShimFI(fullpath string, fileinf os.FileInfo, uniqueid int) *shimFI {
	var fl sync.RWMutex

	if fileinf.IsDir() {
		return &shimFI{
			fpath:    fullpath,
			uid:      uniqueid,
			size:     fileinf.Size(),
			isdir:    true,
			modtime:  fileinf.ModTime(),
			fi:       fileinf,
			fiage:    time.Now(),
			fileLock: fl,
		}
	} else {
		return &shimFI{
			fpath:       fullpath,
			uid:         uniqueid,
			size:        fileinf.Size(),
			isdir:       false,
			modtime:     fileinf.ModTime(),
			fi:          fileinf,
			fiage:       time.Now(),
			fileLock:    fl,
			writechunks: make([]*fileChunk, 0),
			cachechunks: make([]*fileChunk, 0),
		}
	}
}

func (f *shimFI) path() string {
	f.fileLock.RLock()
	retVal := f.fpath
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) updateFi(fi os.FileInfo) {
	f.fileLock.Lock()
	f.fiage = time.Now()
	f.fi = fi
	if fi.Size() > f.size {
		f.size = fi.Size()
	}
	if fi.ModTime().After(f.modtime) {
		f.modtime = fi.ModTime()
	}
	f.fileLock.Unlock()
}

func (f *shimFI) updateDir(diritems []string) {
	f.fileLock.Lock()
	f.dirage = time.Now()
	f.diritems = make([]string, len(diritems))
	copy(f.diritems, diritems)
	f.fileLock.Unlock()
}

func (f *shimFI) fiAge() time.Time {
	f.fileLock.RLock()
	retVal := f.fiage
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) dirAge() time.Time {
	f.fileLock.RLock()
	retVal := f.dirage
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) dirItems() []string {
	f.fileLock.RLock()
	dt := make([]string, len(f.diritems))
	copy(dt, f.diritems)
	f.fileLock.RUnlock()
	return dt
}

type swath struct {
	off   int64
	array []byte
}

//takes a read request, fills as much as it can, and returns a list of unfilled spaces
func (f *shimFI) read(deets swath) ([]swath, int64) {
	f.fileLock.RLock()
	if f.writechunks == nil { //file got deleted before now, so forget it
		f.fileLock.RUnlock()
		return nil, 0
	}

	if len(f.writechunks) == 0 && len(f.cachechunks) == 0 {
		f.fileLock.RUnlock()
		return []swath{deets}, 0
	}

	//Make a copy of the chunklists and unlock quick, we don't want to block the file for too long
	chunks := make([]*fileChunk, len(f.writechunks)+len(f.cachechunks))
	ccc := copy(chunks, f.writechunks)
	copy(chunks[ccc:], f.cachechunks)
	f.fileLock.RUnlock()

	unfulfilled := []swath{deets}
	bytesCopied := int64(0)
	//iterate through the chunks, fill in the request where we can
	for _, chunk := range chunks {
		chunk.chunkLock.RLock()
		if chunk.width == -1 { //this chunk got deleted, ignore it
			chunk.chunkLock.RUnlock()
			continue
		}

		chend := chunk.off + chunk.width
		nunf := make([]swath, 0, 2*len(unfulfilled)) //max swath creation is twice the original number of requests

		//compare against everything we have yet to fullfil
		for _, sw := range unfulfilled {
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
			if sw.off > chend || swend <= chunk.off {
				nunf = append(nunf, sw)
				continue
			}
			//B: covers everything, fullfil and continue
			if chunk.off <= sw.off && swend <= chend {
				bytesCopied += chunk.read(sw)
				continue
			}
			//C: covers the beginning, fill that and save what's needed
			if chunk.off <= sw.off {
				bytesCopied += chunk.read(swath{sw.off, sw.array[:chend-sw.off]})
				nunf = append(nunf, swath{chend, sw.array[chend-sw.off:]})
				continue
			}
			//D: covers the end, fill that and save what's needed
			if swend <= chend {
				bytesCopied += chunk.read(swath{chunk.off, sw.array[chunk.off-sw.off:]})
				nunf = append(nunf, swath{sw.off, sw.array[:chunk.off-sw.off]})
				continue
			}
			//E: covers the middle, fill, and split need in two
			bytesCopied += chunk.read(swath{chunk.off, sw.array[chunk.off-sw.off : chend-sw.off]})
			nunf = append(nunf, swath{sw.off, sw.array[:chunk.off-sw.off]})
			nunf = append(nunf, swath{chend, sw.array[chend-sw.off:]})
		}
		unfulfilled = nunf
		chunk.chunkLock.RUnlock()
	}
	return unfulfilled, bytesCopied
}

//Adds a looked-up swath to the cache array
func (f *shimFI) cache(deets swath) {
	nFC := newFileChunk(deets, true)
	top := deets.off + int64(len(deets.array))

	f.fileLock.Lock()
	if f.cachechunks == nil { //file got deleted before now, so forget it
		f.fileLock.Unlock()
		return
	}
	f.cachechunks = append([]*fileChunk{nFC}, f.cachechunks...)
	if top > f.size {
		f.size = top
	}
	f.fileLock.Unlock()
}

//Adds a newly written swath to the written array, returns created fileChunk
func (f *shimFI) write(deets swath, isSynced bool) *fileChunk {

	nFC := newFileChunk(deets, isSynced)
	top := deets.off + int64(len(deets.array))

	f.fileLock.Lock()
	if f.writechunks == nil { //file got deleted before now, so forget it
		f.fileLock.Unlock()
		return nil
	}
	f.writechunks = append([]*fileChunk{nFC}, f.writechunks...)
	if top > f.size {
		f.size = top
	}
	f.modtime = time.Now()
	f.fileLock.Unlock()

	return nFC
}

//Cleans up if this file is being deleted
func (f *shimFI) delete() error {
	var retVal error
	f.fileLock.Lock()
	if f.isdir {
		if f.diritems == nil { //file already deleted
			retVal = os.ErrNotExist
		}
		f.diritems = nil
	} else {
	if f.writechunks == nil { //file already deleted
			retVal = os.ErrNotExist
	}
	for _, chunk := range f.writechunks {
		chunk.chunkLock.Lock()
		chunk.delete()
		chunk.chunkLock.Unlock()
	}
	f.writechunks = nil
	for _, chunk := range f.cachechunks {
		chunk.chunkLock.Lock()
		chunk.delete()
		chunk.chunkLock.Unlock()
	}
	f.cachechunks = nil
	}
	f.fileLock.Unlock()
	return retVal
}

//Complying with os.FileSystem
func (f *shimFI) Name() string {
	f.fileLock.RLock()
	retVal := f.fi.Name()
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) Size() int64 {
	f.fileLock.RLock()
	size := f.size
	f.fileLock.RUnlock()
	return size
}

func (f *shimFI) Mode() os.FileMode {
	f.fileLock.RLock()
	retVal := f.fi.Mode()
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) ModTime() time.Time {
	f.fileLock.RLock()
	retVal := f.modtime
	f.fileLock.RUnlock()
	return retVal
}

func (f *shimFI) IsDir() bool {
	return f.isdir
}

func (f *shimFI) Sys() interface{} {
	return nil
}

package shimfs

import (
	"os"
	"sync"
	"time"
)

type fileInfo struct {
	fullpath string
	modtime  time.Time
	fi       os.FileInfo  //original fileinfo
	fiage    time.Time    //age of original fileinfo
	chunks   []fileChunk //fileChunks, if file

	dirage   time.Time //age of last readdir, if directory
	diritems []string  //items in, if directory
	fileLock sync.RWMutex
}

type fileChunk struct {
	offset   int64
	size     int64
	isSynced bool
	memback  []byte
	hdback   string
	age      time.Time
}

func (f *fileInfo) GetChunks() []fileChunk {
	f.fileLock.RLock()
	chunks := make([]fileChunk, len(f.chunks))
	copy(chunks, f.chunks)
	f.fileLock.RUnlock()
	return chunks
}

func (f *fileInfo) AddChunk(fc fileChunk) {
	f.fileLock.Lock()
	chunks := make([]fileChunk, len(f.chunks)+1)
	copy(chunks[1:], f.chunks)
	chunks[0] = fc
	f.chunks = chunks
	f.fileLock.Unlock()
}

func (f *fileInfo) RemoveChunk(offset int64) {
	f.fileLock.Lock()
	removeAt := -1
	for i, c := range f.chunks {
		if c.offset == offset {
			removeAt = i
			break
		}
	}
	if removeAt != -1 {
		chunks := make([]fileChunk, len(f.chunks)-1)
		copy(chunks[0:removeAt], f.chunks[0:removeAt])
		copy(chunks[removeAt:], f.chunks[removeAt+1:])
		f.chunks = chunks
	}
	f.fileLock.Unlock()
}

func (f *fileInfo) FiAge() time.Time {
	f.fileLock.RLock()
	retVal := f.fiage
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) DirAge() time.Time {
	f.fileLock.RLock()
	retVal := f.dirage
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) UpdateFi(fi os.FileInfo) {
	f.fileLock.Lock()
	f.fiage = time.Now()
	f.fi = fi
	if fi.ModTime().After(f.modtime) {
		f.modtime = fi.ModTime()
	}
	f.fileLock.Unlock()
}

func (f *fileInfo) UpdateDir(diritems []string) {
	f.fileLock.Lock()
	f.dirage = time.Now()
	f.diritems = make([]string, len(diritems))
	copy(f.diritems, diritems)
	f.fileLock.Unlock()
}

func (f *fileInfo) DirItems() []string {
	f.fileLock.RLock()
	dt := make([]string, len(f.diritems))
	copy(dt, f.diritems)
	f.fileLock.RUnlock()
	return dt
}

func (f *fileInfo) Name() string {
	f.fileLock.RLock()
	retVal := f.fi.Name()
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) Path() string {
	f.fileLock.RLock()
	retVal := f.fullpath
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) Size() int64 {
	f.fileLock.RLock()
	size := f.fi.Size()
	if f.chunks != nil {
		//check if any of the chunks extend beyond the original size
		for _, fc := range f.chunks {
			max := fc.offset + fc.size
			if max > size {
				size = max
			}
		}
	}
	f.fileLock.RUnlock()
	return size
}

func (f *fileInfo) Mode() os.FileMode {
	f.fileLock.RLock()
	retVal := f.fi.Mode()
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) ModTime() time.Time {
	f.fileLock.RLock()
	retVal := f.modtime
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) IsDir() bool {
	f.fileLock.RLock()
	retVal := f.fi.IsDir()
	f.fileLock.RUnlock()
	return retVal
}

func (f *fileInfo) Sys() interface{} {
	return nil
}

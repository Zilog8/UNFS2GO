package shimfs

import (
	"os"
	"sync"
	"time"
)

type shimFI struct {
	uid   int
	size  int64
	isdir bool

	modtime  time.Time
	fi       os.FileInfo //original fileinfo
	fiage    time.Time   //age of original fileinfo
	fileLock *sync.RWMutex

	//if a directory
	dirage   time.Time //age of last readdir, if directory
	diritems []string  //items in, if directory
}

func newShimFI(fullpath string, fileinf os.FileInfo, uniqueid int) *shimFI {
	if fileinf.IsDir() {
		return &shimFI{
			uid:      uniqueid,
			size:     fileinf.Size(),
			isdir:    true,
			modtime:  fileinf.ModTime(),
			fi:       fileinf,
			fiage:    time.Now(),
			diritems: make([]string, 0),
			fileLock: new(sync.RWMutex),
		}
	} else {
		return &shimFI{
			uid:         uniqueid,
			size:        fileinf.Size(),
			isdir:       false,
			modtime:     fileinf.ModTime(),
			fi:          fileinf,
			fiage:       time.Now(),
			fileLock:    new(sync.RWMutex),
		}
	}
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

func (f *shimFI) invalidateFiAge() {
	f.fileLock.Lock()
	f.fiage = *new(time.Time)
	f.fileLock.Unlock()
}

func (f *shimFI) invalidateDirAge() {
	f.fileLock.Lock()
	f.dirage = *new(time.Time)
	f.fileLock.Unlock()
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

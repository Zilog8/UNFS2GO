package shimfs

import (
	"../minfs"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"time"
)

//TODO: make read cache, Test shimfs, make write cache, Test shimfs

//Acts as a buffer for reads and writes to another MinFS filesystem
type shimFS struct {
	tempPath    string        //path to use as a temporary buffer
	tempSize    int64         //max total space to use for buffer
	mfs         minfs.MinFS   //Filesystem being shimmed
	invalid     time.Duration //invalidation time
	fiCache     map[string]*shimFI
	fiCacheLock *sync.RWMutex
	giudCounter chan int
	closed      bool //is usually false, unless the shimfs has been closed
}

func New(tempPath string, tempSize int64, mfs minfs.MinFS) (minfs.MinFS, error) {
	//Clean out any residuals from previous runs
	qw, _ := os.Open(tempPath)
	qa, _ := qw.Readdirnames(-1)
	for _, qd := range qa {
		if strings.HasSuffix(qd, ".shimfs") {
			os.Remove(pathpkg.Clean(tempPath + "/" + qd))
		}
	}
	qw.Close()

	//Check tempPath is usable
	file, err := os.Create(pathpkg.Clean(tempPath + "/test.shimfs"))
	if err != nil {
		return nil, errors.New("Couldn't create in cache directory:" + tempPath + " Error:" + err.Error())
	}
	_, err = file.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	if err != nil {
		return nil, errors.New("Couldn't write to cache directory:" + tempPath + " Error:" + err.Error())
	}
	file.Close()
	err = os.Remove(pathpkg.Clean(tempPath + "/test.shimfs"))
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
	f.fiCacheLock.Lock()
	f.fiCache[fpath] = eval
	f.fiCacheLock.Unlock()
	return eval
}

func (f *shimFS) deleteFI(fpath string) {
	f.fiCacheLock.Lock()
	delete(f.fiCache, fpath) //remove item from cache
	f.fiCacheLock.Unlock()

	//if parent cached, force parent to update dirlist next readdir
	f.invalidateParent(fpath)
}

func (f *shimFS) invalidate(fpath string) {
	f.fiCacheLock.RLock()
	val, ok := f.fiCache[fpath]
	f.fiCacheLock.RUnlock()
	if ok {
		val.invalidateFiAge()
		if val.isdir {
			val.invalidateDirAge()
		}
	}
}

func (f *shimFS) invalidateParent(fpath string) {
	parent := pathpkg.Dir(fpath)
	f.fiCacheLock.RLock()
	val, ok := f.fiCache[parent] //try-get parent from cache
	f.fiCacheLock.RUnlock()

	if ok {
		val.invalidateDirAge()
	}
}

func (f *shimFS) interStat(fpath string) (*shimFI, error) {
	fpath = pathpkg.Clean(fpath)
	f.fiCacheLock.RLock()
	val, ok := f.fiCache[fpath]
	f.fiCacheLock.RUnlock()

	//if we've never done a stat on this path, make a new entry
	if !ok {
		fi, err := f.mfs.Stat(fpath)
		//fmt.Println("shimfs passed a stat to base:", fpath)
		if err != nil {
			return nil, err
		}
		val = f.addFI(fpath, fi)
	}

	//if entry too old, refresh
	if time.Now().After(val.fiAge().Add(f.invalid)) {
		fi, err := f.mfs.Stat(fpath)
		//fmt.Println("shimfs passed a stat to base:", fpath)
		if err != nil {
			if err == os.ErrNotExist { //if entry has ceased to exist, remove
				f.deleteFI(fpath)
			}
			return nil, err
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
		return os.ErrInvalid
	}
	f.closed = true
	f.fiCacheLock.Lock()
	for key, _ := range f.fiCache {
		delete(f.fiCache, key)
	}
	f.fiCacheLock.Unlock()

	return f.mfs.Close()
}

func (f *shimFS) ReadFile(name string, b []byte, off int64) (int, error) {
	if f.closed {
		return 0, os.ErrInvalid
	}
	return f.mfs.ReadFile(name, b, off)
}
func (f *shimFS) WriteFile(name string, b []byte, offset int64) (int, error) {
	if f.closed {
		return 0, os.ErrInvalid
	}
	name = pathpkg.Clean(name)
	f.invalidateParent(name)
	f.invalidate(name)
	return f.mfs.WriteFile(name, b, offset)
}

func (f *shimFS) CreateFile(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	name = pathpkg.Clean(name)
	f.invalidateParent(name)
	f.invalidate(name)
	return f.mfs.CreateFile(name)
}

func (f *shimFS) CreateDirectory(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	name = pathpkg.Clean(name)
	f.invalidateParent(name)
	return f.mfs.CreateDirectory(name)
}

func (f *shimFS) Move(oldpath string, newpath string) error {
	if f.closed {
		return os.ErrInvalid
	}

	oldpath = pathpkg.Clean(oldpath)
	newpath = pathpkg.Clean(newpath)

	err := f.mfs.Move(oldpath, newpath)

	if err != nil {
		f.invalidate(oldpath)
		f.invalidateParent(oldpath)
		f.invalidate(newpath)
		f.invalidateParent(newpath)
		return err
	}

	//Swap the FileInfo from one path to the other
	f.fiCacheLock.Lock()
	fi := f.fiCache[oldpath]
	f.fiCache[newpath] = fi
	delete(f.fiCache, oldpath)
	f.fiCacheLock.Unlock()

	//Invalidate stat and readdir for what's left
	f.invalidateParent(oldpath)
	f.invalidate(newpath)
	f.invalidateParent(newpath)

	//if is a dir, handle it's children as well
	if fi.isdir {
		if !strings.HasSuffix(oldpath, "/") {
			oldpath += "/"
		}
		if !strings.HasSuffix(newpath, "/") {
			newpath += "/"
		}
		trimLength := len(oldpath)
		f.fiCacheLock.Lock()
		for opath, sfi := range f.fiCache {
			if strings.HasPrefix(opath, oldpath) {
				npath := newpath + opath[trimLength:]
				f.fiCache[npath] = sfi
				delete(f.fiCache, opath)
			}
		}
		f.fiCacheLock.Unlock()
	}

	return nil
}

func (f *shimFS) Remove(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	name = pathpkg.Clean(name)
	err := f.mfs.Remove(name)
	if err != nil {
		//On the off chance the item was removed, invalidate it's cache
		//Next time it gets looked up, interStat is forced to Stat it
		//And either update the cache entry or delete it.
		f.invalidate(name)
		f.invalidateParent(name)
		return err
	}

	f.deleteFI(name)

	return nil
}

func (f *shimFS) ReadDirectory(dirpath string) ([]os.FileInfo, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	dirpath = pathpkg.Clean(dirpath)
	f.fiCacheLock.RLock()
	val, ok := f.fiCache[dirpath]
	f.fiCacheLock.RUnlock()
	var err error
	if !ok { //we've never done a stat on this path
		val, err = f.interStat(dirpath)
		//fmt.Println("shimfs passed a readdir to base:", dirpath)
		if err != nil {
			return nil, err
		}
	}

	//check if cache too old
	if time.Now().After(val.dirAge().Add(f.invalid)) {
		fia, err := f.mfs.ReadDirectory(dirpath)
		//fmt.Println("shimfs passed a readdir to base:", dirpath)
		if err != nil {
			return nil, err
		}
		diritems := make([]string, len(fia))

		for i, entry := range fia {
			diritems[i] = entry.Name()

			npath := pathpkg.Clean(dirpath + "/" + entry.Name())
			f.fiCacheLock.RLock()
			eval, ok := f.fiCache[npath]
			f.fiCacheLock.RUnlock()

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
	f.fiCacheLock.RLock()
	for i, name := range diritems {
		retval[i] = f.fiCache[dirpath+"/"+name]
	}
	f.fiCacheLock.RUnlock()
	return retval, nil
}

func (f *shimFS) GetAttribute(fpath string, attribute string) (interface{}, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	fpath = pathpkg.Clean(fpath)
	fi, err := f.interStat(fpath)
	if err != nil {
		return nil, err
	}

	switch attribute {
	case "modtime":
		return fi.ModTime(), nil
	case "mode":
		return fi.Mode(), nil
	case "size":
		return fi.Size(), nil
	default:
		fmt.Println("GetAttribute Error: Unsupported attribute " + attribute)
		return nil, os.ErrInvalid
	}
}

func (f *shimFS) SetAttribute(fpath string, attribute string, newvalue interface{}) error {
	if f.closed {
		return os.ErrInvalid
	}
	fpath = pathpkg.Clean(fpath)
	f.invalidate(fpath)

	switch attribute {
	case "modtime":
		return f.mfs.SetAttribute(fpath, attribute, newvalue)
	case "mode":
		return f.mfs.SetAttribute(fpath, attribute, newvalue)
	case "size":
		return f.mfs.SetAttribute(fpath, attribute, newvalue)
	default:
		fmt.Println("GetAttribute Error: Unsupported attribute " + attribute)
		return os.ErrInvalid
	}
}

func (f *shimFS) Stat(fpath string) (os.FileInfo, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	return f.interStat(fpath)
}

func (f *shimFS) String() string {
	retVal := "shimFS( " + f.mfs.String() + " )"
	if f.closed {
		retVal += " (closed)"
	}
	return retVal
}

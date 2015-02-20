// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var fddb fdCache //translator for file descriptors

//export go_init
func go_init() C.int {
	fddb = fdCache{FDlistLock: new(sync.RWMutex), PathMapA: make(map[string]int), PathMapB: make(map[int]string), FDcounter: 100}
	return 0
}

//export go_fgetpath
func go_fgetpath(fd C.int) *C.char {
	gofd := int(fd)
	path, err := fddb.GetPath(gofd)
	if err != nil {
		fmt.Println("Error on go_fgetpath (fd =", gofd, " of ", fddb.FDcounter, ");", err)
		return nil
	} else {
		//fmt.Println("go_fgetpath: Returning '", path, "' for fd:", gofd)
		return C.CString(path)
	}
}

//export go_readdir_helper
func go_readdir_helper(dirpath *C.char, entryIndex C.int) *C.char {

	pp := pathpkg.Clean("/" + C.GoString(dirpath))
	index := int(entryIndex)
	arr, err := ns.ReadDirectory(pp)

	if err != nil {
		fmt.Println("Error go_readdir_helper path=", pp, "index=", index, "error=", err)
		return C.CString("")
	}
	if index >= len(arr) {
		fmt.Println("Error go_readdir_helper path=", pp, "index=", index, "error=", "index too high for directory contents")
		return C.CString("")
	}
	return C.CString(arr[index].Name())
}

//export go_opendir_helper
func go_opendir_helper(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	arr, err := ns.ReadDirectory(pp)

	if err != nil {
		fmt.Println("Error go_opendir_helper path=", pp, "error=", err)
		return -1
	}
	return C.int(len(arr))
}

//export go_open
func go_open(path *C.char, flags C.int) C.int {
	//Return the filedescriptor for this path
	//If file doesn't exist, return -1
	pp := pathpkg.Clean("/" + C.GoString(path))
	res := fddb.GetFD(pp)
	if res > -1 {
		//check if it's actually a file
		fi, err := ns.Stat(pp)
		if err != nil {
			//fmt.Println("Error go_open statin': ", path, err)
			return -1
		}
		if fi.IsDir() {
			//fmt.Println("Error go_open: ", pp, " is a directory.")
			return -1
		} else {
			return C.int(res)
		}
	} else {
		return -1
	}
}

//export go_close
func go_close(fd C.int) C.int {
	return C.int(0)
}

//export go_fstat
func go_fstat(fd C.int, buf *C.go_statstruct) C.int {
	gofd := int(fd)
	pp, err := fddb.GetPath(gofd)
	if err == nil {
		return getStat(pp, gofd, buf)
	} else {
		fmt.Println("Error go_fstat: GetPath Failed:", err)
		return -1
	}
}

//export go_lstat
func go_lstat(path *C.char, buf *C.go_statstruct) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	fd := fddb.GetFD(pp)
		return getStat(pp, fd, buf)
}

//export go_shutdown
func go_shutdown() {
	shutDown()
}

//export go_fchown
func go_fchown(fd C.int, owner C.int, group C.int) C.int {
	gofd := int(fd)
	pp, err := fddb.GetPath(gofd)
	if err == nil {
		err = ns.SetAttribute(pp, "own", []int{int(owner), int(group)})
		if err == nil {
			return 0
		}
	}
		return -1
	}

//export go_lchown
func go_lchown(path *C.char, owner C.int, group C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	err := ns.SetAttribute(pp, "own", []int{int(owner), int(group)})
	if err == nil {
		return 0
	}
	return -1
}

//export go_fchmod
func go_fchmod(fd C.int, mode C.int) C.int {
	gofd := int(fd)
	pp, err := fddb.GetPath(gofd)
	if err == nil {
		err = ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))
		if err == nil {
			return 0
		}
	}
	return -1
}

//export go_chmod
func go_chmod(path *C.char, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	err := ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))
	if err == nil {
		return 0
	}
	return -1
}

//export go_truncate
func go_truncate(path *C.char, offset3 C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	off := int64(offset3)
	err := ns.SetAttribute(pp, "size", off)
	if err != nil {
		fmt.Println("Error on truncate of", pp, "(size =", off, ")", err)
	return -1
}
	return 0
}

//export go_rename
func go_rename(oldpath *C.char, newpath *C.char) C.int {
	op := pathpkg.Clean("/" + C.GoString(oldpath))
	np := pathpkg.Clean("/" + C.GoString(newpath))

	fi, err := ns.Stat(op)
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "not found") || strings.Contains(lower, "not exist") {
			fmt.Println("Error rename: not found:", op)
			return -2
		}
		fmt.Println("Error rename: ", op, " internal stat errored:", err)
		return -1
	}

	err = ns.Move(op, np)
	if err != nil {
		fmt.Println("Error on rename", op, " to ", np, " due to ", err)
		return -1
	}
	fddb.ReplacePath(op, np, fi.IsDir())
	return 0
}

//export go_utime_helper
func go_utime_helper(path *C.char, actime C.int, modtime C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	mod := time.Unix(int64(modtime), 0)
	err := ns.SetAttribute(pp, "modtime", mod)
	if err != nil {
		fmt.Println("Error setting times:", pp, mod, err)
		return -1
	}
	return 0
}

//export go_ftruncate
func go_ftruncate(fd C.int, offset3 C.int) C.int {
	gofd := int(fd)
	off := int64(offset3)
	pp, err := fddb.GetPath(gofd)
	if err != nil {
		fmt.Println("Error on ftruncate of (fd=", fd, "size =", off, ")", err)
		return -1
	}
	err = ns.SetAttribute(pp, "size", off)
	if err != nil {
		fmt.Println("Error on ftruncate of", pp, "(fd=", fd, "size =", off, ")", err)
	return -1
}
	return 0
}

//export go_open_create
func go_open_create(pathname *C.char, flags C.int, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(pathname))
	err := ns.CreateFile(pp)
	if err != nil {
		fmt.Println("Error open_create file at create: ", pp, " due to: ", err)
		return -1
	}
	return C.int(fddb.GetFD(pp))
}

//export go_remove
func go_remove(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	st, err := ns.Stat(pp)

	if err != nil {
		fmt.Println("Error removing file: ", pp, "\n", err)
		return -1
	}

	//it seems most shells already check for this, but no harm being extra careful.
	if st.IsDir() {
		fmt.Println("Error removing file: ", pp, "\n Is a directory.")
		return -1
	}

	err = ns.Remove(pp, false)
	if err != nil {
		fmt.Println("Error removing file: ", pp, "\n", err)
		return -1
	}
	return 0
}

//export go_rmdir_helper
func go_rmdir_helper(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))

	st, err := ns.Stat(pp)
	
	if err != nil {
		//fmt.Println("Error removing directory: ", pp, "\n", err)
		return -1
	}

	//it seems most shells already check for this, but no harm being extra careful.
	if !st.IsDir() {
		//fmt.Println("Error removing directory: ", pp, "\n Not a directory.")
		return -1
	}

	err = ns.Remove(pp, false)
	if err != nil {
		//fmt.Println("Error removing directory: ", pp, "\n", err)
		if strings.Contains(err.Error(), "directory not empty") {
			return -2
		}
		return -1
	}
	return 0
}

//export go_mkdir
func go_mkdir(path *C.char, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	err := ns.CreateDirectory(pp)
	if err != nil {
		fmt.Println("Error making directory: ", pp, "\n", err)
		return -1
	}
	return 0
	}

//export go_nop
func go_nop(name *C.char) C.int {
	pp := C.GoString(name)
	fmt.Println("Unsupported Command: ", pp)
		return -1
	}

//export go_pwrite
func go_pwrite(fd C.int, buf unsafe.Pointer, count C.int, offset C.int) C.int {
	gofd := int(fd)
	off := int64(offset)
	counted := int(count)

	//prepare the provided buffer for use
	slice := &reflect.SliceHeader{Data: uintptr(buf), Len: counted, Cap: counted}
	cbuf := *(*[]byte)(unsafe.Pointer(slice))

	var (
		pp          string
		copiedBytes int
		err         error
	)
	pp, err = fddb.GetPath(gofd)
	if err != nil {
		fmt.Println("Error on pwrite (GetPath of fd =", gofd, ");", err)
		return -1
	}
	copiedBytes, err = ns.WriteFile(pp, cbuf, off)
	if err != nil {
		fmt.Println("Error on pwrite of", pp, "(fd =", gofd,
		") (start =", off, " count =", counted, ")", err)
	return -1
}
	return C.int(copiedBytes)

}

//export go_pread
func go_pread(fd C.int, buf unsafe.Pointer, count C.int, offset C.int) C.int {
	gofd := int(fd)
	off := int64(offset)
	counted := int(count)

	//prepare the provided buffer for use
	slice := &reflect.SliceHeader{Data: uintptr(buf), Len: counted, Cap: counted}
	cbuf := *(*[]byte)(unsafe.Pointer(slice))

	var (
		pp          string
		copiedBytes int
		err         error
	)
	pp, err = fddb.GetPath(gofd)
	if err != nil {
		fmt.Println("Error on pread (GetPath of fd =", gofd, ");", err)
		return -1
	}
	copiedBytes, err = ns.ReadFile(pp, cbuf, off)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		fmt.Println("Error on pread of", pp, "(fd =", gofd,
		") (start =", off, " count =", counted, ")", err)
	return -1
}
	return C.int(copiedBytes)
}

//export go_fsync
func go_fsync(fd C.int) C.int {
	gofd := int(fd)

	_, err := fddb.GetPath(gofd)
	if err != nil {
		fmt.Println("Error on fsync (fd =", gofd, ");", err)
	return -1
	}
	return 0
}

func getStat(pp string, fd int, buf *C.go_statstruct) C.int {
	fi, err := ns.Stat(pp)
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "not found") || strings.Contains(lower, "not exist") {
			fmt.Println("Error stat: not found:", pp)
			return -2
		}
		fmt.Println("Error stat: ", pp, " internal stat errored:", err)
		return -1
	}
	buf.st_dev = C.uint32(1)
	buf.st_ino = C.uint64(fd)
	buf.st_gen = C.uint32(fd)
	buf.st_size = C.uint64(fi.Size())
	buf.st_atime = C.time_t(time.Now().Unix())
	buf.st_mtime = C.time_t(fi.ModTime().Unix())
	buf.st_ctime = C.time_t(fi.ModTime().Unix())

	if fi.IsDir() {
		buf.st_mode = C.short(fi.Mode() | C.S_IFDIR)
	} else {
		buf.st_mode = C.short(fi.Mode() | C.S_IFREG)
	}
	return 0
}

type fdCache struct {
	PathMapA   map[string]int
	PathMapB   map[int]string
	FDcounter  int
	FDlistLock *sync.RWMutex
}

func (f *fdCache) GetPath(fd int) (string, error) {
	if fd < 100 {
		return "", errors.New(fmt.Sprint("Error GetPath, filedescriptor too low ", fd))
	}
	f.FDlistLock.RLock()
	path, ok := f.PathMapB[fd]
	f.FDlistLock.RUnlock()
	if ok {
		return path, nil
	} else {
		return "", errors.New(fmt.Sprint("Error GetPath, filedescriptor not found ", fd))
	}
}

func (f *fdCache) ReplacePath(oldpath, newpath string, isdir bool) {
	f.FDlistLock.Lock()
	fd := f.PathMapA[oldpath]
	delete(f.PathMapA, oldpath)
	delete(f.PathMapB, fd)
	f.PathMapA[newpath] = fd
	f.PathMapB[fd] = newpath

	if isdir {
		op := oldpath + "/"
		np := newpath + "/"
		for path, fh := range f.PathMapA {
			if strings.HasPrefix(path, op) {
				delete(f.PathMapA, path)
				delete(f.PathMapB, fh)
				path = strings.Replace(path, op, np, 1)
				f.PathMapA[path] = fh
				f.PathMapB[fh] = path
			}
		}
	}
	f.FDlistLock.Unlock()
}

func (f *fdCache) GetFD(path string) int {
	f.FDlistLock.RLock()
	i, ok := f.PathMapA[path]
	f.FDlistLock.RUnlock()
	if ok {
		return i
	}
	f.FDlistLock.Lock()
	f.FDcounter++
	newFD := f.FDcounter
	f.PathMapA[path] = newFD
	f.PathMapB[newFD] = path
	f.FDlistLock.Unlock()
	return newFD
}

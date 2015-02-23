// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
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
	fddb = fdCache{FDlistLock: new(sync.RWMutex), PathMap: make(map[string]int), FDcounter: 100}
	return 0
}

//export go_accept_mount
func go_accept_mount(addr C.int, path *C.char) C.int {
	a := uint32(addr)
	hostaddress := fmt.Sprintf("%d.%d.%d.%d", byte(a), byte(a>>8), byte(a>>16), byte(a>>24))
	gpath := pathpkg.Clean("/" + C.GoString(path))
	if strings.EqualFold(hostaddress, "127.0.0.1") { //TODO: Make this configurable
		fmt.Println("Host allowed to connect:", hostaddress, "path:", gpath)
		return 1
	} else {
		fmt.Println("Host not allowed to connect:", hostaddress, "path:", gpath)
		return 0
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
func go_open(path *C.char, flags C.int) C.int { //flags == 0 if RO, == 1 if RW
	pp := pathpkg.Clean("/" + C.GoString(path))

	//check if exists
	fi, err := ns.Stat(pp)
	if err != nil {
		//fmt.Println("Error go_open statin': ", path, err)
		return -1
	}

	//check if it's actually a file
	if fi.IsDir() {
		//fmt.Println("Error go_open: ", pp, " is a directory.")
		return -2
	}

	return 1
}

//export go_exists
func go_exists(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	_, err := ns.Stat(pp)
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "not found") || strings.Contains(lower, "not exist") {
			//fmt.Println("Error stat: not found:", pp)
			return -2
		}
		fmt.Println("Error stat: ", pp, " internal stat errored:", err)
		return -1
	}
	return 1
}

func errTranslator(err error) C.int {
	switch err {
	case os.ErrPermission:
		return -1
	case os.ErrNotExist:
		return -2
	case os.ErrInvalid:
		return -3
	default:
		return -4
	}
}

//export go_lstat
func go_lstat(path *C.char, buf *C.go_statstruct) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	fi, err := ns.Stat(pp)
	if err != nil {
		retVal := errTranslator(err)
		if retVal == -4 {
			fmt.Println("Error on lstat of", pp, "):", err)
		}
		return retVal
	}
	statTranslator(fi, fddb.GetFD(pp), buf)
	return 0
}

func statTranslator(fi os.FileInfo, fd_ino int, buf *C.go_statstruct) {
	buf.st_dev = C.uint32(1)
	buf.st_ino = C.uint64(fd_ino)
	buf.st_size = C.uint64(fi.Size())
	buf.st_atime = C.time_t(time.Now().Unix())
	buf.st_mtime = C.time_t(fi.ModTime().Unix())
	buf.st_ctime = C.time_t(fi.ModTime().Unix())

	if fi.IsDir() {
		buf.st_mode = C.short(fi.Mode() | C.S_IFDIR)
	} else {
		buf.st_mode = C.short(fi.Mode() | C.S_IFREG)
	}
}

//export go_shutdown
func go_shutdown() {
	shutDown()
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

//export go_open_create
func go_open_create(pathname *C.char, flags C.int, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(pathname))
	err := ns.CreateFile(pp)
	if err != nil {
		fmt.Println("Error open_create file at create: ", pp, " due to: ", err)
		return -1
	}
	return 1
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
func go_pwrite(path *C.char, buf unsafe.Pointer, count C.int, offset C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	off := int64(offset)
	counted := int(count)

	//prepare the provided buffer for use
	slice := &reflect.SliceHeader{Data: uintptr(buf), Len: counted, Cap: counted}
	cbuf := *(*[]byte)(unsafe.Pointer(slice))

	copiedBytes, err := ns.WriteFile(pp, cbuf, off)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "eof") {
		retVal := errTranslator(err)
		if retVal == -4 {
			fmt.Println("Error on pwrite of", pp, "(start =", off, "count =", counted, "copied =", copiedBytes, "):", err)
		}
		return retVal
	}
	return C.int(copiedBytes)

}

//export go_pread
func go_pread(path *C.char, buf unsafe.Pointer, count C.int, offset C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	off := int64(offset)
	counted := int(count)

	//prepare the provided buffer for use
	slice := &reflect.SliceHeader{Data: uintptr(buf), Len: counted, Cap: counted}
	cbuf := *(*[]byte)(unsafe.Pointer(slice))

	copiedBytes, err := ns.ReadFile(pp, cbuf, off)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "eof") {
		retVal := errTranslator(err)
		if retVal == -4 {
			fmt.Println("Error on pread of", pp, "(start =", off, "count =", counted, "copied =", copiedBytes, "):", err)
		}
		return retVal
	}
	return C.int(copiedBytes)
}

//export go_sync
func go_sync(path *C.char, buf *C.go_statstruct) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	fi, err := ns.Stat(pp)

	switch {
	case err == os.ErrPermission: //TODO: Add an " || (don't have write permissions in FI)"
		return -1
	case err == os.ErrNotExist:
		buf.st_dev = C.uint32(666) //hint that stat didn't work out
		return -2
	case fi.IsDir():
		return -3
	case err != nil:
		fmt.Println("Error on sync of", pp, ":", err)
		buf.st_dev = C.uint32(666) //hint that stat didn't work out
		return -4
	default:
		statTranslator(fi, fddb.GetFD(pp), buf)
		return 1
	}
}

type fdCache struct {
	PathMap    map[string]int
	FDcounter  int
	FDlistLock *sync.RWMutex
}

func (f *fdCache) ReplacePath(oldpath, newpath string, isdir bool) {
	f.FDlistLock.Lock()
	fd := f.PathMap[oldpath]
	delete(f.PathMap, oldpath)
	f.PathMap[newpath] = fd

	if isdir {
		op := oldpath + "/"
		np := newpath + "/"
		for path, fh := range f.PathMap {
			if strings.HasPrefix(path, op) {
				delete(f.PathMap, path)
				path = strings.Replace(path, op, np, 1)
				f.PathMap[path] = fh
			}
		}
	}
	f.FDlistLock.Unlock()
}

func (f *fdCache) GetFD(path string) int {
	f.FDlistLock.RLock()
	i, ok := f.PathMap[path]
	f.FDlistLock.RUnlock()
	if ok {
		return i
	}
	f.FDlistLock.Lock()
	f.FDcounter++
	newFD := f.FDcounter
	f.PathMap[path] = newFD
	f.FDlistLock.Unlock()
	return newFD
}

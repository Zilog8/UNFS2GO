// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
	"encoding/binary"
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
	fddb = fdCache{FDlistLock: new(sync.RWMutex),
		PathMapA:  make(map[string]int),
		PathMapB:  make(map[int]string),
		FDcounter: 100}
	return 1
}

//export go_accept_mount
func go_accept_mount(addr C.int, path *C.char) C.int {
	a := uint32(addr)
	hostaddress := fmt.Sprintf("%d.%d.%d.%d", byte(a), byte(a>>8), byte(a>>16), byte(a>>24))
	gpath := pathpkg.Clean("/" + C.GoString(path))
	retVal, _ := errTranslator(nil)
	if strings.EqualFold(hostaddress, "127.0.0.1") { //TODO: Make this configurable
		fmt.Println("Host allowed to connect:", hostaddress, "path:", gpath)
	} else {
		fmt.Println("Host not allowed to connect:", hostaddress, "path:", gpath)
		retVal, _ = errTranslator(os.ErrPermission)
	}
	return retVal
}

//export go_readdir_full
func go_readdir_full(dirpath *C.char, names unsafe.Pointer, entries unsafe.Pointer, maxpathlen C.int, maxentries C.int) C.int {
	mp := int(maxpathlen)
	me := int(maxentries)

	nslice := &reflect.SliceHeader{Data: uintptr(names), Len: mp * me, Cap: mp * me}
	newNames := *(*[]byte)(unsafe.Pointer(nslice))

	eslice := &reflect.SliceHeader{Data: uintptr(entries), Len: 24 * me, Cap: 24 * me}
	newEntries := *(*[]byte)(unsafe.Pointer(eslice))

	dirp := pathpkg.Clean("/" + C.GoString(dirpath))

	arr, err := ns.ReadDirectory(dirp)

	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on go_readdir_full of", dirp, "):", err)
	}

	nbCount := 0 //bytes written to names buffer
	ebCount := 0 //bytes written to entry buffer

	namepointer := uint32(uintptr(names))
	entriespointer := uint32(uintptr(entries))

	//Null out the first entry, in case there are none
	for i := 0; i < 24; i++ {
		newEntries[i] = byte(0)
	}

	for i, fi := range arr {

		if i == me { //If we've reached max entries, break
			break
		}

		if i != 0 { //only if this isn't the first entry
			//Put a pointer to this entry as previous entry's Next
			binary.LittleEndian.PutUint32(newEntries[ebCount-4:], entriespointer+uint32(ebCount))
		}

		fp := pathpkg.Clean(dirp + "/" + fi.Name())

		//Put FileID
		fd := fddb.GetFD(fp)
		binary.LittleEndian.PutUint64(newEntries[ebCount:], uint64(fd))
		ebCount += 8

		//Put Pointer to Name
		binary.LittleEndian.PutUint32(newEntries[ebCount:], namepointer+uint32(nbCount))
		ebCount += 4

		//Actually write Name to namebuf
		bytCount := copy(newNames[nbCount:], []byte(fi.Name()))
		newNames[nbCount+bytCount] = byte(0) //null terminate
		nbCount += mp

		//Put Cookie
		binary.LittleEndian.PutUint64(newEntries[ebCount:], uint64(i+1))
		ebCount += 8

		//Null out this pointer to "next" in case we're the last entry
		binary.LittleEndian.PutUint32(newEntries[ebCount:], uint32(0))
		ebCount += 4
	}

	return retVal
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

//export go_exists
func go_exists(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	_, err := ns.Stat(pp)
	retVal, _ := errTranslator(err)
	return retVal
}

//bool is true if error recognized, otherwise false
func errTranslator(err error) (C.int, bool) {
	switch err {
	case nil:
		return C.NFS3_OK, true
	case os.ErrPermission:
		return C.NFS3ERR_ACCES, true
	case os.ErrNotExist:
		return C.NFS3ERR_NOENT, true
	case os.ErrInvalid:
		return C.NFS3ERR_INVAL, true
	case os.ErrExist:
		return C.NFS3ERR_EXIST, true
	default:
		return C.NFS3ERR_IO, false
	}
}

//export go_lstat
func go_lstat(path *C.char, buf *C.go_statstruct) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	fi, err := ns.Stat(pp)
	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on lstat of", pp, "):", err)
	}
	if fi != nil {
		statTranslator(fi, fddb.GetFD(pp), buf)
	}
	return retVal
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

//export go_chmod
func go_chmod(path *C.char, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	err := ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))

	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on chmod of", pp, "(mode =", os.FileMode(int(mode)), "):", err)
	}
	return retVal
}

//export go_truncate
func go_truncate(path *C.char, offset3 C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	off := int64(offset3)
	err := ns.SetAttribute(pp, "size", off)

	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on truncate of", pp, "(size =", off, "):", err)
	}
	return retVal
}

//export go_rename
func go_rename(oldpath *C.char, newpath *C.char) C.int {
	op := pathpkg.Clean("/" + C.GoString(oldpath))
	np := pathpkg.Clean("/" + C.GoString(newpath))

	fi, err := ns.Stat(op)
	if err != nil {
		retVal, _ := errTranslator(err)
		return retVal
	}

	err = ns.Move(op, np)
	if err != nil {
		retVal, _ := errTranslator(err)
		return retVal
	}

	fddb.ReplacePath(op, np, fi.IsDir())

	retVal, _ := errTranslator(nil)
	return retVal
}

//export go_modtime
func go_modtime(path *C.char, modtime C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	mod := time.Unix(int64(modtime), 0)
	err := ns.SetAttribute(pp, "modtime", mod)

	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error setting modtime (", mod, ") on", pp, ":", err)
	}
	return retVal
}

//export go_create
func go_create(pathname *C.char, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(pathname))

	err := ns.CreateFile(pp)
	if err != nil {
		retVal, known := errTranslator(err)
		if !known {
			fmt.Println("Error go_create file at create: ", pp, " due to: ", err)
		}
		return retVal
	}

	err = ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))
	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on go_create file at setmode:", pp, "(mode =", os.FileMode(int(mode)), "):", err)
	}
	return retVal
}

//export go_createover
func go_createover(pathname *C.char, flags C.int, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(pathname))

	fi, err := ns.Stat(pp)
	if err == nil {
		if fi.IsDir() {
			fmt.Println("Error go_createover file: ", pp, " due to: Name of a pre-existing directory")
			return C.NFS3ERR_ISDIR
		}

		err = ns.Remove(pp)
		if err != nil {
			retVal, known := errTranslator(err)
			if !known {
				fmt.Println("Error go_createover file at remove: ", pp, " due to: ", err)
			}
			return retVal
		}
	}

	err = ns.CreateFile(pp)
	if err != nil {
		retVal, known := errTranslator(err)
		if !known {
			fmt.Println("Error go_createover file at create: ", pp, " due to: ", err)
		}
		return retVal
	}

	err = ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))
	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on go_createover file at setmode:", pp, "(mode =", os.FileMode(int(mode)), "):", err)
	}
	return retVal
}

//export go_remove
func go_remove(path *C.char) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	st, err := ns.Stat(pp)
	if err != nil {
		retVal, known := errTranslator(err)
		if !known {
			fmt.Println("Error removing file: ", pp, "\n", err)
		}
		return retVal
	}

	//it seems most shells already check for this, but no harm being extra careful.
	if st.IsDir() {
		fmt.Println("Error removing file: ", pp, "\n Is a directory.")
		return C.NFS3ERR_ISDIR
	}

	err = ns.Remove(pp)
	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error removing file: ", pp, "\n", err)
	}
	return retVal
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

	err = ns.Remove(pp)
	if err != nil {
		//fmt.Println("Error removing directory: ", pp, "\n", err)
		if strings.Contains(err.Error(), "directory not empty") {
			return -2
		}
		return -1
	}
	return 1
}

//export go_mkdir
func go_mkdir(path *C.char, mode C.int) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	err := ns.CreateDirectory(pp)

	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error making directory: ", pp, "\n", err)
	}
	return retVal
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
		retVal, known := errTranslator(err)
		if !known {
			fmt.Println("Error on pwrite of", pp, "(start =", off, "count =", counted, "copied =", copiedBytes, "):", err)
		}
		//because a successful pwrite can return any non-negative number
		//we can't return standard NF3 errors (which are all positive)
		//so we send them as a negative to indicate it's an error,
		//and the recipient will have to negative it again to get the original error.
		return -retVal
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
		retVal, known := errTranslator(err)
		if !known {
			fmt.Println("Error on pread of", pp, "(start =", off, "count =", counted, "copied =", copiedBytes, "):", err)
		}
		//because a successful pread can return any non-negative number
		//we can't return standard NF3 errors (which are all positive)
		//so we send them as a negative to indicate it's an error,
		//and the recipient will have to negative it again to get the original error.
		return -retVal
	}
	return C.int(copiedBytes)
}

//export go_sync
func go_sync(path *C.char, buf *C.go_statstruct) C.int {
	pp := pathpkg.Clean("/" + C.GoString(path))
	fi, err := ns.Stat(pp)
	retVal, known := errTranslator(err)
	if !known {
		fmt.Println("Error on sync of", pp, ":", err)
	}
	if fi != nil {
		statTranslator(fi, fddb.GetFD(pp), buf)
	}
	return retVal
}

type fdCache struct {
	PathMapA   map[string]int
	PathMapB   map[int]string
	FDcounter  int
	FDlistLock *sync.RWMutex
}

func (f *fdCache) GetPath(fd int) (string, error) {
	if fd < 100 {
		return "", os.ErrInvalid
	}
	f.FDlistLock.RLock()
	path, ok := f.PathMapB[fd]
	f.FDlistLock.RUnlock()
	if ok {
		return path, nil
	} else {
		return "", os.ErrInvalid
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

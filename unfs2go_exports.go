// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
	"./afero"
	"./vfs"
	"container/list"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

//Paths are stored here, at index == filedescriptor - 100
var Pathlist *list.List

var fs vfs.NameSpace //virtual filesystem that will house the backends

//export go_init
func go_init() C.int {
	Pathlist = list.New()
	return 0
}

func getPath(fd int) (string, error) {
	if fd < 100 {
		return "", errors.New(fmt.Sprint("Error getPath, filedescriptor too low ", fd))
	}
	i := 100
	e := Pathlist.Front()
	for {
		if e != nil {
			if i == fd {
				return e.Value.(string), nil
			} else {
				i++
				e = e.Next()
			}
		} else {
			return "", errors.New(fmt.Sprint("Error getPath, filedescriptor too high ", fd))
		}
	}
}

func getFD(path string) int {
	i := 100
	//ok, check the filesystem for path existance
	_, err := fs.Stat(path)
	if err != nil {
		//fmt.Println("Error getFD statin': ", path, " ", err)
		return -1
	}

	//check if already cached
	for e := Pathlist.Front(); e != nil; e = e.Next() {
		if strings.EqualFold(path, e.Value.(string)) {
			return i
		}
		i++
	}

	//Add it to cache and return filedescriptor
	Pathlist.PushBack(path)
	return i
}

//export go_readdir_helper
func go_readdir_helper(dirpath *C.char, entryIndex C.int) *C.char {
	pp := C.GoString(dirpath)
	arr, err := fs.ReadDir(pp)
	if err != nil {
		fmt.Println("Error go_readdir_helper ", pp, " ", err)
		return C.CString("")
	}
	index := int(entryIndex)
	if index >= len(arr) {
		fmt.Println("Error go_readdir_helper ", pp, " index out of bounds")
		return C.CString("")
	}

	
	//TODO: Have to add "." and "..", cause they're not showing up
	//and the lack is causing Transmission's delete to infini-loop
	//at function deleteLocalData( tr_torrent * tor, tr_fileFunc func )
	return C.CString(arr[index].Name())
}

//export go_opendir_helper
func go_opendir_helper(path *C.char) C.int {
	//Return -1 if error; else num of entries
	pp := C.GoString(path)
	fi, err := fs.Stat(pp)
	if err != nil || !fi.IsDir() {
		fmt.Println("Error go_opendir_helper ", pp)
		return -1
	}
	arr, err := fs.ReadDir(pp)
	if err != nil {
		fmt.Println("Error go_opendir_helper ", pp, " ", err)
		return -1
	}

	return C.int(len(arr))
}

//export go_open
func go_open(path *C.char, flags C.int) C.int {
	//Return the filedescriptor for this path
	//If file doesn't exist, return -1
	pp := C.GoString(path)
	res := getFD(pp)
	if res > -1 {
		//check if it's actually a file
		fi, err := fs.Stat(pp)
		if err != nil {
			fmt.Println("Error go_open statin': ", path, " ", err)
			return -1
		}
		if fi.IsDir() {
			fmt.Println("Error go_open: ", pp, " is a directory.")
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
	//current architecture makes close unnecessary
	return C.int(0)
}

func getStat(pp string, fd int, buf *C.go_statstruct) C.int {
	fi, err := fs.Stat(pp)
	if err != nil {
		fmt.Println("Error stat: ", pp, " internal stat errored")
		return -1
	}
	buf.st_dev = C.uint32(1)
	buf.st_ino = C.uint64(fd)
	buf.st_gen = C.uint32(fd)
	buf.st_size = C.uint64(fi.Size())
	if fi.IsDir() {
		buf.st_mode = C.short(fi.Mode() | C.S_IFDIR)
	} else {
		buf.st_mode = C.short(fi.Mode() | C.S_IFREG)
	}
	return 0
}

//export go_fstat
func go_fstat(fd C.int, buf *C.go_statstruct) C.int {
	gofd := int(fd)
	pp, err := getPath(gofd)
	if err == nil {
		return getStat(pp, gofd, buf)
	} else {
		return -1
	}
}

//export go_lstat
func go_lstat(path *C.char, buf *C.go_statstruct) C.int {
	pp := C.GoString(path)
	fd := getFD(pp)
	if fd != -1 {
		return getStat(pp, fd, buf)
	} else {
		return -1
	}
}

//export go_truncate
func go_truncate(path *C.char, offset3 C.int) C.int {
	pp := C.GoString(path)
	off := int64(offset3)

	var (
		file afero.File
		step int
		err  error
	)

	for step = 0; err == nil; step++ {
		switch step {
		case 0:
			file, err = fs.Open(pp)
		case 1:
			err = file.Truncate(off)
		case 2:
			err = file.Close()
		case 3:
			return 0
		}
	}
	fmt.Println("Error on truncate (step =", step-1, ") of", pp, "(size =", off, ")", err)
	return -1
}

//export go_rename
func go_rename(oldpath *C.char, newpath *C.char) C.int {
	op := C.GoString(oldpath)
	np := C.GoString(newpath)
	err := fs.Rename(op, np)
	if err != nil {
		fmt.Println("Error on rename", op, " to ", np, " due to ", err)
		return -1
	}
	return 0
}

//export go_utime_helper
func go_utime_helper(path *C.char, actime C.int, modtime C.int) C.int {
	pp := C.GoString(path)
	act := time.Unix(int64(actime), 0)
	mod := time.Unix(int64(modtime), 0)
	err := fs.Chtimes(pp, act, mod)
	if err != nil {
		fmt.Println("Error setting times:", pp, act, mod, err)
		return -1
	}
	return 0
}

//export go_ftruncate
func go_ftruncate(fd C.int, offset3 C.int) C.int {
	gofd := int(fd)
	off := int64(offset3)

	var (
		pp   string
		file afero.File
		step int
		err  error
	)

	for step = 0; err == nil; step++ {
		switch step {
		case 0:
			pp, err = getPath(gofd)
		case 1:
			file, err = fs.OpenFile(pp, os.O_RDWR, 0644)
		case 2:
			err = file.Truncate(off)
		case 3:
			err = file.Close()
		case 4:
			return 0
		}
	}

	fmt.Println("Error on ftruncate (step =", step-1, ") of", pp, "(fd =", gofd,
		") (size =", off, ")", err)
	return -1
}

//export go_open_create
func go_open_create(pathname *C.char, flags C.int, mode C.int) C.int {
	pp := C.GoString(pathname)
	_, err := fs.OpenFile(pp, int(flags), os.FileMode(mode))
	if err != nil {
		fmt.Println("Error open_create file: ", pp, " due to: ", err)
		return -1
	}
	return C.int(getFD(pp))
}

//export go_remove
func go_remove(path *C.char) C.int {
	pp := C.GoString(path)
	st, err := fs.Stat(pp)

	if err != nil {
		fmt.Println("Error removing file: ", pp, "\n", err)
		return -1
	}

	//it seems most shells already check for this, but no harm being extra careful.
	if st.IsDir() {
		fmt.Println("Error removing file: ", pp, "\n Is a directory.")
		return -1
	}

	err = fs.Remove(pp)
	if err != nil {
		fmt.Println("Error removing file: ", pp, "\n", err)
		return -1
	}
	return 0
}

//export go_rmdir_helper
func go_rmdir_helper(path *C.char) C.int {
	pp := C.GoString(path)

	st, err := fs.Stat(pp)
	
	if err != nil {
		fmt.Println("Error removing directory: ", pp, "\n", err)
		return -1
	}

	//it seems most shells already check for this, but no harm being extra careful.
	if !st.IsDir() {
		fmt.Println("Error removing directory: ", pp, "\n Not a directory.")
		return -1
	}

	err = fs.Remove(pp)
	if err != nil {
		fmt.Println("Error removing directory: ", pp, "\n", err)
		if strings.Contains(err.Error(), "directory not empty") {
			return -2
		}
		return -1
	}
	return 0
}

//export go_mkdir
func go_mkdir(path *C.char, mode C.int) C.int {
	pp := C.GoString(path)
	err := fs.Mkdir(pp, os.FileMode(mode))
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
		file        afero.File
		step        int
		copiedBytes int
		err         error
	)

	for step = 0; err == nil; step++ {
		switch step {
		case 0:
			pp, err = getPath(gofd)
		case 1:
			file, err = fs.OpenFile(pp, os.O_RDWR, 0644)
		case 2:
			copiedBytes, err = file.WriteAt(cbuf, off)
		case 3:
			err = file.Close()
		case 4:
			return C.int(copiedBytes)
		}
	}
	fmt.Println("Error on pwrite (step =", step-1, ") of", pp, "(fd =", gofd,
		") (start =", off, " count =", counted, ")", err)
	return -1
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
		file        afero.File
		step        int
		copiedBytes int
		err         error
	)

	for step = 0; err == nil || strings.Contains(err.Error(), "EOF"); step++ {
		switch step {
		case 0:
			pp, err = getPath(gofd)
		case 1:
			file, err = fs.Open(pp)
		case 2:
			copiedBytes, err = file.ReadAt(cbuf, off)
		case 3:
			file.Close() //If we got our bytes, who cares if Close() errors out or not
		case 4:
	return C.int(copiedBytes)
}
	}
	fmt.Println("Error on pread (step =", step-1, ") of", pp, "(fd =", gofd,
		") (start =", off, " count =", counted, ")", err)
	return -1
}

//export go_fsync
func go_fsync(fd C.int) C.int {
	gofd := int(fd)

	var (
		pp   string
		file afero.File
		step int
		err  error
	)

	for step = 0; err == nil; step++ {
		switch step {
		case 0:
			pp, err = getPath(gofd)
		case 1:
			file, err = fs.Open(pp)
		case 2:
			err = file.Sync()
		case 3:
			err = file.Close()
		case 4:
			return 0
		}
	}

	fmt.Println("Error on fsync (step =", step-1, ")", pp, "(fd =", gofd, ")", err)
	return -1
}

// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
	"./minfs"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

var PathMapA map[string]int
var PathMapB map[int]string
var FDcounter int

var ns minfs.MinFS //filesystem being shared

//export go_init
func go_init() C.int {
	PathMapA = make(map[string]int)
	PathMapB = make(map[int]string)
	FDcounter = 100
	return 0
}

func getPath(fd int) (string, error) {
	if fd < 100 {
		return "", errors.New(fmt.Sprint("Error getPath, filedescriptor too low ", fd))
	}
	path := PathMapB[fd]
	if path != "" {
		return path, nil
			} else {
		return "", errors.New(fmt.Sprint("Error getPath, filedescriptor not found ", fd))
	}
}

func getFD(path string) int {
	i := PathMapA[path]
	if i != 0 {
			return i
		}
	FDcounter++
	PathMapA[path] = FDcounter
	PathMapB[FDcounter] = path
	return FDcounter
}

//export go_readdir_helper
func go_readdir_helper(dirpath *C.char, entryIndex C.int) *C.char {

	pp := C.GoString(dirpath)
	index := int(entryIndex)
	arr, err := ns.ReadDirectory(pp, index, 1)

	if err != nil {
		fmt.Println("Error go_readdir_helper path=", pp, "index=", index, "error=", err)
		return C.CString("")
	}

	return C.CString(arr[0].Name())
}

//export go_opendir_helper
func go_opendir_helper(path *C.char) C.int {
	pp := C.GoString(path)
	arr, err := ns.ReadDirectory(pp, 0, -1)

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
	pp := C.GoString(path)
	res := getFD(pp)
	if res > -1 {
		//check if it's actually a file
		fi, err := ns.Stat(pp)
		if err != nil {
			//fmt.Println("Error go_open statin': ", path, " ", err)
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

func getStat(pp string, fd int, buf *C.go_statstruct) C.int {
	fi, err := ns.Stat(pp)
	if err != nil {
		//fmt.Println("Error stat: ", pp, " internal stat errored. File/Directory probably doesn't exist")
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

//export go_fchmod
func go_fchmod(fd C.int, mode C.int) C.int {
	gofd := int(fd)
	pp, err := getPath(gofd)
	if err == nil {
		err = ns.SetAttribute(pp, "mode", os.FileMode(int(mode)))
		if err == nil {
			return 0
		}
	}
	return -1
}

//export go_truncate
func go_truncate(path *C.char, offset3 C.int) C.int {
	pp := C.GoString(path)
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
	op := C.GoString(oldpath)
	np := C.GoString(newpath)
	err := ns.Move(op, np)
	if err != nil {
		fmt.Println("Error on rename", op, " to ", np, " due to ", err)
		return -1
	}
	return 0
}

//export go_utime_helper
func go_utime_helper(path *C.char, actime C.int, modtime C.int) C.int {
	pp := C.GoString(path)
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
	pp, err := getPath(gofd)
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
	pp := C.GoString(pathname)
	err := ns.CreateFile(pp)
	if err != nil {
		fmt.Println("Error open_create file at create: ", pp, " due to: ", err)
		return -1
	}
	return C.int(getFD(pp))
}

//export go_remove
func go_remove(path *C.char) C.int {
	pp := C.GoString(path)
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
	pp := C.GoString(path)

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
	pp := C.GoString(path)
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
			pp, err = getPath(gofd)
	if err != nil {
		fmt.Println("Error on pwrite (getPath of fd =", gofd, ");", err)
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
			pp, err = getPath(gofd)
	if err != nil {
		fmt.Println("Error on pread (getPath of fd =", gofd, ");", err)
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

	_, err := getPath(gofd)
	if err != nil {
		fmt.Println("Error on fsync (fd =", gofd, ");", err)
	return -1
	}
	return 0
}

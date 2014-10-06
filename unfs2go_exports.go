// Due to a limitation in CGO, exports must be in a separate file from func main
package main

//#include "unfs3/daemon.h"
import "C"
import (
	"./vfs"
	"./vfs/zipfs"
	"archive/zip"
	"container/list"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"unsafe"
)

var zipfilepath string

//Paths are stored here, at index == filedescriptor - 100
var Pathlist *list.List

var fs vfs.FileSystem //filesystem, in this example a zip file

//export go_init
func go_init() C.int {
	C.exports_parse(C.CString("/"), C.CString("ro"))
	//Supposedly these strings have to be freed at some point

	Pathlist = list.New()
	rc, _ := zip.OpenReader(zipfilepath)
	fs = zipfs.New(rc, zipfilepath)
	return 0
}

func getPath(fd int) string {
	if fd < 100 {
		fmt.Println("Error getPath, filedescriptor too low ", fd)
		return ""
	}
	i := 100
	e := Pathlist.Front()
	for {
		if e != nil {
			if i == fd {
				return e.Value.(string)
			} else {
				i++
				e = e.Next()
			}
		} else {
			fmt.Println("Error getPath, filedescriptor too large ", fd)
			return ""
		}
	}
}

func getFD(path string) int {
	i := 100
	//check if already exists
	for e := Pathlist.Front(); e != nil; e = e.Next() {
		if strings.EqualFold(path, e.Value.(string)) {
			return i
		}
		i++
	}

	//ok, check the filesystem for path existance
	_, err := fs.Stat(path)
	if err != nil {
		fmt.Println("Error getFD statin': ", path, " ", err)
		return -1
	}

	//Add it to list and return filedescriptor
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
		fmt.Println("Error stat: ", pp, " interal stat errored")
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
	pp := getPath(gofd)
	if len(pp) != 0 {
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

//export go_pread
func go_pread(fd C.int, buf unsafe.Pointer, count C.int, offset C.int) C.int {
	gofd := int(fd)
	pp := getPath(gofd)
	file, err := fs.Open(pp)
	if err != nil {
		fmt.Println("Error on pread ", pp, " (fd = ", gofd, ") ", err)
		return -1
	}
	//seek doesn't work on the zip file,
	//so just dump it all and excise what's needed
	byt, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Error on pread ", pp)
	}

	off := int(offset)
	if len(byt) <= off {
		//file's too small for offset requested
		return -1
	}

	//prepare the provided buffer for use
	slice := &reflect.SliceHeader{Data: uintptr(buf), Len: int(count), Cap: int(count)}
	cbuf := *(*[]byte)(unsafe.Pointer(slice))

	copiedBytes := copy(cbuf, byt[off:])

	file.Close()
	return C.int(copiedBytes)
}

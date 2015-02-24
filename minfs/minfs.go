//minfs tries to provide an abstraction of a filesystem and it's most basic components. It's inspired by the godoc tool's vfs component ( https://go.googlesource.com/tools/+/master/godoc/vfs ), as well as Steve Francia's afero project  ( https://github.com/spf13/afero )
//However, it tries to be A) more featured than vfs.FileSystem while B) simpler to get up and running than afero.Fs.

package minfs

import (
	"errors"
	"fmt"
	"os"
)

type MinFS interface {
	//Create a file. Should throw an error if file already exists.
	CreateFile(name string) error
	//Write to a file. Should throw an error if file doesn't exists.
	WriteFile(name string, b []byte, off int64) (int, error)
	//Read from a file.
	ReadFile(name string, b []byte, off int64) (int, error)
	//Create a directory. Should throw an error if directory already exists.
	CreateDirectory(name string) error
	ReadDirectory(name string) ([]os.FileInfo, error) //No "." or ".." entries allowed
	Move(oldpath string, newpath string) error
	//Whether or not a Remove on a non-empty directory succeeds is implementation dependant
	Remove(name string) error
	Stat(name string) (os.FileInfo, error)
	String() string
	GetAttribute(path string, attribute string) (interface{}, error)
	SetAttribute(path string, attribute string, newvalue interface{}) error
	Close() error
}

// minFile isn't actually used in unfs2go, but it's got some code that might be useful so I'll just leave it here.
type minFile struct {
	fs         MinFS  //Filesystem it's in
	path       string //path being referred to
	flag       int
	perm       os.FileMode
	currOffset int64
}

func (f minFile) WriteAt(b []byte, off int64) (int, error) {
	return f.fs.WriteFile(f.path, b, off)
}

func (f minFile) Stat() (os.FileInfo, error) {
	return f.fs.Stat(f.path)
}

func (f minFile) Read(b []byte) (int, error) {
	n, err := f.fs.ReadFile(f.path, b, f.currOffset)
	f.currOffset += int64(n)
	return n, err
}

func (f minFile) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

func (f minFile) Write(b []byte) (int, error) {
	n, err := f.fs.WriteFile(f.path, b, f.currOffset)
	f.currOffset += int64(n)
	return n, err
}

func (f minFile) Seek(offset int64, whence int) (int64, error) {
	fi, err := f.fs.Stat(f.path)
	if err != nil {
		size := fi.Size()
		switch whence {
		case 0:
			if offset <= size {
				f.currOffset = offset
			} else {
				err = errors.New(fmt.Sprint("Seek Error on", f.path, ": Offset too big 0:", offset, "filesize:", size))
			}
		case 1:
			offset += f.currOffset
			if offset <= size {
				f.currOffset = offset
			} else {
				err = errors.New(fmt.Sprint("Seek Error on", f.path, ": Offset too big 1:", offset, "filesize:", size))
			}
		case 2:
			err = errors.New(fmt.Sprint("Seek Error: Dunno how to do this 2:", offset, "filesize:", size))
		default:
			err = errors.New(fmt.Sprint("Seek Error: Invalid whence:", whence))
		}
	} else {
		err = errors.New(fmt.Sprint("Seek Error on", f.path, ":", err.Error()))
	}
	return f.currOffset, err
}

func (f minFile) ReadAt(b []byte, off int64) (int, error) {
	return f.fs.ReadFile(f.path, b, off)
}

func (f minFile) Close() error {
	if f.fs == nil {
		return errors.New("Close Error: Already closed " + f.path)
	}
	f.fs = nil
	return nil
}

func (f minFile) Sync() error {
	if f.fs == nil {
		return errors.New("Sync Error: Already closed " + f.path)
	}
	f.fs = nil
	return nil
}

func (f minFile) Readdir(count int) ([]os.FileInfo, error) {
	ta, err := f.fs.ReadDirectory(f.path)
	return ta[:count], err
}

func (f minFile) Readdirnames(count int) ([]string, error) {
	ta, err := f.fs.ReadDirectory(f.path)
	retVal := make([]string, count)
	for i, _ := range retVal {
		retVal[i] = ta[i].Name()
	}
	return retVal, err
}

func (f minFile) Name() string {
	return f.path + " in " + f.fs.String()
}

func (f minFile) Truncate(newSize int64) error {
	return f.fs.SetAttribute(f.path, "size", newSize)
}

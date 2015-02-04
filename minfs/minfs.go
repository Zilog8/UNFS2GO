//minfs tries to provide an abstraction of a filesystem and it's most basic components. It's inspired by the godoc tool's vfs component ( https://go.googlesource.com/tools/+/master/godoc/vfs ), as well as Steve Francia's afero project  ( https://github.com/spf13/afero )
//However, it tries to be A) more featured than vfs.FileSystem while B) simpler to get up and running than afero.Fs.

package minfs

import (
	"errors"
	"fmt"
	"os"
)

type MinFS interface {
	CreateFile(name string) error
	WriteFile(name string, b []byte, off int64) (int, error)
	ReadFile(name string, b []byte, off int64) (int, error)
	CreateDirectory(name string) error
	ReadDirectory(name string, off int, maxn int) ([]os.FileInfo, error)
	Move(oldpath string, newpath string) error
	Remove(name string, recursive bool) error
	Stat(name string) (os.FileInfo, error)
	String() string
	GetAttribute(path string, attribute string) (interface{}, error)
	SetAttribute(path string, attribute string, newvalue interface{}) error
}

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
	return f.fs.ReadDirectory(f.path, 0, count)
}

func (f minFile) Readdirnames(count int) ([]string, error) {
	ta, err := f.fs.ReadDirectory(f.path, 0, count)
	retVal := make([]string, len(ta))
	for i, entry := range ta {
		retVal[i] = entry.Name()
	}
	return retVal, err
}

func (f minFile) Name() string {
	return f.path + " in " + f.fs.String()
}

func (f minFile) Truncate(newSize int64) error {
	return f.fs.SetAttribute(f.path, "size", newSize)
}

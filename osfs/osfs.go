package osfs

import (
	"../minfs"
	"os"
	pathpkg "path"
	"path/filepath"
	"syscall"
	"time"
)

type osFS struct {
	realpath string //Real path being exported
	closed   bool   //false normally, true if closed
}

func New(realpath string) (minfs.MinFS, error) {
	_, err := os.Stat(realpath)
	if err == nil {
		return &osFS{realpath, false}, nil
	}
	return nil, err
}

func (f *osFS) Close() error {
	if f.closed {
		return os.ErrInvalid
	}
	f.closed = true
	return nil
}

func (f *osFS) ReadFile(name string, b []byte, off int64) (int, error) {
	if f.closed {
		return 0, os.ErrInvalid
	}
	realname := f.translate(name)
	fh, err := os.Open(realname)
	if err != nil {
		return 0, err
	}
	defer fh.Close()
	return fh.ReadAt(b, off)
}

func (f *osFS) WriteFile(name string, b []byte, off int64) (int, error) {
	if f.closed {
		return 0, os.ErrInvalid
	}
	realname := f.translate(name)
	fh, err := os.OpenFile(realname, os.O_RDWR, 0644)
	if err != nil {
		return 0, err
	}
	defer fh.Close()
	return fh.WriteAt(b, off)
}

func (f *osFS) CreateFile(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	fil, err := os.Create(realname)
	if err != nil {
		return err
	}
	fil.Close()
	return nil
}

func (f *osFS) CreateDirectory(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	return os.Mkdir(realname, 0777)
}

func (f *osFS) Move(oldpath string, newpath string) error {
	if f.closed {
		return os.ErrInvalid
	}
	orname := f.translate(oldpath)
	nrname := f.translate(newpath)
	return os.Rename(orname, nrname)
}

func (f *osFS) Remove(name string) error {
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	return os.Remove(realname)
}

func (f *osFS) ReadDirectory(name string) ([]os.FileInfo, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(name)
	fh, err := os.Open(realname)
	if err != nil {
		return []os.FileInfo{}, err
	}
	defer fh.Close()
	return fh.Readdir(0)
}

func (f *osFS) GetAttribute(path string, attribute string) (interface{}, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(path)
	fi, err := os.Stat(realname)
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
	}
	return nil, os.ErrInvalid
}

func (f *osFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(path)
	switch attribute {
	case "modtime":
		fi, err := os.Stat(realname)
		if err != nil {
			return err
		}
		stat := fi.Sys().(*syscall.Stat_t)
		atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
		return os.Chtimes(realname, atime, newvalue.(time.Time))
	case "mode":
		return os.Chmod(realname, newvalue.(os.FileMode))
	case "size":
		return os.Truncate(realname, newvalue.(int64))
	}
	return os.ErrInvalid
}

func (f *osFS) Stat(name string) (os.FileInfo, error) {
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(name)
	return os.Stat(realname)
}

func (f *osFS) String() string {
	retVal := "os(" + f.realpath + ")"
	if f.closed {
		retVal += "(Closed)"
	}
	return retVal
}

func (f *osFS) translate(path string) string {
	path = pathpkg.Clean("/" + path)
	return pathpkg.Clean(filepath.Join(f.realpath, path))
}

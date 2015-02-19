package osfs

import (
	"../minfs"
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"time"
)

type osFS struct {
	realpath string //Real path being exported
	closed bool //false normally, true if closed
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
    return errors.New("osFS error: Close: Already Closed")
  }
  f.closed = true
  return nil
}

func (f *osFS) ReadFile(name string, b []byte, off int64) (int, error) {
  if f.closed {
    return 0, errors.New("osFS error: ReadFile: Already Closed")
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
    return 0, errors.New("osFS error: WriteFile: Already Closed")
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
    return errors.New("osFS error: CreateFile: Already Closed")
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
    return errors.New("osFS error: CreateDirectory: Already Closed")
  }
	realname := f.translate(name)
	return os.Mkdir(realname, 0777)
}

func (f *osFS) Move(oldpath string, newpath string) error {
	if f.closed {
    return errors.New("osFS error: Move: Already Closed")
  }
	orname := f.translate(oldpath)
	nrname := f.translate(newpath)
	return os.Rename(orname, nrname)
}

func (f *osFS) Remove(name string, recursive bool) error {
	if f.closed {
    return errors.New("osFS error: Remove: Already Closed")
  }
	realname := f.translate(name)
	if recursive {
		return os.RemoveAll(realname)
	}
	return os.Remove(realname)
}

func (f *osFS) ReadDirectory(name string) ([]os.FileInfo, error) {
	if f.closed {
    return nil, errors.New("osFS error: ReadDirectory: Already Closed")
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
    return nil, errors.New("osFS error: GetAttribute: Already Closed")
  }
	realname := f.translate(path)
		fi, err := os.Stat(realname)
		if err != nil {
		return nil, errors.New("GetAttribute Error Stat'n " + path + "(translated as " + realname + "):" + err.Error())
	}
	switch attribute {
	case "modtime":
		return fi.ModTime(), nil
	case "mode":
		return fi.Mode(), nil
	case "size":
		return fi.Size(), nil
	}
	return nil, errors.New("GetAttribute Error: Unsupported attribute " + attribute)
}

func (f *osFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	if f.closed {
    return errors.New("osFS error: SetAttribute: Already Closed")
  }
	realname := f.translate(path)
	switch attribute {
	case "modtime":
		return os.Chtimes(realname, time.Now(), newvalue.(time.Time))
	case "mode":
		return os.Chmod(realname, newvalue.(os.FileMode))
	case "size":
		return os.Truncate(realname, newvalue.(int64))
	case "own":
		tIA := newvalue.([]int)
		return os.Chown(realname, tIA[0], tIA[1])
	}
	return errors.New("SetAttribute Error: Unsupported attribute " + attribute)
}

func (f *osFS) Stat(name string) (os.FileInfo, error) {
	if f.closed {
    return nil, errors.New("osFS error: Stat: Already Closed")
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

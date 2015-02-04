// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package zipfs file provides an implementation of the FileSystem
// interface based on the contents of a .zip file.
//
// Assumptions:
//
// - The file paths stored in the zip file must use a slash ('/') as path
//   separator; and they must be relative (i.e., they must not start with
//   a '/' - this is usually the case if the file was created w/o special
//   options).
// - The zip file system treats the file paths found in the zip internally
//   like absolute paths w/o a leading '/'; i.e., the paths are considered
//   relative to the root of the file system.
// - All path arguments to file system methods must be absolute paths.
package zipfs // import "golang.org/x/tools/godoc/vfs/zipfs"

import (
	"../minfs"
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// zipFI is the zip-file based implementation of FileInfo
type zipFI struct {
	name string    // directory-local name
	file *zip.File // nil for a directory
}

func (fi zipFI) Name() string {
	return fi.name
}

func (fi zipFI) Size() int64 {
	if f := fi.file; f != nil {
		return int64(f.UncompressedSize)
	}
	return 0 // directory
}

func (fi zipFI) ModTime() time.Time {
	if f := fi.file; f != nil {
		return f.ModTime()
	}
	return time.Time{} // directory has no modified time entry
}

func (fi zipFI) Mode() os.FileMode {
	if fi.file == nil {
		// Unix directories typically are executable, hence 555.
		return os.ModeDir | 0555
	}
	return 0444
}

func (fi zipFI) IsDir() bool {
	return fi.file == nil
}

func (fi zipFI) Sys() interface{} {
	return nil
}

// zipFS is the zip-file based implementation of FileSystem
type zipFS struct {
	*zip.ReadCloser
	list zipList
	name string
}

func (fs *zipFS) Close() error {
	fs.list = nil
	return fs.ReadCloser.Close()
}

func zipPath(name string) string {
	name = path.Clean(name)
	if !path.IsAbs(name) {
		panic(fmt.Sprintf("stat: not an absolute path: %s", name))
	}
	return name[1:] // strip leading '/'
}

func (fs *zipFS) stat(abspath string) (int, zipFI, error) {
	i, exact := fs.list.lookup(abspath)
	if i < 0 {
		// abspath has leading '/' stripped - print it explicitly
		return 0, zipFI{}, fmt.Errorf("file not found: /%s", abspath)
	}
	_, name := path.Split(abspath)
	var file *zip.File
	if exact {
		file = fs.list[i] // exact match found - must be a file
	}
	return i, zipFI{name, file}, nil
}

func (fs *zipFS) ReadDir(abspath string) ([]os.FileInfo, error) {
	path := zipPath(abspath)
	i, fi, err := fs.stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("ReadDir: %s is not a directory", abspath)
	}

	var list []os.FileInfo
	dirname := ""
	if len(path) > 0 {
		dirname = path + "/"
	}
	prevname := ""
	for _, e := range fs.list[i:] {
		if !strings.HasPrefix(e.Name, dirname) {
			break // not in the same directory anymore
		}
		name := e.Name[len(dirname):] // local name
		file := e
		if i := strings.IndexRune(name, '/'); i >= 0 {
			// We infer directories from files in subdirectories.
			// If we have x/y, return a directory entry for x.
			name = name[0:i] // keep local directory name only
			file = nil
		}
		// If we have x/y and x/z, don't return two directory entries for x.
		// TODO(gri): It should be possible to do this more efficiently
		// by determining the (fs.list) range of local directory entries
		// (via two binary searches).
		if name != prevname {
			list = append(list, zipFI{name, file})
			prevname = name
		}
	}

	return list, nil
}

func New(name string) (minfs.MinFS, error) {
	rc, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	list := make(zipList, len(rc.File))
	copy(list, rc.File) // sort a copy of rc.File
	sort.Sort(list)
	return &zipFS{rc, list, name}, nil
}

type zipList []*zip.File

// zipList implements sort.Interface
func (z zipList) Len() int           { return len(z) }
func (z zipList) Less(i, j int) bool { return z[i].Name < z[j].Name }
func (z zipList) Swap(i, j int)      { z[i], z[j] = z[j], z[i] }

// lookup returns the smallest index of an entry with an exact match
// for name, or an inexact match starting with name/. If there is no
// such entry, the result is -1, false.
func (z zipList) lookup(name string) (index int, exact bool) {

	if len(name) == 0 { //probably got a lookup on "/"
		return 1, false
	}
	// look for exact match first (name comes before name/ in z)
	i := sort.Search(len(z), func(i int) bool {
		return name <= z[i].Name
	})
	if i >= len(z) {
		return -1, false
	}
	// 0 <= i < len(z)
	if z[i].Name == name {
		return i, true
	}

	// look for inexact match (must be in z[i:], if present)
	z = z[i:]
	name += "/"
	j := sort.Search(len(z), func(i int) bool {
		return name <= z[i].Name
	})
	if j >= len(z) {
		return -1, false
	}
	// 0 <= j < len(z)
	if strings.HasPrefix(z[j].Name, name) {
		return i + j, false
	}

	return -1, false
}

//Stuff to implement minfs.MinFS
func (fs *zipFS) CreateFile(name string) error {
	return os.ErrPermission
}

func (fs *zipFS) WriteFile(name string, b []byte, off int64) (int, error) {
	return 0, os.ErrPermission
}

func (fs *zipFS) ReadFile(name string, b []byte, off64 int64) (int, error) {
	off := int(off64)
	_, fi, err := fs.stat(zipPath(name))
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return 0, fmt.Errorf("Open: %s is a directory", name)
	}
	r, err := fi.file.Open()
	if err != nil {
		return 0, err
	}
	tba := make([]byte, len(b)+off)
	n, rerr := r.Read(tba)

	if n > off {
		n -= off
		copy(b, tba[off:])
	} else {
		n = 0
	}
	return n, rerr
}

func (fs *zipFS) CreateDirectory(name string) error {
	return os.ErrPermission
}

func (fs *zipFS) ReadDirectory(name string, off int, maxn int) ([]os.FileInfo, error) {
	arr, err := fs.ReadDir(name)

	if err != nil {
		return []os.FileInfo{}, err
	}
	if len(arr) < off {
		return []os.FileInfo{}, errors.New(fmt.Sprint("Reading directory", name, "out of range (", off, "of", len(arr), ")"))
	}
	arr = arr[off:]
	if maxn > 0 && len(arr) > maxn {
		arr = arr[:maxn]
	}

	return arr, nil
}

func (fs *zipFS) Move(oldpath string, newpath string) error {
	return os.ErrPermission
}

func (fs *zipFS) Remove(name string, recursive bool) error {
	return os.ErrPermission
}

func (fs *zipFS) String() string {
	return "zipfs " + fs.name
}

func (fs *zipFS) Stat(abspath string) (os.FileInfo, error) {
	_, fi, err := fs.stat(zipPath(abspath))
	return fi, err
}

func (fs *zipFS) GetAttribute(path string, attribute string) (interface{}, error) {
	fi, err := fs.Stat(path)
	if err != nil {
		return nil, errors.New("GetAttribute Error Stat'n " + path + ":" + err.Error())
	}
	switch attribute {
	case "modtime":
		return fi.ModTime(), nil
	case "size":
		return fi.Size(), nil
	}
	return nil, errors.New("GetAttribute Error: Unsupported attribute " + attribute)
}

func (fs *zipFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	return os.ErrPermission
}

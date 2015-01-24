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
package zipfs

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"errors"
	"../../afero"
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

func (fs *zipFS) Name() string {
	return "zip(" + fs.name + ")"
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
		return -1, zipFI{}, fmt.Errorf("file not found: /%s", abspath)
	}
	_, name := path.Split(abspath)
	var file *zip.File
	if exact {
		file = fs.list[i] // exact match found - must be a file
	}
	return i, zipFI{name, file}, nil
}

func (fs *zipFS) Open(abspath string) (afero.File, error) {
	_, fi, err := fs.stat(zipPath(abspath))
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("Open: %s is a directory", abspath)
	}
	r, err := fi.file.Open()
	if err != nil {
		return nil, err
	}
	return &zipFile{fi.file, r, r, nil, nil, nil, nil}, nil
}

type zipFile struct {
	file *zip.File
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
}

func (f *zipFile) nop() error{
	return errors.New("zipfs: Unsupported operation.")
}

func (f *zipFile) Stat() (os.FileInfo, error) {
	return nil, f.nop()
}
func (f *zipFile) 	Readdir(count int) ([]os.FileInfo, error) {
	return nil, f.nop()
}
func (f *zipFile) 	Readdirnames(n int) ([]string, error) {
	return nil, f.nop()
}
func (f *zipFile) 	WriteString(s string) (ret int, err error) {
	return 0, f.nop()
}
func (f *zipFile) 	Truncate(size int64) error {
	return f.nop()
}
func (f *zipFile) 	Name() string {
	return "zipFile"
}
func (f *zipFile) 	Sync() error {
	return nil
}


func (f *zipFile) Seek(offset int64, whence int) (int64, error) {
	if whence == 0 && offset == 0 {
		r, err := f.file.Open()
		if err != nil {
			return 0, err
		}
		f.Close()
		f.Reader = r
		f.Closer = r
		return 0, nil
	}
	
	//TODO: fix this
	//seek doesn't work on the zip file,
	//so just dump it all and excise what's needed
	/* byt, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("Error on pread ", pp)
	}

	off := int(offset)
	if len(byt) <= off {
		//file's too small for offset requested
		return -1
	} */
	
	return 0, fmt.Errorf("unsupported Seek in %s", f.file.Name)
}

func (fs *zipFS) Chmod(abspath string, mode os.FileMode) error {
	return fs.nop()
}

func (fs *zipFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return fs.nop()
}

func (fs *zipFS) Create(name string) (afero.File, error) {
	return nil, fs.nop()
}

func (fs *zipFS) Mkdir(name string, perm os.FileMode) error {
	return fs.nop()
}

func (fs *zipFS) MkdirAll(path string, perm os.FileMode) error {
	return fs.nop()
}

func (fs *zipFS) Remove(name string) error {
	return fs.nop()
}

func (fs *zipFS) RemoveAll(path string) error {
	return fs.nop()
}

func (fs *zipFS) Rename(oldname, newname string) error {
	return fs.nop()
}

func (fs *zipFS) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error){
	return &zipFile{nil, nil, nil, nil, nil, nil, nil}, fs.nop()
}

func (fs *zipFS) nop() error{
	return errors.New("zipfs: Unsupported operation.")
}

func (fs *zipFS) Stat(abspath string) (os.FileInfo, error) {
	_, fi, err := fs.stat(zipPath(abspath))
	return fi, err
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
	dirname := path + "/"
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

func New(rc *zip.ReadCloser, name string) afero.Fs {
	list := make(zipList, len(rc.File))
	copy(list, rc.File) // sort a copy of rc.File
	sort.Sort(list)
	return &zipFS{rc, list, name}
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

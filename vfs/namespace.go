// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vfs

import (
	"../afero"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"time"
)

// Setting debugNS = true will enable debugging prints about
// name space translations.
const debugNS = true

var debugDB map[string]bool

// A NameSpace is a file system made up of other file systems
// mounted at specific locations in the name space.
//
// The representation is a map from mount point locations
// to the list of file systems mounted at that location.  A traditional
// Unix mount table would use a single file system per mount point,
// but we want to be able to mount multiple file systems on a single
// mount point and have the system behave as if the union of those
// file systems were present at the mount point.
// For example, if the OS file system has a Go installation in
// c:\Go and additional Go path trees in  d:\Work1 and d:\Work2, then
// this name space creates the view we want for the godoc server:
//
//	NameSpace{
//		"/": {
//			{old: "/", fs: OS(`c:\Go`), new: "/"},
//		},
//		"/src/pkg": {
//			{old: "/src/pkg", fs: OS(`c:\Go`), new: "/src/pkg"},
//			{old: "/src/pkg", fs: OS(`d:\Work1`), new: "/src"},
//			{old: "/src/pkg", fs: OS(`d:\Work2`), new: "/src"},
//		},
//	}
//
// This is created by executing:
//
//	ns := NameSpace{}
//	ns.Bind("/", OS(`c:\Go`), "/", BindReplace)
//	ns.Bind("/src/pkg", OS(`d:\Work1`), "/src", BindAfter)
//	ns.Bind("/src/pkg", OS(`d:\Work2`), "/src", BindAfter)
//
// A particular mount point entry is a triple (old, fs, new), meaning that to
// operate on a path beginning with old, replace that prefix (old) with new
// and then pass that path to the afero.Fs implementation fs.
//
// Given this name space, a ReadDir of /src/pkg/code will check each prefix
// of the path for a mount point (first /src/pkg/code, then /src/pkg, then /src,
// then /), stopping when it finds one.  For the above example, /src/pkg/code
// will find the mount point at /src/pkg:
//
//	{old: "/src/pkg", fs: OS(`c:\Go`), new: "/src/pkg"},
//	{old: "/src/pkg", fs: OS(`d:\Work1`), new: "/src"},
//	{old: "/src/pkg", fs: OS(`d:\Work2`), new: "/src"},
//
// ReadDir will when execute these three calls and merge the results:
//
//	OS(`c:\Go`).ReadDir("/src/pkg/code")
//	OS(`d:\Work1').ReadDir("/src/code")
//	OS(`d:\Work2').ReadDir("/src/code")
//
// Note that the "/src/pkg" in "/src/pkg/code" has been replaced by
// just "/src" in the final two calls.
//
// OS is itself an implementation of a file system: it implements
// OS(`c:\Go`).ReadDir("/src/pkg/code") as ioutil.ReadDir(`c:\Go\src\pkg\code`).
//
// Because the new path is evaluated by fs (here OS(root)), another way
// to read the mount table is to mentally combine fs+new, so that this table:
//
//	{old: "/src/pkg", fs: OS(`c:\Go`), new: "/src/pkg"},
//	{old: "/src/pkg", fs: OS(`d:\Work1`), new: "/src"},
//	{old: "/src/pkg", fs: OS(`d:\Work2`), new: "/src"},
//
// reads as:
//
//	"/src/pkg" -> c:\Go\src\pkg
//	"/src/pkg" -> d:\Work1\src
//	"/src/pkg" -> d:\Work2\src
//
// An invariant (a redundancy) of the name space representation is that
// ns[mtpt][i].old is always equal to mtpt (in the example, ns["/src/pkg"]'s
// mount table entries always have old == "/src/pkg").  The 'old' field is
// useful to callers, because they receive just a []mountedFS and not any
// other indication of which mount point was found.
//
type NameSpace map[string][]mountedFS

// A mountedFS handles requests for path by replacing
// a prefix 'old' with 'new' and then calling the fs methods.
type mountedFS struct {
	old string
	fs  afero.Fs
	new string
}

// hasPathPrefix returns true if x == y or x == y + "/" + more
func hasPathPrefix(x, y string) bool {
	return x == y || strings.HasPrefix(x, y) && (strings.HasSuffix(y, "/") || strings.HasPrefix(x[len(y):], "/"))
}

// translate translates path for use in m, replacing old with new.
//
// mountedFS{"/src/pkg", fs, "/src"}.translate("/src/pkg/code") == "/src/code".
func (m mountedFS) translate(path string) string {
	path = pathpkg.Clean("/" + path)
	if !hasPathPrefix(path, m.old) {
		panic("translate " + path + " but old=" + m.old)
	}

	result := pathpkg.Join(m.new, path[len(m.old):])

	if debugNS {
		key := path + " tx.to " + fmt.Sprint(m)
		if !debugDB[key] {
			debugDB[key] = true
			//fmt.Printf("tx %s: %v\n", path, result)
		}
	}

	return result
}

func (NameSpace) String() string {
	return "ns"
}

// Fprint writes a text representation of the name space to w.
func (ns NameSpace) Fprint(w io.Writer) {
	fmt.Fprint(w, "name space {\n")
	var all []string
	for mtpt := range ns {
		all = append(all, mtpt)
	}
	sort.Strings(all)
	for _, mtpt := range all {
		fmt.Fprintf(w, "\t%s:\n", mtpt)
		for _, m := range ns[mtpt] {
			fmt.Fprintf(w, "\t\t%s %s\n", m.fs, m.new)
		}
	}
	fmt.Fprint(w, "}\n")
}

// clean returns a cleaned, rooted path for evaluation.
// It canonicalizes the path so that we can use string operations
// to analyze it.
func (NameSpace) clean(path string) string {
	return pathpkg.Clean("/" + path)
}

type BindMode int

const (
	BindReplace BindMode = iota
	BindBefore
	BindAfter
)

// Bind causes references to old to redirect to the path new in newfs.
// If mode is BindReplace, old redirections are discarded.
// If mode is BindBefore, this redirection takes priority over existing ones,
// but earlier ones are still consulted for paths that do not exist in newfs.
// If mode is BindAfter, this redirection happens only after existing ones
// have been tried and failed.
func (ns NameSpace) Bind(old string, newfs afero.Fs, new string, mode BindMode) {
	if debugDB == nil {
		debugDB = map[string]bool{}
	}

	old = ns.clean(old)
	new = ns.clean(new)
	m := mountedFS{old, newfs, new}
	var mtpt []mountedFS
	switch mode {
	case BindReplace:
		mtpt = append(mtpt, m)
	case BindAfter:
		mtpt = append(mtpt, ns.resolve(old)...)
		mtpt = append(mtpt, m)
	case BindBefore:
		mtpt = append(mtpt, m)
		mtpt = append(mtpt, ns.resolve(old)...)
	}

	// Extend m.old, m.new in inherited mount point entries.
	for i := range mtpt {
		m := &mtpt[i]
		if m.old != old {
			if !hasPathPrefix(old, m.old) {
				// This should not happen.  If it does, panic so
				// that we can see the call trace that led to it.
				panic(fmt.Sprintf("invalid Bind: old=%q m={%q, %s, %q}", old, m.old, m.fs.Name(), m.new))
			}
			suffix := old[len(m.old):]
			m.old = pathpkg.Join(m.old, suffix)
			m.new = pathpkg.Join(m.new, suffix)
		}
	}

	ns[old] = mtpt
}

func (ns NameSpace) Remove(name string) error {
	//TODO, verify that it's not a bind point
	mFS := ns.resolve(name)
	reterr := fmt.Sprintln("No suitable backend found for Remove: ", name)
	for i, m := range mFS {
		err := m.fs.Remove(m.translate(name))
		if err == nil {
			return nil
		} else {
			reterr = fmt.Sprintln(reterr, i, "- Failed at ", m, "due to ", err)
		}
	}
	return errors.New(reterr)
}

func (ns NameSpace) Mkdir(name string, perm os.FileMode) error {
	mFS := ns.resolve(name)

	reterr := fmt.Sprintln("No suitable backend found for Mkdir: ", name)
	for i, m := range mFS {
		err := m.fs.Mkdir(m.translate(name), perm)
		if err == nil {
			return nil
		} else {
			reterr = fmt.Sprintln(reterr, i, "- Failed at ", m, "due to ", err)
		}
	}
	return errors.New(reterr)
}

func (ns NameSpace) Rename(oldpath string, newpath string) error {
	oldFSa := ns.resolve(oldpath)

	var oldFS mountedFS
	found := false
	for _, m := range oldFSa {
		f, err := m.fs.Stat(m.translate(oldpath))
		if err == nil {
			if int64(f.Mode().Perm()) >= 384 { // >= octal 600; Read&Write permission
				oldFS = m
				found = true
				break
			}
		}
	}
	if !found {
		return errors.New(fmt.Sprintln("No suitable backend found for Rename old path: ", oldpath))
	}

	return oldFS.fs.Rename(oldFS.translate(oldpath), oldFS.translate(newpath))
	//TODO: Finish this function to handle all the edge cases: One fs to another, etc.

	/*
		newFSa := ns.resolve(newpath)
		if len(newFSa)==0 {
			return errors.New( fmt.Sprintln("No suitable backend found for Rename new path: ", newpath))
		}

		for _, m := range newFSa {

		}
	*/
}

func (ns NameSpace) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	mFS := ns.resolve(name)

	reterr := fmt.Sprintln("No suitable backend found for OpenFile: ", name)
	for i, m := range mFS {
		f, err := m.fs.OpenFile(m.translate(name), flag, perm)
		if err == nil {
			return f, nil
		} else {
			reterr = fmt.Sprintln(reterr, i, "- Failed at ", m, "due to ", err)
		}
	}
	return nil, errors.New(reterr)
}

// resolve resolves a path to the list of mountedFS to use for path.
func (ns NameSpace) resolve(path string) []mountedFS {
	path = ns.clean(path)
	for {
		if m := ns[path]; m != nil {
			if debugNS {
				key := path + " re.to " + fmt.Sprint(m)
				if !debugDB[key] {
					debugDB[key] = true
					//fmt.Printf("resolved %s: %v\n", path, m)
				}
			}
			return m
		}
		if path == "/" {
			break
		}
		path = pathpkg.Dir(path)
	}
	return nil
}

func (ns NameSpace) Chtimes(name string, atime time.Time, mtime time.Time) error {
	FSa := ns.resolve(name)

	for _, m := range FSa {
		trueName := m.translate(name)
		f, err := m.fs.Stat(trueName)
		if err == nil {
			if int64(f.Mode().Perm()) >= 384 { // >= octal 600; Read&Write permission
				return m.fs.Chtimes(trueName, atime, mtime)
			}
		}
	}
	return errors.New(fmt.Sprintln("No suitable backend found for Chtimes: ", name))
}

// Open implements the afero.Fs Open method.
func (ns NameSpace) Open(path string) (afero.File, error) {
	var err error
	for _, m := range ns.resolve(path) {
		r, err1 := m.fs.Open(m.translate(path))
		if err1 == nil {
			return r, nil
		}
		if err == nil {
			err = err1
		}
	}
	if err == nil {
		err = &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return nil, err
}

// stat implements the afero.Fs Stat method.
func (ns NameSpace) stat(path string, f func(afero.Fs, string) (os.FileInfo, error)) (os.FileInfo, error) {
	var err error
	for _, m := range ns.resolve(path) {
		fi, err1 := f(m.fs, m.translate(path))
		if err1 == nil {
			return fi, nil
		}
		if err == nil {
			err = err1
		}
	}
	if err == nil {
		err = &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	return nil, err
}

func (ns NameSpace) Stat(path string) (os.FileInfo, error) {
	return ns.stat(path, afero.Fs.Stat)
}

// dirInfo is a trivial implementation of os.FileInfo for a directory.
type dirInfo string

func (d dirInfo) Name() string       { return string(d) }
func (d dirInfo) Size() int64        { return 0 }
func (d dirInfo) Mode() os.FileMode  { return os.ModeDir | 0555 }
func (d dirInfo) ModTime() time.Time { return startTime }
func (d dirInfo) IsDir() bool        { return true }
func (d dirInfo) Sys() interface{}   { return nil }

var startTime = time.Now()

// ReadDir implements the virtual filesystem ReadDir method.  It's where most of the magic is.
// (The rest is in resolve.)
//
// Logically, ReadDir must return the union of all the directories that are named
// by path.  In order to avoid misinterpreting Go packages, of all the directories
// that contain Go source code, we only include the files from the first,
// but we include subdirectories from all.
//
// ReadDir must also return directory entries needed to reach mount points.
// If the name space looks like the example in the type NameSpace comment,
// but c:\Go does not have a src/pkg subdirectory, we still want to be able
// to find that subdirectory, because we've mounted d:\Work1 and d:\Work2
// there.  So if we don't see "src" in the directory listing for c:\Go, we add an
// entry for it before returning.
//
func (ns NameSpace) ReadDir(path string) ([]os.FileInfo, error) {
	path = ns.clean(path)

	var (
		haveGo   = false
		haveName = map[string]bool{}
		all      []os.FileInfo
		err      error
		first    []os.FileInfo
	)

	for _, m := range ns.resolve(path) {
		dir, err1 := afero.ReadDir(m.translate(path), m.fs)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}

		if dir == nil {
			dir = []os.FileInfo{}
		}

		if first == nil {
			first = dir
		}

		// If we don't yet have Go files in 'all' and this directory
		// has some, add all the files from this directory.
		// Otherwise, only add subdirectories.
		useFiles := false
		if !haveGo {
			for _, d := range dir {
				if strings.HasSuffix(d.Name(), ".go") {
					useFiles = true
					haveGo = true
					break
				}
			}
		}

		for _, d := range dir {
			name := d.Name()
			if (d.IsDir() || useFiles) && !haveName[name] {
				haveName[name] = true
				all = append(all, d)
			}
		}
	}

	// We didn't find any directories containing Go files.
	// If some directory returned successfully, use that.
	if !haveGo {
		for _, d := range first {
			if !haveName[d.Name()] {
				haveName[d.Name()] = true
				all = append(all, d)
			}
		}
	}

	// Built union.  Add any missing directories needed to reach mount points.
	for old := range ns {
		if hasPathPrefix(old, path) && old != path {
			// Find next element after path in old.
			elem := old[len(path):]
			elem = strings.TrimPrefix(elem, "/")
			if i := strings.Index(elem, "/"); i >= 0 {
				elem = elem[:i]
			}
			if !haveName[elem] {
				haveName[elem] = true
				all = append(all, dirInfo(elem))
			}
		}
	}

	if len(all) == 0 {
		return nil, err
	}

	sort.Sort(byName(all))

	return all, nil
}

// byName implements sort.Interface.
type byName []os.FileInfo

func (f byName) Len() int           { return len(f) }
func (f byName) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f byName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

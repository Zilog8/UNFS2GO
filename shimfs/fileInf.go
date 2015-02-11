package shimfs

import (
	//"fmt"
	"os"
	"time"
)

type fileInfo struct {
	fullpath string
	modtime  time.Time
	fi       os.FileInfo  //original fileinfo
	fiage    time.Time    //age of original fileinfo
	chunks   *[]fileChunk //fileChunks, if file

	dirage   time.Time //age of last readdir, if directory
	diritems []string  //items in, if directory
}

func (f *fileInfo) Update(fi os.FileInfo) {
	f.fiage = time.Now()
	f.fi = fi
	if fi.ModTime().After(f.modtime) {
		f.modtime = fi.ModTime()
	}
}

func (f *fileInfo) Name() string {
	return f.fi.Name()
}

func (f *fileInfo) Size() int64 {
	size := f.fi.Size()
	if f.chunks != nil {
		chunks := *f.chunks
		//check if any of the chunks extend beyond the original size
		for _, fc := range chunks {
			max := fc.offset + fc.size
			if max > size {
				size = max
			}
		}
	}
	return size
}

func (f *fileInfo) Mode() os.FileMode {
	return f.fi.Mode()
}

func (f *fileInfo) ModTime() time.Time {
	return f.modtime
}

func (f *fileInfo) IsDir() bool {
	return f.fi.IsDir()
}

func (f *fileInfo) Sys() interface{} {
	return nil
}

type fileChunk struct {
	offset   int64
	size     int64
	isSynced bool
	memback  []byte
	hdback   string
	age      time.Time
}

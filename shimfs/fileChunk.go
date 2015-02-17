package shimfs

import (
	"os"
	"sync"
	"time"
)

type fileChunk struct {
	off       int64
	width     int64
	synced    bool
	mem       []byte
	hd        string
	aged      time.Time
	chunkLock sync.RWMutex //this mutex is managed by the user, namely shimFI
}

func newFileChunk(data swath, isSynced bool) *fileChunk {
	var cL sync.RWMutex
	newArray := make([]byte, len(data.array))
	copy(newArray, data.array)
	return &fileChunk{off: data.off, mem: newArray, width: int64(len(newArray)), aged: time.Now(), synced: isSynced, chunkLock: cL}
}

//func (f *fileChunk) write(newB []byte, offset int64) {
//	endNew := offset + int64(len(newB))
//	endOld := f.off + f.width
//	if f.mem != nil {
//		copy(f.mem[offset-f.off:], newB) //Copy what we can

//		if endNew > endOld { //Append left overs if necessary
//			f.mem = append(f.mem, newB[endOld-offset:]...)
//			f.width = endNew - f.off
//		}
//	} else {
//		file, _ := os.OpenFile(f.hd, os.O_RDWR|os.O_APPEND, 0666)
//		file.WriteAt(newB, offset-f.off)
//		file.Close()
//		if endNew > endOld {
//			f.width = endNew - f.off
//		}
//	}
//	f.aged = time.Now()
//}

func (f *fileChunk) toHD(tempfile string) {
	f.hd = tempfile
	file, _ := os.Create(tempfile)
	file.Write(f.mem)
	file.Close()
	f.mem = nil
}

func (f *fileChunk) read(dest swath) int64 {
	tof := dest.off - f.off
	var retVal int
	if f.mem != nil {
		retVal = copy(dest.array, f.mem[tof:])
	} else {
		file, _ := os.Open(f.hd)
		retVal, _ = file.ReadAt(dest.array, tof)
		file.Close()
	}
	return int64(retVal)
}

func (f *fileChunk) delete() {
	f.width = -1
	if f.mem != nil {
		f.mem = nil
	} else {
		os.Remove(f.hd)
	}
}

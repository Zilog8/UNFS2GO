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
	chunkLock *sync.RWMutex //this mutex is managed by the user, namely shimFI
}

func newFileChunk(data swath, isSynced bool) *fileChunk {
	newArray := make([]byte, len(data.array))
	copy(newArray, data.array)
	return &fileChunk{off: data.off, mem: newArray, width: int64(len(newArray)), aged: time.Now(), synced: isSynced, chunkLock: new(sync.RWMutex)}
}

func (f *fileChunk) toHD(tempfile string) {
	f.hd = tempfile
	file, _ := os.Create(tempfile)
	file.Write(f.mem)
	file.Close()
	f.mem = nil
}

//Try to limit copying if we're sure the recipient won't be modifying the array
func (f *fileChunk) slice() []byte {
  if f.mem != nil {
    return f.mem[0:]
  }
    destarray := make([]byte, f.width)
    file, _ := os.Open(f.hd)
		file.ReadAt(destarray, f.off)
		file.Close()
		return destarray
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

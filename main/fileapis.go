package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// File is the Node and Handle for Files
type File struct {
	path string
	sync.RWMutex
	attr  fuse.Attr
	fs    *FS
	data  []byte
	isMod bool
}

// Attr implements Node interface for files
func (f *File) Attr(ctx context.Context, o *fuse.Attr) error {
	// log.Printf("File.Attr with caller: %s", f.path)
	f.RLock()
	*o = f.attr
	f.RUnlock()
	return nil
}

// Open implements NodeOpener
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// log.Printf("Open with caller: %s", f.path)
	ret := ReadBlobContents(f.path, f.attr.Size)
	f.attr.Size = uint64(len(ret))
	f.data = ret
	f.attr.Mtime = time.Now()
	f.attr.Atime = time.Now()
	f.attr.Crtime = time.Now()
	f.isMod = false
	return f, nil
}

// ReadAll implements HandleReadAller interface
func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	// log.Printf("ReadAll with caller: %s", f.path)
	return f.data, nil
}

// Write implements HandleWriter interface
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// log.Printf("Write with caller: %s", f.path)
	f.Lock()
	defer f.Unlock()
	l := len(req.Data)
	end := int(req.Offset) + l
	if end > len(f.data) {
		delta := end - len(f.data)
		f.data = append(f.data, make([]byte, delta)...)
		f.attr.Size = uint64(len(f.data))
		atomic.AddInt64(&f.fs.size, int64(delta))
	}
	copy(f.data[req.Offset:end], req.Data)
	f.isMod = true
	resp.Size = l
	return nil
}

// Flush implements HandleFlusher interface
func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// log.Printf("Flush with caller: %s", f.path)
	if f.isMod {
		ret := UploadBlobContents(f.path, f.data, false)
		if ret != 0 {
			return fuse.ENODATA
		}
	}
	return nil
}

// Setattr implements NodeSetattrer interface for files
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	// log.Printf("Setattr with caller: %s", f.path)
	f.Lock()

	if req.Valid.Size() {
		delta := int(req.Size) - len(f.data)
		if delta > 0 {
			f.data = append(f.data, make([]byte, delta)...)
		} else {
			f.data = f.data[0:req.Size]
		}
		f.attr.Size = req.Size
		atomic.AddInt64(&f.fs.size, int64(delta))
	}

	if req.Valid.Mode() {
		f.attr.Mode = req.Mode
	}

	if req.Valid.Atime() {
		f.attr.Atime = req.Atime
	}

	if req.Valid.AtimeNow() {
		f.attr.Atime = time.Now()
	}

	if req.Valid.Mtime() {
		f.attr.Mtime = req.Mtime
	}

	if req.Valid.MtimeNow() {
		f.attr.Mtime = time.Now()
	}

	resp.Attr = f.attr

	f.Unlock()
	return nil
}

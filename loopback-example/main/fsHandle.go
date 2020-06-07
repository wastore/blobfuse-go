package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Handle represent an open file or directory
type Handle struct {
	fs        *FS
	reopener  func() (*os.File, error)
	forgetter func()
	f         *os.File
}

var _ fs.HandleFlusher = (*Handle)(nil)

// Flush implements fs.HandleFlusher interface for *Handle
func (h *Handle) Flush(ctx context.Context, req *fuse.FlushRequest) (err error) {
	log.Printf("Handle.Flush() with caller: %s", h.f.Name())
	return h.f.Sync()
}

var _ fs.HandleReadAller = (*Handle)(nil)

// ReadAll implements fs.HandleReadAller interface for *Handle
func (h *Handle) ReadAll(ctx context.Context) (d []byte, err error) {
	log.Printf("Handle.ReadAll() with caller: %s", h.f.Name())
	return ioutil.ReadAll(h.f)
}

var _ fs.HandleReadDirAller = (*Handle)(nil)

// ReadDirAll implements fs.HandleReadDirAller interface for *Handle
// ReadDirAll returns the Directory enteris in the opened directory
func (h *Handle) ReadDirAll(ctx context.Context) (dirs []fuse.Dirent, err error) {
	log.Printf("Handle.ReadDirAll() with caller: %s", h.f.Name())
	// Readdir returns all the file info within the directory f
	fis, err := h.f.Readdir(0)
	if err != nil {
		return nil, identifyError(err)
	}
	// Readdir() reads up the entire dir stream but never resets the pointer.
	// Consequently, when Readdir is called again on the same *File, it gets
	// nothing. As a result, we need to close the file descriptor and re-open it
	// so next call would work.
	if err = h.f.Close(); err != nil {
		return nil, identifyError(err)
	}
	if h.f, err = h.reopener(); err != nil {
		return nil, identifyError(err)
	}

	return getDirentsWithFileInfos(fis), nil
}

var _ fs.HandleReader = (*Handle)(nil)

// Read implements fs.HandleReader interface for *Handle
// Read the contents of file (handle) and sets in response
func (h *Handle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) (err error) {
	log.Printf("Handle.Read() with caller: %s", h.f.Name())
	if _, err = h.f.Seek(req.Offset, 0); err != nil {
		return identifyError(err)
	}
	resp.Data = make([]byte, req.Size)
	n, err := h.f.Read(resp.Data)
	resp.Data = resp.Data[:n]
	return identifyError(err)
}

var _ fs.HandleReleaser = (*Handle)(nil)

// Release implements fs.HandleReleaser interface for *Handle
// Relese Handle by Closing File descriptor
func (h *Handle) Release(ctx context.Context, req *fuse.ReleaseRequest) (err error) {
	log.Printf("Handle.Release() with caller: %s", h.f.Name())
	if h.forgetter != nil {
		h.forgetter()
	}
	return h.f.Close()
}

var _ fs.HandleWriter = (*Handle)(nil)

// Write implements fs.HandleWriter interface for *Handle
// Write the req.Data to file and store amount of data i res.Size
func (h *Handle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) (err error) {
	log.Printf("Handle.Write() with caller: %s", h.f.Name())
	if _, err = h.f.Seek(req.Offset, 0); err != nil {
		return identifyError(err)
	}
	n, err := h.f.Write(req.Data)
	resp.Size = n
	return identifyError(err)
}

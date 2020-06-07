// Memfs implements an in-memory file system.
package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"../connection"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

const (
	// AccountName holds dtorage account name
	AccountName = "anmodblobwthns"
	// AccountKey hold shared access key for account
	AccountKey = "NzDZ6OKACy3/yYR7titw1fU05RxYVcZKhYwpVcaaMJg1Upgl4zYruXEGGztvBYVBt9/+uCgOE2vh41Cpel2CUw=="
	// ContainerName holds name of storage container in the acoount
	ContainerName = "test"
)

func usage() {
	log.Printf("Usage of %s:\n", os.Args[0])
	log.Printf("  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}

	log.Printf("Validating Account Credentials")
	ret := connection.ValidateAccount(AccountName, AccountKey, ContainerName)
	if ret != 0 {
		log.Printf("Error in Validating Credentials")
		os.Exit(1)
	}
	log.Printf("Account Validation Successful, Mounting Directory as FS")

	mountpoint := flag.Arg(0)
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("memfs"),
		fuse.Subtype("memfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Memory FS"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cfg := &fs.Config{}
	srv := fs.New(c, cfg)
	filesys := NewMemFS()

	if err := srv.Serve(filesys); err != nil {
		log.Fatal(err)
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

// MemFS is the File System
type MemFS struct {
	root      *Dir
	nodeID    uint64
	nodeCount uint64
	size      int64
}

// Compile-time interface checks.
var _ fs.FS = (*MemFS)(nil)
var _ fs.FSStatfser = (*MemFS)(nil)

var _ fs.Node = (*Dir)(nil)
var _ fs.NodeCreater = (*Dir)(nil)
var _ fs.NodeMkdirer = (*Dir)(nil)
var _ fs.NodeRemover = (*Dir)(nil)
var _ fs.NodeRenamer = (*Dir)(nil)
var _ fs.NodeStringLookuper = (*Dir)(nil)

var _ fs.HandleReadAller = (*File)(nil)
var _ fs.HandleWriter = (*File)(nil)
var _ fs.Node = (*File)(nil)
var _ fs.NodeOpener = (*File)(nil)
var _ fs.NodeSetattrer = (*File)(nil)

// NewMemFS Returns a file system object
func NewMemFS() *MemFS {
	log.Printf("NewMemFS")
	fs := &MemFS{
		nodeCount: 1,
	}
	fs.root = fs.newDir("", os.ModeDir|0777, 0, time.Now())
	if fs.root.attr.Inode != 1 {
		panic("Root node should have been assigned id 1")
	}
	return fs
}

func (m *MemFS) nextID() uint64 {
	log.Printf("nextID")
	return atomic.AddUint64(&m.nodeID, 1)
}

func (m *MemFS) newDir(path string, mode os.FileMode, size uint64, mtime time.Time) *Dir {
	log.Printf("newDir")
	n := time.Now()
	return &Dir{
		path: path,
		attr: fuse.Attr{
			Inode:  m.nextID(),
			Atime:  n,
			Mtime:  mtime,
			Ctime:  n,
			Crtime: n,
			Mode:   os.ModeDir | mode,
			Size:   size,
		},
		fs:    m,
		nodes: make(map[string]fs.Node),
	}
}

func (m *MemFS) newFile(path string, mode os.FileMode, size uint64, mtime time.Time) *File {
	log.Printf("NewFile")
	n := time.Now()
	return &File{
		path: path,
		attr: fuse.Attr{
			Inode:  m.nextID(),
			Atime:  n,
			Mtime:  mtime,
			Ctime:  n,
			Crtime: n,
			Mode:   mode,
			Size:   size,
		},
		data: make([]byte, 0),
	}
}

func toName(path string) string {
	namearray := strings.Split(path, "/")
	return namearray[len(namearray)-1]
}

// Dir is the Node and Handle for Directory
type Dir struct {
	path string
	sync.RWMutex
	attr   fuse.Attr
	fs     *MemFS
	parent *Dir
	nodes  map[string]fs.Node //Children
}

// File is the Node and Handle for Files
type File struct {
	path string
	sync.RWMutex
	attr fuse.Attr
	fs   *MemFS
	data []byte
}

// Root implements
func (m *MemFS) Root() (fs.Node, error) {
	log.Printf("Root()")
	return m.root, nil
}

// Statfs implements
func (m *MemFS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	log.Printf("Statfs()")
	resp.Blocks = uint64((atomic.LoadInt64(&m.size) + 511) / 512)
	resp.Bsize = 512
	resp.Files = atomic.LoadUint64(&m.nodeCount)
	return nil
}

// Attr implements
func (f *File) Attr(ctx context.Context, o *fuse.Attr) error {
	log.Printf("Dir.Attr")
	f.RLock()
	*o = f.attr
	f.RUnlock()
	return nil
}

// Attr implements
func (d *Dir) Attr(ctx context.Context, o *fuse.Attr) error {
	log.Printf("File.Attr")
	d.RLock()
	*o = d.attr
	d.RUnlock()
	return nil
}

// Lookup implements
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Printf("Lookup")
	d.RLock()
	n, exist := d.nodes[name]
	d.RUnlock()

	if !exist {
		return nil, fuse.ENOENT
	}
	return n, nil
}

// ReadDirAll implements
func (d *Dir) ReadDirAll(ctx context.Context) (dirs []fuse.Dirent, err error) {
	log.Printf("ReadDirAll")
	blobItems := connection.GetBlobItems(d.path)
	for _, blob := range blobItems {
		if len(blob.Metadata) == 1 {
			// Directory
			dir := d.fs.newDir(d.path+blob.Name+"/", 0o775, uint64(*blob.Properties.ContentLength), blob.Properties.LastModified)
			d.nodes[blob.Name] = dir
		}
		if len(blob.Metadata) == 0 {
			file := d.fs.newFile(d.path+blob.Name, 0o666, uint64(*blob.Properties.ContentLength), blob.Properties.LastModified)
			d.nodes[blob.Name] = file
		}
	}
	for name, node := range d.nodes {
		ent := fuse.Dirent{
			Name: name,
		}
		switch n := node.(type) {
		case *File:
			ent.Inode = n.attr.Inode
			ent.Type = fuse.DT_File
		case *Dir:
			ent.Inode = n.attr.Inode
			ent.Type = fuse.DT_Dir
		}
		dirs = append(dirs, ent)
	}
	return dirs, nil
}

// Mkdir implements
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	log.Printf("Mkdir")
	d.Lock()
	defer d.Unlock()

	if _, exists := d.nodes[req.Name]; exists {
		return nil, fuse.EEXIST
	}

	n := d.fs.newDir(d.path+req.Name+"/", req.Mode, 0, time.Now())
	d.nodes[req.Name] = n
	atomic.AddUint64(&d.fs.nodeCount, 1)

	return n, nil
}

// Create implements
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	log.Printf("Create")
	d.Lock()
	defer d.Unlock()

	if _, exists := d.nodes[req.Name]; exists {
		return nil, nil, fuse.EEXIST
	}

	n := d.fs.newFile(req.Name, req.Mode, 0, time.Now())
	n.fs = d.fs
	d.nodes[req.Name] = n
	atomic.AddUint64(&d.fs.nodeCount, 1)

	resp.Attr = n.attr

	return n, n, nil
}

// Rename implements
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	log.Printf("Rename")
	nd := newDir.(*Dir)
	if d.attr.Inode == nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
	} else if d.attr.Inode < nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
		nd.Lock()
		defer nd.Unlock()
	} else {
		nd.Lock()
		defer nd.Unlock()
		d.Lock()
		defer d.Unlock()
	}

	if _, exists := d.nodes[req.OldName]; !exists {
		return fuse.ENOENT
	}

	// Rename can be used as an atomic replace, override an existing file.
	if old, exists := nd.nodes[req.NewName]; exists {
		atomic.AddUint64(&d.fs.nodeCount, ^uint64(0)) // decrement by one
		if oldFile, ok := old.(*File); !ok {
			atomic.AddInt64(&d.fs.size, -int64(oldFile.attr.Size))
		}
	}

	nd.nodes[req.NewName] = d.nodes[req.OldName]
	delete(d.nodes, req.OldName)
	return nil
}

// Remove implements
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	log.Printf("Remove")
	d.Lock()
	defer d.Unlock()

	if n, exists := d.nodes[req.Name]; !exists {
		return fuse.ENOENT
	} else if req.Dir && len(n.(*Dir).nodes) > 0 {
		return fuse.ENODATA
	}

	delete(d.nodes, req.Name)
	atomic.AddUint64(&d.fs.nodeCount, ^uint64(0)) // decrement by one
	return nil
}

// Open implements
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	log.Printf("Open")
	ret := connection.ReadBlobContents(f.path)
	log.Printf("Got from ReadBLOB: %s", ret)
	f.attr.Size = uint64(len(ret))
	f.data = ret
	f.attr.Mtime = time.Now()
	f.attr.Atime = time.Now()
	f.attr.Crtime = time.Now()
	f.attr.Ctime = time.Now()
	return f, nil
}

// ReadAll implements
func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	log.Printf("ReadAll")
	return f.data, nil
}

// Write implements
func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	log.Printf("Write")
	f.Lock()
	l := len(req.Data)
	end := int(req.Offset) + l
	if end > len(f.data) {
		delta := end - len(f.data)
		f.data = append(f.data, make([]byte, delta)...)
		f.attr.Size = uint64(len(f.data))
		atomic.AddInt64(&f.fs.size, int64(delta))
	}
	copy(f.data[req.Offset:end], req.Data)
	resp.Size = l
	f.Unlock()
	return nil
}

// Setattr imp
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	log.Printf("Setattr")
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

// package main

// import (
// 	"context"
// 	"flag"
// 	"fmt"
// 	"log"
// 	"os"

// 	"../connection"
// 	"bazil.org/fuse"
// 	"bazil.org/fuse/fs"
// 	_ "bazil.org/fuse/fs/fstestutil"
// )

// func usage() {
// 	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
// 	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
// 	flag.PrintDefaults()
// }

// const (
// 	// AccountName holds dtorage account name
// 	AccountName = "anmodblobwthns"
// 	// AccountKey hold shared access key for account
// 	AccountKey = "+ptP/muXgWNe9MXlldIwe+DsvqB2jUnRtzL3euBLk2b9qukG0inBGoAdq+UsN0AkfnVDoA4htYuEONMUx9a31w=="
// 	// ContainerName holds name of storage container in the acoount
// 	ContainerName = "test"
// )

// func main() {
// 	log.Printf("Starting File System")
// 	flag.Usage = usage
// 	flag.Parse()

// 	if flag.NArg() != 1 {
// 		usage()
// 		os.Exit(2)
// 	}
// 	mountpoint := flag.Arg(0)

// 	log.Printf("Validating Account Credentials")
// 	ret := connection.ValidateAccount(AccountName, AccountKey, ContainerName)
// 	if ret != 0 {
// 		log.Printf("Error in Validating Credentials")
// 		os.Exit(1)
// 	}
// 	log.Printf("Account Validation Successful, Mounting Directory as FS")

// 	c, err := fuse.Mount(
// 		mountpoint,
// 		fuse.FSName("blobfuse"),
// 		fuse.Subtype("blobfuse-go"),
// 	)
// 	if err != nil {
// 		log.Printf("Error in Mounting filesystem: %v", err)
// 	}
// 	defer c.Close()

// 	err = fs.Serve(c, &FS{rootPath: mountpoint})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

// // FS implements the hello world file system.
// type FS struct {
// 	rootPath string
// }

// var _ fs.FS = (*FS)(nil)

// // Root implements interface FS
// func (f *FS) Root() (fs.Node, error) {
// 	return &Dir{dirPath: f.rootPath}, nil
// }

// // Dir implements both Node and Handle for directories
// type Dir struct {
// 	dirPath string
// }

// var _ fs.Node = (*Dir)(nil)

// // Attr implements interface Node for Directory
// func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
// 	a.Inode = 0
// 	a.Mode = os.ModeDir | 0o755
// 	return nil
// }

// var _ fs.HandleReadDirAller = (*Dir)(nil)

// // ReadDirAll implements interface HandleReadDirAller
// func (d *Dir) ReadDirAll(ctx context.Context) (dirs []fuse.Dirent, err error) {
// 	blobItems := connection.GetBlobItems(d.dirPath)
// 	var inode uint64 = 1
// 	for _, blob := range blobItems {
// 		var tp fuse.DirentType
// 		tp = fuse.DT_File
// 		if len(blob.Metadata) == 1 {
// 			tp = fuse.DT_Dir
// 		}
// 		dirs = append(dirs, fuse.Dirent{
// 			Inode: inode,
// 			Name:  blob.Name,
// 			Type:  tp,
// 		})
// 		inode++
// 	}
// 	return dirs, nil
// }

// // File implements both Node and Handle for files.
// type File struct {
// 	filePath string
// }

// var _ fs.Node = (*File)(nil)

// // Attr implements interface Node for Files
// func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
// 	a.Inode = 0
// 	a.Mode = 0o755
// 	return nil
// }

// var _ fs.HandleReadAller = (*File)(nil)

// // ReadAll return the contents of file
// func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
// 	fmt.Printf("Name of file: %s", f.filePath)
// 	return []byte("hello"), nil

// }

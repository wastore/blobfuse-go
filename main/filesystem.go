// Filesystem implements an in-memory file system to mount azure storage container
package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

var (
	// MountPoint is the Path of Directory where file system will be mounted
	MountPoint string

	// AccountName is the name of Storage Account to Connect with
	AccountName string

	// AccountKey is the Shared Access Key of Storage Account
	AccountKey string

	// ContainerName is the name of container to be mounted
	ContainerName string
)

func usage() {
	log.Printf("Usage of %s:\n", os.Args[0])
	log.Printf("  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	mountpoint := flag.String("mountPath", "", "Path of folder to act as a file system")
	accountname := flag.String("accountName", "", "Name of Storage Account to Mount")
	accountkey := flag.String("accountKey", "", "Shared Access Key for the storage account")
	containername := flag.String("containerName", "", "Name of stroge container to mount")

	flag.Usage = usage
	flag.Parse()

	MountPoint = *mountpoint
	AccountName = *accountname
	AccountKey = *accountkey
	ContainerName = *containername

	log.Printf("Validating Account Credentials")
	ret := ValidateAccount()
	if ret != 0 {
		log.Printf("Error in Validating Credentials")
		os.Exit(1)
	}
	log.Printf("Account Validation Successful, Mounting Directory as FS")

	c, err := fuse.Mount(
		MountPoint,
		fuse.FSName("blobfuse"),
		fuse.Subtype("blobfuse-go"),
		fuse.LocalVolume(),
		fuse.VolumeName(AccountName),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cfg := &fs.Config{}
	srv := fs.New(c, cfg)
	filesys := NewFS()

	if err := srv.Serve(filesys); err != nil {
		log.Fatal(err)
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

// FS is the File System created to serve the calls at user space
type FS struct {
	root      *Dir
	nodeID    uint64
	nodeCount uint64
	size      int64
}

// Compile-time interface checks.
var _ fs.FS = (*FS)(nil)
var _ fs.FSStatfser = (*FS)(nil)

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
var _ fs.HandleFlusher = (*File)(nil)

// NewFS Returns a file system object for making a connection with
func NewFS() *FS {
	log.Printf("NewFS")
	fs := &FS{
		nodeCount: 1,
	}
	fs.root = fs.NewDir("", os.ModeDir|0777, 0, time.Now())
	if fs.root.attr.Inode != 1 {
		panic("Root node should have been assigned id 1")
	}
	return fs
}

// NewDir is used to create a new fsNode which will act as directory
func (m *FS) NewDir(path string, mode os.FileMode, size uint64, mtime time.Time) *Dir {
	// log.Printf("NewDir with path: %s", path)
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

// NewFile is used to create a new fsNode which will act as directory
func (m *FS) NewFile(path string, mode os.FileMode, size uint64, mtime time.Time) *File {
	// log.Printf("NewFile with path: %s", path)
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
		data:  make([]byte, 0),
		isMod: false,
	}
}

func (m *FS) nextID() uint64 {
	// log.Printf("nextID")
	return atomic.AddUint64(&m.nodeID, 1)
}

// utility function to extract name of a node from its path
func toName(path string) string {
	namearray := strings.Split(path, "/")
	return namearray[len(namearray)-1]
}

// Root implements FS interface for File System
// Root is called to obtain the Node for the file system root.
func (m *FS) Root() (fs.Node, error) {
	log.Printf("Root() with caller: %s", m.root.path)
	return m.root, nil
}

// Statfs is called to obtain file system metadata.
func (m *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	log.Printf("Statfs() with caller: %s", m.root.path)
	resp.Blocks = uint64((atomic.LoadInt64(&m.size) + 511) / 512)
	resp.Bsize = 512
	resp.Files = atomic.LoadUint64(&m.nodeCount)
	log.Printf("Statfs returning")
	return nil
}

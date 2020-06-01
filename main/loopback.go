package main

import (
	"context"
	"flag"
	"log"
	"os"
	"sync"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Two command line argument 1st: Loopback Directory 2nd: mountpoint
func main() {
	log.Printf("Starting Loopback FS")
	flag.Usage = usage
	flag.Parse()

	// Argument parsing
	if flag.NArg() != 2 {
		usage()
		os.Exit(2)
	}
	mountpoint := flag.Arg(1)
	loopbackPath := flag.Arg(0)

	// Options can be changed accordingly: Here non empty directory can be mounted and all users are allowed to mount
	c, err := fuse.Mount(mountpoint, fuse.FSName("loopback"), fuse.Subtype("cache"))
	if err != nil {
		log.Printf("Mount Failed due to error: %v", err)
	}

	// Close the collection
	defer c.Close()

	// Starting server to return calls from kernel to userspace
	err = fs.Serve(c, newFS(loopbackPath))
	if err != nil {
		log.Printf("Error in Staring server to serve calls: %v", err)
	}
}

// FS is File System root pointing to loopback directory
type FS struct {
	rootPath string // loopbackPath
	xlock    sync.RWMutex
	xattrs   map[string]map[string][]byte // Nodes Attributes
	nlock    sync.Mutex
	nodes    map[string][]*Node // File System Nodes
}

// Root implements fs.FS interface required for File System to obtain Node for File System
func (f *FS) Root() (n fs.Node, err error) {
	log.Printf("FS.Root()")
	nn := &Node{
		realPath: f.rootPath,
		isDir:    true,
		fs:       f,
	}
	f.newNode(nn)
	return nn, nil
}

// Statfs implements fs.FSStatfser interface for *FS to obtain file system metadata
func (f *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) (err error) {
	log.Printf("FS.Statfs")
	// kernel DS to keep metadata
	var stat syscall.Statfs_t

	// Retrieving metadata from os
	if err := syscall.Statfs(f.rootPath, &stat); err != nil {
		return identifyError(err)
	}

	// Returning metadata by writting it to response
	resp.Blocks = stat.Blocks
	resp.Bfree = stat.Bfree
	resp.Bavail = stat.Bavail
	resp.Files = 0 // TODO
	resp.Ffree = stat.Ffree
	resp.Bsize = uint32(stat.Bsize)
	resp.Namelen = 255 // TODO
	resp.Frsize = 8    // TODO

	return nil
}

// Destroy implements fs.FSDestroyer interface for *FS to shutdown file system
func (f *FS) Destroy() {
	log.Print("FS.Destroy")
}

// FS.GenerateInode of FSInodeGenerator interface is not implmented default implementation of bazil/fuse to generate dynamic inode numbers will be used

// Create a new node n within the directory specified by rp
func (f *FS) newNode(n *Node) {
	log.Printf("FS.newNode() with param: %s", n.realPath)
	rp := n.realPath
	f.nlock.Lock()
	defer f.nlock.Unlock()
	f.nodes[rp] = append(f.nodes[rp], n)
}

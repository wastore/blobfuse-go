package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"../connection"
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// Dir is the Node and Handle for Directory
type Dir struct {
	path string
	sync.RWMutex
	attr   fuse.Attr
	fs     *FS
	parent *Dir
	nodes  map[string]fs.Node //Children
}

// Attr implements Node interface for directories
func (d *Dir) Attr(ctx context.Context, o *fuse.Attr) error {
	log.Printf("Dir.Attr with caller: %s", d.path)
	d.RLock()
	*o = d.attr
	d.RUnlock()
	return nil
}

// Lookup implements NodeStringLookuper interface of Node
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Printf("Lookup with caller: %s", d.path)
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
	log.Printf("ReadDirAll with caller: %s", d.path)
	blobItems := connection.GetBlobItems(d.path)
	for _, blob := range blobItems {
		if len(blob.Metadata) == 1 {
			// Directory
			dir := d.fs.NewDir(d.path+blob.Name+"/", 0o660, uint64(*blob.Properties.ContentLength), blob.Properties.LastModified)
			log.Printf("Updating in : %s", d.path)
			d.nodes[blob.Name] = dir
		}
		if len(blob.Metadata) == 0 {
			file := d.fs.NewFile(d.path+blob.Name, 0o770, uint64(*blob.Properties.ContentLength), blob.Properties.LastModified)
			log.Printf("Updating in : %s", d.path)
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

// Mkdir implements NodeMkdirer interface for Node
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	log.Printf("Mkdir with caller: %s and param: %s", d.path, req.Name)
	d.Lock()
	defer d.Unlock()
	if _, exists := d.nodes[req.Name]; exists {
		return nil, fuse.EEXIST
	}
	n := d.fs.NewDir(d.path+req.Name+"/", 0o775, 0, time.Now())
	d.nodes[req.Name] = n
	atomic.AddUint64(&d.fs.nodeCount, 1)
	// Upload an empty blob with this name
	ret := connection.UploadBlobContents(d.path+req.Name, []byte(""), true)
	if ret != 0 {
		// log.Printf("Error in Creating Empty Blob")
		return nil, fuse.ENODATA
	}
	return n, nil
}

// Create implements NodeCreater interface
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	log.Printf("Create with caller: %s and param: %s", d.path, req.Name)
	d.Lock()
	defer d.Unlock()
	if _, exists := d.nodes[req.Name]; exists {
		return nil, nil, fuse.EEXIST
	}
	n := d.fs.NewFile(d.path+req.Name, 0o666, 0, time.Now())
	n.fs = d.fs
	d.nodes[req.Name] = n
	atomic.AddUint64(&d.fs.nodeCount, 1)
	resp.Attr = n.attr
	// Upload an empty blob with this name
	ret := connection.UploadBlobContents(n.path, []byte(""), false)
	if ret != 0 {
		// log.Printf("Error in Creating Empty Blob")
		return nil, nil, fuse.ENODATA
	}
	return n, n, nil
}

// Rename implements
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	// log.Printf("Rename")
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
	// log.Printf("Remove")
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

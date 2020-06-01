package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Node represent file and directory inside mounted file system
// A Node is the interface required of a file or directory.
type Node struct {
	fs       *FS // file system to which node belongs
	realPath string
	isDir    bool
	lock     sync.RWMutex
	flushers map[*Handle]bool
}

// Attr implements fs.Node interface for *Dir which is required
// Attr fills attr with the standard metadata for the node.
func (n *Node) Attr(ctx context.Context, a *fuse.Attr) (err error) {
	log.Printf("Node.Attr()")
	fi, err := os.Stat(n.realPath)
	if err != nil {
		return identifyError(err)
	}
	fillAttrWithFileInfo(a, fi)
	return nil
}

// GetAttr is implementation of Node.NodeGetattrer interface
// GetAttr is ommitted due to same standard function as Attr

var _ fs.NodeSetattrer = (*Node)(nil)

// Setattr implements fs.NodeSetattrer interface for *Node
// Setattr sets the standard metadata for the receiver req.Valid is a bitmask of what fields are actually being set
func (n *Node) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) (err error) {
	log.Printf("Node.Setattr()")
	// Check if size can be changed or not if can then truncate
	if req.Valid.Size() {
		if err = syscall.Truncate(n.realPath, int64(req.Size)); err != nil {
			return identifyError(err)
		}
	}

	// Check if Mtime can be changed or not
	if req.Valid.Mtime() {
		var tvs [2]syscall.Timeval
		if !req.Valid.Atime() {
			var t = time.Now()
			tvs[0].Sec = int64(t.Unix())
			tvs[0].Usec = int64(t.UnixNano() % time.Second.Nanoseconds() / time.Microsecond.Nanoseconds())
		} else {
			var t = req.Atime
			tvs[0].Sec = int64(t.Unix())
			tvs[0].Usec = int64(t.UnixNano() % time.Second.Nanoseconds() / time.Microsecond.Nanoseconds())
		}
		var t = req.Mtime
		tvs[1].Sec = int64(t.Unix())
		tvs[1].Usec = int64(t.UnixNano() % time.Second.Nanoseconds() / time.Microsecond.Nanoseconds())
	}

	if req.Valid.Handle() {
		log.Printf("%s.Setattr(): unhandled request: req.Valid.Handle() == true", n.realPath)
	}

	if req.Valid.Mode() {
		if err = os.Chmod(n.realPath, req.Mode); err != nil {
			return identifyError(err)
		}
	}

	if req.Valid.Uid() || req.Valid.Gid() {
		if req.Valid.Uid() && req.Valid.Gid() {
			if err = os.Chown(n.realPath, int(req.Uid), int(req.Gid)); err != nil {
				return identifyError(err)
			}
		}
		fi, err := os.Stat(n.realPath)
		if err != nil {
			return identifyError(err)
		}
		s := fi.Sys().(*syscall.Stat_t)
		if req.Valid.Uid() {
			if err = os.Chown(n.realPath, int(req.Uid), int(s.Gid)); err != nil {
				return identifyError(err)
			}
		} else {
			if err = os.Chown(n.realPath, int(s.Uid), int(req.Gid)); err != nil {
				return identifyError(err)
			}
		}
	}

	fi, err := os.Stat(n.realPath)
	if err != nil {
		return identifyError(err)
	}

	fillAttrWithFileInfo(&resp.Attr, fi)

	return nil
}

var _ fs.NodeRemover = (*Node)(nil)

// Remove implements fs.NodeRemover interface for *Node
// Remove the entry with the given name from the reciever(directory)
// Remove can be for both file or directory(Node)
func (n *Node) Remove(ctx context.Context, req *fuse.RemoveRequest) (err error) {
	log.Printf("Node.Remove()")
	name := filepath.Join(n.realPath, req.Name)
	defer func() {
		if err == nil {
			// Removing all xattrs from the node to be removed
			var f = n.fs
			f.xattrs[name] = nil
		}
	}()
	return os.Remove(name)
}

var _ fs.NodeAccesser = (*Node)(nil)

// Access implements fs.NodeAccesser interface for *Node
// Access checks the Mode of the receiver whether the calling context has permission to perform operation
func (n *Node) Access(ctx context.Context, a *fuse.AccessRequest) (err error) {
	log.Printf("Node.Access()")
	fi, err := os.Stat(n.realPath)
	if err != nil {
		return identifyError(err)
	}
	if a.Mask&uint32(fi.Mode()>>6) != a.Mask {
		return fuse.EPERM
	}
	return nil
}

var _ fs.NodeStringLookuper = (*Node)(nil)

// Lookup implements fs.NodeStringLookuper interface for *Node
func (n *Node) Lookup(ctx context.Context, name string) (ret fs.Node, err error) {

	log.Printf("Node.Lookup() with param: name=%s", name)
	// Works only on directory return error if not a directory
	if !n.isDir {
		return nil, fuse.ENOTSUP
	}
	// p stores the path of Node to be lookedup
	p := filepath.Join(n.realPath, name)
	fi, err := os.Stat(p)
	// fi: FileInfo for the node if present otherwise error
	err = identifyError(err)
	if err != nil {
		return nil, identifyError(err)
	}

	var nn *Node
	if fi.IsDir() {
		nn = &Node{realPath: p, isDir: true, fs: n.fs}
	} else {
		nn = &Node{realPath: p, isDir: false, fs: n.fs}
	}

	n.fs.newNode(nn)
	return nn, nil
}

var _ fs.NodeMkdirer = (*Node)(nil)

// Mkdir implements fs.NodeMkdirer interface for *Node
// Mkdir as name says create a Node(directory) using os.Mkdir
func (n *Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (created fs.Node, err error) {
	log.Printf("Node.Mkdir()")
	name := filepath.Join(n.realPath, req.Name)
	if err = os.Mkdir(name, req.Mode); err != nil {
		return nil, identifyError(err)
	}
	nn := &Node{realPath: name, isDir: true, fs: n.fs}
	n.fs.newNode(nn)
	return nn, nil
}

var _ fs.NodeOpener = (*Node)(nil)

// Open implements fs.NodeOpener interface for *Node
func (n *Node) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (h fs.Handle, err error) {
	log.Printf("Node.Open()")
	// Checking flags and permissions
	flags, perm := fuseOpenFlagsToOSFlagsAndPerms(req.Flags)
	// opener contains file descriptor
	opener := func() (*os.File, error) {
		return os.OpenFile(n.realPath, flags, perm)
	}

	f, err := opener()
	if err != nil {
		return nil, identifyError(err)
	}

	handle := &Handle{fs: n.fs, f: f, reopener: opener}
	n.rememberHandle(handle)
	handle.forgetter = func() {
		n.forgetHandle(handle)
	}
	return handle, nil
}

var _ fs.NodeCreater = (*Node)(nil)

// Create implements fs.NodeCreater interface for *Node
func (n *Node) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fsn fs.Node, fsh fs.Handle, err error) {
	log.Printf("Node.Create()")
	flags, _ := fuseOpenFlagsToOSFlagsAndPerms(req.Flags)
	name := filepath.Join(n.realPath, req.Name)
	opener := func() (f *os.File, err error) {
		return os.OpenFile(name, flags, req.Mode)
	}

	f, err := opener()
	if err != nil {
		return nil, nil, identifyError(err)
	}

	h := &Handle{fs: n.fs, f: f, reopener: opener}

	node := &Node{
		realPath: filepath.Join(n.realPath, req.Name),
		isDir:    req.Mode.IsDir(),
		fs:       n.fs,
	}
	node.rememberHandle(h)
	h.forgetter = func() {
		node.forgetHandle(h)
	}
	n.fs.newNode(node)
	return node, h, nil
}

var _ fs.NodeFsyncer = (*Node)(nil)

// Fsync implements fs.NodeFsyncer interface for *Node
func (n *Node) Fsync(ctx context.Context, req *fuse.FsyncRequest) (err error) {
	log.Printf("Node.Fsync()")
	n.lock.RLock()
	defer n.lock.RUnlock()
	for h := range n.flushers {
		return h.f.Sync()
	}
	return fuse.EIO
}

var _ fs.NodeRenamer = (*Node)(nil)

// Rename implements fs.NodeRenamer interface for *Node
func (n *Node) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) (err error) {
	log.Printf("Node.Rename()")
	np := filepath.Join(newDir.(*Node).realPath, req.NewName)
	op := filepath.Join(n.realPath, req.OldName)
	defer func() {
		if err == nil {
			var f = n.fs
			if f.xattrs[op] != nil {
				f.xattrs[np] = f.xattrs[op]
			}
			f.nodes[np] = append(f.nodes[np], f.nodes[op]...)
			delete(f.nodes, op)
			for _, n := range f.nodes[np] {
				n.realPath = np
			}
		}
	}()
	return os.Rename(op, np)
}

var _ fs.NodeGetxattrer = (*Node)(nil)

// Getxattr implements fs.Getxattrer interface for *Node
func (n *Node) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) (err error) {
	log.Printf("Node.Getxattr()")
	n.fs.xlock.RLock()
	defer n.fs.xlock.RUnlock()
	if x := n.fs.xattrs[n.realPath]; x != nil {

		var ok bool
		resp.Xattr, ok = x[req.Name]
		if ok {
			return nil
		}
	}
	return fuse.ENODATA
}

var _ fs.NodeListxattrer = (*Node)(nil)

// Listxattr implements fs.Listxattrer interface for *Node
func (n *Node) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) (err error) {
	log.Printf("Node.Listxattr()")
	n.fs.xlock.RLock()
	defer n.fs.xlock.RUnlock()
	if x := n.fs.xattrs[n.realPath]; x != nil {
		names := make([]string, 0)
		for k := range x {
			names = append(names, k)
		}
		sort.Strings(names)

		if int(req.Position) >= len(names) {
			return nil
		}
		names = names[int(req.Position):]

		s := int(req.Size)
		if s == 0 || s > len(names) {
			s = len(names)
		}
		if s > 0 {
			resp.Append(names[:s]...)
		}
	}

	return nil
}

var _ fs.NodeSetxattrer = (*Node)(nil)

// Setxattr implements fs.Setxattrer interface for *Node
func (n *Node) Setxattr(ctx context.Context, req *fuse.SetxattrRequest) (err error) {
	log.Printf("Node.Setxattr()")
	n.fs.xlock.Lock()
	defer n.fs.xlock.Unlock()
	if n.fs.xattrs[n.realPath] == nil {
		n.fs.xattrs[n.realPath] = make(map[string][]byte)
	}
	buf := make([]byte, len(req.Xattr))
	copy(buf, req.Xattr)

	n.fs.xattrs[n.realPath][req.Name] = buf
	return nil
}

var _ fs.NodeRemovexattrer = (*Node)(nil)

// Removexattr implements fs.Removexattrer interface for *Node
func (n *Node) Removexattr(ctx context.Context, req *fuse.RemovexattrRequest) (err error) {
	log.Printf("Node.Removexattr()")
	n.fs.xlock.Lock()
	defer n.fs.xlock.Unlock()

	name := req.Name

	if x := n.fs.xattrs[n.realPath]; x != nil {
		var ok bool
		_, ok = x[name]
		if ok {
			delete(x, name)
			return nil
		}
	}
	return fuse.ENODATA
}

// Forget implements fs.NodeForgetter interface for *Node
func (n *Node) Forget() {
	var f = n.fs
	log.Printf("FS.forgetNode() with param: %s", n.realPath)
	f.nlock.Lock()
	defer f.nlock.Unlock()
	nodes, ok := f.nodes[n.realPath]
	if !ok {
		return
	}
	// Check if node is there or not and get index of the node in map
	found := -1
	for i, node := range nodes {
		if node == n {
			found = i
			break
		}
	}

	if found > -1 {
		nodes = append(nodes[:found], nodes[found+1:]...)
	}
	// If it is the only node in the map delete the key value for it
	if len(nodes) == 0 {
		delete(f.nodes, n.realPath)
	} else {
		f.nodes[n.realPath] = nodes
	}
}

func (n *Node) rememberHandle(h *Handle) {
	log.Printf("Node.rememberHandle() %v", h.f.Name())
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.flushers == nil {
		n.flushers = make(map[*Handle]bool)
	}
	n.flushers[h] = true
}

func (n *Node) forgetHandle(h *Handle) {
	log.Printf("Node.forgetHandle() %v", h.f.Name())
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.flushers == nil {
		return
	}
	delete(n.flushers, h)
}

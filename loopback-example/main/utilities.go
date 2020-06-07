package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
)

func usage() {
	fmt.Printf("Usage of %s:\n", os.Args[0])
	fmt.Printf("%s ROOT MOUNTPOINT\n", os.Args[0])
}

func identifyError(err error) error {
	switch {
	case os.IsNotExist(err):
		return fuse.ENOENT
	case os.IsExist(err):
		return fuse.EEXIST
	case os.IsPermission(err):
		return fuse.EPERM
	default:
		return err
	}
}

func newFS(loopbackPath string) *FS {
	return &FS{
		rootPath: loopbackPath,
		xattrs:   make(map[string]map[string][]byte),
		nodes:    make(map[string][]*Node),
	}
}

func fuseOpenFlagsToOSFlagsAndPerms(f fuse.OpenFlags) (flag int, perm os.FileMode) {
	flag = int(f & fuse.OpenAccessModeMask)
	if f&fuse.OpenAppend != 0 {
		perm |= os.ModeAppend
	}
	if f&fuse.OpenCreate != 0 {
		flag |= os.O_CREATE
	}
	if f&fuse.OpenDirectory != 0 {
		perm |= os.ModeDir
	}
	if f&fuse.OpenExclusive != 0 {
		perm |= os.ModeExclusive
	}
	if f&fuse.OpenNonblock != 0 {
		// log.Printf("fuse.OpenNonblock is set in OpenFlags but ignored")
	}
	if f&fuse.OpenSync != 0 {
		flag |= os.O_SYNC
	}
	if f&fuse.OpenTruncate != 0 {
		flag |= os.O_TRUNC
	}

	return flag, perm
}

func fillAttrWithFileInfo(a *fuse.Attr, fi os.FileInfo) {
	s := fi.Sys().(*syscall.Stat_t)
	a.Inode = s.Ino
	a.Size = uint64(s.Size)
	a.Blocks = uint64(s.Blocks)
	a.Atime = time.Unix(s.Atim.Unix())
	a.Mtime = time.Unix(s.Mtim.Unix())
	a.Ctime = time.Unix(s.Ctim.Unix())
	a.Mode = fi.Mode()
	a.Nlink = uint32(s.Nlink)
	a.Uid = s.Uid
	a.Gid = s.Gid
	a.BlockSize = uint32(s.Blksize)
}

func getDirentsWithFileInfos(fis []os.FileInfo) (dirs []fuse.Dirent) {
	for _, fi := range fis {
		stat := fi.Sys().(*syscall.Stat_t)
		var tp fuse.DirentType

		switch {
		case fi.IsDir():
			tp = fuse.DT_Dir
		case fi.Mode().IsRegular():
			tp = fuse.DT_File
		default:
			panic("unsupported dirent type")
		}

		dirs = append(dirs, fuse.Dirent{
			Inode: stat.Ino,
			Name:  fi.Name(),
			Type:  tp,
		})
	}

	return dirs
}

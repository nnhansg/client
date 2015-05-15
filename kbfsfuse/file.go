package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/keybase/kbfs/libkbfs"
	"golang.org/x/net/context"
)

type File struct {
	fs.NodeRef

	parent   *Dir
	de       libkbfs.DirEntry
	pathNode libkbfs.PathNode

	exec bool
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(a *fuse.Attr) {
	a.Size = f.de.Size
	a.Mode = 0644
	if f.exec {
		a.Mode |= 0111
	}
}

func (f *File) getPathLocked() libkbfs.Path {
	p := f.parent.getPathLocked()
	p.Path = append(p.Path, f.pathNode)
	return p
}

// Update the PathNode stored here, and in parents.
//
// Caller is responsible for locking.
func (f *File) updatePathLocked(p libkbfs.Path) {
	pNode := p.Path[len(p.Path)-1]
	if f.pathNode.Name != pNode.Name {
		return
	}
	f.pathNode = pNode
	p.Path = p.Path[:len(p.Path)-1]
	f.parent.updatePathLocked(p)
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	f.parent.folder.mu.RLock()
	defer f.parent.folder.mu.RUnlock()

	p := f.getPathLocked()
	n, err := f.parent.folder.fs.config.KBFSOps().Read(p, resp.Data[:cap(resp.Data)], req.Offset)
	resp.Data = resp.Data[:n]
	return err
}

var _ fs.HandleWriter = (*File)(nil)

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.parent.folder.mu.Lock()
	defer f.parent.folder.mu.Unlock()

	p := f.getPathLocked()
	if err := f.parent.folder.fs.config.KBFSOps().Write(p, req.Data, req.Offset); err != nil {
		return err
	}
	resp.Size = len(req.Data)
	if size := uint64(resp.Size); f.de.Size < size {
		f.de.Size = size
	}
	// TODO should we bump up mtime and ctime, too?
	return nil
}

var _ fs.HandleFlusher = (*File)(nil)

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	f.parent.folder.mu.Lock()
	defer f.parent.folder.mu.Unlock()

	p, err := f.parent.folder.fs.config.KBFSOps().Sync(f.getPathLocked())
	if err != nil {
		return err
	}
	f.updatePathLocked(p)

	// Update mtime and such to be what KBFS thinks they should be.
	// bazil.org/fuse does not currently tolerate attribute fetch
	// failing very well, and the kernel would have to flag such nodes
	// invalid, so we try to do failing operations in advance.
	pp := *p.ParentPath()
	dir, err := f.parent.folder.fs.config.KBFSOps().GetDir(pp)
	if err != nil {
		return err
	}
	if de, ok := dir.Children[f.pathNode.Name]; ok {
		f.de = de
	}

	return nil
}

// ntfsbind is the single adapter that binds the userspace verbs (which speak the
// volumeFS / fileEntry contract) to the concrete ntfsfs backend. Keeping the
// ntfsfs coupling in one file means the operator/tool logic and its stub tests
// do not move when the ntfsfs.Entry struct or method set changes — only the thin
// conversion here does. It also runs the shared open pipeline: image.OpenImage
// -> partition.Partitions/SelectNTFSVolume -> ntfsvol geometry + bounded volume
// reader -> ntfsfs over that volume, all O_RDONLY.
package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/Get-Sybers/GoDFIR-toolz/gomount/image"
	"github.com/Get-Sybers/GoDFIR-toolz/gomount/ntfsfs"
	"github.com/Get-Sybers/GoDFIR-toolz/gomount/ntfsvol"
	"github.com/Get-Sybers/GoDFIR-toolz/gomount/partition"
)

// ntfsFS adapts *ntfsfs.FS to the userspace volumeFS contract.
type ntfsFS struct {
	fs *ntfsfs.FS
}

// entryToFile maps a concrete ntfsfs.Entry onto the CLI's fileEntry. This is the
// only place the two representations meet; the MFT id is rendered to a string so
// the CLI stays agnostic to whether the backend keys entries by record number or
// by a composite inode.
func entryToFile(e ntfsfs.Entry) fileEntry {
	return fileEntry{
		Name:    e.Name,
		Path:    e.Path,
		IsDir:   e.IsDir,
		Size:    e.Size,
		MFTID:   strconv.FormatInt(e.MFTID, 10),
		Deleted: e.Deleted,
		Streams: e.Streams,
		Btime:   e.Btime,
		Mtime:   e.Mtime,
		Ctime:   e.Ctime,
		Atime:   e.Atime,
	}
}

func (a ntfsFS) ReadDir(dir string) ([]fileEntry, error) {
	entries, err := a.fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryToFile(e))
	}
	return out, nil
}

func (a ntfsFS) Stat(p string) (fileEntry, error) {
	e, err := a.fs.Stat(p)
	if err != nil {
		return fileEntry{}, err
	}
	return entryToFile(e), nil
}

func (a ntfsFS) Open(p string) (io.ReadCloser, error) { return a.fs.Open(p) }

// Walk bridges ntfsfs.FS.Walk (which delivers ntfsfs.Entry and returns a
// *ntfsfs.WalkErrors for skipped records) to the volumeFS callback and the
// CLI-local walkPartial the stream verb understands.
func (a ntfsFS) Walk(fn func(e fileEntry, open func() (io.ReadCloser, error)) error) error {
	err := a.fs.Walk(func(e ntfsfs.Entry, open func() (io.ReadCloser, error)) error {
		return fn(entryToFile(e), open)
	})
	var we *ntfsfs.WalkErrors
	if errors.As(err, &we) {
		return &walkPartial{count: we.Count, msg: we.Error()}
	}
	return err
}

// openVolumeFS runs the shared pipeline — open the image O_RDONLY, pick the NTFS
// volume, bound it, parse it — and returns the filesystem plus a closer that
// releases the parser and the image. volume is the 1-based CLI value (0 = auto).
func openVolumeFS(imgPath string, volume int) (volumeFS, func() error, error) {
	prefer := -1
	if volume > 0 {
		prefer = volume - 1
	}
	ra, size, closeImage, err := image.OpenImage(imgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open image: %w", err)
	}
	parts, err := partition.Partitions(ra, size)
	if err != nil {
		closeImage()
		return nil, nil, fmt.Errorf("read partitions: %w", err)
	}
	sel, _, err := partition.SelectNTFSVolume(parts, prefer)
	if err != nil {
		closeImage()
		return nil, nil, fmt.Errorf("select NTFS volume: %w", err)
	}
	geom, err := ntfsvol.ReadNTFSGeometry(ra, sel.Offset, sel.Size)
	if err != nil {
		closeImage()
		return nil, nil, fmt.Errorf("read NTFS geometry: %w", err)
	}
	// VolumeReader is already bounded to the volume, so its byte 0 is the boot
	// sector: ntfsfs opens it at offset 0.
	fsys, err := ntfsfs.Open(ntfsvol.VolumeReader(ra, sel.Offset, geom), 0)
	if err != nil {
		closeImage()
		return nil, nil, err
	}
	closer := func() error {
		fsys.Close()
		return closeImage()
	}
	return ntfsFS{fs: fsys}, closer, nil
}

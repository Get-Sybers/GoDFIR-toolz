// Package image opens a forensic disk image read-only and presents it as a
// byte-addressable io.ReaderAt of an exact length, hiding whether the on-disk
// container is a raw dd/img or an EWF/E01 expert-witness set.
//
// A raw image (.raw/.dd/.img, or any file with no EWF signature) is its own
// media: os.Open gives the io.ReaderAt and the file length is the size. An
// EWF/E01 set is decoded by the permissive pure-Go Velocidex/go-ewf reader
// (Apache-2.0), which stitches the segmented, zlib-compressed chunks behind an
// io.ReaderAt and reports the acquired media size. Both paths open O_RDONLY;
// nothing in gomount writes to evidence.
package image

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	ewf "github.com/Velocidex/go-ewf/parser"
)

// EWF first-segment signatures at byte 0. EVF ("EVF\t\r\n\xff\x00") is EnCase
// E01; EVF2 ("EVF2...") is EnCase v2 (.Ex01). LVF ("LVF...") is a *logical*
// evidence file (L01), not a disk image, and is intentionally not opened here.
var (
	sigEVF  = []byte{0x45, 0x56, 0x46, 0x09, 0x0d, 0x0a, 0xff, 0x00}
	sigEVF2 = []byte{0x45, 0x56, 0x46, 0x32, 0x0d, 0x0a, 0x81, 0x00}
)

// ewfSegmentExt matches an EWF segment extension: .E01..E99 / .EAA..EZZ and the
// SMART .s01 family, plus EnCase v2 .Ex01. Logical (.L01/.Lx01) is excluded.
var ewfSegmentExt = regexp.MustCompile(`(?i)^\.(e|s)x?[0-9a-z]{2}$`)

// OpenImage opens the disk image at path read-only and returns a reader over the
// raw media, the exact media size in bytes, and a closer that releases every
// underlying file. The container is auto-detected:
//
//   - RAW/dd/img: ra is the *os.File, size is its on-disk length.
//   - EWF/E01:    every segment (.E01, .E02…, or .Ex01…) is opened and handed to
//     go-ewf; ra is the decoding *ewf.EWFFile, size is its TotalImageSize.
//
// The returned ReaderAt must be read within [0,size); the callers in this tool
// (partition table scan, boot-sector/BPB read, and the FUSE Read passthrough)
// only ever issue bounded, in-range reads.
func OpenImage(path string) (ra io.ReaderAt, size int64, closer func() error, err error) {
	f, err := os.Open(path) // O_RDONLY
	if err != nil {
		return nil, 0, nil, err
	}

	isEWF, err := looksLikeEWF(f, path)
	if err != nil {
		f.Close()
		return nil, 0, nil, err
	}

	if !isEWF {
		st, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, nil, err
		}
		return f, st.Size(), f.Close, nil
	}

	// EWF: the first file we opened is only used for detection; open the whole
	// ordered segment set and let go-ewf reassemble the media.
	f.Close()
	segs, err := ewfSegments(path)
	if err != nil {
		return nil, 0, nil, err
	}
	files := make([]*os.File, 0, len(segs))
	readers := make([]io.ReaderAt, 0, len(segs))
	closeAll := func() error {
		var first error
		for _, h := range files {
			if e := h.Close(); e != nil && first == nil {
				first = e
			}
		}
		return first
	}
	for _, p := range segs {
		h, err := os.Open(p) // O_RDONLY
		if err != nil {
			closeAll()
			return nil, 0, nil, fmt.Errorf("open EWF segment %s: %w", p, err)
		}
		files = append(files, h)
		readers = append(readers, h)
	}

	ef, err := ewf.OpenEWFFile(nil, readers...)
	if err != nil {
		closeAll()
		return nil, 0, nil, fmt.Errorf("parse EWF set: %w", err)
	}
	if ef.TotalImageSize <= 0 {
		closeAll()
		return nil, 0, nil, errors.New("EWF set reports non-positive media size")
	}
	return ef, ef.TotalImageSize, closeAll, nil
}

// looksLikeEWF reports whether path is an EWF/E01 set, by the first-segment
// signature (authoritative) and, failing a readable signature, by extension.
func looksLikeEWF(f io.ReaderAt, path string) (bool, error) {
	var hdr [8]byte
	n, err := f.ReadAt(hdr[:], 0)
	if err != nil && err != io.EOF {
		return false, err
	}
	if n >= 8 && (bytes.Equal(hdr[:], sigEVF) || bytes.Equal(hdr[:], sigEVF2)) {
		return true, nil
	}
	return ewfSegmentExt.MatchString(filepath.Ext(path)), nil
}

// ewfSegments returns every segment file of the EWF set that path belongs to, in
// sorted (segment) order. go-ewf re-sorts by the segment number in each header,
// so lexical order here only needs to enumerate the set completely.
func ewfSegments(path string) ([]string, error) {
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var segs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if strings.TrimSuffix(name, ext) != base {
			continue
		}
		if ewfSegmentExt.MatchString(ext) {
			segs = append(segs, filepath.Join(dir, name))
		}
	}
	if len(segs) == 0 {
		// The path itself did not survive the directory scan (odd name); fall
		// back to just it so a lone, oddly-named segment still opens.
		segs = []string{path}
	}
	sort.Strings(segs)
	return segs, nil
}

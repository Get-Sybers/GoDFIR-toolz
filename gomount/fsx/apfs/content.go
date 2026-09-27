package apfs

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Get-Sybers/GoDFIR-toolz/gomount/lzfse"
)

// File content: a data fork is the file extents of the inode's private
// (data-stream) id, holes reading as zeros. A decmpfs-compressed file
// (bsd_flags UF_COMPRESSED) keeps its bytes in the "com.apple.decmpfs"
// attribute — a 16-byte header, then inline data for the chunk types — or
// in the "com.apple.ResourceFork" attribute's data stream as a table of
// 64 KiB chunks (zlib, LZVN, LZFSE or raw). Each chunk is decoded on
// demand, so a multi-gigabyte compressed file never lands in memory whole.
const (
	decmpfsMagic        = 0x636d7066 // "fpmc" as stored, "cmpf" little-endian
	decmpfsHeader       = 16
	cmpTypeUncompressed = 1
	cmpZlibInline       = 3
	cmpZlibRsrc         = 4
	cmpZeros            = 5
	cmpLZVNInline       = 7
	cmpLZVNRsrc         = 8
	cmpRawInline        = 9
	cmpRawRsrc          = 10
	cmpLZFSEInline      = 11
	cmpLZFSERsrc        = 12
	cmpLZBitmapInline   = 13
	cmpLZBitmapRsrc     = 14

	chunkSize       = 65536
	maxInlineStream = 64 << 20 // an inline attribute held in a data stream is read whole, up to this
	zlibRsrcTable   = 0x104    // where the zlib resource fork's chunk table sits
)

// extentReader serves a data stream's extents as one io.ReaderAt.
type extentReader struct {
	v    *Volume
	exts []extent
	size int64
}

func (r *extentReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("apfs: negative offset")
	}
	total := 0
	bs := r.v.c.blockSize
	for len(p) > 0 {
		if off >= r.size {
			return total, io.EOF
		}
		n := int64(len(p))
		if off+n > r.size {
			n = r.size - off
		}
		// the extent covering off; a gap between extents is a hole
		var e *extent
		var next int64 = r.size
		for i := range r.exts {
			x := &r.exts[i]
			if uint64(off) >= x.logical && uint64(off) < x.logical+x.length {
				e = x
				break
			}
			if x.logical > uint64(off) && int64(x.logical) < next {
				next = int64(x.logical)
			}
		}
		if e == nil {
			if next-off < n {
				n = next - off
			}
			for i := range p[:n] {
				p[i] = 0
			}
		} else {
			within := off - int64(e.logical)
			if int64(e.length)-within < n {
				n = int64(e.length) - within
			}
			if e.paddr == 0 {
				for i := range p[:n] {
					p[i] = 0
				}
			} else {
				got, err := r.v.c.br.ra.ReadAt(p[:n], int64(e.paddr)*bs+within)
				if err != nil && int64(got) < n {
					return total + got, fmt.Errorf("apfs: read extent at block %d: %w", e.paddr, err)
				}
			}
		}
		total += int(n)
		p = p[n:]
		off += n
	}
	return total, nil
}

// streamReader opens a data stream (an inode's private id or an
// attribute's stream id) of a known size.
func (v *Volume) streamReader(streamID uint64, size int64) (io.ReaderAt, error) {
	exts, err := v.extents(streamID)
	if err != nil {
		return nil, err
	}
	return &extentReader{v: v, exts: exts, size: size}, nil
}

// readStream reads a whole (bounded) data stream into memory.
func (v *Volume) readStream(streamID uint64, size, limit int64) ([]byte, error) {
	if size < 0 || size > limit {
		return nil, fmt.Errorf("apfs: stream %d of %d bytes exceeds the %d-byte limit", streamID, size, limit)
	}
	ra, err := v.streamReader(streamID, size)
	if err != nil {
		return nil, err
	}
	b := make([]byte, size)
	if _, err := ra.ReadAt(b, 0); err != nil && err != io.EOF {
		return nil, err
	}
	return b, nil
}

// contentReader returns the file's bytes as an io.ReaderAt and their size,
// decompressing a decmpfs file transparently.
func (v *Volume) contentReader(in *inode) (io.ReaderAt, int64, error) {
	if in.bsdFlags&ufCompressed != 0 {
		if ra, size, err := v.decmpfsReader(in); err == nil {
			return ra, size, nil
		} else if x, xerr := v.xattr(in.id, xattrDecmpfs); xerr != nil {
			return nil, 0, xerr // the attribute could not even be read
		} else if x != nil {
			return nil, 0, err // it IS compressed and we could not decode it: say so
		}
		// the flag without the attribute: fall through to the data fork
	}
	if v.info.Encrypted {
		return nil, 0, fmt.Errorf("apfs: volume %q is encrypted (FileVault); file content is not readable", v.info.Name)
	}
	ra, err := v.streamReader(in.privateID, in.size)
	if err != nil {
		return nil, 0, err
	}
	return ra, in.size, nil
}

// decmpfsAttr returns the decmpfs attribute's bytes (embedded or streamed).
func (v *Volume) decmpfsAttr(in *inode) ([]byte, error) {
	x, err := v.xattr(in.id, xattrDecmpfs)
	if err != nil {
		return nil, err
	}
	if x == nil {
		return nil, fmt.Errorf("apfs: inode %d is flagged compressed but has no decmpfs attribute", in.id)
	}
	if x.data != nil {
		return x.data, nil
	}
	return v.readStream(x.streamID, x.size, maxInlineStream)
}

// compressedSize reads the uncompressed size off the decmpfs header.
func (v *Volume) compressedSize(in *inode) (int64, bool) {
	b, err := v.decmpfsAttr(in)
	if err != nil || len(b) < decmpfsHeader || binary.LittleEndian.Uint32(b) != decmpfsMagic {
		return 0, false
	}
	return int64(binary.LittleEndian.Uint64(b[8:])), true
}

func (v *Volume) decmpfsReader(in *inode) (io.ReaderAt, int64, error) {
	hdr, err := v.decmpfsAttr(in)
	if err != nil {
		return nil, 0, err
	}
	if len(hdr) < decmpfsHeader || binary.LittleEndian.Uint32(hdr) != decmpfsMagic {
		return nil, 0, fmt.Errorf("apfs: inode %d: decmpfs attribute without the cmpf header", in.id)
	}
	typ := binary.LittleEndian.Uint32(hdr[4:])
	size := int64(binary.LittleEndian.Uint64(hdr[8:]))
	if size < 0 {
		return nil, 0, fmt.Errorf("apfs: inode %d: negative decmpfs size", in.id)
	}
	inline := hdr[decmpfsHeader:]
	switch typ {
	case cmpTypeUncompressed, cmpRawInline:
		if int64(len(inline)) > size {
			inline = inline[:size]
		}
		return bytes.NewReader(inline), int64(len(inline)), nil
	case cmpZeros:
		return &extentReader{v: v, size: size}, size, nil
	case cmpZlibInline, cmpLZVNInline, cmpLZFSEInline:
		out, err := decodeChunk(typ, inline, size)
		if err != nil {
			return nil, 0, fmt.Errorf("apfs: inode %d: %w", in.id, err)
		}
		return bytes.NewReader(out), int64(len(out)), nil
	case cmpZlibRsrc, cmpLZVNRsrc, cmpRawRsrc, cmpLZFSERsrc:
		x, err := v.xattr(in.id, xattrRsrc)
		if err != nil {
			return nil, 0, err
		}
		if x == nil {
			return nil, 0, fmt.Errorf("apfs: inode %d: compressed into a resource fork it does not have", in.id)
		}
		var fork io.ReaderAt
		var forkSize int64
		if x.data != nil {
			fork, forkSize = bytes.NewReader(x.data), int64(len(x.data))
		} else {
			if fork, err = v.streamReader(x.streamID, x.size); err != nil {
				return nil, 0, err
			}
			forkSize = x.size
		}
		cr, err := newChunkReader(typ, fork, forkSize, size)
		if err != nil {
			return nil, 0, fmt.Errorf("apfs: inode %d: %w", in.id, err)
		}
		return cr, size, nil
	case cmpLZBitmapInline, cmpLZBitmapRsrc:
		return nil, 0, fmt.Errorf("apfs: inode %d: LZBITMAP compression is not supported", in.id)
	}
	return nil, 0, fmt.Errorf("apfs: inode %d: unknown decmpfs type %d", in.id, typ)
}

// decodeChunk decodes one compressed chunk of the given decmpfs family
// into at most size bytes. Each family marks an uncompressed chunk with a
// leading byte.
func decodeChunk(typ uint32, b []byte, size int64) ([]byte, error) {
	if size > chunkSize*16 && (typ == cmpLZVNInline || typ == cmpLZFSEInline || typ == cmpZlibInline) {
		// an inline chunk is a small file; a huge claimed size is corruption
		return nil, fmt.Errorf("decmpfs: inline chunk claims %d bytes", size)
	}
	if len(b) == 0 {
		return []byte{}, nil
	}
	switch typ {
	case cmpZlibInline, cmpZlibRsrc:
		if b[0]&0x0f == 0x0f {
			return clip(b[1:], size), nil
		}
		zr, err := zlib.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("decmpfs zlib: %w", err)
		}
		defer zr.Close()
		out := make([]byte, size)
		n, err := io.ReadFull(zr, out)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return nil, fmt.Errorf("decmpfs zlib: %w", err)
		}
		return out[:n], nil
	case cmpLZVNInline, cmpLZVNRsrc:
		if b[0] == 0x06 {
			return clip(b[1:], size), nil
		}
		out := make([]byte, size)
		n, err := lzfse.DecodeLZVN(out, b)
		if err != nil {
			return nil, fmt.Errorf("decmpfs lzvn: %w", err)
		}
		return out[:n], nil
	case cmpLZFSEInline, cmpLZFSERsrc:
		if b[0] == 0xff {
			return clip(b[1:], size), nil
		}
		out := make([]byte, size)
		n, err := lzfse.Decode(out, b)
		if err != nil {
			return nil, fmt.Errorf("decmpfs lzfse: %w", err)
		}
		return out[:n], nil
	case cmpRawInline, cmpRawRsrc:
		if b[0] == 0xcc {
			return clip(b[1:], size), nil
		}
		return clip(b, size), nil
	}
	return nil, fmt.Errorf("decmpfs: type %d has no chunk decoder", typ)
}

func clip(b []byte, size int64) []byte {
	if size < 0 {
		return b[:0]
	}
	if int64(len(b)) > size {
		return b[:size]
	}
	return b
}

// chunkReader decodes a resource fork's 64 KiB chunks on demand.
type chunkReader struct {
	typ    uint32
	fork   io.ReaderAt
	size   int64      // uncompressed file size
	chunks [][2]int64 // (offset, length) of each compressed chunk in the fork
	last   int
	cache  []byte
}

func newChunkReader(typ uint32, fork io.ReaderAt, forkSize, size int64) (*chunkReader, error) {
	cr := &chunkReader{typ: typ, fork: fork, size: size, last: -1}
	le := binary.LittleEndian
	nChunks := int((size + chunkSize - 1) / chunkSize)
	switch typ {
	case cmpZlibRsrc:
		// the resource fork: a 256-byte big-endian header, then at 0x100 the
		// data length and at 0x104 the chunk table (count, then offset/size
		// pairs relative to the table), little-endian
		hdr := make([]byte, 8)
		if _, err := fork.ReadAt(hdr, zlibRsrcTable-4); err != nil {
			return nil, fmt.Errorf("decmpfs zlib fork: read table: %w", err)
		}
		n := int(le.Uint32(hdr[4:]))
		if n <= 0 || n > 1<<20 || n < nChunks {
			return nil, fmt.Errorf("decmpfs zlib fork: %d chunks for %d bytes", n, size)
		}
		tbl := make([]byte, 8*n)
		if _, err := fork.ReadAt(tbl, zlibRsrcTable+4); err != nil && err != io.EOF {
			return nil, fmt.Errorf("decmpfs zlib fork: read table: %w", err)
		}
		for i := 0; i < n; i++ {
			off := int64(le.Uint32(tbl[8*i:])) + zlibRsrcTable
			ln := int64(le.Uint32(tbl[8*i+4:]))
			if off < 0 || ln < 0 || off+ln > forkSize {
				return nil, fmt.Errorf("decmpfs zlib fork: chunk %d out of range", i)
			}
			cr.chunks = append(cr.chunks, [2]int64{off, ln})
		}
	default:
		// LZVN/LZFSE/raw forks: a table of little-endian offsets, the first
		// being the table's own size; chunk i spans [off[i], off[i+1])
		first := make([]byte, 4)
		if _, err := fork.ReadAt(first, 0); err != nil {
			return nil, fmt.Errorf("decmpfs fork: read table: %w", err)
		}
		tblLen := int64(le.Uint32(first))
		n := int(tblLen/4) - 1
		if tblLen < 8 || tblLen > forkSize || n <= 0 || n > 1<<20 || n < nChunks {
			return nil, fmt.Errorf("decmpfs fork: %d chunks for %d bytes", n, size)
		}
		tbl := make([]byte, tblLen)
		if _, err := fork.ReadAt(tbl, 0); err != nil && err != io.EOF {
			return nil, fmt.Errorf("decmpfs fork: read table: %w", err)
		}
		for i := 0; i < n; i++ {
			off := int64(le.Uint32(tbl[4*i:]))
			end := int64(le.Uint32(tbl[4*i+4:]))
			if off < 0 || end < off || end > forkSize {
				return nil, fmt.Errorf("decmpfs fork: chunk %d out of range", i)
			}
			cr.chunks = append(cr.chunks, [2]int64{off, end - off})
		}
	}
	return cr, nil
}

func (cr *chunkReader) chunk(i int) ([]byte, error) {
	if i == cr.last {
		return cr.cache, nil
	}
	if i < 0 || i >= len(cr.chunks) {
		return nil, fmt.Errorf("decmpfs: chunk %d of %d", i, len(cr.chunks))
	}
	want := int64(chunkSize)
	if rem := cr.size - int64(i)*chunkSize; rem < want {
		want = rem
	}
	raw := make([]byte, cr.chunks[i][1])
	if _, err := cr.fork.ReadAt(raw, cr.chunks[i][0]); err != nil && err != io.EOF {
		return nil, fmt.Errorf("decmpfs: read chunk %d: %w", i, err)
	}
	out, err := decodeChunk(cr.typ, raw, want)
	if err != nil {
		return nil, fmt.Errorf("chunk %d: %w", i, err)
	}
	cr.last, cr.cache = i, out
	return out, nil
}

func (cr *chunkReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("apfs: negative offset")
	}
	total := 0
	for len(p) > 0 {
		if off >= cr.size {
			return total, io.EOF
		}
		i := int(off / chunkSize)
		within := int(off % chunkSize)
		c, err := cr.chunk(i)
		if err != nil {
			return total, err
		}
		if within >= len(c) {
			// a chunk decoded short: the rest of it is unrecoverable
			return total, fmt.Errorf("decmpfs: chunk %d decoded to %d bytes, %d wanted", i, len(c), within+1)
		}
		n := copy(p, c[within:])
		total += n
		p = p[n:]
		off += int64(n)
	}
	return total, nil
}

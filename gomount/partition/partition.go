// Package partition finds the partitions on a disk image by parsing the MBR and
// GPT structures directly over an io.ReaderAt (clean-room, no third-party disk
// library) and flags the ones that carry an NTFS volume.
//
// The single subtlety that drives the ordering below: an NTFS boot sector ends
// in the same 0x55 0xAA signature as an MBR, so a partitionless "superfloppy"
// raw NTFS file looks like it has an MBR. It does not — its bytes at +3 spell
// the OEM id "NTFS    " which MBR bootstrap code never does. So a whole-disk
// NTFS boot sector at offset 0 is recognised first and returned as one synthetic
// partition spanning the entire image.
package partition

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Partition is one volume located on the image, described in absolute byte
// offsets into the image ReaderAt.
type Partition struct {
	Offset     int64  // absolute byte offset of the volume's first sector
	Size       int64  // volume length in bytes (from the table; superfloppy = whole image)
	TypeName   string // human label from the MBR type byte or GPT type GUID
	LikelyNTFS bool   // the boot sector at Offset carries the NTFS OEM id + 0xAA55
}

// sector sizes used for LBA->byte conversion. MBR partition entries address in
// 512-byte units by convention regardless of the physical sector size; GPT uses
// the disk's logical block size, which we detect from where "EFI PART" sits.
const (
	mbrLBASize   = 512
	mbrSigOffset = 510
	mbrPartTable = 446
	mbrEntrySize = 16
)

var (
	ntfsOEM  = []byte("NTFS    ") // 8 bytes at boot-sector +3
	gptMagic = []byte("EFI PART")
)

// Partitions parses the partition table on the image and returns its volumes.
//
//   - A whole-disk NTFS boot sector at offset 0 (superfloppy / partitionless)
//     yields a single synthetic Partition{Offset:0, Size:size}.
//   - Otherwise a GPT (protective-MBR aware, header at LBA1) is parsed if present,
//     else the MBR primary + extended (EBR chain) partitions.
//
// A nil error with an empty slice means a table was read but held no usable
// partition; the caller decides how to report "no NTFS volume found".
func Partitions(ra io.ReaderAt, size int64) ([]Partition, error) {
	if size < mbrLBASize {
		return nil, fmt.Errorf("image too small (%d bytes) to hold a boot sector", size)
	}

	// 1. Superfloppy: a raw NTFS volume with no partition table. Checked first
	// because its trailing 0x55AA would otherwise be mistaken for an MBR.
	if looksLikeNTFS(ra, 0) {
		return []Partition{{
			Offset:     0,
			Size:       size,
			TypeName:   "NTFS (superfloppy/partitionless)",
			LikelyNTFS: true,
		}}, nil
	}

	// Read the first sector once for the MBR signature and entries.
	var mbr [512]byte
	if _, err := io.ReadFull(io.NewSectionReader(ra, 0, 512), mbr[:]); err != nil {
		return nil, fmt.Errorf("read MBR: %w", err)
	}
	hasMBRSig := mbr[mbrSigOffset] == 0x55 && mbr[mbrSigOffset+1] == 0xAA

	// 2. GPT: a protective/hybrid MBR (an 0xEE entry) or an "EFI PART" header at
	// LBA1 (512- or 4096-byte logical sectors).
	if hasMBRSig {
		if gpt, sectorSize, ok := gptHeader(ra, size); ok {
			return parseGPT(ra, size, gpt, sectorSize)
		}
	}

	// 3. Classic MBR: four primaries plus any extended (EBR) chain.
	if hasMBRSig {
		return parseMBR(ra, size, mbr[:]), nil
	}

	return nil, nil
}

// looksLikeNTFS reports whether the sector at offset is an NTFS boot sector: the
// OEM id "NTFS    " at +3 and the 0xAA55 magic at +510.
func looksLikeNTFS(ra io.ReaderAt, offset int64) bool {
	var b [512]byte
	if _, err := io.ReadFull(io.NewSectionReader(ra, offset, 512), b[:]); err != nil {
		return false
	}
	if b[mbrSigOffset] != 0x55 || b[mbrSigOffset+1] != 0xAA {
		return false
	}
	return string(b[3:11]) == string(ntfsOEM)
}

// ---- MBR --------------------------------------------------------------------

// mbrTypeName labels the common MBR partition types. Only the label is
// cosmetic; LikelyNTFS is decided by the boot sector, not this byte.
func mbrTypeName(t byte) string {
	switch t {
	case 0x07:
		return "IFS/NTFS/exFAT (0x07)"
	case 0x17:
		return "Hidden IFS/NTFS (0x17)"
	case 0x27:
		return "NTFS WinRE recovery (0x27)"
	case 0xEE:
		return "GPT protective (0xEE)"
	case 0xEF:
		return "EFI System (0xEF)"
	case 0x0C, 0x0B:
		return "FAT32 (0x0B/0x0C)"
	case 0x05, 0x0F, 0x85:
		return "Extended (0x05/0x0F)"
	case 0x00:
		return "empty"
	default:
		return fmt.Sprintf("MBR type 0x%02X", t)
	}
}

func parseMBR(ra io.ReaderAt, size int64, mbr []byte) []Partition {
	var parts []Partition
	for i := 0; i < 4; i++ {
		e := mbr[mbrPartTable+i*mbrEntrySize : mbrPartTable+(i+1)*mbrEntrySize]
		typ := e[4]
		startLBA := binary.LittleEndian.Uint32(e[8:12])
		numSec := binary.LittleEndian.Uint32(e[12:16])
		if typ == 0x00 || numSec == 0 {
			continue
		}
		if typ == 0x05 || typ == 0x0F || typ == 0x85 {
			parts = append(parts, extendedChain(ra, size, int64(startLBA))...)
			continue
		}
		off := int64(startLBA) * mbrLBASize
		sz := int64(numSec) * mbrLBASize
		parts = append(parts, mkPart(ra, size, off, sz, mbrTypeName(typ)))
	}
	return parts
}

// extendedChain walks the EBR linked list of an extended partition. Each EBR has
// a logical-partition entry (relative to that EBR) and a link entry (relative to
// the extended partition's start). A bound caps pathological/looping chains.
func extendedChain(ra io.ReaderAt, size, extStartLBA int64) []Partition {
	var parts []Partition
	cur := extStartLBA
	for hops := 0; hops < 1024; hops++ {
		var ebr [512]byte
		if _, err := io.ReadFull(io.NewSectionReader(ra, cur*mbrLBASize, 512), ebr[:]); err != nil {
			break
		}
		if ebr[mbrSigOffset] != 0x55 || ebr[mbrSigOffset+1] != 0xAA {
			break
		}
		log := ebr[mbrPartTable : mbrPartTable+mbrEntrySize]
		if typ := log[4]; typ != 0x00 {
			relStart := int64(binary.LittleEndian.Uint32(log[8:12]))
			numSec := int64(binary.LittleEndian.Uint32(log[12:16]))
			if numSec > 0 {
				off := (cur + relStart) * mbrLBASize
				parts = append(parts, mkPart(ra, size, off, numSec*mbrLBASize, mbrTypeName(typ)))
			}
		}
		link := ebr[mbrPartTable+mbrEntrySize : mbrPartTable+2*mbrEntrySize]
		if link[4] == 0x00 {
			break
		}
		next := extStartLBA + int64(binary.LittleEndian.Uint32(link[8:12]))
		if next == cur || next <= extStartLBA && cur != extStartLBA {
			break
		}
		cur = next
	}
	return parts
}

// ---- GPT --------------------------------------------------------------------

// gptTypeGUIDs maps the partition-type GUID (canonical string form) to a label.
// NTFS lives in a Basic Data partition; the OEM check confirms it.
var gptTypeGUIDs = map[string]string{
	"c12a7328-f81f-11d2-ba4b-00a0c93ec93b": "EFI System",
	"ebd0a0a2-b9e5-4433-87c0-68b6b72699c7": "Microsoft basic data",
	"e3c9e316-0b5c-4db8-817d-f92df00215ae": "Microsoft reserved",
	"de94bba4-06d1-4d40-a16a-bfd50179d6ac": "Windows recovery",
	"5808c8aa-7e8f-42e0-85d2-e1e90434cfb3": "LDM metadata",
	"af9b60a0-1431-4f62-bc68-3311714a69ad": "LDM data",
}

// gptHeader locates the GPT header, trying 512- then 4096-byte logical sectors,
// and returns the header bytes and the detected sector size.
func gptHeader(ra io.ReaderAt, size int64) (hdr []byte, sectorSize int64, ok bool) {
	for _, ss := range []int64{512, 4096} {
		if ss+92 > size {
			continue
		}
		buf := make([]byte, 92)
		if _, err := io.ReadFull(io.NewSectionReader(ra, ss, 92), buf); err != nil {
			continue
		}
		if string(buf[0:8]) == string(gptMagic) {
			return buf, ss, true
		}
	}
	return nil, 0, false
}

func parseGPT(ra io.ReaderAt, size int64, hdr []byte, sectorSize int64) ([]Partition, error) {
	entryLBA := int64(binary.LittleEndian.Uint64(hdr[72:80]))
	numEntries := int64(binary.LittleEndian.Uint32(hdr[80:84]))
	entrySize := int64(binary.LittleEndian.Uint32(hdr[84:88]))
	if entrySize < 128 || numEntries <= 0 || numEntries > 8192 {
		return nil, fmt.Errorf("implausible GPT header (entries=%d, entrySize=%d)", numEntries, entrySize)
	}
	tableOff := entryLBA * sectorSize
	tableLen := numEntries * entrySize
	if tableOff <= 0 || tableOff+tableLen > size {
		return nil, errors.New("GPT entry array runs past the end of the image")
	}
	table := make([]byte, tableLen)
	if _, err := io.ReadFull(io.NewSectionReader(ra, tableOff, tableLen), table); err != nil {
		return nil, fmt.Errorf("read GPT entries: %w", err)
	}

	var parts []Partition
	for i := int64(0); i < numEntries; i++ {
		e := table[i*entrySize : i*entrySize+128]
		typeGUID := e[0:16]
		if isZeroGUID(typeGUID) {
			continue // unused slot
		}
		firstLBA := int64(binary.LittleEndian.Uint64(e[32:40]))
		lastLBA := int64(binary.LittleEndian.Uint64(e[40:48])) // inclusive
		if lastLBA < firstLBA {
			continue
		}
		off := firstLBA * sectorSize
		sz := (lastLBA - firstLBA + 1) * sectorSize
		name := gptTypeGUIDs[formatGUID(typeGUID)]
		if name == "" {
			name = "GPT " + formatGUID(typeGUID)
		}
		parts = append(parts, mkPart(ra, size, off, sz, name))
	}
	return parts, nil
}

func isZeroGUID(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// formatGUID renders a GPT type GUID in the canonical 8-4-4-4-12 string. The
// first three groups are little-endian on disk, the last two big-endian.
func formatGUID(b []byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[3], b[2], b[1], b[0],
		b[5], b[4],
		b[7], b[6],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15])
}

// ---- shared -----------------------------------------------------------------

// mkPart builds a Partition, clamping the size to the image and confirming NTFS
// by peeking the boot sector at the partition's offset.
func mkPart(ra io.ReaderAt, imgSize, off, sz int64, typeName string) Partition {
	if off < 0 {
		off = 0
	}
	if sz < 0 || off+sz > imgSize {
		sz = imgSize - off
	}
	return Partition{
		Offset:     off,
		Size:       sz,
		TypeName:   typeName,
		LikelyNTFS: off < imgSize && looksLikeNTFS(ra, off),
	}
}

// SelectNTFSVolume chooses which partition to mount. prefer >= 0 selects that
// index (an out-of-range index is an error). prefer < 0 auto-selects the largest
// partition whose boot sector is NTFS. It returns the chosen partition and its
// index in parts.
func SelectNTFSVolume(parts []Partition, prefer int) (Partition, int, error) {
	if len(parts) == 0 {
		return Partition{}, -1, errors.New("no partitions found on image")
	}
	if prefer >= 0 {
		if prefer >= len(parts) {
			return Partition{}, -1, fmt.Errorf("partition index %d out of range (%d found)", prefer, len(parts))
		}
		return parts[prefer], prefer, nil
	}
	best := -1
	for i, p := range parts {
		if !p.LikelyNTFS {
			continue
		}
		if best < 0 || p.Size > parts[best].Size {
			best = i
		}
	}
	if best < 0 {
		return Partition{}, -1, errors.New("no NTFS volume found among partitions")
	}
	return parts[best], best, nil
}

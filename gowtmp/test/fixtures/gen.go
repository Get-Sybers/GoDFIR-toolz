//go:build ignore

// gen.go writes the gowtmp contract-test fixtures into the directory named
// by argv[1]: a wtmp with a login/logout pair, a gzipped rotation, and a
// sparse lastlog — deterministic, no binary blobs committed.
//
//	go run test/fixtures/gen.go <dir>
package main

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

func rec(ut int16, pid int32, line, id, user, host string, ip []byte, sec, usec int32) []byte {
	b := make([]byte, 384)
	binary.LittleEndian.PutUint16(b[0:2], uint16(ut))
	binary.LittleEndian.PutUint32(b[4:8], uint32(pid))
	copy(b[8:40], line)
	copy(b[40:44], id)
	copy(b[44:76], user)
	copy(b[76:332], host)
	binary.LittleEndian.PutUint32(b[340:344], uint32(sec))
	binary.LittleEndian.PutUint32(b[344:348], uint32(usec))
	copy(b[348:364], ip)
	return b
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run gen.go <dir>")
		os.Exit(1)
	}
	dir := filepath.Join(os.Args[1], "var", "log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	ip4 := append([]byte{198, 51, 100, 7}, make([]byte, 12)...)
	wtmp := append(
		rec(7, 4242, "pts/0", "ts/0", "alice", "198.51.100.7", ip4, 1767225600, 481000),
		rec(8, 4242, "pts/0", "ts/0", "", "", nil, 1767229200, 0)...)
	if err := os.WriteFile(filepath.Join(dir, "wtmp"), wtmp, 0o644); err != nil {
		panic(err)
	}

	f, err := os.Create(filepath.Join(dir, "wtmp.1.gz"))
	if err != nil {
		panic(err)
	}
	zw := gzip.NewWriter(f)
	zw.Write(rec(7, 900, "tty1", "", "bob", "", nil, 1735689600, 0))
	zw.Close()
	f.Close()

	ll := make([]byte, 292*1001)
	off := 292 * 1000
	binary.LittleEndian.PutUint32(ll[off:off+4], 1767225600)
	copy(ll[off+4:off+36], "pts/0")
	copy(ll[off+36:off+292], "work.example")
	if err := os.WriteFile(filepath.Join(dir, "lastlog"), ll, 0o644); err != nil {
		panic(err)
	}
}

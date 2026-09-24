//go:build ignore

// gen.go writes the gojournal contract-test fixture into argv[1]: a tiny
// regular-layout, uncompressed journal built from the documented format —
// deterministic, no binary blobs committed.
package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
)

var buf []byte

func align() {
	for len(buf)%8 != 0 {
		buf = append(buf, 0)
	}
}

func obj(typ byte, payload []byte) uint64 {
	align()
	off := uint64(len(buf))
	h := make([]byte, 16)
	h[0] = typ
	binary.LittleEndian.PutUint64(h[8:16], uint64(16+len(payload)))
	buf = append(buf, h...)
	buf = append(buf, payload...)
	return off
}

func data(s string) uint64 {
	return obj(1, append(make([]byte, 48), []byte(s)...))
}

func entry(seq, rt uint64, datas []uint64) uint64 {
	p := make([]byte, 48, 48+16*len(datas))
	le := binary.LittleEndian
	le.PutUint64(p[0:8], seq)
	le.PutUint64(p[8:16], rt)
	copy(p[24:40], bytes.Repeat([]byte{0xAB}, 16))
	for _, d := range datas {
		item := make([]byte, 16)
		le.PutUint64(item[0:8], d)
		p = append(p, item...)
	}
	return obj(3, p)
}

func main() {
	buf = make([]byte, 240)
	d1 := data("MESSAGE=service started cleanly")
	d2 := data("_PID=1201")
	e1 := entry(7, 1767225600123456, []uint64{d1, d2})
	d3 := data("MESSAGE=second entry")
	e2 := entry(8, 1767225601000000, []uint64{d3})
	eaPayload := make([]byte, 8+16)
	binary.LittleEndian.PutUint64(eaPayload[8:16], e1)
	binary.LittleEndian.PutUint64(eaPayload[16:24], e2)
	ea := obj(6, eaPayload)

	le := binary.LittleEndian
	copy(buf[0:8], "LPKSHHRH")
	copy(buf[40:56], bytes.Repeat([]byte{0xCD}, 16))
	le.PutUint64(buf[88:96], 240)
	le.PutUint64(buf[152:160], 2)
	le.PutUint64(buf[176:184], ea)

	d := filepath.Join(os.Args[1], "var/log/journal/cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "system.journal"), buf, 0o644)
}

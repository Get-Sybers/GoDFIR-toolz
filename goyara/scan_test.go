package main

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const marker = "GOYARA_HIT_MARKER"

// writeRuleset writes a two-file ruleset — a top-level rules.yar that `include`s
// marker.yar — and returns the top path. Compiling from the top file proves the
// same include resolution the baked DetectRaptor set relies on (one .yar
// including many).
func writeRuleset(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.yar"),
		[]byte(`rule m { strings: $a = "`+marker+`" condition: $a }`), 0o600); err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(dir, "rules.yar")
	if err := os.WriteFile(top, []byte(`include "marker.yar"`), 0o600); err != nil {
		t.Fatal(err)
	}
	return top
}

// makeTar builds an in-memory tar of (name, body) pairs, mirroring gomount's
// stream: regular-file entries, mode 0400, Size = body length.
func makeTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(body)),
			Mode:     0o400,
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestScanTarStreamHit is the end-to-end proof: compile the marker rule (through
// an include), build a tar with one hitting file and one missing file, scan it,
// and confirm exactly one match record — for "/hit.txt", rule "m", carrying the
// matched string.
func TestScanTarStreamHit(t *testing.T) {
	rules, err := LoadRules(writeRuleset(t))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	defer rules.Destroy()

	tarball := makeTar(t, map[string]string{
		"/hit.txt":  "prefix " + marker + " suffix",
		"/miss.txt": "nothing to see here",
	})

	var got []match
	scanned, errs, err := ScanTarStream(rules, bytes.NewReader(tarball),
		func(m match) error { got = append(got, m); return nil }, Options{})
	if err != nil {
		t.Fatalf("ScanTarStream: %v", err)
	}
	if scanned != 2 {
		t.Fatalf("scanned = %d, want 2 (both regular files)", scanned)
	}
	if errs != 0 {
		t.Fatalf("errs = %d, want 0", errs)
	}
	if len(got) != 1 {
		t.Fatalf("matches = %d, want exactly 1: %+v", len(got), got)
	}

	m := got[0]
	if m.Target != "/hit.txt" {
		t.Errorf("target = %q, want /hit.txt", m.Target)
	}
	if m.Rule != "m" {
		t.Errorf("rule = %q, want m", m.Rule)
	}
	if m.Tool != "yara" || m.Source != "disk" {
		t.Errorf("tool/source = %q/%q, want yara/disk", m.Tool, m.Source)
	}
	if len(m.Strings) != 1 || m.Strings[0].Name != "$a" || m.Strings[0].Data != marker {
		t.Errorf("strings = %+v, want one $a with data %q", m.Strings, marker)
	}
	if m.Strings[0].Offset != 7 { // "prefix " is 7 bytes
		t.Errorf("offset = %d, want 7", m.Strings[0].Offset)
	}
}

// TestScanTarStreamNoMatch confirms a tar with no hits yields zero records while
// still counting the files as scanned.
func TestScanTarStreamNoMatch(t *testing.T) {
	rules, err := LoadRules(writeRuleset(t))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	defer rules.Destroy()

	tarball := makeTar(t, map[string]string{"/a.txt": "clean", "/b.txt": "also clean"})
	n := 0
	scanned, errs, err := ScanTarStream(rules, bytes.NewReader(tarball),
		func(match) error { n++; return nil }, Options{})
	if err != nil || errs != 0 {
		t.Fatalf("err=%v errs=%d", err, errs)
	}
	if scanned != 2 || n != 0 {
		t.Fatalf("scanned=%d matches=%d, want 2/0", scanned, n)
	}
}

// TestCapBoundsMatch proves the per-file cap bounds the scan: a marker placed
// past a tiny MaxBytes is not found, while the same tar scanned with the default
// cap finds it.
func TestCapBoundsMatch(t *testing.T) {
	rules, err := LoadRules(writeRuleset(t))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	defer rules.Destroy()

	body := string(bytes.Repeat([]byte("A"), 100)) + marker
	tarball := makeTar(t, map[string]string{"/big.txt": body})

	n := 0
	if _, _, err := ScanTarStream(rules, bytes.NewReader(tarball),
		func(match) error { n++; return nil }, Options{MaxBytes: 16}); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("matches with 16-byte cap = %d, want 0 (marker is past the cap)", n)
	}

	n = 0
	if _, _, err := ScanTarStream(rules, bytes.NewReader(tarball),
		func(match) error { n++; return nil }, Options{}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("matches with default cap = %d, want 1", n)
	}
}

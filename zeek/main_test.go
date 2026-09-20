package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeZeek writes a stand-in zeek script that records its argv and drops two
// small logs into the current directory, the way zeek does.
func fakeZeek(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "zeek")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > argv.txt\n" +
		"printf '#separator \\\\x09\\n#fields\\tts\\tuid\\nrow1\\nrow2\\n#close\\n' > conn.log\n" +
		"printf '#fields\\tts\\nrow1\\n' > dns.log\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIsCaptureByExtensionAndMagic(t *testing.T) {
	dir := t.TempDir()
	byName := filepath.Join(dir, "trace.PCAP")
	os.WriteFile(byName, []byte("not really"), 0o644)
	byMagic := filepath.Join(dir, "capture.bin")
	os.WriteFile(byMagic, append([]byte{0xd4, 0xc3, 0xb2, 0xa1}, make([]byte, 20)...), 0o644)
	pcapng := filepath.Join(dir, "capture.dat")
	os.WriteFile(pcapng, append([]byte{0x0a, 0x0d, 0x0d, 0x0a}, make([]byte, 20)...), 0o644)
	other := filepath.Join(dir, "notes.txt")
	os.WriteFile(other, []byte("hello world"), 0o644)
	for p, want := range map[string]bool{byName: true, byMagic: true, pcapng: true, other: false} {
		if got := isCapture(p); got != want {
			t.Errorf("isCapture(%s) = %v, want %v", filepath.Base(p), got, want)
		}
	}
}

func TestZeekArgs(t *testing.T) {
	cfg := &batchConfig{Format: "json", getenv: func(k string) string {
		if k == "ZEEK_SCRIPTS" {
			return "local Log::default_rotation_interval=0sec"
		}
		return ""
	}, Prefix: "ZEEK"}
	got := strings.Join(zeekArgs(cfg, "/input/x.pcap"), " ")
	want := "-C -r /input/x.pcap LogAscii::use_json=T LogAscii::json_timestamps=JSON::TS_ISO8601 local Log::default_rotation_interval=0sec"
	if got != want {
		t.Errorf("json args = %q", got)
	}
	cfg.Format = "tsv"
	if got := strings.Join(zeekArgs(cfg, "/input/x.pcap"), " "); got != "-C -r /input/x.pcap local Log::default_rotation_interval=0sec" {
		t.Errorf("tsv args = %q", got)
	}
}

func TestBatchProcessJSONRenamesAndIndexes(t *testing.T) {
	zeekBinary = fakeZeek(t)
	itemDir := t.TempDir()
	cfg := &batchConfig{Tool: "zeek", Prefix: "ZEEK", Format: "json", getenv: func(string) string { return "" }}
	var idx bytes.Buffer
	n, err := batchProcess(cfg, "/input/one.pcap", itemDir, &idx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("records = %d, want 3 (2 conn + 1 dns)", n)
	}
	for _, name := range []string{"conn.json", "dns.json"} {
		if _, err := os.Stat(filepath.Join(itemDir, name)); err != nil {
			t.Errorf("%s missing after rename: %v", name, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(itemDir, "*.log")); len(left) != 0 {
		t.Errorf("*.log left behind in json mode: %v", left)
	}
	var lines []logIndexEntry
	for _, l := range strings.Split(strings.TrimSpace(idx.String()), "\n") {
		var e logIndexEntry
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("index line %q: %v", l, err)
		}
		lines = append(lines, e)
	}
	if len(lines) != 2 || lines[0].Log != "conn.json" || lines[0].Records != 2 {
		t.Errorf("index = %+v", lines)
	}
	argv, _ := os.ReadFile(filepath.Join(itemDir, "argv.txt"))
	if !strings.Contains(string(argv), "LogAscii::use_json=T") {
		t.Errorf("zeek argv lacks the JSON option: %q", argv)
	}
}

func TestBatchProcessTSVKeepsLogs(t *testing.T) {
	zeekBinary = fakeZeek(t)
	itemDir := t.TempDir()
	cfg := &batchConfig{Tool: "zeek", Prefix: "ZEEK", Format: "tsv", getenv: func(string) string { return "" }}
	if _, err := batchProcess(cfg, "/input/one.pcap", itemDir, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, "conn.log")); err != nil {
		t.Errorf("conn.log missing in tsv mode: %v", err)
	}
}

func TestBatchProcessZeekFailure(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zeek")
	os.WriteFile(p, []byte("#!/bin/sh\nexit 3\n"), 0o755)
	zeekBinary = p
	cfg := &batchConfig{Tool: "zeek", Prefix: "ZEEK", Format: "json", getenv: func(string) string { return "" }}
	if _, err := batchProcess(cfg, "/input/bad.pcap", t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Error("a failing zeek must fail the item")
	}
}

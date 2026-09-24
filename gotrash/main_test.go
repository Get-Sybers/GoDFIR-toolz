package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/record"
)

const info = "[Trash Info]\nPath=/home/alice/secret%20plans.docx\nDeletionDate=2026-03-01T22:14:02\n"

func TestParseTrashinfo(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Trash", "info"), 0o755)
	os.MkdirAll(filepath.Join(dir, "Trash", "files"), 0o755)
	ip := filepath.Join(dir, "Trash", "info", "secret plans.docx.trashinfo")
	os.WriteFile(ip, []byte(info), 0o644)
	os.WriteFile(filepath.Join(dir, "Trash", "files", "secret plans.docx"), bytes.Repeat([]byte("x"), 42), 0o644)

	var buf bytes.Buffer
	w, _ := record.NewWriter(&buf, "json", nil)
	f, _ := os.Open(ip)
	defer f.Close()
	n, err := parseTrashinfo(f, ip, w)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	w.Flush()
	var rec map[string]any
	json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec)
	if rec["OriginalPath"] != "/home/alice/secret plans.docx" ||
		rec["OriginalPathRaw"] != "/home/alice/secret%20plans.docx" {
		t.Fatalf("path: %v", rec)
	}
	if rec["EventTime"] != "2026-03-01T22:14:02Z" || rec["TimeKind"] != "deleted" {
		t.Fatalf("time: %v", rec)
	}
	if rec["TrashedFileExists"] != true || rec["TrashedSize"] != float64(42) {
		t.Fatalf("twin: %v", rec)
	}
}

func TestParseTrashinfoRejectsGarbage(t *testing.T) {
	var buf bytes.Buffer
	w, _ := record.NewWriter(&buf, "json", nil)
	if _, err := parseTrashinfo(strings.NewReader("just some text\n"), "", w); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestBatchEndToEnd(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	p := filepath.Join(in, "home/alice/.local/share/Trash/info/x.txt.trashinfo")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(info), 0o644)
	env := map[string]string{"GOTRASH_INPUT_DIR": in, "GOTRASH_OUT_DIR": out, "GOTRASH_WORK_DIR": t.TempDir()}
	var stdout bytes.Buffer
	code := batch.Run(gotrashTool, batch.Options{Version: "test"},
		func(k string) string { return env[k] }, &stdout)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stdout.String())
	}
	var sum batch.Summary
	json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &sum)
	if sum.Records != 1 || sum.Processed != 1 {
		t.Fatalf("summary %+v", sum)
	}
}

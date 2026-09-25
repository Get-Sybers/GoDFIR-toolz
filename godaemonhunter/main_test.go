package main

// main_test.go proves the one-binary shape: subtool dispatch runs a
// parser's ordinary batch mode, hunt runs the layered pipeline — Layer 1
// builds the knowledge store, Layer 2 comes out enriched by it — and the
// whole run prints exactly one aggregate JSON line.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtures(t *testing.T, in string) {
	t.Helper()
	mk := func(rel, content string) {
		p := filepath.Join(in, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("etc/passwd", "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/zsh\n")
	mk("etc/hostname", "web01\n")
	mk("etc/timezone", "Europe/Berlin\n")
	mk("var/log/audit/audit.log",
		`type=SYSCALL msg=audit(1767225600.123:42): arch=c000003e syscall=59 success=yes exit=0 ppid=1200 pid=1201 auid=1000 uid=1000 gid=1000 euid=1000 ses=3 tty=pts0 comm="ls" exe="/usr/bin/ls" key="exec_log"`+"\n"+
			`type=EOE msg=audit(1767225600.123:42):`+"\n")
	mk("var/log/syslog", "Jan 10 22:14:02 web01 systemd[1]: Started daily apt activities.\n")
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSubtoolDispatch(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	p := filepath.Join(in, "home/alice/.local/share/Trash/info/x.txt.trashinfo")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("[Trash Info]\nPath=/home/alice/x.txt\nDeletionDate=2026-03-01T22:14:02\n"), 0o644)

	var buf bytes.Buffer
	code := run([]string{"gotrash"}, env(map[string]string{
		"GOTRASH_INPUT_DIR": in, "GOTRASH_OUT_DIR": out, "GOTRASH_WORK_DIR": t.TempDir(),
	}), &buf)
	if code != 0 {
		t.Fatalf("dispatch exit %d: %s", code, buf.String())
	}
	var sum map[string]any
	json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &sum)
	if sum["tool"] != "gotrash" || sum["records"] != float64(1) {
		t.Fatalf("summary: %v", sum)
	}
}

func TestUnknownAndUsage(t *testing.T) {
	var buf bytes.Buffer
	if code := run([]string{"gonosuch"}, env(nil), &buf); code != 2 {
		t.Fatalf("unknown subtool exit %d", code)
	}
	if code := run(nil, env(nil), &buf); code != 2 {
		t.Fatalf("no args exit %d", code)
	}
	buf.Reset()
	if code := run([]string{"--version"}, env(nil), &buf); code != 0 || !strings.HasPrefix(buf.String(), "godaemonhunter ") {
		t.Fatalf("version: %d %q", code, buf.String())
	}
}

func TestHuntLayeredAndEnriched(t *testing.T) {
	in, out := t.TempDir(), t.TempDir()
	writeFixtures(t, in)

	var buf bytes.Buffer
	code := run([]string{"hunt"}, env(map[string]string{
		"GODAEMONHUNTER_INPUT_DIR": in, "GODAEMONHUNTER_OUT_DIR": out,
		"GODAEMONHUNTER_WORK_DIR": t.TempDir(),
	}), &buf)
	if code != 0 {
		t.Fatalf("hunt exit %d: %s", code, buf.String())
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout is not one line: %d", len(lines))
	}
	var sum huntSummary
	if err := json.Unmarshal([]byte(lines[0]), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Status != "ok" || sum.Tool != "godaemonhunter" || len(sum.Subtools) != len(subs) {
		t.Fatalf("aggregate: %+v", sum)
	}

	// Layer 1 landed in the knowledge dir
	if _, err := os.Stat(filepath.Join(out, "knowledge")); err != nil {
		t.Fatal("knowledge dir missing")
	}

	// Layer 2 came out enriched: the audit event resolved uid 1000 and
	// carries the Host block; the syslog line got the host's zone
	// (Berlin winter, 22:14 local == 21:14Z).
	audit := readOne(t, filepath.Join(out, "goauditd"), "goauditd.jsonl")
	if audit["UIDName"] != "alice" || audit["UID"] != "1000" {
		t.Fatalf("audit enrichment: %v", audit)
	}
	host := audit["Host"].(map[string]any)
	if host["Hostname"] != "web01" || host["Timezone"] != "Europe/Berlin" {
		t.Fatalf("host block: %v", host)
	}
	syslog := readOne(t, filepath.Join(out, "gosyslog"), "gosyslog.jsonl")
	if syslog["EventTime"] != "2026-01-10T21:14:02.000000Z" {
		t.Fatalf("zone not applied: %v", syslog["EventTime"])
	}
}

func readOne(t *testing.T, root, name string) map[string]any {
	t.Helper()
	var rec map[string]any
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != name || rec != nil {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		first := strings.SplitN(strings.TrimSpace(string(b)), "\n", 2)[0]
		return json.Unmarshal([]byte(first), &rec)
	})
	if err != nil || rec == nil {
		t.Fatalf("no %s under %s: %v", name, root, err)
	}
	return rec
}

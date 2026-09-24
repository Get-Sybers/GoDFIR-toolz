package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/record"
)

// a benign execve event (ls -la in /home/alice) plus a service start.
const fixture = `type=SYSCALL msg=audit(1767225600.123:42): arch=c000003e syscall=59 success=yes exit=0 ppid=1200 pid=1201 auid=1000 uid=1000 gid=1000 euid=1000 ses=3 tty=pts0 comm="ls" exe="/usr/bin/ls" key="exec_log"
type=EXECVE msg=audit(1767225600.123:42): argc=2 a0="ls" a1=2D6C61
type=CWD msg=audit(1767225600.123:42): cwd="/home/alice"
type=PATH msg=audit(1767225600.123:42): item=0 name="/usr/bin/ls" inode=131204 mode=0100755 nametype=NORMAL
type=PROCTITLE msg=audit(1767225600.123:42): proctitle=6C73002D6C61
type=EOE msg=audit(1767225600.123:42):
type=SERVICE_START msg=audit(1767225700.000:43): pid=1 uid=0 auid=4294967295 ses=4294967295 msg='unit=nginx comm="systemd" exe="/usr/lib/systemd/systemd" res=success'
`

func parse(t *testing.T, content string) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	w, _ := record.NewWriter(&buf, "json", nil)
	if _, err := parseAudit(strings.NewReader(content), w, func(string, ...interface{}) {}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		json.Unmarshal([]byte(l), &m)
		out = append(out, m)
	}
	return out
}

func TestCoalescedExecve(t *testing.T) {
	recs := parse(t, fixture)
	if len(recs) != 2 {
		t.Fatalf("events: %d", len(recs))
	}
	e := recs[0]
	if e["AuditID"] != "1767225600.123:42" || e["EventTime"] != "2026-01-01T00:00:00.123Z" {
		t.Fatalf("identity/time: %v", e)
	}
	types := e["Types"].([]any)
	if len(types) != 6 || types[0] != "SYSCALL" || types[5] != "EOE" {
		t.Fatalf("types: %v", types)
	}
	if e["Syscall"] != "59" || e["SyscallName"] != "execve" || e["Success"] != "yes" ||
		e["PID"] != "1201" || e["UID"] != "1000" || e["Comm"] != "ls" ||
		e["Exe"] != "/usr/bin/ls" || e["Key"] != "exec_log" {
		t.Fatalf("syscall fields: %v", e)
	}
	argv := e["Argv"].([]any)
	if len(argv) != 2 || argv[0] != "ls" || argv[1] != "-la" { // a1 was hex 2D6C61
		t.Fatalf("argv: %v", argv)
	}
	if e["Cwd"] != "/home/alice" || e["Proctitle"] != "ls -la" {
		t.Fatalf("cwd/proctitle: %v", e)
	}
	paths := e["Paths"].([]any)
	p0 := paths[0].(map[string]any)
	if p0["Name"] != "/usr/bin/ls" || p0["Nametype"] != "NORMAL" || p0["Inode"] != "131204" {
		t.Fatalf("paths: %v", paths)
	}

	s := recs[1]
	if s["AuditID"] != "1767225700.000:43" || s["Syscall"] != nil {
		t.Fatalf("second event: %v", s)
	}
	fields := s["Fields"].(map[string]any)
	if !strings.Contains(fields["msg"].(string), "unit=nginx") {
		t.Fatalf("fields: %v", fields)
	}
}

func TestGarbageRejected(t *testing.T) {
	var buf bytes.Buffer
	w, _ := record.NewWriter(&buf, "json", nil)
	if _, err := parseAudit(strings.NewReader("hello\nworld\n"), w, func(string, ...interface{}) {}); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestParseKVQuoting(t *testing.T) {
	kv := parseKV(`a="x y" b='p q' c=bare d=`)
	if kv["a"] != "x y" || kv["b"] != "p q" || kv["c"] != "bare" || kv["d"] != "" {
		t.Fatalf("kv: %v", kv)
	}
}

func TestIsAuditFile(t *testing.T) {
	if !isAuditFile("var/log/audit/audit.log") || !isAuditFile("var/log/audit/audit.log.3") {
		t.Fatal("audit files not matched")
	}
	if isAuditFile("var/log/audit/audit.rules") || isAuditFile("var/log/syslog") {
		t.Fatal("non-audit matched")
	}
}

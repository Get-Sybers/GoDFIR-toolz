package main

// The thin binary's wiring: the embedded syslog.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// syslog/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gosyslog/syslog"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOSYSLOG_INPUT_DIR": t.TempDir(), "GOSYSLOG_OUT_DIR": t.TempDir(),
		"GOSYSLOG_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(syslog.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if syslog.Tool.Name != "gosyslog" {
		t.Fatalf("tool name %q", syslog.Tool.Name)
	}
}

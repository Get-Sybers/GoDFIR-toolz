package main

// The thin binary's wiring: the embedded auditd.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// auditd/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/goauditd/auditd"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOAUDITD_INPUT_DIR": t.TempDir(), "GOAUDITD_OUT_DIR": t.TempDir(),
		"GOAUDITD_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(auditd.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if auditd.Tool.Name != "goauditd" {
		t.Fatalf("tool name %q", auditd.Tool.Name)
	}
}

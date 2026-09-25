package main

// The thin binary's wiring: the embedded ctl.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// ctl/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/goctl/ctl"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOCTL_INPUT_DIR": t.TempDir(), "GOCTL_OUT_DIR": t.TempDir(),
		"GOCTL_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(ctl.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if ctl.Tool.Name != "goctl" {
		t.Fatalf("tool name %q", ctl.Tool.Name)
	}
}

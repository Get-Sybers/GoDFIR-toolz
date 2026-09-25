package main

// The thin binary's wiring: the embedded wtmp.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// wtmp/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gowtmp/wtmp"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOWTMP_INPUT_DIR": t.TempDir(), "GOWTMP_OUT_DIR": t.TempDir(),
		"GOWTMP_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(wtmp.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if wtmp.Tool.Name != "gowtmp" {
		t.Fatalf("tool name %q", wtmp.Tool.Name)
	}
}

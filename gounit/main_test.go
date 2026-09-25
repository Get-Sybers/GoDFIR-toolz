package main

// The thin binary's wiring: the embedded unit.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// unit/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gounit/unit"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOUNIT_INPUT_DIR": t.TempDir(), "GOUNIT_OUT_DIR": t.TempDir(),
		"GOUNIT_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(unit.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if unit.Tool.Name != "gounit" {
		t.Fatalf("tool name %q", unit.Tool.Name)
	}
}

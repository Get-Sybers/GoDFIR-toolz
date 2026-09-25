package main

// The thin binary's wiring: the embedded shell.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// shell/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/goshell/shell"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOSHELL_INPUT_DIR": t.TempDir(), "GOSHELL_OUT_DIR": t.TempDir(),
		"GOSHELL_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(shell.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if shell.Tool.Name != "goshell" {
		t.Fatalf("tool name %q", shell.Tool.Name)
	}
}

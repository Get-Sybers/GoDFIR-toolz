package main

// The thin binary's wiring: the embedded journal.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// journal/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gojournal/journal"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOJOURNAL_INPUT_DIR": t.TempDir(), "GOJOURNAL_OUT_DIR": t.TempDir(),
		"GOJOURNAL_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(journal.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if journal.Tool.Name != "gojournal" {
		t.Fatalf("tool name %q", journal.Tool.Name)
	}
}

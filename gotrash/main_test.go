package main

// The thin binary's wiring: the embedded trash.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// trash/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gotrash/trash"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOTRASH_INPUT_DIR": t.TempDir(), "GOTRASH_OUT_DIR": t.TempDir(),
		"GOTRASH_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(trash.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if trash.Tool.Name != "gotrash" {
		t.Fatalf("tool name %q", trash.Tool.Name)
	}
}

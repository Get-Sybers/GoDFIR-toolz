package main

// The thin binary's wiring: the embedded users.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// users/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gousers/users"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOUSERS_INPUT_DIR": t.TempDir(), "GOUSERS_OUT_DIR": t.TempDir(),
		"GOUSERS_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(users.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if users.Tool.Name != "gousers" {
		t.Fatalf("tool name %q", users.Tool.Name)
	}
}

package main

// The thin binary's wiring: the embedded network.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// network/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gonetwork/network"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GONETWORK_INPUT_DIR": t.TempDir(), "GONETWORK_OUT_DIR": t.TempDir(),
		"GONETWORK_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(network.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if network.Tool.Name != "gonetwork" {
		t.Fatalf("tool name %q", network.Tool.Name)
	}
}

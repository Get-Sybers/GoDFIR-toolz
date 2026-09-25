package main

// The thin binary's wiring: the embedded cron.Tool drives the shared batch
// runtime under this tool's contract (the substantive parser tests live in
// cron/).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Get-Sybers/GoDFIR-toolz/gocron/cron"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
)

func TestBinaryBatchContract(t *testing.T) {
	env := map[string]string{
		"GOCRON_INPUT_DIR": t.TempDir(), "GOCRON_OUT_DIR": t.TempDir(),
		"GOCRON_WORK_DIR": t.TempDir(),
	}
	var out bytes.Buffer
	code := batch.Run(cron.Tool, batch.Options{Version: "test", Contract: contractYML},
		func(k string) string { return env[k] }, &out)
	if code != 1 || !strings.Contains(out.String(), `"status":"nothing"`) {
		t.Fatalf("empty-tree contract: exit %d, %s", code, out.String())
	}
	if cron.Tool.Name != "gocron" {
		t.Fatalf("tool name %q", cron.Tool.Name)
	}
}

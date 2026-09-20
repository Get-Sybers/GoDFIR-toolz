package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stub writes an executable shell script and returns its path.
func stub(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func cfgFor(t *testing.T, prefix string, env map[string]string) *batchConfig {
	t.Helper()
	return &batchConfig{Tool: "signatures", Prefix: prefix, InputDir: env[prefix+"_INPUT_DIR"],
		WorkDir: t.TempDir(), Format: "json", LogLevel: logWarn,
		getenv: func(k string) string { return env[k] }}
}

func TestParseYaraOutput(t *testing.T) {
	text := "SUSP_Webshell [webshell,php] /scan/host/www/x.php\n" +
		"0x1a:$s1: <?php eval(\n" +
		"0x2f0:$s2: base64_decode\n" +
		"Plain_Rule /scan/host/bin/tool.exe\n" +
		"garbage-without-space\n"
	got := parseYaraOutput(strings.NewReader(text), "/scan")
	if len(got) != 2 {
		t.Fatalf("matches = %d, want 2: %+v", len(got), got)
	}
	if got[0].Rule != "SUSP_Webshell" || strings.Join(got[0].Tags, ",") != "webshell,php" || got[0].Target != "host/www/x.php" {
		t.Errorf("first match = %+v", got[0])
	}
	if len(got[0].Strings) != 2 || got[0].Strings[1].Name != "$s2" || got[0].Strings[1].Offset != "0x2f0" || got[0].Strings[1].Data != "base64_decode" {
		t.Errorf("strings = %+v", got[0].Strings)
	}
	if got[1].Rule != "Plain_Rule" || len(got[1].Tags) != 0 || len(got[1].Strings) != 0 {
		t.Errorf("second match = %+v", got[1])
	}
}

func TestYaraDiscoverAndProcess(t *testing.T) {
	in := t.TempDir()
	os.MkdirAll(filepath.Join(in, "hostA", "www"), 0o755)
	os.WriteFile(filepath.Join(in, "hostA", "www", "x.php"), []byte("<?php"), 0o644)
	os.WriteFile(filepath.Join(in, "loose.bin"), []byte("x"), 0o644)
	yaraBinary = stub(t, "yara", `printf 'Hit_Rule [t] %s/www/x.php\n0x0:$a: <?php\n' "$(eval echo \$$#)"`)
	env := map[string]string{"SIGNATURES_YARA_INPUT_DIR": in, "SIGNATURES_YARA_RULES": "/r.yar"}
	cfg := cfgFor(t, "SIGNATURES_YARA", env)
	items, err := discoverYara(cfg)
	if err != nil || len(items) != 2 {
		t.Fatalf("discover = %v, %v", items, err)
	}
	var out bytes.Buffer
	n, err := processYara(cfg, items[0], t.TempDir(), &out)
	if err != nil || n != 1 {
		t.Fatalf("process = %d, %v", n, err)
	}
	var m yaraMatch
	if err := json.Unmarshal(out.Bytes(), &m); err != nil || m.Rule != "Hit_Rule" || m.Target != "hostA/www/x.php" {
		t.Errorf("record = %+v (%v)", m, err)
	}
	yaraBinary = stub(t, "yara", "exit 1")
	if _, err := processYara(cfg, items[0], t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Error("a failing yara must fail the item")
	}
}

func TestSuricataProcess(t *testing.T) {
	in := t.TempDir()
	os.WriteFile(filepath.Join(in, "a.pcap"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(in, "notes.txt"), []byte("x"), 0o644)
	env := map[string]string{"SIGNATURES_SURICATA_INPUT_DIR": in, "SIGNATURES_SURICATA_SET": "HOME_NET=[10.0.0.0/8],EXTERNAL_NET=any"}
	cfg := cfgFor(t, "SIGNATURES_SURICATA", env)
	items, err := discoverSuricata(cfg)
	if err != nil || len(items) != 1 {
		t.Fatalf("discover = %v, %v", items, err)
	}
	// the stub records its argv and writes two EVE lines into the -l dir
	suricataBinary = stub(t, "suricata", `l=""; while [ $# -gt 0 ]; do case "$1" in -l) l="$2";; esac; shift; done; echo "$@" > /dev/null; printf '{"event_type":"alert"}\n{"event_type":"flow"}\n' > "$l/eve.json"`)
	itemDir := t.TempDir()
	var idx bytes.Buffer
	n, err := processSuricata(cfg, items[0], itemDir, &idx)
	if err != nil || n != 2 {
		t.Fatalf("process = %d, %v", n, err)
	}
	if !strings.Contains(idx.String(), `"records":2`) {
		t.Errorf("index = %s", idx.String())
	}
}

func TestHayabusaDiscoverAndProcess(t *testing.T) {
	in := t.TempDir()
	os.MkdirAll(filepath.Join(in, "hostA", "logs"), 0o755)
	os.WriteFile(filepath.Join(in, "hostA", "logs", "Security.evtx"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(in, "hostB"), 0o755) // no evtx: not an item
	env := map[string]string{"SIGNATURES_HAYABUSA_INPUT_DIR": in}
	cfg := cfgFor(t, "SIGNATURES_HAYABUSA", env)
	items, err := discoverHayabusa(cfg)
	if err != nil || len(items) != 1 || filepath.Base(items[0]) != "hostA" {
		t.Fatalf("discover = %v, %v", items, err)
	}
	hayabusaHome = t.TempDir()
	argvFile := filepath.Join(t.TempDir(), "argv")
	hayabusaBinary = stub(t, "hayabusa", `printf '%s\n' "$@" > `+argvFile+`; o=""; while [ $# -gt 0 ]; do case "$1" in --output) o="$2";; esac; shift; done; printf '{"RuleTitle":"x"}\n' > "$o"`)
	var idx bytes.Buffer
	n, err := processHayabusa(cfg, items[0], t.TempDir(), &idx)
	if err != nil || n != 1 {
		t.Fatalf("process = %d, %v", n, err)
	}
	// the argv is hayabusa 4's dfir-timeline form; the pre-4.0 json-timeline
	// command and its --JSONL-output / --UTC spellings no longer exist
	raw, _ := os.ReadFile(argvFile)
	argv := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(argv) == 0 || argv[0] != "dfir-timeline" {
		t.Fatalf("argv[0] = %q, want dfir-timeline (%v)", argv, argv)
	}
	line := " " + strings.Join(argv, " ") + " "
	for _, want := range []string{" --directory " + items[0] + " ", " --output-type jsonl ", " --profile verbose ",
		" --no-wizard ", " --utc ", " --quiet ", " --quiet-errors ", " --rules " + defaultHayabusaRules + " "} {
		if !strings.Contains(line, want) {
			t.Errorf("argv lacks %q: %s", strings.TrimSpace(want), line)
		}
	}
	for _, gone := range []string{"json-timeline", "--JSONL-output", " --UTC "} {
		if strings.Contains(line, gone) {
			t.Errorf("argv carries the removed %q: %s", strings.TrimSpace(gone), line)
		}
	}
	// no detections -> no output file from hayabusa -> an empty timeline, not a failure
	hayabusaBinary = stub(t, "hayabusa", "exit 0")
	itemDir := t.TempDir()
	if n, err := processHayabusa(cfg, items[0], itemDir, &bytes.Buffer{}); err != nil || n != 0 {
		t.Errorf("empty run = %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, "timeline.jsonl")); err != nil {
		t.Errorf("empty timeline not materialised: %v", err)
	}
}

func TestScanPipe(t *testing.T) {
	in := t.TempDir()
	os.WriteFile(filepath.Join(in, "disk.E01"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(in, "readme.md"), []byte("x"), 0o644)
	env := map[string]string{"SIGNATURES_SCAN_INPUT_DIR": in, "SIGNATURES_SCAN_FILTER": "*.exe"}
	cfg := cfgFor(t, "SIGNATURES_SCAN", env)
	items, err := discoverScan(cfg)
	if err != nil || len(items) != 1 {
		t.Fatalf("discover = %v, %v", items, err)
	}
	gomountBinary = stub(t, "gomount", `printf 'tar-bytes'`)
	goyaraBinary = stub(t, "goyara", `cat >/dev/null; printf '{"rule":"a"}\n{"rule":"b"}\n'`)
	var out bytes.Buffer
	n, err := processScan(cfg, items[0], t.TempDir(), &out)
	if err != nil || n != 2 {
		t.Fatalf("process = %d, %v (%s)", n, err, out.String())
	}
	goyaraBinary = stub(t, "goyara", `cat >/dev/null; printf '{"rule":"a"}\n'; exit 2`)
	if n, err := processScan(cfg, items[0], t.TempDir(), &bytes.Buffer{}); err != nil || n != 1 {
		t.Errorf("exit 2 must be a warning: %d, %v", n, err)
	}
	goyaraBinary = stub(t, "goyara", `cat >/dev/null; exit 1`)
	if _, err := processScan(cfg, items[0], t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Error("goyara exit 1 must fail the item")
	}
}

func TestPassthroughPath(t *testing.T) {
	hayabusaBinary = stub(t, "hayabusa", "exit 0")
	if p, err := passthroughPath("hayabusa"); err != nil || p != hayabusaBinary {
		t.Errorf("hayabusa resolves to %q, %v; want the baked binary %q", p, err, hayabusaBinary)
	}
	if p, err := passthroughPath("sh"); err != nil || p == "" {
		t.Errorf("sh = %q, %v", p, err)
	}
	if _, err := passthroughPath("no-such-tool-for-this-test"); err == nil {
		t.Error("an absent tool must not resolve")
	}
}

func TestSubtoolBindings(t *testing.T) {
	for name, tool := range subtools {
		if tool.name != "signatures" || tool.subtool != name || tool.prefix != "SIGNATURES_"+strings.ToUpper(name) {
			t.Errorf("%s binding = %+v", name, tool)
		}
	}
}

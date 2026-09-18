// goyara — scan gomount's tar stream with compiled YARA rules.
//
// goyara is the FIRST CONSUMER of gomount (GoDFIR-toolz epic #30). gomount walks
// an NTFS disk image in-process and emits a TAR archive — one entry per regular
// file, entry name = the file's path, body = the file's bytes. goyara reads that
// tar on STDIN and scans each file with a compiled YARA ruleset, writing one JSON
// match record per rule hit. It does NO disk or NTFS parsing itself; gomount owns
// all of that. The orchestration is a plain pipe — no mount, no FUSE, no /dev/kvm:
//
//	gomount stream disk.E01 | goyara --rules /opt/dxdfir/yara-rules/detectraptor/detectraptor.yar
//
// The scan itself is the core (scan.go): LoadRules compiles the ruleset once,
// ScanTarStream reads the tar entry-by-entry and scans each with libyara (cgo,
// github.com/hillu/go-yara/v4). This file is only the CLI over that core.
//
// Exit codes: 0 = every file scanned cleanly; 1 = fatal (bad rules, bad args,
// corrupt tar, output failure); 2 = finished but at least one file failed to
// read or scan (those are logged on stderr, the rest are still scanned).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// DefaultRulesPath is where the get-sybers/signatures image bakes the merged
// DetectRaptor ruleset (one detectraptor.yar covering the whole set).
const DefaultRulesPath = "/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar"

func main() { os.Exit(run(os.Args[1:])) }

func run(argv []string) int {
	fs := flag.NewFlagSet("goyara", flag.ContinueOnError)
	fs.Usage = usage
	rulesPath := fs.String("rules", DefaultRulesPath, "compiled YARA ruleset to scan with")
	jsonOut := fs.String("json", "-", "write JSONL match records here; - is stdout")
	maxBytes := fs.Int64("max-bytes", DefaultMaxBytes, "per-file scan cap in bytes")
	if err := fs.Parse(argv); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goyara: reads the tar stream on stdin; takes no positional arguments")
		usage()
		return 1
	}
	if *maxBytes <= 0 {
		fmt.Fprintln(os.Stderr, "goyara: --max-bytes must be positive")
		return 1
	}

	rules, err := LoadRules(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goyara: %v\n", err)
		return 1
	}

	out := os.Stdout
	if *jsonOut != "-" {
		f, err := os.Create(*jsonOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "goyara: %v\n", err)
			return 1
		}
		defer f.Close()
		out = f
	}
	bw := bufio.NewWriter(out)
	enc := json.NewEncoder(bw)

	matches := 0
	emit := func(m match) error {
		matches++
		// gomount trims the leading slash from each entry name; restore it so a
		// record points at the volume-absolute path (/Windows/System32/x.dll).
		m.Target = "/" + strings.TrimPrefix(m.Target, "/")
		return enc.Encode(m)
	}

	scanned, errs, scanErr := ScanTarStream(rules, os.Stdin, emit, Options{MaxBytes: *maxBytes})
	flushErr := bw.Flush()
	if scanErr != nil {
		fmt.Fprintf(os.Stderr, "goyara: %v\n", scanErr)
		return 1
	}
	if flushErr != nil {
		fmt.Fprintf(os.Stderr, "goyara: write output: %v\n", flushErr)
		return 1
	}

	summary, _ := json.Marshal(struct {
		Rules        int `json:"rules"`
		FilesScanned int `json:"files_scanned"`
		Matches      int `json:"matches"`
		Errors       int `json:"errors"`
	}{len(rules.GetRules()), scanned, matches, errs})
	fmt.Fprintln(os.Stderr, string(summary))

	if errs > 0 {
		return 2
	}
	return 0
}

func usage() {
	fmt.Fprint(os.Stderr, "goyara — scan gomount's tar stream on stdin with compiled YARA rules\n\n"+
		"Reads a tar archive on STDIN (one entry per file, as `gomount stream` emits),\n"+
		"scans each file's bytes with the compiled rules, and writes one JSON match\n"+
		"record per rule hit. Does NO disk/NTFS parsing itself — gomount owns that.\n\n"+
		"  gomount stream <image> | goyara [--rules FILE] [--json OUT] [--max-bytes N]\n\n"+
		"  --rules FILE    compiled YARA ruleset (default /opt/dxdfir/yara-rules/detectraptor/detectraptor.yar)\n"+
		"  --json OUT      write JSONL match records here; - is stdout (default -)\n"+
		"  --max-bytes N   per-file scan cap in bytes (default 33554432)\n\n"+
		"A one-line JSON summary {rules,files_scanned,matches,errors} is written to\n"+
		"stderr. Exit 0 clean, 1 fatal (bad rules or args), 2 finished with per-file errors.\n")
}

// godaemonhunter — the Linux matrix as ONE structured binary
// (docs/linux §4, decisions 15–16): every daemon parser lives here as a
// package and runs as a subcommand, plus `hunt`, the layered one-shot —
// Layer 1 (gohost, gousers, gonetwork) runs first and builds the image's
// knowledge store, then every daemon parser runs enriched by it. One
// binary, one run, one structured output tree, one JSON summary line.
//
//	godaemonhunter hunt                the layered run, GODAEMONHUNTER_* driven
//	godaemonhunter <subtool>           one parser's env-driven batch mode,
//	                                   under its canonical <SUBTOOL>_* block
//	godaemonhunter <subtool> <args>    that parser's argv debug pass-through
//	                                   (-f FILE | -d DIR | --tar, -q)
//	godaemonhunter --version | --print-contract
//
// The multi-tool dispatcher shape of docs/framework/04 §4.3 (the plaso
// and signatures precedent). There are no standalone per-parser binaries
// or images: godaemonhunter is the Linux tool.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Get-Sybers/GoDFIR-toolz/pinfo"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"

	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/auditd"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/cron"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/ctl"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/host"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/journal"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/network"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/shell"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/syslog"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/trash"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/unit"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/users"
	"github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter/wtmp"
)

// argvMain is a parser package's Main: batch.Entry plus the tool's argv
// debug modes on the global flag set. It never returns on the batch,
// --version and --print-contract paths, and exits itself on argv errors.

//go:embed contract.yml
var contractYML string

// version is stamped by the Dockerfile (-X main.version=${TOOL_VERSION}).
var version = "0.0.0-dev"

// sub is one embedded parser; layer 1 builds the knowledge store, layer 2
// consumes it. Order is the hunt execution order, deterministic.
type sub struct {
	name  string
	layer int
	tool  batch.Tool
	main  argvMain
}

type argvMain func(version, contractYML string)

var subs = []sub{
	{"gohost", 1, host.Tool, host.Main},
	{"gousers", 1, users.Tool, users.Main},
	{"gonetwork", 1, network.Tool, network.Main},
	{"gojournal", 2, journal.Tool, journal.Main},
	{"goauditd", 2, auditd.Tool, auditd.Main},
	{"gowtmp", 2, wtmp.Tool, wtmp.Main},
	{"gosyslog", 2, syslog.Tool, syslog.Main},
	{"gounit", 2, unit.Tool, unit.Main},
	{"gocron", 2, cron.Tool, cron.Main},
	{"goshell", 2, shell.Tool, shell.Main},
	{"gotrash", 2, trash.Tool, trash.Main},
	{"goctl", 2, ctl.Tool, ctl.Main},
}

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout)) }

// run is main's testable body: dispatch one subtool, or hunt.
func run(args []string, getenv func(string) string, stdout io.Writer) int {
	if len(args) == 1 {
		switch strings.TrimLeft(args[0], "-") {
		case "version":
			fmt.Fprintf(stdout, "godaemonhunter %s\n", version)
			return 0
		case "print-contract":
			io.WriteString(stdout, contractYML)
			return 0
		}
	}
	if len(args) == 0 {
		usage()
		return 2
	}
	if args[0] == "hunt" {
		if len(args) != 1 {
			usage()
			return 2
		}
		return runHunt(getenv, stdout)
	}
	for _, s := range subs {
		if s.name != args[0] {
			continue
		}
		if len(args) == 1 {
			return batch.Run(s.tool, batch.Options{Version: version, Contract: contractYML}, getenv, stdout)
		}
		// argv debug pass-through: hand the rest of the command line to
		// the parser's own Main, busybox-style. It exits the process.
		os.Args = append([]string{"godaemonhunter " + s.name}, args[1:]...)
		s.main(version, contractYML)
		return 0
	}
	fmt.Fprintf(os.Stderr, "godaemonhunter: unknown sub-tool %q\n", args[0])
	usage()
	return 2
}

func usage() {
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = s.name
	}
	fmt.Fprintln(os.Stderr, "usage: godaemonhunter hunt                 (the layered run, GODAEMONHUNTER_* driven)\n"+
		"       godaemonhunter <subtool>            (one parser's env-driven batch: "+strings.Join(names, " ")+")\n"+
		"       godaemonhunter <subtool> <args>     (that parser's argv debug pass-through: -f FILE | -d DIR | --tar, -q)\n"+
		"       godaemonhunter --version | --print-contract")
}

// huntSummary is hunt's single stdout JSON line: the aggregate roll-up
// with every subtool's own summary embedded.
type huntSummary struct {
	Tool         string          `json:"tool"`
	Version      string          `json:"version"`
	Pinfo        string          `json:"pinfo"`
	Status       string          `json:"status"`
	Inputs       int             `json:"inputs"`
	Processed    int             `json:"processed"`
	Skipped      int             `json:"skipped"`
	Failed       int             `json:"failed"`
	Records      int             `json:"records"`
	KnowledgeDir string          `json:"knowledge_dir"`
	Subtools     []batch.Summary `json:"subtools"`
	Exit         int             `json:"exit"`
	Started      string          `json:"started"`
	DurationS    float64         `json:"duration_s"`
}

// runHunt executes the layered pipeline: Layer 1 into the knowledge dir,
// then Layer 2 with the store mounted, each subtool through the ordinary
// batch runtime under a shimmed environment.
func runHunt(getenv func(string) string, stdout io.Writer) int {
	get := func(suffix, def string) string {
		if v := getenv("GODAEMONHUNTER_" + suffix); v != "" {
			return v
		}
		return def
	}
	in := get("INPUT_DIR", "/input")
	out := get("OUT_DIR", "/output")
	work := get("WORK_DIR", "/work")
	force := get("FORCE", "0")
	level := get("LOG_LEVEL", "info")
	kdir := get("KNOWLEDGE_DIR", filepath.Join(out, "knowledge"))

	started := time.Now()
	sum := &huntSummary{
		Tool: "godaemonhunter", Version: version, Pinfo: pinfo.Version,
		KnowledgeDir: kdir, Subtools: []batch.Summary{},
		Started: started.UTC().Format(time.RFC3339),
	}

	sawOK, sawPartial, sawConfig := false, false, false
	for _, s := range subs {
		outDir := kdir
		if s.layer == 2 {
			outDir = filepath.Join(out, s.name)
		}
		shim := shimEnv(s.tool, getenv, map[string]string{
			"INPUT_DIR": in, "OUT_DIR": outDir, "WORK_DIR": work,
			"FORCE": force, "LOG_LEVEL": level, "KNOWLEDGE_DIR": kdir,
		})
		var buf bytes.Buffer
		code := batch.Run(s.tool, batch.Options{Version: version, Contract: contractYML}, shim, &buf)
		var ss batch.Summary
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &ss); err != nil {
			ss = batch.Summary{Tool: s.name, Status: "config_error", Exit: code}
		}
		sum.Subtools = append(sum.Subtools, ss)
		sum.Inputs += ss.Inputs
		sum.Processed += ss.Processed
		sum.Skipped += ss.Skipped
		sum.Failed += ss.Failed
		sum.Records += ss.Records
		switch code {
		case 0:
			sawOK = true
		case 2:
			sawConfig = true
		case 3:
			sawPartial = true
			sawOK = true
		}
	}
	switch {
	case sawConfig:
		sum.Status, sum.Exit = "config_error", 2
	case sawPartial:
		sum.Status, sum.Exit = "partial", 3
	case sawOK:
		sum.Status, sum.Exit = "ok", 0
	default:
		sum.Status, sum.Exit = "nothing", 1
	}
	sum.DurationS = float64(int64(time.Since(started).Seconds()*1000)) / 1000
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(sum); err != nil {
		fmt.Fprintf(os.Stderr, "godaemonhunter: write summary: %v\n", err)
		return 2
	}
	return sum.Exit
}

// shimEnv maps a subtool's reserved variables onto hunt's values while
// letting every other variable fall through to the real environment.
func shimEnv(t batch.Tool, getenv func(string) string, vals map[string]string) func(string) string {
	pfx := batch.Prefix(t.Name) + "_"
	return func(k string) string {
		if suffix, ok := strings.CutPrefix(k, pfx); ok {
			if v, set := vals[suffix]; set {
				return v
			}
		}
		return getenv(k)
	}
}

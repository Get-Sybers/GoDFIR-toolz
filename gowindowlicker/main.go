// gowindowlicker — the Windows artefact dozen as ONE structured binary:
// the godaemonhunter shape (docs/linux decision 16) applied to the Windows
// side. Every Windows parser lives here as a package and a sub-tool of this
// single static binary; the standalone per-parser binaries, images, contracts
// and Dockerfiles are retired. Tool names, `<SUBTOOL>_*` env blocks, record
// shapes and record-file names are unchanged — byakugan and the pipeline see
// the same records; only the packaging is one.
//
//	gowindowlicker                     every parser — the default sweep,
//	                                   GOWINDOWLICKER_* driven
//	gowindowlicker <subtool>           one parser's env-driven batch mode,
//	                                   under its canonical <SUBTOOL>_* block
//	gowindowlicker <subtool> <args>    that parser's argv debug pass-through
//	                                   (-f FILE | -d DIR | --tar, …)
//	gowindowlicker --version | --print-contract
//
// (`lick` stays accepted as the explicit word for the default run.) The
// multi-tool dispatcher shape of docs/framework/04 §4.3 (the plaso,
// signatures and godaemonhunter precedent). Unlike godaemonhunter there is
// no stream vocabulary yet (docs/linux decision 17 admits a word only when
// byakugan maps feed on a parser here): today byakugan consumes goevtx,
// goprefetch, goese, gojle and gore directly and reaches the other artefact
// classes through plaso's l2t maps, so a Windows stream word would strand
// most of the matrix. The sub-tool names and the sweep are the interface
// until the direct Windows maps land. There is also no layered knowledge
// store: these parsers read self-contained artefacts, not a host's own
// record-keeping, so every sub-run is independent.
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

	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/batch"

	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/amcache"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/appcompat"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/ese"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/evtx"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/jle"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/le"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/mft"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/prefetch"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/rb"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/re"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/sbe"
	"github.com/Get-Sybers/GoDFIR-toolz/gowindowlicker/wxt"
)

//go:embed contract.yml
var contractYML string

// version is stamped by the Dockerfile (-X main.version=${TOOL_VERSION}).
var version = "0.0.0-dev"

// argvMain is a parser package's Main: batch.Entry plus the tool's argv
// debug modes on the global flag set. It never returns on the batch,
// --version and --print-contract paths, and exits itself on argv errors.
type argvMain func(version, contractYML string)

// sub is one embedded parser. Order is the sweep execution order,
// deterministic: the artefact-class order of the repo README.
type sub struct {
	name string
	tool batch.Tool
	main argvMain
}

var subs = []sub{
	{"goprefetch", prefetch.Tool, prefetch.Main},
	{"goese", ese.Tool, ese.Main},
	{"gorb", rb.Tool, rb.Main},
	{"gomft", mft.Tool, mft.Main},
	{"goamcache", amcache.Tool, amcache.Main},
	{"goappcompat", appcompat.Tool, appcompat.Main},
	{"goevtx", evtx.Tool, evtx.Main},
	{"gore", re.Tool, re.Main},
	{"gosbe", sbe.Tool, sbe.Main},
	{"gole", le.Tool, le.Main},
	{"gojle", jle.Tool, jle.Main},
	{"gowxt", wxt.Tool, wxt.Main},
}

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout)) }

// run is main's testable body: no arguments is every parser (the default
// sweep), a subtool name is one parser.
func run(args []string, getenv func(string) string, stdout io.Writer) int {
	if len(args) == 1 {
		switch strings.TrimLeft(args[0], "-") {
		case "version":
			fmt.Fprintf(stdout, "gowindowlicker %s\n", version)
			return 0
		case "print-contract":
			io.WriteString(stdout, contractYML)
			return 0
		}
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "lick") {
		return runLick(getenv, stdout)
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
		os.Args = append([]string{"gowindowlicker " + s.name}, args[1:]...)
		s.main(version, contractYML)
		return 0
	}
	fmt.Fprintf(os.Stderr, "gowindowlicker: unknown sub-tool %q\n", args[0])
	usage()
	return 2
}

func usage() {
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = s.name
	}
	fmt.Fprintln(os.Stderr, "usage: gowindowlicker                      (every parser — the default sweep, GOWINDOWLICKER_* driven)\n"+
		"       gowindowlicker <subtool>            (one parser's env-driven batch: "+strings.Join(names, " ")+")\n"+
		"       gowindowlicker <subtool> <args>     (that parser's argv debug pass-through: -f FILE | -d DIR | --tar, …)\n"+
		"       gowindowlicker --version | --print-contract")
}

// lickSummary is the sweep's single stdout JSON line: the aggregate roll-up
// with every subtool's own summary embedded.
type lickSummary struct {
	Tool      string          `json:"tool"`
	Version   string          `json:"version"`
	Status    string          `json:"status"`
	Inputs    int             `json:"inputs"`
	Processed int             `json:"processed"`
	Skipped   int             `json:"skipped"`
	Failed    int             `json:"failed"`
	Records   int             `json:"records"`
	Subtools  []batch.Summary `json:"subtools"`
	Exit      int             `json:"exit"`
	Started   string          `json:"started"`
	DurationS float64         `json:"duration_s"`
}

// runLick executes every parser over the same input tree, each through the
// ordinary batch runtime under a shimmed environment into its own
// <OUT_DIR>/<subtool>/ tree, and prints one aggregate summary line. Format
// stays per parser (three of the twelve are JSONL-only), so a sub-tool's own
// <SUBTOOL>_FORMAT falls through the shim untouched.
func runLick(getenv func(string) string, stdout io.Writer) int {
	get := func(suffix, def string) string {
		if v := getenv("GOWINDOWLICKER_" + suffix); v != "" {
			return v
		}
		return def
	}
	in := get("INPUT_DIR", "/input")
	out := get("OUT_DIR", "/output")
	work := get("WORK_DIR", "/work")
	force := get("FORCE", "0")
	level := get("LOG_LEVEL", "info")

	started := time.Now()
	sum := &lickSummary{
		Tool: "gowindowlicker", Version: version, Subtools: []batch.Summary{},
		Started: started.UTC().Format(time.RFC3339),
	}

	sawOK, sawPartial, sawConfig := false, false, false
	for _, s := range subs {
		shim := shimEnv(s.tool, getenv, map[string]string{
			"INPUT_DIR": in, "OUT_DIR": filepath.Join(out, s.name), "WORK_DIR": work,
			"FORCE": force, "LOG_LEVEL": level,
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
		fmt.Fprintf(os.Stderr, "gowindowlicker: write summary: %v\n", err)
		return 2
	}
	return sum.Exit
}

// shimEnv maps a subtool's reserved variables onto the sweep's values while
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

// signatures-entry — the multi-tool dispatcher ENTRYPOINT of
// get-sybers/signatures (docs/framework 04 §4.3).
//
//	signatures-entry yara | suricata | hayabusa | scan
//	    Self-orchestrate that sub-tool from its own env block
//	    (SIGNATURES_YARA_*, SIGNATURES_SURICATA_*, SIGNATURES_HAYABUSA_*,
//	    SIGNATURES_SCAN_*): discover the inputs under its INPUT_DIR, batch over
//	    them into one output folder per item under its OUT_DIR, skip items that
//	    already have valid output unless FORCE is set, print exactly one JSON
//	    summary line on stdout and exit 0 / 1 / 2 / 3 (batch.go).
//
//	signatures-entry <tool> <args...>
//	    The debug pass-through: exec one of the image's own tools (yara,
//	    suricata, hayabusa, gomount, goyara, the scan-list.sh loop, sh) with
//	    that argv verbatim.
//
//	signatures-entry --version | --print-contract
//
// The sub-tools:
//
//	yara      one item per immediate child of INPUT_DIR (a staged tree or a
//	          file); `yara -w -s -N -r <rules> <item>`; one record per rule hit
//	suricata  one item per capture (pcap/pcapng); `suricata -r <capture> -l
//	          <item dir> -k none -S <rules> [--set …]`; records = eve.json lines
//	hayabusa  one item per directory under INPUT_DIR that holds .evtx files;
//	          `hayabusa json-timeline --directory <item> --output …`; records =
//	          detections
//	scan      one item per disk image; `gomount stream <image> | goyara --rules
//	          <rules>`; records = rule hits across the image's files
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// The executables the dispatcher drives; tests point them at stand-ins.
var (
	yaraBinary     = "yara"
	suricataBinary = "suricata"
	hayabusaBinary = "/opt/dxdfir/hayabusa/hayabusa"
	hayabusaHome   = "/opt/dxdfir/hayabusa"
	gomountBinary  = "gomount"
	goyaraBinary   = "goyara"
)

const (
	defaultYaraRules     = "/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar"
	defaultSuricataRules = "/opt/dxdfir/suricata-rules/suricata.rules"
	defaultHayabusaRules = "/opt/dxdfir/hayabusa/rules"
)

// passthrough is the allow-list of argv[0] values the pass-through mode execs.
var passthrough = map[string]bool{
	"yara": true, "suricata": true, "hayabusa": true, hayabusaBinary: true,
	"gomount": true, "goyara": true, "/opt/dxdfir/scan-list.sh": true, "sh": true, "/bin/sh": true,
}

// subtools maps a sub-tool name to its batch binding.
var subtools = map[string]batchTool{
	"yara":     {name: "signatures", subtool: "yara", prefix: "SIGNATURES_YARA", formats: []string{"json"}, discover: discoverYara, process: processYara},
	"suricata": {name: "signatures", subtool: "suricata", prefix: "SIGNATURES_SURICATA", formats: []string{"json"}, discover: discoverSuricata, process: processSuricata},
	"hayabusa": {name: "signatures", subtool: "hayabusa", prefix: "SIGNATURES_HAYABUSA", formats: []string{"json"}, discover: discoverHayabusa, process: processHayabusa},
	"scan":     {name: "signatures", subtool: "scan", prefix: "SIGNATURES_SCAN", formats: []string{"json"}, discover: discoverScan, process: processScan},
}

// ---- shared helpers ------------------------------------------------------------

// walkFiles returns every regular file under root, sorted; unreadable subtrees
// are noted and skipped, an unreadable root is an error.
func walkFiles(cfg *batchConfig, root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			cfg.logf(logWarn, "skipping unreadable %s: %v", p, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// children returns the immediate entries of dir, sorted.
func children(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// runTool runs one external command with its stdout and stderr sent to the
// container's stderr, returning the exit code and any launch error.
func runTool(cfg *batchConfig, dir string, name string, args ...string) (int, error) {
	cfg.logf(logDebug, "exec %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

// countLines counts the non-empty lines of a file (0 when it is absent).
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			n++
		}
	}
	return n
}

var captureExts = map[string]bool{".pcap": true, ".pcapng": true, ".cap": true}
var imageExts = map[string]bool{".e01": true, ".ex01": true, ".raw": true, ".dd": true, ".img": true, ".vmdk": true, ".vhd": true, ".vhdx": true, ".001": true, ".bin": true}

func hasExt(p string, set map[string]bool) bool { return set[strings.ToLower(filepath.Ext(p))] }

// ---- yara ------------------------------------------------------------------------

// yaraMatch is one rule hit, in the record shape goyara emits.
type yaraMatch struct {
	Tool    string       `json:"tool"`
	Rule    string       `json:"rule"`
	Tags    []string     `json:"tags"`
	Target  string       `json:"target"`
	Strings []yaraString `json:"strings"`
}

type yaraString struct {
	Offset string `json:"offset"`
	Name   string `json:"name"`
	Data   string `json:"data"`
}

// discoverYara: every immediate child of the input root is one scan target.
func discoverYara(cfg *batchConfig) ([]string, error) { return children(cfg.InputDir) }

// parseYaraOutput turns `yara -s` text into match records: a hit line is
// `<rule> [tag,…] <target>` (tags optional), followed by `0x<offset>:$<name>:
// <data>` string lines.
func parseYaraOutput(r io.Reader, base string) []yaraMatch {
	var out []yaraMatch
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "0x") && len(out) > 0 {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) == 3 {
				out[len(out)-1].Strings = append(out[len(out)-1].Strings,
					yaraString{Offset: parts[0], Name: parts[1], Data: strings.TrimPrefix(parts[2], " ")})
			}
			continue
		}
		rule, rest, ok := strings.Cut(line, " ")
		if !ok || rule == "" {
			continue
		}
		var tags []string
		if strings.HasPrefix(rest, "[") {
			if end := strings.Index(rest, "]"); end > 0 {
				if end > 1 {
					tags = strings.Split(rest[1:end], ",")
				}
				rest = strings.TrimSpace(rest[end+1:])
			}
		}
		target := rest
		if base != "" {
			if rel, err := filepath.Rel(base, rest); err == nil && !strings.HasPrefix(rel, "..") {
				target = rel
			}
		}
		if tags == nil {
			tags = []string{}
		}
		out = append(out, yaraMatch{Tool: "yara", Rule: rule, Tags: tags, Target: target, Strings: []yaraString{}})
	}
	return out
}

// processYara scans one item recursively with the rules and writes one record
// per hit. yara's exit 0 covers match and no-match alike; anything else is a
// scan failure.
func processYara(cfg *batchConfig, item, _ string, w io.Writer) (int, error) {
	args := []string{"-w", "-s", "-N", "-r"}
	args = append(args, strings.Fields(cfg.env("ARGS", ""))...)
	args = append(args, cfg.env("RULES", defaultYaraRules), item)
	cfg.logf(logDebug, "exec %s %s", yaraBinary, strings.Join(args, " "))
	cmd := exec.Command(yaraBinary, args...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	matches := parseYaraOutput(stdout, cfg.InputDir)
	if err := cmd.Wait(); err != nil {
		return 0, fmt.Errorf("yara: %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, m := range matches {
		if err := enc.Encode(m); err != nil {
			return 0, err
		}
	}
	return len(matches), nil
}

// ---- suricata ----------------------------------------------------------------------

func discoverSuricata(cfg *batchConfig) ([]string, error) {
	files, err := walkFiles(cfg, cfg.InputDir)
	if err != nil {
		return nil, err
	}
	var items []string
	for _, p := range files {
		if hasExt(p, captureExts) {
			items = append(items, p)
		}
	}
	return items, nil
}

// suricataIndex is the item index line: where the EVE output landed.
type suricataIndex struct {
	Eve     string `json:"eve"`
	Records int    `json:"records"`
}

// processSuricata replays one capture offline into itemDir (eve.json and
// suricata's own logs land there) and indexes the EVE record count.
func processSuricata(cfg *batchConfig, item, itemDir string, w io.Writer) (int, error) {
	eve := filepath.Join(itemDir, "eve.json")
	os.Remove(eve)
	args := []string{"-r", item, "-l", itemDir, "-k", "none",
		"-S", cfg.env("RULES", defaultSuricataRules),
		"--pidfile", filepath.Join(cfg.WorkDir, "suricata.pid"),
		"--set", "unix-command.enabled=no"}
	for _, kv := range strings.Fields(strings.ReplaceAll(cfg.env("SET", ""), ",", " ")) {
		args = append(args, "--set", kv)
	}
	args = append(args, strings.Fields(cfg.env("ARGS", ""))...)
	code, err := runTool(cfg, itemDir, suricataBinary, args...)
	if err != nil {
		return 0, fmt.Errorf("suricata: %w", err)
	}
	if code != 0 {
		return 0, fmt.Errorf("suricata exited %d", code)
	}
	if _, err := os.Stat(eve); err != nil {
		return 0, fmt.Errorf("suricata produced no eve.json")
	}
	n := countLines(eve)
	return n, json.NewEncoder(w).Encode(suricataIndex{Eve: "eve.json", Records: n})
}

// ---- hayabusa ----------------------------------------------------------------------

// dirHasEvtx reports whether dir holds an .evtx anywhere below it.
func dirHasEvtx(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".evtx") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// discoverHayabusa: every immediate subdirectory holding .evtx files is one
// item; when the input root itself holds .evtx files directly, the root is the
// (single) item.
func discoverHayabusa(cfg *batchConfig) ([]string, error) {
	entries, err := os.ReadDir(cfg.InputDir)
	if err != nil {
		return nil, err
	}
	var items []string
	rootHasEvtx := false
	for _, e := range entries {
		p := filepath.Join(cfg.InputDir, e.Name())
		switch {
		case e.IsDir() && dirHasEvtx(p):
			items = append(items, p)
		case !e.IsDir() && strings.EqualFold(filepath.Ext(p), ".evtx"):
			rootHasEvtx = true
		}
	}
	if rootHasEvtx {
		items = append(items, cfg.InputDir)
	}
	sort.Strings(items)
	return items, nil
}

type hayabusaIndex struct {
	Timeline string `json:"timeline"`
	Records  int    `json:"records"`
}

// processHayabusa runs the baked hayabusa over one directory, JSONL with the
// verbose profile so every detection carries its MITRE columns. hayabusa
// refuses to overwrite its output, so a previous timeline is removed first.
func processHayabusa(cfg *batchConfig, item, itemDir string, w io.Writer) (int, error) {
	out := filepath.Join(itemDir, "timeline.jsonl")
	os.Remove(out)
	args := []string{"json-timeline", "--directory", item, "--output", out, "--JSONL-output",
		"--profile", cfg.env("PROFILE", "verbose"), "--no-wizard", "--UTC", "--quiet",
		"--rules", cfg.env("RULES", defaultHayabusaRules)}
	args = append(args, strings.Fields(cfg.env("ARGS", ""))...)
	code, err := runTool(cfg, hayabusaHome, hayabusaBinary, args...)
	if err != nil {
		return 0, fmt.Errorf("hayabusa: %w", err)
	}
	if code != 0 {
		return 0, fmt.Errorf("hayabusa exited %d", code)
	}
	// no detections leaves no output file: an empty timeline is still a result
	if _, err := os.Stat(out); err != nil {
		if f, cerr := os.Create(out); cerr == nil {
			f.Close()
		}
	}
	n := countLines(out)
	return n, json.NewEncoder(w).Encode(hayabusaIndex{Timeline: "timeline.jsonl", Records: n})
}

// ---- scan (gomount stream | goyara) -----------------------------------------------------

func discoverScan(cfg *batchConfig) ([]string, error) {
	files, err := walkFiles(cfg, cfg.InputDir)
	if err != nil {
		return nil, err
	}
	var items []string
	for _, p := range files {
		if hasExt(p, imageExts) {
			items = append(items, p)
		}
	}
	return items, nil
}

// processScan streams every file of one NTFS image out of gomount and through
// goyara in-process, writing goyara's hit records to w. goyara's exit 2
// (the scan completed but some files could not be read) is a warning, not a
// failure.
func processScan(cfg *batchConfig, item, _ string, w io.Writer) (int, error) {
	mountArgs := []string{"stream"}
	if f := cfg.env("FILTER", ""); f != "" {
		mountArgs = append(mountArgs, "--filter", f)
	}
	if v := cfg.env("VOLUME", "0"); v != "0" {
		if _, err := strconv.Atoi(v); err != nil {
			return 0, fmt.Errorf("%s_VOLUME: %q is not a number", cfg.Prefix, v)
		}
		mountArgs = append(mountArgs, "--volume", v)
	}
	mountArgs = append(mountArgs, item)
	scanArgs := []string{"--rules", cfg.env("RULES", defaultYaraRules), "--json", "-",
		"--max-bytes", cfg.env("MAX_BYTES", "33554432")}
	scanArgs = append(scanArgs, strings.Fields(cfg.env("ARGS", ""))...)

	cfg.logf(logDebug, "exec %s %s | %s %s", gomountBinary, strings.Join(mountArgs, " "), goyaraBinary, strings.Join(scanArgs, " "))
	mount := exec.Command(gomountBinary, mountArgs...)
	scan := exec.Command(goyaraBinary, scanArgs...)
	mount.Stderr = os.Stderr
	scan.Stderr = os.Stderr
	pipe, err := mount.StdoutPipe()
	if err != nil {
		return 0, err
	}
	scan.Stdin = pipe
	// goyara's records are counted on the way into the record file
	counter := &lineCounter{w: w}
	scan.Stdout = counter
	if err := mount.Start(); err != nil {
		return 0, fmt.Errorf("gomount: %w", err)
	}
	if err := scan.Start(); err != nil {
		mount.Process.Kill()
		mount.Wait()
		return 0, fmt.Errorf("goyara: %w", err)
	}
	scanErr := scan.Wait()
	mountErr := mount.Wait()
	if mountErr != nil {
		return counter.n, fmt.Errorf("gomount stream: %w", mountErr)
	}
	if scanErr != nil {
		if exitErr, ok := scanErr.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			cfg.logf(logWarn, "%s: goyara could not read every file (exit 2); the hits it found are kept", item)
		} else {
			return counter.n, fmt.Errorf("goyara: %w", scanErr)
		}
	}
	return counter.n, nil
}

// lineCounter passes bytes through to w while counting newline-terminated lines.
type lineCounter struct {
	w io.Writer
	n int
}

func (c *lineCounter) Write(p []byte) (int, error) {
	c.n += strings.Count(string(p), "\n")
	return c.w.Write(p)
}

// ---- entry -----------------------------------------------------------------------

func usage() {
	fmt.Fprint(os.Stderr, "usage: signatures-entry yara|suricata|hayabusa|scan        (env-driven batch)\n"+
		"       signatures-entry <yara|suricata|hayabusa|gomount|goyara|/opt/dxdfir/scan-list.sh|sh> <args...>   (pass-through)\n"+
		"       signatures-entry --version | --print-contract\n")
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 {
		switch strings.TrimLeft(args[0], "-") {
		case "version":
			fmt.Printf("signatures %s\n", version)
			os.Exit(0)
		case "print-contract":
			io.WriteString(os.Stdout, contractYML)
			os.Exit(0)
		}
		if t, ok := subtools[args[0]]; ok {
			os.Exit(runBatch(t, os.Getenv, os.Stdout))
		}
	}
	if len(args) > 0 && passthrough[args[0]] {
		path, err := exec.LookPath(args[0])
		if err == nil {
			err = syscall.Exec(path, args, os.Environ())
		}
		fmt.Fprintf(os.Stderr, "signatures-entry: exec %s: %v\n", args[0], err)
		os.Exit(1)
	}
	usage()
	msg := "no sub-tool named (yara|suricata|hayabusa|scan)"
	if len(args) > 0 {
		msg = fmt.Sprintf("%q is neither a sub-tool nor an allowed pass-through tool", args[0])
	}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(batchSummary{Tool: "signatures", Version: version, Status: "config_error", Outputs: []string{}, Error: msg, Exit: 2})
	os.Exit(2)
}

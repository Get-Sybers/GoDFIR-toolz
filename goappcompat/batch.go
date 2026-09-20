// batch.go — the container-framework batch runtime shared by the GoDFIR-toolz
// Go tools (docs/framework/03 environment contract, 04 self-orchestration).
//
// This file is byte-identical in every Tier-1 tool directory; the tool-specific
// glue (artefact discovery and per-item processing) lives in main.go.
//
// With no arguments the binary runs in batch mode:
//
//  1. read the <TOOL>_* environment (INPUT_DIR, OUT_DIR, WORK_DIR, FORCE,
//     FORMAT, LOG_LEVEL) — every variable has a default;
//  2. discover every artefact the tool handles under INPUT_DIR (recursed);
//  3. process each item into its own subfolder under OUT_DIR, writing the
//     records to <OUT_DIR>/<item>/<tool>.<jsonl|csv>;
//  4. skip an item whose record file already exists unless FORCE is set, so a
//     rerun converges and never duplicates output;
//  5. print exactly one JSON summary line on stdout and exit with the uniform
//     code: 0 success · 1 nothing produced · 2 config error · 3 partial.
//
// Any argument switches to the tool's own argv mode (the debug pass-through),
// except `--version` and `--print-contract`, which the runtime answers itself.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// contractYML is the checked-in contract, embedded so `--print-contract` makes
// the interface discoverable from the binary alone.
//
//go:embed contract.yml
var contractYML string

// version is the per-image version. The Dockerfile stamps it with
// `-ldflags "-X main.version=${TOOL_VERSION}"` so the binary and the
// org.opencontainers.image.version label carry the same value.
var version = "0.0.0-dev"

// batchTool binds the shared runtime to one tool.
type batchTool struct {
	name string
	// subtool names the dispatched sub-tool of a multi-tool image (04 §4.3).
	// It is reported in the summary and names the record file; empty for a
	// single-tool image.
	subtool string
	// prefix overrides the variable prefix derived from name (<NAME>_): a
	// multi-tool image's sub-tool env block, e.g. SIGNATURES_YARA.
	prefix string
	// formats lists the accepted <TOOL>_FORMAT values; the first is the default.
	formats []string
	// discover returns every artefact under cfg.InputDir this tool handles,
	// sorted. An error here is a config error (unreadable input, bad tool
	// setting) and ends the run with exit 2.
	discover func(cfg *batchConfig) ([]string, error)
	// process handles one item. w is the item's record file
	// (<itemDir>/<tool>.<ext>), created by the runtime and committed only when
	// process returns nil; itemDir is there for tools that write further files
	// beside it. It returns the number of records written.
	process func(cfg *batchConfig, item, itemDir string, w io.Writer) (records int, err error)
}

// batchConfig is the resolved environment contract of one run.
type batchConfig struct {
	Tool     string
	Prefix   string
	InputDir string
	OutDir   string
	WorkDir  string
	Force    bool
	Format   string
	LogLevel int
	getenv   func(string) string
}

// Log levels, in decreasing severity; a message prints when its level is at or
// below the configured one. Everything goes to stderr.
const (
	logError = iota
	logWarn
	logInfo
	logDebug
)

var logLevels = map[string]int{"error": logError, "warn": logWarn, "info": logInfo, "debug": logDebug}

// fileBase is the record file's base name: the sub-tool for a multi-tool
// image, otherwise the tool.
func (t batchTool) fileBase() string {
	if t.subtool != "" {
		return t.subtool
	}
	return t.name
}

// envPrefix is the variable prefix this tool reads.
func (t batchTool) envPrefix() string {
	if t.prefix != "" {
		return t.prefix
	}
	return batchPrefix(t.name)
}

// batchSummary is the single JSON line printed on stdout.
type batchSummary struct {
	Tool      string         `json:"tool"`
	Subtool   string         `json:"subtool,omitempty"`
	Version   string         `json:"version"`
	Status    string         `json:"status"`
	Inputs    int            `json:"inputs"`
	Processed int            `json:"processed"`
	Skipped   int            `json:"skipped"`
	Failed    int            `json:"failed"`
	Records   int            `json:"records"`
	Outputs   []string       `json:"outputs"`
	Failures  []batchFailure `json:"failures,omitempty"`
	Error     string         `json:"error,omitempty"`
	Exit      int            `json:"exit"`
	Started   string         `json:"started"`
	DurationS float64        `json:"duration_s"`
}

type batchFailure struct {
	Item  string `json:"item"`
	Error string `json:"error"`
}

// runFrameworkEntry is the first call in main. It answers the framework's own
// argv (`--version`, `--print-contract`) and runs batch mode when there are no
// arguments; it returns only when argv carries the tool's own flags, i.e. the
// debug pass-through mode, which main then handles as before.
func runFrameworkEntry(t batchTool) {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runBatch(t, os.Getenv, os.Stdout))
	}
	if len(args) == 1 {
		switch strings.TrimLeft(args[0], "-") {
		case "version":
			fmt.Printf("%s %s\n", t.name, version)
			os.Exit(0)
		case "print-contract":
			io.WriteString(os.Stdout, contractYML)
			os.Exit(0)
		}
	}
}

// env reads a tool-specific variable (<PREFIX>_<suffix>), falling back to def
// when unset or empty.
func (c *batchConfig) env(suffix, def string) string {
	if v := c.getenv(c.Prefix + "_" + suffix); v != "" {
		return v
	}
	return def
}

// logf prints a `<tool>: ...` line on stderr when level is enabled.
func (c *batchConfig) logf(level int, format string, args ...interface{}) {
	if level > c.LogLevel {
		return
	}
	fmt.Fprintf(os.Stderr, c.Tool+": "+format+"\n", args...)
}

// quiet reports whether per-item progress lines are suppressed (the tools'
// argv-mode -q equivalent).
func (c *batchConfig) quiet() bool { return c.LogLevel < logInfo }

// batchPrefix derives the SCREAMING_SNAKE_CASE variable prefix from a tool name.
func batchPrefix(tool string) string {
	return strings.ToUpper(strings.ReplaceAll(tool, "-", "_"))
}

// parseBool accepts the framework's boolean spellings: 1/true/yes/on and
// 0/false/no/off (case-insensitive; empty is false).
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "", "0", "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("not a boolean (1/true/yes/on or 0/false/no/off): %q", s)
}

// formatExt maps a <TOOL>_FORMAT value to the record file extension.
func formatExt(format string) string {
	if format == "csv" {
		return "csv"
	}
	return "jsonl"
}

// loadBatchConfig resolves the environment contract and validates the mounts.
// Every failure is a config error (exit 2).
func loadBatchConfig(t batchTool, getenv func(string) string) (*batchConfig, error) {
	cfg := &batchConfig{Tool: t.fileBase(), Prefix: t.envPrefix(), getenv: getenv}
	cfg.InputDir = cfg.env("INPUT_DIR", "/input")
	cfg.OutDir = cfg.env("OUT_DIR", "/output")
	cfg.WorkDir = cfg.env("WORK_DIR", "/work")

	force, err := parseBool(cfg.env("FORCE", "0"))
	if err != nil {
		return nil, fmt.Errorf("%s_FORCE: %w", cfg.Prefix, err)
	}
	cfg.Force = force

	lvlName := strings.ToLower(cfg.env("LOG_LEVEL", "info"))
	lvl, ok := logLevels[lvlName]
	if !ok {
		return nil, fmt.Errorf("%s_LOG_LEVEL: %q is not one of error|warn|info|debug", cfg.Prefix, lvlName)
	}
	cfg.LogLevel = lvl

	cfg.Format = strings.ToLower(cfg.env("FORMAT", t.formats[0]))
	valid := false
	for _, f := range t.formats {
		if f == cfg.Format {
			valid = true
		}
	}
	if !valid {
		return nil, fmt.Errorf("%s_FORMAT: %q is not one of %s", cfg.Prefix, cfg.Format, strings.Join(t.formats, "|"))
	}

	st, err := os.Stat(cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("%s_INPUT_DIR %s: %w", cfg.Prefix, cfg.InputDir, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s_INPUT_DIR %s: not a directory", cfg.Prefix, cfg.InputDir)
	}
	if err := ensureWritableDir(cfg.OutDir); err != nil {
		return nil, fmt.Errorf("%s_OUT_DIR %s: %w", cfg.Prefix, cfg.OutDir, err)
	}
	// The work dir is optional scratch: when it cannot be used the run carries
	// on with the process temp dir rather than failing.
	if err := ensureWritableDir(cfg.WorkDir); err != nil {
		cfg.logf(logWarn, "work dir %s not usable (%v); using %s", cfg.WorkDir, err, os.TempDir())
		cfg.WorkDir = os.TempDir()
	}
	return cfg, nil
}

// ensureWritableDir creates dir if needed and proves it is writable by
// creating and removing a probe file.
func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return fmt.Errorf("not writable: %w", err)
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

// batchItemName derives the per-item output folder name from the item's path
// relative to the input root: path separators, whitespace and ':' fold to '_';
// every other character (including '$') is kept. An over-long name is
// truncated and suffixed with a short hash so it stays unique.
func batchItemName(root, item string) string {
	rel, err := filepath.Rel(root, item)
	if err != nil || rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(item)
	}
	rel = filepath.ToSlash(rel)
	var b strings.Builder
	for _, r := range rel {
		if r == '/' || r == '\\' || r == ':' || unicode.IsSpace(r) || r == 0 {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" || name == "." || name == ".." {
		name = "item"
	}
	const maxName = 200
	if len(name) > maxName {
		h := fnv.New32a()
		h.Write([]byte(name))
		name = fmt.Sprintf("%s-%08x", name[:maxName-9], h.Sum32())
	}
	return name
}

// runBatch is the batch-mode main loop. getenv and stdout are injected so the
// runtime is testable; the summary line is the only thing written to stdout.
func runBatch(t batchTool, getenv func(string) string, stdout io.Writer) int {
	started := time.Now()
	sum := &batchSummary{
		Tool:    t.name,
		Subtool: t.subtool,
		Version: version,
		Outputs: []string{},
		Started: started.UTC().Format(time.RFC3339),
	}
	finish := func(status string, code int) int {
		sum.Status, sum.Exit = status, code
		sum.DurationS = float64(int64(time.Since(started).Seconds()*1000)) / 1000
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(sum); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write summary: %v\n", t.name, err)
			return 2
		}
		return code
	}

	cfg, err := loadBatchConfig(t, getenv)
	if err != nil {
		sum.Error = err.Error()
		fmt.Fprintf(os.Stderr, "%s: config error: %v\n", t.name, err)
		return finish("config_error", 2)
	}

	// stdout carries the summary line only: it is muted while items are
	// processed so a library that prints to stdout cannot corrupt it. The
	// summary goes to the writer captured above, not the package variable.
	if devnull, derr := os.OpenFile(os.DevNull, os.O_WRONLY, 0); derr == nil {
		saved := os.Stdout
		os.Stdout = devnull
		defer func() { os.Stdout = saved; devnull.Close() }()
	}

	items, err := t.discover(cfg)
	if err != nil {
		sum.Error = "discover: " + err.Error()
		cfg.logf(logError, "config error: %v", err)
		return finish("config_error", 2)
	}
	sum.Inputs = len(items)
	if len(items) == 0 {
		cfg.logf(logWarn, "no inputs found under %s", cfg.InputDir)
		return finish("nothing", 1)
	}

	ext := formatExt(cfg.Format)
	for _, item := range items {
		itemDir := filepath.Join(cfg.OutDir, batchItemName(cfg.InputDir, item))
		final := filepath.Join(itemDir, t.fileBase()+"."+ext)
		if !cfg.Force {
			if st, serr := os.Stat(final); serr == nil && st.Mode().IsRegular() {
				sum.Skipped++
				sum.Outputs = append(sum.Outputs, itemDir)
				cfg.logf(logInfo, "skip %s (output exists: %s)", item, final)
				continue
			}
		}
		n, perr := runItem(t, cfg, item, itemDir, final)
		if perr != nil {
			sum.Failed++
			sum.Failures = append(sum.Failures, batchFailure{Item: item, Error: perr.Error()})
			cfg.logf(logError, "FAILED %s: %v", item, perr)
			continue
		}
		sum.Processed++
		sum.Records += n
		sum.Outputs = append(sum.Outputs, itemDir)
		cfg.logf(logInfo, "processed %s -> %s (%d records)", item, final, n)
	}
	cfg.logf(logInfo, "%d inputs: %d processed, %d skipped, %d failed, %d records",
		sum.Inputs, sum.Processed, sum.Skipped, sum.Failed, sum.Records)

	switch {
	case sum.Failed > 0 && sum.Processed+sum.Skipped == 0:
		return finish("nothing", 1)
	case sum.Failed > 0:
		return finish("partial", 3)
	}
	return finish("ok", 0)
}

// runItem processes one item into itemDir. The record file is written as
// <final>.part and renamed into place only when process succeeds, so a partial
// or failed run never leaves a file that a later run would take as valid
// output. Under FORCE any previous record file is removed first.
func runItem(t batchTool, cfg *batchConfig, item, itemDir, final string) (int, error) {
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return 0, err
	}
	if cfg.Force {
		if err := os.Remove(final); err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
	}
	part := final + ".part"
	f, err := os.Create(part)
	if err != nil {
		return 0, err
	}
	n, perr := t.process(cfg, item, itemDir, f)
	if cerr := f.Close(); perr == nil {
		perr = cerr
	}
	if perr != nil {
		os.Remove(part)
		return n, perr
	}
	if err := os.Rename(part, final); err != nil {
		os.Remove(part)
		return n, err
	}
	return n, nil
}

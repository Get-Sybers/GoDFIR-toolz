// zeek-run — the env-driven batch entrypoint of get-sybers/zeek.
//
// Zeek has no batch mode of its own: one invocation parses one capture into
// the current directory. This static Go entrypoint gives the image the
// framework's self-orchestration (docs/framework 03/04). With no arguments it
// reads ZEEK_INPUT_DIR / ZEEK_OUT_DIR / ZEEK_FORCE / ZEEK_FORMAT / ZEEK_SCRIPTS,
// finds every capture under the input tree (by .pcap/.pcapng/.cap extension or
// pcap/pcapng magic), runs zeek once per capture inside that capture's own
// output folder, and prints one JSON summary line. With arguments it execs the
// zeek binary with that argv verbatim — the debug pass-through — so
// `… get-sybers/zeek -C -r /pcap/x.pcap …` behaves exactly like calling zeek.
//
// Per capture the folder holds zeek's logs — as *.json when ZEEK_FORMAT=json
// (LogAscii JSON with ISO-8601 timestamps, each *.log renamed to *.json), or
// as zeek's default tab-separated *.log when ZEEK_FORMAT=tsv — plus zeek.jsonl,
// the index of produced logs and their record counts, which marks the item
// done.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// zeekBinary is the zeek executable the runner drives; a test points it at a
// stand-in.
var zeekBinary = "/opt/zeek/bin/zeek"

// zeekTool binds the shared batch runtime to zeek.
var zeekTool = batchTool{
	name:     "zeek",
	formats:  []string{"json", "tsv"},
	discover: batchDiscover,
	process:  batchProcess,
}

// captureExts are the capture file extensions recognised by name.
var captureExts = map[string]bool{".pcap": true, ".pcapng": true, ".cap": true}

// captureMagics are the pcap (both byte orders, microsecond and nanosecond)
// and pcapng file signatures.
var captureMagics = []string{
	"\xa1\xb2\xc3\xd4", "\xd4\xc3\xb2\xa1", "\xa1\xb2\x3c\x4d", "\x4d\x3c\xb2\xa1", "\x0a\x0d\x0d\x0a",
}

// looksLikeCapture reports whether p begins with a pcap or pcapng magic number.
func looksLikeCapture(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	for _, m := range captureMagics {
		if string(hdr[:]) == m {
			return true
		}
	}
	return false
}

// isCapture selects an item by extension or content.
func isCapture(p string) bool {
	return captureExts[strings.ToLower(filepath.Ext(p))] || looksLikeCapture(p)
}

// batchDiscover walks the input tree and keeps every capture. Unreadable
// subtrees are noted and skipped; an unreadable root is an error.
func batchDiscover(cfg *batchConfig) ([]string, error) {
	var items []string
	err := filepath.WalkDir(cfg.InputDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == cfg.InputDir {
				return err
			}
			cfg.logf(logWarn, "skipping unreadable %s: %v", p, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() && isCapture(p) {
			items = append(items, p)
		}
		return nil
	})
	sort.Strings(items)
	return items, err
}

// zeekArgs is the zeek argv for one capture: checksums ignored (captures are
// routinely offloaded), JSON logging when asked for, then the operator's extra
// scripts and option assignments from ZEEK_SCRIPTS.
func zeekArgs(cfg *batchConfig, item string) []string {
	args := []string{"-C", "-r", item}
	if cfg.Format == "json" {
		args = append(args, "LogAscii::use_json=T", "LogAscii::json_timestamps=JSON::TS_ISO8601")
	}
	return append(args, strings.Fields(cfg.env("SCRIPTS", ""))...)
}

// logIndexEntry is one line of zeek.jsonl: a produced log and its record count.
type logIndexEntry struct {
	Log     string `json:"log"`
	Records int    `json:"records"`
}

// countRecords counts a zeek log's record lines (everything that is not a
// `#` header/footer line).
func countRecords(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			n++
		}
	}
	return n, nil
}

// batchProcess runs zeek over one capture inside itemDir, renames the logs to
// *.json in JSON mode, and writes the log index to w. Logs from an earlier run
// are removed first so a forced rerun never mixes old and new output.
func batchProcess(cfg *batchConfig, item, itemDir string, w io.Writer) (int, error) {
	for _, pattern := range []string{"*.log", "*.json"} {
		old, _ := filepath.Glob(filepath.Join(itemDir, pattern))
		for _, p := range old {
			os.Remove(p)
		}
	}
	cmd := exec.Command(zeekBinary, zeekArgs(cfg, item)...)
	cmd.Dir = itemDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("zeek: %w", err)
	}
	logs, _ := filepath.Glob(filepath.Join(itemDir, "*.log"))
	sort.Strings(logs)
	if len(logs) == 0 {
		return 0, fmt.Errorf("zeek produced no logs")
	}
	enc := json.NewEncoder(w)
	total := 0
	for _, p := range logs {
		n, err := countRecords(p)
		if err != nil {
			return total, err
		}
		final := p
		if cfg.Format == "json" {
			final = strings.TrimSuffix(p, ".log") + ".json"
			if err := os.Rename(p, final); err != nil {
				return total, err
			}
		}
		if err := enc.Encode(logIndexEntry{Log: filepath.Base(final), Records: n}); err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func main() {
	runFrameworkEntry(zeekTool)
	// Any argument is zeek's own: hand the argv to the binary unchanged.
	argv := append([]string{"zeek"}, os.Args[1:]...)
	if err := syscall.Exec(zeekBinary, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "zeek-run: exec %s: %v\n", zeekBinary, err)
		os.Exit(1)
	}
}

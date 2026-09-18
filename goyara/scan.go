// goyara core — the go-yara tar-stream scanner.
//
// goyara is the FIRST CONSUMER of gomount: it reads gomount's userspace TAR
// stream on stdin and scans each file's bytes with compiled YARA rules. It does
// NO disk or NTFS parsing itself — gomount owns all of that, walking the NTFS
// volume in-process and emitting one tar entry per regular file (entry name =
// the file's path, mode 0400, Size = the file's logical length). goyara only
// consumes that tar and reports one JSON record per rule hit.
//
// The scan runs entry-by-entry: each file is read (bounded to a cap) and scanned
// with libyara's ScanMem, so memory stays bounded regardless of image size — the
// whole tar is never buffered. A per-file scan failure is counted and skipped;
// the stream keeps going.
package main

import (
	"archive/tar"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
	"unicode/utf8"

	yara "github.com/hillu/go-yara/v4"
)

// DefaultMaxBytes caps how many bytes of any one file are scanned. A file larger
// than the cap is scanned only up to the cap (its head) — the bound keeps memory
// per entry fixed no matter how large the file on the volume is.
const DefaultMaxBytes int64 = 32 << 20 // 32 MiB

// DefaultTimeout bounds a single ScanMem call so one pathological file cannot
// wedge the whole stream.
const DefaultTimeout = 60 * time.Second

// matchString is one YARA string hit inside a matched file: the string's
// identifier ($a), its absolute offset in the scanned bytes, and the matched
// data rendered as UTF-8 when it is printable text, else as hex.
type matchString struct {
	Name   string `json:"name"`
	Offset uint64 `json:"offset"`
	Data   string `json:"data"`
}

// match is the emitted record — one per (file, rule) hit. Shape:
//
//	{"tool":"yara","source":"disk","rule":..,"namespace":..,"target":<path>,
//	 "tags":[..],"strings":[{"name":..,"offset":..,"data":..}]}
//
// target is the tar entry name, which is the file's path on the scanned volume
// (gomount emits it without a leading slash, e.g. "Windows/System32/x.dll").
type match struct {
	Tool      string        `json:"tool"`
	Source    string        `json:"source"`
	Rule      string        `json:"rule"`
	Namespace string        `json:"namespace"`
	Target    string        `json:"target"`
	Tags      []string      `json:"tags"`
	Strings   []matchString `json:"strings"`
}

// Options tunes a scan. The zero value is valid: MaxBytes and Timeout fall back
// to their defaults, and Source defaults to "disk" (the tar came from a disk
// image walked by gomount).
type Options struct {
	MaxBytes int64         // per-file scan cap; <= 0 uses DefaultMaxBytes
	Timeout  time.Duration // per-file scan timeout; <= 0 uses DefaultTimeout
	Source   string        // the "source" field on every record; "" uses "disk"
}

func (o Options) maxBytes() int64 {
	if o.MaxBytes > 0 {
		return o.MaxBytes
	}
	return DefaultMaxBytes
}

func (o Options) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return DefaultTimeout
}

func (o Options) source() string {
	if o.Source != "" {
		return o.Source
	}
	return "disk"
}

// LoadRules compiles a .yar rules file into a scannable ruleset. The file may use
// `include` directives (the DetectRaptor set is one detectraptor.yar that
// includes many rule files); includes resolve relative to the file's own
// directory, so the whole set compiles from the single top-level path. A compile
// error is returned with the offending file:line messages libyara reported.
func LoadRules(path string) (*yara.Rules, error) {
	c, err := yara.NewCompiler()
	if err != nil {
		return nil, fmt.Errorf("new yara compiler: %w", err)
	}
	defer c.Destroy()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// AddFile (not AddString) so libyara anchors `include` resolution to the
	// rules file's directory rather than the process working directory.
	if err := c.AddFile(f, ""); err != nil {
		return nil, fmt.Errorf("compile %s: %w", path, compileErr(c, err))
	}
	rules, err := c.GetRules()
	if err != nil {
		return nil, fmt.Errorf("finalize rules from %s: %w", path, err)
	}
	return rules, nil
}

// compileErr enriches a compiler error with the concrete file:line messages
// libyara collected, so a bad rule points at itself rather than a bare "syntax
// error".
func compileErr(c *yara.Compiler, err error) error {
	if len(c.Errors) == 0 {
		return err
	}
	msg := err.Error()
	for _, m := range c.Errors {
		msg += fmt.Sprintf("\n  %s:%d: %s", m.Filename, m.Line, m.Text)
	}
	return fmt.Errorf("%s", msg)
}

// ScanTarStream reads a TAR archive from r and scans each regular-file entry with
// rules, calling emit once per rule hit. It returns the number of files scanned
// and the number of per-file errors (a scan or read failure on one file is
// counted and skipped — the stream keeps going). err is non-nil only for a fatal
// condition: a corrupt tar, or an emit callback that itself failed.
//
// Memory is bounded: one entry is read and scanned at a time, and each read is
// capped at opts.MaxBytes (default 32 MiB), so the whole tar is never buffered.
func ScanTarStream(rules *yara.Rules, r io.Reader, emit func(match) error, opts Options) (scanned int, errs int, err error) {
	tr := tar.NewReader(r)
	cap := opts.maxBytes()
	timeout := opts.timeout()
	source := opts.source()

	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			// A malformed tar desynchronises every entry after it — fatal.
			return scanned, errs, fmt.Errorf("read tar: %w", nextErr)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue // only regular files carry bytes to scan
		}

		// Read this one entry, bounded to the cap. tr.Next() on the following
		// iteration skips any unread remainder of an over-cap file for us.
		buf, readErr := io.ReadAll(io.LimitReader(tr, cap))
		if readErr != nil {
			errs++
			fmt.Fprintf(os.Stderr, "goyara: %s: read: %v\n", hdr.Name, readErr)
			continue
		}

		var matches yara.MatchRules
		if scanErr := rules.ScanMem(buf, 0, timeout, &matches); scanErr != nil {
			errs++
			fmt.Fprintf(os.Stderr, "goyara: %s: scan: %v\n", hdr.Name, scanErr)
			continue
		}
		scanned++

		for _, m := range matches {
			if emitErr := emit(record(m, hdr.Name, source)); emitErr != nil {
				return scanned, errs, emitErr
			}
		}
	}
	return scanned, errs, nil
}

// record builds the emitted match from a libyara MatchRule and the tar entry
// name it hit in.
func record(m yara.MatchRule, target, source string) match {
	strs := make([]matchString, 0, len(m.Strings))
	for _, s := range m.Strings {
		strs = append(strs, matchString{
			Name:   s.Name,
			Offset: s.Base + s.Offset, // absolute offset in the scanned bytes
			Data:   renderData(s.Data),
		})
	}
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	return match{
		Tool:      "yara",
		Source:    source,
		Rule:      m.Rule,
		Namespace: m.Namespace,
		Target:    target,
		Tags:      tags,
		Strings:   strs,
	}
}

// renderData shows matched bytes as text when they are valid, printable UTF-8,
// and as hex otherwise — so an ASCII IOC reads plainly while a binary signature
// stays unambiguous.
func renderData(b []byte) string {
	if len(b) > 0 && utf8.Valid(b) && isPrintable(b) {
		return string(b)
	}
	return hex.EncodeToString(b)
}

func isPrintable(b []byte) bool {
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}
		// Reject C0/C1 control bytes (tab/newline/CR included: a matched IOC is
		// a single token, not multi-line) so only genuine text renders as text.
		if r < 0x20 || (r >= 0x7f && r < 0xa0) {
			return false
		}
		i += size
	}
	return true
}

// gosyslog — syslog-family text log parser for the DX_DFIR pipeline
// (docs/linux §4). Finds the classic text logs under the input tree —
// syslog, messages, auth.log, secure, kern.log, cron, daemon.log, mail.log,
// user.log, debug — rotations and gzip included, and emits one record per
// line with the timestamp dialects normalised (RFC3164 yearless with
// mtime-anchored year inference, ISO-8601, RFC5424).
//
// Rule-1/2 alignment (docs/linux §4.1): known high-value line families are
// TYPED BY THE PARSER — sshd authentication lines, sudo command lines, pam
// session open/close, and cron job lines each get their own RecordType with
// parsed fields — because byakugan's maps select rows with predicates over
// typed fields and never regex raw messages. The raw line always rides
// along; everything else stays a plain syslog_line. Yearless timestamps are
// recorded naive-as-UTC (the imaged host's zone is byakugan-side context).
//
// With no arguments the binary runs the container-framework batch mode
// (pinfo/batch) under the GOSYSLOG_* environment. The argv flags are the
// debug pass-through:
//
//	gosyslog -f FILE | -d DIR [-q]
package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/batch"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/discover"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/record"
	"github.com/Get-Sybers/GoDFIR-toolz/pinfo/tstamp"
)

//go:embed contract.yml
var contractYML string

var version = "0.0.0-dev"

// syslogRecord is one log line; RecordType is syslog_line or a typed
// family (sshd_event, sudo_event, pam_session, cron_event).
type syslogRecord struct {
	record.Envelope
	Host    string `json:"Host,omitempty"`
	Ident   string `json:"Ident,omitempty"`
	PID     *int64 `json:"PID,omitempty"`
	Message string `json:"Message"`
	// typed families
	SSHEvent    string `json:"SSHEvent,omitempty"`
	Method      string `json:"Method,omitempty"`
	Username    string `json:"Username,omitempty"`
	TargetUser  string `json:"TargetUser,omitempty"`
	InvalidUser bool   `json:"InvalidUser,omitempty"`
	IPAddress   string `json:"IPAddress,omitempty"`
	Port        *int64 `json:"Port,omitempty"`
	KeyType     string `json:"KeyType,omitempty"`
	Fingerprint string `json:"Fingerprint,omitempty"`
	TTY         string `json:"TTY,omitempty"`
	PWD         string `json:"PWD,omitempty"`
	Command     string `json:"Command,omitempty"`
	PamModule   string `json:"PamModule,omitempty"`
	SessionOp   string `json:"SessionOp,omitempty"`
	ByUser      string `json:"ByUser,omitempty"`
	ByUID       *int64 `json:"ByUID,omitempty"`
	Line        int    `json:"Line"`
	Raw         string `json:"Raw"`
}

// ---- discovery -------------------------------------------------------------

var logBases = []string{
	"syslog", "messages", "auth.log", "secure", "kern.log", "cron",
	"cron.log", "daemon.log", "mail.log", "user.log", "debug", "boot.log",
}

func isSyslogFile(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	for _, b := range logBases {
		if discover.Rotated(base, b) {
			return true
		}
	}
	return false
}

// ---- line prefix parsing ---------------------------------------------------

var (
	// "Mar  1 22:14:02 host tail" (RFC3164, no year)
	bsdRe = regexp.MustCompile(`^([A-Z][a-z]{2} [ 0-9]\d \d{2}:\d{2}:\d{2})\s+(\S+)\s+(.*)$`)
	// "2026-03-01T22:14:02.123456+02:00 host tail" (ISO-8601 prefix)
	isoRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+(\S+)\s+(.*)$`)
	// "<13>1 2026-03-01T22:14:02Z host app pid msgid [sd] msg" (RFC5424)
	r5424Re = regexp.MustCompile(`^<\d{1,3}>\d\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+\S+\s+(?:\[[^\]]*\]|-)\s*(.*)$`)
	identRe = regexp.MustCompile(`^([^\s:\[\]]+)(?:\[(\d+)\])?:\s?(.*)$`)
)

// splitIdent splits "ident[pid]: message" off the line tail.
func splitIdent(tail string, rec *syslogRecord) {
	if m := identRe.FindStringSubmatch(tail); m != nil {
		rec.Ident = m[1]
		if m[2] != "" {
			if n, err := strconv.ParseInt(m[2], 10, 64); err == nil {
				rec.PID = &n
			}
		}
		rec.Message = m[3]
		return
	}
	rec.Message = tail
}

func dash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

// parseLine fills one record from a raw line; ref anchors yearless stamps.
func parseLine(line string, ref time.Time, rec *syslogRecord) {
	rec.Raw = line
	switch {
	case r5424Re.MatchString(line):
		m := r5424Re.FindStringSubmatch(line)
		if t, ok := tstamp.Flexible(m[1]); ok {
			rec.EventTime = tstamp.ISO8601(t)
			rec.TimeKind = "event"
		}
		rec.Host = dash(m[2])
		rec.Ident = dash(m[3])
		if m[4] != "-" {
			if n, err := strconv.ParseInt(m[4], 10, 64); err == nil {
				rec.PID = &n
			}
		}
		rec.Message = m[5]
	case isoRe.MatchString(line):
		m := isoRe.FindStringSubmatch(line)
		if t, ok := tstamp.Flexible(m[1]); ok {
			rec.EventTime = tstamp.ISO8601(t)
			rec.TimeKind = "event"
		}
		rec.Host = m[2]
		splitIdent(m[3], rec)
	case bsdRe.MatchString(line):
		m := bsdRe.FindStringSubmatch(line)
		if t, ok := tstamp.Syslog3164(m[1], ref); ok {
			rec.EventTime = tstamp.ISO8601(t)
			rec.TimeKind = "event"
		}
		rec.Host = m[2]
		splitIdent(m[3], rec)
	default:
		rec.Message = line // continuation or free-form line: kept, untyped
	}
	rec.RecordType = "syslog_line"
	typeLine(rec)
}

// ---- typed families --------------------------------------------------------

var (
	sshAuthRe    = regexp.MustCompile(`^(Accepted|Failed) (\S+) for (invalid user )?(.+?) from (\S+) port (\d+)(?: ssh2)?(?::\s+(\S+)\s+(\S+))?\s*$`)
	sshInvalidRe = regexp.MustCompile(`^Invalid user (\S+) from (\S+)(?: port (\d+))?`)
	pamRe        = regexp.MustCompile(`^(pam_unix|pam_[a-z0-9_]+)\(([^)]+)\): session (opened|closed) for user ([^( ]+)(?:\(uid=(\d+)\))?(?: by (?:([^( ]+))?\(uid=(\d+)\))?`)
	sudoRe       = regexp.MustCompile(`^\s*(\S+) : (?:.*?;\s*)?TTY=(\S+)\s*;\s*PWD=(.*?)\s*;\s*USER=(\S+)\s*;\s*(?:ENV=\S+\s*;\s*)?COMMAND=(.*)$`)
	cronCmdRe    = regexp.MustCompile(`^\((\S+)\) CMD \((.*)\)\s*$`)
)

// typeLine promotes a parsed line into its typed family when it is one
// (docs/linux §4.1 rule 1); the raw line and the generic fields remain.
func typeLine(rec *syslogRecord) {
	msg := rec.Message
	switch {
	case rec.Ident == "sshd" || strings.HasPrefix(rec.Ident, "sshd"):
		if m := sshAuthRe.FindStringSubmatch(msg); m != nil {
			rec.RecordType = "sshd_event"
			if m[1] == "Accepted" {
				rec.SSHEvent = "accepted"
			} else {
				rec.SSHEvent = "failed"
			}
			rec.Method = m[2]
			rec.InvalidUser = m[3] != ""
			rec.Username = m[4]
			rec.IPAddress = m[5]
			if n, err := strconv.ParseInt(m[6], 10, 64); err == nil {
				rec.Port = &n
			}
			rec.KeyType, rec.Fingerprint = m[7], m[8]
			return
		}
		if m := sshInvalidRe.FindStringSubmatch(msg); m != nil {
			rec.RecordType = "sshd_event"
			rec.SSHEvent = "invalid_user"
			rec.InvalidUser = true
			rec.Username = m[1]
			rec.IPAddress = m[2]
			if m[3] != "" {
				if n, err := strconv.ParseInt(m[3], 10, 64); err == nil {
					rec.Port = &n
				}
			}
			return
		}
	case rec.Ident == "sudo":
		if m := sudoRe.FindStringSubmatch(msg); m != nil {
			rec.RecordType = "sudo_event"
			rec.Username = m[1]
			rec.TTY = m[2]
			rec.PWD = m[3]
			rec.TargetUser = m[4]
			rec.Command = m[5]
			return
		}
	case rec.Ident == "CRON" || rec.Ident == "crond" || rec.Ident == "cron":
		if m := cronCmdRe.FindStringSubmatch(msg); m != nil {
			rec.RecordType = "cron_event"
			rec.Username = m[1]
			rec.Command = m[2]
			return
		}
	}
	if m := pamRe.FindStringSubmatch(msg); m != nil {
		rec.RecordType = "pam_session"
		rec.PamModule = m[2]
		rec.SessionOp = m[3]
		rec.Username = m[4]
		rec.ByUser = m[6]
		uidStr := m[7]
		if uidStr == "" {
			uidStr = m[5]
		}
		if uidStr != "" {
			if n, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
				rec.ByUID = &n
			}
		}
	}
}

// parseLog emits one record per line of one log stream.
func parseLog(rd io.Reader, ref time.Time, w *record.Writer) (int, error) {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	emitted, lineNo := 0, 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		rec := &syslogRecord{Line: lineNo}
		parseLine(line, ref, rec)
		if err := w.Write(rec); err != nil {
			return emitted, err
		}
		emitted++
	}
	return emitted, sc.Err()
}

// ---- batch binding ---------------------------------------------------------

var gosyslogTool = batch.Tool{
	Name: "gosyslog",
	Discover: func(cfg *batch.Config) ([]string, error) {
		return discover.Files(cfg.InputDir, func(rel string, d fs.DirEntry) bool {
			return isSyslogFile(rel)
		})
	},
	Process: func(cfg *batch.Config, item, _ string, w *record.Writer) (int, error) {
		f, err := discover.OpenAuto(item)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		ref := time.Now().UTC()
		if st, err := os.Stat(item); err == nil {
			ref = st.ModTime().UTC()
		}
		return parseLog(f, ref, w)
	},
}

func main() {
	batch.Entry(gosyslogTool, batch.Options{Version: version, Contract: contractYML})

	var (
		file  = flag.String("f", "", "parse one log file")
		dir   = flag.String("d", "", "recurse a directory for syslog-family files")
		quiet = flag.Bool("q", false, "suppress warnings on stderr")
	)
	flag.Parse()
	if (*file == "") == (*dir == "") || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: gosyslog                           (env-driven batch mode)\n"+
			"       gosyslog -f FILE | -d DIR [-q]")
		os.Exit(1)
	}
	w := record.NewWriter(os.Stdout)
	failed := 0
	one := func(path, rel string) {
		f, err := discover.OpenAuto(path)
		if err == nil {
			ref := time.Now().UTC()
			st, _ := os.Stat(path)
			s := record.Stamp{Tool: "gosyslog", ToolVersion: version, SourceFilename: rel}
			if st != nil {
				ref = st.ModTime().UTC()
				s.SourceModified = tstamp.ISO8601(st.ModTime())
			}
			w.SetStamp(s)
			_, err = parseLog(f, ref, w)
			f.Close()
		}
		if err != nil {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "gosyslog: %s: %v\n", path, err)
			}
			failed++
		}
	}
	if *file != "" {
		one(*file, *file)
	} else {
		items, err := discover.Files(*dir, func(rel string, d fs.DirEntry) bool { return isSyslogFile(rel) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "gosyslog: %v\n", err)
			os.Exit(1)
		}
		for _, it := range items {
			rel, rerr := filepath.Rel(*dir, it)
			if rerr != nil {
				rel = it
			}
			one(it, filepath.ToSlash(rel))
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "gosyslog: %v\n", err)
		os.Exit(1)
	}
	if failed > 0 {
		os.Exit(2)
	}
}

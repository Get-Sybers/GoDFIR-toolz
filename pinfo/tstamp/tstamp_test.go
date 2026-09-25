package tstamp

import (
	"testing"
	"time"
)

func TestISO8601AndUnix(t *testing.T) {
	if got := ISO8601(time.Time{}); got != "" {
		t.Fatalf("zero: %q", got)
	}
	if got := Unix(0, 0); got != "" {
		t.Fatalf("zero unix: %q", got)
	}
	if got := Unix(1767225600, 0); got != "2026-01-01T00:00:00.000000Z" {
		t.Fatalf("unix: %q", got)
	}
	if got := Unix(1767225600, 481000000); got != "2026-01-01T00:00:00.481000Z" {
		t.Fatalf("unix nsec: %q", got)
	}
	if got := UnixMicros(1767225600481000); got != "2026-01-01T00:00:00.481000Z" {
		t.Fatalf("micros: %q", got)
	}
	if got := Days(20454); got != "2026-01-01T00:00:00.000000Z" {
		t.Fatalf("days: %q", got)
	}
	if got := Days(0); got != "" {
		t.Fatalf("zero days: %q", got)
	}
}

func TestSyslog3164YearInference(t *testing.T) {
	ref := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	got, ok := Syslog3164("Mar  1 22:14:02", ref)
	if !ok || got != time.Date(2026, 3, 1, 22, 14, 2, 0, time.UTC) {
		t.Fatalf("same year: %v %v", got, ok)
	}
	// December line read against a January mtime: the year walks back.
	ref = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got, ok = Syslog3164("Dec 31 23:59:59", ref)
	if !ok || got.Year() != 2025 {
		t.Fatalf("rollover: %v %v", got, ok)
	}
	// A line a few hours ahead of the reference stays in the same year.
	got, ok = Syslog3164("Jan  2 06:00:00", ref)
	if !ok || got.Year() != 2026 {
		t.Fatalf("ahead-of-ref: %v %v", got, ok)
	}
	if _, ok := Syslog3164("not a date", ref); ok {
		t.Fatal("garbage accepted")
	}
}

func TestFlexible(t *testing.T) {
	cases := map[string]string{
		"2026-03-01T22:14:02.481Z":      "2026-03-01T22:14:02.481000Z",
		"2026-03-01T22:14:02+02:00":     "2026-03-01T20:14:02.000000Z",
		"2026-03-01 22:14:02":           "2026-03-01T22:14:02.000000Z",
		"2026-03-01T22:14:02.123456":    "2026-03-01T22:14:02.123456Z",
		"2026-03-01 22:14:02.123+01:00": "2026-03-01T21:14:02.123000Z",
	}
	for in, want := range cases {
		got, ok := Flexible(in)
		if !ok || ISO8601(got) != want {
			t.Errorf("Flexible(%q) = %q ok=%v, want %q", in, ISO8601(got), ok, want)
		}
	}
	if _, ok := Flexible("Mar 1 22:14:02"); ok {
		t.Fatal("yearless form must not be Flexible")
	}
}

//go:build ignore

// gen.go writes the gocron contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := os.Args[1]
	os.MkdirAll(filepath.Join(d, "etc/cron.d"), 0o755)
	os.MkdirAll(filepath.Join(d, "var/spool/cron/crontabs"), 0o755)
	os.WriteFile(filepath.Join(d, "etc/crontab"),
		[]byte("SHELL=/bin/sh\n17 * * * * root run-parts /etc/cron.hourly\n"), 0o644)
	os.WriteFile(filepath.Join(d, "etc/cron.d/backup"),
		[]byte("0 3 * * * root /usr/local/bin/backup.sh\n"), 0o644)
	os.WriteFile(filepath.Join(d, "var/spool/cron/crontabs/alice"),
		[]byte("*/5 * * * * /home/alice/sync.sh\n"), 0o644)
}

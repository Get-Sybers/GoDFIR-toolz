//go:build ignore

// gen.go writes the gosyslog contract-test fixtures into argv[1].
package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "var/log")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "syslog"), []byte(
		"Mar  1 22:14:02 web01 systemd[1]: Started daily apt activities.\n"+
			"Mar  1 22:15:01 web01 CRON[1002]: (alice) CMD (/home/alice/bin/sync.sh)\n"+
			"Mar  1 22:16:00 web01 sudo:    alice : TTY=pts/0 ; PWD=/home/alice ; USER=root ; COMMAND=/usr/bin/systemctl status nginx\n"), 0o644)
	f, _ := os.Create(filepath.Join(d, "auth.log.1.gz"))
	zw := gzip.NewWriter(f)
	zw.Write([]byte(
		"2026-03-01T22:14:03.000000+00:00 web01 sshd[901]: Server listening on 0.0.0.0 port 22.\n" +
			"2026-03-01T22:14:05.000000+00:00 web01 sudo: pam_unix(sudo:session): session opened for user root(uid=0) by alice(uid=1000)\n"))
	zw.Close()
	f.Close()
}

//go:build ignore

// gen.go writes the gounit contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "etc/systemd/system")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "legit.service"),
		[]byte("[Unit]\nDescription=Legit\n[Service]\nExecStart=/usr/local/bin/legit\nUser=root\n[Install]\nWantedBy=multi-user.target\n"), 0o644)
	os.WriteFile(filepath.Join(d, "beacon.timer"),
		[]byte("[Timer]\nOnCalendar=*-*-* 03:14:00\nPersistent=true\n"), 0o644)
}

//go:build ignore

// gen.go writes the gonetwork contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "etc")
	os.MkdirAll(filepath.Join(d, "network"), 0o755)
	os.WriteFile(filepath.Join(d, "hosts"), []byte("127.0.0.1 localhost\n192.0.2.10 intranet.example\n"), 0o644)
	os.WriteFile(filepath.Join(d, "resolv.conf"), []byte("nameserver 192.0.2.53\nsearch example.internal\n"), 0o644)
	os.WriteFile(filepath.Join(d, "network", "interfaces"), []byte("auto eth0\niface eth0 inet dhcp\n"), 0o644)
}

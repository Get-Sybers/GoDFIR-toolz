//go:build ignore

// gen.go writes the goctl contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "etc")
	os.MkdirAll(filepath.Join(d, "sysctl.d"), 0o755)
	os.MkdirAll(filepath.Join(d, "modprobe.d"), 0o755)
	os.WriteFile(filepath.Join(d, "sysctl.d", "99-custom.conf"), []byte("net.ipv4.ip_forward = 1\n"), 0o644)
	os.WriteFile(filepath.Join(d, "modprobe.d", "local.conf"), []byte("blacklist pcspkr\noptions snd_hda_intel power_save=1\n"), 0o644)
	os.WriteFile(filepath.Join(d, "ld.so.preload"), []byte("/usr/lib/libexample.so\n"), 0o644)
}

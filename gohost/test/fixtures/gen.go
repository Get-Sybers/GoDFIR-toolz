//go:build ignore

// gen.go writes the gohost contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "etc")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "hostname"), []byte("web01\n"), 0o644)
	os.WriteFile(filepath.Join(d, "machine-id"), []byte("0123456789abcdef0123456789abcdef\n"), 0o644)
	os.WriteFile(filepath.Join(d, "fstab"), []byte(
		"UUID=9f8e7d6c-1a2b-3c4d-5e6f-708192a3b4c5 / ext4 errors=remount-ro 0 1\n"+
			"LABEL=data /data xfs noatime 0 2\n"), 0o644)
	os.WriteFile(filepath.Join(d, "os-release"), []byte(
		"NAME=\"Debian GNU/Linux\"\nID=debian\nVERSION_ID=\"13\"\n"), 0o644)
}

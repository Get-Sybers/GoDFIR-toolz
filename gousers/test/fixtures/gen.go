//go:build ignore

// gen.go writes the gousers contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := os.Args[1]
	os.MkdirAll(filepath.Join(d, "etc"), 0o755)
	os.MkdirAll(filepath.Join(d, "home/alice/.ssh"), 0o755)
	os.WriteFile(filepath.Join(d, "etc/passwd"),
		[]byte("root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/zsh\n"), 0o644)
	os.WriteFile(filepath.Join(d, "etc/shadow"),
		[]byte("alice:$y$j9T$abc:20454:0:99999:7:::\n"), 0o644)
	os.WriteFile(filepath.Join(d, "home/alice/.ssh/authorized_keys"),
		[]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGRlYWRiZWVmZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVm alice@box\n"), 0o644)
}

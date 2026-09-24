//go:build ignore

// gen.go writes the goauditd contract-test fixtures into argv[1]: one
// benign execve event and one service start.
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := filepath.Join(os.Args[1], "var/log/audit")
	os.MkdirAll(d, 0o755)
	os.WriteFile(filepath.Join(d, "audit.log"), []byte(
		`type=SYSCALL msg=audit(1767225600.123:42): arch=c000003e syscall=59 success=yes exit=0 ppid=1200 pid=1201 auid=1000 uid=1000 gid=1000 euid=1000 ses=3 tty=pts0 comm="ls" exe="/usr/bin/ls" key="exec_log"
type=EXECVE msg=audit(1767225600.123:42): argc=2 a0="ls" a1=2D6C61
type=CWD msg=audit(1767225600.123:42): cwd="/home/alice"
type=EOE msg=audit(1767225600.123:42):
type=SERVICE_START msg=audit(1767225700.000:43): pid=1 uid=0 msg='unit=nginx res=success'
`), 0o644)
}

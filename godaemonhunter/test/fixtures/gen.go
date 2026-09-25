//go:build ignore

// gen.go writes the godaemonhunter contract-test fixtures into argv[1]:
// Layer-1 material (passwd, hostname, timezone) plus daemon streams
// (an audit event, a syslog line) so a hunt proves the layering.
package main

import (
	"os"
	"path/filepath"
)

func main() {
	d := os.Args[1]
	mk := func(rel, content string) {
		p := filepath.Join(d, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("etc/passwd", "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/zsh\n")
	mk("etc/hostname", "web01\n")
	mk("etc/timezone", "Europe/Berlin\n")
	mk("var/log/audit/audit.log",
		`type=SYSCALL msg=audit(1767225600.123:42): arch=c000003e syscall=59 success=yes exit=0 pid=1201 auid=1000 uid=1000 gid=1000 euid=1000 comm="ls" exe="/usr/bin/ls"`+"\n"+
			`type=EOE msg=audit(1767225600.123:42):`+"\n")
	mk("var/log/syslog", "Jan 10 22:14:02 web01 systemd[1]: Started daily apt activities.\n")
}

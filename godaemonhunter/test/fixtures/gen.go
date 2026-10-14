//go:build ignore

// gen.go writes the godaemonhunter contract-test fixtures into argv[1]:
// Layer-1 material (passwd, hostname, timezone) plus daemon streams
// (an audit event, a syslog line) so a hunt proves the layering.
package main

import (
	"archive/tar"
	"compress/gzip"
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
	// a UAC collection beside the loose tree: the Unix-like Artifacts
	// Collector's tar.gz (uac.log first, the host's files under [root]/),
	// hunted ON like a disk image through the baked-in gomount
	collection(filepath.Join(d, "uac-web02-linux-20260101000000.tar.gz"))
	mk("etc/passwd", "root:x:0:0:root:/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/zsh\n")
	mk("etc/hostname", "web01\n")
	mk("etc/timezone", "Europe/Berlin\n")
	mk("var/log/audit/audit.log",
		`type=SYSCALL msg=audit(1767225600.123:42): arch=c000003e syscall=59 success=yes exit=0 pid=1201 auid=1000 uid=1000 gid=1000 euid=1000 comm="ls" exe="/usr/bin/ls"`+"\n"+
			`type=EOE msg=audit(1767225600.123:42):`+"\n")
	mk("var/log/syslog", "Jan 10 22:14:02 web01 systemd[1]: Started daily apt activities.\n")
	// a Mac's surface beside it: the same run handles both
	mk("System/Library/CoreServices/SystemVersion.plist", `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>ProductBuildVersion</key><string>21H1015</string><key>ProductName</key><string>macOS</string><key>ProductVersion</key><string>12.7.3</string></dict></plist>
`)
	mk("private/var/db/dslocal/nodes/Default/users/gl.plist", `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>name</key><array><string>gl</string></array><key>uid</key><array><string>501</string></array><key>gid</key><array><string>20</string></array><key>home</key><array><string>/Users/gl</string></array><key>shell</key><array><string>/bin/zsh</string></array></dict></plist>
`)
	mk("Users/gl/Library/LaunchAgents/org.keepassxc.KeePassXC.plist", `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>Label</key><string>org.keepassxc.KeePassXC</string><key>ProgramArguments</key><array><string>/Applications/KeePassXC.app/Contents/MacOS/KeePassXC</string></array><key>RunAtLoad</key><true/></dict></plist>
`)
	mk("private/var/log/install.log", "2023-06-11 18:07:13+00 localhost opendirectoryd[195]: opendirectoryd (build 483.250) launched - installer mode\n")
}

// collection writes a minimal UAC 3.4-shaped archive: the run log, then a
// second host's identity and a syslog line under [root]/.
func collection(path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	for _, m := range []struct{ name, body string }{
		{"uac.log", "2026-01-01 00:00:00 +0000 INF Unix-like Artifacts Collector 3.4.0\n2026-01-01 00:00:00 +0000 INF Operating system: linux\n2026-01-01 00:00:00 +0000 INF Hostname: web02\n2026-01-01 00:00:05 +0000 INF Artifacts collection completed in 5 seconds\n"},
		{"bodyfile/bodyfile.txt", "0|/etc/passwd|131|-rw-r--r--|0|0|32|1|2|3|0\n"},
		{"[root]/etc/hostname", "web02\n"},
		{"[root]/etc/passwd", "root:x:0:0:root:/root:/bin/bash\nbob:x:1001:1001::/home/bob:/bin/bash\n"},
		{"[root]/etc/timezone", "Europe/Tallinn\n"},
		{"[root]/var/log/syslog", "Jan  1 00:00:01 web02 sshd[200]: Accepted publickey for bob from 10.0.0.9 port 4100 ssh2\n"},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: m.name, Mode: 0o644, Size: int64(len(m.body)), Format: tar.FormatGNU}); err != nil {
			panic(err)
		}
		if _, err := tw.Write([]byte(m.body)); err != nil {
			panic(err)
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
}

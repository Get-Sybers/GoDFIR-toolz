// goauditd — thin binary front of the auditd parser package: the same code
// godaemonhunter embeds as a subcommand; the tool itself lives in auditd/.
package main

import (
	_ "embed"

	"github.com/Get-Sybers/GoDFIR-toolz/goauditd/auditd"
)

//go:embed contract.yml
var contractYML string

// version is stamped by the Dockerfile (-X main.version=${TOOL_VERSION}).
var version = "0.0.0-dev"

func main() { auditd.Main(version, contractYML) }

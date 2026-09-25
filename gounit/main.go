// gounit — thin binary front of the unit parser package: the same code
// godaemonhunter embeds as a subcommand; the tool itself lives in unit/.
package main

import (
	_ "embed"

	"github.com/Get-Sybers/GoDFIR-toolz/gounit/unit"
)

//go:embed contract.yml
var contractYML string

// version is stamped by the Dockerfile (-X main.version=${TOOL_VERSION}).
var version = "0.0.0-dev"

func main() { unit.Main(version, contractYML) }

// gojournal — thin binary front of the journal parser package: the same code
// godaemonhunter embeds as a subcommand; the tool itself lives in journal/.
package main

import (
	_ "embed"

	"github.com/Get-Sybers/GoDFIR-toolz/gojournal/journal"
)

//go:embed contract.yml
var contractYML string

// version is stamped by the Dockerfile (-X main.version=${TOOL_VERSION}).
var version = "0.0.0-dev"

func main() { journal.Main(version, contractYML) }

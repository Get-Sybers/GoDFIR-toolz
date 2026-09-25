module github.com/Get-Sybers/GoDFIR-toolz/godaemonhunter

go 1.25

require (
	github.com/Get-Sybers/GoDFIR-toolz/goauditd v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gocron v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/goctl v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gohost v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gojournal v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gonetwork v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/goshell v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gosyslog v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gotrash v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gounit v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gousers v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/gowtmp v0.0.0
	github.com/Get-Sybers/GoDFIR-toolz/pinfo v0.0.0
)

require (
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.30 // indirect
	github.com/ulikunitz/xz v0.5.17 // indirect
)

replace (
	github.com/Get-Sybers/GoDFIR-toolz/goauditd => ../goauditd
	github.com/Get-Sybers/GoDFIR-toolz/gocron => ../gocron
	github.com/Get-Sybers/GoDFIR-toolz/goctl => ../goctl
	github.com/Get-Sybers/GoDFIR-toolz/gohost => ../gohost
	github.com/Get-Sybers/GoDFIR-toolz/gojournal => ../gojournal
	github.com/Get-Sybers/GoDFIR-toolz/gonetwork => ../gonetwork
	github.com/Get-Sybers/GoDFIR-toolz/goshell => ../goshell
	github.com/Get-Sybers/GoDFIR-toolz/gosyslog => ../gosyslog
	github.com/Get-Sybers/GoDFIR-toolz/gotrash => ../gotrash
	github.com/Get-Sybers/GoDFIR-toolz/gounit => ../gounit
	github.com/Get-Sybers/GoDFIR-toolz/gousers => ../gousers
	github.com/Get-Sybers/GoDFIR-toolz/gowtmp => ../gowtmp
	github.com/Get-Sybers/GoDFIR-toolz/pinfo => ../pinfo
)

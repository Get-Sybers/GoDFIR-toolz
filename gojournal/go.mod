module github.com/Get-Sybers/GoDFIR-toolz/gojournal

go 1.25

replace github.com/Get-Sybers/GoDFIR-toolz/pinfo => ../pinfo

require (
	github.com/Get-Sybers/GoDFIR-toolz/pinfo v0.0.0-00010101000000-000000000000
	github.com/klauspost/compress v1.20.0
	github.com/pierrec/lz4/v4 v4.1.30
	github.com/ulikunitz/xz v0.5.17
)

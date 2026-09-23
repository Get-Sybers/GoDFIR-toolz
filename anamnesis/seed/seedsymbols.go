// seed-symbols — BUILD-TIME ONLY; it never ships in the runtime image.
//
// Populates the MemProcFS PDB cache (Symbols/ beside vmm.so, symsrv layout
// <name>/<GUID+age>/<name>) by opening each given memory image with the
// symbol server ENABLED and touching every symbol-dependent surface the
// anamnesis collectors read, so MemProcFS downloads exactly the PDBs those
// surfaces need for the image's exact binary revisions (ntoskrnl, ntdll,
// token types, …). The engine itself is always offline and only ever READS
// the cache this tool bakes; PDB coverage therefore reaches exactly the
// Windows builds the seed images represent.
//
// The Dockerfile copies this file into the cloned anamnesis source tree
// (cmd/seed-symbols/main.go) and builds it there, so it uses the engine's
// own pinned gomemprocfs — the binding version can never drift from the
// engine's.
//
// The seed run must fail the image build rather than bake an empty cache, so
// it exits non-zero unless, for every image, the kernel PDB resolves
// (_EPROCESS.CreateTime offset) and at least one process carries a PEB
// command line and a token SID — the exact fields whose emptiness this bake
// exists to fix.
package main

import (
	"flag"
	"fmt"
	"os"

	mp "github.com/sergeyzav/gomemprocfs"
)

// spokePIDs caps the per-process spoke enumerations (modules/threads/handles):
// the PDB offsets they pull in resolve once, not per process, so a handful of
// processes triggers every download while keeping the seed run short.
const spokePIDs = 8

func main() {
	lib := flag.String("lib", "/opt/anamnesis/lib/vmm.so", "path to vmm.so (Symbols/ is seeded beside it)")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: seed-symbols [-lib vmm.so] <memory-image>...")
		os.Exit(2)
	}
	for _, img := range flag.Args() {
		if err := seed(*lib, img); err != nil {
			fmt.Fprintf(os.Stderr, "seed-symbols: %s: %v\n", img, err)
			os.Exit(1)
		}
	}
}

func seed(lib, img string) error {
	// The symbol server stays ENABLED (no WithDisableSymbolServer): this is
	// the one networked run there is, and Symbols/ beside vmm.so must be
	// writable here or MemProcFS silently caches to /tmp instead (pdb.c
	// PDB_Initialize_InitialValues) and the bake ships nothing.
	vmm, err := mp.NewVmm(lib, mp.WithDevice(img), mp.WithDisablePython())
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer vmm.Close()

	// Kernel PDB — the offset the engine reads for process create time. This
	// blocks on the async kernel-symbol init, so it also acts as the "PDB
	// subsystem is actually up" gate (libpdbcrust.so loaded, download done).
	if _, err := vmm.PdbTypeChildOffset("nt", "_EPROCESS", "CreateTime"); err != nil {
		return fmt.Errorf("kernel PDB did not resolve (_EPROCESS.CreateTime): %w", err)
	}

	infos, err := vmm.GetProcessInfoAll()
	if err != nil {
		return fmt.Errorf("process list: %w", err)
	}
	if len(infos) == 0 {
		return fmt.Errorf("no processes found")
	}
	var cmdlines, sids int
	for i := range infos {
		pid := infos[i].PID
		if s, err := vmm.GetProcessInfoString(pid, mp.ProcessInformationOptStringCmdline); err == nil && s != "" {
			cmdlines++
		}
		if s, err := vmm.GetProcessInfoString(pid, mp.ProcessInformationOptStringSID); err == nil && s != "" {
			sids++
		}
		if i < spokePIDs {
			vmm.GetModuleList(pid, mp.ModuleFlag(0))
			vmm.GetThreadList(pid)
			vmm.GetHandleList(pid)
		}
	}
	// System-wide surfaces the collectors read.
	vmm.GetNetList()
	vmm.GetServiceList()
	vmm.GetKDriverList()

	fmt.Printf("seed-symbols: %s: %d processes, command_line %d, sid %d\n",
		img, len(infos), cmdlines, sids)
	if cmdlines == 0 || sids == 0 {
		return fmt.Errorf("PDB-derived fields still empty (command_line %d, sid %d of %d processes) — symbol download failed?",
			cmdlines, sids, len(infos))
	}
	return nil
}

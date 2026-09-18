// materialise — copy TARGETED forensic artefacts (and their named siblings) out
// of an NTFS volume into a real output directory, so the model-B go* tools
// (gore, goamcache, goappcompat, gosbe, goese, gowxt) consume them from a plain
// directory with their existing -d <dir>, unchanged. It is a pure addition to
// the userspace backend: read-only, no FUSE, no mount, no privilege. It reuses
// the same in-process pipeline (openVolumeFS) as ls/cat/stat/tree/browse/stream
// and never writes to the source image.
//
// The artefact sets it knows are DATA, not code: they live in the embedded
// materialise-sets.yml and are read at run time, so a new set or sibling suffix
// is added by editing that file, never this one.
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	yaml "github.com/Velocidex/yaml/v2"
)

// materialiseSetsYAML is the embedded artefact-set catalogue. Keeping the set
// definitions in a data file (not in Go) satisfies the no-hardcoded-static-refs
// rule: the code reads the YAML rather than carrying the paths inline.
//
//go:embed materialise-sets.yml
var materialiseSetsYAML []byte

// artefactSet is one named group of primary artefacts plus the sibling suffixes
// that travel with each primary (registry transaction logs, SQLite WAL/SHM).
// Primaries are volume-relative path patterns; a "*" path component matches
// every immediate child (a user profile, a CDP profile, a *.mdb leaf).
type artefactSet struct {
	Primaries []string `yaml:"primaries"`
	Siblings  []string `yaml:"siblings"`
}

// artefactCatalogue is the whole materialise-sets.yml document.
type artefactCatalogue struct {
	Sets map[string]artefactSet `yaml:"sets"`
}

// loadArtefactSets decodes the embedded catalogue.
func loadArtefactSets() (map[string]artefactSet, error) {
	var cat artefactCatalogue
	if err := yaml.Unmarshal(materialiseSetsYAML, &cat); err != nil {
		return nil, fmt.Errorf("materialise-sets.yml: %w", err)
	}
	if len(cat.Sets) == 0 {
		return nil, fmt.Errorf("materialise-sets.yml: no artefact sets defined")
	}
	return cat.Sets, nil
}

// materialiseRecord is one manifest line: the tool-facing record of a pulled
// file. It mirrors the stream verb's per-file JSONL so downstream tooling reads
// one shape.
type materialiseRecord struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
	MFTID string `json:"mftid"`
}

// multiFlag collects a repeatable string flag (--set, --select).
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func runMaterialise(argv []string) int {
	fs := flag.NewFlagSet("materialise", flag.ExitOnError)
	volume := fs.Int("volume", 0, "1-based NTFS volume (default 0 = largest NTFS)")
	out := fs.String("out", "", "output directory for the pulled artefacts (required, writable)")
	siblings := fs.Bool("siblings", true, "also pull each artefact's named siblings (transaction logs, WAL/SHM)")
	manifest := fs.Bool("manifest", false, "write <out>/materialise.jsonl listing every pulled file")
	var sets, selects multiFlag
	fs.Var(&sets, "set", "named artefact set to pull (repeatable)")
	fs.Var(&selects, "select", "ad-hoc volume-path glob to pull (repeatable)")
	fs.Usage = usage
	_ = fs.Parse(argv)

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "gomount materialise: usage: materialise [--volume N] --out DIR [--set NAME]... [--select GLOB]... [--siblings=true] [--manifest] <image>")
		return 1
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "gomount materialise: --out DIR is required")
		return 1
	}
	if len(sets) == 0 && len(selects) == 0 {
		fmt.Fprintln(os.Stderr, "gomount materialise: nothing to do: give at least one --set or --select")
		return 1
	}

	catalogue, err := loadArtefactSets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gomount: %v\n", err)
		return 1
	}
	// A set that matches no files is fine; a set name that does not exist is an
	// operator typo, so fail loudly before touching the image.
	for _, name := range sets {
		if _, ok := catalogue[name]; !ok {
			fmt.Fprintf(os.Stderr, "gomount materialise: unknown --set %q (known: %s)\n", name, strings.Join(sortedSetNames(catalogue), ", "))
			return 1
		}
	}
	for _, g := range selects {
		// Validate the normalised form the matcher uses (Windows "\" separators
		// become "/"), so a valid selector is not rejected as a bad pattern.
		if _, err := path.Match(strings.ReplaceAll(g, "\\", "/"), ""); err != nil {
			fmt.Fprintf(os.Stderr, "gomount materialise: invalid --select pattern %q: %v\n", g, err)
			return 1
		}
	}

	fsys, closer, err := openVolumeFS(fs.Arg(0), *volume)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gomount: %v\n", err)
		return 1
	}
	defer closer()

	// Resolve every selector to a de-duplicated, path-sorted set of volume files.
	targets := newTargetSet()
	for _, name := range sets {
		resolveSet(fsys, catalogue[name], *siblings, targets)
	}
	walkErrs := 0
	for _, glob := range selects {
		if err := resolveSelect(fsys, glob, targets); err != nil {
			// A partial walk still yields what it could read; surface it so the
			// operator knows the --select result may be incomplete.
			fmt.Fprintf(os.Stderr, "gomount materialise: --select %q incomplete: %v\n", glob, err)
			walkErrs++
		}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gomount: create --out dir: %v\n", err)
		return 1
	}

	var records []materialiseRecord
	var files, errCount int
	var bytesCopied int64
	for _, e := range targets.sorted() {
		n, err := materialiseFile(fsys, e, *out)
		if err != nil {
			errCount++
			fmt.Fprintf(os.Stderr, "gomount materialise: %s: %v\n", e.Path, err)
			continue
		}
		files++
		bytesCopied += n
		records = append(records, materialiseRecord{
			Path:  e.Path,
			Size:  e.Size,
			Mtime: tsCol(e.Mtime),
			MFTID: e.MFTID,
		})
	}

	if *manifest {
		if err := writeManifest(filepath.Join(*out, "materialise.jsonl"), records); err != nil {
			fmt.Fprintf(os.Stderr, "gomount materialise: write manifest: %v\n", err)
			errCount++
		}
	}

	fmt.Fprintf(os.Stderr, "gomount materialise: %d file(s), %d byte(s) -> %s\n", files, bytesCopied, *out)
	if errCount+walkErrs > 0 {
		return 2
	}
	return 0
}

// targetSet de-duplicates resolved files by their canonical volume path — the
// same file can be named by several sets or selects (SYSTEM by both
// registry-core and shimcache) — and yields them in a stable path order.
type targetSet struct {
	byPath map[string]fileEntry
}

func newTargetSet() *targetSet { return &targetSet{byPath: map[string]fileEntry{}} }

func (t *targetSet) add(e fileEntry) {
	if e.IsDir || e.Path == "" {
		return
	}
	if _, ok := t.byPath[e.Path]; !ok {
		t.byPath[e.Path] = e
	}
}

func (t *targetSet) sorted() []fileEntry {
	out := make([]fileEntry, 0, len(t.byPath))
	for _, e := range t.byPath {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// resolveSet resolves one artefact set: every primary pattern, plus (when
// siblings is set) each primary's named siblings from the SAME parent directory.
// A primary or sibling that does not exist is skipped best-effort, so a set that
// matches nothing is not an error.
func resolveSet(fsys volumeFS, set artefactSet, siblings bool, targets *targetSet) {
	for _, pattern := range set.Primaries {
		for _, primary := range matchPrimaries(fsys, pattern) {
			targets.add(primary)
			if !siblings {
				continue
			}
			for _, suffix := range set.Siblings {
				siblingPath := path.Join(path.Dir(primary.Path), primary.Name+suffix)
				if s, err := fsys.Stat(siblingPath); err == nil && !s.IsDir {
					targets.add(s)
				}
			}
		}
	}
}

// resolveSelect pulls every regular file whose volume path matches the ad-hoc
// glob, reusing the same matchGlob semantics as the stream verb (base name, or
// the whole path when the glob contains "/"). Ad-hoc selects carry no sibling
// rule.
func resolveSelect(fsys volumeFS, glob string, targets *targetSet) error {
	return fsys.Walk(func(e fileEntry, _ func() (io.ReadCloser, error)) error {
		if matchGlob(glob, e.Path) {
			targets.add(e)
		}
		return nil
	})
}

// matchPrimaries resolves a volume-path pattern (with "\" or "/" separators) to
// the files it names. A literal path is Stat'd directly; a path component that
// carries glob metacharacters ("*", "?", "[") is expanded by ReadDir, one
// directory level per component, so "\Users\*\NTUSER.DAT" fans out across every
// user profile and "...\SUM\*.mdb" across every database. Directories, and paths
// that do not exist, yield nothing.
func matchPrimaries(fsys volumeFS, pattern string) []fileEntry {
	norm := strings.Trim(strings.ReplaceAll(pattern, "\\", "/"), "/")
	if norm == "" {
		return nil
	}
	comps := strings.Split(norm, "/")
	var out []fileEntry
	var walk func(dir string, idx int)
	walk = func(dir string, idx int) {
		comp := comps[idx]
		last := idx == len(comps)-1
		if !hasGlobMeta(comp) {
			child := path.Join(dir, comp)
			if last {
				if e, err := fsys.Stat(child); err == nil && !e.IsDir {
					out = append(out, e)
				}
				return
			}
			walk(child, idx+1)
			return
		}
		entries, err := fsys.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if ok, _ := path.Match(strings.ToLower(comp), strings.ToLower(e.Name)); !ok {
				continue
			}
			if last {
				if !e.IsDir {
					if se, err := fsys.Stat(path.Join(dir, e.Name)); err == nil && !se.IsDir {
						out = append(out, se)
					}
				}
				continue
			}
			if e.IsDir {
				walk(path.Join(dir, e.Name), idx+1)
			}
		}
	}
	walk("/", 0)
	return out
}

func hasGlobMeta(s string) bool { return strings.ContainsAny(s, "*?[") }

// materialiseFile copies one volume file to <outRoot>/<volume-relative-path>
// (leading "/" stripped, "\"-normalised — the same layout the stream verb's tar
// uses), creating parents and the file at mode 0o400. The copy streams through
// io.Copy so a multi-gigabyte database never buffers in memory. The image is
// only ever read; nothing is written back to the source. It returns the bytes
// written.
func materialiseFile(fsys volumeFS, e fileEntry, outRoot string) (int64, error) {
	rel := filepath.FromSlash(strings.TrimPrefix(e.Path, "/"))
	dest := filepath.Join(outRoot, rel)

	// Defence in depth against a crafted volume path: never write outside --out.
	absOut, err := filepath.Abs(outRoot)
	if err != nil {
		return 0, err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return 0, err
	}
	if absDest != absOut && !strings.HasPrefix(absDest, absOut+string(os.PathSeparator)) {
		return 0, fmt.Errorf("refusing to write outside --out: %s", e.Path)
	}

	if err := ensureDirBeneath(absOut, filepath.Dir(absDest)); err != nil {
		return 0, err
	}
	r, err := fsys.Open(e.Path)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	// Remove any prior pull first: a 0o400 file cannot be re-opened O_WRONLY, so
	// this keeps a re-run idempotent.
	_ = os.Remove(dest)
	w, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o400)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(w, r)
	closeErr := w.Close()
	if copyErr != nil {
		return n, copyErr
	}
	if closeErr != nil {
		return n, closeErr
	}
	// Pin the mode to 0o400 regardless of the process umask.
	if err := os.Chmod(dest, 0o400); err != nil {
		return n, err
	}
	return n, nil
}

// ensureDirBeneath creates dir and any missing parents under base, refusing to
// traverse or create through a pre-existing symlink so a crafted volume path or a
// symlinked output tree cannot redirect a write outside base. base must be an
// existing real directory (the created --out). Combined with O_NOFOLLOW on the
// file open, the whole path from base to the file is symlink-free.
func ensureDirBeneath(base, dir string) error {
	rel, err := filepath.Rel(base, dir)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to write outside --out: %s", dir)
	}
	cur := base
	for _, comp := range strings.Split(rel, string(os.PathSeparator)) {
		if comp == "" {
			continue
		}
		cur = filepath.Join(cur, comp)
		switch fi, err := os.Lstat(cur); {
		case err == nil:
			if fi.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to write through symlink: %s", cur)
			}
			if !fi.IsDir() {
				return fmt.Errorf("output path component is not a directory: %s", cur)
			}
		case os.IsNotExist(err):
			if err := os.Mkdir(cur, 0o755); err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

// writeManifest writes one JSON object per pulled file to dest (newline-
// delimited), in the path order the files were copied.
func writeManifest(dest string, records []materialiseRecord) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// sortedSetNames returns the catalogue's set names in stable order for a usage
// message.
func sortedSetNames(m map[string]artefactSet) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

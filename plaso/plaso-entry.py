#!/usr/bin/python3
"""plaso-entry — the multi-tool dispatcher ENTRYPOINT of get-sybers/plaso
(docs/framework 04 §4.3).

    plaso-entry.py log2timeline | psort | image_export
        Self-orchestrate that sub-tool from its own env block
        (PLASO_LOG2TIMELINE_*, PLASO_PSORT_*, PLASO_IMAGE_EXPORT_*): discover
        the inputs under its INPUT_DIR, batch over them into one output folder
        per item under its OUT_DIR, skip items that already have valid output
        unless FORCE is set, print exactly one JSON summary line on stdout and
        exit 0 (success) / 1 (nothing produced) / 2 (config error) / 3 (partial).

    plaso-entry.py <tool> <args...>
        The debug pass-through: exec one of plaso's own entry tools
        (log2timeline, psort, image_export, pinfo, psteal — with or without
        .py) or python3 (the baked psort wrapper) with that argv verbatim.

    plaso-entry.py --version | --print-contract

Records go to files; stderr carries plaso's own output and this script's
progress; stdout carries the summary line only.
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import time

TOOL = "plaso"
CONTRACT = "/opt/dxdfir/contract.yml"
VERSION_FILE = "/etc/dxdfir-plaso-version"
SUBTOOLS = ("log2timeline", "psort", "image_export")
PASSTHROUGH = {"log2timeline", "psort", "image_export", "pinfo", "psteal",
               "log2timeline.py", "psort.py", "image_export.py", "pinfo.py", "psteal.py", "python3"}
IMAGE_EXTS = {".e01", ".ex01", ".raw", ".dd", ".img", ".vmdk", ".vhd", ".vhdx", ".qcow2", ".aff4", ".001", ".bin"}
LEVELS = {"error": 0, "warn": 1, "info": 2, "debug": 3}
BOOL_TRUE, BOOL_FALSE = {"1", "true", "yes", "on"}, {"", "0", "false", "no", "off"}


class ConfigError(Exception):
    pass


def version() -> str:
    try:
        with open(VERSION_FILE) as fh:
            return fh.read().split()[1]
    except (OSError, IndexError):
        return "0.0.0-dev"


class Config:
    """The resolved env block of one sub-tool."""

    def __init__(self, subtool: str, env):
        self.subtool = subtool
        self.prefix = f"{TOOL.upper()}_{subtool.upper()}"
        self._env = env
        self.input_dir = self.get("INPUT_DIR", "/input")
        self.out_dir = self.get("OUT_DIR", "/output")
        self.work_dir = self.get("WORK_DIR", "/work")
        self.force = self.bool("FORCE", "0")
        level = self.get("LOG_LEVEL", "info").lower()
        if level not in LEVELS:
            raise ConfigError(f"{self.prefix}_LOG_LEVEL: {level!r} is not one of error|warn|info|debug")
        self.level = LEVELS[level]
        if not os.path.isdir(self.input_dir):
            raise ConfigError(f"{self.prefix}_INPUT_DIR {self.input_dir}: not a readable directory")
        ensure_writable(self.out_dir, f"{self.prefix}_OUT_DIR")
        try:
            ensure_writable(self.work_dir, f"{self.prefix}_WORK_DIR")
        except ConfigError as e:
            self.log(1, f"work dir not usable ({e}); using /tmp")
            self.work_dir = "/tmp"

    def get(self, suffix: str, default: str) -> str:
        return self._env.get(f"{self.prefix}_{suffix}") or default

    def bool(self, suffix: str, default: str) -> bool:
        raw = self.get(suffix, default).strip().lower()
        if raw in BOOL_TRUE:
            return True
        if raw in BOOL_FALSE:
            return False
        raise ConfigError(f"{self.prefix}_{suffix}: {raw!r} is not a boolean (1/true/yes/on or 0/false/no/off)")

    def log(self, level: int, msg: str) -> None:
        if level <= self.level:
            sys.stderr.write(f"{TOOL} {self.subtool}: {msg}\n")
            sys.stderr.flush()


def ensure_writable(path: str, what: str) -> None:
    try:
        os.makedirs(path, exist_ok=True)
        probe = os.path.join(path, f".probe-{os.getpid()}")
        with open(probe, "w"):
            pass
        os.remove(probe)
    except OSError as e:
        raise ConfigError(f"{what} {path}: not writable: {e}") from e


def item_name(root: str, item: str) -> str:
    """The per-item output folder: the path relative to the input root with
    separators and whitespace folded to '_' (batch.go's batchItemName)."""
    rel = os.path.relpath(item, root)
    if rel in (".", "") or rel.startswith(".."):
        rel = os.path.basename(item)
    name = "".join("_" if c in "/\\:" or c.isspace() else c for c in rel)
    return name or "item"


def psort_item_name(root: str, item: str) -> str:
    """psort's per-item output folder: the storage file's item name WITHOUT its
    `.plaso` extension — and when the file sits in a folder of the same name
    (log2timeline's own `<item>/<item>.plaso`), that folder's name — so with
    OUT_DIR pointing at the log2timeline output root the rendered timeline lands
    beside its storage file in the one `<item>/` folder, never in a second
    `<item>_<item>.plaso/` tree beside it."""
    rel = os.path.relpath(item, root)
    if rel in (".", "") or rel.startswith(".."):
        rel = os.path.basename(item)
    parts = rel.replace("\\", "/").split("/")
    base = parts[-1]
    if base.lower().endswith(".plaso"):
        base = base[: -len(".plaso")]
    if len(parts) > 1 and parts[-2] == base:
        parts = parts[:-1]
    else:
        parts[-1] = base
    return item_name(root, os.path.join(root, *parts))


def is_image(path: str) -> bool:
    return os.path.splitext(path)[1].lower() in IMAGE_EXTS


def walk_files(root: str):
    for dirpath, _dirs, files in os.walk(root):
        for f in sorted(files):
            yield os.path.join(dirpath, f)


def run(cfg: Config, argv: list[str], log_path: str) -> int:
    """Run one plaso tool; its stdout+stderr go to log_path and, at debug
    level, to stderr as well. Returns the exit code."""
    cfg.log(3, "exec " + " ".join(argv))
    with open(log_path, "ab") as log:
        proc = subprocess.run(argv, stdout=log, stderr=subprocess.STDOUT, check=False)
    if cfg.level >= 3:
        with open(log_path, "rb") as fh:
            sys.stderr.buffer.write(fh.read())
    return proc.returncode


# ---- sub-tools ---------------------------------------------------------------

def discover_log2timeline(cfg: Config) -> list[str]:
    """Every disk image under the tree, plus every immediate subdirectory of the
    input root (a staged evidence tree)."""
    items = [p for p in walk_files(cfg.input_dir) if is_image(p)]
    for entry in sorted(os.listdir(cfg.input_dir)):
        p = os.path.join(cfg.input_dir, entry)
        if os.path.isdir(p):
            items.append(p)
    return sorted(items)


def process_log2timeline(cfg: Config, item: str, item_dir: str) -> tuple[int, dict]:
    base = os.path.splitext(os.path.basename(item))[0]
    storage = os.path.join(item_dir, f"{base}.plaso")
    if os.path.exists(storage):
        os.remove(storage)
    argv = ["log2timeline", "--status_view", "none", "--temporary_directory", cfg.work_dir]
    if os.path.isfile(item):
        argv += ["--partitions", "all"]
        if cfg.bool("VSS", "1"):
            argv += ["--vss-stores", "all"]
    if cfg.bool("WINREG_BINARY", "1"):
        argv.append("--extract_winreg_binary")
    parsers = cfg.get("PARSERS", "")
    if parsers:
        argv += ["--parsers", parsers]
    argv += cfg.get("ARGS", "").split()
    argv += ["--storage-file", storage, item]
    rc = run(cfg, argv, os.path.join(item_dir, "log2timeline.log"))
    if rc != 0:
        raise RuntimeError(f"log2timeline exited {rc} (see log2timeline.log)")
    if not (os.path.isfile(storage) and os.path.getsize(storage) > 0):
        raise RuntimeError("log2timeline produced no storage file")
    return 1, {"storage_file": os.path.basename(storage), "log": "log2timeline.log"}


def discover_psort(cfg: Config) -> list[str]:
    return sorted(p for p in walk_files(cfg.input_dir) if p.lower().endswith(".plaso"))


def process_psort(cfg: Config, item: str, item_dir: str) -> tuple[int, dict]:
    fmt = cfg.get("OUTPUT_FORMAT", "json_line")
    out = os.path.join(item_dir, "timeline.jsonl")
    module = cfg.get("OUTPUT_MODULE", "")
    if module:
        argv = ["python3", "/opt/dxdfir/psort_wrapper.py", module]
    else:
        argv = ["psort"]
    argv += ["--status_view", "none", "--temporary_directory", cfg.work_dir, "-o", fmt,
             "--output_fallback_hostname"]
    argv += cfg.get("ARGS", "").split()
    argv += ["-w", out, item]
    if os.path.exists(out):
        os.remove(out)
    rc = run(cfg, argv, os.path.join(item_dir, "psort.log"))
    if rc != 0:
        raise RuntimeError(f"psort exited {rc} (see psort.log)")
    if not os.path.isfile(out):
        raise RuntimeError("psort produced no output file")
    with open(out, "rb") as fh:
        records = sum(1 for line in fh if line.strip())
    return records, {"output": os.path.basename(out), "format": fmt, "records": records, "log": "psort.log"}


def discover_image_export(cfg: Config) -> list[str]:
    return sorted(p for p in walk_files(cfg.input_dir) if is_image(p))


def process_image_export(cfg: Config, item: str, item_dir: str) -> tuple[int, dict]:
    export = os.path.join(item_dir, "export")
    shutil.rmtree(export, ignore_errors=True)
    argv = ["image_export", "-q", "--partitions", "all", "--temporary_directory", cfg.work_dir]
    if cfg.bool("VSS", "1"):
        argv += ["--vss-stores", "all"]
    filter_file = cfg.get("FILTER_FILE", "")
    if filter_file:
        argv += ["--filter_file", filter_file]
    else:
        argv += ["--artifact_filters", cfg.get("ARTIFACT_FILTERS", "WindowsEventLogs")]
    argv += cfg.get("ARGS", "").split()
    argv += ["-w", export, item]
    rc = run(cfg, argv, os.path.join(item_dir, "image_export.log"))
    if rc != 0:
        raise RuntimeError(f"image_export exited {rc} (see image_export.log)")
    files = sum(len(fs) for _d, _ds, fs in os.walk(export)) if os.path.isdir(export) else 0
    return files, {"export_dir": "export", "files": files, "log": "image_export.log"}


SUB = {
    "log2timeline": (discover_log2timeline, process_log2timeline),
    "psort": (discover_psort, process_psort),
    "image_export": (discover_image_export, process_image_export),
}


# ---- the batch loop (mirrors batch.go) --------------------------------------

def batch(subtool: str, env, stdout) -> int:
    started = time.time()
    summary = {"tool": TOOL, "subtool": subtool, "version": version(), "status": "", "inputs": 0,
               "processed": 0, "skipped": 0, "failed": 0, "records": 0, "outputs": [], "exit": 0,
               "started": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(started)), "duration_s": 0.0}

    def finish(status: str, code: int) -> int:
        summary["status"], summary["exit"] = status, code
        summary["duration_s"] = round(time.time() - started, 3)
        stdout.write(json.dumps(summary, separators=(",", ":")) + "\n")
        stdout.flush()
        return code

    try:
        cfg = Config(subtool, env)
        discover, process = SUB[subtool]
        items = discover(cfg)
    except ConfigError as e:
        summary["error"] = str(e)
        sys.stderr.write(f"{TOOL} {subtool}: config error: {e}\n")
        return finish("config_error", 2)
    except OSError as e:
        summary["error"] = f"discover: {e}"
        sys.stderr.write(f"{TOOL} {subtool}: config error: {e}\n")
        return finish("config_error", 2)

    summary["inputs"] = len(items)
    if not items:
        cfg.log(1, f"no inputs found under {cfg.input_dir}")
        return finish("nothing", 1)

    failures = []
    name_of = psort_item_name if subtool == "psort" else item_name
    for item in items:
        item_dir = os.path.join(cfg.out_dir, name_of(cfg.input_dir, item))
        marker = os.path.join(item_dir, f"{subtool}.jsonl")
        if not cfg.force and os.path.isfile(marker):
            summary["skipped"] += 1
            summary["outputs"].append(item_dir)
            cfg.log(2, f"skip {item} (output exists: {marker})")
            continue
        try:
            os.makedirs(item_dir, exist_ok=True)
            if cfg.force and os.path.exists(marker):
                os.remove(marker)
            records, index = process(cfg, item, item_dir)
            with open(marker + ".part", "w") as fh:
                fh.write(json.dumps(index, separators=(",", ":")) + "\n")
            os.replace(marker + ".part", marker)
        except Exception as e:  # noqa: BLE001 — one item's failure is counted, the batch continues
            summary["failed"] += 1
            failures.append({"item": item, "error": str(e)})
            cfg.log(0, f"FAILED {item}: {e}")
            continue
        summary["processed"] += 1
        summary["records"] += records
        summary["outputs"].append(item_dir)
        cfg.log(2, f"processed {item} -> {marker} ({records} records)")
    if failures:
        summary["failures"] = failures
    cfg.log(2, "{inputs} inputs: {processed} processed, {skipped} skipped, {failed} failed, {records} records".format(**summary))
    if summary["failed"] and not (summary["processed"] + summary["skipped"]):
        return finish("nothing", 1)
    if summary["failed"]:
        return finish("partial", 3)
    return finish("ok", 0)


def usage() -> None:
    sys.stderr.write("usage: plaso-entry.py log2timeline|psort|image_export   (env-driven batch)\n"
                     "       plaso-entry.py <log2timeline|psort|image_export|pinfo|psteal|python3> <args...>   (pass-through)\n"
                     "       plaso-entry.py --version | --print-contract\n")


def main() -> int:
    argv = sys.argv[1:]
    if len(argv) == 1 and argv[0].lstrip("-") == "version":
        print(f"{TOOL} {version()}")
        return 0
    if len(argv) == 1 and argv[0].lstrip("-") == "print-contract":
        with open(CONTRACT) as fh:
            sys.stdout.write(fh.read())
        return 0
    if len(argv) == 1 and argv[0] in SUBTOOLS:
        return batch(argv[0], os.environ, sys.stdout)
    if argv and argv[0] in PASSTHROUGH:
        os.execvp(argv[0], argv)
    usage()
    if argv:
        error = f"{argv[0]!r} is neither a sub-tool nor an allowed pass-through tool"
    else:
        error = "no sub-tool named (log2timeline|psort|image_export)"
    summary = {"tool": TOOL, "subtool": "", "version": version(), "status": "config_error", "inputs": 0,
               "processed": 0, "skipped": 0, "failed": 0, "records": 0, "outputs": [], "error": error, "exit": 2,
               "started": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "duration_s": 0.0}
    print(json.dumps(summary, separators=(",", ":")))
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

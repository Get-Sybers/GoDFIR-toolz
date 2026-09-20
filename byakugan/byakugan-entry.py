#!/usr/bin/python3
"""Byakugan container ENTRYPOINT — the multi-tool dispatcher of get-sybers/byakugan
(docs/framework 04 §4.3): ONE binary, three operations, selected by the first
argument.

Env-driven batch (the mode DX_DFIR uses — the sub-tool name and nothing else):

    byakugan build        materialise CAR from a processed-evidence tree:
                          byakugan.pipeline --batch BYAKUGAN_BUILD_INPUT_DIR
                          --out BYAKUGAN_BUILD_OUT_DIR [--force] [--derive] [--stix]
    byakugan timeline     the unified, time-ordered CAR timeline of a car tree:
                          byakugan.timeline BYAKUGAN_TIMELINE_INPUT_DIR
                          --out BYAKUGAN_TIMELINE_OUT_DIR/timeline.jsonl [--host …]
    byakugan car-vocab    {object: [car_actions]} JSON — the canonical car_action
                          vocabulary the verify-car gate checks values against
                          (its stdout IS that JSON, one line)

Each batch run prints exactly one JSON summary line on stdout (the engine's own
summary embedded as "engine") and exits 0 (success) / 1 (nothing produced) /
2 (config error); the engine's stdout is captured so nothing else reaches it.

The debug pass-through — a sub-tool name followed by arguments:

    byakugan timeline <car_dir> [flags]   -> byakugan.timeline main(argv)
    byakugan <build flags...>             -> byakugan.pipeline main(argv), the
                                             default: --in/--out or --batch

This is `dxdfir`'s ENTIRE interaction surface with the external Byakugan engine:
the engine is cloned at the pinned ref and baked into this image at build time.
It reconstructs its object model live from its own nested submodules and resolves
that model RELATIVE TO ITS PACKAGE, so it runs from the baked source tree
(PYTHONPATH=/opt/byakugan); its Go parse binary is BYAKUGAN_PARSE_BIN — both are
baked as image ENV.
"""
from __future__ import annotations

import io
import json
import os
import sys
import time

TOOL = "byakugan"
CONTRACT = "/opt/byakugan/contract.yml"
SUBTOOLS = ("build", "timeline", "car-vocab")
LEVELS = {"error": 0, "warn": 1, "info": 2, "debug": 3}
BOOL_TRUE, BOOL_FALSE = {"1", "true", "yes", "on"}, {"", "0", "false", "no", "off"}


class ConfigError(Exception):
    pass


def version() -> str:
    return os.environ.get("BYAKUGAN_VERSION") or "0.0.0-dev"


def engine_ref() -> str:
    return os.environ.get("BYAKUGAN_REF") or ""


class Config:
    """The resolved env block of one sub-tool (BYAKUGAN_<SUBTOOL>_*)."""

    def __init__(self, subtool: str, env):
        self.subtool = subtool
        self.prefix = f"{TOOL.upper()}_{subtool.upper().replace('-', '_')}"
        self._env = env
        self.input_dir = self.get("INPUT_DIR", "/input")
        self.out_dir = self.get("OUT_DIR", "/output")
        self.force = self.bool("FORCE", "0")
        level = self.get("LOG_LEVEL", "info").lower()
        if level not in LEVELS:
            raise ConfigError(f"{self.prefix}_LOG_LEVEL: {level!r} is not one of error|warn|info|debug")
        self.level = LEVELS[level]
        if not os.path.isdir(self.input_dir):
            raise ConfigError(f"{self.prefix}_INPUT_DIR {self.input_dir}: not a readable directory")
        try:
            os.makedirs(self.out_dir, exist_ok=True)
            probe = os.path.join(self.out_dir, f".probe-{os.getpid()}")
            with open(probe, "w"):
                pass
            os.remove(probe)
        except OSError as e:
            raise ConfigError(f"{self.prefix}_OUT_DIR {self.out_dir}: not writable: {e}") from e

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


def run_engine(main, argv: list[str]):
    """Run an engine main() with its stdout captured (it prints its own JSON
    summary there); return (return code or None, captured stdout, SystemExit
    message or None)."""
    saved = sys.stdout
    buf = io.StringIO()
    sys.stdout = buf
    try:
        rc = main(argv)
        message = None
    except SystemExit as e:  # argparse errors and the engine's own fail-fast exits
        rc, message = (e.code if isinstance(e.code, int) else None), (None if isinstance(e.code, int) else str(e.code))
    finally:
        sys.stdout = saved
    return rc, buf.getvalue(), message


def batch(subtool: str, env, stdout) -> int:
    started = time.time()
    summary = {"tool": TOOL, "subtool": subtool, "version": version(), "engine_ref": engine_ref(),
               "status": "", "inputs": 0, "processed": 0, "skipped": 0, "failed": 0, "records": 0,
               "outputs": [], "exit": 0,
               "started": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(started)), "duration_s": 0.0}

    def finish(status: str, code: int) -> int:
        summary["status"], summary["exit"] = status, code
        summary["duration_s"] = round(time.time() - started, 3)
        stdout.write(json.dumps(summary, separators=(",", ":"), default=str) + "\n")
        stdout.flush()
        return code

    try:
        cfg = Config(subtool, env)
    except ConfigError as e:
        summary["error"] = str(e)
        sys.stderr.write(f"{TOOL} {subtool}: config error: {e}\n")
        return finish("config_error", 2)

    if subtool == "build":
        from byakugan.pipeline import main as build_main
        argv = ["--batch", cfg.input_dir, "--out", cfg.out_dir]
        if cfg.force:
            argv.append("--force")
        if cfg.bool("DERIVE", "0"):
            argv.append("--derive")
        if cfg.bool("STIX", "0"):
            argv.append("--stix")
        argv += cfg.get("ARGS", "").split()
        cfg.log(3, "engine argv: " + " ".join(argv))
        rc, out, message = run_engine(build_main, argv)
        results = []
        try:
            results = json.loads(out) if out.strip() else []
        except ValueError:
            cfg.log(1, "engine summary is not JSON; kept verbatim")
            summary["engine_raw"] = out.strip()
        summary["engine"] = results
        if message is not None:
            summary["error"] = message
            cfg.log(0, f"engine: {message}")
            return finish("config_error", 2)
        if isinstance(results, list):
            summary["inputs"] = len(results)
            summary["failed"] = sum(1 for r in results if isinstance(r, dict) and "error" in r)
            summary["skipped"] = sum(1 for r in results if isinstance(r, dict) and r.get("skipped"))
            summary["processed"] = summary["inputs"] - summary["failed"] - summary["skipped"]
            summary["records"] = sum(int(r.get("events", 0) or 0) for r in results if isinstance(r, dict))
            summary["outputs"] = sorted({str(r.get("out") or r.get("out_dir") or cfg.out_dir) for r in results if isinstance(r, dict)}) or [cfg.out_dir]
            if summary["failed"]:
                summary["failures"] = [{"item": str(r.get("source") or r.get("in") or "?"), "error": str(r["error"])}
                                       for r in results if isinstance(r, dict) and "error" in r]
        cfg.log(2, "{inputs} sources: {processed} processed, {skipped} skipped, {failed} failed".format(**summary))
        if rc == 0 and summary["failed"] and summary["processed"] + summary["skipped"]:
            return finish("partial", 3)
        if rc == 0:
            return finish("ok", 0)
        return finish("nothing", 1)

    if subtool == "timeline":
        from byakugan.timeline import main as timeline_main
        out_path = os.path.join(cfg.out_dir, "timeline.jsonl")
        summary["inputs"] = 1
        summary["outputs"] = [cfg.out_dir]
        if os.path.isfile(out_path) and not cfg.force:
            summary["skipped"] = 1
            cfg.log(2, f"skip {cfg.input_dir} (output exists: {out_path})")
            return finish("ok", 0)
        argv = [cfg.input_dir, "--out", out_path]
        for flag in ("HOST", "AFTER", "BEFORE"):
            if v := cfg.get(flag, ""):
                argv += [f"--{flag.lower()}", v]
        argv += cfg.get("ARGS", "").split()
        cfg.log(3, "engine argv: " + " ".join(argv))
        rc, out, message = run_engine(timeline_main, argv)
        if message is not None:
            summary["error"] = message
            cfg.log(0, f"engine: {message}")
            if message.startswith("no car.db"):
                return finish("nothing", 1)
            return finish("config_error", 2)
        try:
            engine = json.loads(out) if out.strip() else {}
        except ValueError:
            engine = {"raw": out.strip()}
        summary["engine"] = engine
        summary["records"] = int(engine.get("entries", 0) or 0) if isinstance(engine, dict) else 0
        summary["processed"] = 1
        cfg.log(2, f"timeline {out_path}: {summary['records']} entries")
        return finish("ok" if rc == 0 else "nothing", 0 if rc == 0 else 1)

    summary["error"] = f"unknown sub-tool {subtool!r}"
    return finish("config_error", 2)


def car_vocab() -> int:
    from byakugan import carmodel
    model = carmodel.load()
    json.dump({obj: sorted(model[obj].get("actions", [])) for obj in model}, sys.stdout)
    sys.stdout.write("\n")
    return 0


def usage() -> None:
    sys.stderr.write("usage: byakugan build|timeline            (env-driven batch: BYAKUGAN_<SUBTOOL>_*)\n"
                     "       byakugan car-vocab                 (the car_action vocabulary, one JSON line)\n"
                     "       byakugan timeline <car_dir> [flags] | byakugan <build flags...>   (pass-through)\n"
                     "       byakugan --version | --print-contract\n")


def main() -> int:
    argv = sys.argv[1:]
    if len(argv) == 1 and argv[0].lstrip("-") == "version":
        print(f"{TOOL} {version()} ({engine_ref() or 'unpinned'})")
        return 0
    if len(argv) == 1 and argv[0].lstrip("-") == "print-contract":
        with open(CONTRACT) as fh:
            sys.stdout.write(fh.read())
        return 0
    if len(argv) == 1 and argv[0] == "car-vocab":
        return car_vocab()
    if len(argv) == 1 and argv[0] in ("build", "timeline"):
        return batch(argv[0], os.environ, sys.stdout)
    if not argv:
        usage()
        print(json.dumps({"tool": TOOL, "subtool": "", "version": version(), "engine_ref": engine_ref(),
                          "status": "config_error", "inputs": 0, "processed": 0, "skipped": 0, "failed": 0,
                          "records": 0, "outputs": [], "error": "no sub-tool named (build|timeline|car-vocab)",
                          "exit": 2, "started": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                          "duration_s": 0.0}, separators=(",", ":")))
        return 2
    # pass-through: the engine's own argv
    if argv[0] == "timeline":
        from byakugan.timeline import main as timeline_main
        return timeline_main(argv[1:])
    if argv[0] == "build":
        argv = argv[1:]
    from byakugan.pipeline import main as build_main
    return build_main(argv)


if __name__ == "__main__":
    raise SystemExit(main())

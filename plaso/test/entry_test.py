#!/usr/bin/env python3
"""test/entry_test.py — host-side tests of the plaso-entry dispatcher's batch
loop (no plaso needed): stub log2timeline/psort/image_export scripts on PATH
stand in for the tools, so discovery, per-item folders, the index marker,
idempotency, FORCE, the config-error and partial exits are all exercised.

    python3 plaso/test/entry_test.py
"""
import io
import json
import os
import shutil
import stat
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
import importlib.util  # noqa: E402

spec = importlib.util.spec_from_file_location("plaso_entry", os.path.join(os.path.dirname(HERE), "plaso-entry.py"))
entry = importlib.util.module_from_spec(spec)
spec.loader.exec_module(entry)

STUB_L2T = """#!/bin/sh
# stub log2timeline: the storage file is the last-but-one argument's value
out=""
while [ $# -gt 0 ]; do
  case "$1" in --storage-file) out="$2"; shift ;; esac
  src="$1"; shift
done
case "$src" in *fail*) echo "boom" >&2; exit 1 ;; esac
printf 'plaso-storage' > "$out"
"""
STUB_PSORT = """#!/bin/sh
out=""
while [ $# -gt 0 ]; do
  case "$1" in -w) out="$2"; shift ;; esac
  shift
done
printf '{"a":1}\\n{"a":2}\\n{"a":3}\\n' > "$out"
"""


class EntryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.bin = os.path.join(self.tmp, "bin")
        os.makedirs(self.bin)
        for name, body in (("log2timeline", STUB_L2T), ("psort", STUB_PSORT)):
            p = os.path.join(self.bin, name)
            with open(p, "w") as fh:
                fh.write(body)
            os.chmod(p, os.stat(p).st_mode | stat.S_IEXEC)
        self.old_path = os.environ["PATH"]
        os.environ["PATH"] = self.bin + os.pathsep + self.old_path
        self.inp = os.path.join(self.tmp, "in")
        self.out = os.path.join(self.tmp, "out")
        os.makedirs(self.inp)

    def tearDown(self):
        os.environ["PATH"] = self.old_path
        shutil.rmtree(self.tmp, ignore_errors=True)

    def run_batch(self, sub, **env):
        base = {f"PLASO_{sub.upper()}_INPUT_DIR": self.inp, f"PLASO_{sub.upper()}_OUT_DIR": self.out,
                f"PLASO_{sub.upper()}_WORK_DIR": os.path.join(self.tmp, "work")}
        base.update(env)
        buf = io.StringIO()
        code = entry.batch(sub, base, buf)
        raw = buf.getvalue()
        self.assertEqual(raw.count("\n"), 1, f"stdout must be one line: {raw!r}")
        summary = json.loads(raw)
        self.assertEqual(summary["exit"], code)
        self.assertEqual(summary["tool"], "plaso")
        self.assertEqual(summary["subtool"], sub)
        for key in ("version", "status", "inputs", "processed", "skipped", "failed", "records", "outputs", "started", "duration_s"):
            self.assertIn(key, summary)
        return code, summary

    def test_log2timeline_lifecycle(self):
        os.makedirs(os.path.join(self.inp, "hostA"))
        with open(os.path.join(self.inp, "hostA", "f.txt"), "w") as fh:
            fh.write("x")
        with open(os.path.join(self.inp, "disk.E01"), "wb") as fh:
            fh.write(b"EVF")
        code, s = self.run_batch("log2timeline")
        self.assertEqual((code, s["status"], s["inputs"], s["processed"]), (0, "ok", 2, 2))
        self.assertTrue(os.path.isfile(os.path.join(self.out, "hostA", "hostA.plaso")))
        self.assertTrue(os.path.isfile(os.path.join(self.out, "disk.E01", "disk.plaso")))
        self.assertTrue(os.path.isfile(os.path.join(self.out, "hostA", "log2timeline.jsonl")))
        # idempotent
        code, s = self.run_batch("log2timeline")
        self.assertEqual((code, s["processed"], s["skipped"]), (0, 0, 2))
        # forced
        code, s = self.run_batch("log2timeline", PLASO_LOG2TIMELINE_FORCE="yes")
        self.assertEqual((code, s["processed"], s["skipped"]), (0, 2, 0))

    def test_partial_and_nothing(self):
        os.makedirs(os.path.join(self.inp, "good"))
        os.makedirs(os.path.join(self.inp, "fail-me"))
        code, s = self.run_batch("log2timeline")
        self.assertEqual((code, s["status"], s["failed"]), (3, "partial", 1))
        self.assertEqual(len(s["failures"]), 1)
        self.assertFalse(os.path.exists(os.path.join(self.out, "fail-me", "log2timeline.jsonl")))
        shutil.rmtree(os.path.join(self.inp, "good"))
        shutil.rmtree(self.out)
        code, s = self.run_batch("log2timeline")
        self.assertEqual((code, s["status"]), (1, "nothing"))
        shutil.rmtree(os.path.join(self.inp, "fail-me"))
        code, s = self.run_batch("log2timeline")
        self.assertEqual((code, s["status"], s["inputs"]), (1, "nothing", 0))

    def test_psort_counts_records(self):
        with open(os.path.join(self.inp, "case.plaso"), "wb") as fh:
            fh.write(b"plaso-storage")
        code, s = self.run_batch("psort")
        self.assertEqual((code, s["records"]), (0, 3))
        self.assertTrue(os.path.isfile(os.path.join(self.out, "case", "timeline.jsonl")))

    def test_config_errors(self):
        for env in ({"PLASO_PSORT_INPUT_DIR": os.path.join(self.tmp, "nope")},
                    {"PLASO_PSORT_FORCE": "maybe"},
                    {"PLASO_PSORT_LOG_LEVEL": "loud"},
                    {"PLASO_PSORT_OUT_DIR": os.path.join(self.inp, "x", "y") if False else "/proc/none/out"}):
            code, s = self.run_batch("psort", **env)
            self.assertEqual((code, s["status"]), (2, "config_error"), env)
            self.assertIn("error", s)

    def test_item_name(self):
        self.assertEqual(entry.item_name("/in", "/in/a/b c/$MFT"), "a_b_c_$MFT")
        self.assertEqual(entry.item_name("/in", "/elsewhere/x"), "x")

    def test_psort_item_name(self):
        # the extension goes; a same-named folder (log2timeline's own layout) collapses to the folder
        self.assertEqual(entry.psort_item_name("/in", "/in/img.E01/img.E01.plaso"), "img.E01")
        self.assertEqual(entry.psort_item_name("/in", "/in/case-a/img.E01/img.E01.plaso"), "case-a_img.E01")
        self.assertEqual(entry.psort_item_name("/in", "/in/storage/other.plaso"), "storage_other")
        self.assertEqual(entry.psort_item_name("/in", "/in/x y/host/OTHER.PLASO"), "x_y_host_OTHER")
        self.assertEqual(entry.psort_item_name("/in", "/elsewhere/img.plaso"), "img")

    def test_psort_renders_beside_the_storage_file(self):
        # log2timeline wrote <out>/<item>/<item>.plaso; psort over that root, into that
        # root, lands timeline.jsonl in the same <item>/ folder (no <item>_<item>.plaso/)
        os.makedirs(os.path.join(self.inp, "img.E01"))
        with open(os.path.join(self.inp, "img.E01", "img.E01.plaso"), "wb") as fh:
            fh.write(b"plaso-storage")
        code, s = self.run_batch("psort", PLASO_PSORT_OUT_DIR=self.inp)
        self.assertEqual(code, 0, s)
        self.assertTrue(os.path.isfile(os.path.join(self.inp, "img.E01", "timeline.jsonl")))
        self.assertTrue(os.path.isfile(os.path.join(self.inp, "img.E01", "psort.jsonl")))
        self.assertFalse(os.path.exists(os.path.join(self.inp, "img.E01_img.E01.plaso")))


if __name__ == "__main__":
    unittest.main(verbosity=1)

"""The ONLY python entry the plaso image may run (baked, allow-listed).

Imports the mounted custom output module so psort discovers it, then hands
psort the remaining argv verbatim:

    python3 /opt/dxdfir/psort_wrapper.py <module.py> <psort args...>
"""
import importlib.util
import sys

if len(sys.argv) < 2:
    sys.exit("usage: psort_wrapper.py <module.py> <psort args...>")

spec = importlib.util.spec_from_file_location("l2t_json_dxdfir", sys.argv[1])
if spec is None or spec.loader is None:
    sys.exit("psort_wrapper: not a loadable python module: %s" % sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

from plaso.scripts.psort import Main

sys.argv = ["psort.py"] + sys.argv[2:]
sys.exit(Main())

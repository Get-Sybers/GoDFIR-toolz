#!/bin/sh
# The ONLY script the signatures image runs for YARA (baked, allow-listed): a
# per-file scan over the mounted list with the mounted include index. No
# arguments accepted. (Carried over from the former get-sybers/yara image; sh
# remains in the consolidated image only for this loop.)
#
# Every file is scanned even if one fails, but failures are NOT silent: any
# yara invocation error (unreadable file, bad index) fails the whole run so a
# partial scan can never pass as a clean one. Match/no-match both exit 0.
set -eu
fail=0
while IFS= read -r f; do
    [ -n "$f" ] || continue
    yara -w -s -N /index.yar "$f" || fail=1
done < /list.txt
exit "$fail"

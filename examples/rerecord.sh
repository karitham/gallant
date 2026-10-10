#!/usr/bin/env bash
# Re-record every example demo GIF and commit the result.
#
# Run from anywhere. Each tape builds the CLI and lints with a model, so
# OPENCODE_API_KEY must be set, and VHS must be installed.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ -z "${OPENCODE_API_KEY:-}" ]; then
	echo "rerecord: OPENCODE_API_KEY is not set; the demos lint with a model" >&2
	exit 1
fi

for tape in examples/*/.vhs/demo.tape; do
	echo "rerecord: $tape"
	vhs "$tape"
done

git add examples/*/.vhs
if git diff --cached --quiet; then
	echo "rerecord: GIFs unchanged"
	exit 0
fi

# The GIFs are generated artifacts, so skip the source lint hook.
git commit --no-verify -m "Re-record example GIFs"

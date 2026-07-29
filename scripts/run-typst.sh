#!/usr/bin/env bash

set -euo pipefail

typst_bin="$(command -v typst)"

# The Snap launcher cannot read repositories mounted below /mnt, even when the
# underlying binary can. CI and non-Snap installations use the normal PATH.
if [[ $typst_bin == "/snap/bin/typst" && -x /snap/typst/current/bin/typst ]]; then
	typst_bin="/snap/typst/current/bin/typst"
fi

exec "$typst_bin" "$@"

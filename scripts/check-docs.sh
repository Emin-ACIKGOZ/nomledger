#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

if [ -e documentation.txt ]; then
	printf 'documentation.txt must not exist; use go doc instead\n' >&2
	exit 1
fi

if [ ! -f README.md ]; then
	printf 'README.md is missing\n' >&2
	exit 1
fi

if ! grep -q 'go doc' README.md; then
	printf 'README.md must point readers to go doc\n' >&2
	exit 1
fi

printf 'Documentation checks passed\n'

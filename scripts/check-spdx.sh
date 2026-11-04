#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
set -euo pipefail

status=0

while IFS= read -r -d '' file; do
	if ! grep -q 'SPDX-License-Identifier: MIT' "$file"; then
		printf 'Missing SPDX-License-Identifier: MIT in %s\n' "$file" >&2
		status=1
	fi
done < <(find . -name '*.go' -not -path './.git/*' -print0)

if [ "$status" -ne 0 ]; then
	exit "$status"
fi

printf 'SPDX check passed\n'

#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
demo_bin=$(mktemp -d)
trap 'rm -rf "$demo_bin"' EXIT
go build -o "$demo_bin/anigrass" ./docs/demo/fixture
PATH="$demo_bin:$PATH" vhs docs/demo/anigrass.tape

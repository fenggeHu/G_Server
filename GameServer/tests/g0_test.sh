#!/bin/sh
set -eu
root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
"$root/tools/sync_shared.sh"
godot --headless --path "$root" --script res://tests/g0_contract.gd

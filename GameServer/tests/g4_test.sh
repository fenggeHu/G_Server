#!/bin/sh
set -eu
root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
godot --headless --path "$root" --script res://tests/g4_contract.gd

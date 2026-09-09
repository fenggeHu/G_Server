#!/bin/sh
set -eu
root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
src="$root/../../3D_App/Shared/protocol.gd"
dst="$root/Shared/protocol.gd"
cp "$src" "$dst"
hash_src=$(shasum -a 256 "$src" | cut -d ' ' -f 1)
hash_dst=$(shasum -a 256 "$dst" | cut -d ' ' -f 1)
[ "$hash_src" = "$hash_dst" ] || exit 1
printf 'G0_PROTOCOL_SYNC_PASS sha256=%s\n' "$hash_dst"

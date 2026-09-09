#!/bin/sh
set -eu
case "$*" in
  *3D_App*) exec "$G2_REAL_GODOT" --script res://Client/tests/g2_peer.gd "$@" ;;
  *) exec "$G2_REAL_GODOT" "$@" ;;
esac

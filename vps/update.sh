#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

compose() {
	if [ "${GRUB_VPS_KICK:-0}" = "1" ]; then
		docker compose -f compose.yaml -f compose.https.yaml -f compose.kick.yaml "$@"
	else
		docker compose -f compose.yaml -f compose.https.yaml "$@"
	fi
}

echo "Pulling the latest GrubDrops image..."
compose pull miner
echo "Restarting the miner with the updated image..."
compose up -d --no-deps miner
echo "Update complete. Use docker compose with the same compose files to follow logs."

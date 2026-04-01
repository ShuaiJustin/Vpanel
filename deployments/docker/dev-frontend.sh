#!/bin/sh
set -eu

cd /workspace/web

lock_hash="$(sha256sum package-lock.json | awk '{print $1}')"
hash_file="node_modules/.package-lock.hash"

if [ ! -d node_modules ] || [ ! -f "${hash_file}" ] || [ "$(cat "${hash_file}" 2>/dev/null || true)" != "${lock_hash}" ]; then
  npm ci --legacy-peer-deps
  mkdir -p node_modules
  printf '%s' "${lock_hash}" > "${hash_file}"
fi

exec npm run dev -- --host 0.0.0.0 --port "${DEV_FRONTEND_PORT:-5173}"

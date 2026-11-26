#!/bin/sh
set -eu

cd /workspace

mkdir -p data logs xray tmp

if [ ! -f configs/config.yaml ] && [ -f configs/config.yaml.example ]; then
  cp configs/config.yaml.example configs/config.yaml
fi

if [ ! -f data/v.db ]; then
  touch data/v.db
fi

exec /go/bin/air -c deployments/docker/.air.toml

#!/bin/bash
# V Panel 快速启动脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/deployments/scripts/menu.sh" "$@"

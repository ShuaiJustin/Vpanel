#!/bin/bash

# API修复验证脚本
# 快速验证所有API修复是否正确应用

echo "=== API修复验证 ==="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 检查计数
PASS=0
FAIL=0

# 检查函数
check_fix() {
  local file=$1
  local pattern=$2
  local description=$3
  
  if grep -q "$pattern" "$file" 2>/dev/null; then
    echo -e "${GREEN}✓${NC} $description"
    ((PASS++))
  else
    echo -e "${RED}✗${NC} $description"
    ((FAIL++))
  fi
}

echo "检查前端API修复..."
echo ""

# 检查所有修复
check_fix "web/src/api/modules/giftcards.js" "/admin/gift-cards/stats" "礼品卡统计API路径"
check_fix "web/src/stores/portalStats.js" "async function fetchStats" "Portal Stats Store fetchStats方法"
check_fix "web/src/views/Monitor.vue" "/system/stats" "系统监控API路径"
check_fix "web/src/views/TrafficMonitor.vue" "/stats/traffic" "流量监控API路径"
check_fix "web/src/views/Traffic.vue" "/stats/user" "流量统计API路径"
check_fix "web/src/views/Settings.vue" "/settings/protocols" "协议管理API路径"
check_fix "web/src/components/XraySimpleManager.vue" "/settings/xray" "Xray设置API路径"

echo ""
echo "检查后端路由..."
echo ""

# 检查后端路由
check_fix "internal/api/routes.go" "adminReports.GET.*pause-stats.*pauseHandler.AdminGetPauseStats" "暂停统计路由"
check_fix "internal/api/routes.go" "adminGiftCards.GET.*stats.*giftCardHandler.AdminGetStats" "礼品卡统计路由"
check_fix "internal/api/routes.go" "portalProtected.GET.*stats/traffic.*portalStatsHandler.GetTrafficStats" "Portal流量统计路由"
check_fix "internal/api/routes.go" "portalProtected.GET.*stats/usage.*portalStatsHandler.GetUsageStats" "Portal使用统计路由"

echo ""
echo "=== 验证结果 ==="
echo -e "通过: ${GREEN}$PASS${NC}"
echo -e "失败: ${RED}$FAIL${NC}"
echo ""

if [ $FAIL -eq 0 ]; then
  echo -e "${GREEN}所有API修复已正确应用！${NC}"
  exit 0
else
  echo -e "${RED}部分修复未正确应用，请检查上述失败项${NC}"
  exit 1
fi

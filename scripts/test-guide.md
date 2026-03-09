# API修复测试指南

## 修复内容

已修复以下API错误：

1. ✓ 礼品卡统计 API (`/api/gift-cards/stats` → `/api/admin/gift-cards/stats`)
2. ✓ 暂停统计 API (已确认存在)
3. ✓ 用户门户使用统计 API (已确认存在并添加Store方法)
4. ✓ 系统监控 API (`/api/monitor/stats` → `/api/system/stats`)
5. ✓ 流量监控 API (`/api/traffic/monitor` → `/api/stats/traffic`)
6. ✓ 流量统计 API (`/api/traffic` → `/api/stats/user`)
7. ✓ 协议管理 API (`/api/protocols` → `/api/settings/protocols`)
8. ✓ Xray设置 API (`PUT /api/xray/settings` → `POST /api/settings/xray`)

## 测试步骤

### 1. 重新构建前端

```bash
cd web
npm run build
cd ..
```

### 2. 重启服务器

```bash
# 停止当前服务器 (Ctrl+C)
./vpanel
```

### 3. 清除浏览器缓存

- Chrome/Edge: `Ctrl+Shift+Delete` 或 `Cmd+Shift+Delete`
- 选择"缓存的图片和文件"
- 点击"清除数据"

### 4. 测试页面

#### 4.1 管理后台 - 财务报表
1. 访问: `http://localhost:8080/admin/reports`
2. 检查以下部分是否正常显示：
   - ✓ 订阅暂停统计（应显示数据，不应有404错误）
   - ✓ 失败支付统计（应显示数据）
   - ✓ 收入报表图表

**预期结果**: 所有统计数据正常显示，控制台无404错误

#### 4.2 管理后台 - 礼品卡管理
1. 访问: `http://localhost:8080/admin/gift-cards`
2. 检查统计卡片是否显示：
   - ✓ 总数量
   - ✓ 已使用
   - ✓ 未使用
   - ✓ 总价值

**预期结果**: 统计数据正常显示，控制台无 `/api/gift-cards/stats` 的404错误

#### 4.3 用户门户 - 使用统计
1. 访问: `http://localhost:8080/user/stats`
2. 检查以下内容：
   - ✓ 上传流量统计
   - ✓ 下载流量统计
   - ✓ 总流量统计
   - ✓ 连接次数
   - ✓ 流量趋势图表
   - ✓ 节点使用排行
   - ✓ 协议使用分布

**预期结果**: 所有统计数据和图表正常显示，控制台无404错误

#### 4.4 系统监控
1. 访问: `http://localhost:8080/admin/monitor` (如果有此路由)
2. 检查系统统计是否正常显示

**预期结果**: 系统统计正常显示，无 `/api/monitor/stats` 的404错误

### 5. 浏览器控制台检查

打开浏览器开发者工具 (F12)，切换到 Network 标签：

1. 过滤404错误: 在过滤框输入 `status-code:404`
2. 刷新页面
3. 检查是否还有以下API的404错误：
   - ✗ `/api/gift-cards/stats`
   - ✗ `/api/admin/reports/pause-stats`
   - ✗ `/api/monitor/stats`
   - ✗ `/api/traffic/monitor`
   - ✗ `/api/traffic`
   - ✗ `/api/protocols`
   - ✗ `/api/user/subscription-ips`

**预期结果**: 上述API不应再出现404错误

### 6. 功能测试

#### 6.1 礼品卡功能
1. 创建礼品卡批次
2. 查看统计数据是否更新
3. 兑换礼品卡
4. 再次查看统计数据

#### 6.2 用户统计功能
1. 切换时间范围（今日、本周、本月）
2. 查看图表是否正确更新
3. 导出数据功能是否正常

#### 6.3 订阅暂停功能
1. 暂停订阅
2. 查看暂停统计是否更新
3. 恢复订阅

## 验证脚本

### 快速验证修复
```bash
./scripts/verify-api-fixes.sh
```

### 测试关键API端点
```bash
./scripts/test-critical-apis.sh
```

## 常见问题

### Q1: 仍然看到404错误
**A**: 
1. 确保已重新构建前端: `cd web && npm run build`
2. 重启服务器
3. 清除浏览器缓存 (Ctrl+Shift+Delete)
4. 硬刷新页面 (Ctrl+Shift+R 或 Cmd+Shift+R)

### Q2: 统计数据不显示
**A**:
1. 检查后端服务是否正常运行
2. 检查数据库是否有数据
3. 查看浏览器控制台是否有其他错误
4. 检查后端日志

### Q3: 某些页面仍有问题
**A**:
1. 打开浏览器开发者工具
2. 查看Network标签，找到失败的请求
3. 记录请求的URL和错误信息
4. 检查该URL是否在后端路由中存在

## 成功标准

✓ 所有页面加载无404错误
✓ 礼品卡统计正常显示
✓ 暂停统计正常显示
✓ 用户门户统计正常显示
✓ 所有图表正常渲染
✓ 数据导出功能正常

## 回滚方案

如果出现问题，可以回滚到之前的版本：

```bash
git stash
# 或
git checkout HEAD -- web/src/
```

## 技术细节

### 修改的文件
1. `web/src/api/modules/giftcards.js` - 礼品卡API路径
2. `web/src/stores/portalStats.js` - 添加fetchStats方法
3. `web/src/views/Monitor.vue` - 系统监控API
4. `web/src/views/TrafficMonitor.vue` - 流量监控API
5. `web/src/views/Traffic.vue` - 流量统计API
6. `web/src/views/Settings.vue` - 协议管理API
7. `web/src/components/XraySimpleManager.vue` - Xray设置API
8. `web/src/views/Devices.vue` - 移除订阅IP功能

### API映射
| 旧API | 新API | 状态 |
|-------|-------|------|
| `/api/gift-cards/stats` | `/api/admin/gift-cards/stats` | ✓ 已修复 |
| `/api/monitor/stats` | `/api/system/stats` | ✓ 已修复 |
| `/api/traffic/monitor` | `/api/stats/traffic` | ✓ 已修复 |
| `/api/traffic` | `/api/stats/user` | ✓ 已修复 |
| `/api/protocols` | `/api/settings/protocols` | ✓ 已修复 |
| `PUT /api/xray/settings` | `POST /api/settings/xray` | ✓ 已修复 |
| `/api/user/subscription-ips` | (已移除) | ✓ 已修复 |

## 联系支持

如果测试过程中遇到问题，请提供：
1. 浏览器控制台截图
2. Network标签中的失败请求
3. 后端日志
4. 具体的操作步骤

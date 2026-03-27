const NODE_STATUS_TYPES = Object.freeze({
  online: "success",
  offline: "info",
  unhealthy: "danger",
});

const NODE_STATUS_TEXT = Object.freeze({
  online: "在线",
  offline: "离线",
  unhealthy: "不健康",
});

const REGION_PRESENTATION = Object.freeze([
  {
    aliases: ["hk", "hong kong", "hongkong", "香港"],
    label: "香港",
    flag: "🇭🇰",
  },
  { aliases: ["tw", "taiwan", "台湾"], label: "台湾", flag: "🇹🇼" },
  { aliases: ["jp", "japan", "日本"], label: "日本", flag: "🇯🇵" },
  { aliases: ["sg", "singapore", "新加坡"], label: "新加坡", flag: "🇸🇬" },
  {
    aliases: ["us", "usa", "united states", "美国"],
    label: "美国",
    flag: "🇺🇸",
  },
  {
    aliases: ["kr", "korea", "south korea", "韩国"],
    label: "韩国",
    flag: "🇰🇷",
  },
  { aliases: ["de", "germany", "德国"], label: "德国", flag: "🇩🇪" },
  {
    aliases: ["uk", "united kingdom", "britain", "英国"],
    label: "英国",
    flag: "🇬🇧",
  },
  { aliases: ["cn", "china", "中国"], label: "中国", flag: "🇨🇳" },
]);

const PROTOCOL_LABELS = Object.freeze({
  vmess: "VMess",
  vless: "VLESS",
  trojan: "Trojan",
  shadowsocks: "Shadowsocks",
  ss: "Shadowsocks",
});

const normalizeNodeRegionKey = (region) =>
  String(region || "")
    .trim()
    .toLowerCase();

const buildRegionCandidate = (region) => {
  const raw = normalizeNodeRegionKey(region);
  const phrase = raw
    .replace(/[\s\-_/.,，、()]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();

  return {
    raw,
    phrase,
    compact: phrase.replace(/\s+/g, ""),
    tokens: phrase ? phrase.split(" ") : [],
  };
};

const isShortAsciiAlias = (alias) => /^[a-z]{2,3}$/.test(alias);

const matchesRegionAlias = (candidate, alias) => {
  const aliasCandidate = buildRegionCandidate(alias);
  if (!candidate.raw || !aliasCandidate.raw) return false;

  const regionKeys = [
    candidate.raw,
    candidate.phrase,
    candidate.compact,
  ].filter(Boolean);
  const aliasKeys = [
    aliasCandidate.raw,
    aliasCandidate.phrase,
    aliasCandidate.compact,
  ].filter(Boolean);

  if (aliasKeys.some((key) => regionKeys.includes(key))) {
    return true;
  }

  if (candidate.tokens.some((token) => aliasKeys.includes(token))) {
    return true;
  }

  if (isShortAsciiAlias(aliasCandidate.compact)) {
    return false;
  }

  return (
    (candidate.phrase &&
      aliasCandidate.phrase &&
      candidate.phrase.includes(aliasCandidate.phrase)) ||
    (candidate.compact &&
      aliasCandidate.compact &&
      candidate.compact.includes(aliasCandidate.compact))
  );
};

const resolveRegionPresentation = (region) => {
  const candidate = buildRegionCandidate(region);
  if (!candidate.raw) return null;

  return (
    REGION_PRESENTATION.find((item) =>
      item.aliases.some((alias) => matchesRegionAlias(candidate, alias)),
    ) || null
  );
};

const NODE_SYNC_STATUS_TYPES = Object.freeze({
  synced: "success",
  pending: "warning",
  failed: "danger",
});

const NODE_SYNC_STATUS_TEXT = Object.freeze({
  synced: "已同步",
  pending: "待同步",
  failed: "同步失败",
});

const RECOVERY_STATUS_TYPES = Object.freeze({
  success: "success",
  failed: "danger",
  dispatched: "warning",
  queued: "info",
});

const RECOVERY_STATUS_TEXT = Object.freeze({
  success: "已恢复",
  failed: "恢复失败",
  dispatched: "已下发",
  queued: "已排队",
});

const RECOVERY_STATUS_COLORS = Object.freeze({
  success: "var(--el-color-success)",
  failed: "var(--el-color-danger)",
  dispatched: "var(--el-color-warning)",
  queued: "var(--el-color-info)",
});

const RECOVERY_SOURCE_TEXT = Object.freeze({
  heartbeat: "节点心跳",
  health_checker: "健康检查器",
  admin: "管理员",
  portal_ping: "用户入口探测",
});

const RECOVERY_COMMAND_TEXT = Object.freeze({
  xray_start: "启动 Xray",
  xray_restart: "重启 Xray",
  xray_status: "刷新 Xray 状态",
  config_sync: "同步节点配置",
});

const TRAFFIC_UNITS = Object.freeze([
  { unit: "B", value: 1 },
  { unit: "KiB", value: 1024 },
  { unit: "MiB", value: 1024 ** 2 },
  { unit: "GiB", value: 1024 ** 3 },
  { unit: "TiB", value: 1024 ** 4 },
]);

const NODE_TRAFFIC_STATE_TEXT = Object.freeze({
  unlimited: "不限流量",
  healthy: "可继续分配",
  soft_capped: "停止新分配",
  hard_capped: "已达上限",
});

const NODE_TRAFFIC_STATE_TYPES = Object.freeze({
  unlimited: "info",
  healthy: "success",
  soft_capped: "warning",
  hard_capped: "danger",
});

const normalizeTrafficBytes = (value) => {
  const normalized = Number(value);
  if (!Number.isFinite(normalized) || normalized <= 0) return 0;
  return normalized;
};

const resolveTrafficLimitValue = (nodeOrLimit, explicitLimit) => {
  if (typeof nodeOrLimit === "object" && nodeOrLimit !== null) {
    return normalizeTrafficBytes(nodeOrLimit.traffic_limit);
  }
  return normalizeTrafficBytes(explicitLimit ?? nodeOrLimit);
};

const resolveTrafficTotalValue = (nodeOrTotal) => {
  if (typeof nodeOrTotal === "object" && nodeOrTotal !== null) {
    return normalizeTrafficBytes(nodeOrTotal.traffic_total);
  }
  return normalizeTrafficBytes(nodeOrTotal);
};

const resolveTrafficThresholdPercent = (nodeOrThreshold) => {
  const rawValue =
    typeof nodeOrThreshold === "object" && nodeOrThreshold !== null
      ? Number(nodeOrThreshold.alert_traffic_threshold)
      : Number(nodeOrThreshold);

  if (!Number.isFinite(rawValue) || rawValue <= 0) return 100;
  if (rawValue > 100) return 100;
  return rawValue;
};

export const formatUsersLimitDisplay = (currentUsers, maxUsers) =>
  maxUsers
    ? `${Number(currentUsers) || 0} / ${maxUsers}`
    : `${Number(currentUsers) || 0} / ∞`;

export const getNodeStatusType = (status) =>
  NODE_STATUS_TYPES[status] || "info";

export const getNodeStatusText = (status) =>
  NODE_STATUS_TEXT[status] || status || "未知";

export const getNodeSyncStatusType = (status) =>
  NODE_SYNC_STATUS_TYPES[status] || "info";

export const getNodeSyncStatusText = (status) =>
  NODE_SYNC_STATUS_TEXT[status] || status || "未知";

export const getNodeLatencyClass = (latency) => {
  const value = Number(latency) || 0;
  if (value <= 0) return "";
  if (value < 100) return "latency-good";
  if (value < 300) return "latency-medium";
  return "latency-bad";
};

export const getRecoveryStatusType = (status) =>
  RECOVERY_STATUS_TYPES[status] || "info";

export const getRecoveryStatusText = (status) =>
  RECOVERY_STATUS_TEXT[status] || status || "未知";

export const getRecoveryStatusColor = (status) =>
  RECOVERY_STATUS_COLORS[status] || "var(--el-border-color)";

export const getRecoverySourceText = (source) =>
  RECOVERY_SOURCE_TEXT[source] || source || "系统";

export const getRecoveryCommandText = (commandType) =>
  RECOVERY_COMMAND_TEXT[commandType] || commandType || "未知命令";

export const formatNodeTime = (time) => {
  if (!time) return "-";
  return new Date(time).toLocaleString("zh-CN");
};

export const formatCoreVersion = (version) => {
  if (!version) return "-";
  return String(version).split("\n")[0];
};

export const formatCoreVersionCompact = (version) => {
  const normalized = formatCoreVersion(version);
  if (normalized === "-") return normalized;

  const matched = normalized.match(/(Xray\s+\d+(?:\.\d+)+)/i);
  return matched?.[1] || normalized;
};

export const parseNodeTags = (tags) => {
  if (Array.isArray(tags)) return tags;
  if (typeof tags === "string") {
    try {
      return JSON.parse(tags);
    } catch {
      return [];
    }
  }
  return [];
};

export const getNodeRegionFlag = (region) =>
  resolveRegionPresentation(region)?.flag || "🌐";

export const getNodeRegionLabel = (region) =>
  resolveRegionPresentation(region)?.label ||
  String(region || "").trim() ||
  "未知地区";

export const getProtocolDisplayName = (protocol) =>
  PROTOCOL_LABELS[
    String(protocol || "")
      .trim()
      .toLowerCase()
  ] ||
  String(protocol || "").trim() ||
  "未知协议";

export const formatNodeTrafficAmount = (bytes) => {
  const normalized = normalizeTrafficBytes(bytes);
  if (normalized === 0) return "0 B";

  let unit = TRAFFIC_UNITS[0];
  for (const candidate of TRAFFIC_UNITS) {
    if (normalized >= candidate.value) {
      unit = candidate;
    }
  }

  const value = normalized / unit.value;
  if (unit.unit === "B") {
    return `${Math.round(value)} ${unit.unit}`;
  }
  if (value >= 100) {
    return `${value.toFixed(0)} ${unit.unit}`;
  }
  if (value >= 10) {
    return `${value.toFixed(1)} ${unit.unit}`;
  }
  return `${value.toFixed(2)} ${unit.unit}`;
};

export const hasNodeTrafficLimit = (nodeOrLimit, explicitLimit) =>
  resolveTrafficLimitValue(nodeOrLimit, explicitLimit) > 0;

export const getNodeTrafficUsagePercent = (nodeOrTotal, explicitLimit) => {
  const total = resolveTrafficTotalValue(nodeOrTotal);
  const limit = resolveTrafficLimitValue(nodeOrTotal, explicitLimit);
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((total / limit) * 100));
};

export const getNodeTrafficRemaining = (nodeOrTotal, explicitLimit) => {
  const total = resolveTrafficTotalValue(nodeOrTotal);
  const limit = resolveTrafficLimitValue(nodeOrTotal, explicitLimit);
  if (limit <= 0) return 0;
  return Math.max(limit - total, 0);
};

export const getNodeTrafficThresholdPercent = (nodeOrThreshold) =>
  resolveTrafficThresholdPercent(nodeOrThreshold);

export const getNodeTrafficStateKey = (node) => {
  if (!hasNodeTrafficLimit(node)) return "unlimited";

  const total = resolveTrafficTotalValue(node);
  const limit = resolveTrafficLimitValue(node);
  if (total >= limit) return "hard_capped";

  const usagePercent = getNodeTrafficUsagePercent(node);
  if (usagePercent >= resolveTrafficThresholdPercent(node)) {
    return "soft_capped";
  }

  return "healthy";
};

export const getNodeTrafficStateText = (node) =>
  NODE_TRAFFIC_STATE_TEXT[getNodeTrafficStateKey(node)] || "未知";

export const getNodeTrafficStateType = (node) =>
  NODE_TRAFFIC_STATE_TYPES[getNodeTrafficStateKey(node)] || "info";

export const formatNodeTrafficUsageSummary = (node) => {
  const total = formatNodeTrafficAmount(resolveTrafficTotalValue(node));
  if (!hasNodeTrafficLimit(node)) {
    return `${total} / ∞`;
  }
  return `${total} / ${formatNodeTrafficAmount(resolveTrafficLimitValue(node))}`;
};

const addOneCalendarMonth = (value) => {
  const nextValue = new Date(value.getTime());
  nextValue.setMonth(nextValue.getMonth() + 1);
  return nextValue;
};

export const formatNodeTrafficResetAt = (value, now = new Date()) => {
  if (!value) return "未设置";

  const anchor = new Date(value);
  if (Number.isNaN(anchor.getTime())) return "未设置";

  const reference = now instanceof Date ? now : new Date(now);
  if (Number.isNaN(reference.getTime())) return "未设置";

  let nextResetAt = addOneCalendarMonth(anchor);
  let guard = 0;
  while (nextResetAt <= reference && guard < 120) {
    nextResetAt = addOneCalendarMonth(nextResetAt);
    guard += 1;
  }

  return nextResetAt.toLocaleString("zh-CN");
};

export const formatNodeTrafficRemaining = (node) =>
  hasNodeTrafficLimit(node)
    ? formatNodeTrafficAmount(getNodeTrafficRemaining(node))
    : "∞";

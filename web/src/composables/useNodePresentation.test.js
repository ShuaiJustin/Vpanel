import { describe, expect, it } from "vitest";

import {
  formatCoreVersionCompact,
  formatNodeTrafficAmount,
  formatNodeTrafficRemaining,
  formatNodeTrafficResetAt,
  formatNodeTrafficUsageSummary,
  formatUsersLimitDisplay,
  getNodeLatencyClass,
  getNodeRegionFlag,
  getNodeRegionLabel,
  getNodeStatusText,
  getNodeStatusType,
  getNodeSyncStatusText,
  getNodeTrafficStateText,
  getNodeTrafficStateType,
  getNodeTrafficUsagePercent,
  getProtocolDisplayName,
  hasNodeTrafficLimit,
  parseNodeTags,
} from "./useNodePresentation";

describe("useNodePresentation helpers", () => {
  it("formats user limits with unlimited fallback", () => {
    expect(formatUsersLimitDisplay(12, 48)).toBe("12 / 48");
    expect(formatUsersLimitDisplay(7, 0)).toBe("7 / ∞");
  });

  it("maps node status labels and types", () => {
    expect(getNodeStatusType("online")).toBe("success");
    expect(getNodeStatusText("offline")).toBe("离线");
    expect(getNodeSyncStatusText("pending")).toBe("待同步");
  });

  it("normalizes latency classes", () => {
    expect(getNodeLatencyClass(0)).toBe("");
    expect(getNodeLatencyClass(88)).toBe("latency-good");
    expect(getNodeLatencyClass(180)).toBe("latency-medium");
    expect(getNodeLatencyClass(360)).toBe("latency-bad");
  });

  it("extracts compact core version text", () => {
    expect(formatCoreVersionCompact("Xray 1.8.24\nBuild")).toBe("Xray 1.8.24");
    expect(formatCoreVersionCompact("custom-version")).toBe("custom-version");
  });

  it("parses tag arrays safely", () => {
    expect(parseNodeTags('["a","b"]')).toEqual(["a", "b"]);
    expect(parseNodeTags("invalid")).toEqual([]);
  });

  it("maps region and protocol presentation helpers", () => {
    expect(getNodeRegionFlag("日本")).toBe("🇯🇵");
    expect(getNodeRegionLabel("jp")).toBe("日本");
    expect(getNodeRegionLabel("中国")).toBe("中国");
    expect(getProtocolDisplayName("vmess")).toBe("VMess");
    expect(getProtocolDisplayName("shadowsocks")).toBe("Shadowsocks");
  });

  it("matches composite region strings for portal display", () => {
    expect(getNodeRegionFlag("中国-Shanghai")).toBe("🇨🇳");
    expect(getNodeRegionLabel("CN-Shanghai")).toBe("中国");
    expect(getNodeRegionFlag("Hong Kong 01")).toBe("🇭🇰");
    expect(getNodeRegionLabel("Shanghai, China")).toBe("中国");
  });

  it("formats node traffic next reset time from the cycle anchor", () => {
    const anchor = "2026-03-27T00:22:38Z";

    expect(
      formatNodeTrafficResetAt(anchor, new Date("2026-03-27T10:00:00Z")),
    ).toBe(new Date("2026-04-27T00:22:38Z").toLocaleString("zh-CN"));

    expect(
      formatNodeTrafficResetAt(anchor, new Date("2026-05-28T00:00:00Z")),
    ).toBe(new Date("2026-06-27T00:22:38Z").toLocaleString("zh-CN"));

    expect(formatNodeTrafficResetAt("invalid-date")).toBe("未设置");
  });

  it("formats node traffic quota helpers", () => {
    const healthyNode = {
      traffic_total: 512 * 1024 ** 3,
      traffic_limit: 1024 * 1024 ** 3,
      alert_traffic_threshold: 80,
    };
    const softCappedNode = {
      traffic_total: 820 * 1024 ** 3,
      traffic_limit: 1000 * 1024 ** 3,
      alert_traffic_threshold: 80,
    };
    const hardCappedNode = {
      traffic_total: 1024 * 1024 ** 3,
      traffic_limit: 1024 * 1024 ** 3,
      alert_traffic_threshold: 80,
    };

    expect(formatNodeTrafficAmount(0)).toBe("0 B");
    expect(formatNodeTrafficAmount(1024 ** 3)).toBe("1.00 GiB");
    expect(hasNodeTrafficLimit(healthyNode)).toBe(true);
    expect(getNodeTrafficUsagePercent(healthyNode)).toBe(50);
    expect(getNodeTrafficStateText(healthyNode)).toBe("可继续分配");
    expect(getNodeTrafficStateType(softCappedNode)).toBe("warning");
    expect(getNodeTrafficStateText(softCappedNode)).toBe("停止新分配");
    expect(getNodeTrafficStateText(hardCappedNode)).toBe("已达上限");
    expect(formatNodeTrafficUsageSummary(healthyNode)).toBe(
      "512 GiB / 1.00 TiB",
    );
    expect(formatNodeTrafficRemaining(healthyNode)).toBe("512 GiB");
  });
});

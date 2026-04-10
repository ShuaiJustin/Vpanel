import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, shallowMount } from "@vue/test-utils";

vi.mock("element-plus", () => ({
  ElMessage: {
    error: vi.fn(),
    success: vi.fn(),
  },
  ElMessageBox: {
    confirm: vi.fn(() => Promise.resolve()),
  },
}));

vi.mock("@/api/index", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

import api from "@/api/index";
import Devices from "./Devices.vue";

function createDeferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });

  return { promise, resolve, reject };
}

function mountDevices() {
  return shallowMount(Devices, {
    global: {
      directives: {
        loading: {},
      },
      stubs: {
        "el-card": {
          template: '<div><slot name="header" /><slot /></div>',
        },
        "el-progress": true,
        "el-button": {
          template: "<button><slot /></button>",
        },
        "el-icon": {
          template: "<i><slot /></i>",
        },
        "el-empty": {
          template: "<div><slot /></div>",
        },
        "el-tag": {
          template: "<span><slot /></span>",
        },
        "el-table": {
          template: "<div />",
        },
        "el-table-column": true,
        "el-pagination": {
          template: "<div />",
        },
        "el-tooltip": {
          template: "<div><slot /></div>",
        },
      },
    },
  });
}

describe("Devices view", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "visible",
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("deduplicates devices auto-refresh while a request is still in flight", async () => {
    const deferredDevices = createDeferred();

    api.get.mockImplementation((url) => {
      if (url === "/user/devices") {
        return deferredDevices.promise;
      }
      if (url === "/user/ip-history") {
        return Promise.resolve({
          data: {
            list: [],
            total: 0,
          },
        });
      }
      throw new Error(`Unexpected URL: ${url}`);
    });

    const wrapper = mountDevices();
    await flushPromises();

    const deviceCalls = () =>
      api.get.mock.calls.filter(([url]) => url === "/user/devices");
    expect(deviceCalls()).toHaveLength(1);
    expect(deviceCalls()[0][1]).toEqual(
      expect.objectContaining({
        cancelKey: "user-devices-page-list",
      }),
    );

    await vi.advanceTimersByTimeAsync(60 * 1000);
    await flushPromises();
    expect(deviceCalls()).toHaveLength(1);

    deferredDevices.resolve({
      data: {
        devices: [],
        max_devices: 0,
      },
    });
    await flushPromises();

    await vi.advanceTimersByTimeAsync(60 * 1000);
    await flushPromises();
    expect(deviceCalls()).toHaveLength(2);

    wrapper.unmount();
  });

  it("shows refresh hints that match the current refresh behavior", async () => {
    api.get.mockImplementation((url) => {
      if (url === "/user/devices") {
        return Promise.resolve({
          data: {
            devices: [],
            max_devices: 0,
          },
        });
      }
      if (url === "/user/ip-history") {
        return Promise.resolve({
          data: {
            list: [],
            total: 0,
          },
        });
      }
      throw new Error(`Unexpected URL: ${url}`);
    });

    const wrapper = mountDevices();
    await flushPromises();

    expect(wrapper.text()).toContain("约每 60 秒自动刷新");
    expect(wrapper.text()).toContain("页面恢复可见时自动刷新");

    wrapper.unmount();
  });
});

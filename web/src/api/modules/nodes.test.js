import { describe, expect, it, vi } from 'vitest';
vi.mock('../base', () => ({ default: { post: vi.fn() } }));
import api from '../base';
import { nodesApi } from './nodes';
describe('network inspection timeout', () => {
 it('allows the SSH connection and remote command to report their own errors', () => {
  nodesApi.inspectNetworkOptimization(23, { ssh: {} });
  const [path, payload, options] = api.post.mock.calls.at(-1);
  expect(path).toBe('/admin/nodes/23/network-optimization/inspect');
  expect(payload).toEqual({ ssh: {} });
  expect(options.silent).toBe(true);
  expect(options.timeout).toBeGreaterThanOrEqual(90000);
 });
});

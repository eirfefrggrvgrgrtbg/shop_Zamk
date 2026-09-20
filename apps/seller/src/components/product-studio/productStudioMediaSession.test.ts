import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createStudioMediaRegistry } from './productStudioMediaSession';

describe('productStudioMediaSession', () => {
  const originalCreateObjectURL = globalThis.URL.createObjectURL;
  const originalRevokeObjectURL = globalThis.URL.revokeObjectURL;

  beforeEach(() => {
    let id = 0;
    globalThis.URL.createObjectURL = vi.fn(() => `blob:http://localhost/test-${++id}`);
    globalThis.URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    globalThis.URL.createObjectURL = originalCreateObjectURL;
    globalThis.URL.revokeObjectURL = originalRevokeObjectURL;
  });

  it('creates and tracks object URLs', () => {
    const registry = createStudioMediaRegistry();
    const file1 = new File(['dummy1'], 'photo1.jpg', { type: 'image/jpeg' });
    const url1 = registry.createObjectUrl(file1);

    expect(url1).toBe('blob:http://localhost/test-1');
    expect(registry.isRegistered(url1)).toBe(true);
    expect(registry.getActiveUrls()).toEqual([url1]);
  });

  it('revokes tracked object URL exactly once and ignores unregistered URLs', () => {
    const registry = createStudioMediaRegistry();
    const file1 = new File(['dummy1'], 'photo1.jpg', { type: 'image/jpeg' });
    const url1 = registry.createObjectUrl(file1);

    registry.revokeObjectUrl(url1);
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledWith(url1);
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledTimes(1);
    expect(registry.isRegistered(url1)).toBe(false);

    // Second call is ignored (prevents double revoke)
    registry.revokeObjectUrl(url1);
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledTimes(1);

    // Unregistered URL is ignored
    registry.revokeObjectUrl('blob:http://localhost/other');
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledTimes(1);
  });

  it('revokeAll cleans up all active object URLs', () => {
    const registry = createStudioMediaRegistry();
    const file1 = new File(['1'], 'p1.jpg', { type: 'image/jpeg' });
    const file2 = new File(['2'], 'p2.jpg', { type: 'image/jpeg' });
    const u1 = registry.createObjectUrl(file1);
    const u2 = registry.createObjectUrl(file2);

    expect(registry.getActiveUrls()).toHaveLength(2);

    registry.revokeAll();

    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledWith(u1);
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledWith(u2);
    expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledTimes(2);
    expect(registry.getActiveUrls()).toHaveLength(0);
  });
});

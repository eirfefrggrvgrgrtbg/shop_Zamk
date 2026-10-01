import { describe, it, expect, vi } from 'vitest';
import { deleteSellerProduct, archiveSellerProduct } from '@zamk/api-client/src/seller';

describe('SELLER ASSORTMENT.1C2 - Canonical API Endpoints', () => {
  it('A: deleteSellerProduct calls DELETE canonical endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      headers: { get: () => null },
      text: async () => '',
    });
    global.fetch = fetchMock;

    await deleteSellerProduct('prod-123');

    expect(fetchMock).toHaveBeenCalled();
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('http://127.0.0.1:8080/api/seller/products/prod-123');
    expect(options.method).toBe('DELETE');
  });

  it('B: archiveSellerProduct calls POST canonical archive endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: { get: () => 'application/json' },
      json: async () => ({}),
      text: async () => '{}',
    });
    global.fetch = fetchMock;

    await archiveSellerProduct('prod-456');

    expect(fetchMock).toHaveBeenCalled();
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('http://127.0.0.1:8080/api/seller/products/prod-456/archive');
    expect(options.method).toBe('POST');
  });
});

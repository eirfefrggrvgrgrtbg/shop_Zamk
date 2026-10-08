import { describe, it, expect, vi, afterEach } from 'vitest';

vi.mock('@zamk/api-client/src/tokenStore', () => ({
  getAccessToken: () => 'test-token',
}));

afterEach(() => {
  vi.restoreAllMocks();
});

const mockFetchResponse = (ok: boolean, status: number, body: any) => {
  global.fetch = vi.fn().mockResolvedValue({
    ok,
    status,
    headers: { get: () => 'application/json' },
    json: () => Promise.resolve(body),
  } as unknown as Response);
};

describe('Admin Marketing Saved Queries API Client', () => {
  it('listAdminMarketingSavedQueries uses GET /admin/marketing/saved-queries', async () => {
    mockFetchResponse(true, 200, [{ id: '1' }]);
    const { listAdminMarketingSavedQueries } = await import('@zamk/api-client/src/admin');
    await listAdminMarketingSavedQueries();
    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect((global.fetch as any).mock.calls[0][0]).toContain('/admin/marketing/saved-queries');
  });

  it('getAdminMarketingSavedQuery uses GET /admin/marketing/saved-queries/:id', async () => {
    mockFetchResponse(true, 200, { id: '1' });
    const { getAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await getAdminMarketingSavedQuery('123');
    expect((global.fetch as any).mock.calls[0][0]).toContain('/admin/marketing/saved-queries/123');
    expect((global.fetch as any).mock.calls[0][0]).not.toContain('/api/api');
  });

  it('createAdminMarketingSavedQuery uses POST /admin/marketing/saved-queries', async () => {
    mockFetchResponse(true, 201, { id: '2' });
    const { createAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await createAdminMarketingSavedQuery({ name: 'test', querySpec: {} as any });
    expect((global.fetch as any).mock.calls[0][1].method).toBe('POST');
  });

  it('updateAdminMarketingSavedQuery uses PATCH /admin/marketing/saved-queries/:id', async () => {
    mockFetchResponse(true, 200, { id: '3' });
    const { updateAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await updateAdminMarketingSavedQuery('456', { description: 'new' });
    expect((global.fetch as any).mock.calls[0][1].method).toBe('PATCH');
  });

  it('deleteAdminMarketingSavedQuery uses DELETE /admin/marketing/saved-queries/:id', async () => {
    mockFetchResponse(true, 204, {});
    const { deleteAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await deleteAdminMarketingSavedQuery('789');
    expect((global.fetch as any).mock.calls[0][1].method).toBe('DELETE');
  });

  it('propagates 400 typed ApiError', async () => {
    mockFetchResponse(false, 400, { error: { message: 'Bad', code: 'bad_request' } });
    const { getAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await expect(getAdminMarketingSavedQuery('123')).rejects.toThrow('Bad');
  });

  it('propagates 404 typed ApiError', async () => {
    mockFetchResponse(false, 404, { error: { message: 'Not found', code: 'not_found' } });
    const { getAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await expect(getAdminMarketingSavedQuery('123')).rejects.toThrow('Ресурс не найден');
  });

  it('propagates 409 typed ApiError', async () => {
    mockFetchResponse(false, 409, { error: { message: 'Conflict', code: 'conflict' } });
    const { createAdminMarketingSavedQuery } = await import('@zamk/api-client/src/admin');
    await expect(createAdminMarketingSavedQuery({ name: 'x', querySpec: {} as any })).rejects.toThrow('Conflict');
  });
});

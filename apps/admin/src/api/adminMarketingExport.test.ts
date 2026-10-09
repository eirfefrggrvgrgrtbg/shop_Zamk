import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { exportAdminMarketingQuery } from '@zamk/api-client/src/admin';
import { requestBlob } from '@zamk/api-client/src/client';
import { ApiError } from '@zamk/api-client/src/errors';
import * as tokenStore from '@zamk/api-client/src/tokenStore';
import type { QueryRequest } from '@zamk/api-client/src/types';

describe('adminMarketingExport & requestBlob', () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    tokenStore.clearAccessToken();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  const sampleQuery: QueryRequest = {
    version: 1,
    period: { from: '2026-09-01', to: '2026-09-30' },
    dimensions: ['source'],
    metrics: ['revenue'],
  };

  it('1. exportAdminMarketingQuery posts exact URL without /api/api and correct payload for CSV', async () => {
    let capturedUrl = '';
    let capturedMethod = '';
    let capturedBody: any = null;

    global.fetch = vi.fn().mockImplementation(async (url: string, options: any) => {
      capturedUrl = url;
      capturedMethod = options.method;
      capturedBody = options.body ? JSON.parse(options.body) : null;
      return {
        ok: true,
        status: 200,
        headers: new Headers({
          'Content-Type': 'text/csv; charset=utf-8',
          'Content-Disposition': 'attachment; filename="zamk-marketing-report-2026-10-09.csv"',
        }),
        blob: async () => new Blob(['col1;col2\nval1;val2'], { type: 'text/csv; charset=utf-8' }),
      } as any;
    });

    const res = await exportAdminMarketingQuery('csv', sampleQuery);

    expect(capturedUrl).not.toContain('/api/api/');
    expect(capturedUrl).toBe('http://127.0.0.1:8080/api/admin/marketing/analytics/export');
    expect(capturedMethod).toBe('POST');
    expect(capturedBody).toEqual({
      format: 'csv',
      query: sampleQuery,
    });
    expect(res.filename).toBe('zamk-marketing-report-2026-10-09.csv');
    expect(res.contentType).toBe('text/csv; charset=utf-8');
    expect(res.blob).toBeInstanceOf(Blob);
  });

  it('2. exportAdminMarketingQuery posts exact payload for XLSX and parses filename', async () => {
    let capturedBody: any = null;

    global.fetch = vi.fn().mockImplementation(async (_url: string, options: any) => {
      capturedBody = options.body ? JSON.parse(options.body) : null;
      return {
        ok: true,
        status: 200,
        headers: new Headers({
          'Content-Type': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
          'Content-Disposition': 'attachment; filename="zamk-marketing-report-2026-10-09.xlsx"',
        }),
        blob: async () => new Blob([new Uint8Array([0x50, 0x4b])], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
      } as any;
    });

    const res = await exportAdminMarketingQuery('xlsx', sampleQuery);

    expect(capturedBody).toEqual({
      format: 'xlsx',
      query: sampleQuery,
    });
    expect(res.filename).toBe('zamk-marketing-report-2026-10-09.xlsx');
    expect(res.contentType).toBe('application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
    expect(res.blob).toBeInstanceOf(Blob);
  });

  it('3. Content-Disposition parsing safely falls back when header is missing or lacks filename', async () => {
    global.fetch = vi.fn().mockImplementation(async () => {
      return {
        ok: true,
        status: 200,
        headers: new Headers({
          'Content-Type': 'text/csv; charset=utf-8',
        }),
        blob: async () => new Blob(['header;row'], { type: 'text/csv' }),
      } as any;
    });

    const res = await requestBlob('POST', '/admin/marketing/analytics/export', {
      body: { format: 'csv', query: sampleQuery },
    });

    expect(res.filename).toBe('download');
    expect(res.filename).not.toContain('attachment');
  });

  it('4. requestBlob propagates HTTP 400 error as typed ApiError', async () => {
    global.fetch = vi.fn().mockImplementation(async () => {
      return {
        ok: false,
        status: 400,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ code: 'invalid_export_format', message: 'unsupported export format: pdf' }),
      } as any;
    });

    await expect(
      requestBlob('POST', '/admin/marketing/analytics/export', {
        body: { format: 'pdf', query: sampleQuery },
      })
    ).rejects.toThrow(ApiError);

    try {
      await requestBlob('POST', '/admin/marketing/analytics/export', {
        body: { format: 'pdf', query: sampleQuery },
      });
    } catch (err: any) {
      expect(err).toBeInstanceOf(ApiError);
      expect(err.status).toBe(400);
      expect(err.code).toBe('invalid_export_format');
    }
  });

  it('5. requestBlob propagates HTTP 403 error as typed ApiError', async () => {
    global.fetch = vi.fn().mockImplementation(async () => {
      return {
        ok: false,
        status: 403,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ code: 'permission_denied', message: 'insufficient permissions' }),
      } as any;
    });

    try {
      await requestBlob('POST', '/admin/marketing/analytics/export', {
        body: { format: 'csv', query: sampleQuery },
      });
      expect.unreachable('Should have thrown ApiError');
    } catch (err: any) {
      expect(err).toBeInstanceOf(ApiError);
      expect(err.status).toBe(403);
      expect(err.code).toBe('permission_denied');
    }
  });

  it('6. requestBlob executes token refresh on 401 and retries once successfully', async () => {
    tokenStore.setAccessToken('expired-token');
    let callCount = 0;

    global.fetch = vi.fn().mockImplementation(async (url: string) => {
      callCount++;
      if (url.includes('/auth/refresh')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({ accessToken: 'refreshed-token' }),
        } as any;
      }

      if (callCount === 1) {
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({ code: 'unauthorized', message: 'token expired' }),
        } as any;
      }

      return {
        ok: true,
        status: 200,
        headers: new Headers({
          'Content-Type': 'text/csv; charset=utf-8',
          'Content-Disposition': 'attachment; filename="zamk-marketing-report-2026-10-09.csv"',
        }),
        blob: async () => new Blob(['refreshed data'], { type: 'text/csv' }),
      } as any;
    });

    const res = await requestBlob('POST', '/admin/marketing/analytics/export', {
      body: { format: 'csv', query: sampleQuery },
    });

    expect(callCount).toBe(3); // Initial request (401), /auth/refresh, retry (200)
    expect(res.blob).toBeInstanceOf(Blob);
    expect(res.filename).toBe('zamk-marketing-report-2026-10-09.csv');
    expect(tokenStore.getAccessToken()).toBe('refreshed-token');
  });

  it('7. requestBlob fails on 401 when refresh fails without infinite loop', async () => {
    tokenStore.setAccessToken('expired-token');
    let callCount = 0;

    global.fetch = vi.fn().mockImplementation(async (url: string) => {
      callCount++;
      if (url.includes('/auth/refresh')) {
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({ code: 'SESSION_EXPIRED', message: 'refresh failed' }),
        } as any;
      }

      return {
        ok: false,
        status: 401,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ code: 'unauthorized', message: 'token expired' }),
      } as any;
    });

    try {
      await requestBlob('POST', '/admin/marketing/analytics/export', {
        body: { format: 'csv', query: sampleQuery },
      });
      expect.unreachable('Should have thrown ApiError');
    } catch (err: any) {
      expect(err).toBeInstanceOf(ApiError);
      expect(err.status).toBe(401);
    }

    expect(callCount).toBe(2); // Initial request (401) + refresh attempt (401)
    expect(tokenStore.getAccessToken()).toBeNull();
  });
});

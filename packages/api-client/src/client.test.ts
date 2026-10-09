import assert from 'assert';
import { request } from './client.js';
import * as tokenStore from './tokenStore.js';

async function runTests() {
  let fetchCallCount = 0;
  let currentStatus = 401;
  let loginStatus = 401;

  (global as any).fetch = async (url: string, options: any) => {
    fetchCallCount++;
    const path = url.replace('http://127.0.0.1:8080/api', '');
    const headers = { get: () => 'application/json' };

    if (path === '/auth/login') {
      return { ok: loginStatus === 200, status: loginStatus, headers, json: async () => ({code:'INVALID_CREDENTIALS', message:'invalid'}) };
    }
    if (path === '/auth/refresh') {
      currentStatus = 200; // FIX: automatically succeed next requests after refresh
      return { ok: true, status: 200, headers, json: async () => ({accessToken:'new-token'}) };
    }
    return { ok: currentStatus === 200, status: currentStatus, headers, json: async () => ({code:'ERROR', message:'error'}) };
  };

  tokenStore.setAccessToken('old-token');

  console.log('Testing concurrent 401 requests...');
  fetchCallCount = 0;
  currentStatus = 401;

  const req1 = request('GET', '/some/endpoint');
  const req2 = request('GET', '/another/endpoint');

  await Promise.all([req1, req2]);
  assert.strictEqual(fetchCallCount, 5, `Expected 5 fetch calls, got ${fetchCallCount}`);

  console.log('Testing login 401 invalid credentials does NOT trigger refresh...');
  fetchCallCount = 0;
  loginStatus = 401;
  try {
    await request('POST', '/auth/login', { body: {} });
  } catch (err: any) {
    assert.strictEqual(err.message, 'Неверный email или пароль');
  }
  assert.strictEqual(fetchCallCount, 1, `Expected 1 fetch call, got ${fetchCallCount}`);

  console.log('Testing invalid refresh -> no infinite loop...');
  fetchCallCount = 0;
  currentStatus = 401;
  (global as any).fetch = async (url: string, options: any) => {
    fetchCallCount++;
    const headers = { get: () => 'application/json' };
    return { ok: false, status: 401, headers, json: async () => ({code:'UNAUTHORIZED'}) };
  };

  try {
    await request('GET', '/some/endpoint');
  } catch (err: any) {
    // Should throw HTTP_ERROR
  }
  console.log('Testing getAdminMarketingProducts path has no double /api...');
  let requestedUrl = '';
  (global as any).fetch = async (url: string, options: any) => {
    requestedUrl = url;
    const headers = { get: () => 'application/json' };
    return {
      ok: true,
      status: 200,
      headers,
      json: async () => ({ coverage: { views: { status: 'available' } }, products: [] }),
    };
  };
  const { getAdminMarketingProducts } = await import('./admin.js');
  await getAdminMarketingProducts('2026-09-01T00:00:00Z', '2026-10-01T00:00:00Z', 'revenue');
  assert.ok(!requestedUrl.includes('/api/api/'), `URL must not contain /api/api/, got: ${requestedUrl}`);
  console.log('Testing executeAdminMarketingQuery path and payload...');
  let queryRequestUrl = '';
  let queryRequestMethod = '';
  let queryRequestBody: any = null;
  (global as any).fetch = async (url: string, options: any) => {
    queryRequestUrl = url;
    queryRequestMethod = options.method;
    queryRequestBody = options.body ? JSON.parse(options.body) : null;
    const headers = { get: () => 'application/json' };
    return {
      ok: true,
      status: 200,
      headers,
      json: async () => ({
        version: 1,
        query: queryRequestBody,
        columns: [{ key: 'source', kind: 'dimension', type: 'string' }],
        rows: [],
        coverage: {},
        warnings: [],
      }),
    };
  };
  const { executeAdminMarketingQuery } = await import('./admin.js');
  const payload = {
    version: 1,
    period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' },
    dimensions: ['source'],
    metrics: ['orders', 'revenue'],
    filters: [{ dimension: 'source', operator: 'eq', values: ['direct'] }],
    sort: [{ field: 'orders', direction: 'desc' as const }],
    limit: 100,
  };
  const queryRes = await executeAdminMarketingQuery(payload);
  assert.ok(!queryRequestUrl.includes('/api/api/'), `URL must not contain /api/api/, got: ${queryRequestUrl}`);
  assert.strictEqual(queryRequestUrl, 'http://127.0.0.1:8080/api/admin/marketing/analytics/query', `URL must be exact endpoint, got: ${queryRequestUrl}`);
  assert.strictEqual(queryRequestMethod, 'POST', `Method must be POST, got: ${queryRequestMethod}`);
  assert.strictEqual(queryRequestBody.version, 1);
  assert.deepStrictEqual(queryRequestBody.period, payload.period);
  assert.deepStrictEqual(queryRequestBody.dimensions, ['source']);
  assert.deepStrictEqual(queryRequestBody.metrics, ['orders', 'revenue']);
  assert.deepStrictEqual(queryRequestBody.filters, payload.filters);
  assert.deepStrictEqual(queryRequestBody.sort, payload.sort);
  assert.strictEqual(queryRequestBody.limit, 100);
  assert.strictEqual(queryRes.version, 1);

  console.log('Testing executeAdminMarketingQuery error propagation...');
  (global as any).fetch = async () => {
    const headers = { get: () => 'application/json' };
    return {
      ok: false,
      status: 400,
      headers,
      json: async () => ({ error: 'unsupported_combination', message: 'metric sessions is unsupported with dimensions' }),
    };
  };
  let errorCaught: any = null;
  try {
    await executeAdminMarketingQuery(payload);
  } catch (err: any) {
    errorCaught = err;
  }
  assert.ok(errorCaught, 'Error must be thrown');
  assert.strictEqual(errorCaught.status, 400);
  assert.strictEqual(errorCaught.code, 'unsupported_combination');

  console.log('Testing exportAdminMarketingQuery path, payload, and content-disposition...');
  const { exportAdminMarketingQuery } = await import('./admin.js');
  const { requestBlob } = await import('./client.js');
  let exportUrl = '';
  let exportMethod = '';
  let exportBody: any = null;

  (global as any).fetch = async (url: string, options: any) => {
    exportUrl = url;
    exportMethod = options.method;
    exportBody = options.body ? JSON.parse(options.body) : null;
    return {
      ok: true,
      status: 200,
      headers: {
        get: (h: string) => {
          if (h.toLowerCase() === 'content-type') return 'text/csv; charset=utf-8';
          if (h.toLowerCase() === 'content-disposition') return 'attachment; filename="zamk-marketing-report-2026-10-09.csv"';
          return null;
        },
      },
      blob: async () => ({ size: 42, type: 'text/csv' }),
    };
  };

  const expRes = await exportAdminMarketingQuery('csv', payload);
  assert.ok(!exportUrl.includes('/api/api/'), `Export URL must not contain /api/api/, got: ${exportUrl}`);
  assert.strictEqual(exportUrl, 'http://127.0.0.1:8080/api/admin/marketing/analytics/export');
  assert.strictEqual(exportMethod, 'POST');
  assert.strictEqual(exportBody.format, 'csv');
  assert.deepStrictEqual(exportBody.query, payload);
  assert.strictEqual(expRes.filename, 'zamk-marketing-report-2026-10-09.csv');
  assert.strictEqual(expRes.contentType, 'text/csv; charset=utf-8');

  console.log('Testing requestBlob error propagation (400, 403)...');
  (global as any).fetch = async () => {
    return {
      ok: false,
      status: 400,
      headers: { get: () => 'application/json' },
      json: async () => ({ code: 'invalid_format', message: 'unsupported format' }),
    };
  };
  let blobErr: any = null;
  try {
    await requestBlob('POST', '/admin/marketing/analytics/export', { body: {} });
  } catch (err: any) {
    blobErr = err;
  }
  assert.ok(blobErr, 'requestBlob must throw on 400');
  assert.strictEqual(blobErr.status, 400);
  assert.strictEqual(blobErr.code, 'invalid_format');

  (global as any).fetch = async () => {
    return {
      ok: false,
      status: 403,
      headers: { get: () => 'application/json' },
      json: async () => ({ code: 'permission_denied', message: 'denied' }),
    };
  };
  blobErr = null;
  try {
    await requestBlob('POST', '/admin/marketing/analytics/export', { body: {} });
  } catch (err: any) {
    blobErr = err;
  }
  assert.ok(blobErr, 'requestBlob must throw on 403');
  assert.strictEqual(blobErr.status, 403);
  assert.strictEqual(blobErr.code, 'permission_denied');

  console.log('Testing requestBlob 401 refresh token retry...');
  let blobCallCount = 0;
  tokenStore.setAccessToken('old-token');
  (global as any).fetch = async (url: string) => {
    blobCallCount++;
    const path = url.replace('http://127.0.0.1:8080/api', '');
    if (path === '/auth/refresh') {
      return { ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({ accessToken: 'new-token' }) };
    }
    if (blobCallCount === 1) {
      return { ok: false, status: 401, headers: { get: () => 'application/json' }, json: async () => ({ code: 'UNAUTHORIZED' }) };
    }
    return {
      ok: true,
      status: 200,
      headers: { get: () => 'text/csv' },
      blob: async () => ({ size: 10 }),
    };
  };
  const refreshedBlobRes = await requestBlob('POST', '/some/endpoint', { body: {} });
  assert.strictEqual(blobCallCount, 3);
  assert.strictEqual(refreshedBlobRes.filename, 'download');

  console.log('ALL TESTS PASSED');
}

runTests().catch(err => {
  console.error(err);
  process.exit(1);
});

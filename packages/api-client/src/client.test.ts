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

  console.log('ALL TESTS PASSED');
}

runTests().catch(err => {
  console.error(err);
  process.exit(1);
});

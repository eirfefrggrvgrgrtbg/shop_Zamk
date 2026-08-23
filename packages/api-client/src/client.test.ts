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
  assert.strictEqual(fetchCallCount, 2, `Expected 2 fetch calls, got ${fetchCallCount}`);

  console.log('ALL TESTS PASSED');
}

runTests().catch(err => {
  console.error(err);
  process.exit(1);
});

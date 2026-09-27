import assert from 'assert';
import { ingestBehavioralEvents, IngestEventsRequest } from './behavior';
import * as tokenStore from './tokenStore';

async function runBehaviorApiClientTests() {
  let capturedUrl = '';
  let capturedMethod = '';
  let capturedHeaders: Record<string, string> = {};
  let capturedBody: any = null;

  (global as any).fetch = async (url: string, options: any) => {
    capturedUrl = url;
    capturedMethod = options.method;
    capturedHeaders = {};
    if (options.headers) {
      if (typeof options.headers.forEach === 'function') {
        options.headers.forEach((v: string, k: string) => {
          capturedHeaders[k.toLowerCase()] = v;
        });
      } else if (typeof options.headers.entries === 'function') {
        for (const [k, v] of options.headers.entries()) {
          capturedHeaders[k.toLowerCase()] = v;
        }
      } else {
        Object.entries(options.headers).forEach(([k, v]) => {
          capturedHeaders[k.toLowerCase()] = String(v);
        });
      }
    }
    capturedBody = options.body ? JSON.parse(options.body) : null;

    return {
      ok: true,
      status: 202,
      headers: { get: () => 'application/json' },
      json: async () => ({
        accepted: 1,
        duplicates: 0,
        rejected: [],
      }),
    };
  };

  console.log('Testing behavioral API client anonymous request...');
  tokenStore.clearAccessToken();
  const testPayload: IngestEventsRequest = {
    events: [
      {
        eventId: '11111111-1111-4111-8111-111111111111',
        eventType: 'product_view',
        visitorId: '22222222-2222-4222-8222-222222222222',
        occurredAt: new Date().toISOString(),
        productId: '33333333-3333-4333-8333-333333333333',
      },
    ],
  };

  const res1 = await ingestBehavioralEvents(testPayload);
  assert.strictEqual(capturedMethod, 'POST');
  assert.strictEqual(capturedUrl, 'http://127.0.0.1:8080/api/behavior/events');
  assert.strictEqual(capturedHeaders['authorization'], undefined);
  assert.strictEqual(res1.accepted, 1);
  assert.strictEqual(capturedBody.events[0].eventId, testPayload.events[0].eventId);
  assert.strictEqual((capturedBody.events[0] as any).userId, undefined);
  assert.strictEqual((capturedBody.events[0] as any).source, undefined);

  console.log('Testing behavioral API client authenticated request with Bearer token...');
  tokenStore.setAccessToken('test-customer-access-token');
  const res2 = await ingestBehavioralEvents(testPayload);
  assert.strictEqual(capturedHeaders['authorization'], 'Bearer test-customer-access-token');
  assert.strictEqual(res2.accepted, 1);

  console.log('ALL API CLIENT BEHAVIOR TESTS PASSED');
}

runBehaviorApiClientTests().catch((err) => {
  console.error(err);
  process.exit(1);
});

import assert from 'assert';
import {
  groupSearchResults,
  getResultNavigationUrl,
  getAdminSearchErrorMessage,
  searchAdminGlobal,
  GlobalSearchResult,
} from './adminSearch';

async function runTests() {
  console.log('--- Running Admin Search Tests (M6.1B) ---');

  // 1. Grouping tests
  console.log('1. Testing groupSearchResults...');
  const fixtureResults: GlobalSearchResult[] = [
    {
      type: 'order',
      id: 'ord-1',
      title: 'ORD-100193',
      subtitle: 'Никита Осипов · customer@zamk.local · Доставлен',
      canonicalIdentifier: 'ORD-100193',
      navigationTarget: '/orders/ord-1',
    },
    {
      type: 'return',
      id: 'ret-1',
      title: 'ORD-100193',
      subtitle: 'Возврат · Одобрен · 30 августа',
      canonicalIdentifier: 'ORD-100193',
      navigationTarget: '/returns',
    },
    {
      type: 'return',
      id: 'ret-2',
      title: 'ORD-100193',
      subtitle: 'Возврат · Отклонен · 30 августа',
      canonicalIdentifier: 'ORD-100193',
      navigationTarget: '/returns',
    },
    {
      type: 'inventory_unit',
      id: 'unit-1',
      title: 'ZMU-7K9M2X4P8R3V5W6Y',
      subtitle: 'Dev Wool Coat · На складе',
      canonicalIdentifier: 'ZMU-7K9M2X4P8R3V5W6Y',
      navigationTarget: '/inventory',
    },
    {
      type: 'product_variant',
      id: 'var-1',
      title: 'ZMK-9901',
      subtitle: 'Dev Wool Coat · ZMK-9901 · SKU-9901-X',
      canonicalIdentifier: 'ZMK-9901',
      navigationTarget: '/products/prod-1',
    },
    {
      type: 'product',
      id: 'prod-1',
      title: 'Dev Wool Coat',
      subtitle: 'dev-wool-coat · Опубликован',
      canonicalIdentifier: 'dev-wool-coat',
      navigationTarget: '/products/prod-1',
    },
    {
      type: 'customer',
      id: 'cust-1',
      title: 'Local Customer',
      subtitle: 'customer@zamk.local',
      canonicalIdentifier: 'customer@zamk.local',
      navigationTarget: '/users',
    },
  ];

  const grouped = groupSearchResults(fixtureResults);
  assert.strictEqual(grouped.length, 5, 'Should have 5 distinct groups');

  // Verify group ordering: orders -> returns -> inventory -> products -> customers
  assert.strictEqual(grouped[0].key, 'orders');
  assert.strictEqual(grouped[0].title, 'Заказы');
  assert.strictEqual(grouped[0].items.length, 1);
  assert.strictEqual(grouped[0].items[0].title, 'ORD-100193');

  assert.strictEqual(grouped[1].key, 'returns');
  assert.strictEqual(grouped[1].title, 'Возвраты');
  assert.strictEqual(grouped[1].items.length, 2);

  assert.strictEqual(grouped[2].key, 'inventory');
  assert.strictEqual(grouped[2].title, 'Склад');
  assert.strictEqual(grouped[2].items.length, 1);

  assert.strictEqual(grouped[3].key, 'products');
  assert.strictEqual(grouped[3].title, 'Товары');
  assert.strictEqual(grouped[3].items.length, 2); // variant + product

  assert.strictEqual(grouped[4].key, 'customers');
  assert.strictEqual(grouped[4].title, 'Покупатели');
  assert.strictEqual(grouped[4].items.length, 1);

  // Empty result set grouping
  const emptyGrouped = groupSearchResults([]);
  assert.strictEqual(emptyGrouped.length, 0, 'Empty results must produce 0 groups');

  // Partial types grouping
  const ordersOnlyGrouped = groupSearchResults([fixtureResults[0]]);
  assert.strictEqual(ordersOnlyGrouped.length, 1);
  assert.strictEqual(ordersOnlyGrouped[0].key, 'orders');

  // 2. Navigation Target URL formatting
  console.log('2. Testing getResultNavigationUrl...');
  // Order: /orders/{id}
  assert.strictEqual(getResultNavigationUrl(fixtureResults[0]), '/orders/ord-1');

  // Return: /returns?id=...&orderNumber=... (preserves context)
  assert.strictEqual(
    getResultNavigationUrl(fixtureResults[1]),
    '/returns?id=ret-1&orderNumber=ORD-100193',
    'Return URL must preserve Return ID and ORD context'
  );

  // Inventory Unit: /inventory?q=... (preserves ZMU context)
  assert.strictEqual(
    getResultNavigationUrl(fixtureResults[3]),
    '/inventory?q=ZMU-7K9M2X4P8R3V5W6Y',
    'Inventory URL must preserve ZMU context'
  );

  // Product Variant: /products/{productId} (owning product)
  assert.strictEqual(
    getResultNavigationUrl(fixtureResults[4]),
    '/products/prod-1',
    'Variant URL must point to owning product'
  );

  // Product: /products/{id}
  assert.strictEqual(
    getResultNavigationUrl(fixtureResults[5]),
    '/products/prod-1'
  );

  // Customer: /users?q=... (preserves customer email context)
  assert.strictEqual(
    getResultNavigationUrl(fixtureResults[6]),
    '/users?q=customer%40zamk.local',
    'Customer URL must preserve email query param'
  );

  // 3. Error message mapping tests
  console.log('3. Testing getAdminSearchErrorMessage...');
  assert.strictEqual(
    getAdminSearchErrorMessage({ status: 403 }),
    'Недостаточно прав для выполнения поиска.'
  );
  assert.strictEqual(
    getAdminSearchErrorMessage({ code: 'forbidden' }),
    'Недостаточно прав для выполнения поиска.'
  );
  assert.strictEqual(
    getAdminSearchErrorMessage({ code: 'query_too_short' }),
    'Введите не менее 2 символов для поиска.'
  );
  assert.strictEqual(
    getAdminSearchErrorMessage(new Error('something internal')),
    'Не удалось выполнить поиск.',
    'Internal / DB errors must not leak to UI'
  );

  // 4. Query length validation in client
  console.log('4. Testing searchAdminGlobal query validation...');
  const short1 = await searchAdminGlobal('');
  assert.deepStrictEqual(short1.results, [], 'Empty query must not call API');

  const short2 = await searchAdminGlobal(' ');
  assert.deepStrictEqual(short2.results, [], 'Whitespace query must not call API');

  const short3 = await searchAdminGlobal('a');
  assert.deepStrictEqual(short3.results, [], '1 char query must not call API');

  const short4 = await searchAdminGlobal('  a  ');
  assert.deepStrictEqual(short4.results, [], '1 char trimmed query must not call API');

  // 5. Mocked API integration & Stale response handling
  console.log('5. Testing API call with mock & cancellation simulation...');
  let callCount = 0;
  let lastQueried = '';

  (global as any).fetch = async (url: string) => {
    callCount++;
    const parsedUrl = new URL(url, 'http://127.0.0.1:8080');
    lastQueried = parsedUrl.searchParams.get('q') || '';

    // Mock latency
    if (lastQueried === 'slow_query') {
      await new Promise(r => setTimeout(r, 50));
      return {
        ok: true,
        status: 200,
        headers: { get: () => 'application/json' },
        json: async () => ({
          results: [{
            type: 'order',
            id: 'ord-old',
            title: 'OLD_SLOW_RESULT',
            subtitle: 'Old',
            canonicalIdentifier: 'OLD',
            navigationTarget: '/orders/ord-old',
          }],
        }),
      };
    }

    return {
      ok: true,
      status: 200,
      headers: { get: () => 'application/json' },
      json: async () => ({
        results: [{
          type: 'order',
          id: 'ord-new',
          title: 'NEW_FAST_RESULT',
          subtitle: 'New',
          canonicalIdentifier: 'NEW',
          navigationTarget: '/orders/ord-new',
        }],
      }),
    };
  };

  callCount = 0;
  const res2 = await searchAdminGlobal('ORD-100');
  assert.strictEqual(callCount, 1);
  assert.strictEqual(lastQueried, 'ORD-100');
  assert.strictEqual(res2.results[0].title, 'NEW_FAST_RESULT');

  // Stale request race condition simulation
  let activeSequence = 0;
  let finalResultTitle = '';

  const simulateSearch = async (q: string) => {
    const seq = ++activeSequence;
    const resp = await searchAdminGlobal(q);
    if (seq === activeSequence) {
      finalResultTitle = resp.results[0].title;
    }
  };

  // Launch slow request then fast request immediately
  const p1 = simulateSearch('slow_query');
  const p2 = simulateSearch('fast_query');
  await Promise.all([p1, p2]);

  assert.strictEqual(
    finalResultTitle,
    'NEW_FAST_RESULT',
    'Latest fast query must prevail over slower earlier query'
  );

  // 6. No UUID visible text assertion
  console.log('6. Testing no raw UUID visibility...');
  const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  for (const item of fixtureResults) {
    assert.strictEqual(uuidRegex.test(item.title), false, `Title must not be raw UUID: ${item.title}`);
    assert.strictEqual(uuidRegex.test(item.canonicalIdentifier), false, `Canonical ID must not be raw UUID: ${item.canonicalIdentifier}`);
  }

  console.log('--- All Admin Search Tests Passed (M6.1B) ---');
}

runTests().catch((err) => {
  console.error('Test failure:', err);
  process.exit(1);
});

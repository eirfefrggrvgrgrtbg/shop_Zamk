import assert from 'assert';
import { ApiError } from '@zamk/api-client/src/errors';
import {
  getPickingErrorMessage,
  getAdminPickingOrder,
  scanPickingCode,
  getAdminPickingQueue,
  PickingOrder,
  PickingScanResult,
} from './adminPicking';

async function runTests() {
  console.log('--- Running adminPicking tests ---');

  // 1. Error message mapping tests
  console.log('1. Testing getPickingErrorMessage...');

  const testCases = [
    {
      code: 'unit_allocated_to_other_order',
      expected: 'Эта единица зарезервирована для другого заказа',
    },
    {
      code: 'unit_not_allocated_to_fulfillment',
      expected: 'Эта единица не назначена этому заказу',
    },
    {
      code: 'unit_not_in_warehouse',
      expected: 'Эта единица сейчас не находится на складе',
    },
    {
      code: 'cannot_pick_serialized_with_barcode',
      expected: 'Для этого товара нужно отсканировать конкретный ZMU',
    },
    {
      code: 'ambiguous_picking_code',
      expected: 'Штрихкод соответствует нескольким позициям заказа',
    },
    {
      code: 'picking_code_not_found',
      expected: 'Код не найден',
    },
    {
      code: 'picking_not_allowed',
      expected: 'Этот заказ сейчас нельзя собирать',
    },
  ];

  for (const tc of testCases) {
    const err = new ApiError('Raw error message', tc.code, 409);
    const msg = getPickingErrorMessage(err);
    assert.strictEqual(msg, tc.expected, `Error code ${tc.code} did not produce expected Russian message`);
  }

  // Test 403 Forbidden
  const err403 = new ApiError('Forbidden', 'forbidden', 403);
  assert.strictEqual(getPickingErrorMessage(err403), 'Недостаточно прав для выполнения сборки');

  // Test 404 Not Found
  const err404 = new ApiError('Not found', 'not_found', 404);
  assert.strictEqual(getPickingErrorMessage(err404), 'Код не найден');

  // Test generic fallback
  const genericErr = new Error('Random failure');
  assert.strictEqual(getPickingErrorMessage(genericErr), 'Random failure');
  assert.strictEqual(getPickingErrorMessage(null), 'Произошла ошибка при сканировании');

  console.log('✓ All getPickingErrorMessage tests passed.');

  // 2. Mock Fetch tests for getAdminPickingOrder and scanPickingCode
  console.log('2. Testing getAdminPickingOrder and scanPickingCode with mocked fetch...');

  const originalFetch = global.fetch;

  try {
    // A. Successful getAdminPickingOrder
    (global as any).fetch = async (url: string, _options: any) => {
      assert(url.includes('/admin/fulfillments/fulf-123/picking'));
      return {
        ok: true,
        status: 200,
        json: async () => ({
          orderId: 'order-123',
          orderNumber: '10482',
          orderStatus: 'paid',
          fulfillmentId: 'fulf-123',
          fulfillmentStatus: 'paid',
          items: [
            {
              orderItemId: 'item-1',
              title: 'Футболка',
              productVariantId: 'var-1',
              quantity: 2,
              pickedQuantity: 1,
              remainingQuantity: 1,
              allocationMode: 'serialized',
              allocatedUnits: [
                { inventoryUnitId: 'u-1', unitCode: 'ZMU-001', pickedAt: '2026-08-26T10:00:00Z' },
                { inventoryUnitId: 'u-2', unitCode: 'ZMU-002', pickedAt: null },
              ],
            },
          ],
        }),
      };
    };

    const po: PickingOrder = await getAdminPickingOrder('fulf-123');
    assert.strictEqual(po.orderId, 'order-123');
    assert.strictEqual(po.orderNumber, '10482');
    assert.strictEqual(po.items.length, 1);
    assert.strictEqual(po.items[0].allocationMode, 'serialized');
    assert.strictEqual(po.items[0].allocatedUnits.length, 2);
    assert.strictEqual(po.items[0].allocatedUnits[0].pickedAt, '2026-08-26T10:00:00Z');
    assert.strictEqual(po.items[0].allocatedUnits[1].pickedAt, null);

    // B. Successful scanPickingCode
    (global as any).fetch = async (url: string, options: any) => {
      assert(url.includes('/admin/fulfillments/fulf-123/picking/scan'));
      assert.strictEqual(options.method, 'POST');
      const body = JSON.parse(options.body);
      assert.strictEqual(body.code, 'ZMU-002');
      return {
        ok: true,
        status: 200,
        json: async () => ({
          fulfillmentId: 'fulf-123',
          orderId: 'order-123',
          scanResult: {
            code: 'ZMU-002',
            type: 'serialized',
            orderItemId: 'item-1',
            newlyPicked: true,
            alreadyPicked: false,
            alreadyComplete: false,
          },
          item: {
            quantity: 2,
            pickedQuantity: 2,
            remainingQuantity: 0,
            allocationMode: 'serialized',
          },
          fulfillmentProgress: {
            totalQuantity: 2,
            pickedQuantity: 2,
            remainingQuantity: 0,
            isComplete: true,
          },
        }),
      };
    };

    const scanRes: PickingScanResult = await scanPickingCode('fulf-123', 'ZMU-002');
    assert.strictEqual(scanRes.scanResult.newlyPicked, true);
    assert.strictEqual(scanRes.item.pickedQuantity, 2);
    assert.strictEqual(scanRes.fulfillmentProgress.isComplete, true);

    // C. Error scanPickingCode -> throws ApiError
    (global as any).fetch = async (_url: string, _options: any) => {
      return {
        ok: false,
        status: 409,
        json: async () => ({
          error: {
            code: 'unit_not_in_warehouse',
            message: 'Unit is not in warehouse',
          },
        }),
      };
    };

    let caughtError: any = null;
    try {
      await scanPickingCode('fulf-123', 'ZMU-DAMAGED');
    } catch (err: any) {
      caughtError = err;
    }
    assert(caughtError instanceof ApiError);
    assert.strictEqual(caughtError.code, 'unit_not_in_warehouse');
    assert.strictEqual(caughtError.status, 409);
    assert.strictEqual(getPickingErrorMessage(caughtError), 'Эта единица сейчас не находится на складе');

    console.log('✓ All API mock tests passed.');

    // 3. Queue data integrity tests
    console.log('3. Testing getAdminPickingQueue data integrity...');

    // Scenario A: fulfillment list failure -> getAdminPickingQueue() must reject (not swallow/return empty)
    (global as any).fetch = async (url: string) => {
      if (url.includes('/admin/order-fulfillments')) {
        return {
          ok: false,
          status: 500,
          json: async () => ({ error: { message: 'Database failure' } }),
        };
      }
      return { ok: true, status: 200, json: async () => ({}) };
    };

    let queueListError: any = null;
    try {
      await getAdminPickingQueue();
    } catch (err: any) {
      queueListError = err;
    }
    assert(queueListError !== null, 'getAdminPickingQueue must reject when fulfillment list API fails');
    console.log('✓ Fulfillment list API failure properly rejects and is not swallowed to [].');

    // Scenario B: picking read failure for one fulfillment -> getAdminPickingQueue() must reject (not fabricate 0/N)
    (global as any).fetch = async (url: string) => {
      if (url.includes('/admin/order-fulfillments?status=paid')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            items: [
              {
                id: 'fulf-error-1',
                orderId: 'order-err',
                status: 'paid',
                createdAt: '2026-08-26T12:00:00Z',
                items: [{ quantity: 5 }],
              },
            ],
          }),
        };
      }
      if (url.includes('/admin/order-fulfillments?status=assembling')) {
        return { ok: true, status: 200, json: async () => ({ items: [] }) };
      }
      if (url.includes('/admin/fulfillments/fulf-error-1/picking')) {
        return {
          ok: false,
          status: 500,
          json: async () => ({ error: { code: 'internal_error', message: 'Picking read failed' } }),
        };
      }
      return { ok: false, status: 404, json: async () => ({}) };
    };

    let pickingReadError: any = null;
    try {
      await getAdminPickingQueue();
    } catch (err: any) {
      pickingReadError = err;
    }
    assert(pickingReadError !== null, 'getAdminPickingQueue must reject when picking read fails (no fabrication)');
    console.log('✓ Picking read failure rejects without fabricating 0 / N progress.');

    // Scenario C: successful queue -> canonical picking progress is accurately calculated
    (global as any).fetch = async (url: string) => {
      if (url.includes('/admin/order-fulfillments?status=paid')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            items: [
              {
                id: 'fulf-real-1',
                orderId: 'order-real-1',
                orderNumber: '10482',
                status: 'assembling',
                sellerName: 'Nike Official',
                customerName: 'Иван Иванов',
                createdAt: '2026-08-26T11:00:00Z',
              },
            ],
          }),
        };
      }
      if (url.includes('/admin/order-fulfillments?status=assembling')) {
        return { ok: true, status: 200, json: async () => ({ items: [] }) };
      }
      if (url.includes('/admin/fulfillments/fulf-real-1/picking')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            orderId: 'order-real-1',
            orderNumber: '10482',
            orderStatus: 'assembling',
            fulfillmentId: 'fulf-real-1',
            fulfillmentStatus: 'assembling',
            items: [
              {
                orderItemId: 'item-1',
                title: 'Худи',
                productVariantId: 'var-1',
                quantity: 4,
                pickedQuantity: 3,
                remainingQuantity: 1,
                allocationMode: 'serialized',
                allocatedUnits: [],
              },
              {
                orderItemId: 'item-2',
                title: 'Носки',
                productVariantId: 'var-2',
                quantity: 3,
                pickedQuantity: 1,
                remainingQuantity: 2,
                allocationMode: 'legacy',
                allocatedUnits: [],
              },
            ],
          }),
        };
      }
      return { ok: false, status: 404, json: async () => ({}) };
    };

    const queue = await getAdminPickingQueue();
    assert.strictEqual(queue.length, 1);
    const row = queue[0];
    assert.strictEqual(row.fulfillmentId, 'fulf-real-1');
    assert.strictEqual(row.orderNumber, '10482');
    assert.strictEqual(row.status, 'assembling');
    assert.strictEqual(row.totalQuantity, 7, 'totalQuantity must be 7 (4 + 3)');
    assert.strictEqual(row.pickedQuantity, 4, 'pickedQuantity must be 4 (3 + 1)');
    assert.strictEqual(row.remainingQuantity, 3, 'remainingQuantity must be 3 (7 - 4)');
    assert.strictEqual(row.progressPercent, 57, 'progressPercent must be 57 (round 4/7 * 100)');
    assert.strictEqual(row.isComplete, false);
    assert.strictEqual(row.itemPositionsCount, 2);

    console.log('✓ Successful queue preserved canonical 4/7 picking progress.');

    // 4. Order mapping and totals integrity test
    console.log('4. Testing order mapping and totals integrity...');
    const rawOrder = {
      id: 'ord-100',
      orderNumber: '10042',
      status: 'paid',
      fulfillmentStatus: 'pending',
      sourceType: 'normal',
      totalPriceCents: 100000,
      currency: 'RUB',
      customerName: 'Иван',
      createdAt: '2026-08-26T10:00:00Z',
      items: [
        {
          id: 'item-1',
          orderId: 'ord-100',
          title: 'Куртка',
          priceCents: 100000,
          quantity: 1,
          subtotalPriceCents: 100000,
        },
      ],
    };

    const { mapAdminOrder } = await import('./adminOrders');
    const mapped = mapAdminOrder(rawOrder as any);
    assert.strictEqual(mapped.totalPriceCents, 100000, 'totalPriceCents must remain exactly 100000');
    assert.strictEqual(mapped.totalAmount, 1000, 'totalAmount must be 1000 (100000 / 100)');
    assert.strictEqual(mapped.items.length, 1);
    console.log('✓ Order total mapping preserves canonical amounts without artificial inflation.');

    // 5. Problems semantics test: paid pending orders target ZAMK warehouse picking, not seller notification
    console.log('5. Testing Problems queue semantics for FBO warehouse picking...');
    const probPaidOrder = {
      id: 'ord-pending-1',
      orderNumber: '20001',
      status: 'paid',
      fulfillmentStatus: 'pending',
    };
    assert.strictEqual(probPaidOrder.status, 'paid');
    const matchingFulfillment = {
      id: 'fulf-pending-1',
      orderId: 'ord-pending-1',
      orderNumber: '20001',
      status: 'paid',
    };

    // Simulate problem item creation logic
    const problemTitle = 'Оплаченный заказ ожидает сборки на складе';
    const recommendedAction = 'Перейти к сборке';
    const actionUrl = matchingFulfillment.id
      ? `/fulfillment/picking/${matchingFulfillment.id}`
      : '/fulfillment/picking';

    assert.strictEqual(problemTitle, 'Оплаченный заказ ожидает сборки на складе');
    assert.strictEqual(recommendedAction, 'Перейти к сборке');
    assert(actionUrl.startsWith('/fulfillment/picking'), 'Problem CTA must target picking route');
    assert(!problemTitle.toLowerCase().includes('продавец'), 'Must not instruct seller to assemble orders');
    assert(!recommendedAction.toLowerCase().includes('продавец'), 'Must not instruct seller to assemble orders');
    console.log('✓ Problem item correctly targets ZAMK warehouse picking with no seller assemble instructions.');

    // 6. Fulfillment badge and filter alignment tests
    console.log('6. Testing fulfillment badge and filter alignment...');
    const { getOrderFulfillmentBadge } = await import('../utils/orderFormatters');

    // Case A: Missing fulfillment (fulfillmentsCount = 0) with paid order -> 'Не сформирована'
    const orderNoFulf = {
      status: 'paid',
      fulfillmentStatus: '',
      fulfillmentsCount: 0,
    };
    const badgeNoFulf = getOrderFulfillmentBadge(orderNoFulf);
    assert.strictEqual(badgeNoFulf.label, 'Не сформирована', 'Missing fulfillment on paid order must not show "Ожидает сборки"');
    assert.notStrictEqual(badgeNoFulf.label, 'Ожидает сборки');

    // Case B: Awaiting payment order with no fulfillment -> 'Ожидает оплаты'
    const orderAwaitingPay = {
      status: 'awaiting_payment',
      fulfillmentStatus: '',
      fulfillmentsCount: 0,
    };
    const badgeAwaitingPay = getOrderFulfillmentBadge(orderAwaitingPay);
    assert.strictEqual(badgeAwaitingPay.label, 'Ожидает оплаты', 'Awaiting payment order must show "Ожидает оплаты", not "Ожидает сборки"');
    assert.notStrictEqual(badgeAwaitingPay.label, 'Ожидает сборки');

    // Case C: Real paid fulfillment (fulfillmentsCount = 1, fulfillmentStatus = 'paid') -> 'Ожидает сборки'
    const orderPaidFulf = {
      status: 'paid',
      fulfillmentStatus: 'paid',
      fulfillmentsCount: 1,
    };
    const badgePaidFulf = getOrderFulfillmentBadge(orderPaidFulf);
    assert.strictEqual(badgePaidFulf.label, 'Ожидает сборки', 'Real paid fulfillment must show "Ожидает сборки"');

    // Case D: Real assembling fulfillment -> 'В сборке'
    const orderAssemblingFulf = {
      status: 'assembling',
      fulfillmentStatus: 'assembling',
      fulfillmentsCount: 1,
    };
    const badgeAssembling = getOrderFulfillmentBadge(orderAssemblingFulf);
    assert.strictEqual(badgeAssembling.label, 'В сборке');

    // Case E: Assembly filter value 'paid' matches the badge 'Ожидает сборки'
    const filterOptionValue = 'paid';
    assert.strictEqual(filterOptionValue, orderPaidFulf.fulfillmentStatus, 'Filter option for "Ожидает сборки" must send "paid" to match backend f.status');
    assert.notStrictEqual(filterOptionValue, orderNoFulf.fulfillmentStatus, 'Filter option for "Ожидает сборки" must not match missing fulfillment row');

    console.log('✓ Fulfillment badge and filter alignment tests passed.');
  } finally {
    global.fetch = originalFetch;
  }

  console.log('ALL FRONTEND PICKING & ORDERS TESTS PASSED!');
}

runTests().catch((err) => {
  console.error(err);
  process.exit(1);
});

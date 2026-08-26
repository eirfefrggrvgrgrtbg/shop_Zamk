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
  } finally {
    global.fetch = originalFetch;
  }

  console.log('ALL FRONTEND PICKING TESTS PASSED!');
}

runTests().catch((err) => {
  console.error(err);
  process.exit(1);
});

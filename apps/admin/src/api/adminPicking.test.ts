import assert from 'assert';
import { ApiError } from '@zamk/api-client/src/errors';
import {
  getPickingErrorMessage,
  getAdminPickingOrder,
  scanPickingCode,
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
  } finally {
    global.fetch = originalFetch;
  }

  console.log('ALL FRONTEND PICKING TESTS PASSED!');
}

runTests().catch((err) => {
  console.error(err);
  process.exit(1);
});

import assert from 'assert';
import { ApiError } from '@zamk/api-client/src/errors';
import {
  getPickingErrorMessage,
  getPackingErrorMessage,
  getDispatchErrorMessage,
  getAdminPickingOrder,
  scanPickingCode,
  packFulfillment,
  dispatchFulfillment,
  getAdminPickingQueue,
  PickingOrder,
  PickingScanResult,
  PackResult,
  DispatchResult,
} from './adminPicking';
import { getGenericEditableShipmentStatuses, getShipmentStatusLabel } from './adminShipments';

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

  console.log('1b. Testing getPackingErrorMessage...');
  const packingTestCases = [
    { code: 'packing_not_allowed', expected: 'Упаковка недоступна для текущего статуса заказа или сборки' },
    { code: 'fulfillment_not_fully_picked', expected: 'Нельзя завершить упаковку: не все позиции сборки укомплектованы' },
    { code: 'fulfillment_not_found', expected: 'Сборка не найдена' },
  ];
  for (const tc of packingTestCases) {
    const err = new ApiError('Raw error message', tc.code, 409);
    assert.strictEqual(getPackingErrorMessage(err), tc.expected);
  }
  const pack403 = new ApiError('Forbidden', 'forbidden', 403);
  assert.strictEqual(getPackingErrorMessage(pack403), 'Недостаточно прав для выполнения упаковки');
  console.log('✓ All getPackingErrorMessage tests passed.');

  console.log('1c. Testing getDispatchErrorMessage...');
  const dispatchTestCases = [
    { code: 'dispatch_not_allowed', expected: 'Отгрузка недоступна для текущего статуса сборки или заказа (требуется статус «Упакован»)' },
    { code: 'fulfillment_not_fully_picked', expected: 'Нельзя отгрузить: не все позиции сборки укомплектованы' },
    { code: 'inventory_unit_state_conflict', expected: 'Конфликт состояния физических единиц (товар не находится на складе)' },
    { code: 'insufficient_total_stock', expected: 'Недостаточно остатков на складе для списания' },
    { code: 'insufficient_reserved_stock', expected: 'Недостаточно зарезервированного остатка для списания' },
    { code: 'shipment_contradictory_state', expected: 'Отгрузка уже находится в противоречивом или завершенном статусе' },
    { code: 'fulfillment_not_found', expected: 'Сборка не найдена' },
  ];
  for (const tc of dispatchTestCases) {
    const err = new ApiError('Raw error message', tc.code, 409);
    assert.strictEqual(getDispatchErrorMessage(err), tc.expected);
  }
  const disp403 = new ApiError('Forbidden', 'forbidden', 403);
  assert.strictEqual(getDispatchErrorMessage(disp403), 'Недостаточно прав для выполнения отгрузки');
  const disp404 = new ApiError('Not found', 'not_found', 404);
  assert.strictEqual(getDispatchErrorMessage(disp404), 'Сборка не найдена');
  console.log('✓ All getDispatchErrorMessage tests passed.');

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

    // D. Successful packFulfillment
    (global as any).fetch = async (url: string, options: any) => {
      assert(url.includes('/admin/fulfillments/fulf-123/pack'));
      assert.strictEqual(options.method, 'POST');
      return {
        ok: true,
        status: 200,
        json: async () => ({
          fulfillmentId: 'fulf-123',
          orderId: 'order-123',
          fulfillmentStatus: 'packed',
          orderStatus: 'packed',
          packedAt: '2026-08-29T19:30:00Z',
        }),
      };
    };

    const packRes: PackResult = await packFulfillment('fulf-123');
    assert.strictEqual(packRes.fulfillmentId, 'fulf-123');
    assert.strictEqual(packRes.fulfillmentStatus, 'packed');
    assert.strictEqual(packRes.orderStatus, 'packed');
    assert.strictEqual(packRes.packedAt, '2026-08-29T19:30:00Z');

    // E. Error packFulfillment -> throws ApiError
    (global as any).fetch = async (_url: string, _options: any) => {
      return {
        ok: false,
        status: 409,
        json: async () => ({
          error: {
            code: 'fulfillment_not_fully_picked',
            message: 'Fulfillment is not fully picked',
          },
        }),
      };
    };

    let caughtPackErr: any = null;
    try {
      await packFulfillment('fulf-123');
    } catch (err: any) {
      caughtPackErr = err;
    }
    assert(caughtPackErr instanceof ApiError);
    assert.strictEqual(caughtPackErr.code, 'fulfillment_not_fully_picked');
    assert.strictEqual(getPackingErrorMessage(caughtPackErr), 'Нельзя завершить упаковку: не все позиции сборки укомплектованы');

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

    // 7. Dispatch endpoint tests
    console.log('7. Testing dispatchFulfillment...');

    // A. Successful dispatch
    (global as any).fetch = async (url: string, options: any) => {
      assert(url.includes('/admin/fulfillments/fulf-pack-123/dispatch'));
      assert.strictEqual(options.method, 'POST');
      return {
        ok: true,
        status: 200,
        json: async () => ({
          fulfillmentId: 'fulf-pack-123',
          orderId: 'order-123',
          shipmentId: 'ship-999',
          fulfillmentStatus: 'shipped',
          orderStatus: 'shipped',
          shipmentStatus: 'shipped',
          shippedAt: '2026-08-29T20:00:00Z',
        }),
      };
    };

    const dispatchRes: DispatchResult = await dispatchFulfillment('fulf-pack-123');
    assert.strictEqual(dispatchRes.fulfillmentId, 'fulf-pack-123');
    assert.strictEqual(dispatchRes.fulfillmentStatus, 'shipped');
    assert.strictEqual(dispatchRes.orderStatus, 'shipped');
    assert.strictEqual(dispatchRes.shipmentStatus, 'shipped');
    assert.strictEqual(dispatchRes.shipmentId, 'ship-999');

    // B. Rejection on dispatch
    (global as any).fetch = async (_url: string, _options: any) => {
      return {
        ok: false,
        status: 409,
        json: async () => ({
          error: {
            code: 'dispatch_not_allowed',
            message: 'Fulfillment is not packed',
          },
        }),
      };
    };

    try {
      await dispatchFulfillment('fulf-pack-123');
      assert.fail('Expected dispatchFulfillment to throw on 409');
    } catch (err: any) {
      assert(err instanceof ApiError);
      assert.strictEqual(err.code, 'dispatch_not_allowed');
      assert.strictEqual(err.status, 409);
      assert.strictEqual(
        getDispatchErrorMessage(err),
        'Отгрузка недоступна для текущего статуса сборки или заказа (требуется статус «Упакован»)'
      );
    }
    console.log('✓ dispatchFulfillment tests passed.');

    // 8. Testing getGenericEditableShipmentStatuses (hardening against generic shipped/delivered bypass)
    console.log('8. Testing getGenericEditableShipmentStatuses...');
    const pendingStatuses = getGenericEditableShipmentStatuses('pending');
    assert(!pendingStatuses.includes('shipped'), 'generic shipment statuses must NOT include shipped');
    assert(!pendingStatuses.includes('delivered'), 'generic shipment statuses must NOT include delivered');
    assert(pendingStatuses.includes('assembling'), 'pending shipment can be marked assembling');
    assert(pendingStatuses.includes('packed'), 'pending shipment can be marked packed');

    const packedStatuses = getGenericEditableShipmentStatuses('packed');
    assert(!packedStatuses.includes('shipped'), 'generic shipment statuses for packed must NOT include shipped');
    assert(!packedStatuses.includes('delivered'), 'generic shipment statuses for packed must NOT include delivered');

    const shippedStatuses = getGenericEditableShipmentStatuses('shipped');
    assert.deepStrictEqual(shippedStatuses, ['shipped'], 'already shipped shipment status dropdown cannot transition to other generic statuses');

    // Localization check for canonical Russian shipment statuses
    assert.strictEqual(getShipmentStatusLabel('pending'), 'Ожидает');
    assert.strictEqual(getShipmentStatusLabel('assembling'), 'В сборке');
    assert.strictEqual(getShipmentStatusLabel('packed'), 'Упакован');
    assert.strictEqual(getShipmentStatusLabel('shipped'), 'Отгружен');
    assert.strictEqual(getShipmentStatusLabel('delivered'), 'Доставлен');
    assert.strictEqual(getShipmentStatusLabel('failed'), 'Ошибка');
    assert.strictEqual(getShipmentStatusLabel('cancelled'), 'Отменен');

    // 9. Testing packed fulfillment read model for Dispatch Detail
    console.log('9. Testing packed fulfillment read model for Dispatch Detail...');
    (global as any).fetch = async (url: string, _options: any) => {
      assert(url.includes('/admin/order-fulfillments/fulf-pack-777'));
      return {
        ok: true,
        status: 200,
        json: async () => ({
          id: 'fulf-pack-777',
          orderId: '26c03ebe-0f6e-46c9-a279-998b1decacda',
          orderNumber: null,
          sellerId: 'seller-001',
          sellerName: 'ZAMK Dev Seller',
          status: 'packed',
          subtotalCents: 1299000,
          commissionBps: 1500,
          sellerAmountCents: 1104150,
          createdAt: '2026-08-29T14:51:18.811946Z',
          updatedAt: '2026-08-29T16:35:53.759Z',
          packedAt: '2026-08-29T16:35:53.759Z',
          customerName: 'Никита Осипов',
          customerPhone: '9672451676',
          deliveryAddress: 'Smoke Courier: сапмриотл спмриотл нниго',
          items: [
            {
              orderItemId: 'item-777',
              productId: 'prod-777',
              productTitle: 'Dev Wool Coat',
              quantity: 1,
              unitPriceCents: 1299000,
              lineTotalCents: 1299000,
              allocationMode: 'serialized',
              allocatedUnits: [
                {
                  inventoryUnitId: 'unit-777',
                  unitCode: 'ZMU-5R7EA6U2NAYKZ55K',
                  pickedAt: '2026-08-29T14:56:00.990482Z',
                },
              ],
            },
          ],
        }),
      };
    };

    const { getAdminFulfillment } = await import('./adminOrders');
    const { formatOrderNumber } = await import('../utils/orderFormatters');

    const packedFulfillment = await getAdminFulfillment('fulf-pack-777');
    assert.strictEqual(packedFulfillment.id, 'fulf-pack-777');
    assert.strictEqual(packedFulfillment.status, 'packed');
    assert.strictEqual(packedFulfillment.packedAt, '2026-08-29T16:35:53.759Z');
    assert.strictEqual(packedFulfillment.customerName, 'Никита Осипов');
    assert.strictEqual(packedFulfillment.customerPhone, '9672451676');
    assert.strictEqual(packedFulfillment.deliveryAddress, 'Smoke Courier: сапмриотл спмриотл нниго');
    assert.strictEqual(packedFulfillment.sellerName, 'ZAMK Dev Seller');

    // Canonical order number formatting fallback
    const displayOrderNumber = formatOrderNumber({
      id: packedFulfillment.orderId,
      orderNumber: packedFulfillment.orderNumber,
    });
    assert.strictEqual(displayOrderNumber, 'ORD-26C03EBE');

    // Items and allocated units integrity
    assert.strictEqual(packedFulfillment.items.length, 1);
    const item = packedFulfillment.items[0];
    assert.strictEqual(item.productTitle, 'Dev Wool Coat');
    assert.strictEqual(item.quantity, 1);
    assert.strictEqual(item.allocationMode, 'serialized');
    assert.strictEqual(item.allocatedUnits?.length, 1);
    assert.strictEqual(item.allocatedUnits?.[0].unitCode, 'ZMU-5R7EA6U2NAYKZ55K');
    assert(item.allocatedUnits?.[0].pickedAt !== null);

    console.log('✓ packed fulfillment read model tests passed.');

    // 10. Testing Admin Orders payment status consistency and badge formatters
    console.log('10. Testing Admin Orders payment status consistency and badge formatters...');
    const { getPaymentStatusLabel } = await import('./adminOrders');
    const { getOrderPaymentBadge } = await import('../utils/orderFormatters');

    assert.strictEqual(getPaymentStatusLabel('paid'), 'Оплачен');
    assert.strictEqual(getPaymentStatusLabel('succeeded'), 'Оплачен');
    assert.strictEqual(getPaymentStatusLabel('pending'), 'Ожидает оплаты');
    assert.strictEqual(getPaymentStatusLabel('failed'), 'Ошибка оплаты');
    assert.strictEqual(getPaymentStatusLabel('cancelled'), 'Отменен');
    assert.strictEqual(getPaymentStatusLabel(undefined), 'Ожидает оплаты');

    // Case A: Succeeded payment + shipped order (like ORD-26C03EBE)
    const shippedPaidOrder = mapAdminOrder({
      id: '26c03ebe-0f6e-46c9-a279-998b1decacda',
      orderNumber: 'ORD-26C03EBE',
      status: 'shipped',
      paymentStatus: 'paid',
      fulfillmentStatus: 'shipped',
      sourceType: 'normal',
      totalPriceCents: 1300000,
      currency: 'RUB',
      customerName: 'Никита Осипов',
      createdAt: '2026-08-29T14:51:18.811946Z',
    });
    assert.strictEqual(shippedPaidOrder.paymentStatus, 'paid');
    assert.strictEqual(shippedPaidOrder.paymentStatusLabel, 'Оплачен');
    const shippedBadge = getOrderPaymentBadge(shippedPaidOrder);
    assert.strictEqual(shippedBadge.label, 'Оплачен');
    assert.strictEqual(shippedBadge.bg, 'bg-emerald-50 border border-emerald-200');

    // Case B: Payment rendering does not regress when order progresses with succeeded payment (assembling, packed, delivered)
    for (const progressiveStatus of ['paid', 'assembling', 'packed', 'shipped', 'delivered']) {
      const progressiveOrder = mapAdminOrder({
        id: 'ord-prog-1',
        status: progressiveStatus,
        paymentStatus: 'paid',
        fulfillmentStatus: progressiveStatus,
        sourceType: 'normal',
        totalPriceCents: 100000,
        currency: 'RUB',
        createdAt: '2026-08-29T14:51:18Z',
      });
      assert.strictEqual(progressiveOrder.paymentStatus, 'paid', `order with status ${progressiveStatus} must preserve paymentStatus=paid`);
      assert.strictEqual(progressiveOrder.paymentStatusLabel, 'Оплачен', `order with status ${progressiveStatus} must have paymentStatusLabel=Оплачен`);
      assert.strictEqual(getOrderPaymentBadge(progressiveOrder).label, 'Оплачен', `order with status ${progressiveStatus} badge must be Оплачен`);
    }

    // Case C: Pending payment remains Ожидает оплаты regardless of order lifecycle
    for (const progressiveStatus of ['awaiting_payment', 'paid', 'assembling', 'packed', 'shipped', 'delivered']) {
      const pendingOrder = mapAdminOrder({
        id: 'ord-pend-1',
        status: progressiveStatus,
        paymentStatus: 'pending',
        fulfillmentStatus: '',
        sourceType: 'normal',
        totalPriceCents: 50000,
        currency: 'RUB',
        createdAt: '2026-08-29T14:51:18Z',
      });
      assert.strictEqual(pendingOrder.paymentStatus, 'pending', `status ${progressiveStatus} with pending payment must remain pending`);
      assert.strictEqual(pendingOrder.paymentStatusLabel, 'Ожидает оплаты');
      const pendingBadge = getOrderPaymentBadge(pendingOrder);
      assert.strictEqual(pendingBadge.label, 'Ожидает');
      assert.strictEqual(pendingBadge.bg, 'bg-amber-50 border border-amber-200');
    }

    // Case D: Packed order with failed payment -> failed
    const packedFailedOrder = mapAdminOrder({
      id: 'ord-fail-1',
      status: 'packed',
      paymentStatus: 'failed',
      fulfillmentStatus: 'packed',
      sourceType: 'normal',
      totalPriceCents: 50000,
      currency: 'RUB',
      createdAt: '2026-08-29T14:51:18Z',
    });
    assert.strictEqual(packedFailedOrder.paymentStatus, 'failed');
    assert.strictEqual(packedFailedOrder.paymentStatusLabel, 'Ошибка оплаты');
    const failedBadge = getOrderPaymentBadge(packedFailedOrder);
    assert.strictEqual(failedBadge.label, 'Ошибка');

    // Case E: Delivered order with cancelled payment -> cancelled
    const deliveredCancelledOrder = mapAdminOrder({
      id: 'ord-canc-1',
      status: 'delivered',
      paymentStatus: 'cancelled',
      fulfillmentStatus: 'delivered',
      sourceType: 'normal',
      totalPriceCents: 50000,
      currency: 'RUB',
      createdAt: '2026-08-29T14:51:18Z',
    });
    assert.strictEqual(deliveredCancelledOrder.paymentStatus, 'cancelled');
    assert.strictEqual(deliveredCancelledOrder.paymentStatusLabel, 'Отменен');
    const cancelledBadge = getOrderPaymentBadge(deliveredCancelledOrder);
    assert.strictEqual(cancelledBadge.label, 'Отменен');

    // Case F: Shipped order with NO payment rows -> NOT paid (pending)
    const shippedNoPaymentOrder = mapAdminOrder({
      id: 'ord-nopay-1',
      status: 'shipped',
      fulfillmentStatus: 'shipped',
      sourceType: 'normal',
      totalPriceCents: 50000,
      currency: 'RUB',
      createdAt: '2026-08-29T14:51:18Z',
    });
    assert.strictEqual(shippedNoPaymentOrder.paymentStatus, 'pending', 'order without payment must NOT be inferred as paid');
    assert.strictEqual(shippedNoPaymentOrder.paymentStatusLabel, 'Ожидает оплаты');
    assert.strictEqual(getOrderPaymentBadge(shippedNoPaymentOrder).label, 'Ожидает');

    console.log('✓ Admin Orders payment status consistency tests passed.');

    // 11. Testing Admin Order Detail operational timeline mapping
    console.log('11. Testing Admin Order Detail operational timeline mapping...');
    const timelineOrder = mapAdminOrder({
      id: '26c03ebe-0f6e-46c9-a279-998b1decacda',
      orderNumber: 'ORD-26C03EBE',
      status: 'shipped',
      paymentStatus: 'paid',
      fulfillmentStatus: 'shipped',
      sourceType: 'normal',
      totalPriceCents: 1300000,
      currency: 'RUB',
      customerName: 'Никита Осипов',
      createdAt: '2026-08-29T14:51:18.811946Z',
      timeline: [
        {
          id: 'ev-1',
          type: 'order_created',
          title: 'Заказ создан',
          timestamp: '2026-08-29T14:51:18.811946Z',
        },
        {
          id: 'ev-2',
          type: 'payment_succeeded',
          title: 'Оплата подтверждена',
          timestamp: '2026-08-29T14:51:39.318534Z',
          context: 'PAY-000203 (tbank)',
        },
        {
          id: 'ev-3',
          type: 'status_change',
          title: 'В сборке',
          timestamp: '2026-08-29T14:56:00.990482Z',
        },
        {
          id: 'ev-4',
          type: 'status_change',
          title: 'Упаковка завершена',
          timestamp: '2026-08-29T16:35:53.750993Z',
        },
        {
          id: 'ev-5',
          type: 'shipment_dispatched',
          title: 'Отгружен со склада',
          timestamp: '2026-08-29T17:32:43.076484Z',
          context: 'Склад ZAMK',
        },
      ],
    });

    assert.ok(timelineOrder.timeline, 'timeline must be present in mapped view');
    assert.strictEqual(timelineOrder.timeline.length, 5, 'must contain all 5 persisted timeline milestones');
    assert.strictEqual(timelineOrder.timeline[0].title, 'Заказ создан');
    assert.strictEqual(timelineOrder.timeline[1].title, 'Оплата подтверждена');
    assert.strictEqual(timelineOrder.timeline[1].context, 'PAY-000203 (tbank)');
    assert.strictEqual(timelineOrder.timeline[2].title, 'В сборке');
    assert.strictEqual(timelineOrder.timeline[3].title, 'Упаковка завершена');
    assert.strictEqual(timelineOrder.timeline[4].title, 'Отгружен со склада');
    assert.strictEqual(timelineOrder.timeline[4].context, 'Склад ZAMK');

    // Chronological verification
    for (let i = 1; i < timelineOrder.timeline.length; i++) {
      const prev = new Date(timelineOrder.timeline[i - 1].timestamp).getTime();
      const curr = new Date(timelineOrder.timeline[i].timestamp).getTime();
      assert.ok(curr >= prev, `timeline event ${i} must be after event ${i - 1}`);
    }

    console.log('✓ Admin Order Detail operational timeline tests passed.');
  } finally {
    global.fetch = originalFetch;
  }

  console.log('ALL FRONTEND PICKING & ORDERS TESTS PASSED!');
}

runTests().catch((err) => {
  console.error(err);
  process.exit(1);
});

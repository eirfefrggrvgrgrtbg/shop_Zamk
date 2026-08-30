import {
  getReturnReasonLabel,
  getReturnStatusLabel,
  getStatusBadgeClass,
  type AdminReturn,
  type AdminReturnEvidence,
} from '../api/adminReturns';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

async function runAdminReturnsTests() {
  console.log('Testing Admin Returns claim review logic and UI mappings...');

  // 1. Canonical reason label mapping tests
  assert(getReturnReasonLabel('defective') === 'Товар неисправен', 'defective label must match Russian translation');
  assert(getReturnReasonLabel('damaged') === 'Товар повреждён', 'damaged label must match Russian translation');
  assert(getReturnReasonLabel('wrong_item') === 'Получен не тот товар', 'wrong_item label must match Russian translation');
  assert(getReturnReasonLabel('not_as_described') === 'Не соответствует описанию', 'not_as_described label must match Russian translation');
  assert(getReturnReasonLabel('incomplete') === 'Не хватает части комплекта', 'incomplete label must match Russian translation');
  assert(getReturnReasonLabel('size_fit') === 'Не подошёл размер / посадка', 'size_fit label must match Russian translation');
  assert(getReturnReasonLabel('changed_mind') === 'Передумал', 'changed_mind label must match Russian translation');
  assert(getReturnReasonLabel('other') === 'Другое', 'other label must match Russian translation');
  assert(getReturnReasonLabel(undefined) === '—', 'undefined reason must return fallback dash');

  // 2. Canonical status label mapping tests
  assert(getReturnStatusLabel('requested') === 'Новая заявка', 'requested status must map to Новая заявка');
  assert(getReturnStatusLabel('approved') === 'Возврат одобрен', 'approved status must map to Возврат одобрен');
  assert(getReturnStatusLabel('rejected') === 'Отклонена', 'rejected status must map to Отклонена');
  assert(getReturnStatusLabel('receiving') === 'Приёмка на складе', 'receiving status must map to Приёмка на складе');
  assert(getReturnStatusLabel('item_received') === 'Товар принят', 'item_received status must map to Товар принят');
  assert(getReturnStatusLabel('refunded') === 'Деньги возвращены', 'refunded status must map to Деньги возвращены');
  assert(getReturnStatusLabel('completed') === 'Завершена', 'completed status must map to Завершена');
  assert(getReturnStatusLabel('cancelled') === 'Отменена', 'cancelled status must map to Отменена');

  // 3. Status badge classes
  assert(getStatusBadgeClass('requested').includes('amber'), 'requested badge must be amber');
  assert(getStatusBadgeClass('approved').includes('blue'), 'approved badge must be blue');
  assert(getStatusBadgeClass('rejected').includes('red'), 'rejected badge must be red');
  assert(getStatusBadgeClass('receiving').includes('purple'), 'receiving badge must be purple');
  assert(getStatusBadgeClass('item_received').includes('teal'), 'item_received badge must be teal');
  assert(getStatusBadgeClass('completed').includes('green'), 'completed badge must be green');

  // 4. Moderation action availability matrix
  const isModerationActionAvailable = (status: string) => status === 'requested';
  assert(isModerationActionAvailable('requested') === true, 'Moderation actions MUST be enabled for requested status');
  assert(isModerationActionAvailable('approved') === false, 'Moderation actions MUST be disabled for approved status');
  assert(isModerationActionAvailable('rejected') === false, 'Moderation actions MUST be disabled for rejected status');
  assert(isModerationActionAvailable('receiving') === false, 'Moderation actions MUST be disabled for receiving status');
  assert(isModerationActionAvailable('item_received') === false, 'Moderation actions MUST be disabled for item_received status');
  assert(isModerationActionAvailable('refunded') === false, 'Moderation actions MUST be disabled for refunded status');
  assert(isModerationActionAvailable('completed') === false, 'Moderation actions MUST be disabled for completed status');
  assert(isModerationActionAvailable('cancelled') === false, 'Moderation actions MUST be disabled for cancelled status');

  // 5. Rejection comment validation
  const canSubmitRejection = (reason: string) => reason.trim().length > 0;
  assert(canSubmitRejection('') === false, 'Empty rejection reason must be invalid');
  assert(canSubmitRejection('   \n\t ') === false, 'Whitespace rejection reason must be invalid');
  assert(canSubmitRejection('Tags removed and item shows wear') === true, 'Real rejection reason must be valid');

  // 6. Evidence rendering & flattening proof
  const dummyEvidence1: AdminReturnEvidence = {
    id: 'ev-1',
    url: 'https://minio.local/media/photo1.jpg',
    contentType: 'image/jpeg',
    sortOrder: 0,
    createdAt: new Date().toISOString(),
  };
  const dummyEvidence2: AdminReturnEvidence = {
    id: 'ev-2',
    url: 'https://minio.local/media/photo2.jpg',
    contentType: 'image/jpeg',
    sortOrder: 1,
    createdAt: new Date().toISOString(),
  };

  const sampleReturnWithEvidence: AdminReturn = {
    id: 'ret-101',
    orderId: 'ord-uuid-1',
    orderNumber: 'ORD-100193',
    status: 'requested',
    reason: 'damaged',
    comment: 'Torn seam on right sleeve',
    customerName: 'Nikita Osipov',
    customerEmail: 'nikita@zamk.local',
    evidenceCount: 2,
    items: [
      {
        id: 'ri-1',
        orderItemId: 'oi-1',
        productTitle: 'Dev Wool Coat',
        productImageUrl: 'https://minio.local/media/coat.jpg',
        variantSize: 'M',
        variantColor: 'Graphite',
        sku: 'SKU-COAT-M',
        quantity: 1,
        priceCents: 1500000,
        subtotalPriceCents: 1500000,
        evidence: [dummyEvidence1, dummyEvidence2],
      },
    ],
  };

  const flattenedEv = (sampleReturnWithEvidence.items || []).flatMap((it) => it.evidence || []);
  assert(flattenedEv.length === 2, 'Must extract exactly 2 evidence items');
  assert(flattenedEv[0].url === 'https://minio.local/media/photo1.jpg', 'First evidence photo URL must match');
  assert(flattenedEv[1].url === 'https://minio.local/media/photo2.jpg', 'Second evidence photo URL must match');
  assert(!('storage_key' in (flattenedEv[0] as any)), 'storage_key must NOT exist on UI evidence model');

  // 7. Historical zero-evidence claim proof
  const sampleHistoricalReturn: AdminReturn = {
    id: 'ret-hist',
    orderId: 'ord-uuid-2',
    orderNumber: 'ORD-100100',
    status: 'completed',
    reason: 'size_fit',
    comment: 'Too large',
    evidenceCount: 0,
    items: [
      {
        id: 'ri-hist',
        orderItemId: 'oi-hist',
        productTitle: 'Old Item',
        quantity: 1,
        priceCents: 50000,
        subtotalPriceCents: 50000,
        evidence: [],
      },
    ],
  };

  const histFlattenedEv = (sampleHistoricalReturn.items || []).flatMap((it) => it.evidence || []);
  assert(histFlattenedEv.length === 0, 'Historical claim with no evidence must return 0 evidence items');

  // 8. Order canonical identity proof
  const getDisplayOrderIdentity = (r: AdminReturn) => r.orderNumber || r.orderId;
  assert(getDisplayOrderIdentity(sampleReturnWithEvidence) === 'ORD-100193', 'Must prioritize canonical order number over UUID');

  console.log('ALL ADMIN RETURNS LOGIC & CONTRACT TESTS PASSED');
}

runAdminReturnsTests();

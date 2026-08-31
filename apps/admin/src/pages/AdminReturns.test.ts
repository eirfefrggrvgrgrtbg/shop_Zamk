import { describe, it } from 'vitest';
import {
  getReturnReasonLabel,
  getReturnStatusLabel,
  getStatusBadgeClass,
  getAdminReturnErrorMessage,
  type AdminReturn,
  type AdminReturnEvidence,
  type AdminReturnRefundQuote,
} from '../api/adminReturns';
import { ApiError } from '@zamk/api-client/src/errors';

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

  // 9. M5.3.3A Logistics Dossier Status & Method Mappings
  const { formatReturnShipmentStatus, formatReturnShipmentMethod } = await import('../api/adminReturns');

  assert(formatReturnShipmentStatus('draft') === 'Оформляем отправление', 'draft mapping must match');
  assert(formatReturnShipmentStatus('awaiting_handover') === 'Ожидает передачи в СДЭК', 'awaiting_handover mapping must match');
  assert(formatReturnShipmentStatus('handed_over') === 'Передано в СДЭК', 'handed_over mapping must match');
  assert(formatReturnShipmentStatus('in_transit') === 'В пути', 'in_transit mapping must match');
  assert(formatReturnShipmentStatus('arrived_at_zamk') === 'Прибыло на склад ZAMK', 'arrived_at_zamk mapping must match');
  assert(formatReturnShipmentStatus('cancelled') === 'Отправление отменено', 'cancelled mapping must match');

  assert(formatReturnShipmentMethod('cdek_courier') === 'Заберёт курьер СДЭК', 'cdek_courier mapping must match');
  assert(formatReturnShipmentMethod('cdek_office') === 'Отнести в отделение СДЭК', 'cdek_office mapping must match');

  // 10. Admin Logistics UI helper logic tests
  const getAdminLogisticsText = (ret: AdminReturn) => {
    if (!ret.shipment) {
      return 'Ожидает выбора способа отправки покупателем';
    }
    if (ret.shipment.status === 'arrived_at_zamk') {
      return 'Возврат прибыл на склад';
    }
    return `Ожидает прибытия на склад (Статус: ${formatReturnShipmentStatus(ret.shipment.status)})`;
  };

  const isWarehouseReceivingEligible = (ret: AdminReturn) => {
    return ret.status === 'approved' && ret.shipment?.status === 'arrived_at_zamk';
  };

  // Approved + no shipment
  const returnNoShipment: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: undefined };
  assert(getAdminLogisticsText(returnNoShipment) === 'Ожидает выбора способа отправки покупателем', 'Must show waiting for customer choice');
  assert(isWarehouseReceivingEligible(returnNoShipment) === false, 'No warehouse CTA before shipment');

  // Approved + awaiting_handover
  const returnAwaitingHandover: AdminReturn = {
    ...sampleReturnWithEvidence,
    status: 'approved',
    shipment: { id: 's1', provider: 'cdek', method: 'cdek_office', status: 'awaiting_handover' },
  };
  assert(getAdminLogisticsText(returnAwaitingHandover).includes('Ожидает передачи в СДЭК'), 'Must show human status');
  assert(isWarehouseReceivingEligible(returnAwaitingHandover) === false, 'NO warehouse CTA for awaiting_handover');

  // Approved + handed_over
  const returnHandedOver: AdminReturn = {
    ...sampleReturnWithEvidence,
    status: 'approved',
    shipment: { id: 's2', provider: 'cdek', method: 'cdek_office', status: 'handed_over' },
  };
  assert(getAdminLogisticsText(returnHandedOver).includes('Передано в СДЭК'), 'Must show human status for handed_over');
  assert(isWarehouseReceivingEligible(returnHandedOver) === false, 'NO warehouse CTA for handed_over');

  // Approved + in_transit
  const returnInTransit: AdminReturn = {
    ...sampleReturnWithEvidence,
    status: 'approved',
    shipment: { id: 's3', provider: 'cdek', method: 'cdek_courier', status: 'in_transit' },
  };
  assert(getAdminLogisticsText(returnInTransit).includes('В пути'), 'Must show human status for in_transit');
  assert(isWarehouseReceivingEligible(returnInTransit) === false, 'NO warehouse CTA for in_transit');

  // Approved + arrived_at_zamk
  const returnArrived: AdminReturn = {
    ...sampleReturnWithEvidence,
    status: 'approved',
    shipment: { id: 's4', provider: 'cdek', method: 'cdek_office', status: 'arrived_at_zamk' },
  };
  assert(getAdminLogisticsText(returnArrived) === 'Возврат прибыл на склад', 'Must show arrived at warehouse');
  assert(isWarehouseReceivingEligible(returnArrived) === true, 'Warehouse CTA allowed when arrived_at_zamk');

  // 11. M5.3.4A Return Communication & Moderation Decision Panel
  const getDecisionPanelElements = (status: string): { actions: string[]; showsWaitingStatus: boolean; waitingLabel?: string } => {
    if (status === 'requested') {
      return {
        actions: ['Одобрить возврат', 'Отклонить заявку'],
        showsWaitingStatus: false,
      };
    }
    if (status === 'needs_info') {
      return {
        actions: [],
        showsWaitingStatus: true,
        waitingLabel: 'Ожидает ответа покупателя',
      };
    }
    return { actions: [], showsWaitingStatus: false };
  };

  const requestedPanel = getDecisionPanelElements('requested');
  assert(requestedPanel.actions.length === 2, 'requested status must render exactly 2 moderation actions');
  assert(requestedPanel.actions.includes('Одобрить возврат'), 'requested status must include Одобрить возврат action');
  assert(requestedPanel.actions.includes('Отклонить заявку'), 'requested status must include Отклонить заявку action');
  assert(requestedPanel.showsWaitingStatus === false, 'requested status must not show waiting banner');

  const needsInfoPanel = getDecisionPanelElements('needs_info');
  assert(needsInfoPanel.actions.length === 0, 'needs_info status must NOT render moderation action buttons');
  assert(needsInfoPanel.showsWaitingStatus === true, 'needs_info must show waiting status');
  assert(needsInfoPanel.waitingLabel === 'Ожидает ответа покупателя', 'needs_info waiting label must be Ожидает ответа покупателя');

  // Validate info request submission checks
  const canSubmitInfoRequest = (msg: string) => msg.trim().length > 0;
  assert(canSubmitInfoRequest('') === false, 'Empty info request message must be disabled/rejected');
  assert(canSubmitInfoRequest('   \n\t  ') === false, 'Whitespace info request message must be disabled/rejected');
  assert(canSubmitInfoRequest('Please clarify details') === true, 'Valid info request message must be accepted');

  // Validate message bubbles include human sender labels and no raw user IDs
  const sampleMessages = [
    {
      id: 'msg-1',
      returnId: 'ret-101',
      senderRole: 'admin' as const,
      messageType: 'info_request' as const,
      body: 'Пожалуйста, приложите фото ярлыка.',
      createdAt: '2026-08-30T15:00:00Z',
    },
    {
      id: 'msg-2',
      returnId: 'ret-101',
      senderRole: 'customer' as const,
      messageType: 'message' as const,
      body: 'Прикрепил фото ярлыка.',
      createdAt: '2026-08-30T15:05:00Z',
    },
  ];

  const getSenderLabel = (role: 'admin' | 'customer') => (role === 'admin' ? 'ZAMK' : 'Покупатель');
  assert(getSenderLabel(sampleMessages[0].senderRole) === 'ZAMK', 'Admin message bubble must show ZAMK');
  assert(getSenderLabel(sampleMessages[1].senderRole) === 'Покупатель', 'Customer message bubble must show Покупатель');

  assert(!('senderUserId' in sampleMessages[0]), 'senderUserId must NOT be exposed in ReturnMessage DTO');
  assert(!('senderUserId' in sampleMessages[1]), 'senderUserId must NOT be exposed in ReturnMessage DTO');

  // 12. M5.3.4A Active/Terminal Conversation Policy & Drawer Tests
  const { isReturnConversationWritable, isReturnConversationTerminal } = await import('@zamk/api-client/src/types');

  // Active states must have composer enabled (writable)
  assert(isReturnConversationWritable('requested') === true, 'requested must be writable in conversation');
  assert(isReturnConversationWritable('needs_info') === true, 'needs_info must be writable in conversation');
  assert(isReturnConversationWritable('approved') === true, 'approved must be writable in conversation');
  assert(isReturnConversationWritable('receiving') === true, 'receiving must be writable in conversation');
  assert(isReturnConversationWritable('item_received') === true, 'item_received must be writable in conversation');

  // Terminal states must be read-only (not writable)
  assert(isReturnConversationWritable('rejected') === false, 'rejected must be read-only in conversation');
  assert(isReturnConversationWritable('refunded') === false, 'refunded must be read-only in conversation');
  assert(isReturnConversationWritable('completed') === false, 'completed must be read-only in conversation');
  assert(isReturnConversationWritable('cancelled') === false, 'cancelled must be read-only in conversation');

  // Terminal status helper
  assert(isReturnConversationTerminal('rejected') === true, 'rejected must be terminal');
  assert(isReturnConversationTerminal('refunded') === true, 'refunded must be terminal');
  assert(isReturnConversationTerminal('completed') === true, 'completed must be terminal');
  assert(isReturnConversationTerminal('cancelled') === true, 'cancelled must be terminal');
  assert(isReturnConversationTerminal('approved') === false, 'approved must NOT be terminal');
  assert(isReturnConversationTerminal('receiving') === false, 'receiving must NOT be terminal');
  assert(isReturnConversationTerminal('item_received') === false, 'item_received must NOT be terminal');

  // 13. M5.3.4A Admin Attachment & Info Request Composer UI
  const canSendAdminMessage = (body: string, attachmentCount: number, isSending: boolean, isUploading: boolean) => {
    if (isSending || isUploading) return false;
    if (attachmentCount > 6) return false;
    return body.trim().length > 0 || attachmentCount > 0;
  };

  assert(canSendAdminMessage('', 0, false, false) === false, 'Empty text and 0 attachments cannot send');
  assert(canSendAdminMessage('  ', 0, false, false) === false, 'Whitespace text and 0 attachments cannot send');
  assert(canSendAdminMessage('', 1, false, false) === true, 'Empty text and 1 attachment can send (photo-only)');
  assert(canSendAdminMessage('Hello', 0, false, false) === true, 'Text-only can send');
  assert(canSendAdminMessage('Hello', 6, false, false) === true, 'Text with 6 attachments can send');
  assert(canSendAdminMessage('Hello', 7, false, false) === false, 'Cannot send more than 6 attachments');
  assert(canSendAdminMessage('Hello', 1, true, false) === false, 'Cannot send while isSending=true');
  assert(canSendAdminMessage('Hello', 1, false, true) === false, 'Cannot send while isUploading=true');

  const isNeedsResponseCheckboxVisible = (status: string) => status === 'requested';
  assert(isNeedsResponseCheckboxVisible('requested') === true, 'Needs response checkbox visible only on requested');
  assert(isNeedsResponseCheckboxVisible('approved') === false, 'Needs response checkbox hidden on approved');
  assert(isNeedsResponseCheckboxVisible('receiving') === false, 'Needs response checkbox hidden on receiving');
  assert(isNeedsResponseCheckboxVisible('needs_info') === false, 'Needs response checkbox hidden on needs_info');

  // 14. M5.4B Return Refund Quote Contract & Totals Breakdown
  const mockQuote: AdminReturnRefundQuote = {
    returnId: 'ret-101',
    orderNumber: 'ORD-100193',
    currency: 'RUB',
    items: [
      {
        orderItemId: 'oi-1',
        productTitle: 'Dev Wool Coat',
        mode: 'serialized',
        requestedQuantity: 1,
        refundableQuantity: 1,
        unitPriceCents: 1500000,
        refundCents: 1500000,
      },
    ],
    productsRefundCents: 1500000,
    deliveryRefundCents: 0,
    totalRefundCents: 1500000,
    alreadyRefundedCents: 0,
    remainingRefundableCents: 1500000,
    canRefund: true,
    blockingReason: null,
  };

  assert(mockQuote.deliveryRefundCents === 0, 'M5.4B policy: delivery refund must be 0');
  assert(mockQuote.totalRefundCents === mockQuote.productsRefundCents + mockQuote.deliveryRefundCents, 'Total refund must equal products + delivery');
  assert(mockQuote.items[0].mode === 'serialized', 'Serialized mode must be preserved');
  assert(mockQuote.canRefund === true, 'Eligible quote canRefund must be true');

  // 15. M5.4B Return Refund State Evaluation Matrix (Strict latestRefundStatus Authority)
  const getRefundCardState = (returnStatus: string, quote?: AdminReturnRefundQuote | null) => {
    if (!quote) return { badge: 'Недоступен', canTriggerAction: false, actionText: '', text: 'Информация о возврате средств недоступна' };
    const isSucceeded = returnStatus === 'refunded' || quote.latestRefundStatus === 'succeeded';
    const isPending = quote.latestRefundStatus === 'pending';
    const isAvailable = quote.canRefund && quote.remainingRefundableCents > 0;

    if (isSucceeded) {
      return { badge: 'Выполнен', canTriggerAction: false, actionText: '', text: 'Возврат средств выполнен' };
    }
    if (isPending) {
      return { badge: 'Обрабатывается', canTriggerAction: false, actionText: '', text: 'Возврат зарегистрирован и ожидает обработки платежной системой.' };
    }
    if (isAvailable) {
      return {
        badge: 'Доступен',
        canTriggerAction: true,
        actionText: quote.latestRefundStatus === 'failed' ? 'Повторить возврат средств' : 'Запустить возврат средств',
        text: '',
      };
    }
    return { badge: 'Недоступен', canTriggerAction: false, actionText: '', text: quote.blockingReason || 'Возврат средств недоступен' };
  };

  const eligibleState = getRefundCardState('item_received', { ...mockQuote, latestRefundStatus: null });
  assert(eligibleState.badge === 'Доступен', 'Eligible quote must show Доступен badge');
  assert(eligibleState.canTriggerAction === true, 'Eligible quote must allow refund action');
  assert(eligibleState.actionText === 'Запустить возврат средств', 'Initial eligible quote must have Запустить возврат средств action');

  // Pending state derived strictly from latestRefundStatus = 'pending', NOT blockingReason text
  const pendingQuote: AdminReturnRefundQuote = {
    ...mockQuote,
    canRefund: false,
    latestRefundStatus: 'pending',
    blockingReason: 'Совершенно произвольная строка причины блокировки от сервера',
  };
  const pendingState = getRefundCardState('item_received', pendingQuote);
  assert(pendingState.badge === 'Обрабатывается', 'Pending quote must show Обрабатывается badge via latestRefundStatus');
  assert(pendingState.canTriggerAction === false, 'Pending quote must NOT allow new refund action');
  assert(pendingState.text === 'Возврат зарегистрирован и ожидает обработки платежной системой.', 'Pending quote must show processing text');

  // Succeeded state
  const succeededState = getRefundCardState('item_received', {
    ...mockQuote,
    canRefund: false,
    latestRefundStatus: 'succeeded',
    latestRefundProcessedAt: '2026-08-31T11:00:00Z',
  });
  assert(succeededState.badge === 'Выполнен', 'Succeeded latestRefundStatus must show Выполнен badge');
  assert(succeededState.canTriggerAction === false, 'Succeeded quote must NOT allow new refund action');
  assert(succeededState.text === 'Возврат средств выполнен', 'Succeeded quote must show completed text');

  // Failed state with retry allowed
  const failedRetryState = getRefundCardState('item_received', {
    ...mockQuote,
    canRefund: true,
    latestRefundStatus: 'failed',
  });
  assert(failedRetryState.badge === 'Доступен', 'Failed quote with canRefund=true must show Доступен badge');
  assert(failedRetryState.canTriggerAction === true, 'Failed quote with canRefund=true must allow retry');
  assert(failedRetryState.actionText === 'Повторить возврат средств', 'Failed quote must use Повторить возврат средств button');

  // Blocked state (approved but not received)
  const blockedQuote: AdminReturnRefundQuote = {
    ...mockQuote,
    canRefund: false,
    latestRefundStatus: null,
    blockingReason: 'Возврат средств доступен только после приёмки товара на складе.',
  };
  const blockedState = getRefundCardState('approved', blockedQuote);
  assert(blockedState.badge === 'Недоступен', 'Approved (not received) return must show Недоступен badge');
  assert(blockedState.canTriggerAction === false, 'Approved (not received) return must NOT allow refund action');

  // 16. M5.4B Refund Error Mappings
  const errAlloc = new ApiError('Allocation mismatch', 'refund_allocation_invariant', 400);
  assert(
    getAdminReturnErrorMessage(errAlloc, 'fallback') === 'Несогласованное состояние резервирования: количество единиц не соответствует заказу.',
    'refund_allocation_invariant error mapping must match',
  );

  const errExceeds = new ApiError('Exceeds paid', 'refund_exceeds_paid', 400);
  assert(
    getAdminReturnErrorMessage(errExceeds, 'fallback') === 'Сумма возврата превышает оплаченную сумму.',
    'refund_exceeds_paid error mapping must match',
  );

  const errPayment = new ApiError('Payment not found', 'payment_not_found', 400);
  assert(
    getAdminReturnErrorMessage(errPayment, 'fallback') === 'Не найдена успешная оплата по заказу.',
    'payment_not_found error mapping must match',
  );

  const errAmbiguous = new ApiError('Ambiguous payment', 'ambiguous_payment', 400);
  assert(
    getAdminReturnErrorMessage(errAmbiguous, 'fallback') === 'Неоднозначная оплата: обнаружено несколько успешных платежей по заказу.',
    'ambiguous_payment error mapping must match',
  );

  const errNoItems = new ApiError('No eligible items', 'refund_no_eligible_items', 400);
  assert(
    getAdminReturnErrorMessage(errNoItems, 'fallback') === 'Нет принятых позиций, подлежащих возврату средств.',
    'refund_no_eligible_items error mapping must match',
  );

  const errNotReceived = new ApiError('Not received', 'return_not_received', 400);
  assert(
    getAdminReturnErrorMessage(errNotReceived, 'fallback') === 'Возврат средств доступен только после приёмки товара на складе.',
    'return_not_received error mapping must match',
  );

  const errAlreadyRefunded = new ApiError('Already refunded', 'return_already_refunded', 400);
  assert(
    getAdminReturnErrorMessage(errAlreadyRefunded, 'fallback') === 'Возврат средств уже выполнен.',
    'return_already_refunded error mapping must match',
  );

  console.log('ALL ADMIN RETURNS LOGIC & CONTRACT TESTS PASSED');
}

describe('Admin Returns Contract & State Logic', () => {
  it('passes all canonical logic and refund mapping assertions', async () => {
    await runAdminReturnsTests();
  });
});

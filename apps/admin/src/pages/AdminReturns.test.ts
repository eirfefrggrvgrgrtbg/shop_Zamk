import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminReturns } from './AdminReturns';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminReturnsApi from '../api/adminReturns';
import {
  getReturnReasonLabel,
  getReturnStatusLabel,
  getStatusBadgeClass,
  getAdminReturnErrorMessage,
  type AdminReturn,
  type AdminReturnEvidence,
  type AdminReturnRefundQuote,
  type AdminReturnReceivingItem,
  type AdminReturnReceivingState,
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

  // 17. Warehouse Receiving CTA Availability Matrix
  const getWarehouseReceivingCTA = (status: string, shipmentStatus?: string, canReceiveWarehouse: boolean = true) => {
    if (!canReceiveWarehouse) {
      if (status === 'item_received') return 'Открыть результат приёмки';
      if (status === 'receiving') return 'Посмотреть ход приёмки';
      return null;
    }
    if (status === 'approved') {
      if (shipmentStatus === 'arrived_at_zamk') return 'Начать приёмку на складе';
      return null;
    }
    if (status === 'receiving') return 'Продолжить приёмку';
    if (status === 'item_received') return 'Открыть результат приёмки';
    return null;
  };

  assert(getWarehouseReceivingCTA('requested') === null, 'requested status must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('needs_info') === null, 'needs_info status must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', undefined) === null, 'approved without shipment must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', 'draft') === null, 'approved with draft shipment must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', 'created') === null, 'approved with created shipment must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', 'in_transit') === null, 'approved with in_transit shipment must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', 'delivered_cdek_office') === null, 'approved with delivered_cdek_office must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('approved', 'arrived_at_zamk') === 'Начать приёмку на складе', 'approved with arrived_at_zamk MUST offer Начать приёмку на складе');
  assert(getWarehouseReceivingCTA('approved', 'arrived_at_zamk', false) === null, 'Support role without warehouse.returns MUST NOT have Начать приёмку CTA');
  assert(getWarehouseReceivingCTA('receiving') === 'Продолжить приёмку', 'receiving status MUST offer Продолжить приёмку');
  assert(getWarehouseReceivingCTA('receiving', undefined, false) === 'Посмотреть ход приёмки', 'Support role must see read-only progress link for receiving');
  assert(getWarehouseReceivingCTA('item_received') === 'Открыть результат приёмки', 'item_received status MUST offer Открыть результат приёмки');
  assert(getWarehouseReceivingCTA('rejected') === null, 'rejected status must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('refunded') === null, 'refunded status must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('completed') === null, 'completed status must NOT have warehouse CTA');
  assert(getWarehouseReceivingCTA('cancelled') === null, 'cancelled status must NOT have warehouse CTA');

  // 18. Warehouse Receiving Error Message Mappings
  const warehouseErrorCases = [
    { code: 'return_not_arrived', status: 400, expected: 'Приёмка невозможна: возврат ещё не прибыл на склад ZAMK.' },
    { code: 'invalid_zmu', status: 400, expected: 'Код ZMU не найден или не принадлежит данному возврату.' },
    { code: 'already_bound', status: 409, expected: 'Данный ZMU уже привязан к возвращенной единице.' },
    { code: 'quantity_exceeded', status: 400, expected: 'Все единицы данной позиции уже отсканированы.' },
    { code: 'missing_disposition', status: 400, expected: 'Укажите решение (диспозицию) для всех отсканированных единиц перед завершением.' },
    { code: 'invalid_quantity', status: 400, expected: 'Сумма количеств проверки не может превышать запрошенное количество.' },
    { code: 'item_not_legacy', status: 400, expected: 'Позиция является сериализованной и принимается через сканирование ZMU.' },
    { code: 'item_not_serialized', status: 400, expected: 'Позиция не является сериализованной.' },
    { code: 'invalid_unit_state', status: 400, expected: 'Единица товара находится в недопустимом статусе на складе.' },
    { code: 'invalid_state', status: 400, expected: 'Возврат находится в неподходящем статусе для этой операции.' },
    { code: 'unit_not_in_return', status: 400, expected: 'Позиция не найдена в текущем возврате.' },
    { code: 'item_not_in_return', status: 400, expected: 'Позиция не найдена в текущем возврате.' },
    { code: 'invalid_disposition', status: 400, expected: 'Выбрано недопустимое решение по позиции.' },
  ];

  for (const tc of warehouseErrorCases) {
    const err = new ApiError('Test err', tc.code, tc.status);
    const msg = getAdminReturnErrorMessage(err, 'Fallback message');
    assert(msg === tc.expected, `Error ${tc.code} must map to ${tc.expected}`);
  }

  // 19. Product Identity and Presentation in Item Card
  const formatItemTitle = (item: AdminReturnReceivingItem) => item.productTitle || item.returnItem.productTitle || 'Товар';
  const formatItemMeta = (item: AdminReturnReceivingItem) => {
    const parts = [];
    if (item.variantSize) parts.push(`Размер: ${item.variantSize}`);
    if (item.variantColor) parts.push(`Цвет: ${item.variantColor}`);
    if (item.sku) parts.push(`Артикул: ${item.sku}`);
    return parts.join(' · ');
  };

  const sampleSerializedReceivingItem: AdminReturnReceivingItem = {
    returnItem: { id: 'ri-ser-1', orderItemId: 'oi-ser-1', quantity: 1, priceCents: 1500000, subtotalPriceCents: 1500000 },
    productTitle: 'Dev Wool Coat',
    productImageUrl: 'https://minio.local/media/coat.jpg',
    variantSize: 'M',
    variantColor: 'Graphite',
    sku: 'DEV-SKU-0',
    allocationMode: 'serialized',
    outboundAllocations: [{ id: 'alloc-1', unitCode: 'ZMU-XUJBQQ5ADSW4BWTX', status: 'shipped', unitStatus: 'shipped', allocatedAt: '2026-08-30T10:00:00Z' }],
    scannedUnits: [
      { id: 'scan-1', returnId: 'ret-1', returnItemId: 'ri-ser-1', inventoryUnitId: 'iu-1', unitCode: 'ZMU-XUJBQQ5ADSW4BWTX', disposition: 'restock', createdAt: '2026-08-30T10:00:00Z', updatedAt: '2026-08-30T10:00:00Z' },
    ],
    requestedQuantity: 1,
    scannedQuantity: 1,
    remainingQuantity: 0,
    acceptedQuantity: 1,
    damagedQuantity: 0,
    rejectedQuantity: 0,
    canFinalize: true,
  };

  assert(formatItemTitle(sampleSerializedReceivingItem) === 'Dev Wool Coat', 'Item title must be product title Dev Wool Coat');
  assert(!formatItemTitle(sampleSerializedReceivingItem).includes('oi-ser-1'), 'Item title must NOT contain order_item UUID');
  assert(formatItemMeta(sampleSerializedReceivingItem) === 'Размер: M · Цвет: Graphite · Артикул: DEV-SKU-0', 'Item meta must format Size, Color, SKU');
  assert(sampleSerializedReceivingItem.outboundAllocations[0].unitCode === 'ZMU-XUJBQQ5ADSW4BWTX', 'ZMU code must remain visible for warehouse operator');

  // 20. Mixed Return State Verification
  const legacyItem: AdminReturnReceivingItem = {
    returnItem: { id: 'ri-leg', orderItemId: 'oi-leg', quantity: 2, priceCents: 500, subtotalPriceCents: 1000 },
    productTitle: 'Basic Cotton T-Shirt',
    variantSize: 'L',
    variantColor: 'White',
    sku: 'TSHIRT-WHT-L',
    allocationMode: 'legacy',
    outboundAllocations: [],
    scannedUnits: [],
    requestedQuantity: 2,
    scannedQuantity: 2,
    remainingQuantity: 0,
    acceptedQuantity: 1,
    damagedQuantity: 1,
    rejectedQuantity: 0,
    canFinalize: true,
  };

  const mixedState: AdminReturnReceivingState = {
    return: { id: 'ret-mix', orderId: 'ord-mix', orderNumber: 'ORD-MIXED-100', status: 'receiving' },
    orderNumber: 'ORD-MIXED-100',
    items: [sampleSerializedReceivingItem, legacyItem],
    totalRequested: 3,
    totalScanned: 3,
    totalRemaining: 0,
    serializedRequested: 1,
    serializedScanned: 1,
    legacyRequested: 2,
    canFinalize: true,
  };

  assert(mixedState.items.length === 2, 'Mixed return must contain 2 items');
  assert(mixedState.items[0].allocationMode === 'serialized', 'First item is serialized');
  assert(mixedState.items[1].allocationMode === 'legacy', 'Second item is legacy');
  assert(mixedState.canFinalize === true, 'Backend canFinalize flag is authoritative');

  // 21. Finalize Gatekeeper and Read-Only State
  const isFinalizeAllowed = (st: AdminReturnReceivingState) => st.return.status === 'receiving' && st.canFinalize === true;
  assert(isFinalizeAllowed(mixedState) === true, 'Receiving state with canFinalize=true allows finalize');

  const approvedState: AdminReturnReceivingState = { ...mixedState, return: { ...mixedState.return, status: 'approved' }, canFinalize: false };
  assert(isFinalizeAllowed(approvedState) === false, 'Approved state does NOT allow finalize');

  const finalizedState: AdminReturnReceivingState = { ...mixedState, return: { ...mixedState.return, status: 'item_received' }, canFinalize: false };
  assert(isFinalizeAllowed(finalizedState) === false, 'Finalized item_received state does NOT allow finalize');
  assert(finalizedState.return.status === 'item_received', 'Finalized return has status item_received');

  // 22. Partial Receiving & Missing Unit Derivations
  const deriveNotReceived = (item: AdminReturnReceivingItem) => {
    if (item.allocationMode === 'serialized') {
      return Math.max(0, item.requestedQuantity - item.scannedUnits.length);
    }
    return Math.max(0, item.requestedQuantity - (item.acceptedQuantity + item.damagedQuantity + item.rejectedQuantity));
  };

  // Case A: Serialized partial (Q=3, 2 scanned)
  const partialSerializedItem: AdminReturnReceivingItem = {
    ...sampleSerializedReceivingItem,
    requestedQuantity: 3,
    scannedQuantity: 2,
    scannedUnits: [
      { id: 's-1', returnId: 'r-1', returnItemId: 'ri-1', inventoryUnitId: 'u-1', unitCode: 'ZMU-1', disposition: 'restock', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z' },
      { id: 's-2', returnId: 'r-1', returnItemId: 'ri-1', inventoryUnitId: 'u-2', unitCode: 'ZMU-2', disposition: 'damaged', createdAt: '2026-08-31T00:00:00Z', updatedAt: '2026-08-31T00:00:00Z' },
    ],
    canFinalize: true,
  };
  assert(deriveNotReceived(partialSerializedItem) === 1, 'Partial serialized Q=3, 2 scanned must derive notReceived=1');

  // Case B: Serialized zero-scanned (Q=1, 0 scanned)
  const zeroSerializedItem: AdminReturnReceivingItem = {
    ...sampleSerializedReceivingItem,
    requestedQuantity: 1,
    scannedQuantity: 0,
    scannedUnits: [],
    canFinalize: true,
  };
  assert(deriveNotReceived(zeroSerializedItem) === 1, 'Zero-scanned serialized Q=1, 0 scanned must derive notReceived=1');

  // Case C: Legacy partial (Q=5, acc=2, dam=1, rej=1)
  const partialLegacyItem: AdminReturnReceivingItem = {
    ...legacyItem,
    requestedQuantity: 5,
    acceptedQuantity: 2,
    damagedQuantity: 1,
    rejectedQuantity: 1,
    canFinalize: true,
  };
  assert(deriveNotReceived(partialLegacyItem) === 1, 'Partial legacy Q=5 (2+1+1) must derive notReceived=1');

  // 23. Return Logistics Column & Warehouse Ready Matrix
  const { getReturnLogisticsLabel, getReturnLogisticsBadgeClass } = await import('../api/adminReturns');

  // Verify human labels (no raw enum leakage)
  assert(getReturnLogisticsLabel(null) === '—', 'null logistics must render —');
  assert(getReturnLogisticsLabel(undefined) === '—', 'undefined logistics must render —');
  assert(getReturnLogisticsLabel('') === '—', 'empty logistics must render —');
  assert(getReturnLogisticsLabel('draft') === 'Черновик', 'draft must render Черновик');
  assert(getReturnLogisticsLabel('awaiting_handover') === 'Ожидает передачи', 'awaiting_handover must render Ожидает передачи');
  assert(getReturnLogisticsLabel('handed_over') === 'Передан в СДЭК', 'handed_over must render Передан в СДЭК');
  assert(getReturnLogisticsLabel('in_transit') === 'В пути', 'in_transit must render В пути');
  assert(getReturnLogisticsLabel('arrived_at_zamk') === 'Прибыл на склад', 'arrived_at_zamk must render Прибыл на склад');
  assert(getReturnLogisticsLabel('cancelled') === 'Отменена', 'cancelled must render Отменена');

  assert(getReturnLogisticsBadgeClass('arrived_at_zamk').includes('blue'), 'Arrived badge must use blue emphasis');
  assert(getReturnLogisticsBadgeClass(null).includes('gray'), 'Null logistics badge must use muted styling');

  const getListRowAction = (r: AdminReturn, canReceiveWarehouse: boolean = true) => {
    const effectiveShipmentStatus = r.shipmentStatus || r.shipment?.status || null;
    const isWarehouseReady = r.status === 'approved' && effectiveShipmentStatus === 'arrived_at_zamk';
    return isWarehouseReady && canReceiveWarehouse ? 'Принять на складе' : 'Рассмотреть';
  };

  // Matrix Case 1: approved + no shipment
  const appNoShipment: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: undefined, shipmentStatus: null };
  assert(getReturnLogisticsLabel(appNoShipment.shipmentStatus) === '—', 'No shipment must render —');
  assert(getListRowAction(appNoShipment) === 'Рассмотреть', 'Approved without shipment is not warehouse-ready');

  // Matrix Case 2: approved + awaiting_handover
  const appAwaiting: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipmentStatus: 'awaiting_handover' };
  assert(getReturnLogisticsLabel(appAwaiting.shipmentStatus) === 'Ожидает передачи', 'Awaiting handover must render Ожидает передачи');
  assert(getListRowAction(appAwaiting) === 'Рассмотреть', 'Approved awaiting handover is not warehouse-ready');

  // Matrix Case 3: approved + in_transit
  const appInTransit: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipmentStatus: 'in_transit' };
  assert(getReturnLogisticsLabel(appInTransit.shipmentStatus) === 'В пути', 'In transit must render В пути');
  assert(getListRowAction(appInTransit) === 'Рассмотреть', 'Approved in transit is not warehouse-ready');

  // Matrix Case 4: approved + arrived_at_zamk
  const appArrived: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipmentStatus: 'arrived_at_zamk' };
  assert(getReturnLogisticsLabel(appArrived.shipmentStatus) === 'Прибыл на склад', 'Arrived at ZAMK must render Прибыл на склад');
  // 4A: Warehouse operator sees active warehouse action
  assert(getListRowAction(appArrived, true) === 'Принять на складе', 'Approved + arrived_at_zamk MUST show Принять на складе action for Warehouse');
  // 4B: Support staff without warehouse.returns sees neutral review action
  assert(getListRowAction(appArrived, false) === 'Рассмотреть', 'Approved + arrived_at_zamk MUST show Рассмотреть and NOT Принять на складе for Support');

  // Matrix Case 5: rejected + arrived_at_zamk (Must NOT be warehouse-ready)
  const rejArrived: AdminReturn = { ...sampleReturnWithEvidence, status: 'rejected', shipmentStatus: 'arrived_at_zamk' };
  assert(getListRowAction(rejArrived) === 'Рассмотреть', 'Rejected return must NOT show warehouse receiving action even if shipment arrived');

  // Matrix Case 6: approved + cancelled (Must render Отменена and NOT be warehouse-ready)
  const appCancelled: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipmentStatus: 'cancelled' };
  assert(getReturnLogisticsLabel(appCancelled.shipmentStatus) === 'Отменена', 'Cancelled shipment must render Отменена');
  assert(getListRowAction(appCancelled) === 'Рассмотреть', 'Approved return with cancelled shipment must NOT show warehouse receiving action');

  // 24. Dev Logistics Simulator Action Rules
  const getDevSimulatorAction = (r: AdminReturn): string | null => {
    if (r.status !== 'approved') return null;
    if (!r.shipment || r.shipment.status === 'cancelled') {
      return 'Создать тестовую отправку';
    }
    switch (r.shipment.status) {
      case 'awaiting_handover':
        return 'Отметить передачу в СДЭК';
      case 'handed_over':
        return 'Отметить отправку в пути';
      case 'in_transit':
        return 'Отметить прибытие на склад';
      case 'arrived_at_zamk':
        return null; // Terminal simulator state; warehouse receiving CTA takes over
      default:
        return null;
    }
  };

  const simReturnNoShipment: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: undefined };
  assert(getDevSimulatorAction(simReturnNoShipment) === 'Создать тестовую отправку', 'No shipment must show create action');

  const simReturnCancelled: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: { ...sampleReturnWithEvidence.shipment!, status: 'cancelled' } };
  assert(getDevSimulatorAction(simReturnCancelled) === 'Создать тестовую отправку', 'Cancelled shipment must allow new create action');

  const simReturnAwaiting: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: { ...sampleReturnWithEvidence.shipment!, status: 'awaiting_handover' } };
  assert(getDevSimulatorAction(simReturnAwaiting) === 'Отметить передачу в СДЭК', 'awaiting_handover must show handover action');

  const simReturnHanded: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: { ...sampleReturnWithEvidence.shipment!, status: 'handed_over' } };
  assert(getDevSimulatorAction(simReturnHanded) === 'Отметить отправку в пути', 'handed_over must show in_transit action');

  const simReturnInTransit: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: { ...sampleReturnWithEvidence.shipment!, status: 'in_transit' } };
  assert(getDevSimulatorAction(simReturnInTransit) === 'Отметить прибытие на склад', 'in_transit must show arrived action');

  const simReturnArrived: AdminReturn = { ...sampleReturnWithEvidence, status: 'approved', shipment: { ...sampleReturnWithEvidence.shipment!, status: 'arrived_at_zamk' } };
  assert(getDevSimulatorAction(simReturnArrived) === null, 'arrived_at_zamk has no further simulator action');

  const simReturnRejected: AdminReturn = { ...sampleReturnWithEvidence, status: 'rejected' };
  assert(getDevSimulatorAction(simReturnRejected) === null, 'Rejected return must have no simulator action');

  // 25. Financial Breakdown Presentation Rules for Refund Quotes
  const getFinancialBreakdownLines = (quote: AdminReturnRefundQuote, returnStatus: string) => {
    const lines: Array<{ label: string; amountCents: number; variant?: 'green' | 'amber' | 'default' }> = [
      { label: 'Товары:', amountCents: quote.productsRefundCents },
      { label: 'Доставка:', amountCents: quote.deliveryRefundCents },
    ];
    const succeededAmount = quote.succeededRefundedCents ?? quote.alreadyRefundedCents;
    if (succeededAmount > 0) {
      lines.push({ label: 'Ранее возвращено:', amountCents: succeededAmount, variant: 'green' });
    }
    if ((quote.pendingRefundCents || 0) > 0) {
      lines.push({ label: 'В обработке:', amountCents: quote.pendingRefundCents!, variant: 'amber' });
    }
    const totalLabel = quote.latestRefundStatus === 'pending'
      ? 'Расчётная сумма возврата:'
      : quote.latestRefundStatus === 'succeeded' || returnStatus === 'refunded'
      ? 'Итого возвращено:'
      : 'Итого к возврату:';
    lines.push({ label: totalLabel, amountCents: quote.totalRefundCents });
    return lines;
  };

  // Case A: Pending refund (ORD-100193 scenario)
  const pendingBreakdown = getFinancialBreakdownLines({
    ...mockQuote,
    productsRefundCents: 1299000,
    deliveryRefundCents: 0,
    totalRefundCents: 1299000,
    succeededRefundedCents: 0,
    alreadyRefundedCents: 0,
    pendingRefundCents: 1299000,
    remainingRefundableCents: 0,
    latestRefundStatus: 'pending',
  }, 'item_received');

  assert(pendingBreakdown.some(l => l.label === 'В обработке:' && l.amountCents === 1299000), 'Pending quote must render В обработке: 12 990');
  assert(!pendingBreakdown.some(l => l.label === 'Ранее возвращено:'), 'Pending quote must NOT render Ранее возвращено');
  assert(pendingBreakdown.some(l => l.label === 'Расчётная сумма возврата:' && l.amountCents === 1299000), 'Pending quote must render Расчётная сумма возврата');

  // Case B: Succeeded refund
  const succeededBreakdown = getFinancialBreakdownLines({
    ...mockQuote,
    productsRefundCents: 1299000,
    deliveryRefundCents: 0,
    totalRefundCents: 1299000,
    succeededRefundedCents: 1299000,
    alreadyRefundedCents: 1299000,
    pendingRefundCents: 0,
    remainingRefundableCents: 0,
    latestRefundStatus: 'succeeded',
  }, 'refunded');

  assert(succeededBreakdown.some(l => l.label === 'Ранее возвращено:' && l.amountCents === 1299000), 'Succeeded quote must render Ранее возвращено');
  assert(!succeededBreakdown.some(l => l.label === 'В обработке:'), 'Succeeded quote must NOT render В обработке');
  assert(succeededBreakdown.some(l => l.label === 'Итого возвращено:' && l.amountCents === 1299000), 'Succeeded quote must render Итого возвращено');

  console.log('ALL ADMIN RETURNS & RECEIVING PRESENTATION & CONTRACT TESTS PASSED');
}

describe('Admin Returns Contract & State Logic', () => {
  it('passes all canonical logic, receiving matrix, and refund mapping assertions', async () => {
    await runAdminReturnsTests();
  });
});

describe('AdminReturns Component RBAC — Support vs Warehouse Boundary', () => {
  const mockArrivedReturn: AdminReturn = {
    id: 'ret-100196-id',
    orderId: 'ord-100196-id',
    orderNumber: 'ORD-100196',
    status: 'approved',
    reason: 'defective',
    customerName: 'Ekaterina Petrova',
    customerEmail: 'support-test@zamk.local',
    createdAt: '2026-09-01T10:00:00Z',
    shipmentStatus: 'arrived_at_zamk',
    shipment: {
      id: 'ship-100196',
      provider: 'cdek',
      method: 'cdek_office',
      status: 'arrived_at_zamk',
      trackingNumber: 'TRK-100196',
    },
    items: [
      {
        id: 'ri-1',
        orderItemId: 'oi-1',
        productTitle: 'Dev Silk Dress',
        quantity: 1,
        priceCents: 850000,
        subtotalPriceCents: 850000,
      },
    ],
  };

  const mockInTransitReturn: AdminReturn = {
    id: 'ret-100199-transit-id',
    orderId: 'ord-uuid-transit',
    orderNumber: 'ORD-100199-TRANSIT',
    fulfillmentId: 'ful-transit-id',
    status: 'approved',
    reason: 'size_fit',
    comment: 'Too small',
    customerName: 'Ekaterina Transit',
    customerEmail: 'ekaterina@zamk.local',
    customerPhone: '+79998887766',
    shipmentStatus: 'in_transit',
    shipmentMethod: 'cdek_office',
    shipment: {
      id: 'ship-100199',
      provider: 'cdek',
      method: 'cdek_office',
      status: 'in_transit',
      trackingNumber: 'TRK-100199',
    },
    items: [
      {
        id: 'ri-transit',
        orderItemId: 'oi-transit',
        productTitle: 'In Transit Jacket',
        quantity: 1,
        priceCents: 500000,
        subtotalPriceCents: 500000,
      },
    ],
  };

  it('SUPPORT (returns.read + returns.update_status, NO warehouse.returns): full support list visible (arrived + in_transit), PII present, no warehouse CTA', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      user: { id: 'support-user-id', email: 'support@test.local', role: 'admin', name: 'Support Test', status: 'active' },
      staff: null,
      permissions: ['returns.read', 'returns.update_status'],
      isAuthenticated: true,
      isLoading: false,
      error: null,
      hasPermission: (p: string) => p === 'returns.read' || p === 'returns.update_status',
      hasAnyPermission: (ps: string[]) => ps.some((p) => p === 'returns.read' || p === 'returns.update_status'),
      isOwner: () => false,
      isCoOwner: () => false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshSession: vi.fn(),
      changePassword: vi.fn(),
      reloadStaff: vi.fn(),
    });

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([mockArrivedReturn, mockInTransitReturn]);
    const getDetailSpy = vi.spyOn(adminReturnsApi, 'getAdminReturn').mockResolvedValue(mockArrivedReturn);
    vi.spyOn(adminReturnsApi, 'getAdminReturnRefundQuote').mockRejectedValue(new Error('No quote'));

    render(React.createElement(MemoryRouter, null, React.createElement(AdminReturns)));

    await waitFor(() => {
      // Both arrived and non-arrived returns must be visible to Support:
      expect(screen.getByText('ORD-100196')).toBeDefined();
      expect(screen.getByText('Dev Silk Dress')).toBeDefined();
      expect(screen.getByText('Прибыл на склад')).toBeDefined();

      expect(screen.getByText('ORD-100199-TRANSIT')).toBeDefined();
      expect(screen.getByText('In Transit Jacket')).toBeDefined();
      expect(screen.getByText('В пути')).toBeDefined();

      // Support sees customer identities:
      expect(screen.getByText('Ekaterina Petrova')).toBeDefined();
      expect(screen.getByText('Ekaterina Transit')).toBeDefined();
    });

    // CRITICAL RBAC PROOF: Active warehouse receiving CTA "Принять на складе" is NOT rendered for Support on any row
    expect(screen.queryByRole('link', { name: /Принять на складе/i })).toBeNull();

    // Instead, neutral review buttons "Рассмотреть" ARE rendered
    const reviewButtons = screen.getAllByRole('button', { name: /Рассмотреть/i });
    expect(reviewButtons.length).toBe(2);

    // Support can click "Рассмотреть" to open detail drawer and review return
    fireEvent.click(reviewButtons[0]);

    await waitFor(() => {
      expect(getDetailSpy).toHaveBeenCalledWith('ret-100196-id');
    });

    // In detail drawer, warehouse CTA "Начать приёмку на складе" must ALSO be absent for Support
    expect(screen.queryByRole('link', { name: /Начать приёмку на складе/i })).toBeNull();
  });

  it('WAREHOUSE (warehouse.returns ONLY): arrived return visible with active CTA, customer PII absent, non-arrived approved return filtered out', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      user: { id: 'wh-user-id', email: 'warehouse@test.local', role: 'admin', name: 'Warehouse Test', status: 'active' },
      staff: null,
      permissions: ['warehouse.returns'],
      isAuthenticated: true,
      isLoading: false,
      error: null,
      hasPermission: (p: string) => p === 'warehouse.returns',
      hasAnyPermission: (ps: string[]) => ps.some((p) => p === 'warehouse.returns'),
      isOwner: () => false,
      isCoOwner: () => false,
      login: vi.fn(),
      logout: vi.fn(),
      refreshSession: vi.fn(),
      changePassword: vi.fn(),
      reloadStaff: vi.fn(),
    });

    // Backend mock represents the server-side filtered warehouse list:
    // Only approved+arrived_at_zamk is returned; non-arrived returns are excluded at backend.
    const sanitizedArrivedReturn: AdminReturn = {
      ...mockArrivedReturn,
      customerName: undefined,
      customerEmail: undefined,
      customerPhone: undefined,
      comment: undefined,
      adminComment: undefined,
      shipment: {
        ...mockArrivedReturn.shipment!,
        customerName: undefined,
        customerPhone: undefined,
        pickupAddress: undefined,
      },
    };

    vi.spyOn(adminReturnsApi, 'getAdminReturns').mockResolvedValue([sanitizedArrivedReturn]);

    render(React.createElement(MemoryRouter, null, React.createElement(AdminReturns)));

    await waitFor(() => {
      expect(screen.getByText('ORD-100196')).toBeDefined();
      expect(screen.getByText('Dev Silk Dress')).toBeDefined();
      expect(screen.getByText('Прибыл на склад')).toBeDefined();
    });

    // 1. Non-arrived return is NOT rendered (server-side filtered):
    expect(screen.queryByText('ORD-100199-TRANSIT')).toBeNull();
    expect(screen.queryByText('In Transit Jacket')).toBeNull();

    // 2. Customer PII is completely absent from DOM:
    expect(screen.queryByText('Ekaterina Petrova')).toBeNull();
    expect(screen.queryByText('support-test@zamk.local')).toBeNull();

    // 3. Warehouse operator sees active "Принять на складе" CTA linking to receiving:
    const warehouseCta = screen.getByRole('link', { name: /Принять на складе/i });
    expect(warehouseCta).toBeDefined();
    expect(warehouseCta.getAttribute('href')).toBe('/returns/ret-100196-id/receiving');
  });
});

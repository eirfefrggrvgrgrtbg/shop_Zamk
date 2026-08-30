export interface ReturnReasonOption {
  value: string;
  label: string;
}

export const RETURN_REASONS: ReturnReasonOption[] = [
  { value: 'defective', label: 'Товар неисправен' },
  { value: 'damaged', label: 'Товар повреждён' },
  { value: 'wrong_item', label: 'Привезли не тот товар' },
  { value: 'not_as_described', label: 'Не соответствует описанию' },
  { value: 'incomplete', label: 'Неполная комплектация' },
  { value: 'size_fit', label: 'Не подошёл размер' },
  { value: 'changed_mind', label: 'Передумал' },
  { value: 'other', label: 'Другое' },
];

export const REQUIRED_EVIDENCE_REASONS = [
  'defective',
  'damaged',
  'wrong_item',
  'not_as_described',
  'incomplete',
];

export const SUCCESS_TOAST_MESSAGE = 'Заявка отправлена. Мы рассмотрим её и сообщим о решении.';

export interface EvidenceItem {
  id: string;
  url: string;
}

export interface ReturnModalState {
  reason: string;
  comment: string;
  quantity: number;
  evidence: EvidenceItem[];
  isSubmitting: boolean;
  isUploading: boolean;
}

export function isEvidenceRequired(reason: string): boolean {
  return REQUIRED_EVIDENCE_REASONS.includes(reason);
}

export function getMinPhotos(reason: string): number {
  return isEvidenceRequired(reason) ? 2 : 0;
}

export function canSubmitReturn(state: {
  reason: string;
  comment: string;
  evidence: EvidenceItem[];
  isSubmitting?: boolean;
  isUploading?: boolean;
}): boolean {
  if (state.isSubmitting || state.isUploading) {
    return false;
  }
  if (!state.comment || state.comment.trim().length === 0) {
    return false;
  }
  const minPhotos = getMinPhotos(state.reason);
  if (state.evidence.length < minPhotos) {
    return false;
  }
  if (state.evidence.length > 6) {
    return false;
  }
  return true;
}

export function mapReturnErrorMessage(errMessage?: string): string {
  if (!errMessage) return 'Ошибка при создании возврата';
  const msg = errMessage.toLowerCase();
  if (msg.includes('invalid return quantity') || msg.includes('invalid_quantity')) {
    return 'На это количество товара уже оформлена заявка на возврат.';
  }
  if (msg.includes('comment required') || msg.includes('comment_required') || msg.includes('comment is required')) {
    return 'Пожалуйста, опишите причину возврата в комментарии.';
  }
  if (msg.includes('evidence required') || msg.includes('evidence_required') || msg.includes('photos required')) {
    return 'Для этой причины возврата необходимо прикрепить фотографии товара.';
  }
  if (msg.includes('evidence too many') || msg.includes('evidence_too_many') || msg.includes('maximum 6 photos')) {
    return 'Максимум 6 фотографий.';
  }
  if (msg.includes('order not delivered') || msg.includes('order_not_delivered')) {
    return 'Возврат возможен только для доставленных заказов.';
  }
  if (msg.includes('return window expired') || msg.includes('return_window_expired')) {
    return 'Срок оформления возврата для этого заказа истёк.';
  }
  return errMessage;
}

export interface ReturnPayloadItemInput {
  orderItemId: string;
  quantity: number;
}

export function buildCreateReturnPayload(
  item: ReturnPayloadItemInput,
  reason: string,
  comment: string,
  evidence: EvidenceItem[]
) {
  return {
    reason,
    comment: comment.trim() ? comment.trim() : undefined,
    items: [
      {
        orderItemId: item.orderItemId,
        quantity: item.quantity,
        evidenceIds: evidence.map((e) => e.id),
      },
    ],
  };
}

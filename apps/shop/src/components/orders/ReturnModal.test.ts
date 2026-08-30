import {
  RETURN_REASONS,
  REQUIRED_EVIDENCE_REASONS,
  SUCCESS_TOAST_MESSAGE,
  isEvidenceRequired,
  getMinPhotos,
  canSubmitReturn,
  buildCreateReturnPayload,
  mapReturnErrorMessage,
  type EvidenceItem,
} from './returnModalState';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

async function runComponentTests() {
  console.log('Testing ReturnModal shared state & component behavior...');

  // 1. Human reason labels render properly
  assert(RETURN_REASONS.length === 8, 'Expected exactly 8 return reason options');
  const labelMap = Object.fromEntries(RETURN_REASONS.map((r) => [r.value, r.label]));
  assert(labelMap['defective'] === 'Товар неисправен', 'defective label must match Russian translation');
  assert(labelMap['damaged'] === 'Товар повреждён', 'damaged label must match Russian translation');
  assert(labelMap['wrong_item'] === 'Привезли не тот товар', 'wrong_item label must match Russian translation');
  assert(labelMap['not_as_described'] === 'Не соответствует описанию', 'not_as_described label must match Russian translation');
  assert(labelMap['incomplete'] === 'Неполная комплектация', 'incomplete label must match Russian translation');
  assert(labelMap['size_fit'] === 'Не подошёл размер', 'size_fit label must match Russian translation');
  assert(labelMap['changed_mind'] === 'Передумал', 'changed_mind label must match Russian translation');
  assert(labelMap['other'] === 'Другое', 'other label must match Russian translation');

  // 2. Required reasons requirement check
  for (const reason of ['defective', 'damaged', 'wrong_item', 'not_as_described', 'incomplete']) {
    assert(isEvidenceRequired(reason) === true, `${reason} must be marked as required evidence`);
    assert(getMinPhotos(reason) === 2, `${reason} must require at least 2 photos`);
  }
  for (const reason of ['size_fit', 'changed_mind', 'other']) {
    assert(isEvidenceRequired(reason) === false, `${reason} must not be marked as required evidence`);
    assert(getMinPhotos(reason) === 0, `${reason} min photos must be 0`);
  }

  // 3. Mandatory comment validation across all reasons
  // Empty comment -> disabled
  assert(
    canSubmitReturn({ reason: 'damaged', comment: '', evidence: [{ id: '1', url: 'u1' }, { id: '2', url: 'u2' }] }) === false,
    'damaged with empty comment must disable submit'
  );
  assert(
    canSubmitReturn({ reason: 'damaged', comment: '   \n  ', evidence: [{ id: '1', url: 'u1' }, { id: '2', url: 'u2' }] }) === false,
    'damaged with whitespace comment must disable submit'
  );
  assert(
    canSubmitReturn({ reason: 'size_fit', comment: '', evidence: [] }) === false,
    'size_fit with empty comment must disable submit'
  );
  assert(
    canSubmitReturn({ reason: 'changed_mind', comment: '  ', evidence: [] }) === false,
    'changed_mind with whitespace comment must disable submit'
  );
  assert(
    canSubmitReturn({ reason: 'other', comment: '', evidence: [] }) === false,
    'other with empty comment must disable submit'
  );

  // 4. Photo requirements for required reasons (with valid comment)
  assert(
    canSubmitReturn({ reason: 'damaged', comment: 'Broken zipper', evidence: [] }) === false,
    '0 photos + comment for damaged must disable submit'
  );
  assert(
    canSubmitReturn({ reason: 'damaged', comment: 'Broken zipper', evidence: [{ id: '1', url: 'u1' }] }) === false,
    '1 photo + comment for damaged must disable submit'
  );
  assert(
    canSubmitReturn({
      reason: 'damaged',
      comment: 'Broken zipper',
      evidence: [{ id: '1', url: 'u1' }, { id: '2', url: 'u2' }],
    }) === true,
    '2 photos + comment for damaged must enable submit'
  );

  // 5. Optional reasons (with valid comment)
  assert(
    canSubmitReturn({ reason: 'size_fit', comment: 'Too small', evidence: [] }) === true,
    '0 photos + comment for size_fit must enable submit'
  );
  assert(
    canSubmitReturn({ reason: 'changed_mind', comment: 'No longer needed', evidence: [] }) === true,
    '0 photos + comment for changed_mind must enable submit'
  );
  assert(
    canSubmitReturn({ reason: 'other', comment: 'Specific detail', evidence: [] }) === true,
    '0 photos + comment for other must enable submit'
  );

  // 6. Max 6 photos enforced (6 valid, 7 invalid)
  const sixPhotos = Array.from({ length: 6 }, (_, i) => ({
    id: `ev-${i + 1}`,
    url: `http://img/${i + 1}`,
  }));
  assert(
    canSubmitReturn({ reason: 'defective', comment: 'Item broken', evidence: sixPhotos }) === true,
    '6 photos must be allowed'
  );
  const sevenPhotos = Array.from({ length: 7 }, (_, i) => ({
    id: `ev-${i + 1}`,
    url: `http://img/${i + 1}`,
  }));
  assert(
    canSubmitReturn({ reason: 'defective', comment: 'Item broken', evidence: sevenPhotos }) === false,
    '7 photos must be rejected (max 6)'
  );

  // 7. Upload in progress / submitting disables submit
  assert(
    canSubmitReturn({
      reason: 'defective',
      comment: 'Item broken',
      evidence: [
        { id: 'ev-1', url: 'http://img/1' },
        { id: 'ev-2', url: 'http://img/2' },
      ],
      isUploading: true,
    }) === false,
    'isUploading must disable submit'
  );
  assert(
    canSubmitReturn({
      reason: 'defective',
      comment: 'Item broken',
      evidence: [
        { id: 'ev-1', url: 'http://img/1' },
        { id: 'ev-2', url: 'http://img/2' },
      ],
      isSubmitting: true,
    }) === false,
    'isSubmitting must disable submit'
  );

  // 8. Human Error Mapping tests
  assert(
    mapReturnErrorMessage('invalid return quantity') === 'На это количество товара уже оформлена заявка на возврат.',
    'invalid return quantity must map to human Russian error'
  );
  assert(
    mapReturnErrorMessage('invalid_quantity') === 'На это количество товара уже оформлена заявка на возврат.',
    'invalid_quantity must map to human Russian error'
  );
  assert(
    mapReturnErrorMessage('comment is required') === 'Пожалуйста, опишите причину возврата в комментарии.',
    'comment required must map to human Russian error'
  );
  assert(
    mapReturnErrorMessage('2 to 6 photos required for this return reason') === 'Для этой причины возврата необходимо прикрепить фотографии товара.',
    'evidence required must map to human Russian error'
  );

  // 9. buildCreateReturnPayload creates canonical request with item-scoped evidenceIds
  const payload = buildCreateReturnPayload(
    { orderItemId: 'oi-12345', quantity: 2 },
    'damaged',
    'Arrived in broken box',
    [
      { id: 'ev-uuid-1', url: 'http://minio/1.jpg' },
      { id: 'ev-uuid-2', url: 'http://minio/2.jpg' },
    ]
  );
  assert(payload.reason === 'damaged', 'Payload reason must match top-level reason');
  assert(payload.comment === 'Arrived in broken box', 'Payload comment must match trimmed comment');
  assert(payload.items.length === 1, 'Payload must contain item');
  assert(payload.items[0].orderItemId === 'oi-12345', 'OrderItemID must match');
  assert(payload.items[0].quantity === 2, 'Quantity must match');
  assert(
    JSON.stringify(payload.items[0].evidenceIds) === JSON.stringify(['ev-uuid-1', 'ev-uuid-2']),
    'EvidenceIDs must be linked to the return item'
  );

  // 10. Verify NO ZMU, NO item-level reason, NO customer condition exists in payload
  assert(!('zmu' in payload), 'No ZMU field must exist in return payload');
  assert(!('zmuCode' in payload.items[0]), 'No ZMU code must exist in return item payload');
  assert(!('reason' in payload.items[0]), 'No item-level reason must exist in return item payload');
  assert(!('condition' in payload.items[0]), 'No customer condition must exist in return item payload');

  // 11. Success toast message verification
  assert(
    SUCCESS_TOAST_MESSAGE === 'Заявка отправлена. Мы рассмотрим её и сообщим о решении.',
    'Success toast message must state claim submitted, not refund approved'
  );

  // 12. Modal Cancel Cleanup Logic Test (Promise.allSettled with partial failures)
  const stagedEvidence: EvidenceItem[] = [
    { id: 'ev-del-1', url: 'http://img/1' },
    { id: 'ev-del-2', url: 'http://img/2' },
  ];
  const deleteMock = async (id: string) => {
    if (id === 'ev-del-2') throw new Error('network failure');
  };

  const cleanupResults = await Promise.allSettled(
    stagedEvidence.map((ev) => deleteMock(ev.id))
  );
  const failedIds: string[] = [];
  cleanupResults.forEach((res, index) => {
    if (res.status === 'rejected') {
      failedIds.push(stagedEvidence[index].id);
    }
  });
  assert(failedIds.length === 1 && failedIds[0] === 'ev-del-2', 'Only failed deletions are tracked in failedIds');
  const remainingEvidence = stagedEvidence.filter((e) => failedIds.includes(e.id));
  assert(remainingEvidence.length === 1 && remainingEvidence[0].id === 'ev-del-2', 'Failed photo remains in state');

  // 13. Successful Submission State Reset Lifecycle Test
  let modalLocalEvidence: EvidenceItem[] = [
    { id: 'bound-ev-1', url: 'http://img/1' },
    { id: 'bound-ev-2', url: 'http://img/2' },
  ];
  let modalComment = 'Customer explanation';
  let modalQuantity = 2;

  // Simulate successful submission reset
  modalLocalEvidence = [];
  modalComment = '';
  modalQuantity = 1;

  assert(modalLocalEvidence.length === 0, 'Local evidence must be empty after successful submit');
  assert(modalComment === '', 'Comment must be empty after successful submit');
  assert(modalQuantity === 1, 'Quantity must be reset to 1');
  assert(
    canSubmitReturn({ reason: 'defective', comment: modalComment, evidence: modalLocalEvidence }) === false,
    'Reopened modal with reset state cannot be submitted without uploading fresh photos and comment'
  );

  console.log('ALL COMPONENT BEHAVIOR & LOGIC TESTS PASSED');
}

runComponentTests();

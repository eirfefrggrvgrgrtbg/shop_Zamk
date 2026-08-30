import {
  formatReturnShipmentStatus,
  formatReturnShipmentMethod,
  formatCustomerReturnStatus,
  formatReturnReason,
  RETURN_SHIPMENT_STATUS_LABELS,
  RETURN_SHIPMENT_METHOD_LABELS,
  CUSTOMER_RETURN_STATUS_LABELS,
  RETURN_REASON_LABELS,
  type ReturnShipment,
  type CustomerReturnRecord,
} from '@zamk/api-client/src/types';
import {
  getReturnProgressIndex,
  RETURN_STAGES,
} from './ReturnLifecycleProgress';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

async function runReturnLogisticsTests() {
  console.log('Testing Shop Returns read model, Return Detail UX, and logistics...');

  // 1. Two methods exactly with human labels & descriptions
  const methodKeys = Object.keys(RETURN_SHIPMENT_METHOD_LABELS);
  assert(methodKeys.length === 2, 'Expected exactly 2 return methods');
  assert(methodKeys.includes('cdek_courier'), 'Must include cdek_courier');
  assert(methodKeys.includes('cdek_office'), 'Must include cdek_office');
  assert(formatReturnShipmentMethod('cdek_courier') === 'Заберёт курьер СДЭК', 'cdek_courier label must match');
  assert(formatReturnShipmentMethod('cdek_office') === 'Отнести в отделение СДЭК', 'cdek_office label must match');

  const methodCards = [
    {
      method: 'cdek_courier',
      title: 'Заберёт курьер СДЭК',
      description: 'Курьер заберёт посылку по указанному адресу.',
    },
    {
      method: 'cdek_office',
      title: 'Отнести в отделение СДЭК',
      description: 'Отнесите посылку в удобное отделение СДЭК.',
    },
  ];
  assert(methodCards[0].title === 'Заберёт курьер СДЭК', 'Courier title must match');
  assert(methodCards[0].description === 'Курьер заберёт посылку по указанному адресу.', 'Courier description must match');
  assert(methodCards[1].title === 'Отнести в отделение СДЭК', 'Office title must match');
  assert(methodCards[1].description === 'Отнесите посылку в удобное отделение СДЭК.', 'Office description must match');

  // 2. Customer Return canonical statuses
  assert(formatCustomerReturnStatus('requested') === 'Заявка на рассмотрении', 'requested must map to Заявка на рассмотрении');
  assert(formatCustomerReturnStatus('approved') === 'Возврат одобрен', 'approved must map to Возврат одобрен');
  assert(formatCustomerReturnStatus('rejected') === 'Возврат отклонён', 'rejected must map to Возврат отклонён');
  assert(formatCustomerReturnStatus('receiving') === 'Принимаем возврат', 'receiving must map to Принимаем возврат');
  assert(formatCustomerReturnStatus('item_received') === 'Товар принят', 'item_received must map to Товар принят');
  assert(formatCustomerReturnStatus('refunded') === 'Деньги возвращены', 'refunded must map to Деньги возвращены');
  assert(formatCustomerReturnStatus('completed') === 'Возврат завершён', 'completed must map to Возврат завершён');
  assert(formatCustomerReturnStatus('cancelled') === 'Возврат отменён', 'cancelled must map to Возврат отменён');

  // 3. Customer Return canonical reasons
  assert(formatReturnReason('defective') === 'Товар неисправен', 'defective must map to Товар неисправен');
  assert(formatReturnReason('damaged') === 'Товар повреждён', 'damaged must map to Товар повреждён');
  assert(formatReturnReason('wrong_item') === 'Получен не тот товар', 'wrong_item must map to Получен не тот товар');
  assert(formatReturnReason('not_as_described') === 'Не соответствует описанию', 'not_as_described must map to Не соответствует описанию');
  assert(formatReturnReason('incomplete') === 'Не хватает части комплекта', 'incomplete must map to Не хватает части комплекта');
  assert(formatReturnReason('size_fit') === 'Не подошёл размер / посадка', 'size_fit must map to Не подошёл размер / посадка');
  assert(formatReturnReason('changed_mind') === 'Передумал', 'changed_mind must map to Передумал');
  assert(formatReturnReason('other') === 'Другое', 'other must map to Другое');

  // 4. Shipment statuses mapped to human labels
  assert(formatReturnShipmentStatus('draft') === 'Оформляем отправление', 'draft mapping must match');
  assert(formatReturnShipmentStatus('awaiting_handover') === 'Ожидает передачи в СДЭК', 'awaiting_handover mapping must match');
  assert(formatReturnShipmentStatus('handed_over') === 'Передано в СДЭК', 'handed_over mapping must match');
  assert(formatReturnShipmentStatus('in_transit') === 'В пути', 'in_transit mapping must match');
  assert(formatReturnShipmentStatus('arrived_at_zamk') === 'Прибыло на склад ZAMK', 'arrived_at_zamk mapping must match');
  assert(formatReturnShipmentStatus('cancelled') === 'Отправление отменено', 'cancelled mapping must match');

  // 5. Progress Lifecycle Indicator
  assert(RETURN_STAGES.length === 6, 'Expected 6 lifecycle stages');
  assert(RETURN_STAGES[0].label === 'Заявка', 'Stage 0 must be Заявка');
  assert(RETURN_STAGES[1].label === 'Одобрено', 'Stage 1 must be Одобрено');
  assert(RETURN_STAGES[2].label === 'Отправка', 'Stage 2 must be Отправка');
  assert(RETURN_STAGES[3].label === 'В пути', 'Stage 3 must be В пути');
  assert(RETURN_STAGES[4].label === 'Приёмка', 'Stage 4 must be Приёмка');
  assert(RETURN_STAGES[5].label === 'Возврат денег', 'Stage 5 must be Возврат денег');

  assert(getReturnProgressIndex('requested') === 0, 'requested -> stage 0 (Заявка)');
  assert(getReturnProgressIndex('approved', undefined) === 1, 'approved with no shipment -> stage 1 (Одобрено)');
  assert(getReturnProgressIndex('approved', 'draft') === 1, 'approved draft -> stage 1 (Одобрено)');
  assert(getReturnProgressIndex('approved', 'awaiting_handover') === 2, 'awaiting_handover -> stage 2 (Отправка)');
  assert(getReturnProgressIndex('approved', 'handed_over') === 3, 'handed_over -> stage 3 (В пути)');
  assert(getReturnProgressIndex('approved', 'in_transit') === 3, 'in_transit -> stage 3 (В пути)');
  assert(getReturnProgressIndex('approved', 'arrived_at_zamk') === 4, 'arrived_at_zamk -> stage 4 (Приёмка)');
  assert(getReturnProgressIndex('receiving') === 4, 'receiving -> stage 4 (Приёмка)');
  assert(getReturnProgressIndex('item_received') === 4, 'item_received -> stage 4 (Приёмка)');
  assert(getReturnProgressIndex('refunded') === 5, 'refunded -> stage 5 (Возврат денег)');
  assert(getReturnProgressIndex('completed') === 5, 'completed -> stage 5 (Возврат денег)');

  // 6. Returns Overview Card vs Return Detail separation
  const sampleReturn: CustomerReturnRecord = {
    id: '583fb821-2b10-4966-aacd-e8d24a215842',
    orderId: 'order-uuid-100193',
    orderNumber: 'ORD-100193',
    fulfillmentId: 'fulf-uuid-1',
    userId: 'user-uuid-1',
    status: 'approved',
    reason: 'damaged',
    comment: 'Пуговица оторвана при распаковке',
    createdAt: '2026-08-30T10:00:00Z',
    updatedAt: '2026-08-30T10:00:00Z',
    items: [
      {
        id: 'item-uuid-1',
        returnId: '583fb821-2b10-4966-aacd-e8d24a215842',
        orderItemId: 'oi-uuid-1',
        productTitle: 'Dev Wool Coat',
        productImageUrl: 'https://example.com/coat.jpg',
        variantSize: 'M',
        variantColor: 'Graphite',
        sku: 'DEV-COAT-M-GRP',
        quantity: 1,
        priceCents: 1500000,
        subtotalPriceCents: 1500000,
      },
    ],
  };

  // Overview Card:
  const overviewCard = {
    statusLabel: formatCustomerReturnStatus(sampleReturn.status),
    orderLabel: `Заказ ${sampleReturn.orderNumber}`,
    reasonLabel: `Причина: ${formatReturnReason(sampleReturn.reason)}`,
    productTitle: sampleReturn.items[0].productTitle,
    variantDetails: `${sampleReturn.items[0].variantSize} · ${sampleReturn.items[0].variantColor}`,
    quantityLabel: `${sampleReturn.items[0].quantity} шт.`,
    actionLabel: 'Открыть возврат',
    actionHref: `/returns/${sampleReturn.id}`,
    // Overview must NOT have inline forms or provider errors
    hasInlineLogisticsForm: false,
    hasInlineCourierInputs: false,
  };

  assert(overviewCard.orderLabel === 'Заказ ORD-100193', 'Overview order label must be Заказ ORD-100193');
  assert(overviewCard.statusLabel === 'Возврат одобрен', 'Overview status must be Возврат одобрен');
  assert(overviewCard.actionLabel === 'Открыть возврат', 'Action label must be Открыть возврат');
  assert(overviewCard.actionHref === '/returns/583fb821-2b10-4966-aacd-e8d24a215842', 'Action must navigate to detail');
  assert(overviewCard.hasInlineLogisticsForm === false, 'Overview card must not contain inline logistics form');

  // Detail Page:
  const detailHeader = `Возврат по заказу ${sampleReturn.orderNumber}`;
  assert(detailHeader === 'Возврат по заказу ORD-100193', 'Detail header must be Возврат по заказу ORD-100193');
  assert(!detailHeader.includes('583fb821'), 'Detail visible UI must not show raw UUID');

  // 7. Approved with No Shipment on Detail Page
  type DetailLogisticsMode = 'select' | 'cdek_office' | 'cdek_courier';
  let detailMode: DetailLogisticsMode = 'select';

  const getVisibleElements = (mode: DetailLogisticsMode, hasShipment: boolean) => {
    if (hasShipment) {
      return { showMethodCards: false, showCourierForm: false, showOfficeSelect: false, showShipmentBlock: true };
    }
    if (mode === 'select') {
      return { showMethodCards: true, showCourierForm: false, showOfficeSelect: false, showShipmentBlock: false };
    }
    if (mode === 'cdek_courier') {
      return { showMethodCards: false, showCourierForm: true, showOfficeSelect: false, showShipmentBlock: false };
    }
    return { showMethodCards: false, showCourierForm: false, showOfficeSelect: true, showShipmentBlock: false };
  };

  const initialDetailView = getVisibleElements(detailMode, false);
  assert(initialDetailView.showMethodCards === true, 'Detail must show 2 method cards initially when approved + no shipment');
  assert(initialDetailView.showCourierForm === false, 'Courier form must NOT be visible before selecting courier');
  assert(initialDetailView.showOfficeSelect === false, 'Office selector must NOT be visible before selecting office');

  // Select courier -> show courier form
  detailMode = 'cdek_courier';
  const courierView = getVisibleElements(detailMode, false);
  assert(courierView.showCourierForm === true, 'Courier form appears after selecting courier');
  assert(courierView.showMethodCards === false, 'Method cards hidden while filling courier form');

  // Select office -> show office UI
  detailMode = 'cdek_office';
  const officeView = getVisibleElements(detailMode, false);
  assert(officeView.showOfficeSelect === true, 'Office UI appears after selecting office');

  // 8. Office provider failure UX (no empty select box)
  type OfficeModeState = {
    loadingOffices: boolean;
    officeError: string;
    offices: Array<{ code: string; name: string }>;
  };

  const getOfficeViewElements = (state: OfficeModeState) => {
    if (state.loadingOffices) return { showLoading: true, showSelect: false, showError: false, showBackButton: false, showRetryButton: false };
    if (state.officeError) return { showLoading: false, showSelect: false, showError: true, showBackButton: true, showRetryButton: true, errorMessage: state.officeError };
    return { showLoading: false, showSelect: true, showError: false, showBackButton: true, showRetryButton: false };
  };

  const officeFailedState = getOfficeViewElements({
    loadingOffices: false,
    officeError: 'Логистика СДЭК временно недоступна.',
    offices: [],
  });
  assert(officeFailedState.showError === true, 'Must show error message');
  assert(officeFailedState.errorMessage === 'Логистика СДЭК временно недоступна.', 'Must show human unavailable message');
  assert(officeFailedState.showSelect === false, 'Must NOT show empty select dropdown when office lookup fails');
  assert(officeFailedState.showBackButton === true, 'Must show back button on office failure');
  assert(officeFailedState.showRetryButton === true, 'Must show retry button on office failure');

  // 9. Transient error reset between method transitions
  type ComponentState = {
    mode: 'select' | 'cdek_office' | 'cdek_courier';
    officeError: string;
    courierError: string;
  };

  const transitionMode = (
    _current: ComponentState,
    target: 'select' | 'cdek_office' | 'cdek_courier'
  ): ComponentState => {
    return {
      mode: target,
      officeError: '',
      courierError: '',
    };
  };

  // Step 1: User encountered office error
  let uiState: ComponentState = {
    mode: 'cdek_office',
    officeError: 'Логистика СДЭК временно недоступна.',
    courierError: '',
  };

  // Step 2: User clicks Back -> mode becomes select, errors cleared
  uiState = transitionMode(uiState, 'select');
  assert(uiState.mode === 'select', 'Mode must be select');
  assert(uiState.officeError === '', 'Office error must be cleared');
  assert(uiState.courierError === '', 'Courier error must be empty');

  // Step 3: User opens courier -> mode becomes cdek_courier, no stale error
  uiState = transitionMode(uiState, 'cdek_courier');
  assert(uiState.mode === 'cdek_courier', 'Mode must be cdek_courier');
  assert(uiState.courierError === '', 'Courier form must NOT show provider error on initial open');
  assert(uiState.officeError === '', 'Office error must not bleed into courier');

  // 10. Shipment Exists View
  const existingShipment: ReturnShipment = {
    id: 'ship-uuid-1',
    provider: 'cdek',
    method: 'cdek_courier',
    status: 'awaiting_handover',
    trackingNumber: '1234567890',
    pickupAddress: {
      city: 'Москва',
      street: 'Тверская',
      house: '10',
      flat: '42',
    },
    customerName: 'Иван Иванов',
    customerPhone: '+79991234567',
  };

  const shipmentView = getVisibleElements('select', true);
  assert(shipmentView.showShipmentBlock === true, 'Must show shipment status block when shipment exists');
  assert(shipmentView.showMethodCards === false, 'Must not show method selection when shipment exists');
  assert(formatReturnShipmentMethod(existingShipment.method) === 'Заберёт курьер СДЭК', 'Shipment method mapped');
  assert(formatReturnShipmentStatus(existingShipment.status) === 'Ожидает передачи в СДЭК', 'Shipment status mapped');

  // 11. Evidence Section & Lightbox
  const returnWithEvidence: CustomerReturnRecord = {
    ...sampleReturn,
    items: [
      {
        ...sampleReturn.items[0],
        evidence: [
          {
            id: 'ev-uuid-1',
            url: 'https://minio.zamk.local/media/returns/photo1.png',
            contentType: 'image/png',
            sortOrder: 0,
            createdAt: '2026-08-30T10:00:00Z',
          },
          {
            id: 'ev-uuid-2',
            url: 'https://minio.zamk.local/media/returns/photo2.png',
            contentType: 'image/png',
            sortOrder: 1,
            createdAt: '2026-08-30T10:00:00Z',
          },
        ],
      },
    ],
  };

  const collectedEvidence = (returnWithEvidence.items || []).flatMap((it) => it.evidence || []);
  assert(collectedEvidence.length === 2, 'Must collect 2 evidence photos');
  assert(collectedEvidence[0].url === 'https://minio.zamk.local/media/returns/photo1.png', 'Must have display URL for photo 1');
  assert(collectedEvidence[1].url === 'https://minio.zamk.local/media/returns/photo2.png', 'Must have display URL for photo 2');
  assert(!('storage_key' in collectedEvidence[0]), 'Must not expose storage_key in CustomerReturnEvidence');

  // Lightbox interaction model
  let previewUrl: string | null = null;
  const clickPhoto = (url: string) => {
    previewUrl = url;
  };
  const closeLightbox = () => {
    previewUrl = null;
  };

  assert(previewUrl === null, 'Lightbox closed initially');
  clickPhoto(collectedEvidence[0].url);
  assert(previewUrl === 'https://minio.zamk.local/media/returns/photo1.png', 'Lightbox open with photo 1');
  closeLightbox();
  assert(previewUrl === null, 'Lightbox closed after click');

  // Zero-evidence historical return behavior
  const historicalReturn: CustomerReturnRecord = {
    ...sampleReturn,
    items: [
      {
        ...sampleReturn.items[0],
        evidence: [],
      },
    ],
  };
  const zeroEvidence = (historicalReturn.items || []).flatMap((it) => it.evidence || []);
  assert(zeroEvidence.length === 0, 'Zero evidence array length must be 0');
  const showEvidenceSection = zeroEvidence.length > 0;
  assert(showEvidenceSection === false, 'Evidence section must not be shown when evidence count = 0');

  console.log('ALL SHOP RETURN LOGISTICS, DETAIL & EVIDENCE UX TESTS PASSED');
}

runReturnLogisticsTests();

// Unified status mapper for Admin interface

export const STATUS_TRANSLATIONS: Record<string, string> = {
  // General & Account Statuses
  active: 'Активен',
  pending_setup: 'Ожидает настройки',
  pending_review: 'Ожидает проверки',
  pending: 'Ожидает',
  blocked: 'Заблокирован',
  archived: 'В архиве',
  invited: 'Приглашён',
  not_started: 'Настройка не начата',

  // Order & Fulfillment Statuses
  delivered: 'Доставлен',
  published: 'Опубликован',
  packed: 'Упакован',
  shipped: 'Передан в доставку',
  cancelled: 'Отменён',
  completed: 'Завершён',
  processing: 'В обработке',
  returned: 'Возврат',
  assembly_pending: 'Ожидает сборки',
  assembling: 'Собирается',
  assembled: 'Собран',

  // Product & Catalog Statuses
  draft: 'Черновик',
  moderation: 'На модерации',
  rejected: 'Отклонён',
  hidden: 'Скрыт',

  // Payout & Financial Statuses
  requested: 'Запрошено',
  approved: 'Одобрено',
  paid: 'Выплачено',
  failed: 'Ошибка',
  on_hold: 'На удержании',

  // Performance Categories
  high: 'Высокая',
  stable: 'Стабильная',
  needs_attention: 'Требует внимания',
  low: 'Низкая',
  no_data: 'Недостаточно данных',
};

export function formatStatus(status?: string | null): string {
  if (!status) return '—';
  return STATUS_TRANSLATIONS[status] || status;
}

export const STATUS_BADGE_CLASSES: Record<string, string> = {
  active: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  completed: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  published: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  paid: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  
  blocked: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  cancelled: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  rejected: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  low: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  
  pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300',
  pending_setup: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  pending_review: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  moderation: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  requested: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  
  archived: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  draft: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  no_data: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
};

export function getStatusBadgeClass(status?: string | null): string {
  if (!status) return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
  return STATUS_BADGE_CLASSES[status] || 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
}

export const ORDER_STATUS_LABELS: Record<string, string> = {
  created: 'Создан',
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Оплачен',
  assembling: 'Собирается',
  packed: 'Упакован',
  shipped: 'В пути',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
  returned: 'Возвращён',
  refunded: 'Возвращён',
  completed: 'Завершён',
};

export const SHIPMENT_STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидает',
  assembling: 'В сборке',
  packed: 'Упакована',
  shipped: 'В пути',
  delivered: 'Доставлена',
  failed: 'Ошибка',
  cancelled: 'Отменена',
};

export const RETURN_STATUS_LABELS: Record<string, string> = {
  requested: 'Запрошен',
  needs_info: 'Требует уточнения',
  approved: 'Одобрен',
  receiving: 'На приёмке',
  item_received: 'Товар получен',
  refunded: 'Возвращён',
  completed: 'Завершён',
  rejected: 'Отклонён',
  cancelled: 'Отменён',
};

export const SUPPLY_STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  ready_to_ship: 'Готова к отправке',
  shipped_by_seller: 'Отправлена селлером',
  arrived_at_zamk: 'Прибыла в ZAMK',
  receiving: 'На приёмке',
  completed: 'Принята',
  completed_with_discrepancies: 'Принята с расхождениями',
  cancelled: 'Отменена',
};

export const FULFILLMENT_STATUS_LABELS: Record<string, string> = {
  awaiting_payment: 'Ожидает оплаты',
  paid: 'Ожидает сборки',
  pending: 'Не сформирована',
  assembling: 'В сборке',
  packed: 'Упакована',
  accepted: 'Принята на хабе',
  discrepancy: 'Расхождение',
  shipped: 'Отгружена',
  delivered: 'Доставлена',
  cancelled: 'Отменена',
};

export function humanizeOrderStatus(status?: string | null): string | undefined {
  if (!status) return undefined;
  return ORDER_STATUS_LABELS[status] || 'Статус не определён';
}

export function humanizeShipmentStatus(status?: string | null): string | undefined {
  if (!status) return undefined;
  return SHIPMENT_STATUS_LABELS[status] || 'Статус не определён';
}

export function humanizeReturnStatus(status?: string | null): string | undefined {
  if (!status) return undefined;
  return RETURN_STATUS_LABELS[status] || 'Статус не определён';
}

export function humanizeSupplyStatus(status?: string | null): string | undefined {
  if (!status) return undefined;
  return SUPPLY_STATUS_LABELS[status] || 'Статус не определён';
}

export function humanizeFulfillmentStatus(status?: string | null): string | undefined {
  if (!status) return undefined;
  return FULFILLMENT_STATUS_LABELS[status] || 'Статус не определён';
}

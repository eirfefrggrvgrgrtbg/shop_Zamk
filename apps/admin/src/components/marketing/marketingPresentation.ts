export const finite = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value);
export const number = (value: unknown) => finite(value) ? value.toLocaleString('ru-RU') : '—';
export const money = (value: unknown) => finite(value)
  ? new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(value / 100)
  : '—';
export const percent = (value: unknown) => finite(value) ? `${(value / 100).toFixed(2)}%` : '—';
export const conversion = (orders: unknown, visits: unknown) =>
  finite(orders) && finite(visits) ? (visits > 0 ? Math.trunc(orders / visits * 10000) : 0) : undefined;
export const date = (value?: string | null) => value && Number.isFinite(Date.parse(value))
  ? new Date(value).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit' }) : '—';

// Campaign dates are day boundaries in UTC: a period "from D to E" runs from D 00:00 UTC to E 23:59:59 UTC.
export const dayUtc = (value?: string | null) => value && Number.isFinite(Date.parse(value))
  ? new Date(value).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'UTC' }) : '—';
export const dateInputValue = (value?: string | null) => value && Number.isFinite(Date.parse(value))
  ? new Date(value).toISOString().slice(0, 10) : '';
export const startOfDayIso = (day: string) => `${day}T00:00:00.000Z`;
export const endOfDayIso = (day: string) => `${day}T23:59:59.000Z`;

/** Validates the planned budget text (whole rubles) and the period. Returns an error message or null. */
export function validateCampaignPlan(budgetRub: string, startDay: string, endDay: string): string | null {
  if (budgetRub.trim() !== '') {
    if (!/^\d+$/.test(budgetRub.trim())) return 'Плановый бюджет — целое число рублей';
    const rub = Number(budgetRub);
    if (rub <= 0) return 'Плановый бюджет должен быть больше нуля или оставьте поле пустым';
    if (rub > 1_000_000_000) return 'Плановый бюджет превышает допустимое значение';
  }
  if (startDay && endDay && endDay < startDay) return 'Дата окончания не может быть раньше даты начала';
  return null;
}

export const statusLabels: Record<string, string> = {
  draft: 'Черновик', submitted: 'На рассмотрении', counter_offered: 'Контрпредложение',
  approved: 'Одобрена', active: 'Активна', ended: 'Завершена', rejected: 'Отклонена', cancelled: 'Отменена',
};
export const channelLabels: Record<string, string> = {
  telegram: 'Telegram', vk: 'VK', instagram: 'Instagram', influencer: 'Блогер',
  email: 'Email', direct: 'Прямой трафик', search_ads: 'Поисковая реклама',
};
export const typeLabels: Record<string, string> = {
  influencer: 'Инфлюенсер', drop: 'Дроп / Запуск', seasonal_sale: 'Сезонная распродажа',
  brand_awareness: 'Узнаваемость бренда', retargeting: 'Ретаргетинг', special_promo: 'Специальная акция',
};
export const purposeLabels: Record<string, string> = {
  advertising: 'Рекламная кампания',
  promotion: 'Промо-кампания',
};

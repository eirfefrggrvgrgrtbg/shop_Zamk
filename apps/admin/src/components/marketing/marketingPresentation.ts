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

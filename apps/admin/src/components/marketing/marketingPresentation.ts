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

// Campaign periods (M1.3). The API stores absolute instants (RFC 3339) and treats the period as the
// half-open interval [startsAt, endsAt). The Admin UI picks calendar days in one fixed business time
// zone, Europe/Moscow (UTC+03:00, no DST): day D starts at D 00:00 MSK, and an inclusive end day E
// is sent as endsAt = (E + 1) 00:00 MSK. Browser-local time is never used for campaign dates.
export const CAMPAIGN_TIME_ZONE = 'Europe/Moscow';
export const CAMPAIGN_TZ_LABEL = 'МСК';
const MSK_OFFSET = '+03:00';

const mskFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: CAMPAIGN_TIME_ZONE, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
});
const mskParts = (ms: number) => {
  const parts: Record<string, string> = {};
  for (const p of mskFormatter.formatToParts(new Date(ms))) parts[p.type] = p.value;
  return parts;
};
const parseInstant = (value?: string | null) => (value && Number.isFinite(Date.parse(value)) ? Date.parse(value) : null);
const mskDayKey = (ms: number) => { const p = mskParts(ms); return `${p.year}-${p.month}-${p.day}`; };
const isMskMidnight = (ms: number) => { const p = mskParts(ms); return p.hour === '00' && p.minute === '00' && p.second === '00' && ms % 1000 === 0; };
const ruDay = (ms: number) => { const p = mskParts(ms); return `${p.day}.${p.month}.${p.year}`; };
const ruDayTime = (ms: number) => { const p = mskParts(ms); return `${p.day}.${p.month}.${p.year} ${p.hour}:${p.minute}`; };

/** Instant of D 00:00 MSK for a yyyy-mm-dd day. */
export const campaignDayStartIso = (day: string) => new Date(`${day}T00:00:00.000${MSK_OFFSET}`).toISOString();
/** Exclusive end for an inclusive last day E: (E + 1) 00:00 MSK. */
export const campaignDayEndExclusiveIso = (day: string) => {
  const d = new Date(`${day}T00:00:00.000${MSK_OFFSET}`);
  d.setUTCDate(d.getUTCDate() + 1);
  return d.toISOString();
};
/** yyyy-mm-dd (MSK) for a date input showing the start. */
export const campaignStartInputValue = (value?: string | null) => { const ms = parseInstant(value); return ms === null ? '' : mskDayKey(ms); };
/** yyyy-mm-dd (MSK) of the last instant inside the period, for a date input showing the end day. */
export const campaignEndInputValue = (value?: string | null) => { const ms = parseInstant(value); return ms === null ? '' : mskDayKey(ms - 1); };

/** Start as "dd.mm.yyyy" (or with time when it is not a day boundary). */
export const formatCampaignStart = (value?: string | null) => {
  const ms = parseInstant(value);
  if (ms === null) return '—';
  return isMskMidnight(ms) ? ruDay(ms) : ruDayTime(ms);
};
/** Inclusive end: the last day for a day-boundary end, otherwise "до dd.mm.yyyy HH:MM" of the exact instant. */
export const formatCampaignEnd = (value?: string | null) => {
  const ms = parseInstant(value);
  if (ms === null) return 'без окончания';
  return isMskMidnight(ms) ? ruDay(ms - 1) : `до ${ruDayTime(ms)}`;
};
export const formatCampaignPeriod = (startsAt?: string | null, endsAt?: string | null) => {
  if (!startsAt && !endsAt) return 'Бессрочно';
  return `${formatCampaignStart(startsAt)} — ${formatCampaignEnd(endsAt)} ${CAMPAIGN_TZ_LABEL}`;
};

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
  approved: 'Одобрена', active: 'Активна', paused: 'Приостановлена', ended: 'Завершена', rejected: 'Отклонена', cancelled: 'Отменена',
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

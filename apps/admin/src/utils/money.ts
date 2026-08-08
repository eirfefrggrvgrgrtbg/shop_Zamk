/**
 * Single source of truth for Money formatting across the Admin application.
 *
 * Rules:
 * - Backend DB stores amounts in CENTS (e.g. 1000000 cents = 10000.00 RUB).
 * - DTO objects may expose priceCents (cents) and price (rubles).
 * - formatMoneyCents formats amounts passed in CENTS.
 * - formatMoneyRubles formats amounts passed in RUBLES.
 */

export const formatMoneyCents = (cents?: number | null, currency = 'RUB'): string => {
  if (cents === undefined || cents === null || isNaN(cents)) {
    return '—';
  }
  const rubles = cents / 100;
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency,
    minimumFractionDigits: cents % 100 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(rubles);
};

export const formatMoneyRubles = (rubles?: number | null, currency = 'RUB'): string => {
  if (rubles === undefined || rubles === null || isNaN(rubles)) {
    return '—';
  }
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency,
    minimumFractionDigits: Number.isInteger(rubles) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(rubles);
};

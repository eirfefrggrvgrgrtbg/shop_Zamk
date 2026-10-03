import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
export type BasicProduct = { price: number; discountPrice?: number };

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatPrice(price: number): string {
  return price.toLocaleString('ru-RU') + ' ₽';
}

export function formatCents(cents: number): string {
  return formatPrice(cents / 100);
}

export function formatPercent(percent: number): string {
  const sign = percent > 0 ? '+' : '';
  return `${sign}${percent.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`;
}

export function getProductEffectivePrice(product: BasicProduct): number {
  return product.discountPrice ?? product.price;
}

/**
 * Parses a Russian Ruble amount string into integer cents (kopecks) exactly,
 * without using binary floating-point arithmetic.
 *
 * Examples:
 * - "1"      -> 100
 * - "1.01"   -> 101
 * - "10.10"  -> 1010
 * - "999.99" -> 99999
 * - "1000"   -> 100000
 *
 * Rejects:
 * - Invalid precision (> 2 decimal places, e.g. "1.001")
 * - Negative or zero amounts ("0", "-5")
 * - Non-numeric strings
 */
export function parseRubToCentsExact(val: string): number {
  const trimmed = val.trim();
  if (!trimmed) {
    throw new Error('Укажите сумму в рублях.');
  }

  // Check if string contains more than 2 decimal digits after dot or comma
  if (/^\d+[.,]\d{3,}$/.test(trimmed)) {
    throw new Error('Максимальная скидка не может содержать более 2 знаков после запятой.');
  }

  // Exact match for positive integer or decimal with 1-2 digits
  const match = trimmed.match(/^(\d+)(?:[.,](\d{1,2}))?$/);
  if (!match) {
    throw new Error('Максимальная скидка должна быть положительным числом.');
  }

  const rubPart = parseInt(match[1], 10);
  const fracStr = match[2] || '';
  const kopecksPart = fracStr.length === 1
    ? parseInt(fracStr, 10) * 10
    : fracStr.length === 2
    ? parseInt(fracStr, 10)
    : 0;

  const totalCents = rubPart * 100 + kopecksPart;
  if (totalCents <= 0) {
    throw new Error('Максимальная скидка должна быть положительным числом.');
  }

  return totalCents;
}

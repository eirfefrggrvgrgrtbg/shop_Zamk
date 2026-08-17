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

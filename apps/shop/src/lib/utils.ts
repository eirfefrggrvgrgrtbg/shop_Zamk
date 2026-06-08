import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import type { Product } from "../types/catalog"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatPrice(price: number): string {
  return price.toLocaleString('ru-RU') + ' ₽';
}

export function getProductEffectivePrice(product: Pick<Product, 'price' | 'discountPrice'>): number {
  return product.discountPrice ?? product.price;
}

import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { Product } from '../lib/mock-data';
import { getProductEffectivePrice } from '../lib/orders';

export interface CartItem {
  product: Product;
  quantity: number;
  selectedSize?: string;
  selectedColor?: string;
}

const CART_STORAGE_KEY = 'zamk_cart';

const normalizeVariantValue = (value?: string) => value?.trim() || undefined;

export function getCartItemKey(productId: string, size?: string, color?: string) {
  return [productId, normalizeVariantValue(size) ?? '', normalizeVariantValue(color) ?? ''].join('::');
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isCartItem(value: unknown): value is CartItem {
  if (!isObject(value) || !isObject(value.product)) {
    return false;
  }

  return typeof value.product.id === 'string' && typeof value.quantity === 'number';
}

function readStoredCartItems(): CartItem[] {
  if (typeof window === 'undefined') {
    return [];
  }

  try {
    const raw = window.localStorage.getItem(CART_STORAGE_KEY);
    if (!raw) {
      return [];
    }

    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed.filter(isCartItem).map((item) => ({
      ...item,
      selectedSize: normalizeVariantValue(item.selectedSize),
      selectedColor: normalizeVariantValue(item.selectedColor),
    }));
  } catch {
    return [];
  }
}

function matchesCartItem(item: CartItem, productId: string, size?: string, color?: string) {
  if (item.product.id !== productId) {
    return false;
  }

  if (size !== undefined && normalizeVariantValue(item.selectedSize) !== normalizeVariantValue(size)) {
    return false;
  }

  if (color !== undefined && normalizeVariantValue(item.selectedColor) !== normalizeVariantValue(color)) {
    return false;
  }

  return true;
}

interface CartContextType {
  items: CartItem[];
  addItem: (product: Product, size?: string, color?: string) => void;
  removeItem: (productId: string, size?: string, color?: string) => void;
  updateQuantity: (productId: string, quantity: number, size?: string, color?: string) => void;
  clearCart: () => void;
  totalItems: number;
  totalPrice: number;
}

const CartContext = createContext<CartContextType | undefined>(undefined);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>(readStoredCartItems);

  useEffect(() => {
    try {
      window.localStorage.setItem(CART_STORAGE_KEY, JSON.stringify(items));
    } catch {
      // Ignore storage errors and keep the in-memory cart working.
    }
  }, [items]);

  const addItem = useCallback((product: Product, size?: string, color?: string) => {
    const normalizedSize = normalizeVariantValue(size);
    const normalizedColor = normalizeVariantValue(color);
    const targetKey = getCartItemKey(product.id, normalizedSize, normalizedColor);

    setItems(prev => {
      const existing = prev.find(i => getCartItemKey(i.product.id, i.selectedSize, i.selectedColor) === targetKey);
      if (existing) {
        return prev.map(i =>
          getCartItemKey(i.product.id, i.selectedSize, i.selectedColor) === targetKey
            ? { ...i, quantity: i.quantity + 1 }
            : i
        );
      }
      return [...prev, { product, quantity: 1, selectedSize: normalizedSize, selectedColor: normalizedColor }];
    });
  }, []);

  const removeItem = useCallback((productId: string, size?: string, color?: string) => {
    setItems(prev => prev.filter(item => !matchesCartItem(item, productId, size, color)));
  }, []);

  const updateQuantity = useCallback((productId: string, quantity: number, size?: string, color?: string) => {
    if (quantity <= 0) {
      setItems(prev => prev.filter(item => !matchesCartItem(item, productId, size, color)));
      return;
    }
    setItems(prev =>
      prev.map(i =>
        matchesCartItem(i, productId, size, color) ? { ...i, quantity } : i
      )
    );
  }, []);

  const clearCart = useCallback(() => setItems([]), []);

  const totalItems = items.reduce((sum, i) => sum + i.quantity, 0);
  const totalPrice = items.reduce((sum, i) => sum + getProductEffectivePrice(i.product) * i.quantity, 0);

  return (
    <CartContext.Provider value={{ items, addItem, removeItem, updateQuantity, clearCart, totalItems, totalPrice }}>
      {children}
    </CartContext.Provider>
  );
}

export function useCart() {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error('useCart must be used within CartProvider');
  return ctx;
}

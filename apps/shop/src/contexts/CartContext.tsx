import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { Product } from '../types/catalog';
import { useAuth } from './AuthContext';
import { getCart, addToCart, updateCartItem as apiUpdateCartItem, removeFromCart as apiRemoveFromCart, clearCart as apiClearCart } from '@zamk/api-client/src/customer';
import type { CartItem as ApiCartItem, Cart as ApiCart } from '@zamk/api-client/src/types';

export interface UIContextCartItem {
  id: string; // The cart item ID
  productId: string;
  productVariantId: string;
  product?: Product; // Local mock mapping or partial data
  quantity: number;
  title?: string;
  price?: number;
  inStock?: boolean;
}

interface CartContextType {
  items: UIContextCartItem[];
  addItem: (productId: string, productVariantId: string, quantity?: number) => Promise<void>;
  removeItem: (itemId: string) => Promise<void>;
  updateQuantity: (itemId: string, quantity: number) => Promise<void>;
  clearCart: () => Promise<void>;
  totalItems: number;
  totalPrice: number;
  isLoadingCart: boolean;
}

const CartContext = createContext<CartContextType | undefined>(undefined);

export function CartProvider({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, openAuthModal, isInitializing } = useAuth();
  const [items, setItems] = useState<UIContextCartItem[]>([]);
  const [totalPrice, setTotalPrice] = useState(0);
  const [isLoadingCart, setIsLoadingCart] = useState(true);

  const fetchCart = useCallback(async () => {
    if (!isAuthenticated) {
      setItems([]);
      setTotalPrice(0);
      setIsLoadingCart(false);
      return;
    }

    try {
      setIsLoadingCart(true);
      const cart = await getCart();
      
      const mappedItems = cart.items.map(item => ({
        id: item.id,
        productId: item.productId,
        productVariantId: item.productVariantId,
        quantity: item.quantity,
        title: item.title,
        price: item.priceCents ? item.priceCents / 100 : 0,
        inStock: item.inStock,
        product: item.product ? {
          id: item.product.id,
          name: item.product.title,
          price: item.product.priceCents / 100,
          image: item.product.mainImageUrl || '',
          brand: 'Бренд не указан',
          category: 'Категория не указана'
        } as Product : undefined
      }));

      // Backend does not return totalPriceCents — compute from items to stay consistent
      const computedTotal = cart.items.reduce(
        (sum, item) => sum + (item.priceCents ?? 0) * item.quantity,
        0
      );

      setItems(mappedItems);
      setTotalPrice(computedTotal / 100);
    } catch (e) {
      console.error('Failed to load cart', e);
    } finally {
      setIsLoadingCart(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    if (!isInitializing) {
      fetchCart();
    }
  }, [isAuthenticated, isInitializing, fetchCart]);


  const addItem = useCallback(async (productId: string, productVariantId: string, quantity = 1) => {
    if (!isAuthenticated) {
      openAuthModal('login');
      return;
    }

    try {
      await addToCart({ productId, productVariantId, quantity });
      await fetchCart();
    } catch (e: any) {
      throw new Error(e.message || 'Ошибка при добавлении в корзину');
    }
  }, [isAuthenticated, openAuthModal, fetchCart]);

  const removeItem = useCallback(async (itemId: string) => {
    try {
      await apiRemoveFromCart(itemId);
      await fetchCart();
    } catch (e) {
      console.error(e);
    }
  }, [fetchCart]);

  const updateQuantity = useCallback(async (itemId: string, quantity: number) => {
    if (quantity <= 0) {
      await removeItem(itemId);
      return;
    }
    try {
      await apiUpdateCartItem(itemId, quantity);
      await fetchCart();
    } catch (e) {
      console.error(e);
    }
  }, [fetchCart, removeItem]);

  const clearCart = useCallback(async () => {
    try {
      await apiClearCart();
      await fetchCart();
    } catch (e) {
      console.error(e);
    }
  }, [fetchCart]);

  const totalItems = items.reduce((sum, i) => sum + i.quantity, 0);

  return (
    <CartContext.Provider value={{ items, addItem, removeItem, updateQuantity, clearCart, totalItems, totalPrice, isLoadingCart }}>
      {children}
    </CartContext.Provider>
  );
}

export function useCart() {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error('useCart must be used within CartProvider');
  return ctx;
}

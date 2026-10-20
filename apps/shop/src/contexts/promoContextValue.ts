import { createContext, useContext } from 'react';

export interface PromoContextType {
  /** Code the customer wants to use (sent to CreateOrder). Null when none. */
  appliedCode: string | null;
  /** Discount in roubles, only when the server confirmed it for the CURRENT cart; otherwise 0. */
  discount: number;
  /** A server check for the current cart is in flight (discount is hidden meanwhile). */
  isChecking: boolean;
  /** A code is selected but the server has not confirmed it for the current cart. */
  isUnconfirmed: boolean;
  error: string | null;
  apply: (rawCode: string) => Promise<boolean>;
  remove: () => void;
  retry: () => void;
  /** Called by checkout when CreateOrder rejected the promo: drops it and shows the reason. */
  rejectAfterCheckout: (error: unknown) => void;
  clear: () => void;
}

export const PromoContext = createContext<PromoContextType | undefined>(undefined);

export function usePromo() {
  const ctx = useContext(PromoContext);
  if (!ctx) throw new Error('usePromo must be used within PromoProvider');
  return ctx;
}

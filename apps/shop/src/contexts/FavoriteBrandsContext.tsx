import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import { getFavoriteBrands, addFavoriteBrand, removeFavoriteBrand } from '@zamk/api-client/src/customer';
import type { FavoriteBrand } from '@zamk/api-client/src/types';
import { setAuthReturnPath } from '../components/account/CustomerProtectedRoute';
import { useAuth } from './AuthContext';

// Favorite brands are deliberately separate from FavoritesContext (favorite products).
interface FavoriteBrandsContextType {
  brands: FavoriteBrand[];
  isLoading: boolean;
  error: string | null;
  pendingIds: string[];
  isFavoriteBrand: (brandId: string) => boolean;
  /** Resolves true on success, false when the user was sent to login or the API failed. */
  toggleFavoriteBrand: (brandId: string) => Promise<boolean>;
  reloadFavoriteBrands: () => Promise<void>;
}

const FavoriteBrandsContext = createContext<FavoriteBrandsContextType | undefined>(undefined);

export function FavoriteBrandsProvider({ children }: { children: React.ReactNode }) {
  const { user, openAuthModal } = useAuth();
  const [brands, setBrands] = useState<FavoriteBrand[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendingIds, setPendingIds] = useState<string[]>([]);

  const load = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      setBrands(await getFavoriteBrands());
    } catch (err) {
      console.error('Failed to load favorite brands', err);
      setError('Не удалось загрузить любимые бренды.');
      setBrands([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user) {
      load();
    } else {
      setBrands([]);
      setError(null);
      setIsLoading(false);
    }
  }, [user, load]);

  const isFavoriteBrand = useCallback((brandId: string) => brands.some((b) => b.id === brandId), [brands]);

  const toggleFavoriteBrand = useCallback(async (brandId: string) => {
    if (!user) {
      setAuthReturnPath(window.location.pathname + window.location.search);
      openAuthModal('login');
      return false;
    }
    if (pendingIds.includes(brandId)) return false;

    const wasFavorite = brands.some((b) => b.id === brandId);
    setPendingIds((prev) => [...prev, brandId]);
    try {
      if (wasFavorite) {
        await removeFavoriteBrand(brandId);
        setBrands((prev) => prev.filter((b) => b.id !== brandId));
      } else {
        await addFavoriteBrand(brandId);
        setBrands(await getFavoriteBrands());
      }
      return true;
    } catch (err) {
      console.error('Failed to toggle favorite brand', err);
      return false;
    } finally {
      setPendingIds((prev) => prev.filter((id) => id !== brandId));
    }
  }, [user, brands, pendingIds, openAuthModal]);

  return (
    <FavoriteBrandsContext.Provider
      value={{ brands, isLoading, error, pendingIds, isFavoriteBrand, toggleFavoriteBrand, reloadFavoriteBrands: load }}
    >
      {children}
    </FavoriteBrandsContext.Provider>
  );
}

export function useFavoriteBrands() {
  const ctx = useContext(FavoriteBrandsContext);
  if (!ctx) throw new Error('useFavoriteBrands must be used within FavoriteBrandsProvider');
  return ctx;
}

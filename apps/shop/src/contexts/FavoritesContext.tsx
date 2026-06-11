import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import { getFavorites, addFavorite, removeFavorite } from '@zamk/api-client/src/customer';
import { mapFavoritesToCatalog } from '../api/publicCatalog';
import { setAuthReturnPath } from '../components/account/CustomerProtectedRoute';
import { useAuth } from './AuthContext';
import type { Product } from '../types/catalog';

interface FavoritesContextType {
  favorites: Product[];
  favoriteIds: string[];
  toggleFavorite: (productId: string) => Promise<void>;
  isFavorite: (productId: string) => boolean;
  clearFavorites: () => void;
  isLoading: boolean;
  error: string | null;
  reloadFavorites: () => Promise<void>;
}

const FavoritesContext = createContext<FavoritesContextType | undefined>(undefined);

export function FavoritesProvider({ children }: { children: React.ReactNode }) {
  const { user, openAuthModal } = useAuth();
  const [favorites, setFavorites] = useState<Product[]>([]);
  const [favoriteIds, setFavoriteIds] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadFavorites = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getFavorites();
      const mapped = await mapFavoritesToCatalog(data || []);
      setFavorites(mapped);
      setFavoriteIds(mapped.map((p) => p.id));
    } catch (err) {
      console.error('Failed to load favorites', err);
      setError('Не удалось загрузить избранное.');
      setFavorites([]);
      setFavoriteIds([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user) {
      loadFavorites();
    } else {
      setFavorites([]);
      setFavoriteIds([]);
      setError(null);
      setIsLoading(false);
    }
  }, [user, loadFavorites]);

  const toggleFavorite = useCallback(async (productId: string) => {
    if (!user) {
      setAuthReturnPath(window.location.pathname + window.location.search);
      openAuthModal('login');
      return;
    }

    const isFav = favoriteIds.includes(productId);

    setFavoriteIds((prev) =>
      isFav ? prev.filter((id) => id !== productId) : [...prev, productId]
    );
    if (isFav) {
      setFavorites((prev) => prev.filter((p) => p.id !== productId));
    }

    try {
      if (isFav) {
        await removeFavorite(productId);
      } else {
        await addFavorite(productId);
        await loadFavorites();
      }
    } catch (err) {
      console.error('Failed to toggle favorite', err);
      await loadFavorites();
    }
  }, [user, favoriteIds, openAuthModal, loadFavorites]);

  const isFavorite = useCallback((productId: string) => {
    return favoriteIds.includes(productId);
  }, [favoriteIds]);

  const clearFavorites = useCallback(() => {
    setFavorites([]);
    setFavoriteIds([]);
    setError(null);
  }, []);

  return (
    <FavoritesContext.Provider
      value={{
        favorites,
        favoriteIds,
        toggleFavorite,
        isFavorite,
        clearFavorites,
        isLoading,
        error,
        reloadFavorites: loadFavorites,
      }}
    >
      {children}
    </FavoritesContext.Provider>
  );
}

export function useFavorites() {
  const ctx = useContext(FavoritesContext);
  if (!ctx) throw new Error('useFavorites must be used within FavoritesProvider');
  return ctx;
}

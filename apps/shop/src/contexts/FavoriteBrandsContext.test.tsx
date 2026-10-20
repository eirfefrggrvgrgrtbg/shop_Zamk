/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import React from 'react';
import { renderHook, act, waitFor } from '@testing-library/react';
import { FavoriteBrandsProvider, useFavoriteBrands } from './FavoriteBrandsContext';

const h = vi.hoisted(() => ({
  user: { id: 'u-1' } as { id: string } | null,
  openAuthModal: vi.fn(),
  getFavoriteBrands: vi.fn(),
  addFavoriteBrand: vi.fn(),
  removeFavoriteBrand: vi.fn(),
  setAuthReturnPath: vi.fn(),
}));
vi.mock('./AuthContext', () => ({ useAuth: () => ({ user: h.user, openAuthModal: h.openAuthModal }) }));
vi.mock('@zamk/api-client/src/customer', () => ({
  getFavoriteBrands: h.getFavoriteBrands,
  addFavoriteBrand: h.addFavoriteBrand,
  removeFavoriteBrand: h.removeFavoriteBrand,
}));
vi.mock('../components/account/CustomerProtectedRoute', () => ({ setAuthReturnPath: h.setAuthReturnPath }));

const wrapper = ({ children }: { children: React.ReactNode }) => <FavoriteBrandsProvider>{children}</FavoriteBrandsProvider>;
const b1 = { id: 'b-1', name: 'One', slug: 'one', addedAt: 'x' };

describe('FavoriteBrandsContext (MARKETING.M1.11)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.user = { id: 'u-1' };
    h.getFavoriteBrands.mockResolvedValue([]);
    h.addFavoriteBrand.mockResolvedValue({});
    h.removeFavoriteBrand.mockResolvedValue({});
  });

  it('anonymous toggle opens login with return path and calls no API', async () => {
    h.user = null;
    window.history.pushState({}, '', '/brand/one');
    const { result } = renderHook(() => useFavoriteBrands(), { wrapper });
    let ok = true;
    await act(async () => { ok = await result.current.toggleFavoriteBrand('b-1'); });
    expect(ok).toBe(false);
    expect(h.setAuthReturnPath).toHaveBeenCalledWith('/brand/one');
    expect(h.openAuthModal).toHaveBeenCalledWith('login');
    expect(h.getFavoriteBrands).not.toHaveBeenCalled();
    expect(h.addFavoriteBrand).not.toHaveBeenCalled();
  });

  it('adds then removes, keeping state in sync', async () => {
    const { result } = renderHook(() => useFavoriteBrands(), { wrapper });
    await waitFor(() => expect(h.getFavoriteBrands).toHaveBeenCalledTimes(1));
    h.getFavoriteBrands.mockResolvedValue([b1]);
    await act(async () => { await result.current.toggleFavoriteBrand('b-1'); });
    expect(h.addFavoriteBrand).toHaveBeenCalledWith('b-1');
    expect(result.current.isFavoriteBrand('b-1')).toBe(true);
    await act(async () => { await result.current.toggleFavoriteBrand('b-1'); });
    expect(h.removeFavoriteBrand).toHaveBeenCalledWith('b-1');
    expect(result.current.isFavoriteBrand('b-1')).toBe(false);
  });

  it('reloads saved brands after a page reload (fresh provider)', async () => {
    h.getFavoriteBrands.mockResolvedValue([b1]);
    const { result } = renderHook(() => useFavoriteBrands(), { wrapper });
    await waitFor(() => expect(result.current.isFavoriteBrand('b-1')).toBe(true));
  });

  it('returns false and leaves state unchanged when the API fails', async () => {
    const { result } = renderHook(() => useFavoriteBrands(), { wrapper });
    await waitFor(() => expect(h.getFavoriteBrands).toHaveBeenCalled());
    h.addFavoriteBrand.mockRejectedValue(new Error('boom'));
    let ok = true;
    await act(async () => { ok = await result.current.toggleFavoriteBrand('b-1'); });
    expect(ok).toBe(false);
    expect(result.current.isFavoriteBrand('b-1')).toBe(false);
    expect(result.current.pendingIds).toEqual([]);
  });

  it('surfaces a list error', async () => {
    h.getFavoriteBrands.mockRejectedValue(new Error('down'));
    const { result } = renderHook(() => useFavoriteBrands(), { wrapper });
    await waitFor(() => expect(result.current.error).toMatch(/Не удалось/));
  });
});

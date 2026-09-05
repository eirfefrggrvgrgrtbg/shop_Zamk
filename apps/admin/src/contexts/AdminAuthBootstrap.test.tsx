/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { AdminAuthProvider } from './AdminAuthContext';
import { AdminProtectedRoute } from '../components/AdminProtectedRoute';
import * as apiAuth from '@zamk/api-client/src/auth';
import * as apiAdmin from '@zamk/api-client/src/admin';
import * as clientModule from '@zamk/api-client/src/client';
import * as tokenStore from '@zamk/api-client/src/tokenStore';

function LocationDisplay() {
  const location = useLocation();
  return (
    <div>
      <div data-testid="location-path">{location.pathname}</div>
      <div data-testid="location-from">{(location.state as any)?.from?.pathname || 'none'}</div>
    </div>
  );
}

function TestApp({ initialPath = '/orders/receiving' }: { initialPath?: string }) {
  return (
    <AdminAuthProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<div><h1>Login Page</h1><LocationDisplay /></div>} />
          <Route
            path="/orders/receiving"
            element={
              <AdminProtectedRoute permission="orders.read">
                <div>
                  <h1>Protected Receiving Page</h1>
                  <LocationDisplay />
                </div>
              </AdminProtectedRoute>
            }
          />
          <Route
            path="/dashboard"
            element={
              <AdminProtectedRoute>
                <div>
                  <h1>Protected Dashboard</h1>
                  <LocationDisplay />
                </div>
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    </AdminAuthProvider>
  );
}

describe('Admin Auth Bootstrap & Deep-Link Invariants', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    tokenStore.clearAccessToken();
  });

  afterEach(() => {
    cleanup();
  });

  it('authenticated persisted session: protected route does not redirect during bootstrap', async () => {
    let resolveRefresh: (val: any) => void;
    const refreshPromise = new Promise<any>((resolve) => {
      resolveRefresh = resolve;
    });

    vi.spyOn(apiAuth, 'refresh').mockReturnValue(refreshPromise);
    vi.spyOn(apiAdmin, 'getAdminMe').mockResolvedValue({
      user: { id: 'admin-1', role: 'admin', email: 'admin@zamk.local' },
      staff: {
        roleCode: 'owner',
        roleName: 'Владелец',
        status: 'active',
        permissions: ['orders.read', 'warehouse.receiving'],
      },
    } as any);

    render(<TestApp initialPath="/orders/receiving" />);

    // 1. During bootstrap, shows loading spinner and does NOT redirect to /login
    expect(screen.queryByText('Login Page')).toBeNull();
    expect(screen.queryByText('Protected Receiving Page')).toBeNull();

    // 2. Resolve refresh
    resolveRefresh!({
      accessToken: 'test-valid-access-token',
      user: { id: 'admin-1', role: 'admin', email: 'admin@zamk.local' },
    });

    // 3. Protected route renders successfully
    await waitFor(() => {
      expect(screen.getByText('Protected Receiving Page')).toBeDefined();
    });

    expect(screen.getByTestId('location-path').textContent).toBe('/orders/receiving');
    expect(screen.queryByText('Login Page')).toBeNull();
    expect(tokenStore.getAccessToken()).toBe('test-valid-access-token');
  });

  it('unauthenticated session: protected route redirects to login with return path preserved', async () => {
    vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('Missing refresh token'));

    render(<TestApp initialPath="/orders/receiving" />);

    // Wait for redirect to /login
    await waitFor(() => {
      expect(screen.getByText('Login Page')).toBeDefined();
    });

    expect(screen.getByTestId('location-path').textContent).toBe('/login');
    expect(screen.getByTestId('location-from').textContent).toBe('/orders/receiving');
    expect(tokenStore.getAccessToken()).toBeNull();
  });

  it('expired/invalid session: non-admin or rejected auth redirects to login', async () => {
    vi.spyOn(apiAuth, 'refresh').mockResolvedValue({
      accessToken: 'test-customer-token',
      user: { id: 'user-2', role: 'customer', email: 'cust@zamk.local' },
    } as any);

    render(<TestApp initialPath="/dashboard" />);

    await waitFor(() => {
      expect(screen.getByText('Login Page')).toBeDefined();
    });

    expect(screen.getByTestId('location-path').textContent).toBe('/login');
    expect(tokenStore.getAccessToken()).toBeNull();
  });

  it('concurrent refresh requests are deduplicated in api-client auth', async () => {
    const requestSpy = vi.spyOn(clientModule, 'request').mockImplementation(async () => {
      await new Promise((r) => setTimeout(r, 20));
      return {
        accessToken: 'shared-access-token',
        user: { id: 'admin-1', role: 'admin' },
      };
    });

    // Fire two calls concurrently
    const [res1, res2] = await Promise.all([
      apiAuth.refresh(),
      apiAuth.refresh(),
    ]);

    expect(res1.accessToken).toBe('shared-access-token');
    expect(res2.accessToken).toBe('shared-access-token');
    // Crucial: Only ONE network call was dispatched to /auth/refresh
    expect(requestSpy).toHaveBeenCalledTimes(1);

    requestSpy.mockRestore();
  });
});

/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { AuthProvider } from '../contexts/AuthContext';
import { SellerProtectedRoute } from '../components/SellerProtectedRoute';
import { SellerLogin } from './SellerLogin';
import { getSafeReturnPath } from '../lib/authRedirect';
import * as apiAuth from '@zamk/api-client/src/auth';

function LocationProbe() {
  const location = useLocation();
  return (
    <div>
      <div data-testid="current-pathname">{location.pathname}</div>
      <div data-testid="current-search">{location.search}</div>
      <div data-testid="current-hash">{location.hash}</div>
      <div data-testid="location-from-pathname">{(location.state as any)?.from?.pathname || 'none'}</div>
      <div data-testid="location-from-full">
        {(location.state as any)?.from
          ? `${(location.state as any).from.pathname}${(location.state as any).from.search || ''}${(location.state as any).from.hash || ''}`
          : 'none'}
      </div>
    </div>
  );
}

function TestSellerApp({ initialEntries = ['/returns/583fb821-2b10-4966-aacd-e8d24a215842'] }: { initialEntries?: string[] }) {
  return (
    <AuthProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <LocationProbe />
        <Routes>
          <Route
            path="/login"
            element={<SellerLogin />}
          />
          <Route
            path="/returns"
            element={
              <SellerProtectedRoute>
                <div>
                  <h1>Protected Returns List</h1>
                </div>
              </SellerProtectedRoute>
            }
          />
          <Route
            path="/returns/:id"
            element={
              <SellerProtectedRoute>
                <div>
                  <h1>Protected Return Detail</h1>
                </div>
              </SellerProtectedRoute>
            }
          />
          <Route
            path="/dashboard"
            element={
              <SellerProtectedRoute>
                <div>
                  <h1>Protected Dashboard</h1>
                </div>
              </SellerProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    </AuthProvider>
  );
}

describe('Seller URL-Preserving Auth Gate & Deep-Link Invariants', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe('getSafeReturnPath unit validation', () => {
    it('accepts safe internal relative paths', () => {
      expect(getSafeReturnPath('/returns/583fb821-2b10-4966-aacd-e8d24a215842')).toBe(
        '/returns/583fb821-2b10-4966-aacd-e8d24a215842'
      );
      expect(getSafeReturnPath('/supplies/123?tab=items#section')).toBe('/supplies/123?tab=items#section');
      expect(getSafeReturnPath('/')).toBe('/');
    });

    it('extracts path from React Router Location-like objects', () => {
      expect(
        getSafeReturnPath({
          pathname: '/returns/583fb821-2b10-4966-aacd-e8d24a215842',
          search: '?status=received',
          hash: '#box-1',
        })
      ).toBe('/returns/583fb821-2b10-4966-aacd-e8d24a215842?status=received#box-1');
    });

    it('falls back to defaultPath when input is empty or null', () => {
      expect(getSafeReturnPath(null)).toBe('/dashboard');
      expect(getSafeReturnPath(undefined)).toBe('/dashboard');
      expect(getSafeReturnPath('')).toBe('/dashboard');
      expect(getSafeReturnPath({}, '/custom')).toBe('/custom');
    });

    it('blocks open-redirect attempts and protocol injection', () => {
      expect(getSafeReturnPath('//evil.com')).toBe('/dashboard');
      expect(getSafeReturnPath('//evil.com/path')).toBe('/dashboard');
      expect(getSafeReturnPath('/\\evil.com')).toBe('/dashboard');
      expect(getSafeReturnPath('https://evil.com')).toBe('/dashboard');
      expect(getSafeReturnPath('http://evil.com')).toBe('/dashboard');
      expect(getSafeReturnPath('javascript:alert(1)')).toBe('/dashboard');
      expect(getSafeReturnPath('/path:evil')).toBe('/dashboard');
      expect(getSafeReturnPath('/path\newil')).toBe('/dashboard');
      expect(getSafeReturnPath('/path\\evil')).toBe('/dashboard');
    });
  });

  describe('URL-Preserving Auth Gate Integration Flows', () => {
    it('initialization state: shows loader and does not render premature login or protected content', async () => {
      let resolveRefresh: (val: any) => void;
      const refreshPromise = new Promise((resolve) => {
        resolveRefresh = resolve;
      });

      vi.spyOn(apiAuth, 'refresh').mockReturnValue(refreshPromise as any);
      vi.spyOn(apiAuth, 'me').mockRejectedValue(new Error('unauthorized'));

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842?tab=items#review';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      // During bootstrap, protected route shows loader, does not show auth gate or protected detail
      expect(screen.queryByTestId('seller-auth-gate')).toBeNull();
      expect(screen.queryByText('Protected Return Detail')).toBeNull();

      // Complete failed auth init
      resolveRefresh!({ user: null });

      // After init completes, auth gate renders IN-PLACE
      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      // Crucial: Address bar path, search, and hash remain EXACTLY as requested
      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns/583fb821-2b10-4966-aacd-e8d24a215842');
      expect(screen.getByTestId('current-search').textContent).toBe('?tab=items');
      expect(screen.getByTestId('current-hash').textContent).toBe('#review');
      // Protected content still NOT rendered
      expect(screen.queryByText('Protected Return Detail')).toBeNull();
    });

    it('unauthenticated /returns/{id}: URL remains in address bar, auth gate shown, login renders detail in place', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('no session'));
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);
      vi.spyOn(apiAuth, 'login').mockResolvedValue({
        accessToken: 'seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842?filter=active#section';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      // 1. In-place auth gate rendered
      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns/583fb821-2b10-4966-aacd-e8d24a215842');
      expect(screen.getByTestId('current-search').textContent).toBe('?filter=active');
      expect(screen.getByTestId('current-hash').textContent).toBe('#section');
      expect(screen.queryByText('Protected Return Detail')).toBeNull();

      // 2. Perform login
      fireEvent.change(screen.getByLabelText(/Email/i, { selector: 'input' }), {
        target: { value: 'seller@zamk.local' },
      });
      fireEvent.change(screen.getByLabelText(/Пароль/i, { selector: 'input' }), {
        target: { value: 'password123' },
      });
      fireEvent.click(screen.getByRole('button', { name: /Войти/i }));

      // 3. Protected detail renders in place at the EXACT same URL with search and hash
      await waitFor(() => {
        expect(screen.getByText('Protected Return Detail')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns/583fb821-2b10-4966-aacd-e8d24a215842');
      expect(screen.getByTestId('current-search').textContent).toBe('?filter=active');
      expect(screen.getByTestId('current-hash').textContent).toBe('#section');
      expect(screen.queryByTestId('seller-auth-gate')).toBeNull();
    });

    it('unauthenticated /returns: URL remains in address bar, auth gate shown, login renders list in place', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('no session'));
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);
      vi.spyOn(apiAuth, 'login').mockResolvedValue({
        accessToken: 'seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      render(<TestSellerApp initialEntries={['/returns']} />);

      // Auth gate rendered in place at /returns
      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns');
      expect(screen.queryByText('Protected Returns List')).toBeNull();

      // Log in
      fireEvent.change(screen.getByLabelText(/Email/i, { selector: 'input' }), {
        target: { value: 'seller@zamk.local' },
      });
      fireEvent.change(screen.getByLabelText(/Пароль/i, { selector: 'input' }), {
        target: { value: 'password123' },
      });
      fireEvent.click(screen.getByRole('button', { name: /Войти/i }));

      // Renders returns list in place at /returns
      await waitFor(() => {
        expect(screen.getByText('Protected Returns List')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns');
    });

    it('already-authenticated seller navigating to deep link renders directly without auth gate', async () => {
      vi.spyOn(apiAuth, 'refresh').mockResolvedValue({ accessToken: 'valid' } as any);
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      await waitFor(() => {
        expect(screen.getByText('Protected Return Detail')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe(targetPath);
      expect(screen.queryByTestId('seller-auth-gate')).toBeNull();
    });

    it('explicit /login: unauthenticated visit shows login form and defaults to /dashboard on success', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('no session'));
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);
      vi.spyOn(apiAuth, 'login').mockResolvedValue({
        accessToken: 'seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      render(<TestSellerApp initialEntries={['/login']} />);

      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/login');

      fireEvent.change(screen.getByLabelText(/Email/i, { selector: 'input' }), {
        target: { value: 'seller@zamk.local' },
      });
      fireEvent.change(screen.getByLabelText(/Пароль/i, { selector: 'input' }), {
        target: { value: 'password123' },
      });
      fireEvent.click(screen.getByRole('button', { name: /Войти/i }));

      // Navigates to /dashboard
      await waitFor(() => {
        expect(screen.getByText('Protected Dashboard')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/dashboard');
    });

    it('explicit /login with safe returnTo parameter navigates to returnTo on success', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('no session'));
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);
      vi.spyOn(apiAuth, 'login').mockResolvedValue({
        accessToken: 'seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      render(<TestSellerApp initialEntries={['/login?returnTo=/returns']} />);

      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      fireEvent.change(screen.getByLabelText(/Email/i, { selector: 'input' }), {
        target: { value: 'seller@zamk.local' },
      });
      fireEvent.change(screen.getByLabelText(/Пароль/i, { selector: 'input' }), {
        target: { value: 'password123' },
      });
      fireEvent.click(screen.getByRole('button', { name: /Войти/i }));

      // Navigates to safe internal returnTo (/returns)
      await waitFor(() => {
        expect(screen.getByText('Protected Returns List')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns');
    });

    it('explicit /login with malicious returnTo parameter falls back to /dashboard', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('no session'));
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);
      vi.spyOn(apiAuth, 'login').mockResolvedValue({
        accessToken: 'seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      render(<TestSellerApp initialEntries={['/login?returnTo=//evil.example/malicious']} />);

      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      fireEvent.change(screen.getByLabelText(/Email/i, { selector: 'input' }), {
        target: { value: 'seller@zamk.local' },
      });
      fireEvent.change(screen.getByLabelText(/Пароль/i, { selector: 'input' }), {
        target: { value: 'password123' },
      });
      fireEvent.click(screen.getByRole('button', { name: /Войти/i }));

      // Falls back to safe /dashboard
      await waitFor(() => {
        expect(screen.getByText('Protected Dashboard')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/dashboard');
    });

    it('already-authenticated seller visiting /login redirects to dashboard immediately', async () => {
      vi.spyOn(apiAuth, 'refresh').mockResolvedValue({ accessToken: 'valid' } as any);
      vi.spyOn(apiAuth, 'me').mockResolvedValue({
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      render(<TestSellerApp initialEntries={['/login']} />);

      await waitFor(() => {
        expect(screen.getByText('Protected Dashboard')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/dashboard');
    });
  });

  describe('Session Persistence Across Refresh Invariants', () => {
    it('restores authenticated session on page refresh via refresh token without flash of login gate', async () => {
      vi.spyOn(apiAuth, 'refresh').mockResolvedValue({
        accessToken: 'persisted-seller-access-token',
        user: { id: 'seller-1', role: 'seller', email: 'seller@zamk.local' },
      } as any);

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842?tab=items';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      // Protected detail renders successfully
      await waitFor(() => {
        expect(screen.getByText('Protected Return Detail')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe('/returns/583fb821-2b10-4966-aacd-e8d24a215842');
      expect(screen.getByTestId('current-search').textContent).toBe('?tab=items');
      expect(screen.queryByTestId('seller-auth-gate')).toBeNull();
    });

    it('rejects access on refresh if session belongs to non-seller user (e.g. customer)', async () => {
      vi.spyOn(apiAuth, 'refresh').mockResolvedValue({
        accessToken: 'customer-access-token',
        user: { id: 'customer-1', role: 'customer', email: 'customer@zamk.local' },
      } as any);

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe(targetPath);
      expect(screen.queryByText('Protected Return Detail')).toBeNull();
    });

    it('renders in-place auth gate on refresh if session is expired or missing', async () => {
      vi.spyOn(apiAuth, 'refresh').mockRejectedValue(new Error('Session expired'));

      const targetPath = '/returns/583fb821-2b10-4966-aacd-e8d24a215842';
      render(<TestSellerApp initialEntries={[targetPath]} />);

      await waitFor(() => {
        expect(screen.getByTestId('seller-auth-gate')).toBeDefined();
      });

      expect(screen.getByTestId('current-pathname').textContent).toBe(targetPath);
      expect(screen.queryByText('Protected Return Detail')).toBeNull();
    });
  });
});

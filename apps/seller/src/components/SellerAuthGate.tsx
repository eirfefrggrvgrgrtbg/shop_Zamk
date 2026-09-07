import React, { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { login } from '@zamk/api-client/src/auth';

export interface SellerLoginFormProps {
  onSuccess?: () => void;
}

export function SellerLoginForm({ onSuccess }: SellerLoginFormProps) {
  const { refreshUser } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      await login({ email, password });
      await refreshUser();
      onSuccess?.();
    } catch (err: any) {
      setError(err.message || 'Ошибка входа');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div data-testid="seller-auth-gate" className="min-h-screen bg-ice flex flex-col justify-center py-12 sm:px-6 lg:px-8 dark:bg-black">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <h2 className="mt-6 text-center text-3xl font-serif text-graphite dark:text-white">
          ZAMK Seller Panel
        </h2>
        <p className="mt-2 text-center text-sm text-ash dark:text-white/60">
          Вход для партнеров
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="bg-white py-8 px-4 shadow sm:rounded-[2rem] sm:px-10 dark:bg-white/5 border border-border-lighter dark:border-white/10">
          <form className="space-y-6" onSubmit={handleSubmit}>
            {error && (
              <div className="bg-red-50 text-red-600 p-3 rounded-xl text-sm">
                {error}
              </div>
            )}
            <div>
              <label htmlFor="seller-email" className="block text-sm font-medium text-graphite dark:text-white/80">
                Email
              </label>
              <div className="mt-1">
                <input
                  id="seller-email"
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="appearance-none block w-full px-3 py-2 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                />
              </div>
            </div>

            <div>
              <label htmlFor="seller-password" className="block text-sm font-medium text-graphite dark:text-white/80">
                Пароль
              </label>
              <div className="mt-1 relative">
                <input
                  id="seller-password"
                  type={showPassword ? "text" : "password"}
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="appearance-none block w-full px-3 py-2 pr-10 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-ash hover:text-graphite dark:hover:text-white"
                  aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
                >
                  {showPassword ? (
                    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"/><path d="M14.084 14.158a3 3 0 0 1-4.242-4.242"/><path d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"/><path d="m2 2 20 20"/></svg>
                  ) : (
                    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/></svg>
                  )}
                </button>
              </div>
            </div>

            <div>
              <button
                type="submit"
                disabled={isLoading}
                className="w-full flex justify-center py-2 px-4 border border-transparent rounded-xl shadow-sm text-sm font-medium text-white bg-graphite hover:bg-black focus:outline-none disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/90 transition-colors"
              >
                {isLoading ? 'Вход...' : 'Войти'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

/**
 * Full-page Seller authentication gate rendered in-place on protected routes.
 * Preserves the current URL in the browser address bar while preventing
 * protected route content or data fetches from running until authenticated.
 */
export function SellerAuthGate() {
  return <SellerLoginForm />;
}

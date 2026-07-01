import React, { useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { changePassword } from '@zamk/api-client/src/auth';

export function SellerProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing, user, logout } = useAuth();
  const location = useLocation();

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  if (isInitializing) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div>
      </div>
    );
  }

  if (!isAuthenticated || user?.role !== 'seller') {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (user?.mustChangePassword) {
    const handleSubmit = async (e: React.FormEvent) => {
      e.preventDefault();
      setError('');
      setIsLoading(true);

      try {
        await changePassword({ currentPassword, newPassword });
        // Force re-login after password change
        await logout();
      } catch (err: any) {
        setError(err.message || 'Ошибка смены пароля');
        setIsLoading(false);
      }
    };

    return (
      <div className="min-h-screen bg-ice flex flex-col justify-center py-12 sm:px-6 lg:px-8 dark:bg-black">
        <div className="sm:mx-auto sm:w-full sm:max-w-md">
          <h2 className="mt-6 text-center text-3xl font-serif text-graphite dark:text-white">
            Требуется смена пароля
          </h2>
          <p className="mt-2 text-center text-sm text-ash dark:text-white/60">
            В целях безопасности вам необходимо изменить временный пароль перед продолжением работы.
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
                <label className="block text-sm font-medium text-graphite dark:text-white/80">
                  Текущий пароль
                </label>
                <div className="mt-1">
                  <input
                    type="password"
                    required
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    className="appearance-none block w-full px-3 py-2 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-graphite dark:text-white/80">
                  Новый пароль
                </label>
                <div className="mt-1">
                  <input
                    type="password"
                    required
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    className="appearance-none block w-full px-3 py-2 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                  />
                </div>
              </div>

              <div>
                <button
                  type="submit"
                  disabled={isLoading}
                  className="w-full flex justify-center py-2 px-4 border border-transparent rounded-xl shadow-sm text-sm font-medium text-white bg-graphite hover:bg-black focus:outline-none disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/90 transition-colors"
                >
                  {isLoading ? 'Смена...' : 'Изменить пароль'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}

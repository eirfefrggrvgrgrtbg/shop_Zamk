import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { login } from '@zamk/api-client/src/auth';

export function SellerLogin() {
  const { isAuthenticated, refreshUser } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      await login({ email, password });
      await refreshUser();
      navigate('/dashboard', { replace: true });
    } catch (err: any) {
      setError(err.message || 'Ошибка входа');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-ice flex flex-col justify-center py-12 sm:px-6 lg:px-8 dark:bg-black">
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
              <label className="block text-sm font-medium text-graphite dark:text-white/80">
                Email
              </label>
              <div className="mt-1">
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="appearance-none block w-full px-3 py-2 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-graphite dark:text-white/80">
                Пароль
              </label>
              <div className="mt-1">
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
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
                {isLoading ? 'Вход...' : 'Войти'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

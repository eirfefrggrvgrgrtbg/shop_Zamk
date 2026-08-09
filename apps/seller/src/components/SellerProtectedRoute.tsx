import React, { useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { changePassword } from '@zamk/api-client/src/auth';
import { Eye, EyeOff, Check } from 'lucide-react';

export function SellerProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing, user, logout } = useAuth();
  const location = useLocation();

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

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
    const rules = {
      length: newPassword.length >= 10,
      upper: /[A-Z]/.test(newPassword),
      lower: /[a-z]/.test(newPassword),
      digit: /[0-9]/.test(newPassword),
      special: /[!@#$%^&*.?\-_+=]/.test(newPassword)
    };

    const isAllRulesMet = Object.values(rules).every(Boolean);
    const isMismatch = newPassword !== confirmPassword && confirmPassword.length > 0;

    const handleSubmit = async (e: React.FormEvent) => {
      e.preventDefault();
      setError('');
      setIsLoading(true);

      if (!currentPassword) {
        setError('Введите текущий пароль');
        setIsLoading(false);
        return;
      }
      if (!isAllRulesMet) {
        setError('Пароль не соответствует требованиям безопасности');
        setIsLoading(false);
        return;
      }
      if (newPassword !== confirmPassword) {
        setError('Пароли не совпадают');
        setIsLoading(false);
        return;
      }

      try {
        await changePassword({ currentPassword: currentPassword, newPassword: newPassword });
        await logout();
      } catch (err: any) {
        let msg = err.message || 'Ошибка смены пароля';
        if (msg.includes('invalid current password') || msg.includes('invalid_password') || msg.includes('Неверный текущий пароль')) {
          msg = 'Неверный текущий пароль';
        } else if (msg.includes('must be different') || msg.includes('должен отличаться')) {
          msg = 'Новый пароль должен отличаться от текущего';
        }
        setError(msg);
        setIsLoading(false);
      }
    };

    const RuleItem = ({ met, text }: { met: boolean; text: string }) => (
      <div className={`flex items-center gap-2 text-sm ${met ? 'text-green-600 dark:text-green-400' : 'text-ash dark:text-white/40'}`}>
        {met ? <Check className="w-4 h-4" /> : <div className="w-4 h-4 rounded-full border border-current opacity-50" />}
        <span>{text}</span>
      </div>
    );

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
                <div className="mt-1 relative">
                  <input
                    type={showPassword ? "text" : "password"}
                    required
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    className="appearance-none block w-full px-3 py-2 pr-10 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                  />
                  <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-500">
                    {showPassword ? <EyeOff className="h-5 w-5" /> : <Eye className="h-5 w-5" />}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-graphite dark:text-white/80">
                  Новый пароль
                </label>
                <div className="mt-1 relative">
                  <input
                    type={showPassword ? "text" : "password"}
                    required
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    className="appearance-none block w-full px-3 py-2 pr-10 border border-border-lighter rounded-xl shadow-sm placeholder-ash focus:outline-none focus:ring-graphite focus:border-graphite sm:text-sm dark:bg-black/20 dark:border-white/20 dark:text-white"
                  />
                  <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-500">
                    {showPassword ? <EyeOff className="h-5 w-5" /> : <Eye className="h-5 w-5" />}
                  </button>
                </div>
                
                <div className="mt-3 space-y-2 bg-gray-50 dark:bg-black/20 p-3 rounded-xl border border-gray-100 dark:border-white/10">
                  <RuleItem met={rules.length} text="Минимум 10 символов" />
                  <RuleItem met={rules.upper} text="Заглавная латинская буква (A-Z)" />
                  <RuleItem met={rules.lower} text="Строчная латинская буква (a-z)" />
                  <RuleItem met={rules.digit} text="Цифра (0-9)" />
                  <RuleItem met={rules.special} text="Спецсимвол (!@#$%^&*.?-_+=)" />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-graphite dark:text-white/80">
                  Повторите новый пароль
                </label>
                <div className="mt-1 relative">
                  <input
                    type={showPassword ? "text" : "password"}
                    required
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    className={`appearance-none block w-full px-3 py-2 pr-10 border ${isMismatch ? 'border-red-300 focus:ring-red-500 focus:border-red-500' : 'border-border-lighter focus:ring-graphite focus:border-graphite'} rounded-xl shadow-sm placeholder-ash focus:outline-none sm:text-sm dark:bg-black/20 dark:text-white`}
                  />
                  <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-400 hover:text-gray-500">
                    {showPassword ? <EyeOff className="h-5 w-5" /> : <Eye className="h-5 w-5" />}
                  </button>
                </div>
                {isMismatch && (
                  <p className="mt-1 text-sm text-red-600">Пароли не совпадают</p>
                )}
              </div>

              <div>
                <button
                  type="submit"
                  disabled={isLoading || !isAllRulesMet || isMismatch}
                  className="w-full flex justify-center py-3 px-4 border border-transparent rounded-xl shadow-sm text-sm font-medium text-white bg-graphite hover:bg-black focus:outline-none disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/90 transition-colors"
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

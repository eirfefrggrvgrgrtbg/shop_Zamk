import { useState } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';

export function AuthModal() {
  const { isAuthModalOpen, closeAuthModal, authView, setAuthView, login, register, resetPassword, changePassword } = useAuth();
  
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [name, setName] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setIsLoading(true);

    try {
      if (authView === 'login') {
        await login(email, password);
      } else if (authView === 'register') {
        await register(name, email, password);
      } else if (authView === 'forgot_password') {
        await resetPassword(email);
        setSuccess('Ссылка для восстановления отправлена на ваш e-mail.');
      } else if (authView === 'change_password') {
        await changePassword(password, newPassword);
      }
    } catch (err: any) {
      setError(err.message || 'Произошла ошибка при отправке');
    } finally {
      setIsLoading(false);
    }
  };

  const isLogin = authView === 'login';
  const isRegister = authView === 'register';
  const isForgot = authView === 'forgot_password';
  const isChangePass = authView === 'change_password';

  return (
    <Modal isOpen={isAuthModalOpen} onClose={closeAuthModal}>
      <div className="p-4 sm:p-6 w-full mt-2 bg-transparent text-graphite dark:text-white relative">
        <div className="text-center mb-8">
          <h2 className="text-2xl font-serif font-medium tracking-wide dark:text-white">
            {isLogin ? 'Вход' : isRegister ? 'Регистрация' : isChangePass ? 'Смена пароля' : 'Сброс пароля'}
          </h2>
          <p className="mt-2 text-[13.5px] text-graphite/60 dark:text-white/70 tracking-wide">
            {isLogin 
              ? 'Войдите в личный кабинет' 
              : isRegister 
                ? 'Присоединяйтесь к ZAMK' 
                : isChangePass 
                  ? 'Пожалуйста, установите новый пароль'
                  : 'Введите e-mail, и мы отправим ссылку для восстановления'}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="p-3 text-[13px] text-red-500 dark:text-red-400 bg-red-50/50 dark:bg-red-950/30 backdrop-blur-sm border border-red-100 dark:border-red-900/50 rounded-2xl text-center">
              {error}
            </div>
          )}
          
          {success && (
            <div className="p-3 text-[13px] text-green-700 dark:text-green-400 bg-green-50/50 dark:bg-green-950/30 backdrop-blur-sm border border-green-100 dark:border-green-900/50 rounded-2xl text-center">
              {success}
            </div>
          )}

          {isRegister && (
            <div>
              <Input 
                placeholder="Ваше имя"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="bg-white/60 dark:bg-white/5 focus:bg-white dark:focus:bg-white/10 backdrop-blur-sm"
              />
            </div>
          )}
          
          {!isChangePass && (
            <div>
              <Input 
                type="email"
                placeholder="E-mail"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="bg-white/60 dark:bg-white/5 focus:bg-white dark:focus:bg-white/10 backdrop-blur-sm"
              />
            </div>
          )}
          
          {!isForgot && (
            <div>
              <Input 
                type="password"
                placeholder={isChangePass ? "Текущий пароль" : "Пароль"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="bg-white/60 dark:bg-white/5 focus:bg-white dark:focus:bg-white/10 backdrop-blur-sm"
              />
            </div>
          )}

          {isChangePass && (
            <div>
              <Input 
                type="password"
                placeholder="Новый пароль"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                className="bg-white/60 dark:bg-white/5 focus:bg-white dark:focus:bg-white/10 backdrop-blur-sm"
              />
            </div>
          )}

          {isLogin && (
            <div className="flex justify-end pt-1">
              <button 
                type="button" 
                onClick={() => {
                  setAuthView('forgot_password');
                  setError('');
                  setSuccess('');
                }}
                className="text-[13px] text-graphite/50 dark:text-white/50 hover:text-graphite dark:hover:text-white transition-colors"
              >
                Забыли пароль?
              </button>
            </div>
          )}

          <div className="pt-4">
            <Button 
              className="w-full h-12 text-[14px] font-medium tracking-wide"
              disabled={isLoading || (isForgot && !!success)}
            >
              {isLoading 
                ? 'Загрузка...' 
                : isLogin 
                  ? 'Войти' 
                  : isRegister 
                    ? 'Создать аккаунт' 
                    : isChangePass 
                      ? 'Сохранить пароль'
                      : 'Отправить ссылку'}
            </Button>
          </div>
        </form>

        {!isChangePass && (
          <div className="mt-8 text-center border-t border-border-lighter dark:border-white/10 pt-6">
            <button 
              type="button"
              onClick={() => {
                setAuthView(isLogin ? 'register' : 'login');
                setError('');
                setSuccess('');
              }}
              className="text-[13px] text-graphite/60 dark:text-white/60 hover:text-graphite dark:hover:text-white transition-colors uppercase tracking-[0.04em] font-medium"
            >
              {isLogin 
                ? 'Нет аккаунта? Зарегистрироваться' 
                : isRegister 
                  ? 'Уже есть аккаунт? Войти' 
                  : 'Вспомнили пароль? Войти'}
            </button>
          </div>
        )}
      </div>
    </Modal>
  );
}
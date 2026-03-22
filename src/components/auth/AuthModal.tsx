import { useState } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';

export function AuthModal() {
  const { isAuthModalOpen, closeAuthModal, authView, setAuthView, login, register, resetPassword } = useAuth();
  
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
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

  return (
    <Modal isOpen={isAuthModalOpen} onClose={closeAuthModal}>
      <div className="p-4 sm:p-6 w-full mt-2 bg-transparent text-graphite relative">
        <div className="text-center mb-8">
          <h2 className="text-2xl font-serif font-medium tracking-wide">
            {isLogin ? 'Вход' : isRegister ? 'Регистрация' : 'Сброс пароля'}
          </h2>
          <p className="mt-2 text-[13.5px] text-graphite/60 tracking-wide">
            {isLogin 
              ? 'Войдите в личный кабинет' 
              : isRegister 
                ? 'Присоединяйтесь к ZAMK' 
                : 'Введите e-mail, и мы отправим ссылку для восстановления'}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="p-3 text-[13px] text-red-600 bg-red-50/50 backdrop-blur-sm border border-red-100 rounded-2xl text-center">
              {error}
            </div>
          )}
          
          {success && (
            <div className="p-3 text-[13px] text-green-700 bg-green-50/50 backdrop-blur-sm border border-green-100 rounded-2xl text-center">
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
                className="bg-white/60 focus:bg-white backdrop-blur-sm"
              />
            </div>
          )}
          
          <div>
            <Input 
              type="email"
              placeholder="E-mail"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="bg-white/60 focus:bg-white backdrop-blur-sm"
            />
          </div>
          
          {!isForgot && (
            <div>
              <Input 
                type="password"
                placeholder="Пароль"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="bg-white/60 focus:bg-white backdrop-blur-sm"
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
                className="text-[13px] text-graphite/50 hover:text-graphite transition-colors"
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
                    : 'Отправить ссылку'}
            </Button>
          </div>
        </form>

        <div className="mt-8 text-center border-t border-border-lighter pt-6">
          <button 
            type="button"
            onClick={() => {
              setAuthView(isLogin ? 'register' : 'login');
              setError('');
              setSuccess('');
            }}
            className="text-[13px] text-graphite/60 hover:text-graphite transition-colors uppercase tracking-[0.04em] font-medium"
          >
            {isLogin 
              ? 'Нет аккаунта? Зарегистрироваться' 
              : isRegister 
                ? 'Уже есть аккаунт? Войти' 
                : 'Вспомнили пароль? Войти'}
          </button>
        </div>
      </div>
    </Modal>
  );
}
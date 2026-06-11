import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { ProfilePanel } from '../components/editorial/StudioKit';
import { AccountNav } from '../components/account/AccountNav';
import { CustomerProtectedRoute } from '../components/account/CustomerProtectedRoute';
import { Lock, LogOut } from 'lucide-react';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { getOrders, getProfile, updateProfile } from '@zamk/api-client/src/customer';

const STATUS_LABELS: Record<string, string> = {
  active: 'Активен',
  blocked: 'Заблокирован',
  deleted: 'Удалён',
};

export function Profile() {
  return (
    <CustomerProtectedRoute
      title="Личный кабинет"
      description="Войдите, чтобы просматривать профиль и управлять заказами."
    >
      <ProfileContent />
    </CustomerProtectedRoute>
  );
}

function ProfileContent() {
  const { totalItems } = useCart();
  const { favorites } = useFavorites();
  const { user, logout, changePassword } = useAuth();
  const [ordersCount, setOrdersCount] = useState<number | null>(null);
  const [isPasswordModalOpen, setPasswordModalOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [isSavingPassword, setIsSavingPassword] = useState(false);

  const [editName, setEditName] = useState(user?.name || '');
  const [editPhone, setEditPhone] = useState('');
  const [profileError, setProfileError] = useState('');
  const [profileSuccess, setProfileSuccess] = useState(false);
  const [isSavingProfile, setIsSavingProfile] = useState(false);

  useEffect(() => {
    getOrders()
      .then((orders) => setOrdersCount(orders.length))
      .catch(() => setOrdersCount(0));

    getProfile()
      .then((data) => {
        setEditName(data.name || '');
        setEditPhone(data.phone || '');
      })
      .catch(console.error);
  }, []);

  const handleSaveProfile = async () => {
    setProfileError('');
    setProfileSuccess(false);
    if (!editName.trim()) {
      setProfileError('Имя обязательно');
      return;
    }
    try {
      setIsSavingProfile(true);
      await updateProfile({ name: editName, phone: editPhone });
      setProfileSuccess(true);
      setTimeout(() => setProfileSuccess(false), 3000);
    } catch (err: any) {
      setProfileError(err.message || 'Не удалось обновить профиль');
    } finally {
      setIsSavingProfile(false);
    }
  };

  const handleChangePassword = async () => {
    setPasswordError('');
    if (!currentPassword || !newPassword) {
      setPasswordError('Заполните все поля');
      return;
    }
    if (newPassword !== confirmPassword) {
      setPasswordError('Пароли не совпадают');
      return;
    }
    if (newPassword.length < 8) {
      setPasswordError('Новый пароль должен быть не короче 8 символов');
      return;
    }
    try {
      setIsSavingPassword(true);
      await changePassword(currentPassword, newPassword);
      setPasswordModalOpen(false);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: any) {
      setPasswordError(err.message || 'Не удалось сменить пароль');
    } finally {
      setIsSavingPassword(false);
    }
  };

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
        <section className="mb-6 max-w-[980px] mx-auto">
          <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">
            Личный кабинет
          </p>
          <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">
            Профиль
          </h1>
        </section>

        <div className="max-w-[980px] mx-auto">
          <AccountNav />
        </div>

        <div className='mt-4 grid grid-cols-1 md:grid-cols-2 gap-5 max-w-[980px] mx-auto'>
          <ProfilePanel title='Аккаунт'>
            <div className="flex flex-col gap-3">
              <div>
                <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Имя</label>
                <Input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  placeholder="Ваше имя"
                  className="bg-white/50 dark:bg-white/5 border-white/40 dark:border-white/20"
                />
              </div>
              <div>
                <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Телефон</label>
                <Input
                  value={editPhone}
                  onChange={(e) => setEditPhone(e.target.value)}
                  placeholder="+7 (999) 000-00-00"
                  className="bg-white/50 dark:bg-white/5 border-white/40 dark:border-white/20"
                />
              </div>
              <div>
                <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Email</label>
                <Input
                  value={user?.email || ''}
                  readOnly
                  disabled
                  className="bg-white/30 dark:bg-white/5 border-white/20 dark:border-white/10 opacity-70 cursor-not-allowed"
                />
              </div>
              {user?.status && (
                <p className='text-sm text-ash mt-2'>
                  Статус: <span className="text-graphite dark:text-gray-200">{STATUS_LABELS[user.status] || user.status}</span>
                </p>
              )}
              {profileError && <p className="text-sm text-red-500 mt-1">{profileError}</p>}
              {profileSuccess && <p className="text-sm text-green-600 dark:text-green-400 mt-1">Профиль обновлён.</p>}
              <Button onClick={handleSaveProfile} disabled={isSavingProfile} className="mt-2 self-start w-auto px-6">
                {isSavingProfile ? 'Сохранение...' : 'Сохранить изменения'}
              </Button>
            </div>
          </ProfilePanel>

          <ProfilePanel title='Статистика'>
            <div className='grid grid-cols-3 gap-3 text-center'>
              <div className='rounded-2xl bg-white dark:bg-white/5 border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite dark:text-gray-200'>
                  {ordersCount === null ? '—' : ordersCount}
                </p>
                <p className='text-xs text-ash'>Заказы</p>
              </div>
              <div className='rounded-2xl bg-white dark:bg-white/5 border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite dark:text-gray-200'>{favorites.length}</p>
                <p className='text-xs text-ash'>Избранное</p>
              </div>
              <div className='rounded-2xl bg-white dark:bg-white/5 border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite dark:text-gray-200'>{totalItems}</p>
                <p className='text-xs text-ash'>Корзина</p>
              </div>
            </div>
          </ProfilePanel>
        </div>

        <ProfilePanel title='Разделы' className='mt-5 max-w-[980px] mx-auto'>
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-3'>
            <Link to='/orders' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Мои заказы</Link>
            <Link to='/favorites' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Избранное</Link>
            <Link to='/orders' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Возвраты</Link>
            <Link to='/reviews' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Мои отзывы</Link>
            <Link to='/settings' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all sm:col-span-2'>Настройки профиля</Link>
          </div>
        </ProfilePanel>

        <ProfilePanel title='Безопасность' className='mt-5 max-w-[980px] mx-auto'>
          <div className="flex flex-col -mx-5 -mb-6">
            <button
              type="button"
              onClick={() => setPasswordModalOpen(true)}
              className="flex items-center justify-between p-4 md:p-5 border-b border-border-lighter dark:border-white/10 hover:bg-white/40 dark:hover:bg-white/5 transition-colors text-left"
            >
              <div className="flex items-center gap-4">
                <div className="w-10 h-10 rounded-full flex items-center justify-center shrink-0 bg-graphite/5 dark:bg-white/5 text-graphite/60 dark:text-gray-300">
                  <Lock className="w-5 h-5 stroke-[1.5]" />
                </div>
                <div>
                  <h4 className="text-[15px] font-medium text-graphite dark:text-gray-200">Сменить пароль</h4>
                  <p className="text-[13px] text-graphite/50 dark:text-gray-400 mt-0.5">Обновить текущий пароль учётной записи</p>
                </div>
              </div>
            </button>
            <button
              type="button"
              onClick={() => logout()}
              className="flex items-center justify-between p-4 md:p-5 hover:bg-white/40 dark:hover:bg-white/5 transition-colors text-left"
            >
              <div className="flex items-center gap-4">
                <div className="w-10 h-10 rounded-full flex items-center justify-center shrink-0 bg-graphite/5 dark:bg-white/5 text-graphite/60 dark:text-gray-300">
                  <LogOut className="w-5 h-5 stroke-[1.5]" />
                </div>
                <div>
                  <h4 className="text-[15px] font-medium text-graphite dark:text-gray-200">Выйти</h4>
                  <p className="text-[13px] text-graphite/50 dark:text-gray-400 mt-0.5">Завершить текущий сеанс</p>
                </div>
              </div>
            </button>
          </div>
        </ProfilePanel>
      </div>

      <Modal isOpen={isPasswordModalOpen} onClose={() => setPasswordModalOpen(false)} title="Смена пароля">
        <div className="space-y-4 pt-2">
          <Input
            type="password"
            placeholder="Текущий пароль"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            className="bg-white/50 dark:bg-white/5"
          />
          <Input
            type="password"
            placeholder="Новый пароль"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            className="bg-white/50 dark:bg-white/5"
          />
          <Input
            type="password"
            placeholder="Повторите новый пароль"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="bg-white/50 dark:bg-white/5"
          />
          {passwordError && <p className="text-sm text-red-500">{passwordError}</p>}
          <Button
            className="w-full mt-4 bg-graphite dark:bg-white dark:text-black"
            onClick={handleChangePassword}
            disabled={isSavingPassword}
          >
            {isSavingPassword ? 'Сохранение...' : 'Сохранить пароль'}
          </Button>
        </div>
      </Modal>
    </div>
  );
}

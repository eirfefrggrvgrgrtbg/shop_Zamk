import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { ProfilePanel } from '../components/editorial/StudioKit';
import { Lock, LogOut, Trash2, ChevronRight } from 'lucide-react';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

function ProfileSectionRow({ icon: Icon, title, description, children, danger, onClick }: any) {
  return (
    <div
      onClick={onClick}
      className={`flex items-center justify-between p-4 md:p-5 border-b border-border-lighter dark:border-white/10 last:border-b-0 hover:bg-white/40 dark:hover:bg-white/5 transition-colors cursor-pointer`}
    >
      <div className="flex items-center gap-4">
        {Icon && (
          <div className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${danger ? 'bg-red-50 dark:bg-red-500/10 text-red-500' : 'bg-graphite/5 dark:bg-white/5 text-graphite/60 dark:text-gray-300'}`}>
            <Icon className="w-5 h-5 stroke-[1.5]" />
          </div>
        )}
        <div>
          <h4 className={`text-[15px] font-medium ${danger ? 'text-red-500' : 'text-graphite dark:text-gray-200'}`}>{title}</h4>
          {description && <p className="text-[13px] text-graphite/50 dark:text-gray-400 mt-0.5">{description}</p>}
        </div>
      </div>
      <div>
        {children || (onClick && <ChevronRight className="w-5 h-5 text-graphite/30 dark:text-gray-500" />)}
      </div>
    </div>
  );
}

export function Profile() {
  const { totalItems } = useCart();
  const { favorites } = useFavorites();
  const { user, logout } = useAuth();

  const [isPasswordModalOpen, setPasswordModalOpen] = useState(false);
  const [isLogoutModalOpen, setLogoutModalOpen] = useState(false);
  const [isDeleteModalOpen, setDeleteModalOpen] = useState(false);

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
        <section className="mb-10 max-w-[980px] mx-auto">
          <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">
            Личный кабинет
          </p>
          <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">
            Профиль
          </h1>
        </section>

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5 max-w-[980px] mx-auto'>
          <ProfilePanel title='Аккаунт'>
            <p className='text-graphite-light dark:text-gray-300'>{user ? user.name : 'Гость'}</p>
            <p className='text-sm text-ash mt-1'>{user ? user.email : 'Авторизуйтесь для управления аккаунтом'}</p>
          </ProfilePanel>

          <ProfilePanel title='Статистика'>
            <div className='grid grid-cols-3 gap-3 text-center'>
              <div className='rounded-2xl bg-white dark:bg-white/5 border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite dark:text-gray-200'>2</p>
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
            <Link to='/returns' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Возвраты</Link>
            <button className='text-left rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Данные</button>
            <button className='text-left rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Адреса</button>
            <Link to='/help' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all'>Поддержка</Link>
            <Link to='/settings' className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white dark:bg-white/5 px-4 py-3 text-sm text-graphite dark:text-gray-300 hover:bg-surface-hover hover:border-graphite/20 shadow-sm transition-all sm:col-span-2'>Настройки</Link>
          </div>
        </ProfilePanel>

        {/* Блок Безопасность */}
        <ProfilePanel title='Безопасность' className='mt-5 max-w-[980px] mx-auto'>
          <div className="flex flex-col -mx-5 -mb-6">
            <ProfileSectionRow
              icon={Lock}
              title="Сменить пароль"
              description="Обновить текущий пароль учетной записи"
              onClick={() => setPasswordModalOpen(true)}
            />
            <ProfileSectionRow
              icon={LogOut}
              title="Выйти со всех устройств"
              description="Завершить сеансы на других устройствах"
              onClick={() => setLogoutModalOpen(true)}
            />
            <ProfileSectionRow
              icon={Trash2}
              title="Удалить аккаунт"
              description="Навсегда удалить профиль и все данные"
              danger
              onClick={() => setDeleteModalOpen(true)}
            />
          </div>
        </ProfilePanel>

      </div>

      {/* Модалки безопасности */}
      <Modal isOpen={isPasswordModalOpen} onClose={() => setPasswordModalOpen(false)} title="Смена пароля">
        <div className="space-y-4 pt-2">
          <Input type="password" placeholder="Текущий пароль" className="bg-white/50 dark:bg-white/5" />
          <Input type="password" placeholder="Новый пароль" className="bg-white/50 dark:bg-white/5" />
          <Input type="password" placeholder="Повторите новый пароль" className="bg-white/50 dark:bg-white/5" />
          <Button className="w-full mt-4 bg-graphite dark:bg-white dark:text-black" onClick={() => setPasswordModalOpen(false)}>Сохранить пароль</Button>
        </div>
      </Modal>

      <Modal isOpen={isLogoutModalOpen} onClose={() => setLogoutModalOpen(false)} title="Завершение сеансов">
        <div className="pt-2 text-center pb-2">
          <p className="text-graphite/70 dark:text-gray-400 text-[15px] mb-6">Вы уверены, что хотите выйти из аккаунта на всех других устройствах? Текущий сеанс будет сохранен.</p>
          <div className="grid grid-cols-2 gap-3">
            <Button variant="outline" className="dark:border-white/20 dark:text-white" onClick={() => setLogoutModalOpen(false)}>Отмена</Button>
            <Button className="bg-graphite dark:bg-white dark:text-black" onClick={() => setLogoutModalOpen(false)}>Подтвердить</Button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={isDeleteModalOpen} onClose={() => setDeleteModalOpen(false)} title="Удаление аккаунта">
        <div className="pt-2 text-center pb-2">
          <div className="w-16 h-16 bg-red-50 dark:bg-red-500/10 text-red-500 rounded-full flex items-center justify-center mx-auto mb-4">
            <Trash2 className="w-8 h-8" />
          </div>
          <p className="text-graphite dark:text-gray-200 mb-2 font-medium">Это действие необратимо</p>
          <p className="text-graphite/60 dark:text-gray-400 text-[14px] mb-8">Вся информация, включая заказы, избранное и адреса, будет удалена навсегда без возможности восстановления.</p>

          <div className="flex flex-col gap-3">
            <Button className="w-full bg-red-500 hover:bg-red-600 dark:bg-red-500 dark:hover:bg-red-600 text-white border-0" onClick={() => { setDeleteModalOpen(false); if (logout) logout(); }}>
              Да, удалить аккаунт
            </Button>
            <button
              onClick={() => setDeleteModalOpen(false)}
              className="py-3 text-[14px] font-medium text-graphite/60 dark:text-gray-400 hover:text-graphite dark:hover:text-white transition-colors mt-2"
            >
              Отменить
            </button>
          </div>
        </div>
      </Modal>

    </div>
  );
}

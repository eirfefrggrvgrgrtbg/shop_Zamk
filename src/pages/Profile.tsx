import { Link } from 'react-router-dom';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { HeroBlock, ProfilePanel } from '../components/editorial/StudioKit';

export function Profile() {
  const { totalItems } = useCart();
  const { favorites } = useFavorites();

  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[980px]'>
        <HeroBlock
          label='Профиль'
          title={<>Личный кабинет ZAMK</>}
          description='Единый интерфейс управления заказами, избранным и настройками в спокойной premium-стилистике.'
        />

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5'>
          <ProfilePanel title='Аккаунт'>
            <p className='text-graphite-light'>Анна Прокофьева</p>
            <p className='text-sm text-ash mt-1'>anna@zamk.store</p>
          </ProfilePanel>

          <ProfilePanel title='Статистика'>
            <div className='grid grid-cols-3 gap-3 text-center'>
              <div className='rounded-2xl bg-white border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite'>2</p>
                <p className='text-xs text-ash'>Заказы</p>
              </div>
              <div className='rounded-2xl bg-white border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite'>{favorites.length}</p>
                <p className='text-xs text-ash'>Избранное</p>
              </div>
              <div className='rounded-2xl bg-white border border-border-lighter p-3'>
                <p className='text-2xl font-serif text-graphite'>{totalItems}</p>
                <p className='text-xs text-ash'>Корзина</p>
              </div>
            </div>
          </ProfilePanel>
        </div>

        <ProfilePanel title='Разделы' className='mt-5'>
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-3'>
            <Link to='/favorites' className='rounded-2xl border border-border-lighter bg-white px-4 py-3 text-sm text-graphite'>Избранное</Link>
            <Link to='/cart' className='rounded-2xl border border-border-lighter bg-white px-4 py-3 text-sm text-graphite'>Корзина</Link>
            <Link to='/delivery' className='rounded-2xl border border-border-lighter bg-white px-4 py-3 text-sm text-graphite'>Доставка и возврат</Link>
            <Link to='/help' className='rounded-2xl border border-border-lighter bg-white px-4 py-3 text-sm text-graphite'>Помощь</Link>
          </div>
        </ProfilePanel>
      </div>
    </div>
  );
}

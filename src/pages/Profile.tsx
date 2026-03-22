import { Link } from 'react-router-dom';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { ProfilePanel } from '../components/editorial/StudioKit';

export function Profile() {
  const { totalItems } = useCart();
  const { favorites } = useFavorites();
  const { user } = useAuth();

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
        <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm'>
          <div className='relative h-[190px] md:h-[240px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.2rem,6.2vw,6.5rem)] text-white/43 leading-[0.8] tracking-[-0.03em]'>ПРОФИЛЬ</h2>
              <h3 className='font-serif text-[clamp(1.8rem,5vw,5rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center'>ЛИЧНЫЙ КАБИНЕТ</h3>
            </div>
          </div>
        </section>

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5 max-w-[980px] mx-auto'>
          <ProfilePanel title='Аккаунт'>
            <p className='text-graphite-light'>{user ? user.name : 'Гость'}</p>
            <p className='text-sm text-ash mt-1'>{user ? user.email : 'Авторизуйтесь для управления аккаунтом'}</p>
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

        <ProfilePanel title='Разделы' className='mt-5 max-w-[980px] mx-auto'>
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

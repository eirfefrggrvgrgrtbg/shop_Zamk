import { Link } from 'react-router-dom';
import { ArrowRight, ShoppingBag } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useToast } from '../contexts/ToastContext';
import { PRODUCTS } from '../lib/mock-data';
import { SectionHeader, WishlistCard } from '../components/editorial/StudioKit';

export function Favorites() {
  const { favorites } = useFavorites();
  const { addItem } = useCart();
  const { showToast } = useToast();

  const favoriteProducts = PRODUCTS.filter((product) => favorites.includes(product.id));

  if (favoriteProducts.length === 0) {
    return (
      <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-5xl'>
          <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm'>
            <div className='relative h-[200px] md:h-[260px]'>
              <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]' />
              <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]' />
              <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
                <h2 className='font-serif text-[clamp(2.6rem,7.4vw,7.8rem)] text-white/43 leading-[0.8] tracking-[-0.03em]'>ИЗБРАННОЕ</h2>
                <h3 className='font-serif text-[clamp(2.2rem,6.3vw,6.8rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center'>ЛИЧНЫЙ АРХИВ</h3>
              </div>
            </div>
          </section>

          <div className='mt-8 bg-white/58 border border-white/50 p-8'>
            <EmptyState
              icon='heart'
              title='Нет сохранённых товаров'
              description='Добавь позиции из каталога'
              action={
                <Link to='/catalog'>
                  <Button className='gap-2'>
                    Перейти в каталог <ArrowRight className='w-4 h-4' />
                  </Button>
                </Link>
              }
            />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm'>
          <div className='relative h-[200px] md:h-[260px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.6rem,7.4vw,7.8rem)] text-white/43 leading-[0.8] tracking-[-0.03em]'>ИЗБРАННОЕ</h2>
              <h3 className='font-serif text-[clamp(2.2rem,6.3vw,6.8rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center'>ЛИЧНЫЙ АРХИВ</h3>
              <h4 className='font-serif text-[clamp(2.2rem,6.6vw,7.2rem)] text-white/42 leading-[0.8] tracking-[-0.03em] text-right'>
                {favoriteProducts.length}
              </h4>
            </div>
          </div>
        </section>

        <section className='mt-10'>
          <SectionHeader label='Товары' title='Ваша подборка' />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {favoriteProducts.map((product) => (
              <div key={product.id} className='group relative'>
                <WishlistCard product={product} />
                <Button
                  size='sm'
                  className='absolute bottom-7 left-1/2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100 w-[76%] gap-2'
                  onClick={(event) => {
                    event.preventDefault();
                    addItem(product);
                    showToast('Товар добавлен в корзину');
                  }}
                >
                  <ShoppingBag className='w-4 h-4' /> В корзину
                </Button>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

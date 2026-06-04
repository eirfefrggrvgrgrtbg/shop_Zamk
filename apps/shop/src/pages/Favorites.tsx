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
      <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-5xl'>
          <section className="mb-12 border-b border-border-lighter pb-8">
            <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
              Личный архив
            </p>
            <div className="flex items-end justify-between">
              <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
                Избранное
              </h1>
            </div>
          </section>

          <div className='mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm'>
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
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className="mb-12 border-b border-border-lighter pb-8 flex items-end justify-between">
          <div>
            <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
              Личный архив
            </p>
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Избранное
            </h1>
          </div>
          <span className="text-sm text-ash mb-1 hidden sm:block">{favoriteProducts.length} позиций</span>
        </section>

        <section className='mt-10'>
          <SectionHeader label='Товары' title='Ваша подборка' />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {favoriteProducts.map((product) => (
              <div key={product.id} className='group relative'>
                <WishlistCard product={product} />
                <Button
                  type='button'
                  size='sm'
                  className='absolute bottom-7 left-1/2 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100 w-[76%] gap-2'
                  onClick={(event) => {
                    event.preventDefault();
                    const hasSelectableSizes = Boolean(product.sizes && product.sizes.length > 0 && !product.sizes.includes('Единый'));

                    if (hasSelectableSizes) {
                      showToast('Выберите размер на странице товара');
                      return;
                    }

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

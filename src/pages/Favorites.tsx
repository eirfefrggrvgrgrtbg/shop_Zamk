import { Link } from 'react-router-dom';
import { ArrowRight, ShoppingBag } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useToast } from '../contexts/ToastContext';
import { PRODUCTS } from '../lib/mock-data';
import { HeroBlock, SectionHeader, WishlistCard } from '../components/editorial/StudioKit';

export function Favorites() {
  const { favorites } = useFavorites();
  const { addItem } = useCart();
  const { showToast } = useToast();

  const favoriteProducts = PRODUCTS.filter((product) => favorites.includes(product.id));

  if (favoriteProducts.length === 0) {
    return (
      <div className='relative z-10 min-h-screen pt-28 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-5xl'>
          <HeroBlock
            label='Избранное'
            title={<>Личная селекция пуста</>}
            description='Сохраняй вещи в избранное, чтобы собрать спокойный премиальный гардероб и вернуться к выбору позже.'
            right={
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
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px]'>
        <HeroBlock
          label='Избранное'
          title={<>Собранная коллекция</>}
          description={`Сохранено ${favoriteProducts.length} позиций. Держим единый студийный ритм на всей странице.`}
        />

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

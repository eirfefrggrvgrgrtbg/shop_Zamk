import { Link } from 'react-router-dom';
import { ArrowRight, Loader2 } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { useFavorites } from '../contexts/FavoritesContext';
import { ProductCard } from '../components/product/ProductCard';
import { AccountLayout } from '../components/account/AccountLayout';

export function Favorites() {
  return (
    <AccountLayout title="Избранное">
      <FavoritesContent />
    </AccountLayout>
  );
}

function FavoritesContent() {
  const { favorites, isLoading, error } = useFavorites();

  return (
    <>
        {isLoading ? (
          <div className="flex justify-center items-center h-64">
            <Loader2 className="w-8 h-8 animate-spin text-graphite/40" />
          </div>
        ) : error ? (
          <div className='mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm max-w-5xl mx-auto'>
            <EmptyState
              icon='heart'
              title='Не удалось загрузить избранное'
              description={error}
            />
          </div>
        ) : favorites.length === 0 ? (
          <div className='mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm max-w-5xl mx-auto'>
            <EmptyState
              icon='heart'
              title='В избранном пока нет товаров'
              description='Сохраняйте понравившиеся товары, чтобы вернуться к ним позже.'
              action={
                <Link to='/catalog'>
                  <Button className='gap-2'>
                    Перейти в каталог <ArrowRight className='w-4 h-4' />
                  </Button>
                </Link>
              }
            />
          </div>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-8 mt-8">
            {favorites.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        )}
    </>
  );
}

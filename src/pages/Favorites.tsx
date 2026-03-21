import { Link } from 'react-router-dom';
import { ArrowRight, ShoppingBag } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { ProductCard } from '../components/product/ProductCard';
import { useFavorites } from '../contexts/FavoritesContext';
import { useCart } from '../contexts/CartContext';
import { useToast } from '../contexts/ToastContext';
import { PRODUCTS } from '../lib/mock-data';

export function Favorites() {
  const { favorites } = useFavorites();
  const { addItem } = useCart();
  const { showToast } = useToast();

  const favoriteProducts = PRODUCTS.filter(p => favorites.includes(p.id));

  if (favoriteProducts.length === 0) {
    return (
      <div className="min-h-screen bg-milk">
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-8">
          <h1 className="text-3xl font-serif text-graphite mb-8">Избранное</h1>
          <EmptyState
            icon="heart"
            title="В избранном пусто"
            description="Нажмите ♡ на карточке товара, чтобы сохранить его в избранное"
            action={
              <Link to="/catalog">
                <Button variant="primary" className="gap-2">
                  Перейти в каталог <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-8">
        <div className="flex items-end justify-between mb-8 gap-4">
          <div>
            <h1 className="text-3xl font-serif text-graphite">Избранное</h1>
            <p className="text-sm text-ash mt-1">{favoriteProducts.length} товаров</p>
          </div>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
          {favoriteProducts.map(product => (
            <div key={product.id} className="relative group">
              <ProductCard product={product} />
              <Button
                variant="secondary"
                size="sm"
                className="absolute bottom-16 left-3 right-3 gap-1.5 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={(e) => {
                  e.preventDefault();
                  addItem(product);
                  showToast('Добавлено в корзину');
                }}
              >
                <ShoppingBag className="w-3.5 h-3.5" />
                В корзину
              </Button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

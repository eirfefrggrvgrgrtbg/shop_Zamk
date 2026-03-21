import { Link } from 'react-router-dom';
import { Heart } from 'lucide-react';
import type { Product } from '../../lib/mock-data';
import { formatPrice } from '../../lib/utils';
import { Badge } from '../ui/Badge';
import { useFavorites } from '../../contexts/FavoritesContext';

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const { toggleFavorite, isFavorite } = useFavorites();
  const favorited = isFavorite(product.id);

  return (
    <div className="group flex flex-col h-full bg-white/80 backdrop-blur-md shadow-[0_8px_30px_rgba(124,156,191,0.06)] hover:shadow-cloud transition-all duration-500 rounded-[2rem] overflow-hidden">
      <Link to={`/product/${product.id}`} className="relative block overflow-hidden aspect-[4/5] bg-ash-light/10">
        <div className="relative w-full h-full overflow-hidden">
          <img
            src={product.images?.[0] ?? product.image}
            alt={product.name}
            className="object-cover w-full h-full transition-transform duration-700 ease-out group-hover:scale-105"
            loading="lazy"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-[#20365114] via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
        </div>

        <div className="absolute top-4 left-4 flex flex-col gap-2">
          {product.isNew && <Badge variant="new">Новинка</Badge>}
          {product.isBestseller && <Badge variant="bestseller">Хит</Badge>}
          {product.discountPrice && <Badge variant="sale">Скидка</Badge>}
        </div>

        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-4 right-4 w-9 h-9 flex items-center justify-center transition-all duration-300 z-10 rounded-full
            ${favorited
              ? 'bg-white text-error shadow-sm'
              : 'bg-white/50 backdrop-blur-md text-graphite hover:bg-white border border-white/50 opacity-0 group-hover:opacity-100'
            }`}
        >
          <Heart className={`w-5 h-5 ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      <div className="p-5 md:p-6 flex flex-col flex-grow">
        <div className="mb-auto">
          <p className="text-[10px] font-semibold tracking-[0.14em] text-ash uppercase mb-2">{product.brand}</p>
          <Link to={`/product/${product.id}`} className="block group-hover:text-primary transition-colors duration-300">
            <h3 className="text-sm md:text-base font-medium text-graphite leading-relaxed">{product.name}</h3>
          </Link>
        </div>

        <div className="mt-4 flex items-baseline gap-2">
          {product.discountPrice ? (
            <>
              <span className="text-base font-bold text-error">{formatPrice(product.discountPrice)}</span>
              <span className="text-xs text-ash-light line-through decoration-ash-light/50">{formatPrice(product.price)}</span>
            </>
          ) : (
            <span className="text-sm md:text-base text-graphite">{formatPrice(product.price)}</span>
          )}
        </div>
      </div>
    </div>
  );
}

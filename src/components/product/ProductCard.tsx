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
    <div className="group rounded-[1.75rem] border border-border-lighter bg-white/90 flex flex-col h-full shadow-[0_12px_22px_rgba(94,125,160,0.08)] hover:shadow-[0_20px_28px_rgba(94,125,160,0.12)] transition-all duration-300">
      <Link to={`/product/${product.id}`} className="relative block overflow-hidden aspect-[4/5] p-2.5">
        <div className="relative w-full h-full rounded-[1.35rem] overflow-hidden">
          <img
            src={product.images?.[0] ?? product.image}
            alt={product.name}
            className="object-cover w-full h-full transition-transform duration-700 ease-out group-hover:scale-105"
            loading="lazy"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-[#2036511c] via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
        </div>

        <div className="absolute top-5 left-5 flex flex-col gap-2">
          {product.isNew && <Badge variant="new">Новинка</Badge>}
          {product.isBestseller && <Badge variant="bestseller">Хит</Badge>}
          {product.discountPrice && <Badge variant="sale">Скидка</Badge>}
        </div>

        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-5 right-5 w-10 h-10 rounded-full flex items-center justify-center transition-all duration-300 z-10
            ${favorited
              ? 'bg-graphite text-white shadow-md'
              : 'bg-white/85 border border-border-lighter text-graphite hover:bg-white hover:text-primary'
            }`}
        >
          <Heart className={`w-5 h-5 ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      <div className="p-5 flex flex-col flex-grow">
        <div className="mb-auto">
          <p className="text-[10px] font-semibold tracking-[0.14em] text-ash uppercase mb-1.5">{product.brand}</p>
          <Link to={`/product/${product.id}`} className="block group-hover:text-primary transition-colors duration-300">
            <h3 className="text-sm font-medium text-graphite line-clamp-2 leading-relaxed">{product.name}</h3>
          </Link>
        </div>

        <div className="mt-4 flex items-baseline gap-2">
          {product.discountPrice ? (
            <>
              <span className="text-base font-bold text-error">{formatPrice(product.discountPrice)}</span>
              <span className="text-xs text-ash-light line-through decoration-ash-light/50">{formatPrice(product.price)}</span>
            </>
          ) : (
            <span className="text-base font-bold text-graphite">{formatPrice(product.price)}</span>
          )}
        </div>
      </div>
    </div>
  );
}

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
    <div className="group flex flex-col h-full bg-white rounded-[1.4rem] shadow-[0_2px_12px_rgba(120,148,180,0.08)] hover:shadow-[0_12px_40px_rgba(100,135,175,0.14)] transition-all duration-500 overflow-hidden ring-1 ring-border-lighter/60 hover:-translate-y-0.5">

      {/* Image area */}
      <Link
        to={`/product/${product.id}`}
        className="relative block overflow-hidden bg-gradient-to-b from-[#f0f5fb] to-[#eef3f9]"
        style={{ aspectRatio: '4/5' }}
      >
        <div className="relative w-full h-full overflow-hidden p-2">
          <img
            src={product.images?.[0] ?? product.image}
            alt={product.name}
            className="object-cover w-full h-full rounded-[1.1rem] transition-transform duration-700 ease-out group-hover:scale-[1.04]"
            loading="lazy"
          />
          {/* Hover vignette */}
          <div className="absolute inset-0 bg-gradient-to-t from-[#1b2b4218] via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 rounded-[1.1rem]" />
        </div>

        {/* Badges — top left, stacked cleanly */}
        <div className="absolute top-4 left-4 flex flex-col gap-1.5 z-10">
          {product.isNew && (
            <Badge variant="new" className="text-[10px] font-semibold px-2.5 py-1 shadow-sm backdrop-blur-sm bg-white/92 rounded-full tracking-[0.04em]">
              Новинка
            </Badge>
          )}
          {product.isBestseller && (
            <Badge variant="bestseller" className="text-[10px] font-semibold px-2.5 py-1 shadow-sm backdrop-blur-sm bg-white/92 rounded-full tracking-[0.04em]">
              Хит
            </Badge>
          )}
          {product.discountPrice && (
            <Badge variant="sale" className="text-[10px] font-semibold px-2.5 py-1 shadow-sm backdrop-blur-sm bg-white/92 rounded-full tracking-[0.04em]">
              Скидка
            </Badge>
          )}
        </div>

        {/* Favorite button — top right */}
        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-4 right-4 w-[34px] h-[34px] flex items-center justify-center transition-all duration-400 z-10 rounded-full
            ${favorited
              ? 'bg-white text-error shadow-md scale-105'
              : 'bg-white/70 text-graphite/60 hover:bg-white hover:text-graphite shadow-sm opacity-0 group-hover:opacity-100 -translate-y-1 group-hover:translate-y-0 hover:scale-105'
            }`}
        >
          <Heart className={`w-[15px] h-[15px] ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      {/* Info area */}
      <div className="px-4 pt-3.5 pb-4 md:px-5 md:pt-4 md:pb-5 flex flex-col flex-grow bg-white">
        <div className="mb-auto">
          {/* Brand label */}
          <p className="text-[10px] font-semibold tracking-[0.1em] text-ash uppercase mb-1">
            {product.brand}
          </p>
          {/* Product name */}
          <Link to={`/product/${product.id}`} className="block">
            <h3 className="text-[15.5px] md:text-[16.5px] font-medium text-graphite leading-[1.3] group-hover:text-primary transition-colors duration-300">
              {product.name}
            </h3>
          </Link>
        </div>

        {/* Price row */}
        <div className="mt-3.5 flex items-baseline gap-2">
          {product.discountPrice ? (
            <>
              <span className="text-[20px] md:text-[22px] leading-none font-semibold tracking-tight text-error">
                {formatPrice(product.discountPrice)}
              </span>
              <span className="text-[12px] text-ash/70 line-through decoration-ash-light flex-shrink-0">
                {formatPrice(product.price)}
              </span>
            </>
          ) : (
            <span className="text-[20px] md:text-[22px] leading-none font-semibold tracking-tight text-graphite">
              {formatPrice(product.price)}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

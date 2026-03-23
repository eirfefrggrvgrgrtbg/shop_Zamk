import { Link } from 'react-router-dom';
import { Heart, Star } from 'lucide-react';
import type { Product } from '../../lib/mock-data';
import { formatPrice } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const { toggleFavorite, isFavorite } = useFavorites();
  const favorited = isFavorite(product.id);

  // Вычисляем процент скидки
  const discountPercent = product.discountPrice
    ? Math.round((1 - product.discountPrice / product.price) * 100)
    : 0;

  return (
    <div className="relative group flex flex-col h-full bg-white dark:bg-[#1a1a1c] rounded-xl shadow-sm hover:shadow-lg transition-all duration-300 overflow-hidden border border-border-lighter dark:border-white/10">

      {/* Image area */}
      <Link
        to={`/product/${product.id}`}
        className="relative block overflow-hidden bg-[#f5f5f7] dark:bg-[#2a2a2c]"
        style={{ aspectRatio: '3/4' }}
      >
        <img
          src={product.images?.[0] ?? product.image}
          alt={product.name}
          className="object-contain w-full h-full p-2 transition-transform duration-500 ease-out group-hover:scale-105"
          loading="lazy"
        />

        {/* Badge — только один, приоритет: Новинка > Хит > Скидка */}
        <div className="absolute top-2.5 left-2.5 z-10">
          {product.isNew ? (
            <span className="inline-flex items-center gap-1 px-2 py-1 rounded bg-graphite text-white text-[10px] font-semibold uppercase">
              New
            </span>
          ) : product.isBestseller ? (
            <span className="px-2 py-1 rounded bg-amber-500 text-white text-[10px] font-semibold uppercase">
              Хит
            </span>
          ) : product.discountPrice ? (
            <span className="px-2 py-1 rounded bg-red-500 text-white text-[10px] font-semibold uppercase">
              -{discountPercent}%
            </span>
          ) : null}
        </div>

        {/* Favorite button */}
        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-2.5 right-2.5 w-8 h-8 flex items-center justify-center transition-all duration-300 z-10 rounded-full
            ${favorited
              ? 'bg-white text-red-500 shadow-md'
              : 'bg-white/80 text-graphite/50 hover:text-graphite shadow-sm opacity-0 group-hover:opacity-100'
            }`}
        >
          <Heart className={`w-4 h-4 ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      {/* Info area */}
      <div className="p-3 flex flex-col flex-grow">
        {/* Brand */}
        <p className="text-[10px] font-medium tracking-wide text-ash uppercase mb-1">
          {product.brand}
        </p>

        {/* Name */}
        <Link to={`/product/${product.id}`} className="block flex-grow">
          <h3 className="text-[13px] font-medium text-graphite dark:text-white leading-snug line-clamp-2 hover:text-primary transition-colors">
            {product.name}
          </h3>
        </Link>

        {/* Rating */}
        {product.rating && (
          <div className="flex items-center gap-1 mt-2">
            <div className="flex items-center">
              {[1, 2, 3, 4, 5].map((star) => (
                <Star
                  key={star}
                  className={`w-3 h-3 ${
                    star <= Math.round(product.rating!)
                      ? 'fill-amber-400 text-amber-400'
                      : 'fill-gray-200 text-gray-200 dark:fill-gray-600 dark:text-gray-600'
                  }`}
                />
              ))}
            </div>
            <span className="text-[11px] text-ash ml-1">
              {product.rating.toFixed(1)}
            </span>
            {product.reviewsCount && (
              <span className="text-[11px] text-ash">
                ({product.reviewsCount})
              </span>
            )}
          </div>
        )}

        {/* Price */}
        <div className="mt-2 flex items-baseline gap-2">
          {product.discountPrice ? (
            <>
              <span className="text-[15px] font-semibold text-red-500">
                {formatPrice(product.discountPrice)}
              </span>
              <span className="text-[12px] text-ash line-through">
                {formatPrice(product.price)}
              </span>
            </>
          ) : (
            <span className="text-[15px] font-semibold text-graphite dark:text-white">
              {formatPrice(product.price)}
            </span>
          )}
        </div>

        {/* Sizes preview */}
        {product.sizes && product.sizes.length > 0 && product.sizes[0] !== 'Единый' && (
          <div className="mt-2 flex items-center gap-1 text-[10px] text-ash">
            {product.sizes.slice(0, 4).map((size) => (
              <span key={size} className="px-1.5 py-0.5 rounded border border-border-lighter dark:border-white/20">
                {size}
              </span>
            ))}
            {product.sizes.length > 4 && (
              <span>+{product.sizes.length - 4}</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

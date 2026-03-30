import { Link } from 'react-router-dom';
import { Heart, Star } from 'lucide-react';
import type { Product } from '../../lib/mock-data';
import { formatPrice } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';

interface ProductCardProps {
  product: Product;
}

// Вспомогательная функция для генерации мета-информации
function getMetaInfo(category: string): string {
  switch (category) {
    case 'jewelry':
      return 'Серебро 925';
    case 'bags':
      return 'Натуральная кожа';
    case 'accessories':
      return 'One size';
    default:
      return 'One size';
  }
}

export function ProductCard({ product }: ProductCardProps) {
  const { toggleFavorite, isFavorite } = useFavorites();
  const favorited = isFavorite(product.id);

  // Вычисляем процент скидки
  const discountPercent = product.discountPrice
    ? Math.round((1 - product.discountPrice / product.price) * 100)
    : 0;

  const hasSizes = product.sizes && product.sizes.length > 0 && product.sizes[0] !== 'Единый';
  const displayRating = product.rating || 0;
  const displayReviewsCount = product.reviewsCount || 0;

  return (
    <div className="relative group flex flex-col h-full overflow-hidden transition-all duration-600 ease-out hover:-translate-y-0.5">

      {/* Image area - отделённая зона с собственным фоном */}
      <Link
        to={`/product/${product.id}`}
        className="relative block overflow-hidden bg-[#f4f4f5] dark:bg-[#232325] rounded-t-[8px]"
        style={{ aspectRatio: '3/4' }}
      >
        <img
          src={product.images?.[0] ?? product.image}
          alt={product.name}
          className="object-contain w-full h-full p-5 transition-transform duration-800 ease-out group-hover:scale-[1.02]"
          loading="lazy"
        />

        {/* Badge — минималистичный */}
        <div className="absolute top-4 left-4 z-10">
          {product.isNew ? (
            <span className="inline-flex items-center justify-center px-2 py-0.5 rounded-[4px] bg-graphite text-white dark:text-white text-[9px] font-medium uppercase tracking-[0.08em]">
              New
            </span>
          ) : product.isBestseller ? (
            <span className="inline-flex items-center justify-center px-2 py-0.5 rounded-[4px] bg-graphite/80 text-white text-[9px] font-medium uppercase tracking-[0.08em]">
              Хит
            </span>
          ) : product.discountPrice ? (
            <span className="inline-flex items-center justify-center px-2 py-0.5 rounded-[4px] bg-graphite/80 text-white text-[9px] font-medium uppercase tracking-[0.08em]">
              -{discountPercent}%
            </span>
          ) : null}
        </div>

        {/* Favorite button - ещё деликатнее */}
        <button
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-4 right-4 w-8 h-8 flex items-center justify-center transition-all duration-400 z-10 rounded-full
            ${favorited
              ? 'bg-white text-red-500'
              : 'bg-white/90 text-graphite/30 hover:text-graphite/60 opacity-0 group-hover:opacity-100'
            }`}
        >
          <Heart className={`w-[14px] h-[14px] ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      {/* Info area - чистый белый контентный блок */}
      <div className="px-4 pt-5 pb-4 lg:px-5 lg:pt-6 lg:pb-5 flex flex-col flex-grow bg-white dark:bg-[#1a1a1c] rounded-b-[8px] border border-t-0 border-gray-100 dark:border-white/[0.04]">

        {/* Brand */}
        <p className="text-[8px] font-normal tracking-[0.14em] text-ash/50 uppercase truncate mb-3 flex-none">
          {product.brand}
        </p>

        {/* Name */}
        <Link to={`/product/${product.id}`} className="block flex-none mb-4 group/name">
          <h3 className="line-clamp-2 min-h-[44px] text-[14px] font-normal text-graphite dark:text-white leading-[22px] tracking-[-0.005em] group-hover/name:text-ash transition-colors duration-400">
            {product.name}
          </h3>
        </Link>

        {/* Rating */}
        <div className="flex items-center gap-2 flex-none mb-6 h-[12px]">
          <div className="flex items-center gap-[2px]">
            {[1, 2, 3, 4, 5].map((star) => (
              <Star
                key={star}
                className={`w-[8px] h-[8px] ${
                  star <= Math.round(displayRating)
                    ? 'fill-graphite/40 text-graphite/40 dark:fill-white/40 dark:text-white/40'
                    : 'fill-gray-200 text-gray-200 dark:fill-white/[0.06] dark:text-white/[0.06]'
                }`}
              />
            ))}
          </div>
          <span className="text-[8px] text-ash/40 tracking-wide">
            {displayRating > 0 ? displayRating.toFixed(1) : '—'} · {displayReviewsCount} отзывов
          </span>
        </div>

        {/* Spacer */}
        <div className="flex-grow"></div>

        {/* Price */}
        <div className="flex items-baseline gap-3 h-[32px] flex-none mb-5">
          {product.discountPrice ? (
            <>
              <span className="text-[24px] font-normal text-graphite dark:text-white leading-none tracking-[-0.03em]">
                {formatPrice(product.discountPrice)}
              </span>
              <span className="text-[12px] text-ash/30 line-through leading-none">
                {formatPrice(product.price)}
              </span>
            </>
          ) : (
            <span className="text-[24px] font-normal text-graphite dark:text-white leading-none tracking-[-0.03em]">
              {formatPrice(product.price)}
            </span>
          )}
        </div>

        {/* Sizes / Meta */}
        <div className="flex items-center min-h-[22px] flex-none pt-4 border-t border-gray-100/80 dark:border-white/[0.04]">
          {hasSizes ? (
            <div className="flex items-center gap-2">
              {product.sizes!.slice(0, 4).map((size, index) => (
                <span key={size} className="flex items-center">
                  <span className="text-[10px] font-normal text-ash/50 dark:text-white/50 tracking-wide">
                    {size}
                  </span>
                  {index < Math.min(product.sizes!.length, 4) - 1 && (
                    <span className="ml-2 text-ash/20 dark:text-white/10">·</span>
                  )}
                </span>
              ))}
              {product.sizes!.length > 4 && (
                <span className="text-[10px] text-ash/35">+{product.sizes!.length - 4}</span>
              )}
            </div>
          ) : (
            <span className="text-[10px] font-normal text-ash/45 tracking-[0.04em]">
              {getMetaInfo(product.category)}
            </span>
          )}
        </div>

      </div>
    </div>
  );
}

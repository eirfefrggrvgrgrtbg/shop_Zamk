import { Link } from 'react-router-dom';
import { Heart, Star } from 'lucide-react';
import type { Product } from '../../types/catalog';
import { formatPrice } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';
import { useAuth } from '../../contexts/AuthContext';
import { useToast } from '../../contexts/ToastContext';

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const { user } = useAuth();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();
  const favorited = isFavorite(product.id);

  // Вычисляем процент скидки
  const discountPercent = product.discountPrice
    ? Math.round((1 - product.discountPrice / product.price) * 100)
    : 0;

  const displayRating = product.rating ?? 0;
  const displayReviewsCount = product.reviewsCount ?? 0;

  return (
    <div className="group relative flex flex-col items-center w-full transition-all duration-500 hover:-translate-y-2">
      {/* Леска и бирка NEW */}
      {product.isNew && (
        <div className="absolute top-0 left-8 z-20 flex flex-col items-center origin-top transition-transform duration-500 group-hover:-rotate-6 group-hover:-translate-x-1">
          {/* Леска */}
          <div className="w-[1px] h-16 bg-black/60 dark:bg-white/40 shadow-sm"></div>
          {/* Бирка */}
          <div className="px-2 py-1 bg-black text-white dark:bg-white dark:text-black text-[9px] uppercase font-bold tracking-widest shadow-md transform -rotate-[6deg] -ml-1 mt-[-2px]">
            Новинка
          </div>
        </div>
      )}

      {/* Леска и бирка ХИТ */}
      {product.isBestseller && !product.isNew && (
        <div className="absolute top-0 left-8 z-20 flex flex-col items-center origin-top transition-transform duration-500 group-hover:-rotate-6 group-hover:-translate-x-1">
          {/* Леска */}
          <div className="w-[1px] h-16 bg-black/60 dark:bg-white/40 shadow-sm"></div>
          {/* Бирка (белая, как на фото) */}
          <div className="px-2 py-1 bg-white text-black dark:bg-black dark:text-white border border-gray-200 dark:border-zinc-700 text-[9px] uppercase font-bold tracking-widest shadow-md transform -rotate-[6deg] -ml-1 mt-[-2px]">
            Хит
          </div>
        </div>
      )}

      {/* Фото Polaroid */}
      <div
        className="relative w-full bg-white/5 dark:bg-zinc-800/5 backdrop-blur-xl p-3 pb-8 shadow-sm hover:shadow-lg dark:shadow-none rounded-2xl border border-white/20 dark:border-white/5 transition-shadow"
      >
        <div className="relative w-full aspect-[3/4] overflow-hidden rounded-[12px] bg-white/5 dark:bg-zinc-900/10 border border-white/10 dark:border-white/5">
          <Link
            to={`/product/${product.id}`}
            className="block w-full h-full"
          >
            <img
              src={product.images?.[0] ?? product.image}
              alt={product.name}
              className="w-full h-full object-cover transition-transform duration-700 ease-out group-hover:scale-105"
              loading="lazy"
            />
          </Link>

          {/* Доп бейджи (скидка) */}
          <div className="absolute top-3 right-3 flex flex-col gap-2 z-10 pointer-events-none">
            {product.discountPrice && (
              <span className="inline-flex px-2 py-1 bg-red-500 text-white text-[9px] font-bold uppercase tracking-wider">
                -{discountPercent}%
              </span>
            )}
          </div>

          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              if (!user) {
                showToast('Войдите, чтобы добавить товар в избранное.');
              }
              toggleFavorite(product.id);
            }}
            aria-label={favorited ? 'Убрать из избранного' : 'Добавить в избранное'}
            className={`absolute top-3 left-3 w-8 h-8 flex items-center justify-center transition-all duration-300 z-20 rounded-full backdrop-blur-md ${
              favorited
                ? 'bg-white/90 dark:bg-black/80 text-red-500 shadow-sm'
                : 'bg-white/50 dark:bg-black/30 text-gray-700 dark:text-white/80 hover:bg-white/90 dark:hover:bg-black/70 opacity-0 group-hover:opacity-100'
            }`}
          >
            <Heart className={`w-4 h-4 ${favorited ? 'fill-current' : ''}`} />
          </button>
        </div>
      </div>

      {/* Бумажный чек (Инфо-блок) */}
      <div className="relative z-10 w-[92%] -mt-6 bg-[#f4f4f4] dark:bg-zinc-900 p-4 pt-4 shadow-lg dark:shadow-black/60 border border-gray-200/80 dark:border-zinc-700 flex flex-col transform rotate-1 group-hover:rotate-0 transition-transform duration-500 font-mono">
        
        {/* Контент чека */}
        <div className="flex flex-col text-[11px] leading-relaxed text-black dark:text-gray-100 font-medium">
          <div className="uppercase tracking-widest text-[9px] mb-2 border-b border-dashed border-gray-400 dark:border-zinc-500 pb-2">
          <div className="flex gap-2 items-center text-black/60 dark:text-white/60">
            {product.sellerId ? (
              <Link to={`/seller/${product.sellerId}`} className="hover:underline hover:text-black dark:hover:text-white transition-colors">
                {product.brand}
              </Link>
            ) : (
              <span>{product.brand}</span>
            )}
            <span> | ЧЕК</span>
          </div>
          </div>
          <div className="flex w-full gap-2 items-start mt-1">
            <span className="uppercase whitespace-nowrap opacity-80">Товар:</span>
            <Link to={`/product/${product.id}`} className="group/name flex-1 min-w-0">
              <span className="line-clamp-2 uppercase group-hover/name:underline decoration-1 underline-offset-4 decoration-gray-500 transition-all leading-tight">
                {product.name}
              </span>
            </Link>
          </div>

          <div className="flex gap-2 items-center mt-1 uppercase">
            <span className="opacity-80">Цена:</span>
            {product.discountPrice ? (
              <div className="flex gap-2 items-baseline">
                <span>{formatPrice(product.discountPrice)}</span>
                <span className="line-through text-gray-500 text-[10px]">{formatPrice(product.price)}</span>
              </div>
            ) : (
              <span>{formatPrice(product.price)}</span>
            )}
          </div>
              
          {displayReviewsCount > 0 ? (
            <div className="flex items-center gap-1.5 mt-2">
              <div className="flex text-[11px] tracking-tighter">
                <span className="text-black dark:text-gray-100">
                  {'★'.repeat(Math.round(displayRating))}
                </span>
                <span className="text-gray-300 dark:text-zinc-600">
                  {'★'.repeat(5 - Math.round(displayRating))}
                </span>
              </div>
              <span className="text-[9px] opacity-80 lowercase">
                {displayReviewsCount} отзывов
              </span>
            </div>
          ) : (
            <p className="mt-2 text-[9px] uppercase tracking-widest opacity-60">Нет отзывов</p>
          )}

          {/* Декоративный штрихкод */}
          <div className="flex flex-col items-center w-full mt-4 pt-3 border-t border-dashed border-gray-400 dark:border-zinc-500">
            <div 
              className="w-full h-6 opacity-80 dark:opacity-60 mix-blend-multiply dark:mix-blend-lighten"
              style={{
                backgroundImage: 'repeating-linear-gradient(to right, currentColor 0, currentColor 2px, transparent 2px, transparent 4px, currentColor 4px, currentColor 5px, transparent 5px, transparent 8px, currentColor 8px, currentColor 11px, transparent 11px, transparent 13px, currentColor 13px, currentColor 14px, transparent 14px, transparent 17px)',
                color: 'currentcolor'
              }}
            ></div>
            <p className="text-[8px] uppercase tracking-widest opacity-70 mt-1.5">
              Артикул {String(product.id).split('-').pop()?.padStart(4, '0') || product.id.slice(0, 4)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}



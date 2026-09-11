import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Heart, ShoppingCart } from 'lucide-react';
import type { Product } from '../../types/catalog';
import { formatPrice, cn } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';
import { useAuth } from '../../contexts/AuthContext';
import { useToast } from '../../contexts/ToastContext';
import { useCart } from '../../contexts/CartContext';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { fetchProductById } from '../../api/publicCatalog';
import { useVariantSelection, isLightColor, type ProductVariantItem } from '../../lib/variantSelection';

interface ProductCardProps {
  product: Product;
  previewUrl?: string;
}

export function ProductCard({ product, previewUrl }: ProductCardProps) {
  const { user, openAuthModal } = useAuth();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();
  const { addItem } = useCart();
  const favorited = isFavorite(product.id);
  const cardTargetUrl = previewUrl || `/product/${product.id}`;

  const [isQuickBuyOpen, setIsQuickBuyOpen] = useState(false);
  const [isLoadingVariants, setIsLoadingVariants] = useState(false);
  const [loadedProduct, setLoadedProduct] = useState<Product | null>(null);
  const [isAddingToCart, setIsAddingToCart] = useState(false);

  const discountPercent = product.discountPrice
    ? Math.round((1 - product.discountPrice / product.price) * 100)
    : 0;

  const displayRating = product.rating ?? 0;
  const displayReviewsCount = product.reviewsCount ?? 0;

  const handleQuickBuyClick = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (product.isPreview) return;
    setIsQuickBuyOpen(true);

    if (!loadedProduct) {
      setIsLoadingVariants(true);
      try {
        const fullProduct = await fetchProductById(product.id);
        setLoadedProduct(fullProduct);
      } catch (e) {
        showToast('Ошибка загрузки вариантов товара', 'error');
        setIsQuickBuyOpen(false);
      } finally {
        setIsLoadingVariants(false);
      }
    }
  };

  const handleAddToCart = async (variant: ProductVariantItem) => {
    if (product.isPreview) return;

    if (!variant || !variant.inStock || !variant.isActive) {
      showToast('Выбранный вариант недоступен', 'error');
      setIsQuickBuyOpen(false);
      return;
    }

    if (!user) {
      openAuthModal('login');
      setIsQuickBuyOpen(false);
      return;
    }

    try {
      setIsAddingToCart(true);
      await addItem(product.id, variant.id, 1);
      showToast('Товар добавлен в корзину', 'success');
      setIsQuickBuyOpen(false);
    } catch (error: any) {
      showToast(error.message || 'Ошибка при добавлении в корзину', 'error');
    } finally {
      setIsAddingToCart(false);
    }
  };

  return (
    <>
      <div className="group relative flex flex-col items-center w-full transition-all duration-500 hover:-translate-y-2">
        {/* Фото Polaroid */}
        <div
          className="relative w-full bg-white/5 dark:bg-zinc-800/5 backdrop-blur-xl p-3 pb-8 shadow-sm hover:shadow-lg dark:shadow-none rounded-2xl border border-white/20 dark:border-white/5 transition-shadow"
        >
          <div className="relative w-full aspect-[3/4] overflow-hidden rounded-[12px] bg-white/5 dark:bg-zinc-900/10 border border-white/10 dark:border-white/5">
            <Link
              to={cardTargetUrl}
              className="block w-full h-full"
            >
              <img
                src={product.image || product.images?.[0]?.url || 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image'}
                alt={product.name}
                className="w-full h-full object-cover transition-transform duration-700 ease-out group-hover:scale-105"
                loading="lazy"
              />
            </Link>

            {/* Доп бейджи (скидка) */}
            <div className="absolute top-3 right-3 flex flex-col gap-2 z-10 pointer-events-none">
              {product.discountPrice && (
                <span className="inline-flex px-2 py-1 bg-red-500 text-white text-[9px] font-bold uppercase tracking-wider shadow-sm">
                  -{discountPercent}%
                </span>
              )}
            </div>

            {/* Избранное */}
            {!product.isPreview && (
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  if (!user) {
                    showToast('Войдите, чтобы добавить товар в избранное.', 'error');
                    openAuthModal('login');
                    return;
                  }
                  toggleFavorite(product.id);
                  showToast(favorited ? 'Удалено из избранного' : 'Добавлено в избранное');
                }}
                aria-label={favorited ? 'Убрать из избранного' : 'Добавить в избранное'}
                className="absolute top-3 left-3 w-8 h-8 flex items-center justify-center rounded-full bg-white/80 dark:bg-black/60 backdrop-blur-sm border border-white/20 text-graphite dark:text-white hover:scale-110 transition-all z-10 opacity-0 group-hover:opacity-100"
              >
                <Heart className={cn("w-4 h-4 transition-colors", favorited && "fill-red-500 text-red-500")} />
              </button>
            )}

            {/* Быстрая покупка (корзина) */}
            {!product.isPreview && (
              <button
                type="button"
                onClick={handleQuickBuyClick}
                aria-label="Быстрая покупка"
                className="absolute bottom-3 right-3 w-10 h-10 flex items-center justify-center transition-all duration-300 z-20 rounded-full backdrop-blur-md bg-white/70 dark:bg-black/50 text-graphite dark:text-white hover:bg-white dark:hover:bg-black hover:scale-110 shadow-sm opacity-0 group-hover:opacity-100 translate-y-2 group-hover:translate-y-0"
              >
                <ShoppingCart className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        {/* Бумажный чек (Инфо-блок) */}
        <div className="relative z-10 w-[92%] -mt-6 bg-[#f4f4f4] dark:bg-zinc-900 p-4 pt-4 shadow-lg dark:shadow-black/60 border border-gray-200/80 dark:border-zinc-700 flex flex-col transform rotate-1 group-hover:rotate-0 transition-transform duration-500 font-mono">

          {/* Контент чека */}
          <div className="flex flex-col text-[11px] leading-relaxed text-black dark:text-gray-100 font-medium">
            <div className="uppercase tracking-widest text-[9px] mb-2 border-b border-dashed border-gray-400 dark:border-zinc-500 pb-2">
              <div className="flex gap-2 items-center text-black/60 dark:text-white/60">
                {product.sellerSlug && !product.isPreview ? (
                  <Link to={`/seller/${product.sellerSlug}`} className="hover:underline hover:text-black dark:hover:text-white transition-colors">
                    {product.sellerName || product.brand}
                  </Link>
                ) : (
                  <span>{product.sellerName || product.brand}</span>
                )}
                <span>| ЧЕК</span>
              </div>
            </div>

            <div className="flex w-full gap-2 items-start mt-1">
              <span className="uppercase whitespace-nowrap opacity-80">Товар:</span>
              <Link to={cardTargetUrl} className="group/name flex-1 min-w-0">
                <span className="line-clamp-2 uppercase group-hover/name:underline decoration-1 underline-offset-4 decoration-gray-500 transition-all leading-tight">
                  {product.name}
                </span>
              </Link>
            </div>

            <div className="flex gap-2 items-center mt-1 uppercase">
              <span className="opacity-80">Цена:</span>
              {product.discountPrice ? (
                <div className="flex gap-2 items-baseline text-red-600 dark:text-red-400">
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

      <Modal isOpen={isQuickBuyOpen} onClose={() => setIsQuickBuyOpen(false)} title="Быстрая покупка">
        {isLoadingVariants ? (
          <div className="py-8 flex justify-center">
            <div className="w-6 h-6 border-2 border-black border-t-transparent rounded-full animate-spin dark:border-white dark:border-t-transparent" />
          </div>
        ) : loadedProduct && loadedProduct.variants && loadedProduct.variants.length > 0 ? (
          <QuickBuyModalBody
            product={loadedProduct}
            onAddToCart={handleAddToCart}
            isAddingToCart={isAddingToCart}
          />
        ) : (
          <div className="py-8 text-center">
            <p className="text-ash mb-4">Варианты недоступны</p>
          </div>
        )}
      </Modal>
    </>
  );
}

interface QuickBuyModalBodyProps {
  product: Product;
  onAddToCart: (variant: ProductVariantItem) => Promise<void>;
  isAddingToCart: boolean;
}

function QuickBuyModalBody({
  product,
  onAddToCart,
  isAddingToCart,
}: QuickBuyModalBodyProps) {
  const {
    dimensionType,
    colors,
    selectedColorId,
    selectedColor,
    sizes,
    selectedSizeId,
    selectedSize,
    selectedVariant,
    isResolved,
    canAddToCart,
    requiresColor,
    requiresSize,
    ctaText,
    selectColor,
    selectSize,
  } = useVariantSelection(product.variants, product.sizeChart);

  const colorImage = selectedColorId && product.images
    ? product.images.find(img => img.colorId === selectedColorId)?.url
    : undefined;

  const displayImage = colorImage || product.image || product.images?.[0]?.url || 'https://placehold.co/400x500';
  const displayPrice = selectedVariant?.priceCents ? selectedVariant.priceCents / 100 : product.price;

  return (
    <div>
      <div className="flex gap-4 mb-6">
        <img
          src={displayImage}
          alt={product.name}
          className="w-24 h-32 object-cover rounded-lg"
        />
        <div>
          <h3 className="font-serif text-lg leading-tight text-graphite dark:text-white mb-2">{product.name}</h3>
          {product.discountPrice ? (
            <div className="flex gap-2 items-baseline">
              <span className="font-mono text-red-500">{formatPrice(product.discountPrice)}</span>
              <span className="line-through text-gray-500 text-sm font-mono">{formatPrice(displayPrice)}</span>
            </div>
          ) : (
            <span className="font-mono text-graphite dark:text-white">{formatPrice(displayPrice)}</span>
          )}
        </div>
      </div>

      <div className="flex flex-col gap-5">
        {/* Colors */}
        {requiresColor && colors.length > 0 && (
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-mono uppercase tracking-widest text-ash">
                Цвет:{' '}
                <span className="text-graphite dark:text-white font-medium">
                  {selectedColor?.name || 'Не выбран'}
                  {selectedColor && !selectedColor.hasInStock ? ' (нет в наличии)' : ''}
                </span>
              </span>
            </div>
            <div className="flex flex-wrap gap-2.5 items-center" role="radiogroup" aria-label="Выбор цвета">
              {colors.map((color) => {
                const isSelected = selectedColorId === color.id;
                const isLight = isLightColor(color.hex);
                return (
                  <button
                    key={color.id}
                    type="button"
                    role="radio"
                    aria-checked={isSelected}
                    aria-label={color.name + (!color.hasInStock ? ' (нет в наличии)' : '')}
                    title={color.name + (!color.hasInStock ? ' (нет в наличии)' : '')}
                    onClick={() => selectColor(color.id)}
                    className={cn(
                      "relative w-8 h-8 rounded-full transition-all duration-150 flex items-center justify-center",
                      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2",
                      isSelected
                        ? "ring-2 ring-graphite dark:ring-white ring-offset-2 ring-offset-white dark:ring-offset-zinc-900 scale-105"
                        : "hover:scale-105 hover:ring-1 hover:ring-black/20 dark:hover:ring-white/30 opacity-90 hover:opacity-100"
                    )}
                  >
                    {color.hex ? (
                      <span
                        style={{ backgroundColor: color.hex }}
                        className={cn(
                          "w-full h-full rounded-full border shadow-inner",
                          isLight
                            ? "border-black/25 dark:border-white/30"
                            : "border-black/10 dark:border-white/15"
                        )}
                      />
                    ) : (
                      <span className="w-full h-full rounded-full border border-black/20 dark:border-white/20 bg-ice dark:bg-white/10 flex items-center justify-center text-[9px] font-semibold text-graphite dark:text-white uppercase">
                        {color.name.slice(0, 2)}
                      </span>
                    )}
                    {!color.hasInStock && (
                      <span
                        aria-hidden="true"
                        className="absolute inset-0 flex items-center justify-center pointer-events-none"
                      >
                        <span className="w-[120%] h-[1.5px] bg-red-500/70 rotate-45 transform" />
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {/* Sizes */}
        {requiresSize && (
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-mono uppercase tracking-widest text-ash">
                Размер: <span className="text-graphite dark:text-white font-medium">{selectedSize?.label || 'Не выбран'}</span>
              </span>
            </div>
            <div className="flex flex-wrap gap-2">
              {sizes.map((sizeObj) => {
                const isSelected = selectedSizeId === sizeObj.id;
                return (
                  <button
                    key={sizeObj.id}
                    type="button"
                    disabled={sizeObj.disabled}
                    onClick={() => {
                      if (!sizeObj.disabled) {
                        selectSize(sizeObj.id);
                      }
                    }}
                    className={cn(
                      "h-9 min-w-[40px] px-3 rounded-md border font-mono text-xs transition-all relative flex items-center justify-center",
                      isSelected
                        ? "bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white shadow-sm"
                        : sizeObj.disabled
                          ? "border-border-lighter/60 dark:border-white/10 text-ash/60 dark:text-white/30 cursor-not-allowed line-through bg-ice/30 dark:bg-white/[0.02]"
                          : "bg-white dark:bg-transparent border-border-lighter dark:border-white/20 text-graphite dark:text-white hover:border-black/50 dark:hover:border-white/50"
                    )}
                  >
                    {sizeObj.label}
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {dimensionType === 'SINGLE_VARIANT' && (
          <p className="text-xs text-ash font-mono">Стандартный размер</p>
        )}

        <Button
          onClick={() => {
            if (selectedVariant && canAddToCart) {
              onAddToCart(selectedVariant);
            }
          }}
          disabled={!isResolved || !canAddToCart || isAddingToCart}
          className="w-full h-12 text-sm uppercase tracking-widest font-mono"
        >
          {isAddingToCart
            ? 'Добавление...'
            : !isResolved
              ? ctaText
              : !canAddToCart
                ? 'Нет в наличии'
                : 'В корзину'}
        </Button>
      </div>
    </div>
  );
}

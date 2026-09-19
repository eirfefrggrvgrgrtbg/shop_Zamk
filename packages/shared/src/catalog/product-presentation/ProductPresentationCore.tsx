import { useState, useRef, useEffect, type ReactNode, type KeyboardEvent, type MouseEvent } from 'react';
import { createPortal } from 'react-dom';
import { ChevronRight, ChevronLeft, Heart, ShoppingBag, ChevronDown, X } from 'lucide-react';
import { cn, formatPrice } from './internal/presentationUtils';
import { lockBodyScroll, unlockBodyScroll } from './internal/presentationScrollLock';
import { PresentationButton } from './internal/PresentationButton';
import { PresentationModal } from './internal/PresentationModal';
import {
  isLightColor,
  isUUID,
  getMeasurementMeta,
  SizeGuideIllustration,
} from './helpers';
import type {
  ProductPresentationCoreProps,
  ProductPresentationCoreProduct,
  ProductPresentationSelectedVariant,
} from './types';

const getProductSpecs = (product: ProductPresentationCoreProduct, selectedVariant?: ProductPresentationSelectedVariant | null) =>
  [
    (selectedVariant?.sellerSku || (product.id && !isUUID(product.id)))
      ? { label: 'Артикул', value: (selectedVariant?.sellerSku || product.id)!.toUpperCase() }
      : (selectedVariant?.sku ? { label: 'Артикул', value: selectedVariant.sku.toUpperCase() } : null),
    product.brand && product.brand !== 'Бренд не указан' && !isUUID(product.brand)
      ? { label: 'Бренд', value: product.brand }
      : null,
    product.category && product.category !== 'Категория не указана' && !isUUID(product.category)
      ? { label: 'Категория', value: product.category }
      : null,
    product.materials ? { label: 'Материал', value: product.materials } : null,
  ].filter((spec): spec is { label: string; value: string } => Boolean(spec));

function AccordionSection({ title, children, defaultOpen = false }: { title: string; children: ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-border-lighter dark:border-white/10">
      <button onClick={() => setIsOpen(!isOpen)} className="w-full py-4 flex items-center justify-between text-left cursor-pointer">
        <span className="text-sm font-medium text-graphite dark:text-white">{title}</span>
        <ChevronDown className={cn("w-4 h-4 text-ash transition-transform", isOpen && "rotate-180")} />
      </button>
      {isOpen && <div className="pb-4">{children}</div>}
    </div>
  );
}

function handleAnchorClick(
  e: MouseEvent<HTMLAnchorElement>,
  onClick?: () => void
) {
  if (!onClick) return;
  // Left-click with no modifier keys -> SPA navigation via callback
  if (e.button === 0 && !e.metaKey && !e.ctrlKey && !e.altKey && !e.shiftKey) {
    e.preventDefault();
    onClick();
  }
}

export function ProductPresentationCore({
  product,
  visibleImages,
  activeImage,
  onActiveImageChange,
  displayPrice,
  colors,
  sizes,
  selectedColorId,
  selectedSizeId,
  selectedColor,
  selectedSize,
  selectedVariant,
  isResolved,
  canAddToCart,
  requiresSize,
  isAddingToCart,
  isProductUnavailable,
  ctaText,
  sizeSelectionNotice,
  refreshErrorNotice,
  sizeError,
  reviewsCountText,
  onColorChange,
  onSizeChange,
  onAddToCart,
  isFavorite,
  onToggleFavorite,
  onScrollToReviews,
  onBrandClick,
  onDeliveryClick,
  onReturnsClick,
  onSellerClick,
}: ProductPresentationCoreProps) {
  const [isLightboxOpen, setIsLightboxOpen] = useState(false);
  const [isZoomed, setIsZoomed] = useState(false);
  const [panOffset, setPanOffset] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const dragStartRef = useRef<{ startX: number; startY: number; initialPanX: number; initialPanY: number }>({
    startX: 0, startY: 0, initialPanX: 0, initialPanY: 0,
  });
  const hasMovedRef = useRef(false);
  const [showSizeChart, setShowSizeChart] = useState(false);
  const mainImageTriggerRef = useRef<HTMLButtonElement | null>(null);
  const lightboxDialogRef = useRef<HTMLDivElement | null>(null);

  const resetZoom = () => {
    setIsZoomed(false);
    setPanOffset({ x: 0, y: 0 });
    setIsDragging(false);
  };

  useEffect(() => {
    if (!isLightboxOpen) {
      resetZoom();
      return;
    }
    const focusTimer = setTimeout(() => {
      lightboxDialogRef.current?.focus();
    }, 0);
    const handleKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        setIsLightboxOpen(false);
        resetZoom();
        mainImageTriggerRef.current?.focus();
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        resetZoom();
        onActiveImageChange((activeImage - 1 + visibleImages.length) % visibleImages.length);
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        resetZoom();
        onActiveImageChange((activeImage + 1) % visibleImages.length);
      } else if (e.key === 'Tab') {
        if (!lightboxDialogRef.current) return;
        const focusable = lightboxDialogRef.current.querySelectorAll<HTMLElement>(
          'button, [tabindex]:not([tabindex="-1"])'
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    lockBodyScroll();
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      clearTimeout(focusTimer);
      unlockBodyScroll();
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isLightboxOpen, visibleImages.length, activeImage, onActiveImageChange]);

  const specs = getProductSpecs(product, selectedVariant);
  const currentActiveImage = activeImage < visibleImages.length ? activeImage : 0;
  const currentImageUrl = visibleImages[currentActiveImage]?.url || 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image';
  const activeFields: string[] = product.sizeChart?.rows?.[0]?.measurements
    ? Object.keys(product.sizeChart.rows[0].measurements)
    : [];

  const handlePrevImage = () => {
    if (visibleImages.length <= 1) return;
    resetZoom();
    onActiveImageChange((activeImage - 1 + visibleImages.length) % visibleImages.length);
  };
  const handleNextImage = () => {
    if (visibleImages.length <= 1) return;
    resetZoom();
    onActiveImageChange((activeImage + 1) % visibleImages.length);
  };
  const handleGalleryKeyDown = (e: React.KeyboardEvent) => {
    if (visibleImages.length <= 1) return;
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      handlePrevImage();
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      handleNextImage();
    }
  };

  return (
    <>
      <div className="grid grid-cols-1 min-[960px]:grid-cols-[1fr_420px] lg:grid-cols-[1fr_460px] min-[1200px]:grid-cols-[704px_500px] gap-6 lg:gap-8 min-[1200px]:gap-14 items-start">
        {/* LEFT GALLERY: Up to ~704px on desktop */}
        <div
          className="w-full flex flex-col min-[960px]:flex-row gap-4 min-[1200px]:gap-6 items-start outline-none"
          tabIndex={visibleImages.length > 1 ? 0 : undefined}
          onKeyDown={handleGalleryKeyDown}
          aria-label="Галерея товара"
        >
          {/* Vertical Thumbnail Rail (Desktop) */}
          {visibleImages.length > 1 && (
            <div className="hidden min-[960px]:flex flex-col gap-2.5 w-16 min-[1200px]:w-[72px] flex-shrink-0 max-h-[680px] overflow-y-auto scrollbar-none">
              {visibleImages.map((image, index) => {
                const isSelected = currentActiveImage === index;
                return (
                  <button
                    key={index + image.url}
                    type="button"
                    data-testid={`pdp-thumbnail-${index}`}
                    onClick={() => onActiveImageChange(index)}
                    aria-label={`Фото ${index + 1}`}
                    className={cn(
                      "w-16 h-20 min-[1200px]:w-[72px] min-[1200px]:h-[90px] flex-shrink-0 rounded-lg overflow-hidden bg-[#f5f5f7] dark:bg-[#1a1a1c] transition-all p-1 flex items-center justify-center cursor-pointer",
                      isSelected
                        ? "border border-graphite dark:border-white ring-1 ring-graphite/20 dark:ring-white/20 opacity-100"
                        : "border border-transparent hover:border-border-soft dark:hover:border-white/20 opacity-70 hover:opacity-100"
                    )}
                  >
                    <img
                      src={image.url}
                      alt=""
                      className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal"
                    />
                  </button>
                );
              })}
              {visibleImages.length > 7 && (
                <span className="text-[10px] text-ash text-center font-medium pt-0.5">
                  +{visibleImages.length - 7}
                </span>
              )}
            </div>
          )}

          {/* Main Stage */}
          <div className="flex-1 w-full max-w-[520px] mx-auto min-[960px]:mx-0">
            <div className="group relative bg-[#f5f5f7] dark:bg-[#1a1a1c] border border-black/5 dark:border-white/5 rounded-xl overflow-hidden w-full aspect-[4/5] max-h-[640px] flex items-center justify-center select-none">
              <img
                key={currentImageUrl}
                src={currentImageUrl}
                alt={product.name}
                data-testid="main-product-image"
                className="w-full h-full object-contain mix-blend-multiply dark:mix-blend-normal transition-opacity duration-200 pointer-events-none"
              />

              {visibleImages.length > 1 ? (
                <div className="absolute inset-0 flex">
                  <button
                    type="button"
                    onClick={handlePrevImage}
                    aria-label="Предыдущее фото"
                    className="w-[28%] h-full cursor-w-resize relative flex items-center justify-start pl-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                  >
                    <span className="w-9 h-9 rounded-full bg-white/80 dark:bg-black/70 text-graphite dark:text-white shadow-sm flex items-center justify-center opacity-0 group-hover:opacity-90 transition-opacity backdrop-blur-xs pointer-events-none">
                      <ChevronLeft className="w-5 h-5 -ml-0.5" />
                    </span>
                  </button>

                  <button
                    ref={mainImageTriggerRef}
                    type="button"
                    onClick={() => setIsLightboxOpen(true)}
                    aria-label="Открыть изображение на полный экран"
                    className="w-[44%] h-full cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                  />

                  <button
                    type="button"
                    onClick={handleNextImage}
                    aria-label="Следующее фото"
                    className="w-[28%] h-full cursor-e-resize relative flex items-center justify-end pr-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                  >
                    <span className="w-9 h-9 rounded-full bg-white/80 dark:bg-black/70 text-graphite dark:text-white shadow-sm flex items-center justify-center opacity-0 group-hover:opacity-90 transition-opacity backdrop-blur-xs pointer-events-none">
                      <ChevronRight className="w-5 h-5 -mr-0.5" />
                    </span>
                  </button>
                </div>
              ) : (
                <button
                  ref={mainImageTriggerRef}
                  type="button"
                  onClick={() => setIsLightboxOpen(true)}
                  aria-label="Открыть изображение на полный экран"
                  className="absolute inset-0 w-full h-full cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                />
              )}

              {visibleImages.length > 1 && (
                <div className="absolute bottom-3 right-3 px-2.5 py-1 rounded-md bg-black/60 backdrop-blur-xs text-white text-[11px] font-medium tracking-wider tabular-nums pointer-events-none">
                  {currentActiveImage + 1} / {visibleImages.length}
                </div>
              )}

              {product.isNew && (
                <span className="absolute top-3.5 left-3.5 px-2.5 py-0.5 rounded bg-graphite text-white dark:bg-white dark:text-black text-[11px] font-semibold uppercase tracking-wider shadow-xs pointer-events-none">
                  Новинка
                </span>
              )}
              {product.discountPrice && (
                <span className="absolute top-3.5 right-3.5 px-2.5 py-0.5 rounded bg-red-600 text-white text-[11px] font-semibold uppercase tracking-wider shadow-xs pointer-events-none">
                  -{Math.round((1 - product.discountPrice / product.price) * 100)}%
                </span>
              )}
            </div>

            {/* Horizontal Thumbnails (Mobile / < 960px) */}
            {visibleImages.length > 1 && (
              <div className="flex min-[960px]:hidden gap-2 overflow-x-auto pt-3 pb-1 scrollbar-none">
                {visibleImages.map((image, index) => {
                  const isSelected = currentActiveImage === index;
                  return (
                    <button
                      key={index + image.url}
                      type="button"
                      onClick={() => onActiveImageChange(index)}
                      aria-label={`Фото ${index + 1}`}
                      className={cn(
                        "w-14 h-[70px] flex-shrink-0 rounded-lg overflow-hidden bg-[#f5f5f7] dark:bg-[#1a1a1c] transition-all p-1 flex items-center justify-center cursor-pointer",
                        isSelected
                          ? "border border-graphite dark:border-white ring-1 ring-graphite/20 dark:ring-white/20 opacity-100"
                          : "border border-transparent hover:border-border-soft dark:hover:border-white/20 opacity-70 hover:opacity-100"
                      )}
                    >
                      <img
                        src={image.url}
                        alt=""
                        className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal"
                      />
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {/* RIGHT PURCHASE COLUMN: ~456px */}
        <div className="w-full flex flex-col">
          {/* BRAND */}
          {product.brand && product.brand !== 'Бренд не указан' && !isUUID(product.brand) ? (
            product.brandId ? (
              <a
                href={`/brand/${product.brandId}`}
                data-testid="product-brand-link"
                onClick={(e) => handleAnchorClick(e, () => product.brandId && onBrandClick?.(product.brandId))}
                className="text-xs font-semibold uppercase tracking-widest text-ash hover:text-graphite dark:hover:text-white transition-colors w-fit"
              >
                {product.brand}
              </a>
            ) : (
              <span data-testid="product-brand-text" className="text-xs font-semibold uppercase tracking-widest text-ash">
                {product.brand}
              </span>
            )
          ) : null}

          {/* TITLE */}
          <h1 className="text-[26px] sm:text-[28px] min-[1200px]:text-[32px] leading-[32px] sm:leading-[34px] min-[1200px]:leading-[38px] font-serif text-graphite dark:text-white font-normal mt-1">
            {product.name}
          </h1>

          {/* COMPACT RATING */}
          {product.rating ? (
            <div className="flex items-center gap-1.5 mt-2 text-xs sm:text-sm text-graphite dark:text-white">
              <span className="text-amber-500 text-xs">★</span>
              <span className="font-medium">{product.rating.toFixed(1).replace('.', ',')}</span>
              {reviewsCountText && (
                <>
                  <span className="text-ash">·</span>
                  <button
                    type="button"
                    onClick={onScrollToReviews}
                    className="text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                  >
                    {reviewsCountText}
                  </button>
                </>
              )}
            </div>
          ) : null}

          {/* PRICE */}
          <div className="mt-3.5 flex items-baseline gap-3">
            {product.discountPrice ? (
              <>
                <span className="text-[26px] min-[1200px]:text-[28px] font-semibold text-red-600 dark:text-red-400">
                  {formatPrice(product.discountPrice)}
                </span>
                <span className="text-base text-ash line-through">
                  {formatPrice(displayPrice)}
                </span>
              </>
            ) : (
              <span className="text-[26px] min-[1200px]:text-[28px] font-semibold text-graphite dark:text-white">
                {formatPrice(displayPrice)}
              </span>
            )}
          </div>

          {/* COLORS */}
          {colors.length > 0 && (
            <div className="mt-5 sm:mt-6">
              <div className="flex items-center justify-between mb-2">
                <p className="text-sm text-graphite dark:text-white">
                  <span className="text-ash">Цвет:</span>{' '}
                  <span className="font-medium">
                    {selectedColor?.name || 'Не выбран'}
                    {selectedColor?.shadeName ? ` (${selectedColor.shadeName})` : ''}
                  </span>
                  {selectedColor && !selectedColor.hasInStock && (
                    <span className="text-error text-xs ml-1.5 font-normal">(нет в наличии)</span>
                  )}
                </p>
              </div>
              <div className="flex flex-wrap gap-2.5 items-center" role="radiogroup" aria-label="Выбор цвета">
                {colors.map((color) => {
                  const isSelected = selectedColorId === color.id;
                  const isWhiteOrLight = isLightColor(color.hex);
                  return (
                    <button
                      key={color.id}
                      type="button"
                      role="radio"
                      data-testid={`color-swatch-${color.id}`}
                      aria-checked={isSelected}
                      aria-label={color.name + (color.shadeName ? ` (${color.shadeName})` : '') + (!color.hasInStock ? ' (нет в наличии)' : '')}
                      title={color.name + (color.shadeName ? ` (${color.shadeName})` : '') + (!color.hasInStock ? ' (нет в наличии)' : '')}
                      onClick={() => onColorChange(color.id)}
                      className={cn(
                        "relative w-11 h-11 rounded-full flex items-center justify-center transition-colors cursor-pointer",
                        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-graphite focus-visible:ring-offset-2",
                        "hover:bg-ice/70 dark:hover:bg-white/5",
                        isSelected && "ring-1 ring-graphite dark:ring-white ring-offset-[3px] ring-offset-white dark:ring-offset-[#121214]"
                      )}
                    >
                      {color.hex ? (
                        <span
                          style={{ backgroundColor: color.hex }}
                          className={cn(
                            "w-7 h-7 rounded-full transition-transform",
                            isWhiteOrLight
                              ? "border border-black/25 dark:border-white/30"
                              : "border border-black/10 dark:border-white/15"
                          )}
                        />
                      ) : (
                        <span className="w-7 h-7 rounded-full border border-border-soft dark:border-white/20 bg-ice dark:bg-white/10 flex items-center justify-center text-[10px] font-semibold text-graphite dark:text-white uppercase">
                          {color.name.slice(0, 2)}
                        </span>
                      )}
                      {!color.hasInStock && (
                        <span
                          aria-hidden="true"
                          className="absolute inset-0 flex items-center justify-center pointer-events-none"
                        >
                          <span className="w-6 h-[1.5px] bg-red-500/70 rotate-45 transform" />
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* SIZES */}
          {requiresSize && (
            <div className="mt-5 sm:mt-6">
              <div className="flex items-center gap-3 mb-2 flex-wrap">
                <p className="text-sm text-graphite dark:text-white">
                  <span className="text-ash">Размер:</span>{' '}
                  <span className="font-medium">{selectedSize?.label || 'Не выбран'}</span>
                </p>
                <button
                  type="button"
                  onClick={() => setShowSizeChart(true)}
                  className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                >
                  Таблица размеров
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                {sizes.map((sizeObj) => {
                  const isSelected = selectedSizeId === sizeObj.id;
                  const isSoldOut = sizeObj.state === 'SOLD_OUT';
                  const isNotOffered = sizeObj.state === 'NOT_OFFERED';
                  const isButtonDisabled = isProductUnavailable || Boolean(sizeObj.disabled);
                  return (
                    <button
                      key={sizeObj.id}
                      type="button"
                      data-testid={`size-button-${sizeObj.label}`}
                      aria-pressed={isSelected}
                      aria-label={sizeObj.accessibleLabel || sizeObj.label}
                      title={sizeObj.accessibleLabel || sizeObj.label}
                      data-state={sizeObj.state}
                      disabled={isButtonDisabled}
                      onClick={() => {
                        if (!isButtonDisabled) {
                          onSizeChange(sizeObj.id);
                        }
                      }}
                      className={cn(
                        "min-w-[48px] h-11 px-3.5 rounded-md text-sm font-medium transition-colors relative flex items-center justify-center cursor-pointer",
                        isSelected
                          ? "bg-graphite text-white dark:bg-white dark:text-black border border-graphite dark:border-white shadow-xs"
                          : isSoldOut
                            ? "border border-border-lighter/60 dark:border-white/10 text-ash/50 dark:text-white/30 cursor-not-allowed line-through bg-ice/30 dark:bg-white/[0.02]"
                            : isNotOffered
                              ? "border border-dashed border-border-lighter/70 dark:border-white/10 text-ash/35 dark:text-white/20 cursor-not-allowed bg-transparent"
                              : "bg-white dark:bg-transparent border border-border-soft dark:border-white/20 text-graphite dark:text-white hover:bg-[#fafafb] dark:hover:bg-white/5 hover:border-graphite/40 dark:hover:border-white/40"
                      )}
                    >
                      {sizeObj.label}
                    </button>
                  );
                })}
              </div>
              {sizeSelectionNotice && !isProductUnavailable && (
                <p data-testid="size-selection-notice" className="mt-2 text-xs sm:text-sm text-amber-600 dark:text-amber-400 font-normal" role="status" aria-live="polite">
                  {sizeSelectionNotice}
                </p>
              )}
              {sizeError && !isProductUnavailable && (
                <p className="mt-2 text-xs sm:text-sm text-error" role="alert">
                  {sizeError}
                </p>
              )}
            </div>
          )}

          {/* Product-level unavailability notice */}
          {isProductUnavailable ? (
            <p className="mt-4 text-xs sm:text-sm text-amber-600 dark:text-amber-400 font-normal" role="status" aria-live="polite">
              {'Товар только что закончился.'}
            </p>
          ) : !requiresSize && sizeSelectionNotice ? (
            <p data-testid="size-selection-notice" className="mt-4 text-xs sm:text-sm text-amber-600 dark:text-amber-400 font-normal" role="status" aria-live="polite">
              {sizeSelectionNotice}
            </p>
          ) : refreshErrorNotice ? (
            <p className="mt-4 text-xs sm:text-sm text-amber-600 dark:text-amber-400 font-normal" role="status" aria-live="polite">
              {refreshErrorNotice}
            </p>
          ) : null}

          {/* PRIMARY CTA + FAVORITE */}
          <div className="mt-6 flex gap-3">
            <PresentationButton
              type="button"
              variant="primary"
              data-testid="add-to-cart-button"
              className="flex-1 h-[52px] rounded-lg text-sm font-medium tracking-wide gap-2"
              onClick={onAddToCart}
              disabled={product.isPreview || isAddingToCart || isProductUnavailable || !isResolved || !canAddToCart}
            >
              <ShoppingBag className="w-4 h-4" />
              {product.isPreview
                ? 'Покупка недоступна в предпросмотре'
                : isProductUnavailable
                  ? 'Товар закончился'
                  : isAddingToCart
                    ? 'Добавление...'
                    : ctaText}
            </PresentationButton>
            <PresentationButton
              type="button"
              variant="secondary"
              size="icon"
              className="h-[52px] w-[52px] shrink-0 rounded-lg border border-border-soft dark:border-white/20 hover:border-graphite/40 dark:hover:border-white/40"
              disabled={product.isPreview}
              aria-label={isFavorite ? 'Убрать из избранного' : 'Добавить в избранное'}
              onClick={() => { if (!product.isPreview) onToggleFavorite(); }}
            >
              <Heart className={cn("w-5 h-5 transition-colors", isFavorite && "fill-current text-red-500 stroke-red-500")} />
            </PresentationButton>
          </div>

          {/* COMPACT TEXTUAL SERVICE ROWS */}
          <div className="mt-6 pt-5 border-t border-border-lighter dark:border-white/10 space-y-2.5 text-xs sm:text-sm text-ash">
            <div className="flex items-center justify-between">
              <span>Доставка: по России от 2 дней</span>
              <a
                href="/delivery"
                onClick={(e) => handleAnchorClick(e, onDeliveryClick)}
                className="text-graphite dark:text-white hover:underline text-xs"
              >
                Подробнее →
              </a>
            </div>
            <div className="flex items-center justify-between">
              <span>Возврат: в течение 14 дней</span>
              <a
                href="/returns"
                onClick={(e) => handleAnchorClick(e, onReturnsClick)}
                className="text-graphite dark:text-white hover:underline text-xs"
              >
                Условия →
              </a>
            </div>
            {product.sellerName && (
              <div className="flex items-center justify-between">
                <span>Продавец: <strong className="text-graphite dark:text-white font-medium">{product.sellerName}</strong></span>
                {product.sellerSlug && (
                  <a
                    href={`/seller/${product.sellerSlug}`}
                    onClick={(e) => handleAnchorClick(e, () => product.sellerSlug && onSellerClick?.(product.sellerSlug))}
                    className="text-graphite dark:text-white hover:underline text-xs font-medium"
                  >
                    В магазин →
                  </a>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* FULL-WIDTH PRODUCT INFORMATION SECTION (BELOW HERO) */}
      <div className="mt-12 min-[1200px]:mt-16 pt-8 min-[1200px]:pt-12 border-t border-border-lighter dark:border-white/10">
        {/* Top 2-Column Overview */}
        <div className="grid grid-cols-1 min-[960px]:grid-cols-12 gap-8 lg:gap-12 mb-8">
          {/* Left / Larger: О вещи */}
          <div className="min-[960px]:col-span-7">
            <h2 className="text-lg font-serif text-graphite dark:text-white mb-3">
              О вещи
            </h2>
            <div className="text-sm text-graphite-light dark:text-white/80 leading-relaxed whitespace-pre-line space-y-3">
              {product.description ? (
                <p>{product.description}</p>
              ) : (
                <p className="text-ash italic">Описание товара уточняется.</p>
              )}
            </div>
          </div>

          {/* Right: Состав и уход */}
          <div className="min-[960px]:col-span-5">
            <h2 className="text-lg font-serif text-graphite dark:text-white mb-3">
              Состав и уход
            </h2>
            <div className="text-sm text-graphite-light dark:text-white/80 leading-relaxed space-y-2">
              {product.materialComposition && product.materialComposition.length > 0 ? (
                <p>
                  <span className="font-medium text-graphite dark:text-white">Состав:</span>{' '}
                  {product.materialComposition.map((mc) => `${mc.materialName || mc.material} — ${mc.percentage}%`).join(', ')}
                </p>
              ) : null}
              {product.materials ? (
                <p>
                  <span className="font-medium text-graphite dark:text-white">Материал:</span> {product.materials}
                </p>
              ) : null}
              {product.careInstructions ? (
                <p>
                  <span className="font-medium text-graphite dark:text-white">Уход:</span> {product.careInstructions}
                </p>
              ) : null}
              {!product.materialComposition?.length && !product.materials && !product.careInstructions && (
                <p className="text-ash italic">Информация о составе не указана.</p>
              )}
            </div>
          </div>
        </div>

        {/* Lower Accordions */}
        <div className="border-t border-border-lighter dark:border-white/10">
          {specs.length > 0 && (
            <AccordionSection title="Характеристики" defaultOpen={false}>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2.5 text-sm py-1">
                {specs.map((spec) => (
                  <div key={spec.label} className="flex justify-between py-1 border-b border-border-lighter/40 dark:border-white/5 sm:border-0">
                    <dt className="text-ash">{spec.label}</dt>
                    <dd className="text-graphite dark:text-white font-medium text-right sm:text-left">{spec.value}</dd>
                  </div>
                ))}
              </dl>
            </AccordionSection>
          )}

          {(product.sizeChart?.rows?.length ?? 0) > 0 && (
            <AccordionSection title="Размер и посадка" defaultOpen={false}>
              <div className="space-y-3 py-1">
                <p className="text-ash text-xs sm:text-sm">
                  Для этого товара доступна индивидуальная размерная сетка изделия в сантиметрах.
                </p>
                <button
                  type="button"
                  onClick={() => setShowSizeChart(true)}
                  className="inline-flex items-center text-xs sm:text-sm text-graphite dark:text-white font-medium hover:underline cursor-pointer"
                >
                  Открыть таблицу измерений изделия →
                </button>
              </div>
            </AccordionSection>
          )}

          <AccordionSection title="Доставка и возврат" defaultOpen={false}>
            <div className="space-y-2 py-1 text-xs sm:text-sm text-graphite-light dark:text-white/80">
              <p>• Бесплатная доставка от 10 000 ₽</p>
              <p>• Доставка по России: 2–7 дней</p>
              <p>• Примерка и возврат в течение 14 дней</p>
              <a
                href="/returns"
                onClick={(e) => handleAnchorClick(e, onReturnsClick)}
                className="inline-block mt-2 text-graphite dark:text-white hover:underline font-medium"
              >
                Подробнее об условиях возврата →
              </a>
            </div>
          </AccordionSection>
        </div>
      </div>

      {/* Size Chart Modal */}
      <PresentationModal
        isOpen={showSizeChart}
        onClose={() => setShowSizeChart(false)}
        title="Таблица размеров и мерки"
        maxWidth="5xl"
      >
        {product.sizeChart && product.sizeChart.rows && product.sizeChart.rows.length > 0 ? (
          <div className="flex flex-col lg:flex-row gap-8 items-start">
            {/* Left: Table */}
            <div className="flex-1 min-w-0 w-full overflow-x-auto">
              <div className="flex items-center justify-between mb-4">
                <h3 className="font-semibold text-base text-graphite dark:text-white">
                  Таблица измерений изделия
                </h3>
                <span className="text-xs text-ash bg-ice/60 dark:bg-white/5 px-2.5 py-1 rounded-full">
                  в сантиметрах (см)
                </span>
              </div>
              <div className="border border-border-lighter dark:border-white/10 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-ice/50 dark:bg-white/5 border-b border-border-lighter dark:border-white/10">
                    <tr>
                      <th className="py-3.5 px-4 text-left font-semibold text-graphite dark:text-white whitespace-nowrap">
                        Размер
                      </th>
                      {activeFields.map((field) => (
                        <th key={field} className="py-3.5 px-4 text-left font-semibold text-graphite dark:text-white whitespace-nowrap">
                          {getMeasurementMeta(field).label}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-lighter dark:divide-white/10">
                    {product.sizeChart.rows.map((row, i) => (
                      <tr key={i} className="hover:bg-ice/30 dark:hover:bg-white/[0.02] transition-colors">
                        <td className="py-3 px-4 font-semibold text-graphite dark:text-white whitespace-nowrap">
                          {row.sizeValueName}
                        </td>
                        {activeFields.map((field) => (
                          <td key={field} className="py-3 px-4 text-ash font-mono text-xs sm:text-sm whitespace-nowrap">
                            {row.measurements?.[field] ?? '—'}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <p className="mt-3 text-xs text-ash">
                * Все замеры сняты по готовому изделию в разложенном виде. Допустимая погрешность ±1-2 см.
              </p>
            </div>

            {/* Right: Illustration and instructions */}
            <div className="w-full lg:w-80 flex-shrink-0 bg-[#f8f9fb] dark:bg-white/5 p-5 rounded-2xl border border-border-lighter dark:border-white/10 flex flex-col items-center">
              <h3 className="font-semibold text-sm text-graphite dark:text-white mb-2 self-start">
                Как снять мерки
              </h3>

              <div className="w-full max-w-[220px] my-2 flex items-center justify-center">
                <SizeGuideIllustration activeFields={activeFields} />
              </div>

              <div className="mt-4 space-y-3 w-full border-t border-border-lighter dark:border-white/10 pt-4">
                {activeFields.map((field) => {
                  const meta = getMeasurementMeta(field);
                  return (
                    <div key={field} className="text-xs">
                      <span className="font-semibold text-graphite dark:text-white block">
                        {meta.shortLabel}
                      </span>
                      <span className="text-ash block mt-0.5 leading-relaxed">
                        {meta.instruction}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        ) : (
          <div className="text-center text-ash p-8">Размерная сетка пока не добавлена для этого товара.</div>
        )}
      </PresentationModal>

      {/* Lightbox / Fullscreen Modal */}
      {isLightboxOpen && typeof document !== 'undefined' && createPortal(
        <div
          ref={lightboxDialogRef}
          tabIndex={-1}
          role="dialog"
          aria-modal="true"
          aria-label="Просмотр фотографии на весь экран"
          className="fixed inset-0 z-[200] flex items-center justify-center bg-[#0a0a0c] select-none outline-none"
        >
          {/* Top Bar */}
          <div className="absolute top-4 inset-x-4 sm:inset-x-8 flex items-center justify-between z-20 pointer-events-none">
            <div className="text-white/80 text-xs sm:text-sm font-mono tracking-widest pointer-events-auto">
              {visibleImages.length > 1 && (
                <span>{currentActiveImage + 1} / {visibleImages.length}</span>
              )}
            </div>
            <button
              type="button"
              onClick={() => {
                setIsLightboxOpen(false);
                mainImageTriggerRef.current?.focus();
              }}
              aria-label="Закрыть"
              className="w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition-colors pointer-events-auto cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Large Left Viewport Navigation Zone */}
          {visibleImages.length > 1 && (
            <button
              type="button"
              onClick={handlePrevImage}
              aria-label="Предыдущее фото"
              className="group absolute left-0 inset-y-0 w-[20%] sm:w-[25%] z-10 cursor-w-resize flex items-center justify-start pl-4 sm:pl-8 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-white"
            >
              <span className="w-11 h-11 rounded-full bg-white/10 group-hover:bg-white/20 text-white flex items-center justify-center transition-all opacity-40 group-hover:opacity-100 pointer-events-none">
                <ChevronLeft className="w-6 h-6 -ml-0.5" />
              </span>
            </button>
          )}

          {/* Centered Large Image Stage */}
          <div
            className="relative z-10 w-full h-full max-w-[94vw] max-h-[88vh] flex items-center justify-center pointer-events-auto overflow-hidden"
            onClick={() => {
              if (hasMovedRef.current) return;
              if (isZoomed) {
                resetZoom();
              } else {
                setIsLightboxOpen(false);
                mainImageTriggerRef.current?.focus();
              }
            }}
          >
            <img
              src={currentImageUrl}
              alt={product.name}
              draggable={false}
              onPointerDown={(e) => {
                if (!isZoomed) return;
                setIsDragging(true);
                hasMovedRef.current = false;
                dragStartRef.current = {
                  startX: e.clientX,
                  startY: e.clientY,
                  initialPanX: panOffset.x,
                  initialPanY: panOffset.y,
                };
                if (typeof (e.target as HTMLElement).setPointerCapture === 'function') {
                  try {
                    (e.target as HTMLElement).setPointerCapture(e.pointerId);
                  } catch {}
                }
              }}
              onPointerMove={(e) => {
                if (!isDragging || !isZoomed) return;
                const dx = e.clientX - dragStartRef.current.startX;
                const dy = e.clientY - dragStartRef.current.startY;
                if (Math.abs(dx) > 3 || Math.abs(dy) > 3) {
                  hasMovedRef.current = true;
                }
                const maxPanX = window.innerWidth * 0.45;
                const maxPanY = window.innerHeight * 0.45;
                const nextX = Math.max(-maxPanX, Math.min(maxPanX, dragStartRef.current.initialPanX + dx));
                const nextY = Math.max(-maxPanY, Math.min(maxPanY, dragStartRef.current.initialPanY + dy));
                setPanOffset({ x: nextX, y: nextY });
              }}
              onPointerUp={(e) => {
                if (isDragging) {
                  setIsDragging(false);
                  if (typeof (e.target as HTMLElement).releasePointerCapture === 'function') {
                    try {
                      (e.target as HTMLElement).releasePointerCapture(e.pointerId);
                    } catch {}
                  }
                }
              }}
              onClick={(e) => {
                e.stopPropagation();
                if (hasMovedRef.current) {
                  hasMovedRef.current = false;
                  return;
                }
                if (isZoomed) {
                  resetZoom();
                } else {
                  setIsZoomed(true);
                  setPanOffset({ x: 0, y: 0 });
                }
              }}
              style={{
                transform: isZoomed
                  ? `translate3d(${panOffset.x}px, ${panOffset.y}px, 0px) scale(2)`
                  : 'translate3d(0px, 0px, 0px) scale(1)',
                transition: isDragging ? 'none' : 'transform 200ms ease-out',
              }}
              className={cn(
                "max-w-full max-h-[88vh] w-auto h-auto object-contain rounded-lg shadow-2xl select-none touch-none",
                isZoomed
                  ? (isDragging ? "cursor-grabbing" : "cursor-grab")
                  : "cursor-zoom-in"
              )}
            />
          </div>

          {/* Large Right Viewport Navigation Zone */}
          {visibleImages.length > 1 && (
            <button
              type="button"
              onClick={handleNextImage}
              aria-label="Следующее фото"
              className="group absolute right-0 inset-y-0 w-[20%] sm:w-[25%] z-10 cursor-e-resize flex items-center justify-end pr-4 sm:pr-8 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-white"
            >
              <span className="w-11 h-11 rounded-full bg-white/10 group-hover:bg-white/20 text-white flex items-center justify-center transition-all opacity-40 group-hover:opacity-100 pointer-events-none">
                <ChevronRight className="w-6 h-6 -mr-0.5" />
              </span>
            </button>
          )}

          {/* Bottom Thumbnails Strip (Lightbox) */}
          {visibleImages.length > 1 && (
            <div className="absolute bottom-4 inset-x-0 flex justify-center gap-2 px-4 overflow-x-auto scrollbar-none z-20">
              {visibleImages.map((img, idx) => {
                const isSelected = idx === currentActiveImage;
                return (
                  <button
                    key={'lb-' + idx + img.url}
                    type="button"
                    onClick={() => onActiveImageChange(idx)}
                    aria-label={`Фото ${idx + 1}`}
                    className={cn(
                      "w-12 h-14 rounded overflow-hidden flex-shrink-0 bg-white/5 p-0.5 transition-all cursor-pointer",
                      isSelected
                        ? "ring-2 ring-white opacity-100 scale-105"
                        : "opacity-40 hover:opacity-80"
                    )}
                  >
                    <img
                      src={img.url}
                      alt=""
                      className="w-full h-full object-contain"
                    />
                  </button>
                );
              })}
            </div>
          )}
        </div>,
        document.body
      )}
    </>
  );
}

import { useState, useEffect, useRef, useMemo } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { ChevronRight, Star } from 'lucide-react';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';
import { PreviewPageMetadata } from '../components/PreviewPageMetadata';
import { fetchProductById, fetchProductReviews, fetchProductPreviewByToken } from '../api/publicCatalog';
import { recordProductView } from '@zamk/api-client/src/customer';
import { isInsufficientStockError } from '@zamk/api-client/src/errors';
import {
  useVariantSelection,
  reconcileSelectionAfterStaleStock,
  getDefaultColorId,
  getVariantColorId,
  getVariantSizeId,
  isVariantBuyable,
  PRODUCT_JUST_SOLD_OUT_NOTICE,
  REFRESH_ERROR_NOTICE,
} from '../lib/variantSelection';
import {
  validateVariantUrlState,
  computeVariantUrlParams,
  areSearchParamsEqual,
} from '../lib/variantUrlState';
import {
  findMediaIndexForColor,
  deduplicateGalleryImages,
} from '../lib/mediaFocus';
import { SimilarProductsBlock } from '../components/product/SimilarProductsBlock';
import { ProductPresentationCore, formatReviewsCount } from '../components/product-detail/ProductPresentationCore';
import type { Product, Review } from '../types/catalog';
import { cn } from '../lib/utils';
import { Button } from '../components/ui/Button';
import type { GalleryMediaItem } from '../lib/mediaFocus';

export function ProductDetail() {
  const { id, token } = useParams<{ id?: string; token?: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const [product, setProduct] = useState<Product | null>(null);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isAddingToCart, setIsAddingToCart] = useState(false);
  const [isProductUnavailable, setIsProductUnavailable] = useState(false);
  const [refreshErrorNotice, setRefreshErrorNotice] = useState<string | null>(null);

  const [activeImage, setActiveImage] = useState(0);
  const [sizeError, setSizeError] = useState('');

  useEffect(() => {
    async function loadProduct() {
      if (!id && !token) return;
      try {
        setIsLoading(true);
        let data: Product;
        if (token) {
          data = await fetchProductPreviewByToken(token);
        } else if (id) {
          data = await fetchProductById(id);
        } else {
          return;
        }
        setProduct(data);
        setError(null);

        if (!data.isPreview && data.id) {
          try {
            const revs = await fetchProductReviews(data.id);
            setReviews(revs);
          } catch (e) {
            console.warn("Failed to fetch reviews", e);
          }
        }
      } catch (err: any) {
        console.error('Failed to load product:', err);
        if (token) {
          if (err?.status === 404 || err?.code === 'invalid_preview_link' || err?.message?.includes('недействительна')) {
            setError('Ссылка предпросмотра недействительна');
          } else if (err?.status === 410 && err?.code === 'product_unavailable') {
            setError('Предпросмотр этого товара больше недоступен');
          } else {
            setError('Срок действия ссылки истёк или ссылка больше недоступна');
          }
        } else {
          setError('Не удалось загрузить товар');
        }
      } finally {
        setIsLoading(false);
      }
    }
    loadProduct();
  }, [id, token]);

  const { addItem } = useCart();
  const { user, isAuthenticated } = useAuth();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();

  const lastTrackedKeyRef = useRef<string | null>(null);

  useEffect(() => {
    // Only record for authenticated customers on canonical published products (never preview)
    if (!isAuthenticated || !user?.id || !product || product.isPreview || !product.id) {
      return;
    }
    const trackKey = `${user.id}:${product.id}`;
    // Prevent duplicate recording on ordinary re-renders of the same product
    if (lastTrackedKeyRef.current === trackKey) {
      return;
    }
    lastTrackedKeyRef.current = trackKey;

    recordProductView(product.id).catch((err) => {
      // Best-effort soft telemetry: failure must never break PDP rendering
      console.debug('Failed to record product view', err);
    });
  }, [isAuthenticated, user?.id, product?.id, product?.isPreview]);

  const {
    dimensionType,
    colors,
    sizes,
    selectedColorId,
    selectedSizeId,
    selectedColor,
    selectedSize,
    selectedVariant,
    isResolved,
    canAddToCart,
    requiresColor,
    requiresSize,
    ctaText,
    sizeSelectionNotice,
    selectColor,
    selectSize,
    clearSelectedSize,
    setSizeSelectionNotice,
    restoreSelection,
  } = useVariantSelection(product?.variants, product?.sizeChart);

  const defaultImage = { url: 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image' };

  // Product-level media stream: photos belong to the product as a whole.
  // One stable, ordered media collection without filtering by selectedColor or selectedVariant.
  const visibleImages: GalleryMediaItem[] = useMemo(() => {
    return deduplicateGalleryImages(product?.images, product?.image, defaultImage);
  }, [product?.images, product?.image]);

  // Track last focused color to avoid re-focusing when manually browsing thumbnails
  const lastFocusedColorRef = useRef<string | null | undefined>(undefined);

  // Synchronize state with URL search params (on load, Back/Forward, or shared link)
  useEffect(() => {
    if (!product?.variants) return;

    const validated = validateVariantUrlState(product.variants, searchParams);

    // 1. Sanitization: if URL has invalid/sold-out/not-offered params, sanitize via REPLACE
    if (validated.needsReplace) {
      setSearchParams(validated.sanitizedSearchParams, { replace: true });
    }

    // 2. Selection state synchronization
    if (validated.hasExplicitColorIntent && validated.targetColorId) {
      if (
        validated.targetColorId !== selectedColorId ||
        validated.targetSizeId !== selectedSizeId ||
        (validated.notice !== null && validated.notice !== sizeSelectionNotice)
      ) {
        const nextNotice = validated.notice !== null ? validated.notice : sizeSelectionNotice;
        restoreSelection(validated.targetColorId, validated.targetSizeId, nextNotice);
      }

      // Media focus: URL-restored color represents explicit color intent (PDP.2D2 Section 15)
      // Focus media only if the explicit color changed from what was last focused
      if (lastFocusedColorRef.current !== validated.targetColorId) {
        lastFocusedColorRef.current = validated.targetColorId;
        const targetIndex = findMediaIndexForColor(visibleImages, validated.targetColorId);
        setActiveImage(targetIndex);
      }
    } else if (dimensionType === 'SIZE_ONLY' && validated.hasExplicitSizeIntent) {
      if (
        validated.targetSizeId !== selectedSizeId ||
        (validated.notice !== null && validated.notice !== sizeSelectionNotice)
      ) {
        const nextNotice = validated.notice !== null ? validated.notice : sizeSelectionNotice;
        restoreSelection(null, validated.targetSizeId, nextNotice);
      }
    } else {
      // Clean URL:
      // If user navigated Back to clean URL from a colored URL, reset media focus to main image:
      if (
        lastFocusedColorRef.current !== undefined &&
        lastFocusedColorRef.current !== null &&
        lastFocusedColorRef.current !== getDefaultColorId(colors)
      ) {
        lastFocusedColorRef.current = getDefaultColorId(colors);
        setActiveImage(0);
      } else if (lastFocusedColorRef.current === undefined) {
        // Initial clean load: mark default color as focused so selecting a size does not refocus media
        lastFocusedColorRef.current = getDefaultColorId(colors);
      }
      const isCleanUrl = !searchParams.has('color') && !searchParams.has('size');
      if (isCleanUrl && (selectedSizeId !== null || (selectedColorId && selectedColorId !== getDefaultColorId(colors)))) {
        restoreSelection(getDefaultColorId(colors), null, null);
      }
    }
  }, [
    product?.variants,
    searchParams,
    dimensionType,
    colors,
    visibleImages,
    selectedColorId,
    selectedSizeId,
    sizeSelectionNotice,
    restoreSelection,
    setSearchParams,
  ]);

  if (isLoading) {
    return (
      <div className="relative z-10 min-h-screen pt-36 pb-20 flex justify-center">
        {token && <PreviewPageMetadata />}
        <div className="animate-spin w-8 h-8 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
      </div>
    );
  }

  if (error || !product) {
    return (
      <div className="relative z-10 min-h-screen pt-36 pb-20">
        {token && <PreviewPageMetadata />}
        <div className="container mx-auto px-4 sm:px-6 max-w-4xl text-center">
          <h1 className="text-4xl font-serif text-graphite dark:text-white">{error || 'Товар не найден'}</h1>
          <Link to="/catalog" className="inline-block mt-6">
            <Button>Вернуться в каталог</Button>
          </Link>
        </div>
      </div>
    );
  }

  const handleColorChange = (colorId: string) => {
    if (isProductUnavailable) return;
    selectColor(colorId);
    setRefreshErrorNotice(null);
    if (sizeError) setSizeError('');

    // Color-aware media focus (PDP.2E1):
    // Focus the first image matching selected color, or fallback to general image, or fallback to index 0.
    const targetIndex = findMediaIndexForColor(visibleImages, colorId);
    setActiveImage(targetIndex);
    lastFocusedColorRef.current = colorId;

    // Check if selected size remains buyable in new color (PDP.2B / PDP.2D2 Section 5)
    let nextSizeId: string | null = null;
    if (selectedSizeId) {
      const targetVariant = product?.variants?.find(
        v => v.isActive !== false &&
             getVariantColorId(v) === colorId &&
             getVariantSizeId(v) === selectedSizeId
      );
      if (isVariantBuyable(targetVariant)) {
        nextSizeId = selectedSizeId;
      }
    }

    const nextParams = computeVariantUrlParams(searchParams, dimensionType, colorId, nextSizeId, product?.variants);
    if (!areSearchParamsEqual(nextParams, searchParams)) {
      setSearchParams(nextParams);
    }
  };

  const handleSizeChange = (sizeId: string) => {
    if (isProductUnavailable) return;
    selectSize(sizeId);
    if (sizeError) setSizeError('');
    setRefreshErrorNotice(null);

    const effColorId = (dimensionType === 'COLOR_AND_SIZE' || dimensionType === 'COLOR_ONLY')
      ? selectedColorId
      : null;

    const nextParams = computeVariantUrlParams(searchParams, dimensionType, effColorId, sizeId, product?.variants);
    if (!areSearchParamsEqual(nextParams, searchParams)) {
      setSearchParams(nextParams);
    }
  };

  const liked = isFavorite(product.id);

  const handleAddToCart = async () => {
    if (product.isPreview || isAddingToCart || isProductUnavailable) return;
    if (requiresSize && !selectedSizeId) {
      setSizeError('Выберите размер перед добавлением в корзину');
      return;
    }
    setSizeError('');

    if (!selectedVariant) {
      showToast('Для товара не указан вариант. Добавление в корзину недоступно.');
      return;
    }

    if (!canAddToCart) {
      showToast('Выбранный вариант товара закончился.');
      return;
    }

    try {
      setIsAddingToCart(true);
      setRefreshErrorNotice(null);
      await addItem(product.id, selectedVariant.id, 1);
      showToast('Товар добавлен в корзину');
    } catch (e: any) {
      if (isInsufficientStockError(e)) {
        const prevSizeLabel = selectedSize?.label || selectedVariant.size;
        try {
          const freshProduct = token
            ? await fetchProductPreviewByToken(token)
            : await fetchProductById(product.id || id!);

          setProduct(freshProduct);

          const reconciliation = reconcileSelectionAfterStaleStock(
            dimensionType,
            selectedColorId,
            selectedSizeId,
            freshProduct.variants,
            prevSizeLabel
          );

          if (!reconciliation.isBuyable) {
            if (reconciliation.nextSizeId === null && selectedSizeId !== null) {
              clearSelectedSize();
              if (searchParams.has('size')) {
                const nextParams = new URLSearchParams(searchParams);
                nextParams.delete('size');
                setSearchParams(nextParams, { replace: true });
              }
            }
            if (reconciliation.notice) {
              setSizeSelectionNotice(reconciliation.notice);
              showToast(reconciliation.notice);
            }
          }
        } catch (refreshErr: any) {
          if (
            refreshErr?.status === 404 ||
            refreshErr?.code === 'not_found' ||
            refreshErr?.status === 410 ||
            refreshErr?.code === 'product_unavailable'
          ) {
            setIsProductUnavailable(true);
            clearSelectedSize();
            if (searchParams.has('size')) {
              const nextParams = new URLSearchParams(searchParams);
              nextParams.delete('size');
              setSearchParams(nextParams, { replace: true });
            }
            setSizeSelectionNotice(PRODUCT_JUST_SOLD_OUT_NOTICE);
            showToast(PRODUCT_JUST_SOLD_OUT_NOTICE);
          } else {
            setRefreshErrorNotice(REFRESH_ERROR_NOTICE);
            showToast(REFRESH_ERROR_NOTICE);
          }
        }
      } else {
        showToast(e.message || 'Ошибка при добавлении в корзину');
      }
    } finally {
      setIsAddingToCart(false);
    }
  };


  const getDisplayPrice = () => {
    return selectedVariant?.priceCents ? selectedVariant.priceCents / 100 : product.price;
  };

  const hasReviews = (product.reviewsCount ?? (reviews ? reviews.length : 0)) > 0;
  const reviewsCountText = hasReviews ? `${product.reviewsCount ?? (reviews ? reviews.length : 0)} ${formatReviewsCount(product.reviewsCount ?? (reviews ? reviews.length : 0))}` : undefined;

  return (
    <div className="relative z-10 min-h-screen pt-20 md:pt-24 pb-20">
      {token && <PreviewPageMetadata />}
      {product.isPreview && (
        <div className="bg-amber-500 text-slate-950 font-bold px-4 py-3 text-center text-xs sm:text-sm sticky top-16 z-40 shadow-md flex items-center justify-center gap-2 mb-4">
          <span>Предпросмотр товара для модерации. Товар ещё не опубликован.</span>
        </div>
      )}

      <div className="container mx-auto px-4 sm:px-6 lg:px-8 max-w-[1324px]">
        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-xs sm:text-sm text-ash mb-5 sm:mb-6 flex-wrap">
          <Link to="/" className="hover:text-graphite dark:hover:text-white transition-colors">Главная</Link>
          <ChevronRight className="w-3.5 h-3.5" />
          <Link to="/catalog" className="hover:text-graphite dark:hover:text-white transition-colors">Каталог</Link>
          <ChevronRight className="w-3.5 h-3.5" />
          <span className="text-graphite dark:text-white line-clamp-1">{product.name}</span>
        </nav>

        <div className="bg-white dark:bg-[#121214] border border-border-lighter/70 dark:border-white/5 rounded-2xl p-5 sm:p-7 min-[1200px]:p-8 shadow-xs">
          <ProductPresentationCore
            product={product}
            visibleImages={visibleImages}
            activeImage={activeImage}
            onActiveImageChange={setActiveImage}
            displayPrice={getDisplayPrice()}
            dimensionType={dimensionType}
            colors={colors}
            sizes={sizes}
            selectedColorId={selectedColorId}
            selectedSizeId={selectedSizeId}
            selectedColor={selectedColor}
            selectedSize={selectedSize}
            selectedVariant={selectedVariant}
            isResolved={isResolved}
            canAddToCart={canAddToCart}
            requiresColor={requiresColor}
            requiresSize={requiresSize}
            isAddingToCart={isAddingToCart}
            isProductUnavailable={isProductUnavailable}
            ctaText={ctaText}
            sizeSelectionNotice={sizeSelectionNotice}
            refreshErrorNotice={refreshErrorNotice}
            sizeError={sizeError}
            reviewsCountText={reviewsCountText}
            onColorChange={handleColorChange}
            onSizeChange={handleSizeChange}
            onAddToCart={handleAddToCart}
            isFavorite={isFavorite(product.id)}
            onToggleFavorite={() => {
              if (!user) {
                showToast('Войдите, чтобы добавить товар в избранное', 'error');
              } else {
                toggleFavorite(product.id);
              }
            }}
            onScrollToReviews={() => {
              const el = document.getElementById('product-reviews-section');
              el?.scrollIntoView({ behavior: 'smooth' });
            }}
          />
        </div>

        {/* Reviews Section */}
{/* Reviews Section */}
        <section id="product-reviews-section" className="mt-12 sm:mt-16">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-2xl font-serif text-graphite dark:text-white">
              Отзывы ({product.reviewsCount || (reviews ? reviews.length : 0)})
            </h2>
            {product.rating && (
              <div className="flex items-center gap-2">
                <div className="flex items-center">
                  {[1, 2, 3, 4, 5].map((star) => (
                    <Star
                      key={star}
                      className={cn(
                        "w-5 h-5",
                        star <= Math.round(product.rating!)
                          ? "fill-amber-400 text-amber-400"
                          : "fill-gray-200 text-gray-200 dark:fill-gray-600 dark:text-gray-600"
                      )}
                    />
                  ))}
                </div>
                <span className="text-lg font-medium text-graphite dark:text-white">{product.rating.toFixed(1)}</span>
              </div>
            )}
          </div>

          {!reviews || reviews.length === 0 ? (
            <p className="text-ash dark:text-white/60">Пока нет отзывов.</p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {reviews.map((review) => (
                <div key={review.id} className="p-5 rounded-xl bg-white dark:bg-[#1a1a1c] border border-border-lighter dark:border-white/10">
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <h4 className="font-medium text-graphite dark:text-white">{review.author}</h4>
                      <p className="text-xs text-ash mt-0.5">{review.date}</p>
                    </div>
                    <div className="flex items-center">
                      {[1, 2, 3, 4, 5].map((star) => (
                        <Star
                          key={star}
                          className={cn(
                            "w-4 h-4",
                            star <= review.rating
                              ? "fill-amber-400 text-amber-400"
                              : "fill-gray-200 text-gray-200 dark:fill-gray-600 dark:text-gray-600"
                          )}
                        />
                      ))}
                    </div>
                  </div>
                  <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed">{review.text}</p>
                  {(review.fit || review.quality) && (
                    <div className="flex flex-wrap gap-2 mt-3">
                      {review.fit && (
                        <span className="text-xs px-2 py-1 rounded bg-ice dark:bg-white/10 text-ash">
                          Крой: {review.fit}
                        </span>
                      )}
                      {review.quality && (
                        <span className="text-xs px-2 py-1 rounded bg-ice dark:bg-white/10 text-ash">
                          Качество: {review.quality}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Similar Products */}
        <SimilarProductsBlock productId={product.id} />
      </div>
    </div>
  );
}

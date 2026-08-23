import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ChevronRight, Heart, Minus, Plus, Ruler, ShoppingBag, Star, Truck, RefreshCw, Shield, ChevronDown, Eye } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';
import { PreviewPageMetadata } from '../components/PreviewPageMetadata';
import { formatPrice, cn } from '../lib/utils';
import { fetchProductById, fetchProductReviews, fetchProductPreviewByToken } from '../api/publicCatalog';
import type { Product, Review } from '../types/catalog';

const getProductSpecs = (product: Product, selectedVariant?: any) =>
  [
    (selectedVariant?.sellerSku || !product.id) ? { label: 'Артикул', value: (selectedVariant?.sellerSku || product.id).toUpperCase() } : null,
    product.brand && product.brand !== 'Бренд не указан' ? { label: 'Бренд', value: product.brand } : null,
    product.category && product.category !== 'Категория не указана' ? { label: 'Категория', value: product.category } : null,
    product.materials ? { label: 'Материал', value: product.materials } : null,
  ].filter((spec): spec is { label: string; value: string } => Boolean(spec));

function AccordionSection({ title, children, defaultOpen = false }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-border-lighter dark:border-white/10">
      <button onClick={() => setIsOpen(!isOpen)} className="w-full py-4 flex items-center justify-between text-left">
        <span className="text-sm font-medium text-graphite dark:text-white">{title}</span>
        <ChevronDown className={cn("w-4 h-4 text-ash transition-transform", isOpen && "rotate-180")} />
      </button>
      {isOpen && <div className="pb-4">{children}</div>}
    </div>
  );
}

const SizeGuideIllustration = ({ category }: { category: string }) => {
  const isLower = category.toLowerCase().includes('брюки') || category.toLowerCase().includes('джинсы') || category.toLowerCase().includes('шорты') || category.toLowerCase().includes('юбки');
  const isFootwear = category.toLowerCase().includes('обувь') || category.toLowerCase().includes('кроссовки') || category.toLowerCase().includes('ботинки');

  if (isFootwear) {
    return (
      <svg viewBox="0 0 100 100" className="w-full max-w-[200px] h-auto text-ash/30" fill="currentColor">
        <path d="M70,80 Q90,80 90,60 Q90,40 60,30 Q40,20 20,40 Q10,50 10,70 Q10,80 30,80 Z" />
        <line x1="10" y1="90" x2="90" y2="90" stroke="red" strokeWidth="2" strokeDasharray="4 4" />
        <text x="50" y="98" fontSize="6" fill="red" textAnchor="middle">Длина стопы</text>
      </svg>
    );
  }

  if (isLower) {
    return (
      <svg viewBox="0 0 100 100" className="w-full max-w-[200px] h-auto text-ash/30" fill="currentColor">
        <path d="M30,10 L70,10 L75,30 L70,90 L55,90 L50,40 L45,90 L30,90 L25,30 Z" />
        <line x1="20" y1="15" x2="80" y2="15" stroke="red" strokeWidth="2" strokeDasharray="2 2" />
        <text x="85" y="17" fontSize="6" fill="red">Талия</text>
        <line x1="20" y1="30" x2="80" y2="30" stroke="red" strokeWidth="2" strokeDasharray="2 2" />
        <text x="85" y="32" fontSize="6" fill="red">Бёдра</text>
      </svg>
    );
  }

  // Default Upper Apparel
  return (
    <svg viewBox="0 0 100 100" className="w-full max-w-[200px] h-auto text-ash/30" fill="currentColor">
      <path d="M35,10 Q50,20 65,10 L85,30 L75,40 L70,30 L70,90 L30,90 L30,30 L25,40 L15,30 Z" />
      <line x1="20" y1="35" x2="80" y2="35" stroke="red" strokeWidth="2" strokeDasharray="2 2" />
      <text x="85" y="37" fontSize="6" fill="red">Грудь</text>
      <line x1="25" y1="55" x2="75" y2="55" stroke="red" strokeWidth="2" strokeDasharray="2 2" />
      <text x="80" y="57" fontSize="6" fill="red">Талия</text>
    </svg>
  );
};
export function ProductDetail() {
  const { id, token } = useParams<{ id?: string; token?: string }>();

  const [product, setProduct] = useState<Product | null>(null);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [activeImage, setActiveImage] = useState(0);
  const [activeSize, setActiveSize] = useState<string | null>(null);
  const [activeColor, setActiveColor] = useState(0);
  const [quantity, setQuantity] = useState(1);
  const [showSizeChart, setShowSizeChart] = useState(false);
  const [sizeError, setSizeError] = useState('');
  const [activeTab, setActiveTab] = useState<'description' | 'specs' | 'reviews'>('description');

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
  const { user } = useAuth();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();

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

  const defaultImage = { url: 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image' };
  const allImages = (product.images && product.images.length > 0)
    ? product.images
    : (product.image ? [{ url: product.image }] : [defaultImage]);

  const liked = isFavorite(product.id);

  const colorMap = new Map<string, { id: string, name: string, hex: string }>();
  product.variants?.forEach(v => {
    if (v.colorId && v.colorName) {
      if (!colorMap.has(v.colorId)) {
        colorMap.set(v.colorId, { id: v.colorId, name: v.colorName, hex: v.colorHex || '#000000' });
      }
    }
  });
  const colors = Array.from(colorMap.values());
  const activeColorObj = colors[activeColor] || null;

  const sizeMap = new Map<string, { id: string, label: string }>();
  product.variants?.forEach(v => {
    if (!v.size || !v.isActive) return;
    if (colors.length > 0) {
      if (activeColorObj && v.colorId === activeColorObj.id) {
        if (!sizeMap.has(v.size)) {
           sizeMap.set(v.size, { id: v.sizeValueId || v.size, label: v.size });
        }
      }
    } else {
      if (!sizeMap.has(v.size)) {
         sizeMap.set(v.size, { id: v.sizeValueId || v.size, label: v.size });
      }
    }
  });

  const selectableSizes = Array.from(sizeMap.values());
  if (product.sizeChart?.rows) {
    const sortOrder = product.sizeChart.rows.map((r: any) => r.sizeValueName);
    selectableSizes.sort((a, b) => {
      const idxA = sortOrder.indexOf(a.label);
      const idxB = sortOrder.indexOf(b.label);
      if (idxA !== -1 && idxB !== -1) return idxA - idxB;
      if (idxA !== -1) return -1;
      if (idxB !== -1) return 1;
      return 0;
    });
  }

  const requiresSizeSelection = selectableSizes.length > 0 && !selectableSizes.some(s => s.label === 'Единый');

  let selectedVariant = product.variants?.[0];
  if (requiresSizeSelection) {
    selectedVariant = product.variants?.find(v =>
      v.size === activeSize && v.isActive &&
      (colors.length === 0 || v.colorId === activeColorObj?.id)
    );
  } else if (colors.length > 0) {
    selectedVariant = product.variants?.find(v => v.isActive && v.colorId === activeColorObj?.id);
  }

  const specs = getProductSpecs(product, selectedVariant);

  const visibleImages = allImages.filter((img: any) => !img.colorId || (activeColorObj && img.colorId === activeColorObj.id));
  if (visibleImages.length === 0 && allImages.length > 0) visibleImages.push(allImages[0]);
  const currentActiveImage = activeImage < visibleImages.length ? activeImage : 0;

  const handleAddToCart = async () => {
    if (product.isPreview) return;
    if (requiresSizeSelection && !activeSize) {
      setSizeError('Выберите размер перед добавлением в корзину');
      return;
    }
    setSizeError('');

    if (!selectedVariant) {
      showToast('Для товара не указан вариант. Добавление в корзину недоступно.');
      return;
    }

    if (!(selectedVariant.inStock ?? true)) {
      showToast('Выбранный вариант товара закончился.');
      return;
    }

    try {
      await addItem(product.id, selectedVariant.id, quantity);
      showToast('Товар добавлен в корзину');
    } catch (e: any) {
      showToast(e.message || 'Ошибка при добавлении');
    }
  };

  const getDisplayPrice = () => {
    return selectedVariant?.priceCents ? selectedVariant.priceCents / 100 : product.price;
  };

  return (
    <div className="relative z-10 min-h-screen pt-20 md:pt-24 pb-20">
      {token && <PreviewPageMetadata />}
      {product.isPreview && (
        <div className="bg-amber-500 text-slate-950 font-bold px-4 py-3 text-center text-xs sm:text-sm sticky top-16 z-40 shadow-md flex items-center justify-center gap-2 mb-4">
          <Eye className="w-4 h-4 flex-shrink-0" />
          <span>Предпросмотр товара для модерации. Товар ещё не опубликован.</span>
        </div>
      )}

      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">

        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-sm text-ash mb-6 flex-wrap">
          <Link to="/" className="hover:text-graphite dark:hover:text-white transition-colors">Главная</Link>
          <ChevronRight className="w-4 h-4" />
          <Link to="/catalog" className="hover:text-graphite dark:hover:text-white transition-colors">Каталог</Link>
          <ChevronRight className="w-4 h-4" />
          <span className="text-graphite dark:text-white line-clamp-1">{product.name}</span>
        </nav>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 lg:gap-12">

          {/* Gallery */}
          <div className="space-y-4">
            {/* Main image */}
            <div className="relative bg-[#f5f5f7] dark:bg-[#1a1a1c] rounded-xl overflow-hidden w-full flex items-center justify-center" style={{ aspectRatio: '4/5', maxHeight: '80vh' }}>
              <img
                src={visibleImages[currentActiveImage]?.url || defaultImage.url}
                alt={product.name}
                className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal"
              />
              {product.isNew && (
                <span className="absolute top-4 left-4 px-3 py-1 rounded bg-graphite text-white dark:text-black text-xs font-semibold uppercase">
                  Новинка
                </span>
              )}
              {product.discountPrice && (
                <span className="absolute top-4 right-4 px-3 py-1 rounded bg-red-500 text-white text-xs font-semibold uppercase">
                  -{Math.round((1 - product.discountPrice / product.price) * 100)}%
                </span>
              )}
            </div>

            {/* Thumbnails */}
            {visibleImages.length > 1 && (
              <div className="flex gap-2 overflow-x-auto pb-2">
                {visibleImages.map((image: any, index: number) => (
                  <button
                    key={index + image.url}
                    onClick={() => setActiveImage(index)}
                    className={cn(
                      "w-20 h-20 flex-shrink-0 rounded-lg overflow-hidden bg-[#f5f5f7] dark:bg-[#1a1a1c] border-2 transition-colors flex items-center justify-center",
                      currentActiveImage === index ? "border-graphite dark:border-white" : "border-transparent"
                    )}
                  >
                    <img src={image.url} alt="" className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal" />
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="lg:sticky lg:top-28 lg:self-start">
            {product.sellerSlug && !product.isPreview ? (
              <Link to={`/seller/${product.sellerSlug}`} className="text-sm text-ash hover:text-graphite dark:hover:text-white transition-colors">
                {product.sellerName || product.brand}
              </Link>
            ) : (
              <span className="text-sm text-ash">
                {product.sellerName || product.brand}
              </span>
            )}

            <h1 className="mt-2 text-2xl md:text-3xl font-serif text-graphite dark:text-white leading-tight">
              {product.name}
            </h1>

            {product.rating && (
              <div className="flex items-center gap-2 mt-3">
                <div className="flex items-center">
                  {[1, 2, 3, 4, 5].map((star) => (
                    <Star
                      key={star}
                      className={cn(
                        "w-4 h-4",
                        star <= Math.round(product.rating!)
                          ? "fill-amber-400 text-amber-400"
                          : "fill-gray-200 text-gray-200 dark:fill-gray-600 dark:text-gray-600"
                      )}
                    />
                  ))}
                </div>
                <span className="text-sm text-graphite dark:text-white font-medium">{product.rating.toFixed(1)}</span>
                {product.reviewsCount && (
                  <button
                    onClick={() => setActiveTab('reviews')}
                    className="text-sm text-ash hover:text-graphite dark:hover:text-white transition-colors"
                  >
                    {product.reviewsCount} отзывов
                  </button>
                )}
              </div>
            )}

            <div className="mt-4 flex items-baseline gap-3">
              {product.discountPrice ? (
                <>
                  <span className="text-2xl font-semibold text-red-500">{formatPrice(product.discountPrice)}</span>
                  <span className="text-lg text-ash line-through">{formatPrice(getDisplayPrice())}</span>
                </>
              ) : (
                <span className="text-2xl font-semibold text-graphite dark:text-white">{formatPrice(getDisplayPrice())}</span>
              )}
            </div>

            {/* Colors */}
            {colors.length > 0 && (
              <div className="mt-6">
                <p className="text-sm text-graphite dark:text-white mb-2">
                  Цвет: <span className="text-ash">{activeColorObj?.name || ''}</span>
                </p>
                <div className="flex gap-2">
                  {colors.map((color, index) => (
                    <button
                      key={color.id}
                      onClick={() => { setActiveColor(index); setActiveImage(0); }}
                      style={{ backgroundColor: color.hex }}
                      className={cn(
                        "w-10 h-10 rounded-full border-2 transition-all",
                        activeColor === index
                          ? "border-graphite dark:border-white ring-2 ring-graphite/20 dark:ring-white/20"
                          : "border-border-lighter dark:border-white/20 hover:scale-105"
                      )}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Sizes */}
            {requiresSizeSelection && (
              <div className="mt-6">
                <div className="flex items-center justify-between mb-2">
                  <p className="text-sm text-graphite dark:text-white">
                    Размер: <span className="text-ash">{activeSize || 'Не выбран'}</span>
                  </p>
                  <button
                    type="button"
                    onClick={() => setShowSizeChart(true)}
                    className="flex items-center gap-1 text-sm text-primary hover:underline"
                  >
                    <Ruler className="w-4 h-4" />
                    Таблица размеров
                  </button>
                </div>
                <div className="flex flex-wrap gap-2">
                  {selectableSizes.map((sizeObj) => (
                    <button
                      key={sizeObj.id}
                      type="button"
                      onClick={() => {
                        setActiveSize(sizeObj.label);
                        if (sizeError) setSizeError('');
                      }}
                      className={cn(
                        "h-10 min-w-[48px] px-4 rounded-lg border text-sm font-medium transition-all",
                        activeSize === sizeObj.label
                            ? "bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white"
                          : "bg-white dark:bg-transparent border-border-lighter dark:border-white/20 text-graphite dark:text-white hover:border-graphite dark:hover:border-white"
                      )}
                    >
                      {sizeObj.label}
                    </button>
                  ))}
                </div>
                {sizeError && (
                  <p className="mt-2 text-sm text-error" role="alert">
                    {sizeError}
                  </p>
                )}
              </div>
            )}

            {/* Quantity */}
            <div className="mt-6">
              <p className="text-sm text-graphite dark:text-white mb-2">Количество</p>
              <div className="flex items-center gap-3">
                <div className="flex items-center border border-border-lighter dark:border-white/20 rounded-lg">
                  <button
                    onClick={() => setQuantity(Math.max(1, quantity - 1))}
                    disabled={product.isPreview}
                    className="w-10 h-10 flex items-center justify-center text-graphite dark:text-white hover:bg-ice dark:hover:bg-white/5 transition-colors disabled:opacity-50"
                  >
                    <Minus className="w-4 h-4" />
                  </button>
                  <span className="w-12 text-center text-sm font-medium text-graphite dark:text-white">{quantity}</span>
                  <button
                    onClick={() => setQuantity(quantity + 1)}
                    disabled={product.isPreview}
                    className="w-10 h-10 flex items-center justify-center text-graphite dark:text-white hover:bg-ice dark:hover:bg-white/5 transition-colors disabled:opacity-50"
                  >
                    <Plus className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>

            {/* Add to cart */}
            <div className="mt-6 flex gap-3">
              <Button
                type="button"
                variant="primary"
                className="flex-1 h-12 gap-2"
                onClick={handleAddToCart}
                disabled={product.isPreview || !selectedVariant || !(selectedVariant.inStock ?? true)}
              >
                <ShoppingBag className="w-5 h-5" />
                {product.isPreview
                  ? 'Покупка недоступна в режиме предпросмотра'
                  : (() => {
                      let vStock = product.variants?.[0]?.inStock ?? true;
                      if (product.variants && activeSize) {
                        const match = product.variants.find(v => v.size === activeSize && (!product.colors || !activeColor || v.color === product.colors[activeColor]?.name));
                        if (match) vStock = match.inStock ?? true;
                      }
                      return (selectedVariant?.inStock ?? true) ? 'Добавить в корзину' : 'Нет в наличии';
                    })()}
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="icon"
                className="h-12 w-12"
                disabled={product.isPreview}
                aria-label={liked ? 'Убрать из избранного' : 'Добавить в избранное'}
                onClick={() => {
                  if (product.isPreview) return;
                  if (!user) {
                    showToast('Войдите, чтобы добавить товар в избранное.');
                  }
                  toggleFavorite(product.id);
                  if (user) {
                    showToast(liked ? 'Удалено из избранного' : 'Добавлено в избранное');
                  }
                }}
              >
                <Heart className={cn("w-5 h-5", liked && "fill-current text-red-500")} />
              </Button>
            </div>

            <div className="mt-8 rounded-[14px] border border-dashed border-border-lighter bg-white p-4 text-sm text-ash dark:border-white/10 dark:bg-white/[0.02] flex items-center justify-between">
              <div>
                <p className="text-graphite dark:text-white font-medium">Продавец: {product.sellerName || product.brand}</p>
              </div>
              {product.sellerSlug && (
                <Link to={`/seller/${product.sellerSlug}`} className="text-primary hover:underline font-medium">
                  Перейти в магазин →
                </Link>
              )}
            </div>
            <div className="mt-6 grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="flex items-center gap-3 p-3 rounded-lg bg-ice/50 dark:bg-white/5">
                <Truck className="w-5 h-5 text-graphite dark:text-white" />
                <div>
                  <p className="text-xs font-medium text-graphite dark:text-white">Доставка</p>
                  <p className="text-[10px] text-ash">от 2 дней</p>
                </div>
              </div>
              <div className="flex items-center gap-3 p-3 rounded-lg bg-ice/50 dark:bg-white/5">
                <RefreshCw className="w-5 h-5 text-graphite dark:text-white" />
                <div>
                  <p className="text-xs font-medium text-graphite dark:text-white">Возврат</p>
                  <p className="text-[10px] text-ash">14 дней</p>
                </div>
              </div>
              <div className="flex items-center gap-3 p-3 rounded-lg bg-ice/50 dark:bg-white/5">
                <Shield className="w-5 h-5 text-graphite dark:text-white" />
                <div>
                  <p className="text-xs font-medium text-graphite dark:text-white">Гарантия</p>
                  <p className="text-[10px] text-ash">Оригинал</p>
                </div>
              </div>
            </div>

            {/* Accordions */}
            <div className="mt-6 border-t border-border-lighter dark:border-white/10">
              <AccordionSection title="Описание" defaultOpen={true}>
                <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed">
                  {product.description || 'Информация пока не указана.'}
                </p>
              </AccordionSection>

              <AccordionSection title="Состав и уход">
                {product.materialComposition && product.materialComposition.length > 0 ? (
                  <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed mb-3">
                    Состав: {product.materialComposition.map((mc: any) => `${mc.materialName} — ${mc.percentage}%`).join(', ')}
                  </p>
                ) : null}
                {product.careInstructions ? (
                  <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed mb-3">
                    Уход: {product.careInstructions}
                  </p>
                ) : null}
                {!(product.materialComposition && product.materialComposition.length > 0) && !product.careInstructions && (
                  <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed mb-3">
                    {product.materials || 'Информация пока не указана.'}
                  </p>
                )}
              </AccordionSection>

              <AccordionSection title="Характеристики">
                <div className="space-y-2">
                  {specs.map((spec) => (
                    <div key={spec.label} className="flex justify-between text-sm">
                      <span className="text-ash">{spec.label}</span>
                      <span className="text-graphite dark:text-white">{spec.value}</span>
                    </div>
                  ))}
                </div>
              </AccordionSection>

              <AccordionSection title="Доставка и возврат">
                <div className="text-sm text-graphite-light dark:text-white/70 space-y-2">
                  <p>• Бесплатная доставка от 10 000 ₽</p>
                  <p>• Доставка по России: 2-7 дней</p>
                  <p>• Возврат в течение 14 дней</p>
                  <Link to="/returns" className="inline-block mt-2 text-primary hover:underline">
                    Подробнее об условиях →
                  </Link>
                </div>
              </AccordionSection>
            </div>
          </div>
        </div>

        {/* Reviews Section */}
        <section className="mt-16">
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

      </div>
      {/* Size Chart Modal */}
      <Modal isOpen={showSizeChart} onClose={() => setShowSizeChart(false)} title="Как выбрать размер">
        {product.sizeChart && product.sizeChart.rows ? (
          <div className="flex flex-col md:flex-row gap-8">
            <div className="flex-1 overflow-x-auto">
              <h3 className="font-medium text-graphite dark:text-white mb-4">Размерная сетка</h3>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border-lighter dark:border-white/10">
                    <th className="py-3 px-4 text-left font-medium text-graphite dark:text-white">Размер</th>
                    {Object.keys(product.sizeChart.rows[0]?.measurements || {}).map((header) => {
                      const label = {
                        CHEST: 'Грудь', WAIST: 'Талия', HIPS: 'Бёдра', LENGTH: 'Длина изделия',
                        SLEEVE: 'Длина рукава', FOOT_LENGTH: 'Длина стопы', INSEAM: 'Внутренний шов',
                        HEAD_CIRCUMFERENCE: 'Обхват головы'
                      }[header] || header;
                      return (
                        <th key={header} className="py-3 px-4 text-left font-medium text-graphite dark:text-white">
                          {label}
                        </th>
                      );
                    })}
                  </tr>
                </thead>
                <tbody>
                  {product.sizeChart.rows.map((row: any, i: number) => (
                    <tr key={i} className="border-b border-border-lighter dark:border-white/10">
                      <td className="py-3 px-4 font-medium text-graphite dark:text-white">{row.sizeValueName}</td>
                      {Object.values(row.measurements || {}).map((val: any, j: number) => (
                        <td key={j} className="py-3 px-4 text-ash">{val}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="w-full md:w-64 flex-shrink-0 bg-ice/30 dark:bg-white/5 p-6 rounded-xl flex flex-col items-center">
              <h3 className="font-medium text-graphite dark:text-white mb-4 self-start">Как снять мерки</h3>
              <SizeGuideIllustration category={product.category || ''} />
              <div className="mt-6 space-y-3 w-full">
                {Object.keys(product.sizeChart.rows[0]?.measurements || {}).map((header) => {
                  const inst = {
                    CHEST: { l: 'Грудь', d: 'измеряйте горизонтально по наиболее выступающим точкам.' },
                    WAIST: { l: 'Талия', d: 'измеряйте вокруг естественной линии талии.' },
                    HIPS: { l: 'Бёдра', d: 'по наиболее выступающим точкам бёдер.' },
                    FOOT_LENGTH: { l: 'Длина стопы', d: 'от пятки до наиболее выступающего пальца.' }
                  }[header];
                  if (!inst) return null;
                  return (
                    <div key={header} className="text-xs">
                      <span className="font-medium text-graphite dark:text-white block">{inst.l}</span>
                      <span className="text-ash block">{inst.d}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        ) : (
          <div className="text-center text-ash p-4">Размерная сетка недоступна</div>
        )}
      </Modal>
    </div>
  );
}

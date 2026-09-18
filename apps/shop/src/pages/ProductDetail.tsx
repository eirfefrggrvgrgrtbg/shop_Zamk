import { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { useParams, Link } from 'react-router-dom';
import { ChevronRight, ChevronLeft, Heart, ShoppingBag, Star, ChevronDown, Eye, X } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { lockBodyScroll, unlockBodyScroll } from '../components/ui/scrollLock';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';
import { PreviewPageMetadata } from '../components/PreviewPageMetadata';
import { formatPrice, cn } from '../lib/utils';
import { fetchProductById, fetchProductReviews, fetchProductPreviewByToken } from '../api/publicCatalog';
import { recordProductView } from '@zamk/api-client/src/customer';
import { useVariantSelection } from '../lib/variantSelection';
import { SimilarProductsBlock } from '../components/product/SimilarProductsBlock';
import type { Product, Review } from '../types/catalog';

export interface MeasurementMeta {
  label: string;      // with unit e.g. "Грудь, см"
  shortLabel: string; // concise label e.g. "Грудь"
  instruction: string;// explanation e.g. "Горизонтально по наиболее выступающим точкам груди."
}

export const MEASUREMENT_FIELDS_MAP: Record<string, MeasurementMeta> = {
  CHEST: {
    label: 'Грудь, см',
    shortLabel: 'Грудь',
    instruction: 'Горизонтально по наиболее выступающим точкам груди.',
  },
  WAIST: {
    label: 'Талия, см',
    shortLabel: 'Талия',
    instruction: 'Горизонтально вокруг самой узкой части талии.',
  },
  HIPS: {
    label: 'Бёдра, см',
    shortLabel: 'Бёдра',
    instruction: 'Горизонтально по наиболее выступающим точкам ягодиц.',
  },
  LENGTH: {
    label: 'Длина изделия, см',
    shortLabel: 'Длина изделия',
    instruction: 'Вертикально от высшей точки плеча до нижнего края изделия.',
  },
  SLEEVE: {
    label: 'Длина рукава, см',
    shortLabel: 'Длина рукава',
    instruction: 'От плечевого шва по внешней стороне руки до запястья.',
  },
  INSEAM: {
    label: 'Внутренний шов, см',
    shortLabel: 'Внутренний шов',
    instruction: 'По внутреннему шву брючины от промежности до низа изделия.',
  },
  FOOT_LENGTH: {
    label: 'Длина стопы, см',
    shortLabel: 'Длина стопы',
    instruction: 'От задней точки пятки до кончика самого длинного пальца стопы.',
  },
  HEAD_CIRCUMFERENCE: {
    label: 'Обхват головы, см',
    shortLabel: 'Обхват головы',
    instruction: 'По окружности головы над бровями и ушами.',
  },
};

export const getMeasurementMeta = (field: string): MeasurementMeta => {
  if (MEASUREMENT_FIELDS_MAP[field]) {
    return MEASUREMENT_FIELDS_MAP[field];
  }
  const formatted = field.toLowerCase().replace(/_/g, ' ');
  return {
    label: `${formatted}, см`,
    shortLabel: formatted,
    instruction: 'Измеряйте согласно стандартам производителя.',
  };
};

export function isLightColor(hex?: string): boolean {
  if (!hex) return false;
  let c = hex.trim().replace('#', '');
  if (c.length === 3) {
    c = c.split('').map(x => x + x).join('');
  }
  if (c.length !== 6) return false;
  const r = parseInt(c.substring(0, 2), 16);
  const g = parseInt(c.substring(2, 4), 16);
  const b = parseInt(c.substring(4, 6), 16);
  if (isNaN(r) || isNaN(g) || isNaN(b)) return false;
  const brightness = (r * 299 + g * 587 + b * 114) / 1000;
  return brightness > 190;
}

export const SizeGuideIllustration = ({ activeFields }: { activeFields: string[] }) => {
  const isFootwear = activeFields.includes('FOOT_LENGTH');
  const isHeadwear = activeFields.includes('HEAD_CIRCUMFERENCE') && !activeFields.includes('CHEST') && !activeFields.includes('WAIST');
  const isBottoms = activeFields.includes('INSEAM') || (
    (activeFields.includes('WAIST') || activeFields.includes('HIPS')) &&
    !activeFields.includes('CHEST') &&
    !activeFields.includes('SLEEVE')
  );

  if (isFootwear) {
    return (
      <svg viewBox="0 0 240 160" className="w-full max-w-[220px] h-auto" fill="none" xmlns="http://www.w3.org/2000/svg">
        {/* Footwear silhouette */}
        <path
          d="M30 110 C30 85 45 75 75 75 C100 75 125 70 145 50 C160 35 180 35 195 55 C210 75 220 95 220 115 C220 125 210 130 190 130 C130 130 90 130 30 130 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
        {/* Sole line */}
        <path d="M25 130 L220 130" className="stroke-graphite/40 dark:stroke-white/40" strokeWidth="1.5" />

        {/* Foot length dimension */}
        {activeFields.includes('FOOT_LENGTH') && (
          <g className="text-graphite dark:text-white">
            <line x1="30" y1="145" x2="220" y2="145" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="30" y1="140" x2="30" y2="150" stroke="currentColor" strokeWidth="1.5" />
            <line x1="220" y1="140" x2="220" y2="150" stroke="currentColor" strokeWidth="1.5" />
            <text x="125" y="157" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Длина стопы
            </text>
          </g>
        )}
      </svg>
    );
  }

  if (isHeadwear) {
    return (
      <svg viewBox="0 0 240 180" className="w-full max-w-[220px] h-auto" fill="none" xmlns="http://www.w3.org/2000/svg">
        {/* Cap/Beanie silhouette */}
        <path
          d="M50 130 C45 70 75 35 120 35 C165 35 195 70 190 130 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
        />
        {activeFields.includes('HEAD_CIRCUMFERENCE') && (
          <g className="text-graphite dark:text-white">
            <line x1="45" y1="130" x2="195" y2="130" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <circle cx="45" cy="130" r="2.5" fill="currentColor" />
            <circle cx="195" cy="130" r="2.5" fill="currentColor" />
            <text x="120" y="148" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Обхват головы
            </text>
          </g>
        )}
      </svg>
    );
  }

  if (isBottoms) {
    return (
      <svg viewBox="0 0 240 240" className="w-full max-w-[220px] h-auto" fill="none" xmlns="http://www.w3.org/2000/svg">
        {/* Pants silhouette */}
        <path
          d="M70 35 L170 35 L175 75 L160 215 L125 215 L120 100 L115 215 L80 215 L65 75 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />

        {/* WAIST */}
        {activeFields.includes('WAIST') && (
          <g className="text-graphite dark:text-white">
            <line x1="65" y1="35" x2="175" y2="35" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="65" y1="31" x2="65" y2="39" stroke="currentColor" strokeWidth="1.5" />
            <line x1="175" y1="31" x2="175" y2="39" stroke="currentColor" strokeWidth="1.5" />
            <text x="120" y="27" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Талия
            </text>
          </g>
        )}

        {/* HIPS */}
        {activeFields.includes('HIPS') && (
          <g className="text-graphite dark:text-white">
            <line x1="60" y1="75" x2="180" y2="75" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="60" y1="71" x2="60" y2="79" stroke="currentColor" strokeWidth="1.5" />
            <line x1="180" y1="71" x2="180" y2="79" stroke="currentColor" strokeWidth="1.5" />
            <text x="120" y="70" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Бёдра
            </text>
          </g>
        )}

        {/* INSEAM */}
        {activeFields.includes('INSEAM') && (
          <g className="text-graphite dark:text-white">
            <line x1="120" y1="100" x2="95" y2="215" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <circle cx="120" cy="100" r="2" fill="currentColor" />
            <circle cx="95" cy="215" r="2" fill="currentColor" />
            <text x="128" y="160" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="start">
              Шов
            </text>
          </g>
        )}

        {/* LENGTH */}
        {activeFields.includes('LENGTH') && (
          <g className="text-graphite dark:text-white">
            <line x1="50" y1="35" x2="50" y2="215" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="46" y1="35" x2="54" y2="35" stroke="currentColor" strokeWidth="1.5" />
            <line x1="46" y1="215" x2="54" y2="215" stroke="currentColor" strokeWidth="1.5" />
            <text x="42" y="125" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="end">
              Длина
            </text>
          </g>
        )}
      </svg>
    );
  }

  // Upper Apparel (Default)
  return (
    <svg viewBox="0 0 240 240" className="w-full max-w-[220px] h-auto" fill="none" xmlns="http://www.w3.org/2000/svg">
      {/* Upper apparel silhouette */}
      <path
        d="M85 35 Q120 48 155 35 L205 75 L180 115 L160 95 L160 210 L80 210 L80 95 L60 115 L35 75 Z"
        className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />

      {/* CHEST (Width across chest) */}
      {activeFields.includes('CHEST') && (
        <g className="text-graphite dark:text-white">
          <line x1="75" y1="110" x2="165" y2="110" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="75" y1="105" x2="75" y2="115" stroke="currentColor" strokeWidth="1.5" />
          <line x1="165" y1="105" x2="165" y2="115" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="103" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Грудь
          </text>
        </g>
      )}

      {/* LENGTH (Garment length from shoulder to hem) */}
      {activeFields.includes('LENGTH') && (
        <g className="text-graphite dark:text-white">
          <line x1="70" y1="40" x2="70" y2="210" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="66" y1="40" x2="74" y2="40" stroke="currentColor" strokeWidth="1.5" />
          <line x1="66" y1="210" x2="74" y2="210" stroke="currentColor" strokeWidth="1.5" />
          <text x="62" y="135" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="end">
            Длина
          </text>
        </g>
      )}

      {/* SLEEVE (Shoulder seam to cuff) */}
      {activeFields.includes('SLEEVE') && (
        <g className="text-graphite dark:text-white">
          <line x1="155" y1="35" x2="195" y2="95" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <circle cx="155" cy="35" r="2" fill="currentColor" />
          <circle cx="195" cy="95" r="2" fill="currentColor" />
          <text x="185" y="60" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="start">
            Рукав
          </text>
        </g>
      )}

      {/* WAIST (Only if WAIST is active in this schema!) */}
      {activeFields.includes('WAIST') && (
        <g className="text-graphite dark:text-white">
          <line x1="80" y1="155" x2="160" y2="155" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="80" y1="151" x2="80" y2="159" stroke="currentColor" strokeWidth="1.5" />
          <line x1="160" y1="151" x2="160" y2="159" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="150" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Талия
          </text>
        </g>
      )}

      {/* HIPS (Only if HIPS is active in this schema!) */}
      {activeFields.includes('HIPS') && (
        <g className="text-graphite dark:text-white">
          <line x1="80" y1="200" x2="160" y2="200" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="80" y1="196" x2="80" y2="204" stroke="currentColor" strokeWidth="1.5" />
          <line x1="160" y1="196" x2="160" y2="204" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="195" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Бёдра
          </text>
        </g>
      )}
    </svg>
  );
};

const isUUID = (str?: string) =>
  Boolean(str && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(str));

function formatReviewsCount(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'отзывов';
  if (mod10 === 1) return 'отзыв';
  if (mod10 >= 2 && mod10 <= 4) return 'отзыва';
  return 'отзывов';
}

const getProductSpecs = (product: Product, selectedVariant?: any) =>
  [
    (selectedVariant?.sellerSku || (product.id && !isUUID(product.id)))
      ? { label: 'Артикул', value: (selectedVariant?.sellerSku || product.id).toUpperCase() }
      : (selectedVariant?.sku ? { label: 'Артикул', value: selectedVariant.sku.toUpperCase() } : null),
    product.brand && product.brand !== 'Бренд не указан' && !isUUID(product.brand)
      ? { label: 'Бренд', value: product.brand }
      : null,
    product.category && product.category !== 'Категория не указана' && !isUUID(product.category)
      ? { label: 'Категория', value: product.category }
      : null,
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

export function ProductDetail() {
  const { id, token } = useParams<{ id?: string; token?: string }>();

  const [product, setProduct] = useState<Product | null>(null);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [activeImage, setActiveImage] = useState(0);
  const [isLightboxOpen, setIsLightboxOpen] = useState(false);
  const [isZoomed, setIsZoomed] = useState(false);
  const [panOffset, setPanOffset] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const dragStartRef = useRef<{ startX: number; startY: number; initialPanX: number; initialPanY: number }>({
    startX: 0,
    startY: 0,
    initialPanX: 0,
    initialPanY: 0,
  });
  const hasMovedRef = useRef(false);
  const [showSizeChart, setShowSizeChart] = useState(false);
  const [sizeError, setSizeError] = useState('');
  const mainImageTriggerRef = useRef<HTMLButtonElement | null>(null);
  const lightboxDialogRef = useRef<HTMLDivElement | null>(null);

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
  } = useVariantSelection(product?.variants, product?.sizeChart);

  const defaultImage = { url: 'https://placehold.co/400x500/e2e8f0/64748b?text=No+Image' };

  // Product-level media stream: photos belong to the product as a whole.
  // One stable, ordered media collection without filtering by selectedColor or selectedVariant.
  const visibleImages: { url: string; colorId?: string }[] = (() => {
    if (product?.images && product.images.length > 0) {
      // Deduplicate by URL while preserving deterministic order
      const seen = new Set<string>();
      const result: { url: string; colorId?: string }[] = [];
      for (const img of product.images) {
        if (img?.url && !seen.has(img.url)) {
          seen.add(img.url);
          result.push({ url: img.url, colorId: img.colorId });
        }
      }
      if (result.length > 0) return result;
    }
    if (product?.image) {
      return [{ url: product.image }];
    }
    return [defaultImage];
  })();

  const resetZoom = () => {
    setIsZoomed(false);
    setPanOffset({ x: 0, y: 0 });
    setIsDragging(false);
  };

  // Lightbox keyboard controls, scroll locking, and focus containment (must be unconditional hook)
  useEffect(() => {
    if (!isLightboxOpen) {
      resetZoom();
      return;
    }

    // Move focus inside the lightbox modal dialog upon open
    const focusTimer = setTimeout(() => {
      lightboxDialogRef.current?.focus();
    }, 0);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        setIsLightboxOpen(false);
        resetZoom();
        mainImageTriggerRef.current?.focus();
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        resetZoom();
        setActiveImage((prev) => (prev - 1 + visibleImages.length) % visibleImages.length);
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        resetZoom();
        setActiveImage((prev) => (prev + 1) % visibleImages.length);
      } else if (e.key === 'Tab') {
        // Focus trap inside lightbox modal
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
  }, [isLightboxOpen, visibleImages.length]);

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
    selectColor(colorId);
    // Preserves active photo index across color switches.
    // Gallery media stream belongs to the product as a whole.
    if (sizeError) setSizeError('');
  };

  const liked = isFavorite(product.id);

  const specs = getProductSpecs(product, selectedVariant);

  const currentActiveImage = activeImage < visibleImages.length ? activeImage : 0;
  const currentImageUrl = visibleImages[currentActiveImage]?.url || defaultImage.url;

  const activeFields: string[] = product.sizeChart?.rows?.[0]?.measurements
    ? Object.keys(product.sizeChart.rows[0].measurements)
    : [];

  const handlePrevImage = () => {
    if (visibleImages.length <= 1) return;
    resetZoom();
    setActiveImage((prev) => (prev - 1 + visibleImages.length) % visibleImages.length);
  };

  const handleNextImage = () => {
    if (visibleImages.length <= 1) return;
    resetZoom();
    setActiveImage((prev) => (prev + 1) % visibleImages.length);
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


  const handleAddToCart = async () => {
    if (product.isPreview) return;
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
      await addItem(product.id, selectedVariant.id, 1);
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

      <div className="container mx-auto px-4 sm:px-6 lg:px-8 max-w-[1324px]">

        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-xs sm:text-sm text-ash mb-5 sm:mb-6 flex-wrap">
          <Link to="/" className="hover:text-graphite dark:hover:text-white transition-colors">Главная</Link>
          <ChevronRight className="w-3.5 h-3.5" />
          <Link to="/catalog" className="hover:text-graphite dark:hover:text-white transition-colors">Каталог</Link>
          <ChevronRight className="w-3.5 h-3.5" />
          <span className="text-graphite dark:text-white line-clamp-1">{product.name}</span>
        </nav>

        {/* Opaque Product Canvas — isolates product reading area from background diagonal pattern */}
        <div className="bg-white dark:bg-[#121214] border border-border-lighter/70 dark:border-white/5 rounded-2xl p-5 sm:p-7 min-[1200px]:p-8 shadow-xs">

          {/* HERO SECTION: Gallery ~704px, Gap 56px, Info ~500px on desktop */}
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
                  {visibleImages.map((image: any, index: number) => {
                    const isSelected = currentActiveImage === index;
                    return (
                      <button
                        key={index + image.url}
                        type="button"
                        onClick={() => setActiveImage(index)}
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

              {/* Main Stage: Reduced to 500-520px wide, 620-650px max-height to avoid media dominance */}
              <div className="flex-1 w-full max-w-[520px] mx-auto min-[960px]:mx-0">
                <div className="group relative bg-[#f5f5f7] dark:bg-[#1a1a1c] border border-black/5 dark:border-white/5 rounded-xl overflow-hidden w-full aspect-[4/5] max-h-[640px] flex items-center justify-center select-none">
                  {/* Base Image */}
                  <img
                    key={currentImageUrl}
                    src={currentImageUrl}
                    alt={product.name}
                    className="w-full h-full object-contain mix-blend-multiply dark:mix-blend-normal transition-opacity duration-200 pointer-events-none"
                  />

                  {/* Interactive Click Zones */}
                  {visibleImages.length > 1 ? (
                    <div className="absolute inset-0 flex">
                      {/* Left Navigation Zone (~28%) */}
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

                      {/* Center Zoom Zone (~44%) */}
                      <button
                        ref={mainImageTriggerRef}
                        type="button"
                        onClick={() => setIsLightboxOpen(true)}
                        aria-label="Открыть изображение на полный экран"
                        className="w-[44%] h-full cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                      />

                      {/* Right Navigation Zone (~28%) */}
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
                    /* Single-image zoom-only target */
                    <button
                      ref={mainImageTriggerRef}
                      type="button"
                      onClick={() => setIsLightboxOpen(true)}
                      aria-label="Открыть изображение на полный экран"
                      className="absolute inset-0 w-full h-full cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-graphite dark:focus-visible:ring-white"
                    />
                  )}

                  {/* Image Counter Badge */}
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
                    {visibleImages.map((image: any, index: number) => {
                      const isSelected = currentActiveImage === index;
                      return (
                        <button
                          key={index + image.url}
                          type="button"
                          onClick={() => setActiveImage(index)}
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
                  <Link
                    to={`/brand/${product.brandId}`}
                    data-testid="product-brand-link"
                    className="text-xs font-semibold uppercase tracking-widest text-ash hover:text-graphite dark:hover:text-white transition-colors w-fit"
                  >
                    {product.brand}
                  </Link>
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
                  {(product.reviewsCount ?? (reviews ? reviews.length : 0)) > 0 && (
                    <>
                      <span className="text-ash">·</span>
                      <button
                        type="button"
                        onClick={() => {
                          const el = document.getElementById('product-reviews-section');
                          el?.scrollIntoView({ behavior: 'smooth' });
                        }}
                        className="text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                      >
                        {product.reviewsCount ?? reviews.length} {formatReviewsCount(product.reviewsCount ?? reviews.length)}
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
                      {formatPrice(getDisplayPrice())}
                    </span>
                  </>
                ) : (
                  <span className="text-[26px] min-[1200px]:text-[28px] font-semibold text-graphite dark:text-white">
                    {formatPrice(getDisplayPrice())}
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
                          aria-checked={isSelected}
                          aria-label={color.name + (color.shadeName ? ` (${color.shadeName})` : '') + (!color.hasInStock ? ' (нет в наличии)' : '')}
                          title={color.name + (color.shadeName ? ` (${color.shadeName})` : '') + (!color.hasInStock ? ' (нет в наличии)' : '')}
                          onClick={() => handleColorChange(color.id)}
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
                      return (
                        <button
                          key={sizeObj.id}
                          type="button"
                          aria-pressed={isSelected}
                          disabled={sizeObj.disabled}
                          onClick={() => {
                            if (!sizeObj.disabled) {
                              selectSize(sizeObj.id);
                              if (sizeError) setSizeError('');
                            }
                          }}
                          className={cn(
                            "min-w-[48px] h-11 px-3.5 rounded-md text-sm font-medium transition-colors relative flex items-center justify-center cursor-pointer",
                            isSelected
                              ? "bg-graphite text-white dark:bg-white dark:text-black border border-graphite dark:border-white shadow-xs"
                              : sizeObj.disabled
                                ? "border border-border-lighter/60 dark:border-white/10 text-ash/50 dark:text-white/30 cursor-not-allowed line-through bg-ice/30 dark:bg-white/[0.02]"
                                : "bg-white dark:bg-transparent border border-border-soft dark:border-white/20 text-graphite dark:text-white hover:bg-[#fafafb] dark:hover:bg-white/5 hover:border-graphite/40 dark:hover:border-white/40"
                          )}
                        >
                          {sizeObj.label}
                        </button>
                      );
                    })}
                  </div>
                  {sizeSelectionNotice && (
                    <p className="mt-2 text-xs sm:text-sm text-amber-600 dark:text-amber-400 font-normal" role="status" aria-live="polite">
                      {sizeSelectionNotice}
                    </p>
                  )}
                  {sizeError && (
                    <p className="mt-2 text-xs sm:text-sm text-error" role="alert">
                      {sizeError}
                    </p>
                  )}
                </div>
              )}

              {/* PRIMARY CTA + FAVORITE */}
              <div className="mt-6 flex gap-3">
                <Button
                  type="button"
                  variant="primary"
                  className="flex-1 h-[52px] rounded-lg text-sm font-medium tracking-wide gap-2"
                  onClick={handleAddToCart}
                  disabled={product.isPreview || !isResolved || !canAddToCart}
                >
                  <ShoppingBag className="w-4 h-4" />
                  {product.isPreview
                    ? 'Покупка недоступна в предпросмотре'
                    : ctaText}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  size="icon"
                  className="h-[52px] w-[52px] shrink-0 rounded-lg border border-border-soft dark:border-white/20 hover:border-graphite/40 dark:hover:border-white/40"
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
                  <Heart className={cn("w-5 h-5 transition-colors", liked && "fill-current text-red-500 stroke-red-500")} />
                </Button>
              </div>

              {/* COMPACT TEXTUAL SERVICE ROWS */}
              <div className="mt-6 pt-5 border-t border-border-lighter dark:border-white/10 space-y-2.5 text-xs sm:text-sm text-ash">
                <div className="flex items-center justify-between">
                  <span>Доставка: по России от 2 дней</span>
                  <Link to="/delivery" className="text-graphite dark:text-white hover:underline text-xs">
                    Подробнее →
                  </Link>
                </div>
                <div className="flex items-center justify-between">
                  <span>Возврат: в течение 14 дней</span>
                  <Link to="/returns" className="text-graphite dark:text-white hover:underline text-xs">
                    Условия →
                  </Link>
                </div>
                {product.sellerName && (
                  <div className="flex items-center justify-between">
                    <span>Продавец: <strong className="text-graphite dark:text-white font-medium">{product.sellerName}</strong></span>
                    {product.sellerSlug && (
                      <Link to={`/seller/${product.sellerSlug}`} className="text-graphite dark:text-white hover:underline text-xs font-medium">
                        В магазин →
                      </Link>
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
                      {product.materialComposition.map((mc: any) => `${mc.materialName || mc.material} — ${mc.percentage}%`).join(', ')}
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

            {/* Lower Accordions: Характеристики, Размер и посадка, Доставка и возврат */}
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

              {product.sizeChart?.rows?.length > 0 && (
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
                  <Link to="/returns" className="inline-block mt-2 text-graphite dark:text-white hover:underline font-medium">
                    Подробнее об условиях возврата →
                  </Link>
                </div>
              </AccordionSection>
            </div>
          </div>
        </div>

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

      {/* Size Chart Modal */}
      <Modal
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
                    {product.sizeChart.rows.map((row: any, i: number) => (
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
      </Modal>

      {/* Lightbox / Fullscreen Modal: True top-level React Portal into document.body */}
      {isLightboxOpen && typeof document !== 'undefined' && createPortal(
        <div
          ref={lightboxDialogRef}
          tabIndex={-1}
          role="dialog"
          aria-modal="true"
          aria-label="Просмотр фотографии на весь экран"
          className="fixed inset-0 z-[200] flex items-center justify-center bg-[#0a0a0c] select-none outline-none"
        >
          {/* Top Bar: Minimal Viewer Controls (Counter & Close) */}
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

          {/* Centered Large Image Stage: Dedicated viewer fitting viewport cleanly with real 2x zoom and pan */}
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
                // Constrain panning to keep garment reasonably in view
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
            <div
              className="absolute bottom-4 inset-x-0 flex justify-center gap-2 px-4 overflow-x-auto scrollbar-none z-20"
            >
              {visibleImages.map((img: any, idx: number) => {
                const isSelected = idx === currentActiveImage;
                return (
                  <button
                    key={'lb-' + idx + img.url}
                    type="button"
                    onClick={() => setActiveImage(idx)}
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
    </div>
  );
}

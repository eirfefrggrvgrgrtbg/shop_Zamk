import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ChevronRight, Heart, Minus, Plus, Ruler, ShoppingBag, Star, Truck, RefreshCw, Shield, ChevronDown, Eye } from 'lucide-react';
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
  const [mediaOrientations, setMediaOrientations] = useState<Record<string, 'portrait' | 'landscape' | 'square'>>({});

  const handleImageLoad = (url: string, e: React.SyntheticEvent<HTMLImageElement>) => {
    const img = e.currentTarget;
    if (!img.naturalWidth || !img.naturalHeight) return;
    const ratio = img.naturalWidth / img.naturalHeight;
    let orientation: 'portrait' | 'landscape' | 'square' = 'portrait';
    if (ratio > 1.15) {
      orientation = 'landscape';
    } else if (ratio >= 0.85) {
      orientation = 'square';
    } else {
      orientation = 'portrait';
    }
    setMediaOrientations(prev => (prev[url] === orientation ? prev : { ...prev, [url]: orientation }));
  };

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

  const colorMap = new Map<string, { id: string, name: string, hex: string, shadeName?: string }>();
  product.variants?.forEach(v => {
    if (v.colorId && v.colorName) {
      if (!colorMap.has(v.colorId)) {
        colorMap.set(v.colorId, {
          id: v.colorId,
          name: v.colorName,
          hex: v.colorHex || '#71717a',
          shadeName: (v as any).colorShadeName || undefined
        });
      }
    }
  });
  const colors = Array.from(colorMap.values());
  const activeColorObj = colors[activeColor] || null;

  const handleColorChange = (newIndex: number) => {
    setActiveColor(newIndex);
    setActiveImage(0);
    const newColorObj = colors[newIndex];
    if (activeSize && newColorObj) {
      const sizeStillValid = product.variants?.some(
        v => v.isActive && v.colorId === newColorObj.id && v.size === activeSize
      );
      if (!sizeStillValid) {
        setActiveSize(null);
      }
    }
  };

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
  const currentImageUrl = visibleImages[currentActiveImage]?.url || defaultImage.url;
  const currentOrientation = mediaOrientations[currentImageUrl] || 'portrait';

  const activeFields: string[] = product.sizeChart?.rows?.[0]?.measurements
    ? Object.keys(product.sizeChart.rows[0].measurements)
    : [];

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
            {/* Main image container */}
            <div
              className={cn(
                "relative bg-[#f5f5f7] dark:bg-[#1a1a1c] border border-black/5 dark:border-white/5 rounded-2xl overflow-hidden w-full flex items-center justify-center transition-all duration-300",
                currentOrientation === 'landscape'
                  ? "aspect-[16/10] max-h-[58vh]"
                  : currentOrientation === 'square'
                  ? "aspect-square max-h-[70vh]"
                  : "aspect-[4/5] max-h-[78vh]"
              )}
            >
              <img
                key={currentImageUrl}
                src={currentImageUrl}
                alt={product.name}
                onLoad={(e) => handleImageLoad(currentImageUrl, e)}
                className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal transition-opacity duration-200"
              />
              {product.isNew && (
                <span className="absolute top-4 left-4 px-3 py-1 rounded-md bg-graphite text-white dark:bg-white dark:text-black text-xs font-semibold uppercase tracking-wider shadow-sm">
                  Новинка
                </span>
              )}
              {product.discountPrice && (
                <span className="absolute top-4 right-4 px-3 py-1 rounded-md bg-red-500 text-white text-xs font-semibold uppercase tracking-wider shadow-sm">
                  -{Math.round((1 - product.discountPrice / product.price) * 100)}%
                </span>
              )}
            </div>

            {/* Thumbnails */}
            {visibleImages.length > 1 && (
              <div className="flex gap-2.5 overflow-x-auto pb-2 scrollbar-hide">
                {visibleImages.map((image: any, index: number) => (
                  <button
                    key={index + image.url}
                    type="button"
                    onClick={() => setActiveImage(index)}
                    className={cn(
                      "w-20 h-20 flex-shrink-0 rounded-xl overflow-hidden bg-[#f5f5f7] dark:bg-[#1a1a1c] border-2 transition-all p-1 flex items-center justify-center",
                      currentActiveImage === index
                        ? "border-graphite dark:border-white ring-1 ring-graphite/20 dark:ring-white/20 scale-[1.02]"
                        : "border-transparent hover:border-border-lighter dark:hover:border-white/20 opacity-80 hover:opacity-100"
                    )}
                  >
                    <img
                      src={image.url}
                      alt=""
                      className="max-w-full max-h-full object-contain mix-blend-multiply dark:mix-blend-normal"
                    />
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
                <div className="flex items-center justify-between mb-2.5">
                  <p className="text-sm font-medium text-graphite dark:text-white">
                    Цвет:{' '}
                    <span className="text-ash font-normal">
                      {activeColorObj?.name || 'Не выбран'}
                      {activeColorObj?.shadeName ? ` (${activeColorObj.shadeName})` : ''}
                    </span>
                  </p>
                </div>
                <div className="flex flex-wrap gap-3 items-center" role="radiogroup" aria-label="Выбор цвета">
                  {colors.map((color, index) => {
                    const isSelected = activeColor === index;
                    const isWhiteOrLight = isLightColor(color.hex);
                    return (
                      <button
                        key={color.id}
                        type="button"
                        role="radio"
                        aria-checked={isSelected}
                        aria-label={color.name + (color.shadeName ? ` (${color.shadeName})` : '')}
                        title={color.name + (color.shadeName ? ` (${color.shadeName})` : '')}
                        onClick={() => handleColorChange(index)}
                        className={cn(
                          "relative w-9 h-9 rounded-full transition-all duration-150 flex items-center justify-center",
                          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2",
                          isSelected
                            ? "ring-2 ring-graphite dark:ring-white ring-offset-2 ring-offset-white dark:ring-offset-[#121214] scale-105"
                            : "hover:scale-105 hover:ring-1 hover:ring-black/20 dark:hover:ring-white/30 opacity-90 hover:opacity-100"
                        )}
                      >
                        {/* Swatch circle with crisp border ensuring light/white colors are clearly visible */}
                        <span
                          style={{ backgroundColor: color.hex || '#71717a' }}
                          className={cn(
                            "w-full h-full rounded-full border shadow-inner",
                            isWhiteOrLight
                              ? "border-black/25 dark:border-white/30"
                              : "border-black/10 dark:border-white/15"
                          )}
                        />
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Sizes */}
            {requiresSizeSelection && (
              <div className="mt-6">
                <div className="flex items-center justify-between mb-2">
                  <p className="text-sm font-medium text-graphite dark:text-white">
                    Размер: <span className="text-ash font-normal">{activeSize || 'Не выбран'}</span>
                  </p>
                  <button
                    type="button"
                    onClick={() => setShowSizeChart(true)}
                    className="flex items-center gap-1.5 text-sm text-primary hover:underline font-medium"
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
                            ? "bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white shadow-sm"
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
              <p className="text-sm font-medium text-graphite dark:text-white mb-2">Количество</p>
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
    </div>
  );
}

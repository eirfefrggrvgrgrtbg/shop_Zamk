import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ChevronRight, Heart, Minus, Plus, Ruler, ShoppingBag, Star, Truck, RefreshCw, Shield, ChevronDown } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { ProductCard } from '../components/product/ProductCard';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useToast } from '../contexts/ToastContext';
import { formatPrice, cn } from '../lib/utils';
import { PRODUCTS, getProductById, getBrandById, getProductsByBrand, getSellerById } from '../lib/mock-data';
import { SectionHeader } from '../components/editorial/StudioKit';

// Размерная сетка
const SIZE_CHART = {
  headers: ['Размер', 'Грудь (см)', 'Талия (см)', 'Бёдра (см)'],
  rows: [
    ['XS', '82-86', '62-66', '88-92'],
    ['S', '86-90', '66-70', '92-96'],
    ['M', '90-94', '70-74', '96-100'],
    ['L', '94-98', '74-78', '100-104'],
    ['XL', '98-102', '78-82', '104-108'],
    ['XXL', '102-106', '82-86', '108-112'],
  ],
};

// Характеристики товара
const getProductSpecs = (product: any) => [
  { label: 'Артикул', value: product.id.toUpperCase() },
  { label: 'Бренд', value: product.brand },
  { label: 'Категория', value: product.category === 'clothing' ? 'Одежда' : product.category === 'bags' ? 'Сумки' : product.category === 'shoes' ? 'Обувь' : product.category === 'accessories' ? 'Аксессуары' : 'Украшения' },
  { label: 'Сезон', value: 'Демисезон' },
  { label: 'Страна производства', value: 'Италия' },
];

// Аккордеон секция
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
  const { id } = useParams<{ id: string }>();
  const product = getProductById(id || '');
    const seller = product ? getSellerById(product.sellerId || '') : null;
  const [activeImage, setActiveImage] = useState(0);
  const [activeSize, setActiveSize] = useState<string | null>(null);
  const [activeColor, setActiveColor] = useState(0);
  const [quantity, setQuantity] = useState(1);
  const [showSizeChart, setShowSizeChart] = useState(false);
  const [activeTab, setActiveTab] = useState<'description' | 'specs' | 'reviews'>('description');

  const { addItem } = useCart();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();

  if (!product) {
    return (
      <div className="relative z-10 min-h-screen pt-36 pb-20">
        <div className="container mx-auto px-4 sm:px-6 max-w-4xl text-center">
          <h1 className="text-4xl font-serif text-graphite dark:text-white">Товар не найден</h1>
          <Link to="/catalog" className="inline-block mt-6">
            <Button>Вернуться в каталог</Button>
          </Link>
        </div>
      </div>
    );
  }

  const images = product.images || [product.image];
  const relatedProducts = PRODUCTS.filter((item) => item.category === product.category && item.id !== product.id).slice(0, 4);
  const brandProducts = getProductsByBrand(product.brandId).filter((item) => item.id !== product.id).slice(0, 4);
  const brand = getBrandById(product.brandId);
  const liked = isFavorite(product.id);
  const specs = getProductSpecs(product);

  const handleAddToCart = () => {
    if (product.sizes && product.sizes[0] !== 'Единый' && !activeSize) {
      showToast('Выберите размер');
      return;
    }
    for (let i = 0; i < quantity; i++) {
      addItem(product, activeSize || undefined, product.colors?.[activeColor]?.name);
    }
    showToast('Товар добавлен в корзину');
  };

  return (
    <div className="relative z-10 min-h-screen pt-24 md:pt-28 pb-20">
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
            <div className="relative bg-[#f5f5f7] dark:bg-[#1a1a1c] rounded-xl overflow-hidden" style={{ aspectRatio: '4/5' }}>
              <img
                src={images[activeImage]}
                alt={product.name}
                className="w-full h-full object-contain p-4"
              />
              {/* Badges */}
              {product.isNew && (
                <span className="absolute top-4 left-4 px-3 py-1 rounded bg-graphite text-white dark:text-black text-xs font-semibold uppercase">
                  New
                </span>
              )}
              {product.discountPrice && (
                <span className="absolute top-4 right-4 px-3 py-1 rounded bg-red-500 text-white text-xs font-semibold uppercase">
                  -{Math.round((1 - product.discountPrice / product.price) * 100)}%
                </span>
              )}
            </div>

            {/* Thumbnails */}
            {images.length > 1 && (
              <div className="flex gap-2 overflow-x-auto pb-2">
                {images.map((image, index) => (
                  <button
                    key={image}
                    onClick={() => setActiveImage(index)}
                    className={cn(
                      "w-20 h-20 flex-shrink-0 rounded-lg overflow-hidden bg-[#f5f5f7] dark:bg-[#1a1a1c] border-2 transition-colors",
                      activeImage === index ? "border-graphite dark:border-white" : "border-transparent"
                    )}
                  >
                    <img src={image} alt="" className="w-full h-full object-contain p-1" />
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Product Info */}
          <div className="lg:sticky lg:top-28 lg:self-start">
            {/* Brand */}
            <Link to={`/brand/${product.brandId}`} className="text-sm text-ash hover:text-graphite dark:hover:text-white transition-colors">
              {product.brand}
            </Link>

            {/* Title */}
            <h1 className="mt-2 text-2xl md:text-3xl font-serif text-graphite dark:text-white leading-tight">
              {product.name}
            </h1>

            {/* Rating */}
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

            {/* Price */}
            <div className="mt-4 flex items-baseline gap-3">
              {product.discountPrice ? (
                <>
                  <span className="text-2xl font-semibold text-red-500">{formatPrice(product.discountPrice)}</span>
                  <span className="text-lg text-ash line-through">{formatPrice(product.price)}</span>
                </>
              ) : (
                <span className="text-2xl font-semibold text-graphite dark:text-white">{formatPrice(product.price)}</span>
              )}
            </div>

            {/* Colors */}
            {product.colors && (
              <div className="mt-6">
                <p className="text-sm text-graphite dark:text-white mb-2">
                  Цвет: <span className="text-ash">{product.colors[activeColor].name}</span>
                </p>
                <div className="flex gap-2">
                  {product.colors.map((color, index) => (
                    <button
                      key={color.name}
                      onClick={() => setActiveColor(index)}
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
            {product.sizes && product.sizes[0] !== 'Единый' && (
              <div className="mt-6">
                <div className="flex items-center justify-between mb-2">
                  <p className="text-sm text-graphite dark:text-white">
                    Размер: <span className="text-ash">{activeSize || 'Не выбран'}</span>
                  </p>
                  <button
                    onClick={() => setShowSizeChart(true)}
                    className="flex items-center gap-1 text-sm text-primary hover:underline"
                  >
                    <Ruler className="w-4 h-4" />
                    Размерная сетка
                  </button>
                </div>
                <div className="flex flex-wrap gap-2">
                  {product.sizes.map((size) => (
                    <button
                      key={size}
                      onClick={() => setActiveSize(size)}
                      className={cn(
                        "h-10 min-w-[48px] px-4 rounded-lg border text-sm font-medium transition-all",
                        activeSize === size
                            ? "bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white"
                          : "bg-white dark:bg-transparent border-border-lighter dark:border-white/20 text-graphite dark:text-white hover:border-graphite dark:hover:border-white"
                      )}
                    >
                      {size}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Quantity */}
            <div className="mt-6">
              <p className="text-sm text-graphite dark:text-white mb-2">Количество</p>
              <div className="flex items-center gap-3">
                <div className="flex items-center border border-border-lighter dark:border-white/20 rounded-lg">
                  <button
                    onClick={() => setQuantity(Math.max(1, quantity - 1))}
                    className="w-10 h-10 flex items-center justify-center text-graphite dark:text-white hover:bg-ice dark:hover:bg-white/5 transition-colors"
                  >
                    <Minus className="w-4 h-4" />
                  </button>
                  <span className="w-12 text-center text-sm font-medium text-graphite dark:text-white">{quantity}</span>
                  <button
                    onClick={() => setQuantity(quantity + 1)}
                    className="w-10 h-10 flex items-center justify-center text-graphite dark:text-white hover:bg-ice dark:hover:bg-white/5 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>

            {/* Add to cart */}
            <div className="mt-6 flex gap-3">
              <Button variant="primary" className="flex-1 h-12 gap-2" onClick={handleAddToCart}>
                <ShoppingBag className="w-5 h-5" />
                Добавить в корзину
              </Button>
              <Button
                variant="secondary"
                size="icon"
                className="h-12 w-12"
                onClick={() => {
                  toggleFavorite(product.id);
                  showToast(liked ? 'Удалено из избранного' : 'Добавлено в избранное');
                }}
              >
                <Heart className={cn("w-5 h-5", liked && "fill-current text-red-500")} />
              </Button>
            </div>

            
              {/* Seller Block */}
              {seller && (
                <Link to={`/seller/${seller.slug}`} className="block mt-8 p-4 rounded-[14px] bg-white dark:bg-white/[0.02] border border-border-lighter dark:border-white/10 hover:border-graphite/20 dark:hover:border-white/20 transition-all group shadow-sm hover:shadow-md">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-full overflow-hidden shrink-0">
                      <img src={seller.avatar} alt={seller.name} className="w-full h-full object-cover" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="text-sm font-semibold text-graphite dark:text-white truncate pr-4 group-hover:underline decoration-1 underline-offset-4">{seller.name}</h3>
                        <ChevronRight className="w-4 h-4 text-graphite/40 group-hover:text-graphite dark:text-white/40 dark:group-hover:text-white transition-transform group-hover:translate-x-0.5" />
                      </div>
                      <div className="flex items-center gap-2 text-xs">
                        <div className="flex items-center text-graphite dark:text-white font-medium">
                          <Star className="w-3 h-3 fill-amber-400 text-amber-400 mr-1" />
                          {seller.rating.toFixed(1)}
                        </div>
                        <span className="text-graphite/20 dark:text-white/20">•</span>
                        <span className="text-ash truncate">{seller.reviewCount} отзывов</span>
                      </div>
                    </div>
                  </div>
                </Link>
              )}

              {/* Benefits */}
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
                  {product.description || 'Минималистичная вещь для капсульного гардероба в эстетике современного минимализма. Качественные материалы и безупречный крой.'}
                </p>
              </AccordionSection>

              <AccordionSection title="Состав и уход">
                <p className="text-sm text-graphite-light dark:text-white/70 leading-relaxed mb-3">
                  {product.materials || 'Высококачественные материалы с акцентом на долговечность и комфорт.'}
                </p>
                <ul className="text-sm text-graphite-light dark:text-white/70 space-y-1">
                  <li>• Машинная стирка при 30°C</li>
                  <li>• Не отбеливать</li>
                  <li>• Гладить при низкой температуре</li>
                </ul>
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
        {product.reviews && product.reviews.length > 0 && (
          <section className="mt-16">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-2xl font-serif text-graphite dark:text-white">
                Отзывы ({product.reviewsCount})
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

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {product.reviews.map((review) => (
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
          </section>
        )}

        {/* Brand Products */}
        {brandProducts.length > 0 && (
          <motion.section className="mt-16" initial={{ opacity: 0, y: 16 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }}>
            <SectionHeader label={product.brand} title="Ещё от бренда" />
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {brandProducts.map((item) => (
                <ProductCard key={item.id} product={item} />
              ))}
            </div>
          </motion.section>
        )}

        {/* Related Products */}
        {relatedProducts.length > 0 && (
          <motion.section className="mt-16" initial={{ opacity: 0, y: 16 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }}>
            <SectionHeader label="Похожие товары" title="Вам может понравиться" />
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {relatedProducts.map((item) => (
                <ProductCard key={item.id} product={item} />
              ))}
            </div>
          </motion.section>
        )}
      </div>

      {/* Size Chart Modal */}
      <Modal isOpen={showSizeChart} onClose={() => setShowSizeChart(false)} title="Размерная сетка">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border-lighter dark:border-white/10">
                {SIZE_CHART.headers.map((header) => (
                  <th key={header} className="py-3 px-4 text-left font-medium text-graphite dark:text-white">
                    {header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {SIZE_CHART.rows.map((row, i) => (
                <tr key={i} className="border-b border-border-lighter dark:border-white/10">
                  {row.map((cell, j) => (
                    <td key={j} className={cn("py-3 px-4", j === 0 ? "font-medium text-graphite dark:text-white" : "text-ash")}>
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-4 text-xs text-ash">
          * Размеры указаны в сантиметрах. Рекомендуем измерить ваши параметры и сравнить с таблицей.
        </p>
      </Modal>
    </div>
  );
}

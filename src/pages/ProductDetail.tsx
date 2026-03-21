import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Heart, ShoppingBag, Truck, RotateCcw, ChevronDown, ArrowLeft } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { ProductCard } from '../components/product/ProductCard';
import { getProductById, PRODUCTS } from '../lib/mock-data';
import { formatPrice } from '../lib/utils';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useToast } from '../contexts/ToastContext';

export function ProductDetail() {
  const { id } = useParams<{ id: string }>();
  const product = getProductById(id || '');
  const { addItem } = useCart();
  const { isFavorite, toggleFavorite } = useFavorites();
  const { showToast } = useToast();

  const [selectedSize, setSelectedSize] = useState<string | null>(null);
  const [selectedColor, setSelectedColor] = useState(0);
  const [activeImage, setActiveImage] = useState(0);
  const [showDescription, setShowDescription] = useState(true);
  const [showMaterials, setShowMaterials] = useState(false);
  const [showDelivery, setShowDelivery] = useState(false);

  if (!product) {
    return (
      <div className="min-h-screen bg-milk flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-2xl font-serif text-graphite mb-4">Товар не найден</h2>
          <Link to="/catalog"><Button variant="primary">Вернуться в каталог</Button></Link>
        </div>
      </div>
    );
  }

  const images = product.images || [product.image];
  const liked = isFavorite(product.id);
  const relatedProducts = PRODUCTS.filter(p => p.category === product.category && p.id !== product.id).slice(0, 4);

  const handleAddToCart = () => {
    addItem(product, selectedSize || undefined, product.colors?.[selectedColor]?.name);
    showToast('Добавлено в корзину');
  };

  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-6">
        {/* Back button */}
        <Link to="/catalog" className="inline-flex items-center gap-1.5 text-sm text-ash hover:text-graphite transition-colors mb-6">
          <ArrowLeft className="w-4 h-4" /> Назад в каталог
        </Link>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 lg:gap-12">
          {/* Gallery */}
          <div className="space-y-3">
            <motion.div
              className="relative aspect-square bg-surface rounded-3xl overflow-hidden flex items-center justify-center"
              layoutId={`product-image-${product.id}`}
            >
              <img
                src={images[activeImage]}
                alt={product.name}
                className="w-[90%] h-[90%] object-cover rounded-2xl"
              />
              {product.isNew && (
                <div className="absolute top-4 left-4">
                  <Badge variant="new">Новинка</Badge>
                </div>
              )}
              {product.oldPrice && (
                <div className="absolute top-4 left-4">
                  <Badge variant="sale">Скидка</Badge>
                </div>
              )}
            </motion.div>
            {images.length > 1 && (
              <div className="flex gap-2">
                {images.map((img, i) => (
                  <button
                    key={i}
                    onClick={() => setActiveImage(i)}
                    className={`w-20 h-20 rounded-xl overflow-hidden border-2 transition-all ${
                      activeImage === i ? 'border-primary' : 'border-border-lighter hover:border-primary/30'
                    }`}
                  >
                    <img src={img} alt="" className="w-full h-full object-cover" />
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Product Info */}
          <div className="space-y-6">
            <div>
              <Link to={`/brand/${product.brandId}`} className="text-xs font-semibold tracking-wider text-ash uppercase hover:text-primary transition-colors">
                {product.brand}
              </Link>
              <h1 className="text-2xl sm:text-3xl font-serif text-graphite mt-1 mb-3">{product.name}</h1>
              <div className="flex items-center gap-3">
                <span className="text-2xl font-semibold text-primary">{formatPrice(product.price)}</span>
                {product.oldPrice && (
                  <span className="text-lg text-ash line-through">{formatPrice(product.oldPrice)}</span>
                )}
              </div>
            </div>

            {/* Colors */}
            {product.colors && product.colors.length > 0 && (
              <div>
                <p className="text-sm font-medium text-graphite mb-2.5">
                  Цвет: <span className="text-ash font-normal">{product.colors[selectedColor].name}</span>
                </p>
                <div className="flex gap-2">
                  {product.colors.map((color, i) => (
                    <button
                      key={i}
                      onClick={() => setSelectedColor(i)}
                      className={`w-10 h-10 rounded-xl border-2 transition-all ${
                        selectedColor === i ? 'border-primary scale-110' : 'border-border-lighter hover:border-primary/30'
                      }`}
                      style={{ backgroundColor: color.hex }}
                      title={color.name}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Sizes */}
            {product.sizes && product.sizes.length > 0 && product.sizes[0] !== 'One Size' && (
              <div>
                <p className="text-sm font-medium text-graphite mb-2.5">Размер</p>
                <div className="flex flex-wrap gap-2">
                  {product.sizes.map(size => (
                    <button
                      key={size}
                      onClick={() => setSelectedSize(size)}
                      className={`min-w-[44px] h-11 px-3 rounded-xl border text-sm font-medium transition-all ${
                        selectedSize === size
                          ? 'bg-primary text-white border-primary'
                          : 'bg-white text-graphite border-border-soft hover:border-primary/40'
                      }`}
                    >
                      {size}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Action buttons */}
            <div className="flex gap-3 pt-2">
              <Button variant="primary" size="lg" className="flex-1 gap-2" onClick={handleAddToCart}>
                <ShoppingBag className="w-4 h-4" />
                Добавить в корзину
              </Button>
              <Button
                variant="secondary"
                size="icon"
                className={`w-12 h-12 shrink-0 ${liked ? 'text-primary border-primary/30 bg-primary/5' : ''}`}
                onClick={() => {
                  toggleFavorite(product.id);
                  showToast(liked ? 'Удалено из избранного' : 'Добавлено в избранное');
                }}
              >
                <Heart className={`w-5 h-5 ${liked ? 'fill-primary' : ''}`} />
              </Button>
            </div>

            {/* Info Delivery */}
            <div className="flex gap-4 py-4 border-t border-b border-border-lighter">
              <div className="flex items-center gap-2 text-sm text-ash">
                <Truck className="w-4 h-4 text-primary" />
                <span>Бесплатная доставка от 10 000 ₽</span>
              </div>
              <div className="flex items-center gap-2 text-sm text-ash">
                <RotateCcw className="w-4 h-4 text-primary" />
                <span>Возврат 14 дней</span>
              </div>
            </div>

            {/* Accordion sections */}
            <div className="space-y-0">
              {/* Description */}
              <button
                onClick={() => setShowDescription(!showDescription)}
                className="flex items-center justify-between w-full py-3.5 border-b border-border-lighter text-sm font-medium text-graphite"
              >
                Описание
                <ChevronDown className={`w-4 h-4 text-ash transition-transform ${showDescription ? 'rotate-180' : ''}`} />
              </button>
              {showDescription && product.description && (
                <motion.div initial={{ height: 0 }} animate={{ height: 'auto' }} className="overflow-hidden">
                  <p className="py-3 text-sm text-ash leading-relaxed">{product.description}</p>
                </motion.div>
              )}

              {/* Materials */}
              <button
                onClick={() => setShowMaterials(!showMaterials)}
                className="flex items-center justify-between w-full py-3.5 border-b border-border-lighter text-sm font-medium text-graphite"
              >
                Состав и материалы
                <ChevronDown className={`w-4 h-4 text-ash transition-transform ${showMaterials ? 'rotate-180' : ''}`} />
              </button>
              {showMaterials && product.materials && (
                <motion.div initial={{ height: 0 }} animate={{ height: 'auto' }} className="overflow-hidden">
                  <p className="py-3 text-sm text-ash leading-relaxed">{product.materials}</p>
                </motion.div>
              )}

              {/* Delivery */}
              <button
                onClick={() => setShowDelivery(!showDelivery)}
                className="flex items-center justify-between w-full py-3.5 border-b border-border-lighter text-sm font-medium text-graphite"
              >
                Доставка и возврат
                <ChevronDown className={`w-4 h-4 text-ash transition-transform ${showDelivery ? 'rotate-180' : ''}`} />
              </button>
              {showDelivery && (
                <motion.div initial={{ height: 0 }} animate={{ height: 'auto' }} className="overflow-hidden">
                  <div className="py-3 text-sm text-ash leading-relaxed space-y-2">
                    <p>• Доставка по Москве: 1-2 рабочих дня</p>
                    <p>• Доставка по России: 3-7 рабочих дней</p>
                    <p>• Бесплатная доставка от 10 000 ₽</p>
                    <p>• Возврат в течение 14 дней с момента получения</p>
                  </div>
                </motion.div>
              )}

              {/* Brand link */}
              <Link
                to={`/brand/${product.brandId}`}
                className="flex items-center justify-between w-full py-3.5 border-b border-border-lighter text-sm font-medium text-graphite hover:text-primary transition-colors"
              >
                О бренде {product.brand}
                <ChevronDown className="w-4 h-4 text-ash -rotate-90" />
              </Link>
            </div>
          </div>
        </div>

        {/* Related Products */}
        {relatedProducts.length > 0 && (
          <section className="mt-16">
            <h2 className="text-2xl font-serif text-graphite mb-8">Похожие товары</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
              {relatedProducts.map(p => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          </section>
        )}
      </div>

      {/* Sticky Purchase Panel (mobile) */}
      <div className="fixed bottom-16 left-0 right-0 md:hidden glass-strong border-t border-border-lighter px-4 py-3 z-40">
        <div className="flex items-center gap-3">
          <div className="flex-1">
            <p className="text-xs text-ash">{product.brand}</p>
            <p className="text-base font-semibold text-graphite">{formatPrice(product.price)}</p>
          </div>
          <Button variant="primary" className="gap-2" onClick={handleAddToCart}>
            <ShoppingBag className="w-4 h-4" />
            В корзину
          </Button>
        </div>
      </div>
    </div>
  );
}

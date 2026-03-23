import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, Heart, ShoppingBag } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { ProductCard } from '../components/product/ProductCard';
import { useCart } from '../contexts/CartContext';
import { useFavorites } from '../contexts/FavoritesContext';
import { useToast } from '../contexts/ToastContext';
import { formatPrice } from '../lib/utils';
import { PRODUCTS, getProductById, getBrandById, getProductsByBrand } from '../lib/mock-data';
import { InfoPanel, PillFilter, SectionHeader } from '../components/editorial/StudioKit';

export function ProductDetail() {
  const { id } = useParams<{ id: string }>();
  const product = getProductById(id || '');
  const [activeImage, setActiveImage] = useState(0);
  const [activeSize, setActiveSize] = useState<string | null>(null);
  const [activeColor, setActiveColor] = useState(0);

  const { addItem } = useCart();
  const { toggleFavorite, isFavorite } = useFavorites();
  const { showToast } = useToast();

  if (!product) {
    return (
      <div className='relative z-10 min-h-screen pt-36 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite'>Товар не найден</h1>
          <Link to='/catalog' className='inline-block mt-6'>
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

  const handleAddToCart = () => {
    addItem(product, activeSize || undefined, product.colors?.[activeColor]?.name);
    showToast('Товар добавлен в корзину');
  };

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <Link to='/catalog' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite'>
          <ArrowLeft className='w-4 h-4' /> Назад в каталог
        </Link>

        <section className='mt-6 overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm'>
          <div className='relative h-[200px] md:h-[250px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.3rem,6.6vw,7rem)] text-white/43 leading-[0.8] tracking-[-0.03em]'>АРХИВ</h2>
              <h3 className='font-serif text-[clamp(2.1rem,5.8vw,6.1rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center'>{product.brand}</h3>
              <h4 className='font-serif text-[clamp(2rem,5.6vw,5.6rem)] text-white/42 leading-[0.8] tracking-[-0.03em] text-right'>ТОВАР</h4>
            </div>
          </div>
        </section>

        <div className='mt-8 grid grid-cols-1 xl:grid-cols-12 gap-8'>
          <div className='xl:col-span-7'>
            <div className='bg-white border border-[#e6edf6] p-3 shadow-[0_10px_24px_rgba(142,165,191,0.11)]'>
              <div className='overflow-hidden rounded-[0.75rem] aspect-[4/5]'>
                <img src={images[activeImage]} alt={product.name} className='w-full h-full object-cover' />
              </div>
            </div>
            {images.length > 1 && (
              <div className='mt-3 flex gap-2'>
                {images.map((image, index) => (
                  <button
                    key={image}
                    onClick={() => setActiveImage(index)}
                    className={`h-20 w-20 rounded-[0.5rem] overflow-hidden border ${
                      activeImage === index ? 'border-graphite' : 'border-border-lighter'
                    }`}
                  >
                    <img src={image} alt='' className='w-full h-full object-cover' />
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className='xl:col-span-5'>
            <section className='bg-white/72 border border-white/60 p-6 md:p-8 relative'>
              <Link to={`/brands`} className='text-xs uppercase tracking-[0.14em] text-ash hover:text-graphite transition-colors'>{product.brand}</Link>
              <h1 className='mt-2 text-[40px] leading-[0.95] font-serif text-graphite mb-1'>{product.name}</h1>
              
              {(product.rating || product.reviewsCount) && (
                <div className="flex items-center gap-2 text-[13px] text-graphite-light mb-4 mt-2">
                  {product.rating && <span className="flex items-center font-medium"><span className="text-yellow-500 mr-1 text-base leading-none">★</span> {product.rating.toFixed(1)}</span>}
                  {product.rating && product.reviewsCount && <span className="opacity-50">·</span>}
                  {product.reviewsCount && <span>{product.reviewsCount} отзыв{product.reviewsCount % 10 === 1 && product.reviewsCount % 100 !== 11 ? '' : (product.reviewsCount % 10 >= 2 && product.reviewsCount % 10 <= 4 && (product.reviewsCount % 100 < 10 || product.reviewsCount % 100 >= 20) ? 'а' : 'ов')}</span>}
                </div>
              )}

              <p className='mt-2 text-[34px] font-medium text-graphite'>{formatPrice(product.price)}</p>

              {product.colors && (
                <div className='mt-6'>
                  <p className='text-sm text-graphite-light mb-2'>Цвет: {product.colors[activeColor].name}</p>
                  <div className='flex gap-2'>
                    {product.colors.map((color, index) => (
                      <button
                        key={color.name}
                        onClick={() => setActiveColor(index)}
                        style={{ backgroundColor: color.hex }}
                        className={`h-9 w-9 rounded-full border-2 ${activeColor === index ? 'border-graphite' : 'border-white'}`}
                      />
                    ))}
                  </div>
                </div>
              )}

              {product.sizes && product.sizes[0] !== 'Единый' && (
                <div className='mt-5'>
                  <p className='text-sm text-graphite-light mb-2'>Размер</p>
                  <div className='flex flex-wrap gap-2'>
                    {product.sizes.map((size) => (
                      <PillFilter key={size} label={size} active={activeSize === size} onClick={() => setActiveSize(size)} />
                    ))}
                  </div>
                </div>
              )}

              <div className='mt-7 flex gap-3'>
                <Button variant='primary' className='flex-1 gap-2' onClick={handleAddToCart}>
                  <ShoppingBag className='w-4 h-4' /> Добавить в корзину
                </Button>
                <Button
                  variant='secondary'
                  size='icon'
                  onClick={() => {
                    toggleFavorite(product.id);
                    showToast(liked ? 'Удалено из избранного' : 'Добавлено в избранное');
                  }}
                >
                  <Heart className={`w-5 h-5 ${liked ? 'fill-current text-error' : ''}`} />
                </Button>
              </div>
            </section>
          </div>
        </div>

        <div className='mt-10 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5'>
          <InfoPanel title='Описание'>{product.description || 'Минималистичная вещь для капсульного гардероба в эстетике холодного premium.'}</InfoPanel>
          <InfoPanel title='Состав'>{product.materials || 'Высокотехнологичные смесовые ткани с акцентом на долговечность и тактильность.'}</InfoPanel>
          <div className='glass-panel p-6 flex flex-col'>
            <h4 className='font-serif text-[22px] text-graphite mb-3'>Возврат и обмен</h4>
            <p className='text-[13px] text-graphite-light leading-relaxed mb-4'>
              Бесплатная доставка от 10 000 ₽. Возврат в течение 14 дней при сохранении товарного вида и бирок.
            </p>
            <Link to="/returns" className="mt-auto text-sm text-primary hover:underline font-medium">Подробнее об условиях →</Link>
          </div>
        </div>

        {/* Brand Info */}
        {brand && (
          <section className='mt-14 glass-panel p-8 md:p-12'>
            <div className="grid grid-cols-1 md:grid-cols-12 gap-8 items-center">
              <div className="md:col-span-4 lg:col-span-3">
                <div className="aspect-square rounded-full overflow-hidden border border-border-lighter opacity-90 shadow-sm max-w-[200px] mx-auto md:max-w-none">
                  <img src={brand.image} alt={brand.name} className="w-full h-full object-cover grayscale opacity-90" />
                </div>
              </div>
              <div className="md:col-span-8 lg:col-span-9">
                <p className="text-xs uppercase tracking-[0.14em] text-ash mb-2 font-semibold">О бренде</p>
                <h3 className="font-serif text-[32px] md:text-[40px] text-graphite leading-tight mb-4">{brand.name}</h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-6 text-[14px] text-graphite-light leading-relaxed">
                  <div>
                    <strong className="block text-graphite font-medium mb-1 text-xs uppercase tracking-wider">История</strong>
                    {brand.history || brand.description}
                  </div>
                  <div>
                    <strong className="block text-graphite font-medium mb-1 text-xs uppercase tracking-wider">Философия</strong>
                    {brand.philosophy || `Проект из ${brand.origin || brand.country}, исследующий формы и материалы.`}
                  </div>
                </div>
                <Link to={`/brand/${brand.id}`}>
                  <Button variant="secondary" className="mt-6">Страница бренда</Button>
                </Link>
              </div>
            </div>
          </section>
        )}

        {/* Reviews */}
        {product.reviews && product.reviews.length > 0 && (
          <section className='mt-14'>
            <SectionHeader
              label='Мнения'
              title='Отзывы об архиве'
              description={`${product.reviewsCount} отзыв${product.reviewsCount && product.reviewsCount % 10 === 1 && product.reviewsCount % 100 !== 11 ? '' : (product.reviewsCount && product.reviewsCount % 10 >= 2 && product.reviewsCount % 10 <= 4 && (product.reviewsCount % 100 < 10 || product.reviewsCount % 100 >= 20) ? 'а' : 'ов')}. Рейтинг ${product.rating?.toFixed(1)}`}
            />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5 mt-8">
              {product.reviews.map(review => (
                <div key={review.id} className="glass-panel p-6">
                  <div className="flex justify-between items-start mb-4">
                    <div>
                      <h4 className="font-medium text-graphite text-sm">{review.author}</h4>
                      <p className="text-[11px] text-ash tracking-wide uppercase mt-0.5">{review.date}</p>
                    </div>
                    <div className="flex text-yellow-500 text-sm">
                      {Array.from({ length: 5 }).map((_, i) => (
                        <span key={i} className={i < review.rating ? 'opacity-100' : 'text-border-soft'}>★</span>
                      ))}
                    </div>
                  </div>
                  <p className="text-[14px] text-graphite-light leading-relaxed mb-4">"{review.text}"</p>
                  <div className="flex flex-wrap gap-2">
                    {review.fit && <span className="text-[11px] px-2 py-1 bg-ice/50 border border-border-lighter rounded-md text-ash font-medium tracking-wide">Крой: {review.fit}</span>}
                    {review.quality && <span className="text-[11px] px-2 py-1 bg-ice/50 border border-border-lighter rounded-md text-ash font-medium tracking-wide">Качество: {review.quality}</span>}
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        {brandProducts.length > 0 && (
          <motion.section className='mt-14' initial={{ opacity: 0, y: 16 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }}>
            <SectionHeader
              label='Ассортимент марки'
              title={`Ещё от ${product.brand}`}
              description='Вещи, формирующие общую капсулу.'
            />
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
              {brandProducts.map((item) => (
                <ProductCard key={item.id} product={item} />
              ))}
            </div>
          </motion.section>
        )}

        {relatedProducts.length > 0 && (
          <motion.section className='mt-14' initial={{ opacity: 0, y: 16 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }}>
            <SectionHeader
              label='Похожие товары'
              title='Продолжить подборку'
              description='Товары той же категории в едином карточном языке.'
            />
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
              {relatedProducts.map((item) => (
                <ProductCard key={item.id} product={item} />
              ))}
            </div>
          </motion.section>
        )}
      </div>
    </div>
  );
}

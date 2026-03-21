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
import { PRODUCTS, getProductById } from '../lib/mock-data';
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
            <section className='bg-white/72 border border-white/60 p-6 md:p-8'>
              <p className='text-xs uppercase tracking-[0.14em] text-ash'>{product.brand}</p>
              <h1 className='mt-2 text-[40px] leading-[0.95] font-serif text-graphite'>{product.name}</h1>
              <p className='mt-4 text-[34px] font-medium text-graphite'>{formatPrice(product.price)}</p>

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

        <div className='mt-10 grid grid-cols-1 lg:grid-cols-3 gap-5'>
          <InfoPanel title='Описание'>{product.description || 'Минималистичная вещь для капсульного гардероба в эстетике холодного premium.'}</InfoPanel>
          <InfoPanel title='Состав'>{product.materials || 'Высокотехнологичные смесовые ткани с акцентом на долговечность и тактильность.'}</InfoPanel>
          <InfoPanel title='Доставка и возврат'>
            Бесплатная доставка от 10 000 ₽. Возврат в течение 14 дней при сохранении товарного вида.
          </InfoPanel>
        </div>

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

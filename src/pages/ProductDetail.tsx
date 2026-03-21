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
import { HeroBlock, InfoPanel, PillFilter, SectionHeader } from '../components/editorial/StudioKit';

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
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px]'>
        <Link to='/catalog' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite'>
          <ArrowLeft className='w-4 h-4' /> Назад в каталог
        </Link>

        <div className='mt-6 grid grid-cols-1 xl:grid-cols-12 gap-8'>
          <div className='xl:col-span-7'>
            <div className='glass-panel-strong p-3'>
              <div className='overflow-hidden rounded-[2rem] aspect-[4/5]'>
                <img src={images[activeImage]} alt={product.name} className='w-full h-full object-cover' />
              </div>
            </div>
            {images.length > 1 && (
              <div className='mt-3 flex gap-2'>
                {images.map((image, index) => (
                  <button
                    key={image}
                    onClick={() => setActiveImage(index)}
                    className={`h-20 w-20 rounded-2xl overflow-hidden border ${
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
            <HeroBlock
              className='h-full'
              label={product.brand}
              title={<>{product.name}</>}
              description='Премиальная студийная подача: мягкие панели, спокойная типографика и деликатный контроль параметров.'
              right={
                <div className='space-y-5'>
                  <p className='text-3xl font-semibold text-graphite'>{formatPrice(product.price)}</p>

                  {product.colors && (
                    <div>
                      <p className='text-sm text-graphite-light mb-2'>Цвет: {product.colors[activeColor].name}</p>
                      <div className='flex gap-2'>
                        {product.colors.map((color, index) => (
                          <button
                            key={color.name}
                            onClick={() => setActiveColor(index)}
                            style={{ backgroundColor: color.hex }}
                            className={`h-9 w-9 rounded-full border-2 ${
                              activeColor === index ? 'border-graphite' : 'border-white'
                            }`}
                          />
                        ))}
                      </div>
                    </div>
                  )}

                  {product.sizes && product.sizes[0] !== 'Единый' && (
                    <div>
                      <p className='text-sm text-graphite-light mb-2'>Размер</p>
                      <div className='flex flex-wrap gap-2'>
                        {product.sizes.map((size) => (
                          <PillFilter key={size} label={size} active={activeSize === size} onClick={() => setActiveSize(size)} />
                        ))}
                      </div>
                    </div>
                  )}

                  <div className='flex gap-3'>
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
                </div>
              }
            />
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

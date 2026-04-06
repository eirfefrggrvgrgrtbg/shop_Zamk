import { Link } from 'react-router-dom';
import { Minus, Plus, Trash2, ArrowRight } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { ProductCard } from '../components/product/ProductCard';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';
import { PRODUCTS } from '../lib/mock-data';
import { InfoPanel, SectionHeader } from '../components/editorial/StudioKit';

export function Cart() {
  const { items, updateQuantity, removeItem, totalPrice } = useCart();
  const delivery = totalPrice >= 10000 ? 0 : 590;
  const finalTotal = totalPrice + delivery;
  const recommendations = PRODUCTS.filter((product) => !items.find((item) => item.product.id === product.id)).slice(0, 4);

  if (items.length === 0) {
    return (
      <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-5xl'>
          <section className="mb-12 border-b border-border-lighter pb-8">
            <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
              Новая поставка
            </p>
            <div className="flex items-end justify-between">
              <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
                Корзина пуста
              </h1>
            </div>
          </section>

          <div className='mt-8 bg-white/58 border border-white/50 p-8'>
            <EmptyState
              icon='cart'
              title='Нет товаров'
              description='Собери свою капсулу'
              action={
                <Link to='/catalog'>
                  <Button className='gap-2'>
                    Перейти в каталог <ArrowRight className='w-4 h-4' />
                  </Button>
                </Link>
              }
            />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className="mb-12 border-b border-border-lighter pb-8 flex items-end justify-between">
          <div>
            <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
              Подготовка заказа
            </p>
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Корзина
            </h1>
          </div>
          <span className="text-sm text-ash mb-1 hidden sm:block">{items.length} товаров</span>
        </section>

        <div className='mt-10 grid grid-cols-1 lg:grid-cols-12 gap-6'>
          <div className='lg:col-span-8 space-y-4'>
            {items.map((item) => (
              <article key={item.product.id} className='bg-white/62 border border-white/50 p-4 md:p-5 flex gap-4'>
                <Link to={`/product/${item.product.id}`} className='h-28 w-24 overflow-hidden rounded-[0.45rem] border border-border-lighter'>
                  <img src={item.product.image} alt={item.product.name} className='h-full w-full object-cover' />
                </Link>
                <div className='flex-1'>
                  <p className='text-xs uppercase tracking-[0.14em] text-ash'>{item.product.brand}</p>
                  <h3 className='text-lg font-medium text-graphite'>{item.product.name}</h3>
                  <p className='mt-1 text-sm text-graphite-light'>{formatPrice(item.product.price)}</p>

                  <div className='mt-4 flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <button className='h-8 w-8 rounded-full border border-border-soft flex items-center justify-center' onClick={() => updateQuantity(item.product.id, item.quantity - 1)}>
                        <Minus className='w-3.5 h-3.5' />
                      </button>
                      <span className='w-7 text-center text-sm'>{item.quantity}</span>
                      <button className='h-8 w-8 rounded-full border border-border-soft flex items-center justify-center' onClick={() => updateQuantity(item.product.id, item.quantity + 1)}>
                        <Plus className='w-3.5 h-3.5' />
                      </button>
                    </div>
                    <button className='text-ash hover:text-error' onClick={() => removeItem(item.product.id)}>
                      <Trash2 className='w-4 h-4' />
                    </button>
                  </div>
                </div>
              </article>
            ))}
          </div>

          <aside className='lg:col-span-4'>
            <InfoPanel title='Сводка заказа' className='sticky top-32'>
              <div className='space-y-3 text-sm'>
                <div className='flex justify-between'>
                  <span>Сумма товаров</span>
                  <span className='text-graphite'>{formatPrice(totalPrice)}</span>
                </div>
                <div className='flex justify-between'>
                  <span>Доставка</span>
                  <span className='text-graphite'>{delivery ? formatPrice(delivery) : 'Бесплатно'}</span>
                </div>
                <div className='border-t border-border-lighter pt-3 mt-2 flex justify-between text-base font-semibold text-graphite'>
                  <span>Итого</span>
                  <span>{formatPrice(finalTotal)}</span>
                </div>
                <Link to='/checkout' className='block mt-4'>
                  <Button className='w-full gap-2'>
                    Оформить заказ <ArrowRight className='w-4 h-4' />
                  </Button>
                </Link>
              </div>
            </InfoPanel>
          </aside>
        </div>

        <section className='mt-14'>
          <SectionHeader label='Рекомендации' title='Может дополнить корзину' />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {recommendations.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

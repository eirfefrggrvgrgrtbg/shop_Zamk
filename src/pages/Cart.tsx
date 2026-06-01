import { Link } from 'react-router-dom';
import { Minus, Plus, Trash2, ArrowRight } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { ProductCard } from '../components/product/ProductCard';
import { getCartItemKey, useCart } from '../contexts/CartContext';
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

          <div className='mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm'>
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
              <article key={getCartItemKey(item.product.id, item.selectedSize, item.selectedColor)} className='bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-2xl p-4 md:p-5 flex gap-4 backdrop-blur-md shadow-sm'>
                <Link to={`/product/${item.product.id}`} className='h-full w-28 md:w-32 overflow-hidden rounded-[0.45rem] border border-border-lighter dark:border-white/10 shrink-0'>
                  <img src={item.product.image} alt={item.product.name} className='h-full w-full object-cover' />
                </Link>
                <div className='flex-1 flex flex-col'>
                  <p className='text-xs uppercase tracking-[0.14em] text-ash'>{item.product.brand}</p>
                  <h3 className='text-lg font-medium text-graphite dark:text-gray-200 leading-tight mt-1'>{item.product.name}</h3>
                  <p className='mt-2 text-sm text-graphite-light dark:text-gray-400'>{formatPrice(item.product.price)}</p>
                  {(item.selectedSize || item.selectedColor) && (
                    <div className='mt-2 flex flex-wrap gap-2 text-xs text-ash'>
                      {item.selectedSize && <span>Размер: {item.selectedSize}</span>}
                      {item.selectedColor && <span>Цвет: {item.selectedColor}</span>}
                    </div>
                  )}

                  <div className='mt-auto pt-4 flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <button type='button' aria-label={`Уменьшить количество ${item.product.name}`} className='h-8 w-8 rounded-full border border-border-soft dark:border-white/20 flex items-center justify-center text-graphite dark:text-gray-300 hover:bg-graphite/5 dark:hover:bg-white/10' onClick={() => updateQuantity(item.product.id, item.quantity - 1, item.selectedSize, item.selectedColor)}>
                        <Minus className='w-3.5 h-3.5' />
                      </button>
                      <span className='w-7 text-center text-sm font-medium text-graphite dark:text-gray-200'>{item.quantity}</span>
                      <button type='button' aria-label={`Увеличить количество ${item.product.name}`} className='h-8 w-8 rounded-full border border-border-soft dark:border-white/20 flex items-center justify-center text-graphite dark:text-gray-300 hover:bg-graphite/5 dark:hover:bg-white/10' onClick={() => updateQuantity(item.product.id, item.quantity + 1, item.selectedSize, item.selectedColor)}>
                        <Plus className='w-3.5 h-3.5' />
                      </button>
                    </div>
                    <button type='button' aria-label={`Удалить ${item.product.name} из корзины`} className='text-ash hover:text-error transition-colors p-2' onClick={() => removeItem(item.product.id, item.selectedSize, item.selectedColor)}>
                      <Trash2 className='w-5 h-5' />
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

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Check } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';
import { CheckoutPanel, PillFilter, SectionHeader } from '../components/editorial/StudioKit';

export function Checkout() {
  const { items, totalPrice, clearCart } = useCart();
  const [step, setStep] = useState(1);
  const [done, setDone] = useState(false);

  if (done) {
    return (
      <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl'>
          <CheckoutPanel>
            <div className='text-center'>
              <div className='mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-white dark:bg-white/10 border border-border-lighter dark:border-white/15'>
                <Check className='w-10 h-10 text-success' />
              </div>
              <h1 className='mt-6 text-4xl font-serif text-graphite dark:text-white'>Заказ оформлен</h1>
              <p className='mt-3 text-graphite-light dark:text-white/70'>Спасибо за покупку. Подтверждение уже отправлено на вашу почту.</p>
              <div className='mt-7 flex justify-center gap-3'>
                <Link to='/'><Button>На главную</Button></Link>
                <Link to='/catalog'><Button variant='secondary'>В каталог</Button></Link>
              </div>
            </div>
          </CheckoutPanel>
        </div>
      </div>
    );
  }

  if (!items.length) {
    return (
      <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite dark:text-white'>Корзина пуста</h1>
          <Link to='/catalog' className='inline-block mt-6'><Button>Перейти в каталог</Button></Link>
        </div>
      </div>
    );
  }

  const delivery = totalPrice >= 10000 ? 0 : 590;
  const total = totalPrice + delivery;

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <Link to='/cart' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/55 dark:hover:text-white'>
          <ArrowLeft className='w-4 h-4' /> Вернуться в корзину
        </Link>

        <section className="mt-6 mb-8">
          <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">
            Безопасная оплата
          </p>
          <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">
            Оформление заказа
          </h1>
        </section>

        <div className='mt-8 grid grid-cols-1 lg:grid-cols-12 gap-6'>
          <div className='lg:col-span-8 space-y-5'>
            <CheckoutPanel>
              <SectionHeader label='Шаг 1' title='Контактные данные' />
              <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                <Input placeholder='Имя' />
                <Input placeholder='Фамилия' />
                <Input placeholder='Эл. почта' type='email' />
                <Input placeholder='Телефон' type='tel' />
              </div>
            </CheckoutPanel>

            <CheckoutPanel>
              <SectionHeader label='Шаг 2' title='Доставка' />
              <div className='flex flex-wrap gap-2'>
                <PillFilter label='Курьер 1-2 дня' active={step === 1} onClick={() => setStep(1)} />
                <PillFilter label='Самовывоз' active={step === 2} onClick={() => setStep(2)} />
                <PillFilter label='Экспресс 3 часа' active={step === 3} onClick={() => setStep(3)} />
              </div>
            </CheckoutPanel>

            <CheckoutPanel>
              <SectionHeader label='Шаг 3' title='Оплата' />
              <div className='flex flex-wrap gap-2'>
                <PillFilter label='Карта онлайн' active={true} />
                <PillFilter label='СБП' active={false} />
                <PillFilter label='При получении' active={false} />
              </div>
            </CheckoutPanel>
          </div>

          <div className='lg:col-span-4'>
            <CheckoutPanel>
              <SectionHeader label='Итог' title='Сводка заказа' />
              <div className='space-y-3 text-sm'>
                {items.map((item) => (
                  <div key={item.product.id} className='flex justify-between text-graphite-light dark:text-white/68'>
                    <span>{item.product.name} × {item.quantity}</span>
                    <span>{formatPrice(item.product.price * item.quantity)}</span>
                  </div>
                ))}
                <div className='border-t border-border-lighter dark:border-white/10 pt-3 mt-3 space-y-2'>
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Товары</span><span>{formatPrice(totalPrice)}</span></div>
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Доставка</span><span>{delivery ? formatPrice(delivery) : 'Бесплатно'}</span></div>
                  <div className='flex justify-between text-base font-semibold text-graphite dark:text-white'><span>Итого</span><span>{formatPrice(total)}</span></div>
                </div>
              </div>

              <Button className='w-full mt-5' onClick={() => { setDone(true); clearCart(); }}>
                Оформить заказ
              </Button>
            </CheckoutPanel>
          </div>
        </div>
      </div>
    </div>
  );
}

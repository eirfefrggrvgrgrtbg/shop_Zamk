import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Check } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';
import { CheckoutPanel, PillFilter, SectionHeader } from '../components/editorial/StudioKit';
import { createOrder, createPayment } from '@zamk/api-client/src/customer';
import { useAuth } from '../contexts/AuthContext';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const DELIVERY_OPTIONS = {
  1: 'Курьер 1-2 дня',
  2: 'Самовывоз',
  3: 'Экспресс 3 часа',
} as const;

export function Checkout() {
  const { items, totalPrice, clearCart } = useCart();
  const [step, setStep] = useState(1);
  const [done, setDone] = useState(false);
  const { isAuthenticated, user, openAuthModal } = useAuth();
  const [firstName, setFirstName] = useState(user?.name?.split(' ')[0] || '');
  const [lastName, setLastName] = useState(user?.name?.split(' ')[1] || '');
  const [email, setEmail] = useState(user?.email || '');
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [validationError, setValidationError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleCheckout = async () => {
    if (!isAuthenticated) {
      openAuthModal('login');
      return;
    }
    
    if (isSubmitting || done) {
      return;
    }

    setIsSubmitting(true);

    const trimmedFirstName = firstName.trim();
    const trimmedEmail = email.trim();
    const trimmedPhone = phone.trim();
    const phoneDigits = trimmedPhone.replace(/\D/g, '');
    const trimmedAddress = address.trim();

    if (!trimmedFirstName || !trimmedEmail || !trimmedPhone || !trimmedAddress) {
      setValidationError('Укажите имя, email, телефон и адрес доставки.');
      setIsSubmitting(false);
      return;
    }

    if (!EMAIL_PATTERN.test(trimmedEmail)) {
      setValidationError('Укажите корректный email.');
      setIsSubmitting(false);
      return;
    }

    if (phoneDigits.length < 7) {
      setValidationError('Укажите корректный телефон, минимум 7 цифр.');
      setIsSubmitting(false);
      return;
    }

    try {
      const order = await createOrder({
        customerName: `${trimmedFirstName} ${lastName}`.trim(),
        customerEmail: trimmedEmail,
        customerPhone: trimmedPhone,
        deliveryAddress: `${DELIVERY_OPTIONS[step as keyof typeof DELIVERY_OPTIONS]}: ${trimmedAddress}`
      });

      const payment = await createPayment(order.id);
      
      setValidationError('');
      setDone(true);
      await clearCart();
      
      if (payment.paymentUrl) {
        window.location.href = payment.paymentUrl;
      }
    } catch (e: any) {
      setValidationError(e.message || 'Не удалось сохранить заказ. Попробуйте ещё раз.');
      setIsSubmitting(false);
    }
  };

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

  const delivery = 0; // Доставка рассчитывается позже
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
                <Input placeholder='Имя' value={firstName} onChange={(event) => { setFirstName(event.target.value); if (validationError) setValidationError(''); }} autoComplete='given-name' required />
                <Input placeholder='Фамилия' value={lastName} onChange={(event) => setLastName(event.target.value)} autoComplete='family-name' />
                <Input placeholder='Эл. почта' type='email' value={email} onChange={(event) => { setEmail(event.target.value); if (validationError) setValidationError(''); }} autoComplete='email' required />
                <Input placeholder='Телефон' type='tel' value={phone} onChange={(event) => { setPhone(event.target.value); if (validationError) setValidationError(''); }} autoComplete='tel' required />
                <div className='md:col-span-2'>
                  <Input placeholder='Полный адрес доставки' value={address} onChange={(event) => { setAddress(event.target.value); if (validationError) setValidationError(''); }} required />
                </div>
              </div>
              {validationError && (
                <p className='mt-3 text-sm text-error' role='alert'>
                  {validationError}
                </p>
              )}
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
              <p className='mt-4 text-[13px] text-graphite-light dark:text-white/60 bg-ice dark:bg-white/5 p-3 rounded-xl border border-border-lighter dark:border-white/10'>
                Внимание: Выполняется тестовая оплата для разработки. Реальное списание средств не производится.
              </p>
            </CheckoutPanel>
          </div>

          <div className='lg:col-span-4'>
            <CheckoutPanel>
              <SectionHeader label='Итог' title='Сводка заказа' />
              <div className='space-y-3 text-sm'>
                {items.map((item) => {
                  const productName = item.title || item.product?.name || 'Неизвестный товар';
                  const itemPrice = item.price ? item.price * item.quantity : 0;
                  return (
                  <div key={item.id} className='flex justify-between text-graphite-light dark:text-white/68'>
                    <span>
                      {productName} × {item.quantity}
                    </span>
                    <span>{formatPrice(itemPrice)}</span>
                  </div>
                )})}
                <div className='border-t border-border-lighter dark:border-white/10 pt-3 mt-3 space-y-2'>
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Товары</span><span>{formatPrice(totalPrice)}</span></div>
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Доставка</span><span className='text-xs'>Рассчитывается при оформлении</span></div>
                  <div className='flex justify-between text-base font-semibold text-graphite dark:text-white'><span>Итого</span><span>{formatPrice(total)}</span></div>
                </div>
              </div>

              <Button type='button' className='w-full mt-5' onClick={handleCheckout} disabled={isSubmitting}>
                Оформить заказ
              </Button>
            </CheckoutPanel>
          </div>
        </div>
      </div>
    </div>
  );
}

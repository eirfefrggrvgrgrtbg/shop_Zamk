import { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Check } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';
import { CheckoutPanel, PillFilter, SectionHeader } from '../components/editorial/StudioKit';
import { createOrder, createPayment, getPaymentMethods } from '@zamk/api-client/src/customer';
import { getDeliveryMethods } from '@zamk/api-client/src/public';
import type { AvailablePaymentMethods } from '@zamk/api-client/src/customer';
import type { PublicDeliveryMethod } from '@zamk/api-client/src/types';
import { useAuth } from '../contexts/AuthContext';
import { formatDeliveryAddress } from '../lib/addressValidation';
import { PaymentMethodPicker } from '../components/PaymentMethodPicker';
import { formatVariantDetails } from '../lib/variantSelection';
import { PromoCodeBox } from '../components/PromoCodeBox';
import { usePromo } from '../contexts/promoContextValue';
import { isPromoRejection } from '../lib/promo';
import { cartFixableMessage, isCartFixableCheckoutError, isCheckoutKeyConflict, isPaymentInProgressError, isPaymentMethodUnavailableError, serverMessageOr } from '../lib/checkoutErrors';
import { trackCheckoutStarted, getOrRenewSession, getOrCreateVisitorId } from '../lib/behavior';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function Checkout() {
  const { items, totalPrice, clearCart, refreshCart } = useCart();
  const { appliedCode, discount, isChecking: isPromoChecking, isUnconfirmed: isPromoUnconfirmed, rejectAfterCheckout, clear: clearPromo } = usePromo();
  const submitLockRef = useRef(false);
  const [idempotencyKey, setIdempotencyKey] = useState(() => crypto.randomUUID());
  const [deliveryMethods, setDeliveryMethods] = useState<PublicDeliveryMethod[]>([]);
  const [selectedMethodId, setSelectedMethodId] = useState<string>('');
  const [done, setDone] = useState(false);
  // Number of the created order, shown on the confirmation (before the payment page opens).
  const [confirmedOrderNumber, setConfirmedOrderNumber] = useState<string | null>(null);
  const { isAuthenticated, user, openAuthModal } = useAuth();
  const [firstName, setFirstName] = useState(user?.name?.split(' ')[0] || '');
  const [lastName, setLastName] = useState(user?.name?.split(' ')[1] || '');
  const [email, setEmail] = useState(user?.email || '');
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [validationError, setValidationError] = useState('');
  // The order was refused for a reason fixed in the cart (stock, unavailable item): offer the way back.
  const [cartFixable, setCartFixable] = useState(false);
  // Another order's payment is still in progress: offer the way to «Мои заказы».
  const [paymentBlocked, setPaymentBlocked] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  // Payment methods come from the server (its configuration decides); nothing is preselected until they are known.
  const [paymentMethod, setPaymentMethod] = useState<string>('');
  const [availableMethods, setAvailableMethods] = useState<AvailablePaymentMethods | null>(null);
  const [methodsFailed, setMethodsFailed] = useState(false);
  const [pendingOrderId, setPendingOrderId] = useState<string | null>(() => {
    return sessionStorage.getItem('zamk_pending_order_id');
  });
  const [pendingOrderDetails, setPendingOrderDetails] = useState<any | null>(null);

  const [savedAddresses, setSavedAddresses] = useState<any[]>([]);
  const [selectedAddressId, setSelectedAddressId] = useState<string>('new');

  const hasTrackedCheckoutRef = useRef(false);
  useEffect(() => {
    if (!hasTrackedCheckoutRef.current) {
      hasTrackedCheckoutRef.current = true;
      trackCheckoutStarted();
    }
  }, []);

  useEffect(() => {
    if (pendingOrderId && !items.length) {
      import('@zamk/api-client/src/customer').then(({ getOrder }) => {
        getOrder(pendingOrderId).then(ord => {
          if (ord && ord.status === 'awaiting_payment') {
            setPendingOrderDetails(ord);
          } else {
            sessionStorage.removeItem('zamk_pending_order_id');
            setPendingOrderId(null);
          }
        }).catch(() => {
          sessionStorage.removeItem('zamk_pending_order_id');
          setPendingOrderId(null);
        });
      });
    }
  }, [pendingOrderId, items.length]);

  const loadPaymentMethods = () => {
    setMethodsFailed(false);
    return getPaymentMethods().then(res => {
      const list = res?.methods ?? [];
      setAvailableMethods({ methods: list, default: res?.default ?? '' });
      setPaymentMethod(prev => (list.some(m => m.method === prev) ? prev : (res?.default ?? '')));
    }).catch(() => {
      setAvailableMethods(null);
      setMethodsFailed(true);
      setPaymentMethod('');
    });
  };

  useEffect(() => {
    void loadPaymentMethods();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const noPaymentMethods = methodsFailed || (availableMethods !== null && availableMethods.methods.length === 0);
  const noPaymentMethodsMessage = methodsFailed
    ? 'Не удалось получить способы оплаты. Обновите страницу и попробуйте ещё раз.'
    : 'Сейчас нет доступных способов оплаты. Заказ не оформлен — попробуйте позже.';

  // The confirmation replaces the long checkout form: show it from the top (on mobile the page was scrolled to the button).
  useEffect(() => {
    if (done) window.scrollTo({ top: 0, behavior: 'instant' });
  }, [done]);

  useEffect(() => {
    getDeliveryMethods().then(methods => {
      setDeliveryMethods(methods);
      if (methods.length > 0) {
        setSelectedMethodId(methods[0].id);
      }
    }).catch(console.error);
  }, []);

  useEffect(() => {
    import('@zamk/api-client/src/customer').then(({ getAddresses }) => {
      if (isAuthenticated) {
        getAddresses().then(data => {
          setSavedAddresses(data || []);
          const def = (data || []).find((a: any) => a.isDefault);
          if (def) {
            setSelectedAddressId(def.id);
          } else if (data && data.length > 0) {
            setSelectedAddressId(data[0].id);
          }
        }).catch(console.error);
      }
    });
  }, [isAuthenticated]);

  // Regenerate idempotency key when user actively modifies customer checkout details
  useEffect(() => {
    setIdempotencyKey(crypto.randomUUID());
  }, [selectedMethodId, selectedAddressId, firstName, lastName, email, phone, address, appliedCode]);

  const handleCheckout = async () => {
    if (!isAuthenticated) {
      openAuthModal('login');
      return;
    }

    if (submitLockRef.current || isSubmitting || done) {
      return;
    }

    if (appliedCode && (isPromoChecking || isPromoUnconfirmed)) {
      setValidationError('Дождитесь подтверждения промокода или удалите его.');
      return;
    }

    if (items.some(i => i.inStock === false)) {
      setValidationError('В заказе есть недоступные товары. Вернитесь в корзину и удалите их.');
      return;
    }

    // No available method: do not create an order that cannot be paid.
    if (noPaymentMethods || !paymentMethod) {
      setValidationError(availableMethods === null && !methodsFailed ? 'Способы оплаты ещё загружаются. Повторите через секунду.' : noPaymentMethodsMessage);
      return;
    }

    submitLockRef.current = true;
    setIsSubmitting(true);
    setCartFixable(false);
    setPaymentBlocked(false);
    let creatingOrder = false;

    const trimmedEmail = email.trim();
    let finalName = `${firstName.trim()} ${lastName.trim()}`.trim();
    let finalPhone = phone.trim();
    let finalAddress = address.trim();

    if (selectedAddressId !== 'new') {
      const addr = savedAddresses.find(a => a.id === selectedAddressId);
      if (addr) {
        finalName = addr.recipientName;
        finalPhone = addr.phone;
        finalAddress = formatDeliveryAddress(addr);
      }
    }

    if (!finalName || !trimmedEmail || !finalPhone || !finalAddress) {
      setValidationError('Укажите имя, email, телефон и адрес доставки.');
      submitLockRef.current = false;
      setIsSubmitting(false);
      return;
    }

    if (!EMAIL_PATTERN.test(trimmedEmail)) {
      setValidationError('Укажите корректный email.');
      submitLockRef.current = false;
      setIsSubmitting(false);
      return;
    }

    const phoneDigits = finalPhone.replace(/\D/g, '');
    if (phoneDigits.length < 7) {
      setValidationError('Укажите корректный телефон, минимум 7 цифр.');
      submitLockRef.current = false;
      setIsSubmitting(false);
      return;
    }

    try {
      const deliveryAddressText = finalAddress;

      let orderId = pendingOrderId;
      if (orderId) {
        // Paying the already created order again (e.g. with another method): keep the number already shown.
        setConfirmedOrderNumber(prev => pendingOrderDetails?.orderNumber ?? prev);
      }
      if (!orderId) {
        const visitorId = getOrCreateVisitorId();
        const sessionInfo = visitorId ? getOrRenewSession(visitorId) : null;
        const analyticsContext = sessionInfo && visitorId ? {
          visitorId: visitorId,
          sessionId: sessionInfo.sessionId
        } : undefined;

        creatingOrder = true;
        const order = await createOrder({
          customerName: finalName,
          customerEmail: trimmedEmail,
          customerPhone: finalPhone,
          deliveryAddress: deliveryAddressText,
          deliveryMethodId: selectedMethodId,
          promoCode: appliedCode ?? undefined,
          analyticsContext
        }, idempotencyKey);
        creatingOrder = false;
        orderId = order.id;
        setConfirmedOrderNumber(order.orderNumber ?? null);
        setPendingOrderId(order.id);
        sessionStorage.setItem('zamk_pending_order_id', order.id);
      }

      const payment = await createPayment(orderId, paymentMethod);

      setValidationError('');
      setDone(true);
      sessionStorage.removeItem('zamk_pending_order_id');
      clearPromo();
      await clearCart();

      if (payment.integrationMode === 'mock') {
        setTimeout(() => {
          window.location.href = payment.paymentUrl;
        }, 500);
      } else {
        if (payment.paymentUrl) {
          window.location.href = payment.paymentUrl;
        }
      }
    } catch (e: any) {
      if (isPromoRejection(e)) {
        // The promo stopped being valid between preview and order creation. No order was created;
        // totals fall back to the real full price and the customer must confirm again.
        rejectAfterCheckout(e);
        setIdempotencyKey(crypto.randomUUID());
        setValidationError('Промокод больше не действует. Проверьте обновлённую сумму и оформите заказ ещё раз.');
      } else if (creatingOrder && isCartFixableCheckoutError(e)) {
        // No order was created and nothing reserved: reload the cart so the unavailable lines are shown there.
        setCartFixable(true);
        setValidationError(cartFixableMessage(e));
        setIdempotencyKey(crypto.randomUUID());
        void refreshCart?.();
      } else if (isPaymentInProgressError(e)) {
        // Refused before the order was created (or, for an order created earlier, before its payment started):
        // the earlier order must be paid or its payment cancelled first. Nothing was reserved for a new order.
        setPaymentBlocked(true);
        setValidationError(serverMessageOr(e, 'Оплата предыдущего заказа ещё не завершена. Завершите её в разделе «Мои заказы».'));
        if (creatingOrder) setIdempotencyKey(crypto.randomUUID());
      } else if (isPaymentMethodUnavailableError(e)) {
        // The order is saved and still awaits payment; the next submit pays it with the newly chosen method.
        void loadPaymentMethods();
        setValidationError(`${serverMessageOr(e, 'Этот способ оплаты сейчас недоступен.')} Заказ сохранён — выберите другой способ оплаты и нажмите кнопку ещё раз.`);
      } else if (isCheckoutKeyConflict(e)) {
        setValidationError('Вы изменили данные после отправки заказа. Пожалуйста, проверьте информацию и попробуйте снова.');
        setIdempotencyKey(crypto.randomUUID()); // Reset key for next attempt
        setPendingOrderId(null);
        sessionStorage.removeItem('zamk_pending_order_id');
      } else if (e.message && (e.message.includes('not awaiting payment') || e.message.includes('order not found') || e.message.includes('cancelled'))) {
        setPendingOrderId(null);
        sessionStorage.removeItem('zamk_pending_order_id');
        setValidationError(e.message || 'Заказ не может быть оплачен. Пожалуйста, оформите новый заказ.');
      } else if (e.message && e.message.includes('insufficient_stock')) {
        setValidationError('Товара больше нет в наличии для завершения оплаты этого заказа.');
      } else {
        setValidationError(e.message || 'Не удалось сохранить заказ. Попробуйте ещё раз.');
      }
      submitLockRef.current = false;
      setIsSubmitting(false);
    }
  };

  if (done) {
    return (
      <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl'>
          <CheckoutPanel>
            <div className='text-center'>
              <div className='mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-white dark:bg-white/10 border border-border-lighter dark:border-white/15'>
                <Check className='w-10 h-10 text-success' />
              </div>
              <h1 className='mt-6 text-4xl font-serif text-graphite dark:text-white'>Заказ оформлен</h1>
              {confirmedOrderNumber && (
                <p className='mt-3 text-graphite dark:text-white' data-testid='checkout-order-number'>Номер заказа: <span className='font-semibold'>{confirmedOrderNumber}</span></p>
              )}
              <p className='mt-3 text-graphite-light dark:text-white/70'>Заказ ожидает оплаты — открываем страницу оплаты. Заказ всегда доступен в разделе «Мои заказы».</p>
              <div className='mt-7 flex justify-center gap-3'>
                <Link to='/orders'><Button>Мои заказы</Button></Link>
                <Link to='/catalog'><Button variant='secondary'>В каталог</Button></Link>
              </div>
            </div>
          </CheckoutPanel>
        </div>
      </div>
    );
  }

  if (!items.length && pendingOrderDetails) {
    return (
      <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-xl'>
          <CheckoutPanel>
            <h1 className='text-2xl font-serif text-graphite dark:text-white mb-2'>
              Оплата заказа {pendingOrderDetails.orderNumber || pendingOrderDetails.id}
            </h1>
            <p className='text-sm text-graphite-light dark:text-white/70 mb-6'>
              Заказ уже сформирован и ожидает оплаты. Выберите способ и завершите покупку.
            </p>

            {validationError && (
              <div className="mb-6 p-4 rounded-xl bg-danger/10 border border-danger/20 text-danger text-sm">
                {validationError}
              </div>
            )}

            <div className="mb-6">
              <PaymentMethodPicker available={availableMethods} failed={methodsFailed} selected={paymentMethod} onSelect={setPaymentMethod} />
            </div>

            <div className="flex flex-col gap-3">
              <Button
                onClick={async () => {
                  try {
                    setIsSubmitting(true);
                    setValidationError('');
                    const { createPayment } = await import('@zamk/api-client/src/customer');
                    const payment = await createPayment(pendingOrderDetails.id, paymentMethod);
                    sessionStorage.removeItem('zamk_pending_order_id');
                    if (payment.paymentUrl) {
                      window.location.href = payment.paymentUrl;
                    }
                  } catch (e: any) {
                    setIsSubmitting(false);
                    if (e.message && e.message.includes('insufficient_stock')) {
                      setValidationError('Товара больше нет в наличии для завершения оплаты заказа.');
                    } else {
                      setValidationError(e.message || 'Не удалось провести оплату. Попробуйте еще раз.');
                    }
                  }
                }}
                disabled={isSubmitting || !paymentMethod}
                className="w-full"
              >
                {isSubmitting ? 'Обработка...' : `Оплатить ${(pendingOrderDetails.totalPriceCents / 100).toLocaleString('ru-RU')} ₽`}
              </Button>

              <div className="flex justify-between items-center text-xs mt-2">
                <Link to="/orders" className="text-ash hover:text-graphite dark:hover:text-white underline">
                  Перейти в мои заказы
                </Link>
                <button
                  type="button"
                  onClick={() => {
                    sessionStorage.removeItem('zamk_pending_order_id');
                    setPendingOrderId(null);
                    setPendingOrderDetails(null);
                  }}
                  className="text-ash hover:text-danger underline"
                >
                  Отменить и перейти в каталог
                </button>
              </div>
            </div>
          </CheckoutPanel>
        </div>
      </div>
    );
  }

  if (!items.length) {
    return (
      <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite dark:text-white'>Корзина пуста</h1>
          <Link to='/catalog' className='inline-block mt-6'><Button>Перейти в каталог</Button></Link>
        </div>
      </div>
    );
  }

  const selectedMethod = deliveryMethods.find(m => m.id === selectedMethodId);
  const delivery = selectedMethod ? selectedMethod.priceCents / 100 : 0;
  const total = Math.max(totalPrice - discount, 0) + delivery;

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
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
              <SectionHeader label='Шаг 1' title='Контактные данные и адрес' />

              {isAuthenticated && savedAddresses.length > 0 && (
                <div className="mb-4">
                  <div className="flex items-center justify-between gap-3 mb-2">
                    <label className="text-sm font-medium text-graphite dark:text-white block">Сохранённый адрес</label>
                    <Link to="/settings" data-testid="manage-addresses" className="text-[12.5px] text-graphite/60 dark:text-white/60 underline underline-offset-2">Управлять адресами</Link>
                  </div>
                  <select
                    data-testid="saved-address-select"
                    className="w-full bg-white dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-3 text-sm text-graphite dark:text-white outline-none focus:ring-1 focus:ring-graphite/20 dark:focus:ring-white/30"
                    value={selectedAddressId}
                    onChange={(e) => setSelectedAddressId(e.target.value)}
                  >
                    {savedAddresses.map(a => (
                      <option key={a.id} value={a.id}>{a.label || a.city + ', ' + a.street} ({a.recipientName}){a.isDefault ? ' · основной' : ''}</option>
                    ))}
                    <option value="new">Ввести новый адрес...</option>
                  </select>
                </div>
              )}

              {selectedAddressId === 'new' || savedAddresses.length === 0 ? (
                <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                  <Input placeholder='Имя' value={firstName} onChange={(event) => { setFirstName(event.target.value); if (validationError) setValidationError(''); }} autoComplete='given-name' required />
                  <Input placeholder='Фамилия' value={lastName} onChange={(event) => setLastName(event.target.value)} autoComplete='family-name' />
                  <Input placeholder='Эл. почта' type='email' value={email} onChange={(event) => { setEmail(event.target.value); if (validationError) setValidationError(''); }} autoComplete='email' required />
                  <Input placeholder='Телефон' type='tel' value={phone} onChange={(event) => { setPhone(event.target.value); if (validationError) setValidationError(''); }} autoComplete='tel' required />
                  <div className='md:col-span-2'>
                    <Input placeholder='Полный адрес доставки' value={address} onChange={(event) => { setAddress(event.target.value); if (validationError) setValidationError(''); }} required />
                  </div>
                </div>
              ) : (
                <div data-testid="saved-address-summary" className="p-4 bg-ice/50 dark:bg-white/5 rounded-xl border border-border-lighter dark:border-white/10 text-sm text-graphite dark:text-white mt-4">
                  {(() => {
                    const a = savedAddresses.find(x => x.id === selectedAddressId);
                    if (!a) return null;
                    return (
                      <>
                        <p className="font-medium text-[15px] mb-1">{a.recipientName}</p>
                        <p className="text-graphite/70 dark:text-white/70 mb-2">{a.phone}</p>
                        <p className="text-graphite-light dark:text-white/80 leading-relaxed">
                          {formatDeliveryAddress(a)}
                        </p>
                      </>
                    );
                  })()}
                  <div className="mt-4 pt-4 border-t border-border-lighter dark:border-white/10">
                    <Input placeholder='Эл. почта для получения чека' type='email' value={email} onChange={(e) => { setEmail(e.target.value); if (validationError) setValidationError(''); }} required />
                  </div>
                </div>
              )}

              {validationError && (
                <p className='mt-3 text-sm text-error' role='alert'>
                  {validationError}
                  {cartFixable && (
                    <>
                      {' '}
                      <Link to='/cart' data-testid='checkout-back-to-cart' className='underline underline-offset-2'>Вернуться в корзину</Link>
                    </>
                  )}
                  {paymentBlocked && (
                    <>
                      {' '}
                      <Link to='/orders' data-testid='checkout-to-orders' className='underline underline-offset-2'>Мои заказы</Link>
                    </>
                  )}
                </p>
              )}
            </CheckoutPanel>

            <CheckoutPanel>
              <SectionHeader label='Шаг 2' title='Доставка' />
              <div className='flex flex-wrap gap-2'>
                {deliveryMethods.length > 0 ? (
                  deliveryMethods.map((m) => (
                    <PillFilter
                      key={m.id}
                      label={`${m.name} (${m.priceCents === 0 ? "Бесплатно" : formatPrice(m.priceCents / 100)})`}
                      active={selectedMethodId === m.id}
                      onClick={() => setSelectedMethodId(m.id)}
                    />
                  ))
                ) : (
                  <p className='text-sm text-graphite-light dark:text-white/60'>Загрузка способов доставки...</p>
                )}
              </div>
            </CheckoutPanel>

            <CheckoutPanel>
              <SectionHeader label='Шаг 3' title='Оплата' />
              <PaymentMethodPicker available={availableMethods} failed={methodsFailed} selected={paymentMethod} onSelect={setPaymentMethod} />
            </CheckoutPanel>
          </div>

          <div className='lg:col-span-4'>
            <CheckoutPanel>
              <SectionHeader label='Итог' title='Сводка заказа' />
              <div className='space-y-3 text-sm'>
                {items.map((item) => {
                  const productName = item.title || item.product?.name || 'Неизвестный товар';
                  const itemPrice = item.price ? item.price * item.quantity : 0;
                  const variantDetails = formatVariantDetails(item.color, item.size);
                  const isUnavailable = item.inStock === false;
                  return (
                  <div key={item.id} className='flex justify-between items-start text-graphite-light dark:text-white/68'>
                    <div className='flex flex-col pr-2'>
                      <span className='font-medium text-graphite dark:text-white'>
                        {productName}
                      </span>
                      {variantDetails && (
                        <span className='text-xs text-ash'>
                          {variantDetails}
                        </span>
                      )}
                      <span className='text-xs text-ash mt-0.5'>
                        × {item.quantity}
                      </span>
                      {isUnavailable && (
                        <span className='text-xs text-error font-medium mt-0.5'>
                          (нет в наличии)
                        </span>
                      )}
                    </div>
                    <span className='whitespace-nowrap font-medium text-graphite dark:text-white'>
                      {formatPrice(itemPrice)}
                    </span>
                  </div>
                )})}
                <div className='border-t border-border-lighter dark:border-white/10 pt-3 mt-3 space-y-2'>
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Товары</span><span>{formatPrice(totalPrice)}</span></div>
                  {discount > 0 && (
                    <div className='flex justify-between text-success' data-testid='checkout-promo-discount'>
                      <span>Промокод {appliedCode}</span>
                      <span>−{formatPrice(discount)}</span>
                    </div>
                  )}
                  <PromoCodeBox />
                  <div className='flex justify-between text-graphite dark:text-white/84'><span>Доставка</span><span className='text-xs'>{delivery === 0 ? 'Бесплатно' : formatPrice(delivery)}</span></div>
                  <div className='flex justify-between text-base font-semibold text-graphite dark:text-white'><span>Итого</span><span>{isPromoChecking && appliedCode ? 'Пересчитываем…' : formatPrice(total)}</span></div>
                </div>
              </div>

              {items.some(i => i.inStock === false) && (
                <p className='mt-3 text-xs text-error'>
                  В заказе есть недоступные товары. Пожалуйста, вернитесь в корзину и удалите их.
                </p>
              )}

              <Button
                type='button'
                className='w-full mt-5'
                onClick={handleCheckout}
                disabled={isSubmitting || !paymentMethod || items.some(i => i.inStock === false) || (!!appliedCode && (isPromoChecking || isPromoUnconfirmed))}
              >
                Оформить заказ
              </Button>
            </CheckoutPanel>
          </div>
        </div>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft, Check, CreditCard, Truck, MapPin } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';

export function Checkout() {
  const { items, totalPrice, clearCart } = useCart();
  const [step, setStep] = useState(1);
  const [orderPlaced, setOrderPlaced] = useState(false);

  const delivery = totalPrice >= 10000 ? 0 : 590;
  const finalTotal = totalPrice + delivery;

  if (orderPlaced) {
    return (
      <div className="min-h-screen bg-milk flex items-center justify-center px-4">
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          className="text-center max-w-md"
        >
          <div className="w-20 h-20 rounded-full bg-success/10 flex items-center justify-center mx-auto mb-6">
            <Check className="w-10 h-10 text-success" />
          </div>
          <h2 className="text-2xl font-serif text-graphite mb-3">Заказ оформлен!</h2>
          <p className="text-sm text-ash mb-6">
            Спасибо за покупку! Номер заказа: #ZMK-{Math.floor(Math.random() * 90000) + 10000}. Мы отправим подтверждение на вашу почту.
          </p>
          <div className="flex gap-3 justify-center">
            <Link to="/">
              <Button variant="primary">На главную</Button>
            </Link>
            <Link to="/catalog">
              <Button variant="secondary">Продолжить покупки</Button>
            </Link>
          </div>
        </motion.div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="min-h-screen bg-milk flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-2xl font-serif text-graphite mb-4">Корзина пуста</h2>
          <Link to="/catalog"><Button variant="primary">Перейти в каталог</Button></Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-4xl py-8">
        <Link to="/cart" className="inline-flex items-center gap-1.5 text-sm text-ash hover:text-graphite transition-colors mb-6">
          <ArrowLeft className="w-4 h-4" /> Назад в корзину
        </Link>

        <h1 className="text-3xl font-serif text-graphite mb-8">Оформление заказа</h1>

        {/* Steps indicator */}
        <div className="flex items-center gap-3 mb-10">
          {['Контакты', 'Доставка', 'Оплата'].map((label, i) => (
            <div key={i} className="flex items-center gap-3 flex-1">
              <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold shrink-0 transition-all ${
                step > i + 1
                  ? 'bg-success text-white'
                  : step === i + 1
                    ? 'bg-primary text-white'
                    : 'bg-surface text-ash'
              }`}>
                {step > i + 1 ? <Check className="w-4 h-4" /> : i + 1}
              </div>
              <span className={`text-sm hidden sm:block ${step === i + 1 ? 'text-graphite font-medium' : 'text-ash'}`}>{label}</span>
              {i < 2 && <div className={`flex-1 h-px ${step > i + 1 ? 'bg-success' : 'bg-border-lighter'}`} />}
            </div>
          ))}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Form */}
          <div className="lg:col-span-2">
            {step === 1 && (
              <motion.div initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }} className="space-y-5">
                <h2 className="text-xl font-semibold text-graphite flex items-center gap-2"><MapPin className="w-5 h-5 text-primary" /> Контактные данные</h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <Input placeholder="Имя" />
                  <Input placeholder="Фамилия" />
                </div>
                <Input placeholder="Эл. почта" type="email" />
                <Input placeholder="Телефон" type="tel" />
                <h3 className="text-lg font-semibold text-graphite pt-4">Адрес доставки</h3>
                <Input placeholder="Город" />
                <Input placeholder="Улица, дом, квартира" />
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <Input placeholder="Индекс" />
                  <Input placeholder="Комментарий к заказу" />
                </div>
                <Button variant="primary" size="lg" className="w-full sm:w-auto" onClick={() => setStep(2)}>
                  Далее: Доставка
                </Button>
              </motion.div>
            )}

            {step === 2 && (
              <motion.div initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }} className="space-y-5">
                <h2 className="text-xl font-semibold text-graphite flex items-center gap-2"><Truck className="w-5 h-5 text-primary" /> Способ доставки</h2>
                {[
                  { id: 'courier', label: 'Курьерская доставка', desc: '1-2 рабочих дня по Москве', price: delivery === 0 ? 'Бесплатно' : '590 ₽' },
                  { id: 'pickup', label: 'Самовывоз', desc: 'Пункт выдачи на выбор', price: 'Бесплатно' },
                  { id: 'express', label: 'Экспресс-доставка', desc: 'Сегодня-завтра', price: '990 ₽' },
                ].map(opt => (
                  <label key={opt.id} className="flex items-center gap-4 p-4 bg-white rounded-2xl border border-border-lighter cursor-pointer hover:border-primary/30 transition-all">
                    <input type="radio" name="delivery" defaultChecked={opt.id === 'courier'} className="accent-primary w-4 h-4" />
                    <div className="flex-1">
                      <p className="text-sm font-medium text-graphite">{opt.label}</p>
                      <p className="text-xs text-ash">{opt.desc}</p>
                    </div>
                    <span className="text-sm font-medium text-graphite">{opt.price}</span>
                  </label>
                ))}
                <div className="flex gap-3">
                  <Button variant="secondary" onClick={() => setStep(1)}>Назад</Button>
                  <Button variant="primary" size="lg" onClick={() => setStep(3)}>Далее: Оплата</Button>
                </div>
              </motion.div>
            )}

            {step === 3 && (
              <motion.div initial={{ opacity: 0, x: 10 }} animate={{ opacity: 1, x: 0 }} className="space-y-5">
                <h2 className="text-xl font-semibold text-graphite flex items-center gap-2"><CreditCard className="w-5 h-5 text-primary" /> Способ оплаты</h2>
                {[
                  { id: 'card', label: 'Банковская карта', desc: 'Visa, Mastercard, Мир' },
                  { id: 'sbp', label: 'СБП', desc: 'Система быстрых платежей' },
                  { id: 'cash', label: 'При получении', desc: 'Картой или наличными' },
                ].map(opt => (
                  <label key={opt.id} className="flex items-center gap-4 p-4 bg-white rounded-2xl border border-border-lighter cursor-pointer hover:border-primary/30 transition-all">
                    <input type="radio" name="payment" defaultChecked={opt.id === 'card'} className="accent-primary w-4 h-4" />
                    <div>
                      <p className="text-sm font-medium text-graphite">{opt.label}</p>
                      <p className="text-xs text-ash">{opt.desc}</p>
                    </div>
                  </label>
                ))}
                <div className="flex gap-3">
                  <Button variant="secondary" onClick={() => setStep(2)}>Назад</Button>
                  <Button variant="primary" size="lg" className="gap-2" onClick={() => { setOrderPlaced(true); clearCart(); }}>
                    Оплатить {formatPrice(finalTotal)}
                  </Button>
                </div>
              </motion.div>
            )}
          </div>

          {/* Sidebar Summary */}
          <div className="lg:col-span-1">
            <div className="bg-white rounded-2xl border border-border-lighter p-5 space-y-4 lg:sticky lg:top-24">
              <h3 className="text-base font-semibold text-graphite">Ваш заказ</h3>
              <div className="space-y-3">
                {items.map(item => (
                  <div key={item.product.id} className="flex gap-3">
                    <div className="w-12 h-14 rounded-lg bg-surface overflow-hidden shrink-0">
                      <img src={item.product.image} alt="" className="w-full h-full object-cover" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs text-graphite font-medium truncate">{item.product.name}</p>
                      <p className="text-xs text-ash">{item.quantity} × {formatPrice(item.product.price)}</p>
                    </div>
                  </div>
                ))}
              </div>
              <hr className="border-border-lighter" />
              <div className="space-y-2 text-sm">
                <div className="flex justify-between"><span className="text-ash">Сумма</span><span>{formatPrice(totalPrice)}</span></div>
                <div className="flex justify-between"><span className="text-ash">Доставка</span><span>{delivery === 0 ? 'Бесплатно' : formatPrice(delivery)}</span></div>
                <hr className="border-border-lighter" />
                <div className="flex justify-between font-semibold text-base"><span>Итого</span><span>{formatPrice(finalTotal)}</span></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

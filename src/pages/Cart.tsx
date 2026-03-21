import { Link } from 'react-router-dom';
import { Minus, Plus, Trash2, Tag, ArrowRight } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { EmptyState } from '../components/ui/EmptyState';
import { ProductCard } from '../components/product/ProductCard';
import { useCart } from '../contexts/CartContext';
import { formatPrice } from '../lib/utils';
import { PRODUCTS } from '../lib/mock-data';
import { useState } from 'react';

export function Cart() {
  const { items, updateQuantity, removeItem, totalPrice } = useCart();
  const [promoCode, setPromoCode] = useState('');
  const [promoApplied, setPromoApplied] = useState(false);

  const delivery = totalPrice >= 10000 ? 0 : 590;
  const discount = promoApplied ? Math.round(totalPrice * 0.1) : 0;
  const finalTotal = totalPrice - discount + delivery;

  const recommendations = PRODUCTS.filter(p => !items.find(i => i.product.id === p.id)).slice(0, 4);

  if (items.length === 0) {
    return (
      <div className="min-h-screen bg-milk">
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-8">
          <h1 className="text-3xl font-serif text-graphite mb-8">Корзина</h1>
          <EmptyState
            icon="cart"
            title="Корзина пуста"
            description="Добавьте товары из каталога, чтобы оформить заказ"
            action={
              <Link to="/catalog">
                <Button variant="primary" className="gap-2">
                  Перейти в каталог <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-8">
        <h1 className="text-3xl font-serif text-graphite mb-2">Корзина</h1>
        <p className="text-sm text-ash mb-8">{items.length} товаров · Бесплатная доставка от 10 000 ₽</p>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Cart Items */}
          <div className="lg:col-span-2 space-y-4">
            <AnimatePresence>
              {items.map(item => (
                <motion.div
                  key={item.product.id}
                  layout
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, x: -100 }}
                  className="bg-white rounded-2xl border border-border-lighter p-4 flex gap-4"
                >
                  <Link to={`/product/${item.product.id}`} className="w-24 h-28 rounded-xl bg-surface overflow-hidden shrink-0">
                    <img src={item.product.image} alt={item.product.name} className="w-full h-full object-cover" />
                  </Link>
                  <div className="flex-1 min-w-0">
                    <div className="flex justify-between items-start gap-2">
                      <div className="min-w-0">
                        <Link to={`/product/${item.product.id}`}>
                          <h3 className="text-sm font-medium text-graphite truncate hover:text-primary transition-colors">{item.product.name}</h3>
                        </Link>
                        <p className="text-xs text-ash mt-0.5">{item.product.brand}</p>
                        {item.selectedSize && <p className="text-xs text-ash mt-0.5">Размер: {item.selectedSize}</p>}
                        {item.selectedColor && <p className="text-xs text-ash mt-0.5">Цвет: {item.selectedColor}</p>}
                      </div>
                      <p className="text-sm font-semibold text-graphite whitespace-nowrap">{formatPrice(item.product.price)}</p>
                    </div>
                    <div className="flex items-center justify-between mt-3">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => updateQuantity(item.product.id, item.quantity - 1)}
                          className="w-8 h-8 rounded-lg border border-border-soft flex items-center justify-center text-ash hover:text-graphite hover:border-primary/40 transition-all"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                        <span className="w-8 text-center text-sm font-medium">{item.quantity}</span>
                        <button
                          onClick={() => updateQuantity(item.product.id, item.quantity + 1)}
                          className="w-8 h-8 rounded-lg border border-border-soft flex items-center justify-center text-ash hover:text-graphite hover:border-primary/40 transition-all"
                        >
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                      <button
                        onClick={() => removeItem(item.product.id)}
                        className="text-ash hover:text-error transition-colors p-1"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>

          {/* Order Summary (sticky) */}
          <div className="lg:col-span-1">
            <div className="bg-white rounded-2xl border border-border-lighter p-6 space-y-5 lg:sticky lg:top-24">
              <h3 className="text-lg font-semibold text-graphite">Итого</h3>

              {/* Promo */}
              <div className="flex gap-2">
                <div className="flex-1 relative">
                  <Tag className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ash-light" />
                  <input
                    type="text"
                    placeholder="Промокод"
                    value={promoCode}
                    onChange={e => setPromoCode(e.target.value)}
                    className="w-full h-10 rounded-xl border border-border-soft bg-white pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/30 transition-all"
                  />
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => { if (promoCode) setPromoApplied(true); }}
                  disabled={!promoCode}
                >
                  Применить
                </Button>
              </div>
              {promoApplied && (
                <p className="text-xs text-success font-medium">✓ Промокод применён — скидка 10%</p>
              )}

              <div className="space-y-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-ash">Сумма</span>
                  <span className="text-graphite">{formatPrice(totalPrice)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-ash">Доставка</span>
                  <span className="text-graphite">{delivery === 0 ? 'Бесплатно' : formatPrice(delivery)}</span>
                </div>
                {discount > 0 && (
                  <div className="flex justify-between text-success">
                    <span>Скидка</span>
                    <span>-{formatPrice(discount)}</span>
                  </div>
                )}
                <hr className="border-border-lighter" />
                <div className="flex justify-between text-base font-semibold">
                  <span className="text-graphite">Итого</span>
                  <span className="text-graphite">{formatPrice(finalTotal)}</span>
                </div>
              </div>

              <Link to="/checkout">
                <Button variant="primary" size="lg" className="w-full gap-2">
                  Оформить заказ
                  <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
            </div>
          </div>
        </div>

        {/* Recommendations */}
        {recommendations.length > 0 && (
          <section className="mt-16">
            <h2 className="text-2xl font-serif text-graphite mb-8">Вам может понравиться</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
              {recommendations.map(p => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}

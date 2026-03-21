import { Truck, RotateCcw, Clock, Shield } from 'lucide-react';

export function Delivery() {
  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl py-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-8">Доставка и возврат</h1>

        <div className="space-y-6">
          {/* Delivery */}
          <div className="bg-white rounded-3xl border border-border-lighter p-6 sm:p-8">
            <div className="flex items-center gap-3 mb-5">
              <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center"><Truck className="w-5 h-5 text-primary" /></div>
              <h2 className="text-xl font-semibold text-graphite">Доставка</h2>
            </div>
            <div className="space-y-4 text-sm text-ash leading-relaxed">
              <div className="flex justify-between items-center py-3 border-b border-border-lighter">
                <span>Курьерская доставка по Москве</span>
                <span className="text-graphite font-medium">1-2 рабочих дня</span>
              </div>
              <div className="flex justify-between items-center py-3 border-b border-border-lighter">
                <span>Доставка по России (СДЭК, Почта)</span>
                <span className="text-graphite font-medium">3-7 рабочих дней</span>
              </div>
              <div className="flex justify-between items-center py-3 border-b border-border-lighter">
                <span>Экспресс-доставка</span>
                <span className="text-graphite font-medium">В день заказа</span>
              </div>
              <p className="pt-2">Бесплатная доставка для заказов от 10 000 ₽. Стоимость стандартной доставки — 590 ₽.</p>
            </div>
          </div>

          {/* Returns */}
          <div className="bg-white rounded-3xl border border-border-lighter p-6 sm:p-8">
            <div className="flex items-center gap-3 mb-5">
              <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center"><RotateCcw className="w-5 h-5 text-primary" /></div>
              <h2 className="text-xl font-semibold text-graphite">Возврат</h2>
            </div>
            <div className="space-y-3 text-sm text-ash leading-relaxed">
              <p>Вы можете вернуть товар в течение 14 дней с момента получения.</p>
              <p>Товар должен быть в оригинальном состоянии, с бирками и в фирменной упаковке.</p>
              <p>Возврат денежных средств осуществляется в течение 3-5 рабочих дней после получения нами товара.</p>
            </div>
          </div>

          {/* Guarantees */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="bg-white rounded-2xl border border-border-lighter p-5 flex items-start gap-3">
              <Clock className="w-5 h-5 text-primary shrink-0 mt-0.5" />
              <div>
                <h3 className="text-sm font-semibold text-graphite mb-1">Быстрая обработка</h3>
                <p className="text-xs text-ash">Заказы обрабатываются в течение 2 часов в рабочее время</p>
              </div>
            </div>
            <div className="bg-white rounded-2xl border border-border-lighter p-5 flex items-start gap-3">
              <Shield className="w-5 h-5 text-primary shrink-0 mt-0.5" />
              <div>
                <h3 className="text-sm font-semibold text-graphite mb-1">Гарантия подлинности</h3>
                <p className="text-xs text-ash">Все товары — 100% оригинал, напрямую от брендов</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

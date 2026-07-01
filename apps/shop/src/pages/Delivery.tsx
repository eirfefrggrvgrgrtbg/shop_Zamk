import { Truck, RotateCcw, Clock, Shield } from 'lucide-react';

export function Delivery() {
  return (
    <div className="min-h-screen relative z-10 pt-32 md:pt-40 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="mb-12 max-w-[980px] mx-auto border-b border-border-lighter pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
            Условия магазина
          </p>
          <div className="flex items-end justify-between">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Доставка и возврат
            </h1>
          </div>
        </section>

        <div className="mt-8 space-y-8 max-w-[980px] mx-auto">
          {/* Delivery */}
          <div className="bg-white/70 backdrop-blur-xl rounded-[2.5rem] border border-white p-8 md:p-10 shadow-cloud">
            <div className="flex items-center gap-4 mb-6">
              <div className="w-12 h-12 rounded-[1rem] bg-white border border-border-lighter shadow-sm flex items-center justify-center"><Truck className="w-6 h-6 text-primary" /></div>
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
          <div className="bg-white/70 backdrop-blur-xl rounded-[2.5rem] border border-white p-8 md:p-10 shadow-cloud">
            <div className="flex items-center gap-4 mb-6">
              <div className="w-12 h-12 rounded-[1rem] bg-white border border-border-lighter shadow-sm flex items-center justify-center"><RotateCcw className="w-6 h-6 text-primary" /></div>
              <h2 className="text-xl font-semibold text-graphite">Возврат</h2>
            </div>
            <div className="space-y-3 text-sm text-ash leading-relaxed">
              <p>Вы можете вернуть товар в течение 14 дней с момента получения.</p>
              <p>Товар должен быть в оригинальном состоянии, с бирками и в фирменной упаковке.</p>
              <p>Возврат денежных средств осуществляется в течение 3-5 рабочих дней после получения нами товара.</p>
            </div>
          </div>

          {/* Guarantees */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
            <div className="bg-white/60 backdrop-blur-md rounded-[2rem] border border-white p-6 sm:p-8 shadow-sm flex items-start gap-4">
              <Clock className="w-5 h-5 text-primary shrink-0 mt-0.5" />
              <div>
                <h3 className="text-sm font-semibold text-graphite mb-1">Быстрая обработка</h3>
                <p className="text-xs text-ash">Заказы обрабатываются в течение 2 часов в рабочее время</p>
              </div>
            </div>
            <div className="bg-white/60 backdrop-blur-md rounded-[2rem] border border-white p-6 sm:p-8 shadow-sm flex items-start gap-4">
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

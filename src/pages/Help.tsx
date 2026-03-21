import { useState } from 'react';
import { ChevronDown, HelpCircle } from 'lucide-react';

const faqData = [
  { q: 'Как оформить заказ?', a: 'Выберите товар, добавьте его в корзину, перейдите к оформлению заказа. Укажите контактные данные, адрес доставки, выберите способ оплаты и подтвердите заказ.' },
  { q: 'Какие способы оплаты доступны?', a: 'Мы принимаем банковские карты (Visa, Mastercard, Мир), СБП, а также оплату при получении (картой или наличными).' },
  { q: 'Как отследить заказ?', a: 'После отправки заказа вы получите трек-номер на эл. почту. Отследить посылку можно в личном кабинете в разделе «Мои заказы».' },
  { q: 'Можно ли изменить или отменить заказ?', a: 'Вы можете изменить или отменить заказ до момента его отправки. Свяжитесь с нами по эл. почте или через форму обратной связи.' },
  { q: 'Как подобрать размер?', a: 'На каждой странице товара указаны размеры. Если вы сомневаетесь, свяжитесь с нашими стилистами — мы поможем с выбором.' },
  { q: 'Все ли товары оригинальные?', a: 'Да, мы работаем только напрямую с брендами и авторизованными дистрибьюторами. Гарантируем 100% подлинность каждого товара.' },
];

export function Help() {
  const [openIndex, setOpenIndex] = useState<number | null>(0);

  return (
    <div className="min-h-screen relative z-10 pt-16 md:pt-20 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm">
          <div className="relative h-[190px] md:h-[240px]">
            <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]" />
            <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]" />
            <div className="relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7">
              <h2 className="font-serif text-[clamp(2.4rem,6.6vw,7rem)] text-white/43 leading-[0.8] tracking-[-0.03em]">ПОМОЩЬ</h2>
              <h3 className="font-serif text-[clamp(1.8rem,5vw,5rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center">ЧАСТЫЕ ВОПРОСЫ</h3>
            </div>
          </div>
        </section>

        <div className="mt-8 space-y-4 max-w-[980px] mx-auto">
          {faqData.map((item, i) => (
            <div key={i} className="bg-white/70 backdrop-blur-xl rounded-[2rem] border border-white overflow-hidden shadow-sm hover:shadow-cloud transition-all duration-300">
              <button
                onClick={() => setOpenIndex(openIndex === i ? null : i)}
                className="w-full flex items-center justify-between px-6 py-5 md:py-6 text-left"
              >
                <span className="text-base md:text-lg font-medium text-graphite pr-4">{item.q}</span>
                <ChevronDown className={`w-4 h-4 text-ash shrink-0 transition-transform ${openIndex === i ? 'rotate-180' : ''}`} />
              </button>
              {openIndex === i && (
                <div className="px-6 pb-6 pt-2">
                  <p className="text-base text-ash leading-relaxed">{item.a}</p>
                </div>
              )}
            </div>
          ))}
        </div>

        <div className="mt-12 bg-white/60 backdrop-blur-xl rounded-[2.5rem] border border-white p-8 md:p-12 text-center shadow-cloud max-w-[980px] mx-auto">
          <div className="w-16 h-16 rounded-[1.2rem] bg-white border border-white shadow-sm flex items-center justify-center mx-auto mb-6">
            <HelpCircle className="w-8 h-8 text-primary" />
          </div>
          <h3 className="text-2xl font-serif text-graphite mb-3">Не нашли ответ?</h3>
          <p className="text-base text-ash mb-6">Напишите нам, и мы ответим в течение 24 часов</p>
          <a href="mailto:help@zamk.store" className="inline-block text-base font-medium text-white bg-graphite hover:bg-primary transition-colors px-8 py-3.5 rounded-full shadow-sm">
            help@zamk.store
          </a>
        </div>
      </div>
    </div>
  );
}

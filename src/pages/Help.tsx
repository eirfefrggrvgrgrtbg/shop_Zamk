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
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl py-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-2">Помощь</h1>
        <p className="text-sm text-ash mb-8">Ответы на часто задаваемые вопросы</p>

        <div className="space-y-3">
          {faqData.map((item, i) => (
            <div key={i} className="bg-white rounded-2xl border border-border-lighter overflow-hidden">
              <button
                onClick={() => setOpenIndex(openIndex === i ? null : i)}
                className="w-full flex items-center justify-between px-5 py-4 text-left"
              >
                <span className="text-sm font-medium text-graphite pr-4">{item.q}</span>
                <ChevronDown className={`w-4 h-4 text-ash shrink-0 transition-transform ${openIndex === i ? 'rotate-180' : ''}`} />
              </button>
              {openIndex === i && (
                <div className="px-5 pb-4">
                  <p className="text-sm text-ash leading-relaxed">{item.a}</p>
                </div>
              )}
            </div>
          ))}
        </div>

        <div className="mt-10 bg-white rounded-3xl border border-border-lighter p-6 sm:p-8 text-center">
          <div className="w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center mx-auto mb-4">
            <HelpCircle className="w-6 h-6 text-primary" />
          </div>
          <h3 className="text-lg font-semibold text-graphite mb-2">Не нашли ответ?</h3>
          <p className="text-sm text-ash mb-4">Напишите нам, и мы ответим в течение 24 часов</p>
          <a href="mailto:help@zamk.store" className="text-sm font-medium text-primary hover:text-primary-hover transition-colors">
            help@zamk.store
          </a>
        </div>
      </div>
    </div>
  );
}

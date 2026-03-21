import { Link } from 'react-router-dom';

export function About() {
  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl py-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-8">О нас</h1>

        <div className="space-y-6 text-ash leading-relaxed">
          <p className="text-lg">
            <span className="font-serif text-graphite text-2xl">ZAMK</span> — это кураторский fashion-магазин нового поколения. Мы отбираем лучшие бренды со всего мира и создаём пространство, где каждая вещь — осознанный выбор.
          </p>

          <div className="bg-white rounded-3xl border border-border-lighter p-8">
            <h2 className="text-xl font-semibold text-graphite mb-4">Наша философия</h2>
            <p>Мы верим, что гардероб должен быть компактным, продуманным и качественным. Поэтому мы не гонимся за количеством, а тщательно выбираем каждый бренд и каждую коллекцию.</p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {[
              { number: '8+', label: 'Брендов-партнёров' },
              { number: '16+', label: 'Наименований' },
              { number: '2026', label: 'Год основания' },
            ].map(stat => (
              <div key={stat.label} className="bg-white rounded-2xl border border-border-lighter p-6 text-center">
                <p className="text-3xl font-serif text-primary mb-1">{stat.number}</p>
                <p className="text-xs text-ash">{stat.label}</p>
              </div>
            ))}
          </div>

          <div className="bg-white rounded-3xl border border-border-lighter p-8">
            <h2 className="text-xl font-semibold text-graphite mb-4">Что нас отличает</h2>
            <ul className="space-y-3">
              {[
                'Кураторский подход к отбору брендов',
                'Только оригинальные товары напрямую от производителей',
                'Бережная доставка в экологичной упаковке',
                'Профессиональные стилисты для персональных подборок',
                'Программа лояльности для постоянных клиентов',
              ].map((item, i) => (
                <li key={i} className="flex items-start gap-3">
                  <span className="w-6 h-6 rounded-full bg-primary/10 text-primary text-xs flex items-center justify-center shrink-0 mt-0.5">✓</span>
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

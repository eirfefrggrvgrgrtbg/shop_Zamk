import { InfoPanel } from '../components/editorial/StudioKit';

export function About() {
  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
        <section className="mb-12 border-b border-border-lighter pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
            Кураторская платформа
          </p>
          <div className="flex items-end justify-between">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              О витрине
            </h1>
          </div>
        </section>

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5'>
          <InfoPanel title='Наша философия'>
            Осознанный гардероб формируется не количеством, а качеством выбора. Поэтому мы работаем как кураторы, а не как массовый ритейл.
          </InfoPanel>
          <InfoPanel title='Что нас отличает'>
            Прямые контракты с брендами, прозрачное происхождение товаров, спокойный UX и единый художественный язык интерфейса.
          </InfoPanel>
        </div>

        <div className='mt-5 grid grid-cols-1 sm:grid-cols-3 gap-4'>
          {[
            { value: '8+', label: 'Брендов-партнёров' },
            { value: '16+', label: 'Кураторских позиций' },
            { value: '2026', label: 'Год запуска' },
          ].map((item) => (
            <div key={item.label} className='bg-white/62 dark:bg-white/5 border border-border-lighter dark:border-white/10 p-6 text-center rounded-2xl'>
              <p className='text-3xl font-serif text-graphite dark:text-gray-200'>{item.value}</p>
              <p className='mt-1 text-xs uppercase tracking-[0.14em] text-ash'>{item.label}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

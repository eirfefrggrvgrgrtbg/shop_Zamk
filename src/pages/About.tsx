import { InfoPanel } from '../components/editorial/StudioKit';

export function About() {
  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
        <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 dark:border-white/10 dark:bg-white/5 backdrop-blur-sm'>
          <div className='relative h-[190px] md:h-[240px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)] dark:opacity-20' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))] dark:hidden' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.2rem,6.4vw,6.8rem)] text-white/43 dark:text-white/20 leading-[0.8] tracking-[-0.03em]'>О ВИТРИНЕ</h2>
              <h3 className='font-serif text-[clamp(1.9rem,5.2vw,5.4rem)] text-white/42 dark:text-white/20 leading-[0.82] tracking-[-0.03em] text-center'>КУРАТОРСКАЯ ПЛАТФОРМА</h3>
            </div>
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

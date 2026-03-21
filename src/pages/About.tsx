import { HeroBlock, InfoPanel } from '../components/editorial/StudioKit';

export function About() {
  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[980px]'>
        <HeroBlock
          label='О проекте'
          title={<>Кураторская платформа ZAMK</>}
          description='Мы собираем независимые модные бренды в единое цифровое пространство с премиальной редакционной подачей.'
        />

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
            <div key={item.label} className='glass-panel p-6 text-center'>
              <p className='text-3xl font-serif text-graphite'>{item.value}</p>
              <p className='mt-1 text-xs uppercase tracking-[0.14em] text-ash'>{item.label}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

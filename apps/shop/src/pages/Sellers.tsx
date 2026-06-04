import { BrandCard } from '../components/editorial/StudioKit';
import { BRANDS } from '../lib/mock-data';

export function Sellers() {
  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className="mb-12 border-b border-border-lighter dark:border-white/10 pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
            Коллекция
          </p>
          <div className="flex items-end justify-between">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Продавцы: Новая волна
            </h1>
          </div>
        </section>

        <div className='mt-10 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6'>
          {BRANDS.map((brand) => (
            <BrandCard key={brand.id} brand={brand} />
          ))}
        </div>
      </div>
    </div>
  );
}

import { BrandCard } from '../components/editorial/StudioKit';
import { BRANDS } from '../lib/mock-data';

export function Brands() {
  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm'>
          <div className='relative h-[200px] md:h-[260px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.6rem,7.4vw,7.8rem)] text-white/43 leading-[0.8] tracking-[-0.03em]'>БРЕНДЫ</h2>
              <h3 className='font-serif text-[clamp(2.2rem,6.3vw,6.8rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center'>
                НОВАЯ
                <br />
                ВОЛНА
              </h3>
              <h4 className='font-serif text-[clamp(2.2rem,6.6vw,7.2rem)] text-white/42 leading-[0.8] tracking-[-0.03em] text-right'>КОЛЛЕКЦИЯ</h4>
            </div>
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

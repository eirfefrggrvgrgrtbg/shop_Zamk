import { BrandCard, HeroBlock } from '../components/editorial/StudioKit';
import { BRANDS } from '../lib/mock-data';

export function Brands() {
  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px]'>
        <HeroBlock
          label='Бренды'
          title={
            <>
              Цифровая витрина
              <br />
              независимых марок
            </>
          }
          description='Кураторская выборка брендов с собственной философией и ясным дизайнерским кодом.'
          right={
            <div className='glass-panel p-7'>
              <p className='studio-label'>Партнёры ZAMK</p>
              <p className='mt-3 text-4xl font-serif text-graphite'>{BRANDS.length}</p>
              <p className='mt-2 text-sm text-graphite-light'>брендов в текущем архиве</p>
            </div>
          }
        />

        <div className='mt-10 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6'>
          {BRANDS.map((brand) => (
            <BrandCard key={brand.id} brand={brand} />
          ))}
        </div>
      </div>
    </div>
  );
}

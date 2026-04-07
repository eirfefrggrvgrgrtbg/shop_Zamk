import { Link, useParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { ProductCard } from '../components/product/ProductCard';
import { InfoPanel, SectionHeader } from '../components/editorial/StudioKit';
import { getBrandById, getProductsByBrand } from '../lib/mock-data';

export function BrandDetail() {
  const { id } = useParams<{ id: string }>();
  const brand = getBrandById(id || '');
  const products = getProductsByBrand(id || '');

  if (!brand) {
    return (
      <div className='relative z-10 min-h-screen pt-36'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite dark:text-white'>Бренд не найден</h1>
          <Link to='/brands' className='inline-block mt-6'>
            <Button>К брендам</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <Link to='/brands' className='inline-flex items-center gap-2 text-sm text-ash dark:text-white/60 hover:text-graphite dark:hover:text-white'>
          <ArrowLeft className='w-4 h-4' /> Все бренды
        </Link>

        <section className='mt-6 overflow-hidden rounded-[0.8rem] border border-white/45 dark:border-white/10 bg-white/16 dark:bg-white/5 backdrop-blur-sm'>
          <div className='relative h-[210px] md:h-[270px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)] dark:opacity-20' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))] dark:hidden' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.2rem,6.2vw,6.4rem)] text-white/43 dark:text-white/30 leading-[0.8] tracking-[-0.03em]'>{brand.country}</h2>
              <h3 className='font-serif text-[clamp(2.1rem,5.9vw,6.2rem)] text-white/42 dark:text-white/30 leading-[0.82] tracking-[-0.03em] text-center'>{brand.name}</h3>
              <div className='h-[124px] w-[96px] md:h-[154px] md:w-[118px] overflow-hidden border border-border-lighter dark:border-white/20 bg-white/50 dark:bg-black/30'>
                <img src={brand.image} alt={brand.name} className='h-full w-full object-cover grayscale' />
              </div>
            </div>
          </div>
        </section>

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5'>
          <InfoPanel title='История бренда'>
            {brand.history || brand.description}
          </InfoPanel>
          <InfoPanel title='Философия'>
            {brand.philosophy || `Проект из ${brand.origin || brand.country}, исследующий формы и материалы. Акцент на архитектурный крой, тактильные материалы и функциональность.`}
          </InfoPanel>
        </div>

        <section className='mt-12'>
          <SectionHeader
            label='Товары бренда'
            title={`${brand.name} в каталоге`}
            description={`Доступно ${products.length} позиций`}
          />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {products.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

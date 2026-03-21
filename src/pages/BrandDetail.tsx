import { Link, useParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { ProductCard } from '../components/product/ProductCard';
import { HeroBlock, InfoPanel, SectionHeader } from '../components/editorial/StudioKit';
import { getBrandById, getProductsByBrand } from '../lib/mock-data';

export function BrandDetail() {
  const { id } = useParams<{ id: string }>();
  const brand = getBrandById(id || '');
  const products = getProductsByBrand(id || '');

  if (!brand) {
    return (
      <div className='relative z-10 min-h-screen pt-36'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite'>Бренд не найден</h1>
          <Link to='/brands' className='inline-block mt-6'>
            <Button>К брендам</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px]'>
        <Link to='/brands' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite'>
          <ArrowLeft className='w-4 h-4' /> Все бренды
        </Link>

        <div className='mt-6'>
          <HeroBlock
            label={brand.country}
            title={<>{brand.name}</>}
            description={brand.description}
            right={
              <div className='rounded-[2.1rem] border border-border-lighter bg-white/85 p-2'>
                <div className='overflow-hidden rounded-[1.7rem] aspect-[4/5]'>
                  <img src={brand.image} alt={brand.name} className='w-full h-full object-cover' />
                </div>
              </div>
            }
          />
        </div>

        <div className='mt-8 grid grid-cols-1 lg:grid-cols-2 gap-5'>
          <InfoPanel title='Философия бренда'>
            Акцент на архитектурный крой, тактильные материалы и функциональность без визуального шума.
          </InfoPanel>
          <InfoPanel title='Кураторский взгляд'>
            Бренд отобран за цельность эстетики, качество производства и способность формировать личный стиль.
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

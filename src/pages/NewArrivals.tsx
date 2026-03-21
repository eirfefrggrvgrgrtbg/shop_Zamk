import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { PRODUCTS } from '../lib/mock-data';
import { HeroBlock, SectionHeader } from '../components/editorial/StudioKit';

export function NewArrivals() {
  const items = PRODUCTS.filter((product) => product.isNew);

  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px]'>
        <HeroBlock
          label='Новая поставка'
          title={
            <>
              Новинки сезона
              <br />
              в эстетике ZAMK
            </>
          }
          description='Свежие релизы независимых брендов, собранные в цельную редакционную подачу.'
          primaryCta={{ label: 'Полный каталог', to: '/catalog' }}
          secondaryCta={{ label: 'Бренды платформы', to: '/brands' }}
        />

        <section className='mt-10'>
          <SectionHeader
            label='Архив'
            title='Новые релизы'
            description='Чистые карточки и большое количество воздуха вместо перегруженной витрины.'
          />

          {items.length > 0 ? (
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
              {items.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          ) : (
            <div className='glass-panel-strong p-12 text-center'>
              <h3 className='text-3xl font-serif text-graphite'>Скоро новая поставка</h3>
              <p className='mt-3 text-sm text-graphite-light'>Команда ZAMK уже формирует следующий дроп.</p>
              <Link to='/catalog' className='inline-block mt-6'>
                <Button className='gap-2'>
                  Перейти в каталог <ArrowRight className='w-4 h-4' />
                </Button>
              </Link>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import {
  BrandCard,
  CategoryCard,
  FloatingBadge,
  HeroBlock,
  SectionHeader,
} from '../components/editorial/StudioKit';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { BRANDS, CATEGORIES, COLLECTIONS, PRODUCTS, getNewProducts } from '../lib/mock-data';

const reveal = {
  initial: { opacity: 0, y: 20 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.75, ease: [0.16, 1, 0.3, 1] as const },
};

export function Home() {
  const newProducts = getNewProducts();
  const editorPick = PRODUCTS.slice(0, 4);

  return (
    <div className='relative z-10 min-h-screen pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1280px] space-y-16'>
        <HeroBlock
          label='Кураторская платформа ZAMK'
          title={
            <>
              Архив новой
              <br />
              волны <span className='text-accent'>стиля</span>
            </>
          }
          description='Цифровая витрина независимых брендов в эстетике showroom: молочный свет, мягкая композиция, отборные капсулы и спокойный премиальный ритм.'
          primaryCta={{ label: 'Перейти в каталог', to: '/catalog' }}
          secondaryCta={{ label: 'Смотреть новинки', to: '/new' }}
          right={
            <div className='relative'>
              <div className='relative rounded-[2.3rem] border border-border-lighter bg-white/85 p-2 shadow-cloud'>
                <div className='overflow-hidden rounded-[1.8rem] aspect-[4/5]'>
                  <img
                    src='https://images.unsplash.com/photo-1496747611176-843222e1e57c?w=1200&auto=format'
                    alt='Модный лукбук'
                    className='w-full h-full object-cover'
                  />
                </div>
              </div>
              <div className='absolute -left-4 top-8'>
                <FloatingBadge>Новая поставка</FloatingBadge>
              </div>
              <div className='absolute -right-4 bottom-8'>
                <FloatingBadge>Студийный свет</FloatingBadge>
              </div>
            </div>
          }
        />

        <motion.section {...reveal}>
          <SectionHeader
            label='Новые бренды'
            title='Новая волна партнёров'
            description='Выбранные бренды, которые формируют современный независимый язык моды.'
            action={
              <Link to='/brands'>
                <Button variant='secondary' className='gap-2'>
                  Смотреть все <ArrowRight className='w-4 h-4' />
                </Button>
              </Link>
            }
          />
          <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6'>
            {BRANDS.slice(0, 3).map((brand) => (
              <BrandCard key={brand.id} brand={brand} />
            ))}
          </div>
        </motion.section>

        <motion.section {...reveal}>
          <SectionHeader
            label='Капсульные подборки'
            title='Кураторские наборы'
            description='Большие тематические блоки вместо обычной витрины: собранные образы и цельные истории.'
          />
          <div className='grid grid-cols-1 lg:grid-cols-3 gap-6'>
            {COLLECTIONS.map((collection) => (
              <Link
                key={collection.id}
                to={`/catalog?collection=${collection.id}`}
                className='group block rounded-[2.2rem] border border-border-lighter bg-white/86 p-2 shadow-sm transition-all hover:-translate-y-1 hover:shadow-cloud'
              >
                <div className='relative overflow-hidden rounded-[1.7rem] aspect-[4/3]'>
                  <img src={collection.image} alt={collection.title} className='w-full h-full object-cover transition-transform duration-700 group-hover:scale-105' />
                  <div className='absolute inset-0 bg-gradient-to-t from-graphite/45 to-transparent' />
                  <div className='absolute left-5 right-5 bottom-5 text-white'>
                    <p className='text-xs uppercase tracking-[0.16em] text-white/80'>{collection.subtitle}</p>
                    <h3 className='mt-2 text-2xl font-serif'>{collection.title}</h3>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </motion.section>

        <motion.section {...reveal}>
          <SectionHeader
            label='Новая поставка'
            title='Свежие поступления'
            description='Светлая продуктовая система: rounded image block, деликатные бейджи и спокойная типографика.'
          />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {newProducts.slice(0, 4).map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </motion.section>

        <motion.section {...reveal} className='glass-panel p-8 md:p-10'>
          <SectionHeader
            label='Выбор редакции'
            title='Собрано редакцией ZAMK'
            description='Вещи с ясным архитектурным кроем и выразительным, но спокойным характером.'
          />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {editorPick.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </motion.section>

        <motion.section {...reveal}>
          <SectionHeader
            label='Категории'
            title='Навигация по архиву'
            description='Большие блоки категорий в единой editorial-системе.'
          />
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6'>
            {CATEGORIES.filter((c) => c.id !== 'all').map((category) => (
              <CategoryCard key={category.id} category={category} />
            ))}
          </div>
        </motion.section>
      </div>
    </div>
  );
}

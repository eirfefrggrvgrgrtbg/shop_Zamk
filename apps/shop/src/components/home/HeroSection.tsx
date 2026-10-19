import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { ArrowRight } from 'lucide-react';
import { QuestionHero3D } from '../hero/QuestionHero3D';
import { HeroShadow } from '../hero/HeroShadow';
import type { Product } from '../../types/catalog';

interface HeroSectionProps {
  /** Real storefront products with photos. Without them the brand mark is shown instead. */
  products?: Product[];
}

function HeroPhoto({ product, className, priority }: { product: Product; className: string; priority?: boolean }) {
  return (
    <Link
      to={`/product/${product.id}`}
      data-testid="hero-photo"
      className={`group relative block overflow-hidden rounded-[1.6rem] bg-ice dark:bg-white/5 ${className}`}
    >
      <img
        src={product.image || product.images?.[0]?.url}
        alt={product.name}
        loading={priority ? 'eager' : 'lazy'}
        className="h-full w-full object-cover transition-transform duration-[1200ms] ease-out group-hover:scale-105"
      />
      <span className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent px-4 pb-3 pt-10 text-white">
        <span className="block truncate text-[10px] font-semibold uppercase tracking-[0.16em] text-white/75">{product.brand}</span>
        <span className="block truncate font-serif text-lg leading-tight">{product.name}</span>
      </span>
    </Link>
  );
}

export const HeroSection = ({ products = [] }: HeroSectionProps) => {
  const [main, second, third] = products;

  return (
    <section className="relative overflow-hidden pt-24 pb-10 md:pt-36 md:pb-16 lg:pt-40 lg:pb-20">
      <div className="container mx-auto px-4 lg:px-8 relative z-10">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-12 items-center">
          <motion.div
            initial={{ opacity: 0, y: 32 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, ease: 'easeOut' }}
            className="lg:col-span-6 z-20"
          >
            <p className="studio-label mb-4 md:mb-5">ZAMK · Локальные студии</p>
            <h1 className="font-serif text-[2.1rem] sm:text-6xl lg:text-[4.6rem] leading-[1.02] tracking-tight text-graphite dark:text-white">
              Мода, которую создают в&nbsp;студиях, а не&nbsp;на&nbsp;конвейере
            </h1>
            <p className="mt-4 md:mt-6 max-w-md text-sm md:text-base text-graphite-light dark:text-white/70 leading-relaxed">
              Одежда независимых брендов и дизайнеров. Каждая вещь отобрана и проходит модерацию ZAMK.
            </p>
            <div className="mt-6 md:mt-9 flex gap-2.5 md:gap-3">
              <Link
                to="/new"
                className="inline-flex items-center gap-2 rounded-full bg-graphite px-5 py-3.5 text-sm md:px-8 md:py-4 font-medium text-white transition-transform hover:scale-[1.03] active:scale-95 dark:bg-white dark:text-black"
              >
                Новые поступления <ArrowRight className="h-4 w-4" />
              </Link>
              <Link
                to="/catalog"
                className="inline-flex items-center rounded-full border border-graphite/25 px-5 py-3.5 text-sm md:px-8 md:py-4 font-medium text-graphite transition-colors hover:border-graphite dark:border-white/25 dark:text-white dark:hover:border-white"
              >
                Каталог
              </Link>
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, scale: 0.97 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 1, delay: 0.2, ease: 'easeOut' }}
            className="lg:col-span-6"
          >
            {main ? (
              <div className="mx-auto grid max-w-[560px] grid-cols-5 gap-3 md:gap-4 lg:max-w-none">
                <HeroPhoto
                  product={main}
                  priority
                  className={second ? 'col-span-3 aspect-[3/4.2]' : 'col-span-5 aspect-[4/3] md:aspect-[3/2]'}
                />
                {second && (
                  <div className="col-span-2 flex flex-col gap-3 md:gap-4 pt-8 md:pt-12">
                    <HeroPhoto product={second} className="aspect-[3/4]" />
                    {third && <HeroPhoto product={third} className="hidden aspect-[3/4] sm:block" />}
                  </div>
                )}
              </div>
            ) : (
              <div className="relative mx-auto aspect-square w-full max-w-[420px] lg:max-w-[520px]">
                <div className="absolute inset-0" style={{ zIndex: 2 }}>
                  <QuestionHero3D />
                </div>
                <HeroShadow />
              </div>
            )}
          </motion.div>
        </div>
      </div>
    </section>
  );
};

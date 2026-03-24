import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import { BrandCard, CategoryCard, SectionHeader } from '../components/editorial/StudioKit';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { BRANDS, CATEGORIES, COLLECTIONS, PRODUCTS, getNewProducts } from '../lib/mock-data';
import { HeroSection } from '../components/home/HeroSection';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

export function Home() {
  const newProducts = getNewProducts();
  const editorPick = PRODUCTS.slice(0, 4);

  return (
    <div className="relative z-10 min-h-screen pb-20">

      {/* ═══════════════════════════════════════════════════════
          HERO SECTION — Minimalist design with typography art
      ═══════════════════════════════════════════════════════ */}
      <HeroSection />

      {/* ═══════════════════════════════════════════════════════
          CONTENT SECTIONS
      ═══════════════════════════════════════════════════════ */}
      <div className="max-w-[1400px] mx-auto px-4 sm:px-6 flex flex-col gap-16 md:gap-20 pt-2">

        {/* New partners */}
        <motion.section {...reveal}>
          <SectionHeader
            label="Партнёры"
            title="Бренды"
            action={
              <Link to="/brands">
                <Button variant="secondary" className="gap-2">
                  Смотреть все <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
            }
          />
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {BRANDS.slice(0, 3).map((brand) => (
              <BrandCard key={brand.id} brand={brand} />
            ))}
          </div>
        </motion.section>

        {/* Collections */}
        <motion.section {...reveal}>
          <SectionHeader
            label="Подборки"
            title="Кураторские наборы"
          />
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
            {COLLECTIONS.map((collection) => (
              <Link
                key={collection.id}
                to={`/catalog?collection=${collection.id}`}
                className="group block rounded-[2rem] border border-border-lighter dark:border-white/10 bg-white/86 dark:bg-white/5 p-2 shadow-sm transition-all hover:-translate-y-1 hover:shadow-[0_16px_40px_rgba(120,150,185,0.12)] dark:hover:shadow-[0_16px_40px_rgba(0,0,0,0.5)]"
              >
                <div className="relative overflow-hidden rounded-[1.5rem] aspect-[4/3]">
                  <img
                    src={collection.image}
                    alt={collection.title}
                    className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-105"
                  />
                  <div className="absolute inset-0 bg-gradient-to-t from-graphite/50 to-transparent" />
                  <div className="absolute left-5 right-5 bottom-5 text-white">
                    <p className="text-[10px] uppercase tracking-[0.16em] text-white/75">{collection.subtitle}</p>
                    <h3 className="mt-1.5 text-[22px] font-serif">{collection.title}</h3>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </motion.section>

        {/* New arrivals */}
        <motion.section {...reveal}>
          <SectionHeader
            label="Новинки"
            title="Свежие поступления"
          />
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
            {newProducts.slice(0, 4).map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </motion.section>

        {/* Editor picks */}
        <motion.section {...reveal} className="glass-panel p-7 md:p-10">
          <SectionHeader
            label="Выбор редакции"
            title="Собрано редакцией"
          />
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
            {editorPick.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </motion.section>

        {/* Categories */}
        <motion.section {...reveal}>
          <SectionHeader
            label="Каталог"
            title="Категории"
          />
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
            {CATEGORIES.filter((c) => c.id !== 'all').map((category) => (
              <CategoryCard key={category.id} category={category} />
            ))}
          </div>
        </motion.section>
      </div>
    </div>
  );
}

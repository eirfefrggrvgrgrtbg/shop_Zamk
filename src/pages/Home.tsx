import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import { BrandCard, CategoryCard, SectionHeader } from '../components/editorial/StudioKit';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { BRANDS, CATEGORIES, COLLECTIONS, PRODUCTS, getNewProducts } from '../lib/mock-data';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

const TABS = ['Все', 'Платья', 'Топы', 'Аксессуары'] as const;
type Tab = typeof TABS[number];

export function Home() {
  const newProducts = getNewProducts();
  const editorPick = PRODUCTS.slice(0, 4);
  const topProducts = newProducts.length >= 4 ? newProducts.slice(0, 4) : PRODUCTS.slice(0, 4);
  const [activeTab, setActiveTab] = useState<Tab>('Все');

  return (
    <div className="relative z-10 min-h-screen pb-20">

      {/* ═══════════════════════════════════════════════════════
          HERO SCENE — Unified first-screen composition
      ═══════════════════════════════════════════════════════ */}
      <section className="relative w-full overflow-hidden pt-[88px] md:pt-[96px]">

        {/* Ghost typography layer — atmospheric, not dominant */}
        <div
          aria-hidden="true"
          className="absolute inset-0 pointer-events-none select-none overflow-hidden z-0"
        >
          {/* Horizontal ghost words fading across the background */}
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center justify-center gap-[6vw] whitespace-nowrap">
            <span
              className="font-serif text-[clamp(3.5rem,7vw,9rem)] text-[#8aa4c0] leading-none tracking-[-0.01em]"
              style={{ opacity: 0.13, filter: 'blur(1.5px)' }}
            >
              АРХИВ
            </span>
            <span
              className="font-serif text-[clamp(3rem,5.5vw,7rem)] text-[#8aa4c0] leading-none tracking-[-0.01em]"
              style={{ opacity: 0.08, filter: 'blur(2px)' }}
            >
              ВОЛНА
            </span>
            <span
              className="font-serif text-[clamp(3.5rem,7vw,9rem)] text-[#8aa4c0] leading-none tracking-[-0.01em]"
              style={{ opacity: 0.11, filter: 'blur(1.5px)' }}
            >
              ЗАМК
            </span>
          </div>
          {/* Subtle vertical word */}
          <div
            className="absolute right-[6%] top-[40%] font-serif text-[clamp(2.5rem,4vw,6rem)] text-[#7a9ab8] leading-none tracking-[0.12em]"
            style={{ opacity: 0.07, filter: 'blur(2px)', writingMode: 'vertical-rl' }}
          >
            СТИЛЬ
          </div>
        </div>

        {/* Hero content shell */}
        <div className="max-w-[1400px] mx-auto px-4 sm:px-6 pt-8 pb-0 relative z-10">

          {/* Hero headline — tight & editorial */}
          <div className="mb-8 md:mb-10 flex flex-col gap-1">
            <p className="text-[11px] font-semibold tracking-[0.18em] text-ash uppercase">
              Кураторская платформа
            </p>
            <h1 className="font-serif text-[clamp(2.4rem,5.5vw,4.2rem)] text-graphite leading-[0.95] tracking-tight">
              Архив новой волны
            </h1>
          </div>

          {/* ── Tabs + control row ── */}
          <div className="relative mb-7 md:mb-9">
            {/* Tab row */}
            <div className="flex items-center justify-between gap-4 flex-wrap">
              <div className="flex items-center gap-1">
                {TABS.map((tab) => (
                  <button
                    key={tab}
                    onClick={() => setActiveTab(tab)}
                    className={`text-[13px] font-medium px-4 py-2 rounded-full transition-all duration-300 tracking-[0.01em] ${
                      activeTab === tab
                        ? 'bg-graphite text-white shadow-[0_4px_14px_rgba(28,39,51,0.18)]'
                        : 'text-graphite/55 hover:text-graphite hover:bg-white/60'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>

              {/* New arrivals pill — part of the control row, not floating */}
              <Link
                to="/new"
                className="flex items-center gap-1.5 rounded-full border border-primary/20 bg-primary/8 px-4 py-2 text-[12.5px] font-medium text-primary hover:bg-primary hover:text-white hover:border-primary shadow-[0_3px_12px_rgba(152,90,215,0.12)] transition-all duration-300 tracking-[0.02em]"
              >
                <span className="w-1.5 h-1.5 rounded-full bg-primary flex-shrink-0" />
                Новинки
              </Link>
            </div>

            {/* Separator — subtle rule connecting tabs to products */}
            <div className="mt-5 h-px bg-gradient-to-r from-graphite/8 via-graphite/14 to-transparent" />
          </div>

          {/* ── Product grid intro label ── */}
          <div className="flex items-center gap-3 mb-5">
            <div className="h-px w-6 bg-graphite/20" />
            <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase">
              Актуальное
            </p>
          </div>

          {/* ── Hero product grid ── */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4 pb-14 md:pb-18">
            {topProducts.map((product, i) => (
              <motion.div
                key={product.id}
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.65, delay: i * 0.08, ease: [0.16, 1, 0.3, 1] }}
              >
                <ProductCard product={product} />
              </motion.div>
            ))}
          </div>
        </div>

        {/* Bottom fade-out edge — blends hero into content below */}
        <div className="absolute bottom-0 left-0 right-0 h-20 bg-gradient-to-t from-ice/80 to-transparent pointer-events-none z-10" />
      </section>

      {/* ═══════════════════════════════════════════════════════
          CONTENT SECTIONS
      ═══════════════════════════════════════════════════════ */}
      <div className="max-w-[1400px] mx-auto px-4 sm:px-6 flex flex-col gap-16 md:gap-20 pt-2">

        {/* New partners */}
        <motion.section {...reveal}>
          <SectionHeader
            label="Новые бренды"
            title="Новая волна партнёров"
            description="Выбранные бренды, которые формируют современный независимый язык моды."
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
            label="Капсульные подборки"
            title="Кураторские наборы"
            description="Большие тематические блоки вместо обычной витрины: собранные образы и цельные истории."
          />
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
            {COLLECTIONS.map((collection) => (
              <Link
                key={collection.id}
                to={`/catalog?collection=${collection.id}`}
                className="group block rounded-[2rem] border border-border-lighter bg-white/86 p-2 shadow-sm transition-all hover:-translate-y-1 hover:shadow-[0_16px_40px_rgba(120,150,185,0.12)]"
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
            label="Новая поставка"
            title="Свежие поступления"
            description="Светлая продуктовая система: rounded image block, деликатные бейджи и спокойная типографика."
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
            title="Собрано редакцией ZAMK"
            description="Вещи с ясным архитектурным кроем и выразительным, но спокойным характером."
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
            label="Категории"
            title="Навигация по архиву"
            description="Большие блоки категорий в единой editorial-системе."
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

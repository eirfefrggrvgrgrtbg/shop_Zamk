import { Link } from 'react-router-dom';
import { ArrowRight, Sparkles } from 'lucide-react';
import { motion } from 'framer-motion';
import { EditorialHero } from '../components/editorial/EditorialHero';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { PRODUCTS, BRANDS, CATEGORIES, COLLECTIONS, getNewProducts } from '../lib/mock-data';

const fadeInUp = {
  initial: { opacity: 0, y: 20 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-50px' },
  transition: { duration: 0.6, ease: [0.16, 1, 0.3, 1] },
};

export function Home() {
  const newProducts = getNewProducts();
  const editorPicks = PRODUCTS.slice(0, 4);
  const bestsellers = PRODUCTS.filter(p => p.isBestseller);

  return (
    <div className="flex flex-col w-full">
      <EditorialHero />

      {/* ─── Categories Pills ─── */}
      <section className="py-8 bg-white border-b border-border-lighter">
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <div className="flex gap-2.5 overflow-x-auto pb-2 scrollbar-hide">
            {CATEGORIES.filter(c => c.id !== 'all').map(cat => (
              <Link key={cat.id} to={`/catalog?category=${cat.id}`}>
                <Button variant="pill" size="sm" className="shrink-0 gap-1.5">
                  <span>{cat.icon}</span>
                  <span>{cat.name}</span>
                </Button>
              </Link>
            ))}
          </div>
        </div>
      </section>

      {/* ─── New Brands Banner ─── */}
      <motion.section className="py-16 sm:py-20 bg-white" {...fadeInUp}>
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <div className="relative rounded-3xl overflow-hidden bg-gradient-to-r from-primary/10 via-ice to-primary-soft p-8 sm:p-12">
            <div className="relative z-10 max-w-md">
              <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary-hover uppercase tracking-wider mb-3">
                <Sparkles className="w-3.5 h-3.5" /> Новые бренды недели
              </span>
              <h2 className="text-2xl sm:text-3xl font-serif text-graphite mb-3 leading-tight">
                5 главных коллекций, которые стоит попробовать уже сейчас
              </h2>
              <p className="text-sm text-ash mb-6 leading-relaxed">
                Обновление каталога каждую неделю. Только проверенные бренды с уникальным характером.
              </p>
              <Link to="/brands">
                <Button variant="primary" className="gap-2">
                  Смотреть
                  <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
            </div>
            {/* Decorative blobs */}
            <div className="absolute top-0 right-0 w-48 h-48 rounded-full bg-primary/5 -translate-y-1/4 translate-x-1/4 blur-2xl" />
            <div className="absolute bottom-0 right-1/4 w-32 h-32 rounded-full bg-primary-light/20 translate-y-1/4 blur-xl" />
          </div>
        </div>
      </motion.section>

      {/* ─── New Arrivals ─── */}
      <motion.section className="py-16 sm:py-20 bg-milk" {...fadeInUp}>
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-10 gap-4">
            <div>
              <h2 className="text-2xl sm:text-3xl font-serif text-graphite mb-2">Новинки</h2>
              <p className="text-sm text-ash">Свежие поступления этой недели</p>
            </div>
            <Link to="/catalog?filter=new">
              <Button variant="outline" size="sm" className="gap-2 group">
                Все новинки
                <ArrowRight className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" />
              </Button>
            </Link>
          </div>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6">
            {newProducts.slice(0, 4).map(product => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </div>
      </motion.section>

      {/* ─── Brand Marquee ─── */}
      <section className="py-14 bg-white border-y border-border-lighter overflow-hidden">
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <p className="text-[10px] font-bold tracking-[0.2em] text-center text-ash uppercase mb-8">
            Избранные бренды
          </p>
          <div className="flex flex-wrap justify-center gap-x-10 gap-y-6 md:gap-x-16">
            {BRANDS.map(brand => (
              <Link
                key={brand.id}
                to={`/brand/${brand.id}`}
                className="text-lg md:text-xl font-serif text-graphite/50 tracking-wide hover:text-primary transition-colors"
              >
                {brand.name}
              </Link>
            ))}
          </div>
        </div>
      </section>

      {/* ─── Collections ─── */}
      <motion.section className="py-16 sm:py-20 bg-milk" {...fadeInUp}>
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <h2 className="text-2xl sm:text-3xl font-serif text-graphite mb-10">Подборки</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
            {COLLECTIONS.map(col => (
              <Link key={col.id} to={`/catalog`} className="group relative block rounded-3xl overflow-hidden aspect-[3/4]">
                <img src={col.image} alt={col.title} className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-105" />
                <div className="absolute inset-0 bg-gradient-to-t from-black/50 via-black/10 to-transparent" />
                <div className="absolute bottom-0 left-0 right-0 p-6">
                  <h3 className="text-xl font-serif text-white mb-1">{col.title}</h3>
                  <p className="text-sm text-white/70">{col.subtitle}</p>
                </div>
              </Link>
            ))}
          </div>
        </div>
      </motion.section>

      {/* ─── Editor's Choice ─── */}
      <motion.section className="py-16 sm:py-20 bg-white" {...fadeInUp}>
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
          <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-10 gap-4">
            <div>
              <h2 className="text-2xl sm:text-3xl font-serif text-graphite mb-2">Выбор редакции</h2>
              <p className="text-sm text-ash">Наши стилисты рекомендуют</p>
            </div>
            <Link to="/catalog">
              <Button variant="outline" size="sm" className="gap-2 group">
                Смотреть все
                <ArrowRight className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" />
              </Button>
            </Link>
          </div>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6">
            {editorPicks.map(product => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </div>
      </motion.section>

      {/* ─── Editorial Story Block ─── */}
      <motion.section className="py-16 sm:py-24 bg-milk" {...fadeInUp}>
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl grid grid-cols-1 md:grid-cols-2 gap-12 items-center">
          <div className="relative aspect-[3/4] rounded-3xl overflow-hidden">
            <img
              src="https://images.unsplash.com/photo-1550614000-4b95d4158223?q=80&w=2000&auto=format&fit=crop"
              alt="Editorial"
              className="w-full h-full object-cover"
            />
            {/* Editorial overlay */}
            <div className="absolute inset-0 bg-gradient-to-t from-black/30 to-transparent" />
            <div className="absolute bottom-6 left-6 right-6">
              <p className="text-5xl font-serif text-white/90 leading-none">Искусство<br />сдержанности.</p>
            </div>
          </div>
          <div className="max-w-md">
            <span className="text-xs font-semibold tracking-[0.15em] text-primary-hover uppercase mb-3 block">Эдиториал</span>
            <h2 className="text-3xl sm:text-4xl font-serif text-graphite mb-5 leading-tight">
              Когда меньше — значит больше
            </h2>
            <p className="text-ash leading-relaxed mb-8">
              Настоящая роскошь шепчет. Она — в идеальной драпировке шёлка, в архитектурной точности пальто оверсайз и в тихой уверенности минималистичного дизайна. Наш новый эдиториал о вещах переходного сезона.
            </p>
            <Link to="/catalog">
              <Button variant="link" className="text-graphite font-medium gap-2 group">
                Читать эдиториал
                <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-1" />
              </Button>
            </Link>
          </div>
        </div>
      </motion.section>

      {/* ─── Bestsellers ─── */}
      {bestsellers.length > 0 && (
        <motion.section className="py-16 sm:py-20 bg-white" {...fadeInUp}>
          <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
            <h2 className="text-2xl sm:text-3xl font-serif text-graphite mb-10">Популярное</h2>
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6">
              {bestsellers.map(product => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          </div>
        </motion.section>
      )}
    </div>
  );
}

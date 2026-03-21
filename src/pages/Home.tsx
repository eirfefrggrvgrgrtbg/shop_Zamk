import { Link } from 'react-router-dom';
import { ArrowRight, Play, Sparkles } from 'lucide-react';
import { motion } from 'framer-motion';
import { EditorialHero } from '../components/editorial/EditorialHero';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { BRANDS, CATEGORIES, COLLECTIONS, PRODUCTS, getNewProducts } from '../lib/mock-data';

const fadeInUp = {
  initial: { opacity: 0, y: 20 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-40px' },
  transition: { duration: 0.6, ease: [0.16, 1, 0.3, 1] as const },
};

export function Home() {
  const newProducts = getNewProducts();
  const editorPicks = PRODUCTS.slice(0, 4);
  const bestsellers = PRODUCTS.filter((product) => product.isBestseller).slice(0, 4);

  return (
    <div className="flex flex-col gap-24 md:gap-32 w-full pt-16">
      <EditorialHero />

      <section className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="flex z-[9999] overflow-x-auto scrollbar-hide gap-3 pb-4">
          {CATEGORIES.map((category) => (
            <Link
              key={category.id}
              to={`/catalog?category=${category.slug}`}
              className="flex-shrink-0 group flex items-center gap-2 glass-panel px-6 py-3.5 rounded-full hover:bg-white/70 hover:-translate-y-1 transition-all duration-300"
            >
              <span className="text-base group-hover:scale-110 transition-transform duration-300">{category.icon}</span>
              <span className="text-sm font-medium text-graphite group-hover:text-primary transition-colors">{category.name}</span>
            </Link>
          ))}
        </div>
      </section>

      <motion.section {...fadeInUp} className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="capsule p-8 sm:p-12 md:p-16 flex flex-col md:flex-row items-center justify-between gap-8 border-primary/20">
          <div className="max-w-xl">
            <div className="flex items-center gap-2 text-primary font-bold text-xs tracking-widest uppercase mb-4">
              <Sparkles className="w-4 h-4" />
              <span>Кураторская подборка недели</span>
            </div>
            <h2 className="text-3xl sm:text-4xl md:text-5xl font-serif text-graphite mb-4 leading-tight">
              5 нишевых брендов,
              <br />
              которые формируют сцену.
            </h2>
            <p className="text-graphite-light text-sm md:text-base leading-relaxed mb-8">
              Еженедельно собираем вещи с выразительным кроем, цифровой фактурой и холодным дизайнерским настроением.
            </p>
            <Button variant="primary" size="lg" className="w-full sm:w-auto shadow-primary/30">
              Смотреть подборку <ArrowRight className="w-4 h-4 ml-2" />
            </Button>
          </div>
          <div className="hidden md:block w-full max-w-sm rounded-[2rem] overflow-hidden shadow-2xl rotate-3 hover:rotate-0 transition-transform duration-500 border-4 border-white/50">
            <img
              src="https://images.unsplash.com/photo-1544441893-675973e31985?auto=format&fit=crop&q=80"
              alt="Капсульный стиль"
              className="w-full aspect-[3/4] object-cover"
            />
          </div>
        </div>
      </motion.section>

      <motion.section {...fadeInUp} className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="flex items-end justify-between mb-8">
          <div>
            <h2 className="text-3xl font-serif text-graphite mb-2">Новая поставка</h2>
            <p className="text-sm text-ash">Свежие поступления и лимитированные релизы</p>
          </div>
          <Button variant="ghost" className="hidden sm:flex">
            Все хиты <ArrowRight className="w-4 h-4 ml-2" />
          </Button>
        </div>

        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
          {newProducts.map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
        <Button variant="secondary" className="w-full mt-6 sm:hidden">
          Смотреть все новинки
        </Button>
      </motion.section>

      <section className="relative z-10 py-12 glass-panel border-x-0 !rounded-none shadow-none">
        <div className="container mx-auto px-4">
          <p className="text-center text-xs font-bold text-ash uppercase tracking-widest mb-8">Бренды цифровой витрины</p>
          <div className="flex flex-wrap justify-center items-center gap-x-12 gap-y-8 opacity-70">
            {BRANDS.map((brand) => (
              <Link key={brand.id} to={`/brand/${brand.id}`} className="hover:opacity-100 hover:scale-105 transition-all grayscale hover:grayscale-0">
                <span className="font-serif font-bold text-xl md:text-2xl text-graphite">{brand.name}</span>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <motion.section {...fadeInUp} className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-serif text-graphite mb-2">Кураторские подборки</h2>
          <p className="text-sm text-ash">Асимметричные капсулы и редакторские темы</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 auto-rows-[300px] md:auto-rows-[400px]">
          {COLLECTIONS.map((collection, index) => (
            <Link
              key={collection.id}
              to={`/catalog?collection=${collection.id}`}
              className={`group capsule overflow-hidden block ${index === 0 ? 'md:col-span-2' : ''}`}
            >
              <div className="absolute inset-0">
                <img
                  src={collection.image}
                  alt={collection.title}
                  className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-105"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-graphite/80 via-graphite/20 to-transparent opacity-85 group-hover:opacity-90 transition-opacity" />
              </div>
              <div className="absolute bottom-0 left-0 p-8 w-full">
                <span className="inline-block px-3 py-1 bg-white/20 backdrop-blur-md rounded-full text-white text-[10px] font-bold uppercase tracking-widest mb-3">
                  {collection.itemCount} товаров
                </span>
                <h3 className="text-2xl font-serif font-medium text-white mb-2">{collection.title}</h3>
                <p className="text-white/80 text-sm line-clamp-2 max-w-md">{collection.description}</p>
              </div>
            </Link>
          ))}
        </div>
      </motion.section>

      <motion.section {...fadeInUp} className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="flex flex-col md:flex-row items-center gap-12">
          <div className="md:w-1/3 w-full">
            <h2 className="text-3xl font-serif text-graphite mb-4">Выбор куратора</h2>
            <p className="text-ash mb-8 leading-relaxed text-sm">
              Вещи с особым характером: деконструкция, объем, холодные фактуры и независимый дизайнерский язык.
            </p>
            <Button variant="outline" className="w-full md:w-auto">
              Изучить селекцию
            </Button>
          </div>
          <div className="md:w-2/3 w-full grid grid-cols-2 gap-4 sm:gap-6">
            {editorPicks.slice(0, 2).map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </div>
      </motion.section>

      <motion.section {...fadeInUp} className="relative z-10 w-full mb-8">
        <div className="container mx-auto px-4 sm:px-6">
          <div className="capsule aspect-video w-full flex items-center justify-center p-0 border border-primary/20 bg-graphite/5 relative group cursor-pointer overflow-hidden shadow-2xl">
            <img
              src="https://images.unsplash.com/photo-1550614000-4b95d415dc14?auto=format&fit=crop&q=80"
              className="absolute inset-0 w-full h-full object-cover opacity-80 group-hover:scale-105 transition-transform duration-[2s]"
              alt="Видео-история коллекции"
            />
            <div className="absolute inset-0 bg-graphite/30 group-hover:bg-graphite/40 transition-colors" />
            <div className="relative w-20 h-20 bg-white/20 backdrop-blur-md rounded-full flex items-center justify-center border border-white/50 group-hover:scale-110 transition-transform">
              <Play className="w-8 h-8 text-white ml-2" />
            </div>

            <div className="absolute bottom-8 left-8">
              <p className="text-white/80 text-xs font-bold uppercase tracking-widest mb-2 drop-shadow-md">Видеодневник / сезон 26</p>
              <h3 className="text-3xl font-serif text-white drop-shadow-md">Зимняя Меланхолия</h3>
            </div>
          </div>
        </div>
      </motion.section>

      <motion.section {...fadeInUp} className="container mx-auto px-4 sm:px-6 relative z-10 mb-20">
        <div className="flex items-end justify-between mb-8">
          <div>
            <h2 className="text-3xl font-serif text-graphite mb-2">Архив хитов</h2>
            <p className="text-sm text-ash">Модели, которые выбирают чаще всего</p>
          </div>
          <Button variant="ghost" className="hidden sm:flex">
            В каталог <ArrowRight className="w-4 h-4 ml-2" />
          </Button>
        </div>

        <div className="grid grid-cols-2 gap-4 sm:gap-6 md:grid-cols-4">
          {bestsellers.map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
      </motion.section>
    </div>
  );
}

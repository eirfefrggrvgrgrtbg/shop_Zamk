import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import { BrandCard, CategoryCard, SectionHeader } from '../components/editorial/StudioKit';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { fetchBrands, fetchCategories, fetchProducts, fetchDirectSaleProducts } from '../api/publicCatalog';
import { HeroSection } from '../components/home/HeroSection';
import { HomeAuctionBlock } from '../components/home/HomeAuctionBlock';
import type { Brand, Category, Product } from '../types/catalog';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

function EmptyHomeSection({ text }: { text: string }) {
  return (
    <div className="rounded-[2rem] border border-dashed border-border-lighter bg-white/70 p-8 text-center text-sm text-ash dark:border-white/10 dark:bg-white/5">
      {text}
    </div>
  );
}

export function Home() {
  const [products, setProducts] = useState<Product[]>([]);
  const [directSaleProducts, setDirectSaleProducts] = useState<Product[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    async function loadHomeData() {
      setIsLoading(true);
      setError('');

      try {
        const [productsRes, dsProductsRes, apiBrands, apiCategories] = await Promise.all([
          fetchProducts(),
          fetchDirectSaleProducts(),
          fetchBrands(),
          fetchCategories(),
        ]);

        if (!cancelled) {
          setProducts(productsRes.items);
          setDirectSaleProducts(dsProductsRes.items);
          setBrands(apiBrands);
          setCategories(apiCategories);
        }
      } catch {
        if (!cancelled) {
          setError('Не удалось загрузить данные витрины. Проверьте, запущен ли backend.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadHomeData();

    return () => {
      cancelled = true;
    };
  }, []);

  const featuredProducts = products.slice(0, 4);
  const recentProducts = [...products]
    .sort((left, right) => Number(right.id > left.id) - Number(left.id > right.id))
    .slice(0, 4);

  return (
    <div className="relative z-10 min-h-screen pb-20">
      <HeroSection />

      <div className="max-w-[1400px] mx-auto px-4 sm:px-6 flex flex-col gap-16 md:gap-20 pt-2">
        {error && <div className="rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error}</div>}

        {isLoading ? (
          <div className="rounded-[2rem] border border-border-lighter bg-white/70 p-8 text-center text-sm text-ash dark:border-white/10 dark:bg-white/5">
            Загрузка витрины...
          </div>
        ) : (
          <>
            <HomeAuctionBlock />

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
              {brands.length > 0 ? (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                  {brands.slice(0, 3).map((brand) => (
                    <BrandCard key={brand.id} brand={brand} />
                  ))}
                </div>
              ) : (
                <EmptyHomeSection text="Бренды пока не добавлены" />
              )}
            </motion.section>

            <motion.section {...reveal}>
              <SectionHeader
                label="Подборки"
                title="Кураторские наборы"
                description="Коллекции пока не подключены к backend."
                action={
                  <Link to="/collections">
                    <Button variant="secondary" className="gap-2">
                      Открыть раздел <ArrowRight className="w-4 h-4" />
                    </Button>
                  </Link>
                }
              />
              <EmptyHomeSection text="Коллекции пока не подключены" />
            </motion.section>

            <motion.section {...reveal}>
              <SectionHeader label="Новинки" title="Свежие поступления" />
              {recentProducts.length > 0 ? (
                <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
                  {recentProducts.map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
              ) : (
                <EmptyHomeSection text="Нет данных" />
              )}
            </motion.section>

            {directSaleProducts.length > 0 && (
              <motion.section {...reveal} className="glass-panel p-7 md:p-10 relative overflow-hidden">
                <div className="absolute inset-0 bg-gradient-to-r from-primary/5 via-transparent to-transparent opacity-50"></div>
                <SectionHeader 
                  label="Архив" 
                  title="Вещи ZAMK" 
                  action={
                    <Link to="/zamk">
                      <Button variant="secondary" className="gap-2 bg-white/50 dark:bg-black/50">
                        В архив <ArrowRight className="w-4 h-4" />
                      </Button>
                    </Link>
                  }
                />
                <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5 relative z-10">
                  {directSaleProducts.slice(0, 4).map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
              </motion.section>
            )}

            <motion.section {...reveal} className="glass-panel p-7 md:p-10">
              <SectionHeader label="Каталог" title="Товары из API" />
              {featuredProducts.length > 0 ? (
                <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
                  {featuredProducts.map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
              ) : (
                <EmptyHomeSection text="Товары пока не добавлены" />
              )}
            </motion.section>

            <motion.section {...reveal}>
              <SectionHeader label="Каталог" title="Категории" />
              {categories.length > 0 ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
                  {categories.map((category) => (
                    <CategoryCard key={category.id} category={category} />
                  ))}
                </div>
              ) : (
                <EmptyHomeSection text="Категории пока не добавлены" />
              )}
            </motion.section>
          </>
        )}
      </div>
    </div>
  );
}

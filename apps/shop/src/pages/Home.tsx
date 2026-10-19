import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import { BrandCard, SectionHeader } from '../components/editorial/StudioKit';
import { Button } from '../components/ui/Button';
import {
  fetchBrands,
  fetchCategories,
  fetchProducts,
  fetchDirectSaleProducts,
  fetchHomeRecommendations,
  fetchPopularProducts,
  type UIHomeRecommendationBlock,
} from '../api/publicCatalog';
import { HeroSection } from '../components/home/HeroSection';
import { HomeAuctionBlock } from '../components/home/HomeAuctionBlock';
import { HomeCategoryStrip } from '../components/home/HomeCategoryStrip';
import { ProductRail, ProductRailSkeleton } from '../components/home/ProductRail';
import { RecommendationBlock } from '../components/home/RecommendationBlock';
import { RecentlyViewedBlock } from '../components/home/RecentlyViewedBlock';
import {
  categoriesWithProducts,
  choosePersonalSection,
  dedupeProducts,
  pickHeroProducts,
} from '../components/home/homeSelection';
import { useAuth } from '../contexts/AuthContext';
import type { Brand, Category, Product } from '../types/catalog';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

const NEW_ARRIVALS_LIMIT = 8;

function EmptyHomeSection({ text }: { text: string }) {
  return (
    <div className="rounded-[2rem] border border-dashed border-border-lighter bg-white/70 p-8 text-center text-sm text-ash dark:border-white/10 dark:bg-white/5">
      {text}
    </div>
  );
}

export function Home() {
  const { isAuthenticated, user } = useAuth();
  const isCustomer = isAuthenticated && (!user?.role || user?.role === 'customer');

  const [newest, setNewest] = useState<Product[]>([]);
  const [directSaleProducts, setDirectSaleProducts] = useState<Product[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [customerBlocks, setCustomerBlocks] = useState<UIHomeRecommendationBlock[]>([]);
  const [guestPopular, setGuestPopular] = useState<Product[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [reloadKey, setReloadKey] = useState(0);

  const retry = useCallback(() => setReloadKey((key) => key + 1), []);

  useEffect(() => {
    let cancelled = false;

    async function loadHomeData() {
      setIsLoading(true);
      setError('');

      try {
        const [productsRes, dsProductsRes, apiBrands, apiCategories] = await Promise.all([
          fetchProducts({ sort: 'newest' }),
          fetchDirectSaleProducts(),
          fetchBrands(),
          fetchCategories(),
        ]);

        if (!cancelled) {
          setNewest(productsRes.items);
          setDirectSaleProducts(dsProductsRes.items);
          setBrands(apiBrands);
          setCategories(apiCategories);
        }
      } catch {
        if (!cancelled) {
          setError('Не удалось загрузить витрину. Проверьте соединение и попробуйте ещё раз.');
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
  }, [reloadKey]);

  // Personal discovery. Customers get the backend composer (favorites / views / paid orders);
  // guests get only the public popular selection. Failures degrade to "no section".
  useEffect(() => {
    let cancelled = false;

    if (isCustomer) {
      fetchHomeRecommendations()
        .then((res) => {
          if (!cancelled) setCustomerBlocks(res.blocks || []);
        })
        .catch(() => {
          if (!cancelled) setCustomerBlocks([]);
        });
    } else {
      fetchPopularProducts(12)
        .then((res) => {
          if (!cancelled) setGuestPopular(res.items);
        })
        .catch(() => {
          if (!cancelled) setGuestPopular([]);
        });
    }

    return () => {
      cancelled = true;
    };
  }, [isCustomer, reloadKey]);

  const heroProducts = useMemo(() => pickHeroProducts(newest), [newest]);
  const categoryLinks = useMemo(() => categoriesWithProducts(categories, newest), [categories, newest]);

  // Sections are deduplicated in display order: For you > New arrivals > Popular.
  const { personal, newArrivals, popular } = useMemo(() => {
    // Only the source matching the current auth state counts (state may linger from a previous login).
    const blocks = isCustomer ? customerBlocks : [];
    const section = choosePersonalSection(blocks, isCustomer ? [] : guestPopular);
    const personalItems = section?.kind === 'for_you' ? section.items : [];
    const shown = new Set(personalItems.map((p) => p.id));

    const newItems = dedupeProducts(newest, shown).slice(0, NEW_ARRIVALS_LIMIT);
    newItems.forEach((p) => shown.add(p.id));

    const popularCandidates =
      section?.kind === 'for_you'
        ? blocks.find((b) => b.type === 'popular')?.items ?? []
        : section?.items ?? [];

    return {
      personal: section?.kind === 'for_you' ? section : null,
      newArrivals: newItems,
      popular: dedupeProducts(popularCandidates, shown),
    };
  }, [isCustomer, customerBlocks, guestPopular, newest]);

  const hasAnyProducts = newest.length > 0;

  return (
    <div className="relative z-10 min-h-screen pb-20">
      <HeroSection products={heroProducts} />

      <div className="max-w-[1400px] mx-auto px-4 sm:px-6 flex flex-col gap-14 md:gap-20">
        {error && (
          <div
            role="alert"
            className="flex flex-col items-start gap-3 rounded-2xl border border-red-200 bg-red-50 p-5 text-sm text-red-700 sm:flex-row sm:items-center sm:justify-between"
          >
            <span>{error}</span>
            <Button variant="secondary" onClick={retry}>
              Повторить
            </Button>
          </div>
        )}

        {isLoading ? (
          <section aria-busy="true" aria-label="Загрузка витрины">
            <div className="mb-7 h-9 w-56 animate-pulse rounded-full bg-ice/80 dark:bg-white/5" />
            <ProductRailSkeleton />
            <p className="sr-only">Загрузка витрины...</p>
          </section>
        ) : (
          <>
            {categoryLinks.length > 0 && <HomeCategoryStrip categories={categoryLinks} />}

            {personal && (
              <RecommendationBlock
                block={{ type: 'for_you', title: 'Для вас', items: personal.items }}
                description="Подобрано по вашему избранному, просмотрам и заказам."
              />
            )}

            {newArrivals.length > 0 ? (
              <motion.section {...reveal} data-testid="new-arrivals-section">
                <SectionHeader
                  label="Новинки"
                  title="Новые поступления"
                  action={
                    <Link to="/new">
                      <Button variant="secondary" className="gap-2">
                        Все новинки <ArrowRight className="w-4 h-4" />
                      </Button>
                    </Link>
                  }
                />
                <ProductRail products={newArrivals} placement="home_new_arrivals" layout="grid" />
              </motion.section>
            ) : (
              !error &&
              !hasAnyProducts && (
                <section data-testid="catalog-empty">
                  <EmptyHomeSection text="Товары пока не добавлены. Загляните позже." />
                </section>
              )
            )}

            {popular.length > 0 && (
              <RecommendationBlock
                block={{ type: 'popular', title: 'Популярное', items: popular }}
                description="Чаще всего покупают за последние 30 дней."
              />
            )}

            <HomeAuctionBlock />

            {isCustomer && <RecentlyViewedBlock />}

            {brands.length > 0 && (
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
                  {brands.slice(0, 3).map((brand) => (
                    <BrandCard key={brand.id} brand={brand} />
                  ))}
                </div>
              </motion.section>
            )}

            {directSaleProducts.length > 0 && (
              <motion.section {...reveal}>
                <SectionHeader
                  label="Архив"
                  title="Вещи ZAMK"
                  action={
                    <Link to="/zamk">
                      <Button variant="secondary" className="gap-2">
                        В архив <ArrowRight className="w-4 h-4" />
                      </Button>
                    </Link>
                  }
                />
                <ProductRail products={directSaleProducts.slice(0, 4)} placement="home_archive" layout="grid" />
              </motion.section>
            )}
          </>
        )}
      </div>
    </div>
  );
}

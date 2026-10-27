import { useCallback, useEffect, useMemo, useState } from 'react';
import { fetchBrands, fetchCategories, fetchHomeRecommendations, fetchPopularProducts, fetchProducts } from '../../api/publicCatalog';
import type { UIHomeRecommendationBlock } from '../../api/publicCatalog';
import { useAuth } from '../../contexts/AuthContext';
import type { Brand, Category, Product } from '../../types/catalog';
import { V5ArchiveBrands } from './V5ArchiveBrands';
import { V5Footer } from './V5Footer';
import { V5ForYou } from './V5ForYou';
import { V5Header } from './V5Header';
import { V5Hero } from './V5Hero';
import { V5NewArrivals } from './V5NewArrivals';
import { useV5Resource } from './useV5Resource';
import { chooseForYouSection, pickArrivals, pickIndexCategories } from './v5Selection';
import './v5-home.css';

const FONTS_HREF =
  'https://fonts.googleapis.com/css2?family=Cormorant+Garamond:ital,wght@0,300;0,400;0,500;1,300;1,400&family=Manrope:wght@300;400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap';
const FONTS_LINK_ID = 'v5-home-fonts';
const CATALOG_ERROR = 'Не удалось загрузить каталог. Проверьте соединение и попробуйте ещё раз.';
const BRANDS_ERROR = 'Не удалось загрузить бренды. Проверьте соединение и попробуйте ещё раз.';

/**
 * IBM Plex Mono is not part of the current Shop fonts: load the V5 families only while the preview is mounted.
 * Content is never hidden while the fonts load (font-display: swap): the fallback text is shown immediately.
 */
function useV5Fonts() {
  useEffect(() => {
    if (document.getElementById(FONTS_LINK_ID)) return;
    const link = document.createElement('link');
    link.id = FONTS_LINK_ID;
    link.rel = 'stylesheet';
    link.href = FONTS_HREF;
    document.head.appendChild(link);
    return () => link.remove();
  }, []);
}

/**
 * Shop V5 home. Dev-only preview: mounted at /v5-preview, the production `/` (Home.tsx) is untouched.
 * Data: existing storefront endpoints only (newest products, brands, categories, popular, customer recommendations).
 */
export function ShopV5Home() {
  useV5Fonts();
  const { isAuthenticated, user } = useAuth();
  const isCustomer = isAuthenticated && (!user?.role || user.role === 'customer');

  const [reloadKey, setReloadKey] = useState(0);
  const retry = useCallback(() => setReloadKey((k) => k + 1), []);

  const catalog = useV5Resource(`catalog:${reloadKey}`, async () => {
    const [products, categories] = await Promise.all([
      fetchProducts({ sort: 'newest' }),
      fetchCategories().catch(() => [] as Category[]),
    ]);
    return { newest: products.items as Product[], totalCount: products.totalCount, categories };
  });
  const brandsRes = useV5Resource(`brands:${reloadKey}`, () => fetchBrands());
  // Customers: backend composer (favorites / views / paid orders). Guests: public popular ranking only.
  const personal = useV5Resource(`personal:${isCustomer}:${reloadKey}`, async () =>
    isCustomer
      ? { customerBlocks: (await fetchHomeRecommendations()).blocks || [], guestPopular: [] as Product[] }
      : { customerBlocks: [] as UIHomeRecommendationBlock[], guestPopular: (await fetchPopularProducts(12)).items },
  );

  const newest = useMemo(() => catalog.data?.newest ?? [], [catalog.data]);
  const totalCount = catalog.data?.totalCount ?? 0;
  const categories = useMemo(() => catalog.data?.categories ?? [], [catalog.data]);
  const brands = useMemo<Brand[]>(() => brandsRes.data ?? [], [brandsRes.data]);
  const customerBlocks = personal.data?.customerBlocks;
  const guestPopular = personal.data?.guestPopular;

  const arrivals = useMemo(() => pickArrivals(newest), [newest]);
  const indexCategories = useMemo(() => pickIndexCategories(categories, newest), [categories, newest]);
  const forYou = useMemo(
    () =>
      chooseForYouSection({
        isCustomer,
        // Only the source matching the current auth state counts: state may linger from a previous login.
        customerBlocks: isCustomer ? customerBlocks ?? [] : [],
        guestPopular: isCustomer ? [] : guestPopular ?? [],
        newest,
        exclude: new Set(arrivals.map((p) => p.id)),
      }),
    [isCustomer, customerBlocks, guestPopular, newest, arrivals],
  );

  return (
    <div className="v5" data-testid="v5-home">
      <div className="v5-atmo" aria-hidden="true" />
      <V5Header />
      <main>
        <V5Hero categories={indexCategories} totalCount={totalCount} />
        <V5NewArrivals
          products={arrivals}
          totalCount={totalCount}
          isLoading={catalog.isLoading}
          error={catalog.error ? CATALOG_ERROR : ''}
          onRetry={retry}
        />
        <V5ForYou section={forYou} isLoading={catalog.isLoading || personal.isLoading} />
        <V5ArchiveBrands brands={brands} featuredBrandId={arrivals[0]?.brandId || undefined} isLoading={brandsRes.isLoading} error={brandsRes.error ? BRANDS_ERROR : ''} onRetry={retry} />
      </main>
      <V5Footer />
    </div>
  );
}

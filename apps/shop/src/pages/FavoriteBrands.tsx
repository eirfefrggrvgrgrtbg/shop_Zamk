import { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Loader2, X } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';
import { AccountLayout } from '../components/account/AccountLayout';
import { useFavoriteBrands } from '../contexts/FavoriteBrandsContext';
import { ProductCard } from '../components/product/ProductCard';
import { fetchFavoriteBrandProducts } from '../api/publicCatalog';
import type { Product } from '../types/catalog';

const FEED_PAGE_SIZE = 12;

export function FavoriteBrands() {
  return (
    <AccountLayout title="Любимые бренды">
      <FavoriteBrandsContent />
    </AccountLayout>
  );
}

function FavoriteBrandsContent() {
  const { brands, isLoading, error, pendingIds, toggleFavoriteBrand, reloadFavoriteBrands } = useFavoriteBrands();
  const [actionError, setActionError] = useState<string | null>(null);

  const handleRemove = async (brandId: string) => {
    setActionError(null);
    const ok = await toggleFavoriteBrand(brandId);
    if (!ok) setActionError('Не удалось удалить бренд из избранного. Попробуйте ещё раз.');
  };

  const card = 'mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm max-w-5xl mx-auto';

  if (isLoading && brands.length === 0) {
    return (
      <div className="flex justify-center items-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-graphite/40" />
      </div>
    );
  }

  if (error) {
    return (
      <div className={card}>
        <EmptyState
          icon="heart"
          title="Не удалось загрузить любимые бренды"
          description={error}
          action={<Button onClick={() => reloadFavoriteBrands()}>Повторить</Button>}
        />
      </div>
    );
  }

  if (brands.length === 0) {
    return (
      <div className={card}>
        <EmptyState
          icon="heart"
          title="Любимых брендов пока нет"
          description="Сохраняйте бренды, за которыми хотите следить, — их витрины всегда будут под рукой."
          action={
            <Link to="/brands">
              <Button className="gap-2">
                Смотреть бренды <ArrowRight className="w-4 h-4" />
              </Button>
            </Link>
          }
        />
      </div>
    );
  }

  return (
    <>
      {actionError && (
        <div role="alert" className="mt-6 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-950/20 dark:text-red-300">
          {actionError}
        </div>
      )}
      <ul className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5 mt-8">
        {brands.map((brand) => (
          <li
            key={brand.id}
            data-testid={`favorite-brand-${brand.id}`}
            className="relative rounded-[2rem] border border-border-lighter bg-white/70 p-6 shadow-sm backdrop-blur-md dark:border-white/10 dark:bg-white/5"
          >
            <Link to={`/brand/${brand.slug || brand.id}`} className="flex items-center gap-4 group">
              <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-ice text-xl font-serif text-graphite dark:bg-white/10 dark:text-white border border-border-lighter dark:border-white/10">
                {brand.logoUrl ? (
                  <img src={brand.logoUrl} alt={brand.name} className="h-full w-full object-cover" loading="lazy" />
                ) : (
                  <span>{brand.name.trim().slice(0, 1).toUpperCase()}</span>
                )}
              </div>
              <div className="min-w-0 flex-1 pr-8">
                <h2 className="text-xl font-serif font-medium text-graphite dark:text-white truncate">{brand.name}</h2>
                <span className="mt-1 inline-flex items-center gap-1 text-xs uppercase tracking-[0.14em] text-ash group-hover:text-graphite dark:group-hover:text-white transition-colors">
                  Смотреть витрину <ArrowRight className="w-3.5 h-3.5" />
                </span>
              </div>
            </Link>
            {brand.description?.trim() && (
              <p className="mt-4 text-sm text-graphite-light dark:text-white/70 line-clamp-2">{brand.description.trim()}</p>
            )}
            <button
              type="button"
              onClick={() => handleRemove(brand.id)}
              disabled={pendingIds.includes(brand.id)}
              aria-label={`Удалить ${brand.name} из любимых брендов`}
              className="absolute top-4 right-4 p-2 rounded-full text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/10 disabled:opacity-50 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </li>
        ))}
      </ul>
      <FavoriteBrandsFeed brandKey={brands.map((b) => b.id).join(',')} />
    </>
  );
}

// Personal feed. Ordered by the canonical catalog order (not a reliable "new arrivals" signal), hence the neutral title.
function FavoriteBrandsFeed({ brandKey }: { brandKey: string }) {
  const [items, setItems] = useState<Product[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [moreError, setMoreError] = useState<string | null>(null);
  const requestId = useRef(0);

  const loadFirst = useCallback(async () => {
    const id = ++requestId.current;
    setIsLoading(true);
    setError(null);
    setMoreError(null);
    try {
      const res = await fetchFavoriteBrandProducts(FEED_PAGE_SIZE, 0);
      if (id !== requestId.current) return;
      setItems(res.items);
      setTotal(res.totalCount);
    } catch {
      if (id !== requestId.current) return;
      setError('Не удалось загрузить вещи любимых брендов.');
    } finally {
      if (id === requestId.current) setIsLoading(false);
    }
  }, []);

  // Refetch whenever the set of favorite brands changes (e.g. a brand was removed above).
  useEffect(() => {
    loadFirst();
  }, [brandKey, loadFirst]);

  const loadMore = async () => {
    const id = requestId.current;
    setIsLoadingMore(true);
    setMoreError(null);
    try {
      const res = await fetchFavoriteBrandProducts(FEED_PAGE_SIZE, items.length);
      if (id !== requestId.current) return;
      setItems((prev) => {
        const seen = new Set(prev.map((p) => p.id));
        return [...prev, ...res.items.filter((p) => !seen.has(p.id))];
      });
      setTotal(res.totalCount);
    } catch {
      if (id === requestId.current) setMoreError('Не удалось загрузить ещё. Попробуйте снова.');
    } finally {
      if (id === requestId.current) setIsLoadingMore(false);
    }
  };

  return (
    <section className="mt-12" aria-labelledby="favorite-brands-feed-title" data-testid="favorite-brands-feed">
      <h2 id="favorite-brands-feed-title" className="text-2xl md:text-3xl font-serif text-graphite dark:text-white mb-6">
        Вещи любимых брендов
      </h2>
      {isLoading ? (
        <div className="flex justify-center items-center h-40" role="status" aria-label="Загрузка">
          <Loader2 className="w-6 h-6 animate-spin text-graphite/40" />
        </div>
      ) : error ? (
        <div role="alert" className="rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-950/20 dark:text-red-300 flex flex-wrap items-center justify-between gap-3">
          <span>{error}</span>
          <Button variant="outline" onClick={loadFirst}>Повторить</Button>
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-border-lighter dark:border-white/10 bg-white/40 dark:bg-white/5 p-8 text-center text-sm text-graphite-light dark:text-white/60">
          Сейчас у ваших любимых брендов нет доступных вещей. Они появятся здесь, как только бренды опубликуют новые товары.
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
            {items.map((product) => (
              <ProductCard key={product.id} product={product} placement="favorite_brands_feed" />
            ))}
          </div>
          {moreError && (
            <p role="alert" className="mt-4 text-center text-xs text-red-600 dark:text-red-300">{moreError}</p>
          )}
          {items.length < total && (
            <div className="mt-8 flex justify-center">
              <Button variant="outline" onClick={loadMore} disabled={isLoadingMore}>
                {isLoadingMore ? 'Загрузка…' : 'Показать ещё'}
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  );
}

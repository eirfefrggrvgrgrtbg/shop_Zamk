import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { ChevronDown, SlidersHorizontal, Check, Search } from 'lucide-react';
import { fetchPublicSeller, fetchCategories, fetchBrands } from '../api/publicCatalog';
import { ProductCard } from '../components/product/ProductCard';
import { SortDropdown } from '../components/editorial/StudioKit';
import { Drawer } from '../components/ui/Drawer';
import type { Product, Category, Brand } from '../types/catalog';
import { cn } from '../lib/utils';

const SORT_OPTIONS = [
  { value: 'newest', label: 'Сначала новые' },
  { value: 'price_asc', label: 'Цена по возрастанию' },
  { value: 'price_desc', label: 'Цена по убыванию' },
];

const SIZES = ['XS', 'S', 'M', 'L', 'XL', 'XXL'];
const DEFAULT_PRICE_RANGE: [number, number] = [0, 100000];

function FilterSection({ title, children, defaultOpen = true }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-black/10 dark:border-white/10 py-4">
      <button onClick={() => setIsOpen(!isOpen)} className="w-full flex items-center justify-between text-left group">
        <span className="text-[10px] font-mono uppercase tracking-widest text-black dark:text-white/90">{title}</span>
        <ChevronDown className={cn("w-3.5 h-3.5 text-black/50 dark:text-white/50 transition-transform duration-300", isOpen && "rotate-180")} />
      </button>
      <div className={cn("overflow-hidden transition-all duration-300 ease-in-out", isOpen ? "mt-4 max-h-[1000px] opacity-100" : "max-h-0 opacity-0")}>
        {children}
      </div>
    </div>
  );
}

function FilterCheckbox({ label, isActive, onClick }: { label: string; isActive: boolean; onClick: () => void }) {
  return (
    <button onClick={onClick} className="flex items-center justify-between w-full py-1.5 group text-left">
      <div className="flex items-center gap-3">
        <div className="w-3.5 h-3.5 flex items-center justify-center shrink-0">
          {isActive && <Check className="w-4 h-4 text-black dark:text-white" strokeWidth={2.5} />}
        </div>
        <span className={cn("text-[12px] font-sans font-light tracking-wide", isActive ? "text-black dark:text-white font-normal" : "text-black/70 dark:text-white/70 group-hover:text-black dark:group-hover:text-white")}>
          {label}
        </span>
      </div>
    </button>
  );
}

export function SellerDetail() {
  const { slugOrId } = useParams<{ slugOrId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  
  const [seller, setSeller] = useState<any>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [categories, setCategories] = useState<Category[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);

  // Filters
  const [q, setQ] = useState(searchParams.get('q') || '');
  const [activeCategory, setActiveCategory] = useState(searchParams.get('categoryId') || 'all');
  const [activeBrand, setActiveBrand] = useState<string | null>(searchParams.get('brandId'));
  const [activeSize, setActiveSize] = useState<string | null>(searchParams.get('size'));
  const [onlyInStock, setOnlyInStock] = useState(searchParams.get('inStock') === 'true');
  const [priceRange, setPriceRange] = useState<[number, number]>([
    Number(searchParams.get('minPriceCents')) / 100 || DEFAULT_PRICE_RANGE[0],
    Number(searchParams.get('maxPriceCents')) / 100 || DEFAULT_PRICE_RANGE[1]
  ]);
  const [sortBy, setSortBy] = useState(searchParams.get('sort') || 'newest');
  
  const [offset, setOffset] = useState(0);
  const [limit] = useState(24);
  const [isFetchingMore, setIsFetchingMore] = useState(false);
  
  const [showMobileFilters, setShowMobileFilters] = useState(false);

  const metaLoaded = useRef(false);

  const handleFilterChange = useCallback((setter: any, value: any) => {
    setter(value);
    setOffset(0);
  }, []);

  const buildParams = useCallback(() => {
    const params: Record<string, any> = { sort: sortBy, limit, offset };
    if (q) params.q = q;
    if (activeCategory !== 'all') params.categoryId = activeCategory;
    if (activeBrand) params.brandId = activeBrand;
    if (activeSize) params.size = activeSize;
    if (onlyInStock) params.inStock = 'true';
    if (priceRange[0] > DEFAULT_PRICE_RANGE[0]) params.minPriceCents = priceRange[0] * 100;
    if (priceRange[1] < DEFAULT_PRICE_RANGE[1]) params.maxPriceCents = priceRange[1] * 100;
    return params;
  }, [q, activeCategory, activeBrand, activeSize, onlyInStock, priceRange, sortBy]);

  useEffect(() => {
    async function load() {
      if (!slugOrId) return;
      setIsLoading(true);
      setError(null);
      try {
        const params = buildParams();
        const [res, apiCategories, apiBrands] = metaLoaded.current 
          ? [await fetchPublicSeller(slugOrId, params), categories, brands]
          : await Promise.all([fetchPublicSeller(slugOrId, params), fetchCategories(), fetchBrands()]);

        if (!metaLoaded.current) {
          setCategories(apiCategories as Category[]);
          setBrands(apiBrands as Brand[]);
          metaLoaded.current = true;
        }

        if (offset === 0) {
          setProducts(res.items);
        } else {
          // Prevent duplicates if API returns overlapping items (though it shouldn't)
          setProducts(prev => {
            const existingIds = new Set(prev.map(p => p.id));
            const newItems = res.items.filter((p: any) => !existingIds.has(p.id));
            return [...prev, ...newItems];
          });
        }
        setTotalCount(res.totalCount);

        // Sync URL (don't sync offset/limit to keep URL clean for initial load)
        const newParams = new URLSearchParams(searchParams);
        if (q) newParams.set('q', q); else newParams.delete('q');
        if (activeCategory !== 'all') newParams.set('categoryId', activeCategory); else newParams.delete('categoryId');
        if (activeBrand) newParams.set('brandId', activeBrand); else newParams.delete('brandId');
        if (activeSize) newParams.set('size', activeSize); else newParams.delete('size');
        if (onlyInStock) newParams.set('inStock', 'true'); else newParams.delete('inStock');
        if (priceRange[0] > DEFAULT_PRICE_RANGE[0]) newParams.set('minPriceCents', (priceRange[0] * 100).toString()); else newParams.delete('minPriceCents');
        if (priceRange[1] < DEFAULT_PRICE_RANGE[1]) newParams.set('maxPriceCents', (priceRange[1] * 100).toString()); else newParams.delete('maxPriceCents');
        newParams.set('sort', sortBy);
        setSearchParams(newParams, { replace: true });
        
      } catch (err: any) {
        if (err?.response?.status === 404 || err?.status === 404 || err?.message?.includes('404')) {
          setError('not_found');
        } else {
          setError('unknown_error');
        }
      } finally {
        setIsLoading(false);
        setIsFetchingMore(false);
      }
    }
    const delayDebounce = setTimeout(() => {
      load();
    }, 300);
    return () => clearTimeout(delayDebounce);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slugOrId, buildParams]); 

  const hasActivePriceFilter = priceRange[0] !== DEFAULT_PRICE_RANGE[0] || priceRange[1] !== DEFAULT_PRICE_RANGE[1];
  const hasActiveFilters = q !== '' || activeCategory !== 'all' || activeBrand !== null || activeSize !== null || onlyInStock || hasActivePriceFilter;
  const activeFiltersCount = (q ? 1 : 0) + (activeBrand ? 1 : 0) + (activeCategory !== 'all' ? 1 : 0) + (activeSize ? 1 : 0) + (onlyInStock ? 1 : 0) + (hasActivePriceFilter ? 1 : 0);

  const resetFilters = () => {
    setQ('');
    setActiveCategory('all');
    setActiveBrand(null);
    setActiveSize(null);
    setOnlyInStock(false);
    setPriceRange(DEFAULT_PRICE_RANGE);
    setOffset(0);
  };

  const FiltersContent = () => (
    <div className="flex flex-col px-1 pb-4">
      {/* Search */}
      <div className="relative mb-6">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-black/40 dark:text-white/40" />
        <input 
          type="text" 
          placeholder="Поиск по магазину..." 
          value={q} 
          onChange={(e) => handleFilterChange(setQ, e.target.value)}
          className="w-full h-10 pl-9 pr-4 rounded-lg bg-black/5 dark:bg-white/5 border border-transparent focus:border-black/20 dark:focus:border-white/20 text-sm outline-none transition-colors"
        />
      </div>

      <FilterSection title="Категории" defaultOpen={activeCategory !== 'all'}>
        <div className="flex flex-col gap-1">
          <FilterCheckbox label="Все товары" isActive={activeCategory === 'all'} onClick={() => handleFilterChange(setActiveCategory, 'all')} />
          {categories.map((c) => (
            <FilterCheckbox key={c.id} label={c.name} isActive={activeCategory === c.id} onClick={() => handleFilterChange(setActiveCategory, c.id)} />
          ))}
        </div>
      </FilterSection>

      <FilterSection title="Бренды" defaultOpen={activeBrand !== null}>
        <div className="flex flex-col gap-1">
          <FilterCheckbox label="Все бренды" isActive={activeBrand === null} onClick={() => handleFilterChange(setActiveBrand, null)} />
          {brands.map((b) => (
            <FilterCheckbox key={b.id} label={b.name} isActive={activeBrand === b.id} onClick={() => handleFilterChange(setActiveBrand, b.id)} />
          ))}
        </div>
      </FilterSection>

      <FilterSection title="Размер" defaultOpen={activeSize !== null}>
        <div className="grid grid-cols-4 gap-2">
          {SIZES.map((size) => (
            <button
              key={size}
              onClick={() => handleFilterChange(setActiveSize, (prev: string | null) => prev === size ? null : size)}
              className={cn(
                "h-8 border text-[11px] font-mono flex items-center justify-center rounded-[2px]",
                activeSize === size
                  ? "bg-black text-white border-black dark:bg-white dark:text-black dark:border-white"
                  : "bg-transparent border-black/20 dark:border-white/20 text-black/60 dark:text-white/60 hover:border-black/50"
              )}
            >
              {size}
            </button>
          ))}
        </div>
      </FilterSection>

      <FilterSection title="Цена" defaultOpen={hasActivePriceFilter}>
        <div className="flex items-center justify-between gap-2 mt-1">
          <input
            type="number"
            value={priceRange[0] || ''}
            onChange={(e) => handleFilterChange(setPriceRange, [Number(e.target.value) || 0, priceRange[1]])}
            placeholder="От"
            className="w-full h-8 px-2 rounded border border-black/20 dark:border-white/20 bg-transparent text-sm"
          />
          <input
            type="number"
            value={priceRange[1] === 100000 ? '' : priceRange[1]}
            onChange={(e) => handleFilterChange(setPriceRange, [priceRange[0], Number(e.target.value) || 100000])}
            placeholder="До"
            className="w-full h-8 px-2 rounded border border-black/20 dark:border-white/20 bg-transparent text-sm"
          />
        </div>
        {priceRange[0] > priceRange[1] && <div className="text-red-500 text-xs mt-2">Некорректный диапазон</div>}
      </FilterSection>

      <FilterSection title="Наличие" defaultOpen={onlyInStock}>
        <FilterCheckbox label="Только в наличии" isActive={onlyInStock} onClick={() => handleFilterChange(setOnlyInStock, !onlyInStock)} />
      </FilterSection>

      {hasActiveFilters && (
        <button
          onClick={resetFilters}
          className="w-full mt-6 h-8 text-[10px] font-mono uppercase tracking-widest text-black/60 border-b border-dashed border-black/20 hover:border-black/60"
        >
          Сбросить фильтры
        </button>
      )}
    </div>
  );

  if (isLoading && !seller) {
    return (
      <div className="min-h-screen pt-32 pb-24 px-6 md:px-12 flex items-center justify-center">
        <div className="text-[12px] font-mono tracking-widest text-black/40 uppercase">Загрузка...</div>
      </div>
    );
  }

  if (error === 'not_found' || (!isLoading && !seller)) {
    return (
      <div className="min-h-screen pt-32 pb-24 px-6 md:px-12 flex flex-col items-center justify-center">
        <h1 className="text-4xl md:text-5xl font-light mb-6">Магазин не найден</h1>
        <Link to="/catalog" className="h-12 px-8 flex items-center justify-center bg-black text-white text-[11px] font-mono uppercase">
          В каталог
        </Link>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f1f5f9] dark:bg-[#0a0a0a] transition-colors duration-500 pt-24 pb-24">
      {/* Header Info */}
      <div className="border-b border-black/10 dark:border-white/10 bg-white/50 dark:bg-black/50 backdrop-blur-xl sticky top-[60px] z-30">
        <div className="max-w-[1600px] mx-auto px-6 md:px-12 py-8 md:py-12">
          <div className="flex flex-col md:flex-row gap-8 items-start md:items-center">
            <div className="w-24 h-24 md:w-32 md:h-32 rounded-2xl overflow-hidden bg-black/5 shrink-0 border border-black/5">
              {seller.logoUrl ? (
                <img src={seller.logoUrl} alt={seller.name} className="w-full h-full object-cover" />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-black/20">
                  <span className="text-[10px] font-mono uppercase text-center">Нет<br/>лого</span>
                </div>
              )}
            </div>
            <div className="flex-1 min-w-0">
              <h1 className="text-3xl md:text-5xl font-light mb-4">{seller.name}</h1>
              {seller.description && <p className="text-[15px] font-light text-black/60 dark:text-white/60 max-w-2xl leading-relaxed">{seller.description}</p>}
            </div>
          </div>
        </div>
      </div>

      <div className="max-w-[1600px] mx-auto px-6 md:px-12 mt-12">
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-4">
            <button
              className="lg:hidden inline-flex items-center gap-2 h-10 px-4 rounded-lg border border-black/10 text-sm"
              onClick={() => setShowMobileFilters(true)}
            >
              <SlidersHorizontal className="w-4 h-4" />
              Фильтры
              {hasActiveFilters && <span className="w-5 h-5 rounded-full bg-black text-white text-xs flex items-center justify-center">{activeFiltersCount}</span>}
            </button>
            <h2 className="hidden lg:block text-[13px] font-mono tracking-widest uppercase text-black/40">Все товары [{totalCount}]</h2>
          </div>
          <SortDropdown value={sortBy} onChange={(v) => handleFilterChange(setSortBy, v)} options={SORT_OPTIONS} />
        </div>

        <div className="flex gap-10">
          {/* Desktop Filters */}
          <aside className="hidden lg:block w-[280px] shrink-0 relative">
            <div className="sticky top-28 bg-white/5 rounded-[24px] border-[1px] border-black/5 dark:border-white/10 shadow-[0_8px_32px_rgba(0,0,0,0.04)] overflow-hidden">
              <div className="px-6 pb-6 pt-6"><FiltersContent /></div>
            </div>
          </aside>

          {/* Grid */}
          <div className="flex-1 min-w-0">
            {isLoading && offset === 0 ? (
              <div className="py-32 flex flex-col items-center justify-center">
                <div className="animate-spin w-8 h-8 mx-auto border-2 border-black border-t-transparent rounded-full" />
              </div>
            ) : products.length > 0 ? (
              <>
                <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-8">
                  {products.map((product) => <ProductCard key={product.id} product={product} />)}
                </div>
                {products.length < totalCount && (
                  <div className="mt-12 flex justify-center">
                    <button
                      onClick={() => {
                        setIsFetchingMore(true);
                        setOffset(prev => prev + limit);
                      }}
                      disabled={isFetchingMore}
                      className="px-8 h-12 rounded-full border border-black/10 dark:border-white/10 text-sm font-medium hover:bg-black/5 dark:hover:bg-white/5 transition-colors disabled:opacity-50 flex items-center gap-2"
                    >
                      {isFetchingMore ? <div className="w-4 h-4 rounded-full border-2 border-black border-t-transparent animate-spin" /> : null}
                      Показать еще
                    </button>
                  </div>
                )}
              </>
            ) : (
              <div className="py-32 flex flex-col items-center justify-center border border-dashed border-black/10 rounded-2xl">
                <p className="text-[14px] font-light text-black/50 text-center mb-6">У этого продавца пока нет активных товаров по заданным фильтрам</p>
                {hasActiveFilters && <button onClick={resetFilters} className="h-10 px-6 bg-black text-white rounded text-sm">Сбросить фильтры</button>}
              </div>
            )}
          </div>
        </div>
      </div>

      <Drawer isOpen={showMobileFilters} onClose={() => setShowMobileFilters(false)} title="Фильтры">
        <div className="pb-20 bg-white/80"><FiltersContent /></div>
        <div className="fixed bottom-0 left-0 right-0 p-4 bg-white/90 border-t">
          <button onClick={() => setShowMobileFilters(false)} className="w-full h-10 rounded-lg bg-black text-white text-[12px] font-mono uppercase">
            Показать {totalCount} товаров
          </button>
        </div>
      </Drawer>
    </div>
  );
}

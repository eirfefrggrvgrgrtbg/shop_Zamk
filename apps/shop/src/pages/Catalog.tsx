import { useCallback, useMemo, useState, useEffect, useRef } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { ChevronDown, ChevronRight, SlidersHorizontal, X, Check } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Drawer } from '../components/ui/Drawer';
import { SortDropdown } from '../components/editorial/StudioKit';
import { Button } from '../components/ui/Button';
import { fetchBrands, fetchCategories, fetchColors, fetchProducts, type CatalogColor } from '../api/publicCatalog';
import { useAuth } from '../contexts/AuthContext';
import type { Brand, Category, Product } from '../types/catalog';
import { cn } from '../lib/utils';
import { buildSizeFilterModel, sanitizeSizeSystem } from '../lib/catalogSizes';

const SORT_OPTIONS = [
  { value: 'default', label: 'По умолчанию' },
  { value: 'newest', label: 'Сначала новые' },
  { value: 'price_asc', label: 'Цена по возрастанию' },
  { value: 'price_desc', label: 'Цена по убыванию' },
  { value: 'rating_desc', label: 'По высокому рейтингу' },
];

const DEFAULT_PRICE_RANGE: [number, number] = [0, 100000];

// Компонент раскрывающейся секции фильтра
function FilterSection({ title, children, defaultOpen = true }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-black/10 dark:border-white/10 py-4">
      <button
        type="button"
        aria-expanded={isOpen}
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between text-left group"
      >
        <span className="text-[10px] font-mono uppercase tracking-widest text-black dark:text-white/90">{title}</span>
        <ChevronDown className={cn("w-3.5 h-3.5 text-black/50 dark:text-white/50 transition-transform duration-300 group-hover:text-black dark:group-hover:text-white", isOpen && "rotate-180")} />
      </button>
      {/* grid-rows animates to the content's real height: nothing is clipped, nothing needs a magic max-height */}
      <div
        aria-hidden={!isOpen}
        className={cn("grid transition-[grid-template-rows,opacity,margin] duration-300 ease-in-out", isOpen ? "mt-4 grid-rows-[1fr] opacity-100" : "mt-0 grid-rows-[0fr] opacity-0 pointer-events-none")}
      >
        <div className="min-h-0 min-w-0 overflow-hidden">{children}</div>
      </div>
    </div>
  );
}

// Длинные списки (категории, бренды, цвета) прокручиваются внутри секции, а не растягивают панель
const LIST_SCROLL = 'max-h-72 overflow-y-auto overscroll-contain pr-1 [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-black/10 dark:[&::-webkit-scrollbar-thumb]:bg-white/10';

// Строгий чекбокс для списков (категории, бренды, т.д.)
function FilterCheckbox({
  label,
  isActive,
  onClick,
  count
}: {
  label: string;
  isActive: boolean;
  onClick: () => void;
  count?: number;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center justify-between gap-2 w-full py-1.5 group text-left"
    >
      <div className="flex items-center gap-3 min-w-0">
        <div className="w-3.5 h-3.5 flex items-center justify-center shrink-0">
          {isActive && <Check className="w-4 h-4 text-black dark:text-white" strokeWidth={2.5} />}
        </div>
        <span title={label} className={cn(
          "text-[12px] font-sans font-light tracking-wide transition-colors truncate",
          isActive ? "text-black dark:text-white font-normal" : "text-black/70 dark:text-white/70 group-hover:text-black dark:group-hover:text-white"
        )}>
          {label}
        </span>
      </div>
      {count !== undefined && (
        <span className="text-[10px] font-mono text-black/40 dark:text-white/40 shrink-0">
          [{count}]
        </span>
      )}
    </button>
  );
}

export function Catalog() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { isAuthenticated, user } = useAuth();
  const isCustomer = Boolean(isAuthenticated && user?.role === 'customer');

  // The URL is the single source of truth for every filter (category, brand, price, sizes, sort, stock):
  // the UI is derived from it, so reloads, direct links and Back/Forward always restore the full filter set.
  const searchParamsRef = useRef(searchParams);
  searchParamsRef.current = searchParams;

  const activeCategory = searchParams.get('categoryId') || 'all';
  const activeBrand = searchParams.get('brandId');
  const sortBy = searchParams.get('sort') || 'default';
  const onlyInStock = searchParams.get('inStock') === 'true';
  const minPriceUrl = Number(searchParams.get('minPriceCents')) / 100 || DEFAULT_PRICE_RANGE[0];
  const maxPriceUrl = Number(searchParams.get('maxPriceCents')) / 100 || DEFAULT_PRICE_RANGE[1];
  const priceRange: [number, number] = [minPriceUrl, maxPriceUrl];
  const colorKey = searchParams.get('colorId') || '';
  const activeColors = useMemo(() => colorKey.split(',').filter(Boolean), [colorKey]);
  const sizeKey = searchParams.get('size') || '';
  const activeSizeSystem = searchParams.get('sizeSystem');
  const activeSizes = useMemo(() => sizeKey.split(',').filter(Boolean), [sizeKey]);

  // Single writer: always starts from the LATEST params (ref is updated synchronously), so rapid or batched
  // changes never overwrite each other. User changes push one history entry; sanitising uses replace.
  const updateParams = (mutate: (p: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(searchParamsRef.current);
    mutate(next);
    if (next.toString() === searchParamsRef.current.toString()) return;
    searchParamsRef.current = next;
    setSearchParams(next, { replace });
  };
  const setParam = (p: URLSearchParams, key: string, value: string | null) => {
    if (value) p.set(key, value); else p.delete(key);
  };
  const setActiveBrand = (id: string | null) => updateParams((p) => setParam(p, 'brandId', id));
  const setSortBy = (value: string) => updateParams((p) => setParam(p, 'sort', value && value !== 'default' ? value : null));
  const setOnlyInStock = (next: boolean) => updateParams((p) => setParam(p, 'inStock', next ? 'true' : null));
  const setPriceRange = ([min, max]: [number, number]) =>
    updateParams((p) => {
      setParam(p, 'minPriceCents', min > DEFAULT_PRICE_RANGE[0] ? String(min * 100) : null);
      setParam(p, 'maxPriceCents', max < DEFAULT_PRICE_RANGE[1] ? String(max * 100) : null);
    });

  // Price inputs keep a local draft while typing and commit ONE history entry after the user pauses.
  const [priceDraft, setPriceDraft] = useState<[string, string]>([
    minPriceUrl > DEFAULT_PRICE_RANGE[0] ? String(minPriceUrl) : '',
    maxPriceUrl < DEFAULT_PRICE_RANGE[1] ? String(maxPriceUrl) : '',
  ]);
  useEffect(() => {
    setPriceDraft([
      minPriceUrl > DEFAULT_PRICE_RANGE[0] ? String(minPriceUrl) : '',
      maxPriceUrl < DEFAULT_PRICE_RANGE[1] ? String(maxPriceUrl) : '',
    ]);
  }, [minPriceUrl, maxPriceUrl]);
  const priceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const editPrice = (index: 0 | 1, raw: string) => {
    const draft: [string, string] = index === 0 ? [raw, priceDraft[1]] : [priceDraft[0], raw];
    setPriceDraft(draft);
    if (priceTimer.current) clearTimeout(priceTimer.current);
    priceTimer.current = setTimeout(() => {
      setPriceRange([Number(draft[0]) || DEFAULT_PRICE_RANGE[0], Number(draft[1]) || DEFAULT_PRICE_RANGE[1]]);
    }, 500);
  };
  useEffect(() => () => { if (priceTimer.current) clearTimeout(priceTimer.current); }, []);

  const [showMobileFilters, setShowMobileFilters] = useState(false);
  const [apiProducts, setApiProducts] = useState<Product[]>([]);
  const [totalProducts, setTotalProducts] = useState(0);
  const [categories, setCategories] = useState<Category[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [colors, setColors] = useState<CatalogColor[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const PAGE_LIMIT = 24;

  const metaLoaded = useRef(false);
  const buildParams = useCallback(() => {
    const params: Record<string, any> = { limit: PAGE_LIMIT };
    if (sortBy && sortBy !== 'default') {
      params.sort = sortBy;
    }
    if (activeCategory !== 'all') params.categoryId = activeCategory;
    if (activeBrand) params.brandId = activeBrand;
    if (activeSizes.length > 0) {
      params.size = activeSizes.join(',');
      if (activeSizeSystem) params.sizeSystem = activeSizeSystem;
    }
    if (activeColors.length > 0) params.colorId = activeColors.join(',');
    if (onlyInStock) params.inStock = 'true';
    if (priceRange[0] > DEFAULT_PRICE_RANGE[0]) params.minPriceCents = priceRange[0] * 100;
    if (priceRange[1] < DEFAULT_PRICE_RANGE[1]) params.maxPriceCents = priceRange[1] * 100;
    const searchQuery = searchParams.get('q');
    if (searchQuery) params.q = searchQuery;
    return params;
  }, [activeCategory, activeBrand, activeSizes, activeSizeSystem, activeColors, onlyInStock, minPriceUrl, maxPriceUrl, sortBy, searchParams]);

  useEffect(() => {
    setOffset(0);
    setApiProducts([]);
    // Ignore responses of superseded requests (rapid filter toggling must not show stale results).
    let cancelled = false;

    async function loadProducts() {
      try {
        setIsLoading(true);
        const params = { ...buildParams(), offset: 0 };

        const [productsRes, apiCategories, apiBrands, apiColors] = metaLoaded.current
          ? [await fetchProducts(params, { isCustomer }), categories, brands, colors]
          : await Promise.all([
              fetchProducts(params, { isCustomer }),
              fetchCategories(),
              fetchBrands(),
              // the color list is optional: a failure must not break the catalog
              Promise.resolve().then(() => fetchColors()).catch(() => [] as CatalogColor[]),
            ]);

        if (cancelled) return;
        if (!metaLoaded.current) {
          setCategories(apiCategories as Category[]);
          setBrands(apiBrands as Brand[]);
          setColors(apiColors as CatalogColor[]);
          metaLoaded.current = true;
        }

        setApiProducts(productsRes.items);
        setTotalProducts(productsRes.totalCount);
        setHasMore(productsRes.items.length === PAGE_LIMIT && productsRes.totalCount > PAGE_LIMIT);
        setError(null);

      } catch (err) {
        if (cancelled) return;
        console.error('Failed to load products:', err);
        setError('Не удалось загрузить товары. Попробуйте позже.');
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }
    loadProducts();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeCategory, activeBrand, sizeKey, activeSizeSystem, colorKey, onlyInStock, minPriceUrl, maxPriceUrl, sortBy, searchParams.get('q'), isCustomer]);

  const loadMore = async () => {
    if (isLoadingMore || !hasMore) return;
    try {
      setIsLoadingMore(true);
      const newOffset = offset + PAGE_LIMIT;
      const params = { ...buildParams(), offset: newOffset };
      const res = await fetchProducts(params, { isCustomer });
      setApiProducts(prev => [...prev, ...res.items]);
      setOffset(newOffset);
      setHasMore(res.items.length === PAGE_LIMIT && (newOffset + PAGE_LIMIT) < res.totalCount);
    } catch (err) {
      console.error('Failed to load more products:', err);
    } finally {
      setIsLoadingMore(false);
    }
  };

  const categoryOptions: Category[] = [{ id: 'all', slug: 'all', name: 'Все товары', icon: '✦' }, ...categories];
  const hasActivePriceFilter = priceRange[0] !== DEFAULT_PRICE_RANGE[0] || priceRange[1] !== DEFAULT_PRICE_RANGE[1];
  const hasActiveFilters = activeCategory !== 'all' || activeBrand !== null || activeSizes.length > 0 || activeColors.length > 0 || onlyInStock || hasActivePriceFilter;
  const activeFiltersCount = (activeBrand ? 1 : 0) + (activeCategory !== 'all' ? 1 : 0) + (activeSizes.length > 0 ? 1 : 0) + (activeColors.length > 0 ? 1 : 0) + (onlyInStock ? 1 : 0) + (hasActivePriceFilter ? 1 : 0);

  const resetFilters = () =>
    updateParams((p) => {
      ['categoryId', 'brandId', 'size', 'sizeSystem', 'colorId', 'inStock', 'minPriceCents', 'maxPriceCents'].forEach((k) => p.delete(k));
    });

  // Size filter model: the current category's own system; for "all products" a few distinct systems, or a prompt
  // to pick a category when there are too many / indistinguishable ones. Names are never merged; identity is `code`.
  const sizeModel = useMemo(() => buildSizeFilterModel(categories, activeCategory), [activeCategory, categories]);
  const sizeGroups = sizeModel.view.groups;
  const validSizeSystems = sizeModel.validSystems;

  const writeSizes = (sizes: string[], system: string | null, replace: boolean) =>
    updateParams((p) => {
      if (sizes.length > 0 && system) {
        p.set('size', sizes.join(','));
        p.set('sizeSystem', system);
      } else {
        p.delete('size');
        p.delete('sizeSystem');
      }
    }, replace);

  const clearSizes = (replace = false) => writeSizes([], null, replace);

  const readList = (p: URLSearchParams, key: string) => (p.get(key) || '').split(',').filter(Boolean);
  // Toggles read the LATEST params inside the writer, so rapid consecutive clicks never lose an update.
  const writeColors = (next: string[]) => updateParams((p) => setParam(p, 'colorId', next.length > 0 ? next.join(',') : null));
  const toggleColor = (id: string) =>
    updateParams((p) => {
      const cur = readList(p, 'colorId');
      setParam(p, 'colorId', (cur.includes(id) ? cur.filter((c) => c !== id) : [...cur, id]).join(',') || null);
    });
  const removeColor = (id: string) =>
    updateParams((p) => setParam(p, 'colorId', readList(p, 'colorId').filter((c) => c !== id).join(',') || null));
  const colorName = (id: string) => colors.find((c) => c.id === id)?.name ?? 'Цвет';

  const toggleSize = (systemCode: string, size: string) =>
    updateParams((p) => {
      // Selecting inside another system replaces the selection: systems are never mixed.
      const base = p.get('sizeSystem') === systemCode ? readList(p, 'size') : [];
      const picked = base.includes(size) ? base.filter((s) => s !== size) : [...base, size];
      // canonical order = the system's own size order, so URLs are stable regardless of click order
      const order = validSizeSystems.find((g) => g.code === systemCode)?.values.map((v) => v.value) ?? [];
      const next = [...picked].sort((a, b) => order.indexOf(a) - order.indexOf(b));
      if (next.length > 0) {
        p.set('size', next.join(','));
        p.set('sizeSystem', systemCode);
      } else {
        p.delete('size');
        p.delete('sizeSystem');
      }
    });

  const removeSize = (size: string) =>
    updateParams((p) => {
      const next = readList(p, 'size').filter((s) => s !== size);
      if (next.length > 0) p.set('size', next.join(',')); else { p.delete('size'); p.delete('sizeSystem'); }
    });

  // Category change is ONE URL write (one history entry): category + only the sizes valid for it.
  const changeCategory = (categoryId: string) =>
    updateParams((p) => {
      setParam(p, 'categoryId', categoryId === 'all' ? null : categoryId);
      if (categoryId === 'all' || activeSizes.length === 0) return;
      const sys = sanitizeSizeSystem(categories.find((c) => c.id === categoryId)?.sizeSystem);
      const allowed = sys && sys.code === activeSizeSystem ? new Set(sys.values.map((v) => v.value)) : null;
      const valid = allowed ? activeSizes.filter((v) => allowed.has(v)) : [];
      if (sys && valid.length > 0) {
        p.set('size', valid.join(','));
        p.set('sizeSystem', sys.code);
      } else {
        p.delete('size');
        p.delete('sizeSystem');
      }
    });

  // A crafted/stale URL must not keep sizes that don't exist for the current category.
  useEffect(() => {
    if (!metaLoaded.current || activeSizes.length === 0) return;
    const group = validSizeSystems.find((g) => g.code === activeSizeSystem);
    if (!group) return clearSizes(true);
    const allowed = new Set(group.values.map((v) => v.value));
    const valid = activeSizes.filter((v) => allowed.has(v));
    if (valid.length !== activeSizes.length) writeSizes(valid, activeSizeSystem, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [validSizeSystems, sizeKey, activeSizeSystem, categories.length]);

  const FiltersContent = () => (
    <div className="flex flex-col px-1 pb-4">
      {/* Категории */}
      <FilterSection title="Категории" defaultOpen={activeCategory !== 'all'}>
        <div className={cn("flex flex-col gap-1", LIST_SCROLL)}>
          {categories.length === 0 ? (
            <p className="py-2 text-xs text-black/45 dark:text-white/45">Категории пока не добавлены</p>
          ) : (
            categoryOptions.map((category) => (
              <FilterCheckbox
                key={category.id}
                label={category.name}
                count={category.count}
                isActive={activeCategory === category.id}
                onClick={() => changeCategory(category.id)}
              />
            ))
          )}
        </div>
      </FilterSection>

      {/* Размеры */}
      <FilterSection title="Размер" defaultOpen>
        {sizeModel.view.kind === 'prompt' ? (
          <div className="flex flex-col gap-2" data-testid="size-filter-prompt">
            <p className="py-1 text-xs leading-relaxed text-black/55 dark:text-white/55">
              Выберите категорию, чтобы увидеть доступные размеры
            </p>
            {activeSizes.length > 0 && (
              <button type="button" onClick={() => clearSizes()} className="self-start text-[11px] underline text-black/60 dark:text-white/60">
                Сбросить размеры
              </button>
            )}
          </div>
        ) : sizeGroups.length === 0 ? (
          <p className="py-1 text-xs text-black/45 dark:text-white/45" data-testid="size-filter-empty">
            Для этой категории размерная сетка не задана
          </p>
        ) : (
          <div className="flex flex-col gap-4" data-testid="size-filter">
            {sizeGroups.map((group) => (
              <div key={group.code} className="min-w-0">
                {sizeGroups.length > 1 && (
                  <p title={group.name} className="mb-2 truncate text-[10px] font-mono uppercase tracking-widest text-black/40 dark:text-white/40">{group.name}</p>
                )}
                <div className="grid grid-cols-4 gap-2">
                  {group.values.map(({ value }) => {
                    const isActive = group.code === activeSizeSystem && activeSizes.includes(value);
                    return (
                      <button
                        key={`${group.code}-${value}`}
                        type="button"
                        aria-pressed={isActive}
                        title={value}
                        data-testid={`size-filter-${group.code}-${value}`}
                        onClick={() => toggleSize(group.code, value)}
                        className={cn(
                          "h-8 min-w-0 px-1 border text-[11px] font-mono transition-all flex items-center justify-center rounded-[2px]",
                          isActive
                            ? "bg-black text-white border-black dark:bg-white dark:text-black dark:border-white"
                            : "bg-transparent border-black/20 dark:border-white/20 text-black/60 dark:text-white/60 hover:border-black/50 dark:hover:border-white/50 hover:text-black dark:hover:text-white"
                        )}
                      >
                        <span className="truncate">{value}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
            {activeSizes.length > 0 && (
              <button type="button" onClick={() => clearSizes()} className="self-start text-[11px] underline text-black/60 dark:text-white/60">
                Сбросить размеры
              </button>
            )}
          </div>
        )}
      </FilterSection>

      {/* Цвет */}
      <FilterSection title="Цвет" defaultOpen>
        {colors.length === 0 ? (
          <p className="py-1 text-xs text-black/45 dark:text-white/45" data-testid="color-filter-empty">Цвета пока недоступны</p>
        ) : (
          <div className={cn("flex flex-col gap-1", LIST_SCROLL)} data-testid="color-filter">
            {colors.map((color) => {
              const isActive = activeColors.includes(color.id);
              return (
                <button
                  key={color.id}
                  type="button"
                  aria-pressed={isActive}
                  data-testid={`color-filter-${color.id}`}
                  onClick={() => toggleColor(color.id)}
                  className="flex items-center gap-3 w-full py-1.5 text-left group"
                >
                  <span
                    aria-hidden="true"
                    className={cn(
                      "w-4 h-4 rounded-full border shrink-0 flex items-center justify-center",
                      isActive ? "border-black dark:border-white ring-1 ring-black dark:ring-white ring-offset-1 ring-offset-white dark:ring-offset-black" : "border-black/25 dark:border-white/30",
                      !color.hex && "bg-transparent"
                    )}
                    style={color.hex ? { backgroundColor: color.hex } : undefined}
                  />
                  <span title={color.name} className={cn(
                    "text-[12px] font-sans font-light tracking-wide transition-colors truncate min-w-0",
                    isActive ? "text-black dark:text-white font-normal" : "text-black/70 dark:text-white/70 group-hover:text-black dark:group-hover:text-white"
                  )}>
                    {color.name}
                  </span>
                  {isActive && <Check className="w-3.5 h-3.5 ml-auto text-black dark:text-white" strokeWidth={2.5} />}
                </button>
              );
            })}
            {activeColors.length > 0 && (
              <button type="button" onClick={() => writeColors([])} className="self-start mt-1 text-[11px] underline text-black/60 dark:text-white/60">
                Сбросить цвета
              </button>
            )}
          </div>
        )}
      </FilterSection>

      {/* Продавцы */}
      <FilterSection title="Продавец" defaultOpen={activeBrand !== null}>
        <div className={cn("flex flex-col gap-1", LIST_SCROLL)}>
          <FilterCheckbox
            label="Все продавцы"
            isActive={activeBrand === null}
            onClick={() => setActiveBrand(null)}
          />
          {brands.length === 0 ? (
            <p className="py-2 text-xs text-black/45 dark:text-white/45">Бренды пока не добавлены</p>
          ) : (
            brands.map((brand) => (
              <FilterCheckbox
                key={brand.id}
                label={brand.name}
                isActive={activeBrand === brand.id}
                onClick={() => setActiveBrand(brand.id)}
              />
            ))
          )}
        </div>
      </FilterSection>

      {/* Цена */}
      <FilterSection title="Цена" defaultOpen={hasActivePriceFilter}>
        <div className="flex items-center justify-between gap-2 mt-1">
          <div className="relative flex-1">
            <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[9px] uppercase font-mono text-black/40 dark:text-white/40">От</span>
            <input
              type="number"
              value={priceDraft[0]}
              onChange={(e) => editPrice(0, e.target.value)}
              className="w-full h-8 pl-8 pr-2 rounded-[2px] border border-black/20 dark:border-white/20 bg-transparent text-[12px] font-mono text-black dark:text-white transition-colors focus:border-black/60 dark:focus:border-white/60 outline-none"
            />
          </div>
          <div className="w-2 h-px bg-black/20 dark:bg-white/20" />
          <div className="relative flex-1">
            <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[9px] uppercase font-mono text-black/40 dark:text-white/40">До</span>
            <input
              type="number"
              value={priceDraft[1]}
              onChange={(e) => editPrice(1, e.target.value)}
              className="w-full h-8 pl-8 pr-2 rounded-[2px] border border-black/20 dark:border-white/20 bg-transparent text-[12px] font-mono text-black dark:text-white transition-colors focus:border-black/60 dark:focus:border-white/60 outline-none"
            />
          </div>
        </div>
      </FilterSection>

      {/* В наличии */}
      <FilterSection title="Наличие" defaultOpen={onlyInStock}>
        <button
          onClick={() => setOnlyInStock(!onlyInStock)}
          className="flex items-center gap-3 py-1.5 group text-left w-full"
        >
          <div className="w-3.5 h-3.5 flex items-center justify-center shrink-0">
            {onlyInStock && <Check className="w-4 h-4 text-black dark:text-white" strokeWidth={2.5} />}
          </div>
          <span className={cn(
            "text-[12px] font-sans font-light tracking-wide transition-colors",
            onlyInStock ? "text-black dark:text-white font-normal" : "text-black/70 dark:text-white/70 group-hover:text-black dark:group-hover:text-white"
          )}>
            Только в наличии
          </span>
        </button>
      </FilterSection>

      {/* Кнопка сброса */}
      {hasActiveFilters && (
        <div className="mt-6">
          <Button
            variant="outline"
            onClick={resetFilters}
            className="w-full h-10 text-xs font-mono uppercase tracking-widest border-dashed border-black/20 dark:border-white/20"
          >
            Сбросить фильтры
          </Button>
        </div>
      )}

      {/* Технический штамп / подвал (Archivecore) */}
      <div className="mt-8 pt-4 border-t border-black/10 dark:border-white/10 opacity-30 select-none pointer-events-none flex flex-col gap-1">
        <span className="text-[10px] font-mono uppercase font-semibold text-black dark:text-white">ARCH.REF.NZ001</span>
        <span className="text-[8px] font-mono uppercase tracking-[0.2em] text-black dark:text-white">Система фильтрации {new Date().getFullYear()}</span>
        <div className="w-16 h-[1px] bg-black dark:bg-white mt-1"></div>
      </div>
    </div>
  );

  return (
    <div className="relative z-10 min-h-screen pt-24 md:pt-28 pb-20 bg-background text-foreground transition-colors duration-300">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">
        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-sm text-ash mb-6">
          <Link to="/" className="hover:text-graphite dark:hover:text-white transition-colors">Главная</Link>
          <ChevronRight className="w-4 h-4" />
          <span className="text-graphite dark:text-white">Каталог</span>
        </nav>

        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
          <div>
            <h1 className="text-[1.75rem] md:text-[2rem] font-serif text-graphite dark:text-white leading-tight">
              Каталог
            </h1>
            <p className="text-sm text-ash mt-1">{totalProducts} товаров</p>
          </div>

          <div className="flex items-center gap-3">
            {/* Кнопка фильтров на мобильном */}
            <button
              className="lg:hidden inline-flex items-center gap-2 h-10 px-4 rounded-lg border border-border-lighter dark:border-white/20 text-sm text-graphite dark:text-white"
              onClick={() => setShowMobileFilters(true)}
            >
              <SlidersHorizontal className="w-4 h-4" />
              Фильтры
              {hasActiveFilters && <span className="w-5 h-5 rounded-full bg-graphite text-white dark:text-black text-xs flex items-center justify-center">{activeFiltersCount}</span>}
            </button>

            <SortDropdown value={sortBy} options={SORT_OPTIONS} onChange={setSortBy} />
          </div>
        </div>

        {/* Main content */}
        <div className="flex gap-10">
          {/* Desktop Sidebar Filters */}
          <aside className="hidden lg:block w-[280px] flex-shrink-0 relative">
            <div className="sticky top-28 flex flex-col max-h-[calc(100vh-120px)] bg-white/5 dark:bg-[#111111]/5 backdrop-blur-[24px] rounded-[24px] border-[1px] border-white/20 dark:border-white/10 shadow-[0_8px_32px_rgba(0,0,0,0.04)] overflow-hidden">
              <div className="flex items-center justify-between px-6 pt-6 pb-4 shrink-0 z-10 border-b border-black/5 dark:border-white/5">
                <h2 className="text-[14px] font-mono tracking-widest uppercase font-semibold text-black dark:text-white">Фильтры</h2>
                {hasActiveFilters && (
                  <button type="button" onClick={resetFilters} className="text-[10px] font-mono uppercase tracking-widest text-black/50 hover:text-black dark:text-white/50 dark:hover:text-white transition-colors">
                    Сбросить
                  </button>
                )}
              </div>
              <div className="px-6 pb-6 pt-2 overflow-y-auto [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-black/10 dark:[&::-webkit-scrollbar-thumb]:bg-white/10 [&::-webkit-scrollbar-track]:bg-transparent">
                {FiltersContent()}
              </div>
            </div>
          </aside>

          {/* Products Grid */}
          <div className="flex-1 min-w-0">
            {/* Active filters tags */}
            {hasActiveFilters && (
              <div className="flex flex-wrap items-center gap-2 mb-6">
                {activeCategory !== 'all' && (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {categoryOptions.find(c => c.id === activeCategory)?.name}
                    <button type="button" aria-label="Снять фильтр категории" onClick={() => changeCategory('all')} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                )}
                {activeBrand && (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {brands.find(b => b.id === activeBrand)?.name}
                    <button type="button" aria-label="Снять фильтр бренда" onClick={() => setActiveBrand(null)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                )}
                {activeSizes.map((size) => (
                  <span key={size} data-testid={`size-tag-${size}`} className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {size}
                    <button type="button" aria-label={`Снять фильтр размера ${size}`} onClick={() => removeSize(size)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                ))}
                {activeColors.map((id) => (
                  <span key={id} data-testid={`color-tag-${id}`} className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {colorName(id)}
                    <button type="button" aria-label={`Снять фильтр цвета ${colorName(id)}`} onClick={() => removeColor(id)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                ))}
                {onlyInStock && (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    В наличии
                    <button type="button" aria-label="Снять фильтр наличия" onClick={() => setOnlyInStock(false)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                )}
                <Button variant="ghost" size="sm" onClick={resetFilters} className="text-xs ml-2">Сбросить все</Button>
              </div>
            )}

            {isLoading ? (
              <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-3 gap-4 md:gap-5">
                {[...Array(6)].map((_, i) => (
                  <div key={i} className="animate-pulse flex flex-col gap-2">
                    <div className="w-full aspect-[3/4] bg-black/5 dark:bg-white/5 rounded-2xl"></div>
                    <div className="h-4 bg-black/5 dark:bg-white/5 rounded w-3/4"></div>
                    <div className="h-4 bg-black/5 dark:bg-white/5 rounded w-1/2"></div>
                  </div>
                ))}
              </div>
            ) : error ? (
              <div className="text-center py-16 px-4">
                <h3 className="text-xl font-serif text-error mb-2">Ошибка</h3>
                <p className="text-sm text-ash">{error}</p>
                <Button variant="outline" onClick={() => window.location.reload()} className="mt-4">Обновить страницу</Button>
              </div>
            ) : apiProducts.length > 0 ? (
              <>
                <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-3 gap-4 md:gap-5">
                  {apiProducts.map((product) => (
                    <ProductCard key={product.id} product={product} placement="catalog_grid" />
                  ))}
                </div>
                {hasMore && (
                  <div className="flex justify-center mt-10">
                    <Button
                      variant="outline"
                      onClick={loadMore}
                      disabled={isLoadingMore}
                      className="px-8"
                    >
                      {isLoadingMore ? 'Загрузка...' : 'Показать ещё'}
                    </Button>
                  </div>
                )}
              </>
            ) : (
              <div className="text-center py-16 px-4 border border-dashed border-black/10 dark:border-white/10 rounded-2xl">
                <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-ice dark:bg-white/10 flex items-center justify-center">
                  <SlidersHorizontal className="w-7 h-7 text-ash" />
                </div>
                <h3 className="text-xl font-serif text-graphite dark:text-white mb-2">Ничего не найдено</h3>
                <p className="text-sm text-ash mb-6">Попробуйте изменить параметры фильтрации</p>
                <Button
                  onClick={resetFilters}
                >
                  Сбросить фильтры
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Mobile Filters Drawer */}
      <Drawer isOpen={showMobileFilters} onClose={() => setShowMobileFilters(false)} title="Фильтры">
        <div className="pb-20 bg-white/80 dark:bg-black/40 backdrop-blur-xl">
          {FiltersContent()}
        </div>
        <div className="fixed bottom-0 left-0 right-0 p-4 bg-white/90 dark:bg-black/90 backdrop-blur-xl border-t border-black/10 dark:border-white/10">
          <Button
            onClick={() => setShowMobileFilters(false)}
            className="w-full h-10"
          >
            Показать {totalProducts} товаров
          </Button>
        </div>
      </Drawer>
    </div>
  );
}

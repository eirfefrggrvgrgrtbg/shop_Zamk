import { useState, useMemo } from 'react';
import { SlidersHorizontal, ChevronDown } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Drawer } from '../components/ui/Drawer';
import { PRODUCTS, CATEGORIES, BRANDS } from '../lib/mock-data';

const SORT_OPTIONS = [
  { value: 'new', label: 'Сначала новые' },
  { value: 'price-asc', label: 'Цена: по возрастанию' },
  { value: 'price-desc', label: 'Цена: по убыванию' },
  { value: 'name', label: 'По названию' },
];

export function Catalog() {
  const [searchQuery, setSearchQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState('all');
  const [activeBrand, setActiveBrand] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState('new');
  const [showSort, setShowSort] = useState(false);
  const [showFilters, setShowFilters] = useState(false);

  const filteredProducts = useMemo(() => {
    let result = [...PRODUCTS];

    // Search
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      result = result.filter(p =>
        p.name.toLowerCase().includes(q) ||
        p.brand.toLowerCase().includes(q)
      );
    }

    // Category
    if (activeCategory !== 'all') {
      result = result.filter(p => p.category === activeCategory);
    }

    // Brand
    if (activeBrand) {
      result = result.filter(p => p.brandId === activeBrand);
    }

    // Sort
    switch (sortBy) {
      case 'price-asc': result.sort((a, b) => a.price - b.price); break;
      case 'price-desc': result.sort((a, b) => b.price - a.price); break;
      case 'name': result.sort((a, b) => a.name.localeCompare(b.name)); break;
      case 'new': result.sort((a, b) => (b.isNew ? 1 : 0) - (a.isNew ? 1 : 0)); break;
    }

    return result;
  }, [searchQuery, activeCategory, activeBrand, sortBy]);

  const currentSort = SORT_OPTIONS.find(s => s.value === sortBy);

  return (
    <div className="min-h-screen relative z-10 pt-28 pb-16">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="studio-shell p-6 md:p-8 mb-8">
          <p className="studio-label mb-2">Каталог ZAMK</p>
          <h1 className="studio-title mb-3">Кураторская выборка независимых брендов</h1>
          <p className="studio-subtitle max-w-3xl">
            Одежда, обувь, сумки и аксессуары в светлой cold-premium эстетике.
            Чистая витрина без шума массового ритейла.
          </p>
        </section>

        <div className="mb-6">
          <Input
            isSearch
            placeholder="Поиск по каталогу и брендам"
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
          />
        </div>

        <div className="flex gap-2 overflow-x-auto pb-2 mb-6 scrollbar-hide">
          {CATEGORIES.map(cat => (
            <Button
              key={cat.id}
              variant={activeCategory === cat.id ? 'primary' : 'pill'}
              size="sm"
              onClick={() => setActiveCategory(cat.id)}
              className="shrink-0"
            >
              {cat.name}
            </Button>
          ))}
        </div>

        <div className="flex items-center justify-between mb-6 gap-3">
          <div className="hidden md:flex gap-2 overflow-x-auto">
            <Button
              variant={activeBrand === null ? 'secondary' : 'pill'}
              size="sm"
              onClick={() => setActiveBrand(null)}
            >
              Все бренды
            </Button>
            {BRANDS.slice(0, 5).map(brand => (
              <Button
                key={brand.id}
                variant={activeBrand === brand.id ? 'primary' : 'pill'}
                size="sm"
                onClick={() => setActiveBrand(brand.id)}
                className="shrink-0"
              >
                {brand.name}
              </Button>
            ))}
          </div>

          <Button
            variant="secondary"
            size="sm"
            className="md:hidden gap-1.5"
            onClick={() => setShowFilters(true)}
          >
            <SlidersHorizontal className="w-3.5 h-3.5" />
            Фильтры
          </Button>

          <div className="relative">
            <Button
              variant="secondary"
              size="sm"
              className="gap-1.5"
              onClick={() => setShowSort(!showSort)}
            >
              {currentSort?.label}
              <ChevronDown className="w-3.5 h-3.5" />
            </Button>

            {showSort && (
              <>
                <div className="fixed inset-0 z-30" onClick={() => setShowSort(false)} />
                <div className="absolute right-0 top-full mt-2 w-56 glass-panel-strong py-2 z-40 overflow-hidden rounded-2xl">
                  {SORT_OPTIONS.map(opt => (
                    <button
                      key={opt.value}
                      className={`w-full text-left px-4 py-2.5 text-sm transition-colors ${
                        sortBy === opt.value ? 'text-primary font-medium bg-primary/5' : 'text-graphite hover:bg-surface'
                      }`}
                      onClick={() => { setSortBy(opt.value); setShowSort(false); }}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-[280px,1fr] gap-6 lg:gap-8 items-start">
          <aside className="hidden lg:block sticky top-24 studio-shell p-5">
            <p className="studio-label mb-4">Фильтры</p>
            <div className="space-y-6">
              <div>
                <p className="text-xs uppercase tracking-[0.14em] text-ash mb-3">Категории</p>
                <div className="space-y-1.5">
                  {CATEGORIES.map(cat => (
                    <button
                      key={cat.id}
                      className={`w-full text-left px-3 py-2 rounded-xl text-sm transition-colors ${
                        activeCategory === cat.id ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-primary-soft/60'
                      }`}
                      onClick={() => setActiveCategory(cat.id)}
                    >
                      {cat.name}
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <p className="text-xs uppercase tracking-[0.14em] text-ash mb-3">Бренды</p>
                <div className="space-y-1.5 max-h-72 overflow-y-auto pr-1">
                  <button
                    className={`w-full text-left px-3 py-2 rounded-xl text-sm transition-colors ${
                      !activeBrand ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-primary-soft/60'
                    }`}
                    onClick={() => setActiveBrand(null)}
                  >
                    Все бренды
                  </button>
                  {BRANDS.map(brand => (
                    <button
                      key={brand.id}
                      className={`w-full text-left px-3 py-2 rounded-xl text-sm transition-colors ${
                        activeBrand === brand.id ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-primary-soft/60'
                      }`}
                      onClick={() => setActiveBrand(brand.id)}
                    >
                      {brand.name}
                    </button>
                  ))}
                </div>
              </div>

              <Button variant="secondary" className="w-full" onClick={() => { setSearchQuery(''); setActiveCategory('all'); setActiveBrand(null); }}>
                Сбросить фильтры
              </Button>
            </div>
          </aside>

          <div>
            <div className="flex items-center justify-between mb-4">
              <p className="text-sm text-ash">Найдено: {filteredProducts.length} товаров</p>
            </div>

            {filteredProducts.length > 0 ? (
              <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-3 gap-4 md:gap-6">
                {filteredProducts.map(product => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            ) : (
              <div className="studio-shell flex flex-col items-center justify-center py-20 text-center">
                <div className="w-16 h-16 rounded-2xl bg-surface flex items-center justify-center mb-4">
                  <span className="text-2xl">🔍</span>
                </div>
                <h3 className="text-lg font-semibold text-graphite mb-2">Ничего не найдено</h3>
                <p className="text-sm text-ash mb-4">Попробуйте изменить критерии поиска</p>
                <Button variant="secondary" onClick={() => { setSearchQuery(''); setActiveCategory('all'); setActiveBrand(null); }}>
                  Сбросить фильтры
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>

      <Drawer isOpen={showFilters} onClose={() => setShowFilters(false)} title="Фильтры" position="left">
        <div className="space-y-6">
          <div>
            <h4 className="font-semibold text-sm text-graphite mb-3">Категория</h4>
            <div className="flex flex-col gap-1.5">
              {CATEGORIES.map(cat => (
                <button
                  key={cat.id}
                  className={`text-left px-3 py-2 rounded-xl text-sm transition-all ${
                    activeCategory === cat.id ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-surface'
                  }`}
                  onClick={() => setActiveCategory(cat.id)}
                >
                  {cat.name}
                </button>
              ))}
            </div>
          </div>
          <hr className="border-border-lighter" />
          <div>
            <h4 className="font-semibold text-sm text-graphite mb-3">Бренд</h4>
            <div className="flex flex-col gap-1.5">
              <button
                className={`text-left px-3 py-2 rounded-xl text-sm transition-all ${
                  !activeBrand ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-surface'
                }`}
                onClick={() => setActiveBrand(null)}
              >
                Все бренды
              </button>
              {BRANDS.map(brand => (
                <button
                  key={brand.id}
                  className={`text-left px-3 py-2 rounded-xl text-sm transition-all ${
                    activeBrand === brand.id ? 'bg-primary-soft text-primary font-medium' : 'text-graphite hover:bg-surface'
                  }`}
                  onClick={() => setActiveBrand(brand.id)}
                >
                  {brand.name}
                </button>
              ))}
            </div>
          </div>
          <Button variant="primary" className="w-full mt-4" onClick={() => setShowFilters(false)}>
            Показать {filteredProducts.length} товаров
          </Button>
        </div>
      </Drawer>
    </div>
  );
}

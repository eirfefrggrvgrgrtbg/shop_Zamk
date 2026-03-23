import { useMemo, useState } from 'react';
import { ProductCard } from '../components/product/ProductCard';
import { Drawer } from '../components/ui/Drawer';
import { FilterGroup, PillFilter, SearchField, SortDropdown } from '../components/editorial/StudioKit';
import { BRANDS, CATEGORIES, PRODUCTS } from '../lib/mock-data';

const SORT_OPTIONS = [
  { value: 'new', label: 'Сначала новые' },
  { value: 'price-asc', label: 'Цена по возрастанию' },
  { value: 'price-desc', label: 'Цена по убыванию' },
  { value: 'name', label: 'По названию' },
];

export function Catalog() {
  const [searchQuery, setSearchQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState('all');
  const [activeBrand, setActiveBrand] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState('new');
  const [showFilters, setShowFilters] = useState(false);

  const filteredProducts = useMemo(() => {
    let result = [...PRODUCTS];

    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter((item) => item.name.toLowerCase().includes(query) || item.brand.toLowerCase().includes(query));
    }

    if (activeCategory !== 'all') {
      result = result.filter((item) => item.category === activeCategory);
    }

    if (activeBrand) {
      result = result.filter((item) => item.brandId === activeBrand);
    }

    if (sortBy === 'price-asc') result.sort((a, b) => a.price - b.price);
    if (sortBy === 'price-desc') result.sort((a, b) => b.price - a.price);
    if (sortBy === 'name') result.sort((a, b) => a.name.localeCompare(b.name));
    if (sortBy === 'new') result.sort((a, b) => Number(b.isNew) - Number(a.isNew));

    return result;
  }, [searchQuery, activeCategory, activeBrand, sortBy]);

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className='overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 dark:border-white/10 dark:bg-white/5 backdrop-blur-sm'>
          <div className='relative h-[220px] md:h-[280px]'>
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(164,193,223,0.56),transparent_50%),radial-gradient(ellipse_at_58%_50%,rgba(197,217,238,0.72),transparent_56%),radial-gradient(ellipse_at_86%_52%,rgba(167,196,225,0.58),transparent_52%)] dark:opacity-20' />
            <div className='absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.76),rgba(236,242,249,0.66))] dark:hidden' />
            <div className='relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7'>
              <h2 className='font-serif text-[clamp(2.8rem,8vw,8.3rem)] text-white/43 dark:text-white/20 leading-[0.8] tracking-[-0.03em]'>КАТАЛОГ</h2>
              <h3 className='font-serif text-[clamp(2.4rem,6.8vw,7.4rem)] text-white/42 dark:text-white/20 leading-[0.82] tracking-[-0.03em] text-center'>
                НОВАЯ
                <br />
                ВОЛНА
              </h3>
              <h4 className='font-serif text-[clamp(2.4rem,7vw,7.8rem)] text-white/42 dark:text-white/20 leading-[0.8] tracking-[-0.03em] text-right'>АРХИВ</h4>
            </div>
          </div>
        </section>

        <div className='mt-10 space-y-8'>
          <SearchField
            placeholder='Поиск по бренду или названию товара'
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
          />

          <section className='border-t border-b border-border-lighter dark:border-white/10 bg-white/46 dark:bg-white/5 px-4 md:px-6 py-4'>
            <div className='flex flex-wrap items-center gap-2 md:gap-3'>
              {CATEGORIES.map((category) => (
                <PillFilter
                  key={category.id}
                  label={category.name}
                  active={activeCategory === category.id}
                  onClick={() => setActiveCategory(category.id)}
                />
              ))}
              <div className='ml-auto'>
                <SortDropdown value={sortBy} options={SORT_OPTIONS} onChange={setSortBy} />
              </div>
            </div>

            <div className='mt-6 grid grid-cols-1 lg:grid-cols-3 gap-4'>
              <FilterGroup title='Бренды'>
                <div className='flex flex-wrap gap-2'>
                  <PillFilter label='Все бренды' active={activeBrand === null} onClick={() => setActiveBrand(null)} />
                  {BRANDS.map((brand) => (
                    <PillFilter
                      key={brand.id}
                      label={brand.name}
                      active={activeBrand === brand.id}
                      onClick={() => setActiveBrand(brand.id)}
                    />
                  ))}
                </div>
              </FilterGroup>

              <FilterGroup title='Режим показа'>
                <p className='text-sm text-graphite-light leading-relaxed'>
                  Показано {filteredProducts.length} позиций. На мобильных фильтры можно открыть в отдельной панели.
                </p>
                <button
                  className='mt-3 inline-flex h-10 items-center rounded-full border border-border-soft px-5 text-sm text-graphite lg:hidden'
                  onClick={() => setShowFilters(true)}
                >
                  Открыть фильтры
                </button>
              </FilterGroup>

              <FilterGroup title='Сброс параметров'>
                <button
                  className='inline-flex h-10 items-center rounded-full bg-graphite px-5 text-sm text-white'
                  onClick={() => {
                    setSearchQuery('');
                    setActiveCategory('all');
                    setActiveBrand(null);
                  }}
                >
                  Сбросить всё
                </button>
              </FilterGroup>
            </div>
          </section>

          {filteredProducts.length > 0 ? (
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6'>
              {filteredProducts.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          ) : (
            <section className='glass-panel-strong p-10 text-center'>
              <h3 className='text-3xl font-serif text-graphite'>Ничего не найдено</h3>
              <p className='mt-3 text-sm text-graphite-light'>Измени параметры поиска или сбрось фильтры.</p>
            </section>
          )}
        </div>
      </div>

      <Drawer isOpen={showFilters} onClose={() => setShowFilters(false)} title='Фильтры'>
        <div className='space-y-4'>
          <FilterGroup title='Категории'>
            <div className='space-y-2'>
              {CATEGORIES.map((category) => (
                <button
                  key={category.id}
                  className={`block w-full rounded-2xl px-4 py-2 text-left text-sm ${
                    activeCategory === category.id ? 'bg-ice text-graphite font-medium' : 'text-graphite-light hover:bg-milk'
                  }`}
                  onClick={() => setActiveCategory(category.id)}
                >
                  {category.name}
                </button>
              ))}
            </div>
          </FilterGroup>

          <FilterGroup title='Бренды'>
            <div className='space-y-2'>
              <button
                className={`block w-full rounded-2xl px-4 py-2 text-left text-sm ${
                  activeBrand === null ? 'bg-ice text-graphite font-medium' : 'text-graphite-light hover:bg-milk'
                }`}
                onClick={() => setActiveBrand(null)}
              >
                Все бренды
              </button>
              {BRANDS.map((brand) => (
                <button
                  key={brand.id}
                  className={`block w-full rounded-2xl px-4 py-2 text-left text-sm ${
                    activeBrand === brand.id ? 'bg-ice text-graphite font-medium' : 'text-graphite-light hover:bg-milk'
                  }`}
                  onClick={() => setActiveBrand(brand.id)}
                >
                  {brand.name}
                </button>
              ))}
            </div>
          </FilterGroup>
        </div>
      </Drawer>
    </div>
  );
}

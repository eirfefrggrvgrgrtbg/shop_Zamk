import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight, SlidersHorizontal, X } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Drawer } from '../components/ui/Drawer';
import { SortDropdown } from '../components/editorial/StudioKit';
import { BRANDS, CATEGORIES, PRODUCTS } from '../lib/mock-data';
import { cn } from '../lib/utils';

const SORT_OPTIONS = [
  { value: 'new', label: 'Сначала новые' },
  { value: 'price-asc', label: 'Цена по возрастанию' },
  { value: 'price-desc', label: 'Цена по убыванию' },
  { value: 'popular', label: 'По популярности' },
];

const SIZES = ['XS', 'S', 'M', 'L', 'XL', 'XXL'];
const COLORS = [
  { name: 'Чёрный', hex: '#1a1a1a' },
  { name: 'Белый', hex: '#ffffff' },
  { name: 'Серый', hex: '#8c8c8c' },
  { name: 'Бежевый', hex: '#d4c4b0' },
  { name: 'Синий', hex: '#2d4a6f' },
  { name: 'Коричневый', hex: '#6b4423' },
];
const MATERIALS = ['Хлопок', 'Шерсть', 'Лён', 'Кашемир', 'Полиэстер', 'Шёлк'];

// Компонент раскрывающейся секции фильтра
function FilterSection({ title, children, defaultOpen = true }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-border-lighter dark:border-white/10 py-4">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between text-left"
      >
        <span className="text-sm font-medium text-graphite dark:text-white">{title}</span>
        <ChevronDown className={cn("w-4 h-4 text-ash transition-transform", isOpen && "rotate-180")} />
      </button>
      {isOpen && <div className="mt-3">{children}</div>}
    </div>
  );
}

export function Catalog() {
  const [activeCategory, setActiveCategory] = useState('all');
  const [activeBrand, setActiveBrand] = useState<string | null>(null);
  const [activeSizes, setActiveSizes] = useState<string[]>([]);
  const [activeColors, setActiveColors] = useState<string[]>([]);
  const [activeMaterials, setActiveMaterials] = useState<string[]>([]);
  const [priceRange, setPriceRange] = useState<[number, number]>([0, 100000]);
  const [sortBy, setSortBy] = useState('new');
  const [showMobileFilters, setShowMobileFilters] = useState(false);

  const hasActiveFilters = activeCategory !== 'all' || activeBrand !== null || activeSizes.length > 0 || activeColors.length > 0 || activeMaterials.length > 0;

  const filteredProducts = useMemo(() => {
    let result = [...PRODUCTS];

    if (activeCategory !== 'all') {
      result = result.filter((item) => item.category === activeCategory);
    }

    if (activeBrand) {
      result = result.filter((item) => item.brandId === activeBrand);
    }

    // Фильтр по цене
    result = result.filter((item) => {
      const price = item.discountPrice || item.price;
      return price >= priceRange[0] && price <= priceRange[1];
    });

    // Сортировка
    if (sortBy === 'price-asc') result.sort((a, b) => (a.discountPrice || a.price) - (b.discountPrice || b.price));
    if (sortBy === 'price-desc') result.sort((a, b) => (b.discountPrice || b.price) - (a.discountPrice || a.price));
    if (sortBy === 'popular') result.sort((a, b) => (b.rating || 0) - (a.rating || 0));
    if (sortBy === 'new') result.sort((a, b) => Number(b.isNew) - Number(a.isNew));

    return result;
  }, [activeCategory, activeBrand, activeSizes, activeColors, activeMaterials, priceRange, sortBy]);

  const resetFilters = () => {
    setActiveCategory('all');
    setActiveBrand(null);
    setActiveSizes([]);
    setActiveColors([]);
    setActiveMaterials([]);
    setPriceRange([0, 100000]);
  };

  const toggleSize = (size: string) => {
    setActiveSizes(prev => prev.includes(size) ? prev.filter(s => s !== size) : [...prev, size]);
  };

  const toggleColor = (color: string) => {
    setActiveColors(prev => prev.includes(color) ? prev.filter(c => c !== color) : [...prev, color]);
  };

  const toggleMaterial = (material: string) => {
    setActiveMaterials(prev => prev.includes(material) ? prev.filter(m => m !== material) : [...prev, material]);
  };

  // Контент фильтров (переиспользуется для desktop и mobile)
  const FiltersContent = () => (
    <>
      {/* Категории */}
      <FilterSection title="Категории">
        <div className="space-y-1">
          {CATEGORIES.map((category) => (
            <button
              key={category.id}
              onClick={() => setActiveCategory(category.id)}
              className={cn(
                "w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition-colors",
                activeCategory === category.id
                  ? "bg-graphite text-white dark:bg-white dark:text-graphite"
                  : "text-graphite-light dark:text-white/70 hover:bg-ice dark:hover:bg-white/5"
              )}
            >
              <span>{category.name}</span>
              <span className="text-xs opacity-60">{category.count}</span>
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Размеры */}
      <FilterSection title="Размер">
        <div className="flex flex-wrap gap-2">
          {SIZES.map((size) => (
            <button
              key={size}
              onClick={() => toggleSize(size)}
              className={cn(
                "h-9 min-w-[40px] px-3 rounded-md border text-sm font-medium transition-all",
                activeSizes.includes(size)
                  ? "bg-graphite text-white border-graphite dark:bg-white dark:text-graphite dark:border-white"
                  : "bg-white dark:bg-transparent border-border-lighter dark:border-white/20 text-graphite dark:text-white hover:border-graphite dark:hover:border-white/50"
              )}
            >
              {size}
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Цвета */}
      <FilterSection title="Цвет">
        <div className="flex flex-wrap gap-2">
          {COLORS.map((color) => (
            <button
              key={color.name}
              onClick={() => toggleColor(color.name)}
              title={color.name}
              className={cn(
                "w-8 h-8 rounded-full border-2 transition-all",
                activeColors.includes(color.name)
                  ? "border-graphite dark:border-white scale-110 ring-2 ring-graphite/20 dark:ring-white/20"
                  : "border-border-lighter dark:border-white/20 hover:scale-105"
              )}
              style={{ backgroundColor: color.hex }}
            />
          ))}
        </div>
      </FilterSection>

      {/* Бренды */}
      <FilterSection title="Бренд">
        <div className="space-y-1 max-h-[200px] overflow-y-auto">
          <button
            onClick={() => setActiveBrand(null)}
            className={cn(
              "w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors",
              activeBrand === null
                ? "bg-graphite text-white dark:bg-white dark:text-graphite"
                : "text-graphite-light dark:text-white/70 hover:bg-ice dark:hover:bg-white/5"
            )}
          >
            <span className={cn("w-4 h-4 rounded border flex items-center justify-center", activeBrand === null ? "bg-white border-white" : "border-border-lighter dark:border-white/30")}>
              {activeBrand === null && <span className="text-graphite text-xs">✓</span>}
            </span>
            Все бренды
          </button>
          {BRANDS.map((brand) => (
            <button
              key={brand.id}
              onClick={() => setActiveBrand(brand.id)}
              className={cn(
                "w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors",
                activeBrand === brand.id
                  ? "bg-graphite text-white dark:bg-white dark:text-graphite"
                  : "text-graphite-light dark:text-white/70 hover:bg-ice dark:hover:bg-white/5"
              )}
            >
              <span className={cn("w-4 h-4 rounded border flex items-center justify-center", activeBrand === brand.id ? "bg-white border-white" : "border-border-lighter dark:border-white/30")}>
                {activeBrand === brand.id && <span className="text-graphite text-xs">✓</span>}
              </span>
              {brand.name}
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Цена */}
      <FilterSection title="Цена">
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <input
              type="number"
              placeholder="От"
              value={priceRange[0] || ''}
              onChange={(e) => setPriceRange([Number(e.target.value) || 0, priceRange[1]])}
              className="w-full h-10 px-3 rounded-lg border border-border-lighter dark:border-white/20 bg-white dark:bg-transparent text-sm text-graphite dark:text-white placeholder:text-ash"
            />
            <span className="text-ash">—</span>
            <input
              type="number"
              placeholder="До"
              value={priceRange[1] === 100000 ? '' : priceRange[1]}
              onChange={(e) => setPriceRange([priceRange[0], Number(e.target.value) || 100000])}
              className="w-full h-10 px-3 rounded-lg border border-border-lighter dark:border-white/20 bg-white dark:bg-transparent text-sm text-graphite dark:text-white placeholder:text-ash"
            />
          </div>
        </div>
      </FilterSection>

      {/* Материал */}
      <FilterSection title="Материал" defaultOpen={false}>
        <div className="space-y-1">
          {MATERIALS.map((material) => (
            <button
              key={material}
              onClick={() => toggleMaterial(material)}
              className={cn(
                "w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition-colors",
                activeMaterials.includes(material)
                  ? "bg-graphite text-white dark:bg-white dark:text-graphite"
                  : "text-graphite-light dark:text-white/70 hover:bg-ice dark:hover:bg-white/5"
              )}
            >
              <span className={cn("w-4 h-4 rounded border flex items-center justify-center", activeMaterials.includes(material) ? "bg-white border-white" : "border-border-lighter dark:border-white/30")}>
                {activeMaterials.includes(material) && <span className="text-graphite text-xs">✓</span>}
              </span>
              {material}
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Кнопка сброса */}
      {hasActiveFilters && (
        <button
          onClick={resetFilters}
          className="w-full mt-4 h-10 rounded-lg border border-error/30 text-error text-sm font-medium hover:bg-error/5 transition-colors"
        >
          Сбросить фильтры
        </button>
      )}
    </>
  );

  return (
    <div className="relative z-10 min-h-screen pt-24 md:pt-28 pb-20">
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
            <p className="text-sm text-ash mt-1">{filteredProducts.length} товаров</p>
          </div>

          <div className="flex items-center gap-3">
            {/* Кнопка фильтров на мобильном */}
            <button
              className="lg:hidden inline-flex items-center gap-2 h-10 px-4 rounded-lg border border-border-lighter dark:border-white/20 text-sm text-graphite dark:text-white"
              onClick={() => setShowMobileFilters(true)}
            >
              <SlidersHorizontal className="w-4 h-4" />
              Фильтры
              {hasActiveFilters && <span className="w-5 h-5 rounded-full bg-graphite text-white text-xs flex items-center justify-center">{activeSizes.length + activeColors.length + activeMaterials.length + (activeBrand ? 1 : 0) + (activeCategory !== 'all' ? 1 : 0)}</span>}
            </button>

            <SortDropdown value={sortBy} options={SORT_OPTIONS} onChange={setSortBy} />
          </div>
        </div>

        {/* Main content */}
        <div className="flex gap-8">
          {/* Desktop Sidebar Filters */}
          <aside className="hidden lg:block w-[260px] flex-shrink-0">
            <div className="sticky top-28 bg-white/60 dark:bg-white/[0.03] backdrop-blur-xl rounded-2xl border border-white/60 dark:border-white/10 p-5">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-graphite dark:text-white">Фильтры</h2>
                {hasActiveFilters && (
                  <button onClick={resetFilters} className="text-xs text-error hover:underline">
                    Сбросить
                  </button>
                )}
              </div>
              <FiltersContent />
            </div>
          </aside>

          {/* Products Grid */}
          <div className="flex-1 min-w-0">
            {/* Active filters tags */}
            {hasActiveFilters && (
              <div className="flex flex-wrap gap-2 mb-4">
                {activeCategory !== 'all' && (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {CATEGORIES.find(c => c.id === activeCategory)?.name}
                    <button onClick={() => setActiveCategory('all')} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                )}
                {activeBrand && (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {BRANDS.find(b => b.id === activeBrand)?.name}
                    <button onClick={() => setActiveBrand(null)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                )}
                {activeSizes.map(size => (
                  <span key={size} className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {size}
                    <button onClick={() => toggleSize(size)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                ))}
                {activeColors.map(color => (
                  <span key={color} className="inline-flex items-center gap-1 px-3 py-1.5 rounded-full bg-ice dark:bg-white/10 text-sm text-graphite dark:text-white">
                    {color}
                    <button onClick={() => toggleColor(color)} className="ml-1 hover:text-error"><X className="w-3.5 h-3.5" /></button>
                  </span>
                ))}
              </div>
            )}

            {filteredProducts.length > 0 ? (
              <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-3 gap-4 md:gap-5">
                {filteredProducts.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            ) : (
              <div className="text-center py-16 px-4">
                <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-ice dark:bg-white/10 flex items-center justify-center">
                  <SlidersHorizontal className="w-7 h-7 text-ash" />
                </div>
                <h3 className="text-xl font-serif text-graphite dark:text-white mb-2">Ничего не найдено</h3>
                <p className="text-sm text-ash mb-4">Попробуйте изменить параметры фильтрации</p>
                <button
                  onClick={resetFilters}
                  className="inline-flex h-10 items-center rounded-lg bg-graphite px-5 text-sm text-white"
                >
                  Сбросить фильтры
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Mobile Filters Drawer */}
      <Drawer isOpen={showMobileFilters} onClose={() => setShowMobileFilters(false)} title="Фильтры">
        <div className="pb-20">
          <FiltersContent />
        </div>
        <div className="fixed bottom-0 left-0 right-0 p-4 bg-white dark:bg-[#111214] border-t border-border-lighter dark:border-white/10">
          <button
            onClick={() => setShowMobileFilters(false)}
            className="w-full h-12 rounded-xl bg-graphite text-white text-sm font-medium"
          >
            Показать {filteredProducts.length} товаров
          </button>
        </div>
      </Drawer>
    </div>
  );
}

import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight, SlidersHorizontal, X, Check } from 'lucide-react';
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

const STYLES = ['Все', 'Спортвир', 'Арт', 'Авангард', 'Горпкор', 'Стритвир', 'Опиум', 'Y2K', 'Апсайкл', 'Архив', 'Кэжуал'];

// Компонент раскрывающейся секции фильтра
function FilterSection({ title, children, defaultOpen = true }: { title: string; children: React.ReactNode; defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-black/10 dark:border-white/10 py-4">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between text-left group"
      >
        <span className="text-[10px] font-mono uppercase tracking-widest text-black dark:text-white/90">{title}</span>
        <ChevronDown className={cn("w-3.5 h-3.5 text-black/50 dark:text-white/50 transition-transform duration-300 group-hover:text-black dark:group-hover:text-white", isOpen && "rotate-180")} />
      </button>
      <div className={cn("overflow-hidden transition-all duration-300 ease-in-out", isOpen ? "mt-4 max-h-[1000px] opacity-100" : "max-h-0 opacity-0")}>
        {children}
      </div>
    </div>
  );
}

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
      onClick={onClick}
      className="flex items-center justify-between w-full py-1.5 group text-left"
    >
      <div className="flex items-center gap-3">
        <div className="w-3.5 h-3.5 flex items-center justify-center shrink-0">
          {isActive && <Check className="w-4 h-4 text-black dark:text-white" strokeWidth={2.5} />}
        </div>
        <span className={cn(
          "text-[12px] font-sans font-light tracking-wide transition-colors",
          isActive ? "text-black dark:text-white font-normal" : "text-black/70 dark:text-white/70 group-hover:text-black dark:group-hover:text-white"
        )}>
          {label}
        </span>
      </div>
      {count !== undefined && (
        <span className="text-[10px] font-mono text-black/40 dark:text-white/40">
          [{count}]
        </span>
      )}
    </button>
  );
}

export function Catalog() {
  const [activeCategory, setActiveCategory] = useState('all');
  const [activeBrand, setActiveBrand] = useState<string | null>(null);
  const [activeStyles, setActiveStyles] = useState<string[]>([]);
  const [activeSizes, setActiveSizes] = useState<string[]>([]);
  const [activeColors, setActiveColors] = useState<string[]>([]);
  const [activeMaterials, setActiveMaterials] = useState<string[]>([]);
  const [priceRange, setPriceRange] = useState<[number, number]>([0, 100000]);
  const [sortBy, setSortBy] = useState('new');
  const [showMobileFilters, setShowMobileFilters] = useState(false);

  const hasActiveFilters = activeCategory !== 'all' || activeBrand !== null || activeStyles.length > 0 || activeSizes.length > 0 || activeColors.length > 0 || activeMaterials.length > 0;

  const filteredProducts = useMemo(() => {
    let result = [...PRODUCTS];

    if (activeCategory !== 'all') {
      result = result.filter((item) => item.category === activeCategory);
    }

    if (activeBrand) {
      result = result.filter((item) => item.brandId === activeBrand);
    }
    
    // Фильтр по стилям
    if (activeStyles.length > 0) {
      result = result.filter((item) => item.styles?.some(style => activeStyles.includes(style)));
    }

    // Фильтр по размеру
    if (activeSizes.length > 0) {
      result = result.filter((item) => item.sizes?.some(size => activeSizes.includes(size)));
    }

    // Фильтр по цвету
    if (activeColors.length > 0) {
      result = result.filter((item) => item.colors?.some(color => activeColors.includes(color.name)));
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
  }, [activeCategory, activeBrand, activeStyles, activeSizes, activeColors, activeMaterials, priceRange, sortBy]);

  const resetFilters = () => {
    setActiveCategory('all');
    setActiveBrand(null);
    setActiveStyles([]);
    setActiveSizes([]);
    setActiveColors([]);
    setActiveMaterials([]);
    setPriceRange([0, 100000]);
  };

  const toggleStyle = (style: string) => {
    if (style === 'Все') {
      setActiveStyles([]);
      return;
    }
    setActiveStyles((prev) =>
      prev.includes(style)
        ? prev.filter((s) => s !== style)
        : [...prev, style]
    );
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
    <div className="flex flex-col px-1 pb-4">
      {/* Стили */}
      <FilterSection title="Стиль" defaultOpen={true}>
        <div className="flex flex-col gap-1">
          {STYLES.map((style) => (
            <FilterCheckbox
              key={style}
              label={style}
              isActive={style === 'Все' ? activeStyles.length === 0 : activeStyles.includes(style)}
              onClick={() => toggleStyle(style)}
            />
          ))}
        </div>
      </FilterSection>

      {/* Категории */}
      <FilterSection title="Категории" defaultOpen={activeCategory !== 'all'}>
        <div className="flex flex-col gap-1">
          {CATEGORIES.map((category) => (
            <FilterCheckbox
              key={category.id}
              label={category.name}
              count={category.count}
              isActive={activeCategory === category.id}
              onClick={() => setActiveCategory(category.id)}
            />
          ))}
        </div>
      </FilterSection>

      {/* Размеры */}
      <FilterSection title="Размер" defaultOpen={activeSizes.length > 0}>
        <div className="grid grid-cols-4 gap-2">
          {SIZES.map((size) => (
            <button
              key={size}
              onClick={() => toggleSize(size)}
              className={cn(
                "h-8 border text-[11px] font-mono transition-all flex items-center justify-center rounded-[2px]",
                activeSizes.includes(size)
                  ? "bg-black text-white border-black dark:bg-white dark:text-black dark:border-white"
                  : "bg-transparent border-black/20 dark:border-white/20 text-black/60 dark:text-white/60 hover:border-black/50 dark:hover:border-white/50 hover:text-black dark:hover:text-white"
              )}
            >
              {size}
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Цвета */}
      <FilterSection title="Цвет" defaultOpen={activeColors.length > 0}>
        <div className="grid grid-cols-3 gap-2">
          {COLORS.map((color) => (
            <button
              key={color.name}
              onClick={() => toggleColor(color.name)}
              title={color.name}
              className={cn(
                "h-8 px-2 border text-[10px] font-mono tracking-wider transition-all flex items-center gap-1.5 rounded-[2px]",
                activeColors.includes(color.name)
                  ? "bg-black text-white border-black dark:bg-white dark:text-black dark:border-white"
                  : "bg-transparent border-black/20 dark:border-white/20 text-black/60 dark:text-white/60 hover:border-black/50 dark:hover:border-white/50 hover:text-black dark:hover:text-white"
              )}
            >
              <span 
                className="w-2.5 h-2.5 rounded-[1px] border border-black/30 shadow-inner block"
                style={{ backgroundColor: color.hex }}
              />
              <span className="truncate">{color.name}</span>
            </button>
          ))}
        </div>
      </FilterSection>

      {/* Бренды */}
      <FilterSection title="Бренд" defaultOpen={activeBrand !== null}>
        <div className="flex flex-col gap-1">
          <FilterCheckbox
            label="Все бренды"
            isActive={activeBrand === null}
            onClick={() => setActiveBrand(null)}
          />
          {BRANDS.map((brand) => (
            <FilterCheckbox
              key={brand.id}
              label={brand.name}
              isActive={activeBrand === brand.id}
              onClick={() => setActiveBrand(brand.id)}
            />
          ))}
        </div>
      </FilterSection>

      {/* Цена */}
      <FilterSection title="Цена" defaultOpen={priceRange[0] !== 0 || priceRange[1] !== 100000}>
        <div className="flex items-center justify-between gap-2 mt-1">
          <div className="relative flex-1">
            <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[9px] uppercase font-mono text-black/40 dark:text-white/40">От</span>
            <input
              type="number"
              value={priceRange[0] || ''}
              onChange={(e) => setPriceRange([Number(e.target.value) || 0, priceRange[1]])}
              className="w-full h-8 pl-8 pr-2 rounded-[2px] border border-black/20 dark:border-white/20 bg-transparent text-[12px] font-mono text-black dark:text-white transition-colors focus:border-black/60 dark:focus:border-white/60 outline-none"
            />
          </div>
          <div className="w-2 h-px bg-black/20 dark:bg-white/20" />
          <div className="relative flex-1">
            <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[9px] uppercase font-mono text-black/40 dark:text-white/40">До</span>
            <input
              type="number"
              value={priceRange[1] === 100000 ? '' : priceRange[1]}
              onChange={(e) => setPriceRange([priceRange[0], Number(e.target.value) || 100000])}
              className="w-full h-8 pl-8 pr-2 rounded-[2px] border border-black/20 dark:border-white/20 bg-transparent text-[12px] font-mono text-black dark:text-white transition-colors focus:border-black/60 dark:focus:border-white/60 outline-none"
            />
          </div>
        </div>
      </FilterSection>

      {/* Материал */}
      <FilterSection title="Материал" defaultOpen={activeMaterials.length > 0}>
        <div className="flex flex-col gap-1">
          {MATERIALS.map((material) => (
            <FilterCheckbox
              key={material}
              label={material}
              isActive={activeMaterials.includes(material)}
              onClick={() => toggleMaterial(material)}
            />
          ))}
        </div>
      </FilterSection>

      {/* Кнопка сброса */}
      {hasActiveFilters && (
        <button
          onClick={resetFilters}
          className="w-full mt-6 h-8 text-[10px] font-mono uppercase tracking-widest text-black/60 hover:text-black dark:text-white/60 dark:hover:text-white border-b border-dashed border-black/20 dark:border-white/20 hover:border-black/60 dark:hover:border-white/60 transition-colors"
        >
          Сбросить фильтры
        </button>
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
              {hasActiveFilters && <span className="w-5 h-5 rounded-full bg-graphite text-white dark:text-black text-xs flex items-center justify-center">{activeStyles.length + activeSizes.length + activeColors.length + activeMaterials.length + (activeBrand ? 1 : 0) + (activeCategory !== 'all' ? 1 : 0)}</span>}
            </button>

            <SortDropdown value={sortBy} options={SORT_OPTIONS} onChange={setSortBy} />
          </div>
        </div>

        {/* Main content */}
        <div className="flex gap-10">
          {/* Desktop Sidebar Filters (Archivecore / Liquid Glass) */}
          <aside className="hidden lg:block w-[280px] flex-shrink-0 relative">
            <div className="sticky top-28 flex flex-col max-h-[calc(100vh-120px)] bg-white/5 dark:bg-[#111111]/5 backdrop-blur-[24px] rounded-[24px] border-[1px] border-white/20 dark:border-white/10 shadow-[0_8px_32px_rgba(0,0,0,0.04)] overflow-hidden">
              <div className="flex items-center justify-between px-6 pt-6 pb-4 shrink-0 z-10 border-b border-black/5 dark:border-white/5">
                <h2 className="text-[14px] font-mono tracking-widest uppercase font-semibold text-black dark:text-white">Фильтры</h2>
                {hasActiveFilters && (
                  <button onClick={resetFilters} className="text-[10px] font-mono uppercase tracking-widest text-black/50 hover:text-black dark:text-white/50 dark:hover:text-white transition-colors">
                    Сбросить
                  </button>
                )}
              </div>
              <div className="px-6 pb-6 pt-2 overflow-y-auto [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-black/10 dark:[&::-webkit-scrollbar-thumb]:bg-white/10 [&::-webkit-scrollbar-track]:bg-transparent">
                <FiltersContent />
              </div>
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
        <div className="pb-20 bg-white/80 dark:bg-black/40 backdrop-blur-xl">
          <FiltersContent />
        </div>
        <div className="fixed bottom-0 left-0 right-0 p-4 bg-white/90 dark:bg-black/90 backdrop-blur-xl border-t border-black/10 dark:border-white/10">
          <button
            onClick={() => setShowMobileFilters(false)}
            className="w-full h-10 rounded-lg bg-black text-white dark:bg-white dark:text-black text-[12px] font-mono uppercase tracking-widest font-semibold"
          >
            Показать {filteredProducts.length} товаров
          </button>
        </div>
      </Drawer>
    </div>
  );
}


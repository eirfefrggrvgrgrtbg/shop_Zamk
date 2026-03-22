import { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, Loader2, ArrowRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useSearch } from '../../contexts/SearchContext';
import { PRODUCTS, type Product } from '../../lib/mock-data';

const TOP_QUERIES = [
  'деконструированные жакеты',
  'кожаные тренчи',
  'Maison Margiela',
  'авангардный трикотаж'
];

// Отбираем 4 продукта для кураторского выбора чтобы красиво заполнить сетку
const RECOMMENDED_PRODUCTS = PRODUCTS.slice(0, 4);

// Настройки анимации списков (Stagger effect)
const containerVariants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.05, delayChildren: 0.05 }
  },
  exit: {
    opacity: 0,
    transition: { staggerChildren: 0.02, staggerDirection: -1 }
  }
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  show: { opacity: 1, y: 0, transition: { duration: 0.7, ease: [0.16, 1, 0.3, 1] } },
  exit: { opacity: 0, y: -10, transition: { duration: 0.3 } }
};

export function SearchOverlay() {
  const { isSearchOpen, closeSearch } = useSearch();
  const [query, setQuery] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [results, setResults] = useState<Product[]>([]);

  // Scroll lock
  useEffect(() => {
    if (isSearchOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
      setQuery('');
      setResults([]);
    }
    return () => { document.body.style.overflow = ''; };
  }, [isSearchOpen]);

  // Handle escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isSearchOpen) closeSearch();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isSearchOpen, closeSearch]);

  // Fake Network Request for Search
  useEffect(() => {
    const cleanQuery = query.trim().toLowerCase();
    if (!cleanQuery) {
      setResults([]);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    const timer = setTimeout(() => {
      const filtered = PRODUCTS.filter(p => 
        p.name.toLowerCase().includes(cleanQuery) || 
        p.brand.toLowerCase().includes(cleanQuery) ||
        p.category.toLowerCase().includes(cleanQuery)
      );
      setResults(filtered);
      setIsLoading(false);
    }, 400);

    return () => clearTimeout(timer);
  }, [query]);

  return (
    <AnimatePresence>
      {isSearchOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
          className="fixed inset-0 z-[100] flex flex-col bg-white/10 backdrop-blur-xl"
        >
          {/* Background layer to close on click */}
          <div className="absolute inset-0 z-0" onClick={closeSearch} />
          
          <div className="relative z-10 w-full max-w-[1400px] mx-auto px-6 md:px-12 pt-8 md:pt-14 flex flex-col h-full">
            
            {/* Header / Close */}
            <div className="flex justify-end mb-6 md:mb-10">
              <button 
                onClick={closeSearch}
                className="group flex items-center justify-center w-12 h-12 rounded-full hover:bg-white/10 transition-colors"
                aria-label="Закрыть поиск"
              >
                <X className="w-7 h-7 text-graphite/70 group-hover:text-graphite transition-colors stroke-[1.5]" />
              </button>
            </div>

            {/* Вынесенный компонент ввода */}
            <SearchInput query={query} setQuery={setQuery} isLoading={isLoading} />

            {/* Content Area */}
            <div className="flex-1 overflow-y-auto mt-10 pb-20 scrollbar-hide">
              <AnimatePresence mode="wait">
                {query.trim() === '' ? (
                  <SearchSuggestions 
                    onQueryClick={setQuery} 
                    onProductClick={closeSearch} 
                  />
                ) : (
                  <SearchResults 
                    query={query} 
                    results={results} 
                    isLoading={isLoading} 
                    onProductClick={closeSearch} 
                  />
                )}
              </AnimatePresence>
            </div>
            
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

// ----------------------------------------------------
// UI Компоненты логики
// ----------------------------------------------------

function SearchInput({ query, setQuery, isLoading }: { query: string, setQuery: (q: string) => void, isLoading: boolean }) {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setTimeout(() => inputRef.current?.focus(), 150);
  }, []);

  return (
    <div className="relative flex items-center border-b border-graphite/10 focus-within:border-graphite/30 transition-colors duration-500 pb-4 md:pb-6 group">
      <input 
        ref={inputRef}
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="поиск по архиву..."
        className="w-full text-4xl md:text-6xl lg:text-[5rem] font-light tracking-tight text-graphite placeholder:text-graphite/30 bg-transparent focus:ring-0 focus:outline-none transition-colors"
        spellCheck={false}
      />
      <div className="absolute right-0 bottom-6 md:bottom-8 flex items-center gap-4">
        {isLoading ? (
          <Loader2 className="w-8 h-8 md:w-10 md:h-10 text-graphite/40 animate-spin" />
        ) : (
          <button className="p-2 md:p-3 rounded-full hover:bg-white/10 transition-colors text-graphite/50 hover:text-graphite">
            <ArrowRight className="w-8 h-8 md:w-10 md:h-10 stroke-[1.5]" />
          </button>
        )}
      </div>
    </div>
  );
}

function SearchSuggestions({ onQueryClick, onProductClick }: { onQueryClick: (q: string) => void, onProductClick: () => void }) {
  return (
    <motion.div
      key="default-view"
      variants={containerVariants}
      initial="hidden"
      animate="show"
      exit="exit"
      className="flex flex-col gap-10 md:gap-14"
    >
      {/* ВЕРХНЯЯ ЧАСТЬ: Топ запросы (одна строка) */}
      <div className="flex flex-col gap-4">
        <motion.h3 variants={itemVariants} className="text-[11px] md:text-xs font-semibold tracking-[0.15em] uppercase text-graphite/40">
          Популярное
        </motion.h3>
        <ul className="flex items-center gap-x-6 md:gap-x-12 overflow-x-auto scrollbar-hide pb-2">
          {TOP_QUERIES.map((q, idx) => (
            <motion.li key={idx} variants={itemVariants} className="shrink-0">
              <button
                onClick={() => onQueryClick(q)}
                className="text-lg md:text-[1.35rem] leading-tight font-light text-graphite/60 hover:text-graphite transition-colors whitespace-nowrap"
              >
                {q}
              </button>
            </motion.li>
          ))}
        </ul>
      </div>

      {/* НИЖНЯЯ ЧАСТЬ: Кураторский выбор */}
      <div className="flex flex-col gap-6 md:gap-8">
        <motion.h3 variants={itemVariants} className="text-[11px] md:text-xs font-semibold tracking-[0.15em] uppercase text-graphite/40">
          Кураторский выбор
        </motion.h3>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6 md:gap-8">
          {RECOMMENDED_PRODUCTS.map((product) => (
            <ProductCard key={product.id} product={product} onClick={onProductClick} />
          ))}
        </div>
      </div>
    </motion.div>
  );
}

function SearchResults({ query, results, isLoading, onProductClick }: { query: string, results: Product[], isLoading: boolean, onProductClick: () => void }) {
  return (
    <motion.div
      key="search-results"
      variants={containerVariants}
      initial="hidden"
      animate="show"
      exit="exit"
      className="flex flex-col h-full"
    >
      {isLoading ? (
        <div className="w-full flex justify-center py-20" /> // Скелетон
      ) : results.length > 0 ? (
        <div>
          <motion.h3 variants={itemVariants} className="text-[11px] md:text-xs font-semibold tracking-[0.15em] uppercase text-graphite/40 mb-8">
            Результаты ({results.length})
          </motion.h3>
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-12">
            {results.map((product) => (
              <ProductCard key={product.id} product={product} onClick={onProductClick} />
            ))}
          </div>
        </div>
      ) : (
        <motion.div variants={itemVariants} className="pt-20">
          <p className="text-2xl md:text-3xl text-graphite/50 font-light tracking-wide">
            По запросу «<span className="text-graphite">{query}</span>» ничего не найдено
          </p>
        </motion.div>
      )}
    </motion.div>
  );
}

function ProductCard({ product, onClick }: { product: Product, onClick: () => void }) {
  return (
    <motion.div variants={itemVariants} className="group h-full">
      <Link 
        to={`/product/${product.id}`} 
        onClick={onClick} 
        className="flex flex-col h-full bg-white/40 backdrop-blur-lg border border-white/40 rounded-[2rem] p-3 md:p-4 hover:bg-white/60 hover:border-white/70 transition-all duration-500 shadow-[0_8px_32px_rgba(20,30,40,0.04)] hover:shadow-[0_12px_48px_rgba(20,30,40,0.08)]"
      >
        <div className="aspect-[3/4] bg-white/50 rounded-[1.25rem] overflow-hidden mb-4 md:mb-5">
          <img 
            src={product.image} 
            alt={product.name}
            className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-[1.5s] ease-[0.16,1,0.3,1]"
          />
        </div>
        <div className="px-1 md:px-2 pb-1 flex-1 flex flex-col justify-end">
          <h4 className="font-serif text-lg md:text-xl text-graphite leading-tight mb-1 truncate">
            {product.brand}
          </h4>
          <p className="text-[13px] md:text-sm text-graphite/70 font-light truncate mb-3 md:mb-4">
            {product.name}
          </p>
          <p className="text-[13px] md:text-sm font-medium text-graphite tracking-wide mt-auto">
            {product.price.toLocaleString('ru-RU')} ₽
          </p>
        </div>
      </Link>
    </motion.div>
  );
}
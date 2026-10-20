import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, RotateCcw, Search, X } from 'lucide-react';
import { cn } from '../lib/utils';
import { Button } from '../components/ui/Button';
import { fetchBrands } from '../api/publicCatalog';
import type { Brand } from '../types/catalog';

export const LATIN_LETTERS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');
export const CYRILLIC_LETTERS = 'АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЭЮЯ'.split('');

export function getBrandInitial(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return '#';
  const first = trimmed.charAt(0).toUpperCase();
  if (/^[A-Z]$/i.test(first)) return first;
  if (/^[А-ЯЁ]$/i.test(first)) return first === 'Ё' ? 'Е' : first;
  return '#';
}

function getBrandPlural(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'брендов';
  if (mod10 === 1) return 'бренд';
  if (mod10 >= 2 && mod10 <= 4) return 'бренда';
  return 'брендов';
}

export function Brands() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedLetter, setSelectedLetter] = useState('ALL');

  useEffect(() => {
    let cancelled = false;

    async function loadBrands() {
      setIsLoading(true);
      setError('');

      try {
        const data = await fetchBrands();
        if (!cancelled) {
          setBrands(data);
        }
      } catch {
        if (!cancelled) {
          setError('Не удалось загрузить бренды. Проверьте, запущен ли backend.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadBrands();

    return () => {
      cancelled = true;
    };
  }, []);

  const letterCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const b of brands) {
      const initial = getBrandInitial(b.name);
      counts[initial] = (counts[initial] || 0) + 1;
    }
    return counts;
  }, [brands]);

  const filteredBrands = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return brands.filter((b) => {
      if (selectedLetter !== 'ALL') {
        const initial = getBrandInitial(b.name);
        if (initial !== selectedLetter) return false;
      }
      if (q) {
        if (!b.name.toLowerCase().includes(q)) return false;
      }
      return true;
    });
  }, [brands, selectedLetter, searchQuery]);

  const handleLetterClick = (letter: string) => {
    setSelectedLetter((prev) => (prev === letter ? 'ALL' : letter));
  };

  const handleResetFilters = () => {
    setSearchQuery('');
    setSelectedLetter('ALL');
  };

  return (
    <div className="relative z-10 min-h-screen pt-32 md:pt-40 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">
        {/* Header */}
        <section className="mb-10 border-b border-border-lighter dark:border-white/10 pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
            Сообщество
          </p>
          <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
            <div>
              <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
                Бренды
              </h1>
              <p className="mt-3 text-sm md:text-base text-graphite-light dark:text-white/70 max-w-xl">
                Независимые марки, концептуальные дизайнеры и архивные коллекции на ZAMK.
              </p>
            </div>
            {!isLoading && !error && brands.length > 0 && (
              <span className="text-xs font-mono uppercase tracking-wider text-ash">
                {filteredBrands.length} {getBrandPlural(filteredBrands.length)}
              </span>
            )}
          </div>
        </section>

        {error && (
          <div className="rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-950/20 dark:text-red-300">
            {error}
          </div>
        )}

        {isLoading ? (
          <div className="rounded-3xl border border-border-lighter bg-white/60 p-12 text-center text-ash dark:border-white/10 dark:bg-white/5">
            Загрузка брендов...
          </div>
        ) : brands.length === 0 ? (
          <div className="rounded-3xl border border-dashed border-border-lighter bg-white/60 p-12 text-center text-ash dark:border-white/10 dark:bg-white/5">
            <h3 className="font-serif text-2xl text-graphite dark:text-white">Каталог брендов пуст</h3>
            <p className="mt-2 text-sm text-graphite-light dark:text-white/70">
              В данный момент активные бренды не опубликованы.
            </p>
          </div>
        ) : (
          <div>
            {/* Filters bar */}
            <section className="mb-8 space-y-4">
              {/* Search input + Reset shortcut */}
              <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4">
                <div className="relative flex-1 max-w-md">
                  <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-ash pointer-events-none" />
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder="Поиск по названию бренда..."
                    aria-label="Поиск бренда по названию"
                    className="w-full h-11 pl-11 pr-10 rounded-full bg-white/80 dark:bg-white/10 border border-border-soft dark:border-white/15 text-sm text-graphite dark:text-white placeholder:text-ash-light dark:placeholder:text-white/40 focus:outline-none focus:border-graphite/40 dark:focus:border-white/40 transition-colors shadow-sm"
                  />
                  {searchQuery && (
                    <button
                      type="button"
                      onClick={() => setSearchQuery('')}
                      aria-label="Очистить поиск"
                      className="absolute right-3.5 top-1/2 -translate-y-1/2 text-ash hover:text-graphite dark:hover:text-white p-1"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  )}
                </div>

                {(searchQuery || selectedLetter !== 'ALL') && (
                  <button
                    type="button"
                    onClick={handleResetFilters}
                    className="inline-flex items-center gap-1.5 text-xs uppercase tracking-wider text-ash hover:text-graphite dark:hover:text-white transition-colors self-start sm:self-auto py-2"
                  >
                    <RotateCcw className="w-3.5 h-3.5" />
                    Сбросить фильтры
                  </button>
                )}
              </div>

              {/* Alphabet navigation strip */}
              <div className="overflow-x-auto scrollbar-none -mx-4 px-4 sm:mx-0 sm:px-0 py-1">
                <div className="flex items-center gap-1 sm:gap-1.5 min-w-max md:flex-wrap">
                  <button
                    type="button"
                    onClick={() => setSelectedLetter('ALL')}
                    aria-pressed={selectedLetter === 'ALL'}
                    className={cn(
                      'h-8 px-4 rounded-full text-xs font-medium uppercase tracking-wider transition-all shrink-0',
                      selectedLetter === 'ALL'
                        ? 'bg-graphite text-white dark:bg-white dark:text-black shadow-sm font-semibold'
                        : 'bg-white/80 dark:bg-white/10 text-graphite dark:text-white border border-border-lighter dark:border-white/15 hover:border-graphite/40 dark:hover:border-white/40',
                    )}
                  >
                    Все
                  </button>

                  <span className="h-4 w-px bg-border-lighter dark:bg-white/10 mx-1 shrink-0" aria-hidden="true" />

                  {/* Latin letters */}
                  {LATIN_LETTERS.map((letter) => {
                    const count = letterCounts[letter] || 0;
                    const isSelected = selectedLetter === letter;
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={letter}
                        type="button"
                        disabled={isDisabled}
                        aria-disabled={isDisabled}
                        aria-pressed={isSelected}
                        onClick={() => handleLetterClick(letter)}
                        className={cn(
                          'h-8 w-8 rounded-full text-xs font-medium font-mono flex items-center justify-center transition-all shrink-0',
                          isSelected
                            ? 'bg-graphite text-white dark:bg-white dark:text-black shadow-sm font-semibold ring-2 ring-graphite/20 dark:ring-white/30'
                            : isDisabled
                            ? 'opacity-25 text-ash-light dark:text-white/20 border border-transparent cursor-not-allowed'
                            : 'bg-white/80 dark:bg-white/10 text-graphite dark:text-white border border-border-lighter dark:border-white/15 hover:border-graphite/40 dark:hover:border-white/40 cursor-pointer',
                        )}
                      >
                        {letter}
                      </button>
                    );
                  })}

                  <span className="h-4 w-px bg-border-lighter dark:bg-white/10 mx-1 shrink-0" aria-hidden="true" />

                  {/* Cyrillic letters */}
                  {CYRILLIC_LETTERS.map((letter) => {
                    const count = letterCounts[letter] || 0;
                    const isSelected = selectedLetter === letter;
                    const isDisabled = count === 0;

                    return (
                      <button
                        key={letter}
                        type="button"
                        disabled={isDisabled}
                        aria-disabled={isDisabled}
                        aria-pressed={isSelected}
                        onClick={() => handleLetterClick(letter)}
                        className={cn(
                          'h-8 w-8 rounded-full text-xs font-medium font-mono flex items-center justify-center transition-all shrink-0',
                          isSelected
                            ? 'bg-graphite text-white dark:bg-white dark:text-black shadow-sm font-semibold ring-2 ring-graphite/20 dark:ring-white/30'
                            : isDisabled
                            ? 'opacity-25 text-ash-light dark:text-white/20 border border-transparent cursor-not-allowed'
                            : 'bg-white/80 dark:bg-white/10 text-graphite dark:text-white border border-border-lighter dark:border-white/15 hover:border-graphite/40 dark:hover:border-white/40 cursor-pointer',
                        )}
                      >
                        {letter}
                      </button>
                    );
                  })}

                  <span className="h-4 w-px bg-border-lighter dark:bg-white/10 mx-1 shrink-0" aria-hidden="true" />

                  {/* Symbol / Digit (#) */}
                  {(() => {
                    const count = letterCounts['#'] || 0;
                    const isSelected = selectedLetter === '#';
                    const isDisabled = count === 0;

                    return (
                      <button
                        type="button"
                        disabled={isDisabled}
                        aria-disabled={isDisabled}
                        aria-pressed={isSelected}
                        onClick={() => handleLetterClick('#')}
                        className={cn(
                          'h-8 w-8 rounded-full text-xs font-medium font-mono flex items-center justify-center transition-all shrink-0',
                          isSelected
                            ? 'bg-graphite text-white dark:bg-white dark:text-black shadow-sm font-semibold ring-2 ring-graphite/20 dark:ring-white/30'
                            : isDisabled
                            ? 'opacity-25 text-ash-light dark:text-white/20 border border-transparent cursor-not-allowed'
                            : 'bg-white/80 dark:bg-white/10 text-graphite dark:text-white border border-border-lighter dark:border-white/15 hover:border-graphite/40 dark:hover:border-white/40 cursor-pointer',
                        )}
                      >
                        #
                      </button>
                    );
                  })()}
                </div>
              </div>
            </section>

            {/* Brands listing */}
            {filteredBrands.length === 0 ? (
              <div className="rounded-3xl border border-border-lighter bg-white/60 p-12 text-center text-ash dark:border-white/10 dark:bg-white/5 my-6">
                <div className="w-12 h-12 mx-auto rounded-full bg-ice dark:bg-white/10 flex items-center justify-center text-ash mb-4">
                  <Search className="w-5 h-5" />
                </div>
                <h3 className="text-2xl font-serif font-medium text-graphite dark:text-white">
                  Ничего не найдено
                </h3>
                <p className="mt-2 text-sm text-graphite-light dark:text-white/70 max-w-md mx-auto">
                  {searchQuery && selectedLetter !== 'ALL'
                    ? `По запросу «${searchQuery}» на букву «${selectedLetter}» подходящих брендов не найдено.`
                    : searchQuery
                    ? `По запросу «${searchQuery}» подходящих брендов не найдено.`
                    : `На букву «${selectedLetter}» бренды пока не представлены в каталоге.`}
                </p>
                <div className="mt-6">
                  <Button
                    variant="secondary"
                    onClick={handleResetFilters}
                    className="rounded-full px-6 gap-2"
                  >
                    <RotateCcw className="w-4 h-4" />
                    Сбросить фильтры
                  </Button>
                </div>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
                {filteredBrands.map((brand) => {
                  const brandTarget = `/brand/${brand.slug || brand.id}`;
                  const initial = brand.name.trim().slice(0, 1).toUpperCase();

                  return (
                    <Link
                      key={brand.id}
                      to={brandTarget}
                      data-testid={`brand-card-${brand.id}`}
                      className="group relative flex flex-col justify-between rounded-[2rem] border border-border-lighter bg-white/70 p-6 shadow-sm backdrop-blur-md dark:border-white/10 dark:bg-white/5 transition-all duration-300 hover:-translate-y-1 hover:shadow-cloud dark:hover:shadow-none hover:border-graphite/30 dark:hover:border-white/30"
                    >
                      <div>
                        <div className="flex items-center gap-4">
                          <div className="flex h-14 w-14 sm:h-16 sm:w-16 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-ice text-xl font-serif font-medium text-graphite dark:bg-white/10 dark:text-white border border-border-lighter dark:border-white/10">
                            {brand.image ? (
                              <img
                                src={brand.image}
                                alt={brand.name}
                                className="h-full w-full object-cover"
                                loading="lazy"
                              />
                            ) : (
                              <span>{initial}</span>
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <h2 className="text-xl sm:text-2xl font-serif font-medium text-graphite dark:text-white group-hover:text-graphite-dark dark:group-hover:text-white transition-colors truncate">
                              {brand.name}
                            </h2>
                            <span className="mt-1 inline-flex items-center gap-1 text-xs uppercase tracking-[0.14em] text-ash group-hover:text-graphite dark:group-hover:text-white transition-colors">
                              Смотреть витрину <ArrowRight className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" />
                            </span>
                          </div>
                        </div>

                        {brand.description?.trim() ? (
                          <p className="mt-4 text-sm text-graphite-light dark:text-white/70 line-clamp-2 leading-relaxed">
                            {brand.description.trim()}
                          </p>
                        ) : null}
                      </div>
                    </Link>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

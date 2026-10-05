import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import {
  Search,
  ShoppingCart,
  RotateCcw,
  Boxes,
  Package,
  User,
  AlertCircle,
  X,
  Loader2,
  CornerDownLeft,
} from 'lucide-react';
import {
  groupSearchResults,
  getResultNavigationUrl,
  getAdminSearchErrorMessage,
  searchAdminGlobal,
  GlobalSearchResult,
  GlobalSearchResultType,
} from '../../api/adminSearch';

export interface NavItem {
  name: string;
  path: string;
  icon: React.ElementType;
  permission?: string | string[];
  count?: number;
}

export interface NavGroup {
  title: string;
  items: NavItem[];
}

export interface AdminSearchPaletteProps {
  isOpen: boolean;
  onClose: () => void;
  navGroups?: NavGroup[];
  isRouteActive?: (path: string) => boolean;
}

const RESULT_ICONS: Record<GlobalSearchResultType, React.ElementType> = {
  order: ShoppingCart,
  return: RotateCcw,
  inventory_unit: Boxes,
  product_variant: Package,
  product: Package,
  customer: User,
};

export function AdminSearchPalette({
  isOpen,
  onClose,
  navGroups = [],
  isRouteActive = () => false,
}: AdminSearchPaletteProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<GlobalSearchResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);

  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const activeItemRef = useRef<HTMLAnchorElement | HTMLButtonElement | null>(null);
  const requestSeqRef = useRef(0);

  // Filter navigation items immediately based on search query
  const filteredNavGroups = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return navGroups;

    return navGroups
      .map((group) => ({
        ...group,
        items: group.items.filter((item) =>
          item.name.toLowerCase().includes(q)
        ),
      }))
      .filter((g) => g.items.length > 0);
  }, [navGroups, query]);

  const flatNavItems = useMemo(() => {
    return filteredNavGroups.flatMap((g) => g.items);
  }, [filteredNavGroups]);

  // Group and flatten API results for navigation
  const groupedResults = useMemo(() => groupSearchResults(results), [results]);
  const flatItems = useMemo(() => {
    const items: GlobalSearchResult[] = [];
    for (const group of groupedResults) {
      items.push(...group.items);
    }
    return items;
  }, [groupedResults]);

  // Total items available for keyboard selection
  const totalItems = flatNavItems.length + flatItems.length;

  // Reset transient state
  const resetSearch = useCallback(() => {
    requestSeqRef.current++; // Invalidate any in-flight requests
    setQuery('');
    setResults([]);
    setIsLoading(false);
    setError(null);
    setActiveIndex(0);
  }, []);

  // Handle select / navigate for search results
  const handleSelect = useCallback(
    (item: GlobalSearchResult) => {
      const url = getResultNavigationUrl(item);
      navigate(url);
      onClose();
      resetSearch();
    },
    [navigate, onClose, resetSearch]
  );

  // Auto-focus input on open
  useEffect(() => {
    if (isOpen) {
      resetSearch();
      const timer = setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    } else {
      requestSeqRef.current++; // Invalidate when closed
    }
  }, [isOpen, resetSearch]);

  // Debounced search with sequence invalidation on EVERY query transition
  useEffect(() => {
    const currentSeq = ++requestSeqRef.current;
    const trimmed = query.trim();

    if (trimmed.length < 2) {
      setResults([]);
      setIsLoading(false);
      setError(null);
      setActiveIndex(0);
      return;
    }

    setIsLoading(true);
    setError(null);

    const debounceTimer = setTimeout(async () => {
      try {
        const resp = await searchAdminGlobal(trimmed);
        if (requestSeqRef.current === currentSeq) {
          setResults(resp.results || []);
          setActiveIndex(0);
        }
      } catch (err: unknown) {
        if (requestSeqRef.current === currentSeq) {
          setError(getAdminSearchErrorMessage(err));
          setResults([]);
        }
      } finally {
        if (requestSeqRef.current === currentSeq) {
          setIsLoading(false);
        }
      }
    }, 250);

    return () => clearTimeout(debounceTimer);
  }, [query]);

  // Scroll active item into view
  useEffect(() => {
    if (activeItemRef.current && typeof activeItemRef.current.scrollIntoView === 'function') {
      activeItemRef.current.scrollIntoView({ block: 'nearest' });
    }
  }, [activeIndex]);

  // Global keydown within palette
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }

    if (totalItems === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((prev) => (prev < totalItems - 1 ? prev + 1 : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((prev) => (prev > 0 ? prev - 1 : totalItems - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (activeIndex < flatNavItems.length) {
        const selectedNav = flatNavItems[activeIndex];
        if (selectedNav) {
          navigate(selectedNav.path);
          onClose();
          resetSearch();
        }
      } else {
        const selectedResult = flatItems[activeIndex - flatNavItems.length];
        if (selectedResult) {
          handleSelect(selectedResult);
        }
      }
    }
  };

  if (!isOpen) return null;

  let currentFlatIndex = -1;

  return (
    <div
      data-testid="admin-search-palette-overlay"
      className="fixed inset-0 z-50 flex items-start justify-center pt-8 sm:pt-14 px-4 bg-slate-900/60 backdrop-blur-xs transition-opacity animate-in fade-in duration-150"
      onClick={onClose}
    >
      <div
        data-testid="admin-search-palette"
        role="dialog"
        aria-modal="true"
        aria-label="Глобальный поиск"
        className="w-full max-w-4xl bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-gray-200 dark:border-slate-800 overflow-hidden flex flex-col max-h-[85vh] sm:max-h-[700px] animate-in zoom-in-95 duration-150"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search Header / Input Bar */}
        <div className="relative flex items-center px-4 py-3.5 border-b border-gray-100 dark:border-slate-800 shrink-0">
          <Search className="w-5 h-5 text-gray-400 dark:text-slate-500 mr-3 shrink-0" />
          <input
            ref={inputRef}
            data-testid="admin-search-input"
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Поиск по заказу, ZMU, ZMK, SKU, email или товару"
            aria-label="Поиск по заказу, ZMU, ZMK, SKU, email или товару"
            className="flex-1 bg-transparent text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 text-base focus:outline-hidden"
            autoComplete="off"
            spellCheck={false}
          />
          {isLoading && (
            <Loader2
              data-testid="admin-search-loading"
              className="w-5 h-5 text-indigo-500 animate-spin mr-2 shrink-0"
            />
          )}
          {query && !isLoading && (
            <button
              type="button"
              onClick={() => {
                setQuery('');
                inputRef.current?.focus();
              }}
              className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-slate-300 rounded-md hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors mr-1"
              title="Очистить"
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className="text-xs font-semibold px-2 py-1 bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 rounded-md hover:bg-gray-200 dark:hover:bg-slate-700 transition-colors"
          >
            ESC
          </button>
        </div>

        {/* Content Body: Catalog & Search Results */}
        <div ref={listRef} className="flex-1 overflow-y-auto p-3 divide-y divide-gray-100 dark:divide-slate-800/60">
          {/* Navigation Destination Catalog */}
          {filteredNavGroups.length > 0 && (
            <div data-testid="admin-nav-catalog" className="pb-4">
              {query.trim().length > 0 ? (
                <div className="px-3 py-1.5 text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-2">
                  Разделы ({flatNavItems.length})
                </div>
              ) : (
                <div className="px-3 py-1 text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
                  Каталог сервисов и навигации
                </div>
              )}
              <div
                className={`grid gap-4 ${
                  query.trim().length > 0
                    ? 'grid-cols-1 sm:grid-cols-2'
                    : 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3'
                }`}
              >
                {filteredNavGroups.map((group) => (
                  <div
                    key={group.title}
                    data-testid={`admin-nav-group-${group.title}`}
                    className="bg-slate-50 dark:bg-slate-800/50 rounded-xl p-3 border border-slate-200/80 dark:border-slate-700/60 flex flex-col"
                  >
                    <div className="text-[11px] font-bold text-slate-600 dark:text-slate-300 uppercase tracking-wider px-2 pb-2 mb-1.5 border-b border-slate-200 dark:border-slate-700/60 flex items-center justify-between">
                      <span>{group.title}</span>
                      <span className="text-[10px] font-semibold text-slate-500 dark:text-slate-400 bg-slate-200/70 dark:bg-slate-700/60 px-1.5 py-0.5 rounded-full">{group.items.length}</span>
                    </div>
                    <div className="space-y-0.5 flex-1">
                      {group.items.map((item) => {
                        currentFlatIndex++;
                        const itemIndex = currentFlatIndex;
                        const isSelected = itemIndex === activeIndex;
                        const isActive = isRouteActive(item.path);
                        const Icon = item.icon;

                        return (
                          <Link
                            key={item.path}
                            to={item.path}
                            ref={isSelected ? (activeItemRef as any) : null}
                            data-testid={`admin-nav-item-${item.path.replace(/\//g, '-')}`}
                            onClick={() => {
                              onClose();
                              resetSearch();
                            }}
                            onMouseEnter={() => setActiveIndex(itemIndex)}
                            className={`group flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-indigo-600 dark:focus-visible:ring-indigo-400 ${
                              isActive && isSelected
                                ? 'bg-indigo-600 text-white font-semibold shadow-xs ring-1 ring-indigo-700'
                                : isActive
                                ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-900 dark:text-indigo-200 font-semibold border-l-2 border-indigo-600'
                                : isSelected
                                ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-950 dark:text-indigo-100 border border-indigo-200/80 dark:border-indigo-800/60 font-semibold shadow-xs'
                                : 'text-slate-700 dark:text-slate-200 hover:bg-slate-200/70 dark:hover:bg-slate-700/60 hover:text-slate-900 dark:hover:text-white'
                            }`}
                          >
                            <div className="flex items-center min-w-0">
                              <Icon
                                className={`w-4 h-4 mr-2.5 shrink-0 transition-colors ${
                                  isActive && isSelected
                                    ? 'text-white'
                                    : isActive
                                    ? 'text-indigo-600 dark:text-indigo-400'
                                    : isSelected
                                    ? 'text-indigo-600 dark:text-indigo-400'
                                    : 'text-slate-500 dark:text-slate-400 group-hover:text-slate-700 dark:group-hover:text-slate-200'
                                }`}
                              />
                              <span className="truncate">
                                {item.name}
                              </span>
                            </div>
                            {item.count !== undefined && item.count > 0 && (
                              <span
                                className={`ml-2 px-1.5 py-0.5 rounded-full text-[10px] font-bold transition-colors ${
                                  isActive && isSelected
                                    ? 'bg-white/20 text-white'
                                    : isActive
                                    ? 'bg-indigo-100 dark:bg-indigo-900/60 text-indigo-700 dark:text-indigo-300'
                                    : isSelected
                                    ? 'bg-indigo-100 dark:bg-indigo-900/60 text-indigo-800 dark:text-indigo-300'
                                    : 'bg-slate-200/80 dark:bg-slate-700 text-slate-600 dark:text-slate-300 group-hover:bg-slate-300 dark:group-hover:bg-slate-600'
                                }`}
                              >
                                {item.count}
                              </span>
                            )}
                          </Link>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Initial / Short query helper when catalog has no groups (e.g. standalone test) */}
          {filteredNavGroups.length === 0 && query.trim().length < 2 && !error && (
            <div className="py-12 text-center text-gray-400 dark:text-slate-500 text-sm">
              <p>Введите номер заказа (ORD), единицу (ZMU), артикул (ZMK/SKU), email или название товара</p>
              <p className="text-xs text-gray-400 dark:text-slate-600 mt-1">Минимум 2 символа</p>
            </div>
          )}

          {/* Error state */}
          {error && (
            <div data-testid="admin-search-error" className="py-8 text-center px-4">
              <AlertCircle className="w-8 h-8 text-rose-500 mx-auto mb-2" />
              <p className="text-sm font-medium text-gray-800 dark:text-slate-200">{error}</p>
            </div>
          )}

          {/* No results state */}
          {query.trim().length >= 2 &&
            !isLoading &&
            !error &&
            results.length === 0 &&
            filteredNavGroups.length === 0 && (
              <div
                data-testid="admin-search-no-results"
                className="py-12 text-center text-gray-400 dark:text-slate-500 text-sm"
              >
                <p className="font-medium text-gray-700 dark:text-slate-300">Ничего не найдено</p>
                <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                  По запросу &laquo;{query.trim()}&raquo; совпадений не обнаружено
                </p>
              </div>
            )}

          {/* Grouped Search Results from API */}
          {groupedResults.length > 0 && (
            <div className="pt-4 border-t border-gray-100 dark:border-slate-800/60">
              {groupedResults.map((group) => (
                <div
                  key={group.key}
                  data-testid={`admin-search-group-${group.key}`}
                  className="py-2 first:pt-0 last:pb-0"
                >
                  <div className="px-3 py-1.5 text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                    {group.title}
                  </div>
                  <div className="space-y-0.5">
                    {group.items.map((item) => {
                      currentFlatIndex++;
                      const itemIndex = currentFlatIndex;
                      const isSelected = itemIndex === activeIndex;
                      const Icon = RESULT_ICONS[item.type] || Package;

                      return (
                        <button
                          key={`${item.type}:${item.id}:${item.canonicalIdentifier}`}
                          ref={isSelected ? (activeItemRef as any) : null}
                          data-testid={`admin-search-item-${item.type}-${item.id}`}
                          type="button"
                          onClick={() => handleSelect(item)}
                          onMouseEnter={() => setActiveIndex(itemIndex)}
                          className={`w-full flex items-center px-3 py-2.5 rounded-xl text-left transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-indigo-600 dark:focus-visible:ring-indigo-400 ${
                            isSelected
                              ? 'bg-indigo-50 dark:bg-slate-800 text-indigo-950 dark:text-white border-l-2 border-indigo-600'
                              : 'text-slate-700 dark:text-slate-300 hover:bg-slate-100/80 dark:hover:bg-slate-800/60 hover:text-slate-900 dark:hover:text-white'
                          }`}
                        >
                          <div
                            className={`p-2 rounded-lg mr-3 shrink-0 transition-colors ${
                              isSelected
                                ? 'bg-indigo-100 dark:bg-indigo-900/60 text-indigo-700 dark:text-indigo-300'
                                : 'bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400'
                            }`}
                          >
                            <Icon className="w-4 h-4" />
                          </div>
                          <div className="flex-1 min-w-0 pr-2">
                            <div className={`text-sm font-semibold truncate leading-snug ${isSelected ? 'text-indigo-950 dark:text-white' : 'text-slate-900 dark:text-slate-100'}`}>
                              {item.title}
                            </div>
                            {item.subtitle && (
                              <div className={`text-xs truncate mt-0.5 ${isSelected ? 'text-indigo-700/80 dark:text-slate-300' : 'text-slate-500 dark:text-slate-400'}`}>
                                {item.subtitle}
                              </div>
                            )}
                          </div>
                          {isSelected && (
                            <div className="shrink-0 text-indigo-600 dark:text-indigo-400 flex items-center text-xs font-medium">
                              <span className="hidden sm:inline mr-1">Перейти</span>
                              <CornerDownLeft className="w-3.5 h-3.5" />
                            </div>
                          )}
                        </button>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer Shortcut Hints */}
        <div className="px-4 py-2.5 bg-gray-50 dark:bg-slate-900/80 border-t border-gray-100 dark:border-slate-800 flex items-center justify-between text-xs text-gray-400 dark:text-slate-500 shrink-0">
          <div className="flex items-center space-x-3">
            <span>
              <kbd className="px-1.5 py-0.5 bg-gray-200 dark:bg-slate-800 rounded text-gray-700 dark:text-slate-300 font-mono">
                ↑
              </kbd>{' '}
              <kbd className="px-1.5 py-0.5 bg-gray-200 dark:bg-slate-800 rounded text-gray-700 dark:text-slate-300 font-mono">
                ↓
              </kbd>{' '}
              выбрать
            </span>
            <span>
              <kbd className="px-1.5 py-0.5 bg-gray-200 dark:bg-slate-800 rounded text-gray-700 dark:text-slate-300 font-mono">
                ↵
              </kbd>{' '}
              открыть
            </span>
            <span>
              <kbd className="px-1.5 py-0.5 bg-gray-200 dark:bg-slate-800 rounded text-gray-700 dark:text-slate-300 font-mono">
                Esc
              </kbd>{' '}
              закрыть
            </span>
          </div>
          <span className="hidden sm:inline text-[11px] text-gray-400 dark:text-slate-600">
            ZAMK Search & Navigation Shell
          </span>
        </div>
      </div>
    </div>
  );
}

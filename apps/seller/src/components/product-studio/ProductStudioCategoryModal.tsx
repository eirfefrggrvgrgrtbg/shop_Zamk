import { useState, useMemo, useEffect } from 'react';
import { getSellerCategories, type SellerCategory } from '@zamk/api-client';
import { Search, Folder, ChevronRight, X } from 'lucide-react';

export interface ProductStudioCategoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSelectCategory: (category: SellerCategory) => void;
  currentCategoryId?: string;
}

export function ProductStudioCategoryModal({
  isOpen,
  onClose,
  onSelectCategory,
  currentCategoryId,
}: ProductStudioCategoryModalProps) {
  const [categories, setCategories] = useState<SellerCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [activeParentId, setActiveParentId] = useState<string | null>(null);

  // Load categories when modal opens
  useEffect(() => {
    if (isOpen && categories.length === 0) {
      setLoading(true);
      getSellerCategories()
        .then((data) => setCategories(data || []))
        .catch((err) => console.error('Failed to load categories in modal:', err))
        .finally(() => setLoading(false));
    }
  }, [isOpen, categories.length]);

  // Reset search / parent when closing or opening
  useEffect(() => {
    if (!isOpen) {
      setSearch('');
      setActiveParentId(null);
    }
  }, [isOpen]);

  // Keydown Escape
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  // Category maps & leaf calculation
  const catMap = useMemo(() => {
    const map = new Map<string, SellerCategory>();
    categories.forEach((c) => map.set(c.id, c));
    return map;
  }, [categories]);

  const isParent = useMemo(() => {
    return new Set(categories.map((c) => c.parentId).filter(Boolean));
  }, [categories]);

  const leaves = useMemo(() => {
    return categories.filter((c) => !isParent.has(c.id));
  }, [categories, isParent]);

  const getPath = (cat: SellerCategory): string => {
    const parts = [cat.name];
    let curr = cat;
    while (curr.parentId && catMap.has(curr.parentId)) {
      curr = catMap.get(curr.parentId)!;
      parts.unshift(curr.name);
    }
    return parts.join(' › ');
  };

  const getPathArray = (cat: SellerCategory): SellerCategory[] => {
    const parts = [cat];
    let curr = cat;
    while (curr.parentId && catMap.has(curr.parentId)) {
      curr = catMap.get(curr.parentId)!;
      parts.unshift(curr);
    }
    return parts;
  };

  const filteredLeaves = useMemo(() => {
    if (!search.trim()) return [];
    const q = search.toLowerCase();
    return leaves.filter((l) => getPath(l).toLowerCase().includes(q));
  }, [search, leaves, catMap]);

  const viewCategories = useMemo(() => {
    if (search.trim()) return [];
    return categories
      .filter((c) => (c.parentId || null) === activeParentId)
      .sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0));
  }, [categories, activeParentId, search]);

  const activeBreadcrumbs = useMemo(() => {
    if (!activeParentId) return [];
    const curr = catMap.get(activeParentId);
    if (!curr) return [];
    return getPathArray(curr);
  }, [activeParentId, catMap]);

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="category-modal-title"
      data-testid="category-modal"
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="bg-white dark:bg-[#18181b] border border-gray-200 dark:border-white/10 rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden flex flex-col max-h-[85vh] animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="p-4 border-b border-gray-100 dark:border-white/10 flex items-center justify-between">
          <div>
            <h2 id="category-modal-title" className="text-base font-semibold text-gray-900 dark:text-white">
              Выберите категорию товара
            </h2>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
              Для настройки размеров и характеристик выберите конечную категорию
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть"
            className="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/5 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search */}
        <div className="p-3 border-b border-gray-100 dark:border-white/10 bg-gray-50/50 dark:bg-white/[0.02]">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Поиск категории (например: Футболки, Худи)..."
              data-testid="category-search-input"
              className="w-full pl-9 pr-4 py-2 bg-white dark:bg-[#202024] border border-gray-200 dark:border-white/10 rounded-lg text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto min-h-[300px] max-h-[420px]">
          {loading ? (
            <div className="p-8 text-center text-sm text-gray-500 dark:text-gray-400">
              Загрузка категорий...
            </div>
          ) : search.trim() ? (
            /* Search Results */
            filteredLeaves.length === 0 ? (
              <div className="p-12 text-center text-gray-500 dark:text-gray-400">
                <Search className="w-8 h-8 mx-auto mb-2 text-gray-300 dark:text-gray-600" />
                <p className="text-sm">Категории по запросу «{search}» не найдены</p>
              </div>
            ) : (
              <ul className="divide-y divide-gray-100 dark:divide-white/5">
                {filteredLeaves.map((cat) => {
                  const isSelected = cat.id === currentCategoryId;
                  return (
                    <li key={cat.id}>
                      <button
                        type="button"
                        onClick={() => {
                          onSelectCategory(cat);
                          onClose();
                        }}
                        data-testid={`category-leaf-${cat.id}`}
                        className={`w-full p-3.5 text-left flex items-center justify-between hover:bg-gray-50 dark:hover:bg-white/5 transition-colors group ${
                          isSelected ? 'bg-indigo-50/50 dark:bg-indigo-900/20' : ''
                        }`}
                      >
                        <span className="text-sm text-gray-700 dark:text-gray-300 group-hover:text-gray-900 dark:group-hover:text-white">
                          {getPath(cat)}
                        </span>
                        <div
                          className={`w-4 h-4 rounded-full border transition-colors flex items-center justify-center shrink-0 ml-2 ${
                            isSelected
                              ? 'border-indigo-600 bg-indigo-600'
                              : 'border-gray-300 dark:border-gray-600 group-hover:border-indigo-500'
                          }`}
                        >
                          {isSelected && <div className="w-1.5 h-1.5 rounded-full bg-white" />}
                        </div>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )
          ) : (
            /* Drill-Down Hierarchy */
            <div className="flex flex-col">
              {activeParentId && (
                <div className="flex items-center gap-1.5 px-4 py-2.5 border-b border-gray-100 dark:border-white/10 bg-gray-50/70 dark:bg-white/[0.02] text-xs overflow-x-auto shrink-0">
                  <button
                    type="button"
                    onClick={() => setActiveParentId(null)}
                    className="text-gray-500 dark:text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400 whitespace-nowrap font-medium"
                  >
                    Все категории
                  </button>
                  {activeBreadcrumbs.map((b, i) => (
                    <div key={b.id} className="flex items-center gap-1.5 whitespace-nowrap">
                      <ChevronRight className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                      <button
                        type="button"
                        onClick={() => setActiveParentId(b.id)}
                        className={`font-medium ${
                          i === activeBreadcrumbs.length - 1
                            ? 'text-gray-900 dark:text-white'
                            : 'text-gray-500 dark:text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400'
                        }`}
                      >
                        {b.name}
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <ul className="divide-y divide-gray-100 dark:divide-white/5">
                {viewCategories.map((cat) => {
                  const leaf = !isParent.has(cat.id);
                  const isSelected = cat.id === currentCategoryId;

                  return (
                    <li key={cat.id}>
                      <button
                        type="button"
                        onClick={() => {
                          if (leaf) {
                            onSelectCategory(cat);
                            onClose();
                          } else {
                            setActiveParentId(cat.id);
                          }
                        }}
                        data-testid={leaf ? `category-leaf-${cat.id}` : `category-parent-${cat.id}`}
                        className={`w-full p-3.5 text-left flex items-center justify-between hover:bg-gray-50 dark:hover:bg-white/5 transition-colors group ${
                          isSelected ? 'bg-indigo-50/50 dark:bg-indigo-900/20' : ''
                        }`}
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          {!leaf && (
                            <Folder className="w-4 h-4 text-gray-400 dark:text-gray-500 group-hover:text-indigo-600 dark:group-hover:text-indigo-400 shrink-0 transition-colors" />
                          )}
                          <span
                            className={`text-sm truncate ${
                              leaf
                                ? 'text-gray-800 dark:text-gray-200 group-hover:text-gray-900 dark:group-hover:text-white'
                                : 'font-medium text-gray-900 dark:text-white'
                            }`}
                          >
                            {cat.name}
                          </span>
                        </div>
                        {leaf ? (
                          <div
                            className={`w-4 h-4 rounded-full border transition-colors flex items-center justify-center shrink-0 ml-2 ${
                              isSelected
                                ? 'border-indigo-600 bg-indigo-600'
                                : 'border-gray-300 dark:border-gray-600 group-hover:border-indigo-500'
                            }`}
                          >
                            {isSelected && <div className="w-1.5 h-1.5 rounded-full bg-white" />}
                          </div>
                        ) : (
                          <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-700 dark:group-hover:text-white shrink-0 ml-2 transition-colors" />
                        )}
                      </button>
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-3 border-t border-gray-100 dark:border-white/10 bg-gray-50/50 dark:bg-white/[0.02] flex items-center justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-200/60 dark:hover:bg-white/10 rounded-lg transition-colors"
          >
            Отмена
          </button>
        </div>
      </div>
    </div>
  );
}

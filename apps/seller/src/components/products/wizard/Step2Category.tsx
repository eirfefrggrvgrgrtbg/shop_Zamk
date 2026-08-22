import { useState, useMemo } from 'react';
import type { WizardState } from './WizardState';
import type { SellerCategory } from '@zamk/api-client/src/seller';
import { ChevronRight, Search, Folder } from 'lucide-react';

export function Step2Category({ 
  state, 
  updateState, 
  categories, 
  onNext, 
  onPrev 
}: { 
  state: WizardState; 
  updateState: (u: Partial<WizardState>) => void; 
  categories: SellerCategory[]; 
  onNext: () => void; 
  onPrev: () => void; 
}) {
  const [showConfirm, setShowConfirm] = useState(false);
  const [pendingCategory, setPendingCategory] = useState('');
  const [search, setSearch] = useState('');
  const [activeParentId, setActiveParentId] = useState<string | null>(null);

  // Map to find parents quickly
  const catMap = useMemo(() => {
    const map = new Map<string, SellerCategory>();
    categories.forEach(c => map.set(c.id, c));
    return map;
  }, [categories]);

  // Find all leaf categories (no children)
  const isParent = useMemo(() => {
    return new Set(categories.map(c => c.parentId).filter(Boolean));
  }, [categories]);

  const leaves = useMemo(() => {
    return categories.filter(c => !isParent.has(c.id));
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
    return leaves.filter(l => getPath(l).toLowerCase().includes(q));
  }, [search, leaves, catMap]);

  const [isEditing, setIsEditing] = useState(!state.categoryId);

  const handleChange = (newId: string) => {
    if (state.categoryId && state.categoryId !== newId) {
      setPendingCategory(newId);
      setShowConfirm(true);
    } else {
      updateState({ categoryId: newId });
      setIsEditing(false);
      setSearch('');
    }
  };

  const confirmChange = () => {
    updateState({ 
      categoryId: pendingCategory,
      productAttributes: {},
      materialComposition: [],
      selectedSizeSystemId: '',
      selectedSizeValueIds: [],
      variants: [],
      sizeChartRows: {}
    });
    setShowConfirm(false);
    setIsEditing(false);
    setSearch('');
  };

  const selectedPath = state.categoryId && catMap.has(state.categoryId) ? getPath(catMap.get(state.categoryId)!) : '';

  // Current view categories for drill-down
  const viewCategories = useMemo(() => {
    if (search.trim()) return [];
    return categories.filter(c => (c.parentId || null) === activeParentId).sort((a, b) => a.sortOrder - b.sortOrder);
  }, [categories, activeParentId, search]);

  const activeBreadcrumbs = useMemo(() => {
    if (!activeParentId) return [];
    const curr = catMap.get(activeParentId);
    if (!curr) return [];
    return getPathArray(curr);
  }, [activeParentId, catMap]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-medium mb-1">Выберите категорию</h2>
        <p className="text-gray-500 text-sm">Правильная категория поможет покупателям быстрее найти ваш товар.</p>
      </div>

      {!isEditing && selectedPath ? (
        <div className="p-5 bg-gray-50 rounded-xl flex items-center justify-between">
          <div>
            <div className="text-xs text-gray-500 uppercase tracking-wider mb-1">Выбранная категория</div>
            <div className="font-medium text-gray-900">{selectedPath}</div>
          </div>
          <button 
            onClick={() => setIsEditing(true)}
            className="text-sm font-medium text-black underline underline-offset-4 hover:text-gray-600 transition-colors"
          >
            Изменить
          </button>
        </div>
      ) : (
        <div className="border border-gray-200 rounded-xl bg-white overflow-hidden shadow-sm">
          <div className="p-3 border-b border-gray-100 bg-white">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input 
                type="text" 
                value={search} 
                onChange={e => setSearch(e.target.value)} 
                placeholder="Поиск категории, например: Худи" 
                className="w-full pl-9 pr-4 py-2.5 bg-gray-50 border-none rounded-lg focus:ring-2 focus:ring-black outline-none transition-shadow text-sm"
              />
            </div>
          </div>

          {search.trim() ? (
            <div className="max-h-[400px] overflow-y-auto">
              {filteredLeaves.length === 0 ? (
                <div className="p-12 text-center text-gray-500">
                  <Search className="w-8 h-8 mx-auto mb-3 text-gray-300" />
                  <p>Категории по запросу «{search}» не найдены</p>
                </div>
              ) : (
                <ul className="divide-y divide-gray-100">
                  {filteredLeaves.map(c => (
                    <li 
                      key={c.id} 
                      onClick={() => handleChange(c.id)}
                      className="p-4 cursor-pointer hover:bg-gray-50 transition-colors flex items-center justify-between group"
                    >
                      <span className="text-gray-700 group-hover:text-black">{getPath(c)}</span>
                      <div className="w-4 h-4 rounded-full border border-gray-300 group-hover:border-black flex items-center justify-center"></div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ) : (
            <div className="flex flex-col h-[400px]">
              {activeParentId && (
                <div className="flex items-center gap-2 p-3 border-b border-gray-100 bg-white text-sm overflow-x-auto shrink-0">
                  <button 
                    onClick={() => setActiveParentId(null)}
                    className="text-gray-500 hover:text-black whitespace-nowrap"
                  >
                    Все категории
                  </button>
                  {activeBreadcrumbs.map((b, i) => (
                    <div key={b.id} className="flex items-center gap-2 whitespace-nowrap">
                      <ChevronRight className="w-4 h-4 text-gray-400" />
                      <button 
                        onClick={() => setActiveParentId(b.id)}
                        className={`${i === activeBreadcrumbs.length - 1 ? 'font-medium text-black' : 'text-gray-500 hover:text-black'}`}
                      >
                        {b.name}
                      </button>
                    </div>
                  ))}
                </div>
              )}
              
              <ul className="divide-y divide-gray-100 overflow-y-auto flex-1">
                {viewCategories.map(c => {
                  const leaf = !isParent.has(c.id);
                  
                  return (
                    <li 
                      key={c.id} 
                      onClick={() => leaf ? handleChange(c.id) : setActiveParentId(c.id)}
                      className="p-4 cursor-pointer hover:bg-gray-50 transition-colors flex items-center justify-between group"
                    >
                      <div className="flex items-center gap-3">
                        {!leaf && <Folder className="w-4 h-4 text-gray-400 group-hover:text-black transition-colors" />}
                        <span className="text-gray-800 group-hover:text-black">{c.name}</span>
                      </div>
                      {leaf ? (
                        <div className="w-4 h-4 rounded-full border border-gray-300 group-hover:border-black transition-colors flex items-center justify-center"></div>
                      ) : (
                        <ChevronRight className="w-5 h-5 text-gray-400 group-hover:text-black transition-colors" />
                      )}
                    </li>
                  );
                })}
              </ul>
            </div>
          )}
        </div>
      )}

      {showConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-xl p-6 max-w-sm w-full shadow-xl">
            <h3 className="text-xl font-medium mb-2">Сменить категорию?</h3>
            <p className="text-gray-600 text-sm mb-6">
              Смена категории сбросит уже введенные характеристики и варианты. Название, описание и фото будут сохранены.
            </p>
            <div className="flex justify-end gap-3">
              <button onClick={() => setShowConfirm(false)} className="px-4 py-2 border rounded-lg font-medium hover:bg-gray-50 transition-colors">Отмена</button>
              <button onClick={confirmChange} className="px-4 py-2 bg-black text-white rounded-lg font-medium hover:bg-gray-800 transition-colors">Сменить</button>
            </div>
          </div>
        </div>
      )}

      <div className="flex justify-between pt-6">
        <button onClick={onPrev} className="px-6 py-2.5 border rounded-lg font-medium hover:bg-gray-50 transition-colors">Назад</button>
        <button onClick={onNext} disabled={!state.categoryId} className="px-8 py-2.5 bg-black text-white rounded-lg font-medium hover:bg-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">Далее</button>
      </div>
    </div>
  );
}

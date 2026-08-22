import { useState, useMemo } from 'react';
import type { WizardState } from './WizardState';
import type { SellerCategory } from '@zamk/api-client/src/seller';
import { ChevronRight, Search, Check, Folder } from 'lucide-react';

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

  const handleChange = (newId: string) => {
    if (state.categoryId && state.categoryId !== newId) {
      setPendingCategory(newId);
      setShowConfirm(true);
    } else {
      updateState({ categoryId: newId });
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

      {selectedPath && (
        <div className="p-4 bg-green-50 border border-green-200 rounded-lg flex items-center justify-between">
          <div>
            <div className="text-xs text-green-700 font-medium uppercase tracking-wider mb-1">Выбранная категория</div>
            <div className="font-medium text-green-900">{selectedPath}</div>
          </div>
          <Check className="w-5 h-5 text-green-600" />
        </div>
      )}

      <div className="border rounded-xl bg-white overflow-hidden shadow-sm">
        <div className="p-3 border-b bg-gray-50/50">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input 
              type="text" 
              value={search} 
              onChange={e => setSearch(e.target.value)} 
              placeholder="Поиск категории, например: Худи" 
              className="w-full pl-9 pr-4 py-2 bg-white border rounded-lg focus:ring-2 focus:ring-black outline-none transition-shadow text-sm"
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
              <ul className="divide-y">
                {filteredLeaves.map(c => {
                  const isSelected = state.categoryId === c.id;
                  return (
                    <li 
                      key={c.id} 
                      onClick={() => handleChange(c.id)}
                      className={`p-4 cursor-pointer hover:bg-gray-50 transition-colors flex items-center justify-between group ${isSelected ? 'bg-blue-50/50' : ''}`}
                    >
                      <span className={`${isSelected ? 'font-medium text-blue-900' : 'text-gray-700'}`}>{getPath(c)}</span>
                      <div className={`w-4 h-4 rounded-full border flex items-center justify-center ${isSelected ? 'border-blue-600' : 'border-gray-300 group-hover:border-black'}`}>
                        {isSelected && <div className="w-2 h-2 bg-blue-600 rounded-full" />}
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        ) : (
          <div className="flex flex-col h-[400px]">
            {activeParentId && (
              <div className="flex items-center gap-2 p-3 border-b bg-gray-50 text-sm overflow-x-auto shrink-0">
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
            
            <ul className="divide-y overflow-y-auto flex-1">
              {viewCategories.map(c => {
                const leaf = !isParent.has(c.id);
                const isSelected = state.categoryId === c.id;
                
                return (
                  <li 
                    key={c.id} 
                    onClick={() => leaf ? handleChange(c.id) : setActiveParentId(c.id)}
                    className={`p-4 cursor-pointer hover:bg-gray-50 transition-colors flex items-center justify-between group ${isSelected ? 'bg-blue-50/50' : ''}`}
                  >
                    <div className="flex items-center gap-3">
                      {!leaf && <Folder className="w-4 h-4 text-gray-400 group-hover:text-black" />}
                      <span className={`${isSelected ? 'font-medium text-blue-900' : 'text-gray-800'}`}>{c.name}</span>
                    </div>
                    {leaf ? (
                      <div className={`w-4 h-4 rounded-full border flex items-center justify-center ${isSelected ? 'border-blue-600' : 'border-gray-300 group-hover:border-black'}`}>
                        {isSelected && <div className="w-2 h-2 bg-blue-600 rounded-full" />}
                      </div>
                    ) : (
                      <ChevronRight className="w-5 h-5 text-gray-400 group-hover:text-black" />
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        )}
      </div>

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

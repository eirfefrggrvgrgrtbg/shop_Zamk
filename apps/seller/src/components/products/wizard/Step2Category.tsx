import { useState, useMemo } from 'react';
import type { WizardState } from './WizardState';
import type { SellerCategory } from '@zamk/api-client/src/seller';

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

  // Map to find parents quickly
  const catMap = useMemo(() => {
    const map = new Map<string, SellerCategory>();
    categories.forEach(c => map.set(c.id, c));
    return map;
  }, [categories]);

  // Find all leaf categories (no children)
  const leaves = useMemo(() => {
    const isParent = new Set(categories.map(c => c.parentId).filter(Boolean));
    return categories.filter(c => !isParent.has(c.id));
  }, [categories]);

  const getPath = (cat: SellerCategory): string => {
    const parts = [cat.name];
    let curr = cat;
    while (curr.parentId && catMap.has(curr.parentId)) {
      curr = catMap.get(curr.parentId)!;
      parts.unshift(curr.name);
    }
    return parts.join(' › ');
  };

  const filteredLeaves = useMemo(() => {
    if (!search.trim()) return leaves;
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

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Категория</h2>
      
      {selectedPath && (
        <div className="p-3 bg-gray-50 border rounded text-sm mb-4">
          <span className="text-gray-500">Выбрано:</span> <span className="font-medium">{selectedPath}</span>
        </div>
      )}

      <div>
        <label className="block text-sm mb-2">Поиск категории</label>
        <input 
          type="text" 
          value={search} 
          onChange={e => setSearch(e.target.value)} 
          placeholder="Например: Худи" 
          className="w-full border p-2 rounded mb-4"
        />

        <div className="border rounded overflow-y-auto max-h-64">
          {filteredLeaves.length === 0 ? (
            <div className="p-4 text-center text-gray-500 text-sm">Ничего не найдено</div>
          ) : (
            <ul className="divide-y text-sm">
              {filteredLeaves.map(c => {
                const isSelected = state.categoryId === c.id;
                return (
                  <li 
                    key={c.id} 
                    onClick={() => handleChange(c.id)}
                    className={`p-3 cursor-pointer hover:bg-gray-50 ${isSelected ? 'bg-black/5 font-medium' : ''}`}
                  >
                    {getPath(c)}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>

      {showConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg p-6 max-w-sm w-full">
            <h3 className="text-lg font-medium">Сменить категорию?</h3>
            <p className="mt-2 text-sm text-gray-500">
              Смена категории сбросит уже введенные характеристики и варианты. Название, описание и фото будут сохранены.
            </p>
            <div className="mt-4 flex justify-end gap-3">
              <button onClick={() => setShowConfirm(false)} className="px-3 py-1.5 border rounded">Отмена</button>
              <button onClick={confirmChange} className="px-3 py-1.5 bg-black text-white rounded">Сменить</button>
            </div>
          </div>
        </div>
      )}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} disabled={!state.categoryId} className="px-4 py-2 bg-black text-white rounded disabled:opacity-50">Далее</button>
      </div>
    </div>
  );
}

import { useState } from 'react';
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

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Категория</h2>
      <div>
        <label className="block text-sm mb-2">Выберите категорию</label>
        <select 
          value={state.categoryId} 
          onChange={e => handleChange(e.target.value)} 
          className="block w-full border border-gray-300 rounded-md p-2"
        >
          <option value="">Не выбрана</option>
          {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>

      {showConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4">
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

import { useState, useEffect } from 'react';
import type { WizardState } from './WizardState';
import { getSellerMaterials, getSellerDictionaryValues, type SellerCategorySchema, type SellerMaterial } from '@zamk/api-client/src/seller';

export function Step3Attributes({ 
  state, 
  updateState, 
  schema, 
  onNext, 
  onPrev 
}: { 
  state: WizardState; 
  updateState: (u: Partial<WizardState>) => void; 
  schema: SellerCategorySchema | null;
  onNext: () => void; 
  onPrev: () => void; 
}) {
  const [materials, setMaterials] = useState<SellerMaterial[]>([]);
  const [dicts, setDicts] = useState<Record<string, any[]>>({});

  useEffect(() => {
    getSellerMaterials().then(setMaterials).catch(console.error);
  }, []);

  useEffect(() => {
    if (!schema) return;
    schema.attributes.forEach(attr => {
      if (attr.valueType === 'DICTIONARY' && attr.dictionaryId && !dicts[attr.dictionaryId]) {
        getSellerDictionaryValues(attr.dictionaryId).then(vals => {
          setDicts(prev => ({ ...prev, [attr.dictionaryId!]: vals }));
        }).catch(console.error);
      }
    });
  }, [schema]);

  const pAttrs = schema?.attributes.filter(a => a.scope === 'PRODUCT') || [];
  const compAttr = pAttrs.find(a => a.valueSource === 'MATERIAL_COMPOSITION');
  const genericAttrs = pAttrs.filter(a => a.valueSource !== 'MATERIAL_COMPOSITION');

  const addComp = () => {
    updateState({ materialComposition: [...state.materialComposition, { materialId: '', percentage: 0 }] });
  };
  const updateComp = (i: number, matId: string, pct: number) => {
    const nw = [...state.materialComposition];
    nw[i] = { materialId: matId, percentage: pct };
    updateState({ materialComposition: nw });
  };
  const removeComp = (i: number) => {
    updateState({ materialComposition: state.materialComposition.filter((_, idx) => idx !== i) });
  };

  const compTotal = state.materialComposition.reduce((sum, c) => sum + (c.percentage || 0), 0);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Характеристики</h2>
      
      {compAttr && (
        <div className="p-4 border rounded-md">
          <h3 className="font-medium mb-4">Состав {compAttr.required && '*'}</h3>
          {state.materialComposition.map((c, i) => (
            <div key={i} className="flex gap-2 mb-2">
              <select value={c.materialId} onChange={e => updateComp(i, e.target.value, c.percentage)} className="flex-1 border p-2 rounded">
                <option value="">Выберите материал</option>
                {materials.map(m => <option key={m.id} value={m.id}>{m.nameRu}</option>)}
              </select>
              <input type="number" min="0" max="100" value={c.percentage || ''} onChange={e => updateComp(i, c.materialId, Number(e.target.value))} className="w-24 border p-2 rounded" placeholder="%" />
              <button onClick={() => removeComp(i)} className="text-red-500 px-2">Удалить</button>
            </div>
          ))}
          <button onClick={addComp} className="text-sm text-blue-600">+ Добавить материал</button>
          <div className="mt-2 text-sm text-gray-500">Итого: {compTotal}%</div>
        </div>
      )}

      {genericAttrs.map(attr => (
        <div key={attr.id}>
          <label className="block text-sm">{attr.nameRu} {attr.required && '*'}</label>
          {attr.valueType === 'DICTIONARY' && (
            <select 
              value={state.productAttributes[attr.id] as string || ''}
              onChange={e => updateState({ productAttributes: { ...state.productAttributes, [attr.id]: e.target.value }})}
              className="mt-1 block w-full border p-2 rounded"
            >
              <option value="">Не выбрано</option>
              {(dicts[attr.dictionaryId || ''] || []).map(d => <option key={d.id} value={d.id}>{d.nameRu}</option>)}
            </select>
          )}
          {attr.valueType === 'TEXT' && (
            <input 
              type="text" 
              value={state.productAttributes[attr.id] as string || ''}
              onChange={e => updateState({ productAttributes: { ...state.productAttributes, [attr.id]: e.target.value }})}
              className="mt-1 block w-full border p-2 rounded"
            />
          )}
          {attr.valueType === 'NUMBER' && (
            <input 
              type="number" 
              value={state.productAttributes[attr.id] as number || ''}
              onChange={e => updateState({ productAttributes: { ...state.productAttributes, [attr.id]: Number(e.target.value) }})}
              className="mt-1 block w-full border p-2 rounded"
            />
          )}
          {attr.valueType === 'BOOLEAN' && (
            <input 
              type="checkbox" 
              checked={state.productAttributes[attr.id] as boolean || false}
              onChange={e => updateState({ productAttributes: { ...state.productAttributes, [attr.id]: e.target.checked }})}
              className="mt-1"
            />
          )}
        </div>
      ))}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}

import { useState, useEffect } from 'react';
import type { WizardState, VariantDraft } from './WizardState';
import { getSellerColors, getSellerSizeValues, getSellerDictionaryValues, type SellerCategorySchema, type SellerColor, type SellerSizeValue } from '@zamk/api-client/src/seller';

export function Step4Variants({ 
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
  const [colors, setColors] = useState<SellerColor[]>([]);
  const [sizeValues, setSizeValues] = useState<SellerSizeValue[]>([]);
  const [dicts, setDicts] = useState<Record<string, any[]>>({});

  useEffect(() => {
    getSellerColors().then(setColors).catch(console.error);
  }, []);

  const hasColor = (schema?.attributes || []).some(a => a.valueSource === 'VARIANT_COLOR') || false;
  const hasSize = (schema?.attributes || []).some(a => a.valueSource === 'VARIANT_SIZE') || false;
  const allowedSystems = schema?.allowedSizeSystems || [];

  const genericVariantAttrs = (schema?.attributes || []).filter(a => a.scope === 'VARIANT' && !a.variantAxis) || [];

  useEffect(() => {
    if (!schema) return;
    genericVariantAttrs.forEach(attr => {
      if (attr.valueSource === 'DICTIONARY' && attr.dictionaryId && !dicts[attr.dictionaryId]) {
        getSellerDictionaryValues(attr.dictionaryId).then(vals => {
          setDicts(prev => ({ ...prev, [attr.dictionaryId!]: vals }));
        }).catch(console.error);
      }
    });
  }, [schema]);

  useEffect(() => {
    if (allowedSystems.length === 1 && !state.selectedSizeSystemId) {
      updateState({ selectedSizeSystemId: allowedSystems[0].id });
    }
  }, [allowedSystems, state.selectedSizeSystemId]);

  useEffect(() => {
    if (state.selectedSizeSystemId) {
      getSellerSizeValues(state.selectedSizeSystemId).then(setSizeValues).catch(console.error);
    } else {
      setSizeValues([]);
    }
  }, [state.selectedSizeSystemId]);

  const toggleColor = (id: string) => {
    const nw = state.selectedColorIds.includes(id) 
      ? state.selectedColorIds.filter(c => c !== id)
      : [...state.selectedColorIds, id];
    updateState({ selectedColorIds: nw });
  };

  const toggleSize = (id: string) => {
    const nw = state.selectedSizeValueIds.includes(id)
      ? state.selectedSizeValueIds.filter(s => s !== id)
      : [...state.selectedSizeValueIds, id];
    updateState({ selectedSizeValueIds: nw });
  };

  const updateShade = (colorId: string, shade: string) => {
    updateState({ shadeNamesByColor: { ...state.shadeNamesByColor, [colorId]: shade }});
  };

  const regenerateMatrix = () => {
    const newVariants: VariantDraft[] = [];
    const colorIds = hasColor && state.selectedColorIds.length > 0 ? state.selectedColorIds : [undefined];
    const sizeIds = hasSize && state.selectedSizeValueIds.length > 0 ? state.selectedSizeValueIds : [undefined];

    colorIds.forEach(c => {
      sizeIds.forEach(s => {
        const existing = state.variants.find(v => v.colorId === c && v.sizeValueId === s);
        if (existing) {
          newVariants.push(existing);
        } else {
          newVariants.push({
            id: undefined,
            colorId: c,
            sizeValueId: s,
            active: true,
            sellerSku: '',
            barcode: '',
            attributes: {}
          });
        }
      });
    });
    updateState({ variants: newVariants });
  };

  const updateVariantAttr = (vIdx: number, attrId: string, value: any) => {
    const nw = [...state.variants];
    if (!nw[vIdx].attributes) nw[vIdx].attributes = {};
    nw[vIdx].attributes![attrId] = value;
    updateState({ variants: nw });
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Варианты</h2>

      {hasColor && (
        <div>
          <h3 className="font-medium mb-2">Цвета</h3>
          <div className="flex flex-wrap gap-2">
            {colors.map(c => (
              <button 
                key={c.id} 
                onClick={() => toggleColor(c.id)}
                className={`flex items-center gap-2 border rounded-full px-3 py-1 ${state.selectedColorIds.includes(c.id) ? 'border-black ring-1 ring-black' : 'border-gray-300'}`}
              >
                <span className="w-4 h-4 rounded-full border border-gray-200" style={{ backgroundColor: c.hexValue }} />
                <span className="text-sm">{c.nameRu}</span>
              </button>
            ))}
          </div>
          {state.selectedColorIds.length > 0 && (
            <div className="mt-4 space-y-2">
              {state.selectedColorIds.map(cId => {
                const c = colors.find(x => x.id === cId);
                return (
                  <div key={cId} className="flex items-center gap-4">
                    <span className="text-sm w-32">{c?.nameRu}</span>
                    <input type="text" placeholder="Название оттенка (опционально)" value={state.shadeNamesByColor[cId] || ''} onChange={e => updateShade(cId, e.target.value)} className="border p-1 text-sm rounded w-64" />
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {hasSize && (
        <div>
          <h3 className="font-medium mb-2">Размеры</h3>
          {allowedSystems.length > 1 && (
            <select value={state.selectedSizeSystemId} onChange={e => updateState({ selectedSizeSystemId: e.target.value, selectedSizeValueIds: [] })} className="mb-4 block border p-2 rounded">
              <option value="">Выберите систему размеров</option>
              {allowedSystems.map(s => <option key={s.id} value={s.id}>{s.nameRu}</option>)}
            </select>
          )}
          {sizeValues.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {sizeValues.map(s => (
                <button 
                  key={s.id}
                  onClick={() => toggleSize(s.id)}
                  className={`border rounded px-3 py-1 text-sm ${state.selectedSizeValueIds.includes(s.id) ? 'bg-black text-white border-black' : 'border-gray-300'}`}
                >
                  {s.value}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="pt-4">
        <button onClick={regenerateMatrix} className="px-4 py-2 border rounded text-sm bg-gray-50">Сгенерировать комбинации</button>
      </div>

      {state.variants.length > 0 && (
        <div className="mt-4 space-y-4">
          {state.variants.map((v, i) => {
            const c = colors.find(x => x.id === v.colorId);
            const s = sizeValues.find(x => x.id === v.sizeValueId);
            return (
              <div key={i} className="border p-4 rounded bg-gray-50">
                <div className="flex items-center justify-between font-medium">
                  <span>{c?.nameRu || ''} {s?.value ? `- Размер ${s.value}` : ''}</span>
                  <label className="flex items-center gap-2 text-sm font-normal">
                    <input 
                      type="checkbox" 
                      checked={v.active} 
                      onChange={e => {
                        const nw = [...state.variants];
                        nw[i].active = e.target.checked;
                        updateState({ variants: nw });
                      }} 
                    />
                    Активен
                  </label>
                </div>
                
                {genericVariantAttrs.length > 0 && v.active && (
                  <div className="mt-4 space-y-3 pl-4 border-l-2">
                    {genericVariantAttrs.map(attr => (
                      <div key={attr.id}>
                        <label className="block text-xs font-medium text-gray-700">{attr.nameRu} {attr.required && '*'}</label>
                        {attr.valueSource === 'DICTIONARY' && attr.valueType === 'ENUM' && (
                          <select 
                            value={(v.attributes && v.attributes[attr.id] as string) || ''}
                            onChange={e => updateVariantAttr(i, attr.id, e.target.value)}
                            className="mt-1 block w-full border p-1 rounded text-sm bg-white"
                          >
                            <option value="">Не выбрано</option>
                            {(dicts[attr.dictionaryId || ''] || []).map(d => <option key={d.id} value={d.id}>{d.nameRu}</option>)}
                          </select>
                        )}
                        {attr.valueSource === 'DICTIONARY' && attr.valueType === 'MULTI_ENUM' && (
                          <select 
                            multiple
                            value={(v.attributes && v.attributes[attr.id] as string[]) || []}
                            onChange={e => {
                              const values = Array.from(e.target.selectedOptions, option => option.value);
                              updateVariantAttr(i, attr.id, values);
                            }}
                            className="mt-1 block w-full border p-1 rounded text-sm bg-white h-24"
                          >
                            {(dicts[attr.dictionaryId || ''] || []).map(d => <option key={d.id} value={d.id}>{d.nameRu}</option>)}
                          </select>
                        )}
                        {attr.valueSource !== 'DICTIONARY' && attr.valueType === 'TEXT' && (
                          <input 
                            type="text" 
                            value={(v.attributes && v.attributes[attr.id] as string) || ''}
                            onChange={e => updateVariantAttr(i, attr.id, e.target.value)}
                            className="mt-1 block w-full border p-1 rounded text-sm bg-white"
                          />
                        )}
                        {attr.valueSource !== 'DICTIONARY' && attr.valueType === 'NUMBER' && (
                          <input 
                            type="number" 
                            value={(v.attributes && v.attributes[attr.id] as number) || ''}
                            onChange={e => updateVariantAttr(i, attr.id, Number(e.target.value))}
                            className="mt-1 block w-full border p-1 rounded text-sm bg-white"
                          />
                        )}
                        {attr.valueSource !== 'DICTIONARY' && attr.valueType === 'BOOLEAN' && (
                          <input 
                            type="checkbox" 
                            checked={(v.attributes && v.attributes[attr.id] as boolean) || false}
                            onChange={e => updateVariantAttr(i, attr.id, e.target.checked)}
                            className="mt-1"
                          />
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}

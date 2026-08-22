import { useState, useEffect } from 'react';
import type { WizardState } from './WizardState';
import { getSellerColors, getSellerSizeValues, type SellerColor, type SellerSizeValue } from '@zamk/api-client/src/seller';

export function Step7Pricing({ 
  state, 
  updateState, 
  onNext, 
  onPrev 
}: { 
  state: WizardState; 
  updateState: (u: Partial<WizardState>) => void; 
  onNext: () => void; 
  onPrev: () => void; 
}) {
  const [colors, setColors] = useState<SellerColor[]>([]);
  const [sizeValues, setSizeValues] = useState<SellerSizeValue[]>([]);
  const [bulkPrice, setBulkPrice] = useState('');

  useEffect(() => {
    getSellerColors().then(setColors).catch(console.error);
    if (state.selectedSizeSystemId) {
      getSellerSizeValues(state.selectedSizeSystemId).then(setSizeValues).catch(console.error);
    }
  }, [state.selectedSizeSystemId]);

  const applyBulkPrice = () => {
    const val = Number(bulkPrice);
    if (val > 0) {
      const nw = state.variants.map(v => ({ ...v, priceCents: val }));
      updateState({ variants: nw });
    }
  };

  const generateSkus = () => {
    const nw = state.variants.map(v => {
      if (v.sellerSku) return v;
      const c = colors.find(x => x.id === v.colorId);
      const s = sizeValues.find(x => x.id === v.sizeValueId);
      const cCode = c ? c.code : 'XX';
      const sCode = s ? s.value : 'XX';
      return { ...v, sellerSku: `ZMK-${cCode}-${sCode}-${Math.floor(Math.random() * 1000)}` };
    });
    updateState({ variants: nw });
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Цена и идентификаторы</h2>
      
      <div className="p-4 border rounded bg-gray-50 flex gap-4 items-end">
        <div>
          <label className="block text-sm mb-1">Применить цену ко всем вариантам</label>
          <input type="number" value={bulkPrice} onChange={e => setBulkPrice(e.target.value)} className="border p-2 rounded w-48" placeholder="Цена в копейках" />
        </div>
        <button onClick={applyBulkPrice} className="px-4 py-2 bg-white border rounded shadow-sm hover:bg-gray-100">Применить</button>
        <div className="flex-1 text-right">
          <button onClick={generateSkus} className="px-4 py-2 bg-white border rounded shadow-sm hover:bg-gray-100">Сгенерировать артикулы</button>
        </div>
      </div>

      <div className="overflow-x-auto border rounded mt-4">
        <table className="w-full text-left text-sm whitespace-nowrap">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-2 border-b">Вариант</th>
              <th className="p-2 border-b">Артикул продавца *</th>
              <th className="p-2 border-b">Штрихкод</th>
              <th className="p-2 border-b">Цена (копейки) *</th>
            </tr>
          </thead>
          <tbody>
            {state.variants.filter(v => v.active).map((v, i) => {
              const c = colors.find(x => x.id === v.colorId);
              const s = sizeValues.find(x => x.id === v.sizeValueId);
              const name = [c?.nameRu, s?.value].filter(Boolean).join(' / ') || 'Базовый';
              
              const updateV = (field: string, val: any) => {
                const nw = [...state.variants];
                const activeIdx = state.variants.findIndex(x => x === v);
                nw[activeIdx] = { ...nw[activeIdx], [field]: val };
                updateState({ variants: nw });
              };

              return (
                <tr key={i} className="border-b last:border-0">
                  <td className="p-2 font-medium">{name}</td>
                  <td className="p-2">
                    <input type="text" value={v.sellerSku || ''} onChange={e => updateV('sellerSku', e.target.value)} className="border p-1 rounded w-32" />
                  </td>
                  <td className="p-2">
                    <input type="text" value={v.barcode || ''} onChange={e => updateV('barcode', e.target.value)} className="border p-1 rounded w-32" placeholder="ZAMK сгенерирует" />
                  </td>
                  <td className="p-2">
                    <input type="number" value={v.priceCents || ''} onChange={e => updateV('priceCents', Number(e.target.value))} className="border p-1 rounded w-32" />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}

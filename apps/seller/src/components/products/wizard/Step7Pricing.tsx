import { useState, useEffect } from 'react';
import type { WizardState } from './WizardState';
import { getSellerColors, getSellerSizeValues, generateSellerSKUs, type SellerColor, type SellerSizeValue } from '@zamk/api-client/src/seller';
import { Loader2 } from 'lucide-react';

export function Step7Pricing({
  state,
  updateState,
  onNext,
  onPrev,
  onError
}: {
  state: WizardState;
  updateState: (u: Partial<WizardState>) => void;
  onNext: () => void;
  onPrev: () => void;
  onError?: (msg: string) => void;
}) {
  const [colors, setColors] = useState<SellerColor[]>([]);
  const [sizeValues, setSizeValues] = useState<SellerSizeValue[]>([]);
  const [bulkPriceRub, setBulkPriceRub] = useState('');
  const [generatingSkus, setGeneratingSkus] = useState(false);


  useEffect(() => {
    getSellerColors().then(setColors).catch(console.error);
    if (state.selectedSizeSystemId) {
      getSellerSizeValues(state.selectedSizeSystemId).then(setSizeValues).catch(console.error);
    }
  }, [state.selectedSizeSystemId]);

  const applyBulkPrice = () => {
    const val = Number(bulkPriceRub);
    if (val > 0) {
      const cents = Math.round(val * 100);
      const nw = state.variants.map(v => ({ ...v, priceCents: cents }));
      updateState({ variants: nw });
    }
  };

  const generateSkus = async () => {
    try {
      setGeneratingSkus(true);
      const neededCount = state.variants.filter(v => v.active && !v.sellerSku).length;
      if (neededCount === 0) return;

      const res = await generateSellerSKUs(neededCount);

      let skuIdx = 0;
      const nw = state.variants.map(v => {
        if (!v.active || v.sellerSku) return v;
        const assignedSku = res.skus[skuIdx++];
        return { ...v, sellerSku: assignedSku };
      });
      updateState({ variants: nw });
    } catch (err: any) {
      onError?.('Ошибка генерации артикулов: ' + err.message);
    } finally {
      setGeneratingSkus(false);
    }
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Цена и идентификаторы</h2>

      <div className="p-4 border rounded bg-gray-50 flex gap-4 items-end">
        <div>
          <label className="block text-sm mb-1">Применить цену ко всем вариантам</label>
          <div className="relative">
            <input
              type="number"
              value={bulkPriceRub}
              onChange={e => setBulkPriceRub(e.target.value)}
              className="border p-2 rounded w-48 pr-8"
              placeholder="Цена (₽)"
            />
            <span className="absolute right-3 top-2.5 text-gray-500">₽</span>
          </div>
        </div>
        <button onClick={applyBulkPrice} className="px-4 py-2 bg-white border rounded shadow-sm hover:bg-gray-100">Применить</button>
        <div className="flex-1 text-right">
          <button onClick={generateSkus} disabled={generatingSkus} className="px-4 py-2 bg-white border rounded shadow-sm hover:bg-gray-100 flex items-center justify-center ml-auto">
            {generatingSkus ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
            Сгенерировать артикулы
          </button>
        </div>
      </div>

      <div className="overflow-x-auto border rounded mt-4">
        <table className="w-full text-left text-sm whitespace-nowrap">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-2 border-b">Вариант</th>
              <th className="p-2 border-b">Артикул продавца *</th>
              <th className="p-2 border-b min-w-[200px]">Штрихкод</th>
              <th className="p-2 border-b">Цена (₽) *</th>
            </tr>
          </thead>
          <tbody>
            {state.variants.filter(v => v.active).map((v, i) => {
              const c = colors.find(x => x.id === v.colorId);
              const s = sizeValues.find(x => x.id === v.sizeValueId);
              const name = [c?.nameRu, s?.value].filter(Boolean).join(' / ') || 'Базовый';

              const updateV = (field: keyof typeof v, val: any) => {
                const nw = [...state.variants];

                nw[activeIdx] = { ...nw[activeIdx], [field]: val };
                updateState({ variants: nw });
              };

              const activeIdx = state.variants.findIndex(x => x === v);

              return (
                <tr key={i} className="border-b last:border-0">
                  <td className="p-2 font-medium">{name}</td>
                  <td className="p-2">
                    <input type="text" value={v.sellerSku || ''} onChange={e => updateV('sellerSku', e.target.value)} className="border p-1 rounded w-32" />
                  </td>
                  <td className="p-2">
                    {v.barcode ? (
                      <div className="flex flex-col">
                        <span className="font-mono text-xs">{v.barcode}</span>
                        <span className="text-[10px] text-gray-400">Штрихкод ZAMK</span>
                      </div>
                    ) : (
                      <span className="text-gray-500 italic text-xs">Создан автоматически</span>
                    )}
                  </td>
                  <td className="p-2">
                    <div className="relative w-32">
                      <input
                        type="number"
                        value={v.priceCents ? Math.round(v.priceCents / 100) : ''}
                        onChange={e => updateV('priceCents', Math.round(Number(e.target.value) * 100))}
                        className="border p-1 rounded w-full pr-6"
                      />
                      <span className="absolute right-2 top-1.5 text-gray-500 text-xs">₽</span>
                    </div>
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

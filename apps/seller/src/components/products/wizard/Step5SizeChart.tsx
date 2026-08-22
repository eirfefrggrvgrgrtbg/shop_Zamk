import { useEffect, useState } from 'react';
import type { WizardState } from './WizardState';
import { getSellerSizeValues, type SellerCategorySchema, type SellerSizeValue } from '@zamk/api-client/src/seller';

export function Step5SizeChart({ 
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
  const [sizeValues, setSizeValues] = useState<SellerSizeValue[]>([]);

  useEffect(() => {
    if (state.selectedSizeSystemId) {
      getSellerSizeValues(state.selectedSizeSystemId).then(setSizeValues).catch(console.error);
    }
  }, [state.selectedSizeSystemId]);

  if (!schema?.sizeChartRequired) {
    return (
      <div className="space-y-6">
        <h2 className="text-xl font-medium">Таблица размеров</h2>
        <p className="text-gray-500">Для этой категории таблица размеров не требуется.</p>
        <div className="flex justify-between pt-4">
          <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
          <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
        </div>
      </div>
    );
  }

  const fields = [...(schema?.sizeChartFields || [])].sort((a, b) => a.sortOrder - b.sortOrder);

  const updateCell = (sizeId: string, fieldCode: string, val: number) => {
    const nw = { ...state.sizeChartRows };
    if (!nw[sizeId]) nw[sizeId] = {};
    nw[sizeId][fieldCode] = val;
    updateState({ sizeChartRows: nw });
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Таблица размеров</h2>
      <div className="overflow-x-auto border rounded">
        <table className="w-full text-left text-sm whitespace-nowrap">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-2 border-b">Размер</th>
              {fields.map(f => (
                <th key={f.code} className="p-2 border-b">
                  {f.name} {f.unit && `(${f.unit})`} {f.isRequired && '*'}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {[...state.selectedSizeValueIds].sort((a,b) => {
              const sa = sizeValues.find(x => x.id === a);
              const sb = sizeValues.find(x => x.id === b);
              return (sa?.sortOrder || 0) - (sb?.sortOrder || 0);
            }).map(sId => {
              const s = sizeValues.find(x => x.id === sId);
              if (!s) return null;
              return (
                <tr key={sId} className="border-b last:border-0 hover:bg-gray-50/50">
                  <td className="p-2 font-medium bg-gray-50/50">{s.value}</td>
                  {fields.map(f => (
                    <td key={f.code} className="p-2">
                      <input 
                        type="text"
                        inputMode="numeric"
                        pattern="[0-9]*"
                        value={state.sizeChartRows[sId]?.[f.code] || ''}
                        onChange={e => {
                          const val = e.target.value.replace(/[^0-9]/g, '');
                          if (val) updateCell(sId, f.code, parseInt(val, 10));
                          else {
                            const nw = { ...state.sizeChartRows };
                            if (nw[sId]) delete nw[sId][f.code];
                            updateState({ sizeChartRows: nw });
                          }
                        }}
                        className="border border-gray-200 rounded p-1.5 w-24 text-center focus:ring-1 focus:ring-black outline-none"
                      />
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {state.selectedSizeValueIds.length === 0 && (
        <p className="text-sm text-gray-500">Сначала выберите размеры на предыдущем шаге.</p>
      )}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}

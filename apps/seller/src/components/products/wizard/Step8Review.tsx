import type { WizardState } from './WizardState';

export function Step8Review({ 
  state, 
  submitting,
  onSubmit, 
  onPrev 
}: { 
  state: WizardState; 
  submitting?: boolean;
  onSubmit: () => void; 
  onPrev: () => void; 
}) {
  const activeVariants = state.variants.filter(v => v.active);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Проверка</h2>
      
      <div className="bg-gray-50 border rounded p-6 space-y-6">
        <div>
          <h3 className="text-sm text-gray-500 font-medium">Основное</h3>
          <p className="mt-1 font-medium">{state.title || '-'}</p>
          <p className="text-sm mt-2 whitespace-pre-wrap">{state.description || 'Нет описания'}</p>
        </div>

        <div>
          <h3 className="text-sm text-gray-500 font-medium mb-2">Варианты ({activeVariants.length})</h3>
          {activeVariants.length > 0 ? (
            <div className="bg-white border rounded-lg overflow-hidden">
              <div className="max-h-[300px] overflow-y-auto">
                <table className="w-full text-left text-sm whitespace-nowrap">
                  <thead className="bg-gray-50 sticky top-0 border-b">
                    <tr>
                      <th className="p-2 font-medium text-gray-500">SKU</th>
                      <th className="p-2 font-medium text-gray-500">Штрихкод</th>
                      <th className="p-2 font-medium text-gray-500 text-right">Цена</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {activeVariants.map((v, i) => (
                      <tr key={i} className="hover:bg-gray-50/50">
                        <td className="p-2 font-medium">{v.sellerSku || <span className="text-gray-400 font-normal italic">Нет SKU</span>}</td>
                        <td className="p-2">{v.barcode ? v.barcode : <span className="text-gray-400 italic">Сгенерирует ZAMK</span>}</td>
                        <td className="p-2 text-right font-medium">{v.priceCents ? `${(v.priceCents / 100).toFixed(2)} ₽` : <span className="text-red-500">Нет цены</span>}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ) : (
            <div className="text-sm text-gray-500 italic">Нет активных вариантов</div>
          )}
        </div>

        <div>
          <h3 className="text-sm text-gray-500 font-medium">Фотографии</h3>
          <p className="mt-1 text-sm">Общих фото: {state.commonImages.length}</p>
          <p className="text-sm">Цветовых фото: {Object.values(state.colorImages).flat().length}</p>
        </div>
      </div>

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} disabled={submitting} className="px-4 py-2 border rounded hover:bg-gray-50 disabled:opacity-50">Назад</button>
        <button
          onClick={onSubmit}
          disabled={submitting}
          className="px-4 py-2 bg-black text-white rounded font-medium hover:bg-zinc-800 disabled:opacity-50"
        >
          {submitting ? 'Отправка...' : 'Отправить на модерацию'}
        </button>
      </div>
    </div>
  );
}

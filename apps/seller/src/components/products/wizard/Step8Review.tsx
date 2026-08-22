import type { WizardState } from './WizardState';

export function Step8Review({ 
  state, 
  onSubmit, 
  onPrev 
}: { 
  state: WizardState; 
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
          <h3 className="text-sm text-gray-500 font-medium">Варианты ({activeVariants.length})</h3>
          <div className="mt-2 space-y-2">
            {activeVariants.map((v, i) => (
              <div key={i} className="flex justify-between items-center bg-white p-3 border rounded text-sm">
                <div>
                  <span className="font-medium text-gray-900">{v.sellerSku || 'Нет SKU'}</span>
                  {v.barcode ? <span className="ml-4 text-gray-500">Штрихкод: {v.barcode}</span> : <span className="ml-4 text-gray-400 italic">Штрихкод ZAMK</span>}
                </div>
                <div className="font-medium">
                  {v.priceCents ? `${(v.priceCents / 100).toFixed(2)} ₽` : 'Нет цены'}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div>
          <h3 className="text-sm text-gray-500 font-medium">Фотографии</h3>
          <p className="mt-1 text-sm">Общих фото: {state.commonImages.length}</p>
          <p className="text-sm">Цветовых фото: {Object.values(state.colorImages).flat().length}</p>
        </div>
      </div>

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onSubmit} className="px-4 py-2 bg-black text-white rounded font-medium">Отправить на модерацию</button>
      </div>
    </div>
  );
}

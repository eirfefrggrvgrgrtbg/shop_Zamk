import type { WizardState } from './WizardState';

export function Step8Review({ 
  state, 
  onPrev,
  onSubmit
}: { 
  state: WizardState; 
  onPrev: () => void; 
  onSubmit: () => void;
}) {
  const activeVariants = state.variants.filter(v => v.active);

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Проверка</h2>
      
      <div className="bg-gray-50 p-4 rounded text-sm space-y-4">
        <div>
          <span className="font-semibold text-gray-700">Название:</span> {state.title || '-'}
        </div>
        <div>
          <span className="font-semibold text-gray-700">Описание:</span> {state.description || '-'}
        </div>
        <div>
          <span className="font-semibold text-gray-700">Активных вариантов:</span> {activeVariants.length}
        </div>
        <div>
          <span className="font-semibold text-gray-700">Заполнено фото:</span> Общих {state.commonImages.length}
        </div>
      </div>

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onSubmit} className="px-4 py-2 bg-black text-white rounded">Отправить на модерацию</button>
      </div>
    </div>
  );
}

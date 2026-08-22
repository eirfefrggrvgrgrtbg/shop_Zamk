import type { WizardState } from './WizardState';

export function Step1Basics({ state, updateState, onNext }: { state: WizardState; updateState: (u: Partial<WizardState>) => void; onNext: () => void }) {
  return (
    <div className="space-y-8 max-w-3xl">
      <div>
        <h2 className="text-2xl font-medium mb-1">Основная информация</h2>
        <p className="text-gray-500 text-sm">Укажите главное о вашем товаре. Название должно быть четким и понятным.</p>
      </div>

      <div className="space-y-6">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">Название товара <span className="text-red-500">*</span></label>
          <input 
            type="text" 
            value={state.title} 
            onChange={e => updateState({ title: e.target.value })} 
            placeholder="Например: Худи оверсайз базовое"
            className="block w-full border border-gray-300 rounded-lg p-3 focus:ring-2 focus:ring-black focus:border-black outline-none transition-shadow text-base placeholder:text-gray-400" 
          />
          <p className="text-xs text-gray-500 mt-2">Без указания бренда и лишних слов.</p>
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">Описание <span className="text-red-500">*</span></label>
          <textarea 
            value={state.description} 
            onChange={e => updateState({ description: e.target.value })} 
            rows={8} 
            placeholder="Расскажите о преимуществах, особенностях кроя и материалах..."
            className="block w-full border border-gray-300 rounded-lg p-3 focus:ring-2 focus:ring-black focus:border-black outline-none transition-shadow text-base placeholder:text-gray-400 resize-y" 
          />
        </div>
      </div>

      <div className="flex justify-end pt-6 border-t">
        <button 
          onClick={onNext} 
          disabled={!state.title.trim() || !state.description.trim()}
          className="px-8 py-2.5 bg-black text-white rounded-lg font-medium hover:bg-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Далее
        </button>
      </div>
    </div>
  );
}

import type { WizardState } from './WizardState';

export function Step1Basics({ state, updateState, onNext }: { state: WizardState; updateState: (u: Partial<WizardState>) => void; onNext: () => void }) {
  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Основное</h2>
      <div>
        <label className="block text-sm">Название *</label>
        <input 
          type="text" 
          value={state.title} 
          onChange={e => updateState({ title: e.target.value })} 
          className="mt-1 block w-full border border-gray-300 rounded-md p-2" 
        />
      </div>
      <div>
        <label className="block text-sm">Описание *</label>
        <textarea 
          value={state.description} 
          onChange={e => updateState({ description: e.target.value })} 
          rows={6} 
          className="mt-1 block w-full border border-gray-300 rounded-md p-2" 
        />
      </div>
      <div className="flex justify-end pt-4">
        <button 
          onClick={onNext} 
          disabled={!state.title || !state.description}
          className="px-4 py-2 bg-black text-white rounded disabled:opacity-50"
        >
          Далее
        </button>
      </div>
    </div>
  );
}

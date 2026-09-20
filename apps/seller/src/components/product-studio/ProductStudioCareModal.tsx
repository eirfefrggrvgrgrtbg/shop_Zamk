import { useState, useEffect } from 'react';
import { X, Sparkles } from 'lucide-react';

export interface ProductStudioCareModalProps {
  isOpen: boolean;
  onClose: () => void;
  careInstructions?: string;
  onSave: (careInstructions: string) => void;
}

export function ProductStudioCareModal({
  isOpen,
  onClose,
  careInstructions = '',
  onSave,
}: ProductStudioCareModalProps) {
  const [pendingCare, setPendingCare] = useState('');

  useEffect(() => {
    if (isOpen) {
      setPendingCare(careInstructions || '');
    }
  }, [isOpen, careInstructions]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleSave = () => {
    onSave(pendingCare.trim());
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="care-modal"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft dark:border-white/10 shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800/40 flex items-center justify-center text-blue-600 dark:text-blue-400 shrink-0">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-graphite dark:text-white">
                Рекомендации по уходу
              </h2>
              <p className="text-xs text-ash mt-0.5">
                Укажите рекомендации по стирке, сушке и глажке (необязательно)
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть"
            className="p-1.5 rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer shrink-0 ml-2"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-5 overflow-y-auto space-y-4 flex-1">
          <div>
            <label className="block text-xs font-medium text-graphite dark:text-white mb-2">
              Инструкции по уходу
            </label>
            <textarea
              rows={4}
              value={pendingCare}
              onChange={(e) => setPendingCare(e.target.value)}
              placeholder="Например: Деликатная стирка при 30°C, отжим до 800 об/мин, сушка в расправленном виде, гладить с изнанки при температуре до 110°C"
              data-testid="care-instructions-input"
              className="w-full px-3.5 py-2.5 text-sm bg-white dark:bg-zinc-900 border border-border-soft dark:border-white/10 rounded-xl text-graphite dark:text-white placeholder:text-ash/50 focus:outline-hidden focus:border-graphite dark:focus:border-white resize-none"
            />
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border-soft dark:border-white/10 bg-[#fafafc] dark:bg-white/[0.01] flex items-center justify-end gap-2.5 shrink-0">
          <button
            type="button"
            data-testid="care-modal-cancel"
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-graphite dark:text-white bg-white dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-xl hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
          >
            Отмена
          </button>
          <button
            type="button"
            data-testid="care-modal-save"
            onClick={handleSave}
            className="px-5 py-2 text-xs font-medium text-white bg-graphite dark:bg-white dark:text-graphite rounded-xl hover:opacity-90 transition-opacity cursor-pointer shadow-xs"
          >
            Сохранить
          </button>
        </div>
      </div>
    </div>
  );
}

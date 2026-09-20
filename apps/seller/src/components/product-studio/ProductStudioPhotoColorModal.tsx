import { useState, useEffect } from 'react';
import { X, Check } from 'lucide-react';
import type { ProductStudioImage } from '../../contexts/ProductStudioContext';

export interface ProductStudioPhotoColorModalProps {
  isOpen: boolean;
  onClose: () => void;
  images: ProductStudioImage[];
  colors: Array<{ id: string; name: string; hex?: string }>;
  onSave: (updatedImages: ProductStudioImage[]) => void;
}

export function ProductStudioPhotoColorModal({
  isOpen,
  onClose,
  images,
  colors,
  onSave,
}: ProductStudioPhotoColorModalProps) {
  // Local pending assignments: map index -> colorId | null
  const [pendingAssignments, setPendingAssignments] = useState<Record<number, string | null>>({});

  useEffect(() => {
    if (isOpen) {
      const initial: Record<number, string | null> = {};
      images.forEach((img, idx) => {
        initial[idx] = img.colorId ?? null;
      });
      setPendingAssignments(initial);
    }
  }, [isOpen, images]);

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

  // Calculate summary counts
  let generalCount = 0;
  const colorCounts: Record<string, number> = {};
  colors.forEach((c) => {
    colorCounts[c.id] = 0;
  });

  images.forEach((_, idx) => {
    const assignedId = pendingAssignments[idx];
    if (!assignedId) {
      generalCount++;
    } else if (colorCounts[assignedId] !== undefined) {
      colorCounts[assignedId]++;
    } else {
      generalCount++;
    }
  });

  const handleApply = () => {
    const updatedImages = images.map((img, idx) => ({
      ...img,
      colorId: pendingAssignments[idx] ?? null,
    }));
    onSave(updatedImages);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="photo-color-binding-modal"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft dark:border-white/10">
          <div>
            <h2 className="text-base font-semibold text-graphite dark:text-white">
              Привязка фотографий к цветам
            </h2>
            <p className="text-xs text-ash mt-1 leading-relaxed">
              Выберите, для какого цвета показывать каждое фото. Фото без привязки используются как общие.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть"
            className="p-1 rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer ml-3 shrink-0"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Summary Badges */}
        <div className="px-5 py-2.5 bg-gray-50/80 dark:bg-white/[0.02] border-b border-border-soft dark:border-white/10 flex items-center gap-3 text-xs text-ash flex-wrap">
          <span className="font-medium text-graphite dark:text-white">
            Общие: <strong className="font-semibold">{generalCount}</strong>
          </span>
          {colors.map((c) => (
            <span key={c.id} className="flex items-center gap-1.5 font-medium text-graphite dark:text-white">
              <span
                className="w-2.5 h-2.5 rounded-full border border-black/10 dark:border-white/20 shrink-0 inline-block"
                style={{ backgroundColor: c.hex || '#000000' }}
              />
              {c.name}: <strong className="font-semibold">{colorCounts[c.id] || 0}</strong>
            </span>
          ))}
        </div>

        {/* Photos List */}
        <div className="p-5 overflow-y-auto space-y-3 flex-1">
          {images.map((img, idx) => {
            const currentVal = pendingAssignments[idx] ?? '';
            return (
              <div
                key={img.id || img.url || idx}
                data-testid={`photo-binding-row-${idx}`}
                className="flex items-center justify-between gap-4 p-2.5 rounded-xl border border-border-soft/60 dark:border-white/5 bg-paper-light/50 dark:bg-white/[0.01]"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <div className="w-12 h-14 rounded-lg overflow-hidden bg-black/5 dark:bg-white/5 shrink-0 border border-border-soft dark:border-white/10 flex items-center justify-center">
                    <img
                      src={img.url}
                      alt=""
                      className="w-full h-full object-cover"
                    />
                  </div>
                  <div className="min-w-0">
                    <span className="text-sm font-medium text-graphite dark:text-white block truncate">
                      Фото {idx + 1}
                    </span>
                    {idx === 0 && (
                      <span className="text-[10px] text-indigo-600 dark:text-indigo-400 font-medium">
                        Обложка товара
                      </span>
                    )}
                  </div>
                </div>

                <div className="shrink-0 w-44">
                  <select
                    data-testid={`photo-color-select-${idx}`}
                    aria-label={`Цвет для фото ${idx + 1}`}
                    value={currentVal}
                    onChange={(e) => {
                      const val = e.target.value ? e.target.value : null;
                      setPendingAssignments((prev) => ({
                        ...prev,
                        [idx]: val,
                      }));
                    }}
                    className="w-full text-xs font-medium bg-white dark:bg-[#202024] border border-border-soft dark:border-white/20 rounded-lg px-2.5 py-2 text-graphite dark:text-white focus:outline-none focus:ring-1 focus:ring-indigo-500 cursor-pointer"
                  >
                    <option value="">Общее (все цвета)</option>
                    {colors.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            );
          })}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border-soft dark:border-white/10 flex items-center justify-end gap-3 bg-gray-50/50 dark:bg-white/[0.02]">
          <button
            type="button"
            data-testid="photo-color-modal-cancel"
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-lg transition-colors cursor-pointer"
          >
            Отмена
          </button>
          <button
            type="button"
            data-testid="photo-color-modal-submit"
            onClick={handleApply}
            className="px-5 py-2 text-xs font-semibold bg-graphite text-white dark:bg-white dark:text-black rounded-lg hover:opacity-90 transition-opacity cursor-pointer flex items-center gap-1.5"
          >
            <Check className="w-3.5 h-3.5" />
            <span>Готово</span>
          </button>
        </div>
      </div>
    </div>
  );
}

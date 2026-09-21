import { useState, useEffect } from 'react';
import { X, Check, HelpCircle } from 'lucide-react';
import type { ProductStudioImage } from '../../contexts/ProductStudioContext';
import { getProductStudioImageDisplayUrl } from './productStudioMediaHelper';
import { cn } from '../../lib/utils';

export function getColorDisplayName(c?: {
  id?: string;
  name?: string;
  nameRu?: string;
  colorName?: string;
  code?: string;
} | null): string {
  if (!c) return 'Цвет без названия';
  const name = (c.nameRu || c.name || c.colorName || '').trim();
  return name || 'Цвет без названия';
}

export interface ProductStudioPhotoColorModalProps {
  isOpen: boolean;
  onClose: () => void;
  images: ProductStudioImage[];
  colors: Array<{
    id: string;
    name?: string;
    nameRu?: string;
    colorName?: string;
    code?: string;
    hex?: string;
  }>;
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
  const [selectedPhotoIndex, setSelectedPhotoIndex] = useState<number>(0);

  useEffect(() => {
    if (isOpen) {
      const initial: Record<number, string | null> = {};
      images.forEach((img, idx) => {
        initial[idx] = img.colorId ?? null;
      });
      setPendingAssignments(initial);
      setSelectedPhotoIndex(0);
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

  const activePhoto = images[selectedPhotoIndex];
  const activeColorId = activePhoto ? (pendingAssignments[selectedPhotoIndex] ?? null) : null;
  const activeColor = activeColorId ? colors.find((c) => c.id === activeColorId) : null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="photo-color-binding-modal"
      onClick={onClose}
    >
      <div
        className="w-full max-w-3xl bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[88vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft dark:border-white/10">
          <div>
            <h2 className="text-base font-semibold text-graphite dark:text-white">
              Привязка фотографий к цветам
            </h2>
            <p className="text-xs text-ash mt-1 leading-relaxed">
              Выберите фото в списке слева и укажите его принадлежность к цвету справа.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть"
            className="p-1.5 rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer ml-3 shrink-0"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Summary Badges Bar */}
        <div
          data-testid="photo-color-summary-bar"
          className="px-5 py-2.5 bg-gray-50/80 dark:bg-white/[0.02] border-b border-border-soft dark:border-white/10 flex items-center gap-2.5 text-xs text-ash flex-wrap"
        >
          <span className="font-medium text-graphite dark:text-white">
            Общие: <strong className="font-semibold">{generalCount}</strong>
          </span>
          {colors.map((c) => {
            const displayName = getColorDisplayName(c);
            return (
              <span key={c.id} className="flex items-center gap-1.5 font-medium text-graphite dark:text-white">
                <span className="text-gray-300 dark:text-gray-600">·</span>
                <span
                  className="w-2.5 h-2.5 rounded-full border border-black/10 dark:border-white/20 shrink-0 inline-block"
                  style={{ backgroundColor: c.hex || '#000000' }}
                />
                <span>{displayName}: </span>
                <strong className="font-semibold">{colorCounts[c.id] || 0}</strong>
              </span>
            );
          })}
        </div>

        {/* Two-Column Workspace */}
        <div className="flex flex-col md:flex-row flex-1 min-h-0 overflow-hidden divide-y md:divide-y-0 md:divide-x divide-border-soft dark:divide-white/10">
          {/* LEFT COLUMN: Photos List */}
          <div className="w-full md:w-7/12 p-4 overflow-y-auto flex flex-col">
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs font-semibold text-graphite dark:text-white">
                Фотографии ({images.length})
              </span>
              <span className="text-[11px] text-ash">
                Нажмите для выбора
              </span>
            </div>

            {images.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-ash text-xs">
                Нет фотографий товара
              </div>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2.5">
                {images.map((img, idx) => {
                  const isSelected = selectedPhotoIndex === idx;
                  const isCover = idx === 0;
                  const assignedId = pendingAssignments[idx];
                  const boundColor = assignedId ? colors.find((c) => c.id === assignedId) : null;
                  const boundColorName = boundColor ? getColorDisplayName(boundColor) : null;

                  return (
                    <button
                      key={img.uiKey || idx}
                      type="button"
                      data-testid={`photo-card-${idx}`}
                      onClick={() => setSelectedPhotoIndex(idx)}
                      className={cn(
                        "group relative rounded-xl border p-1.5 text-left transition-all cursor-pointer flex flex-col",
                        isSelected
                          ? "ring-2 ring-indigo-500 border-indigo-500 bg-indigo-50/20 dark:bg-indigo-950/20"
                          : "border-border-soft dark:border-white/10 hover:border-gray-400 dark:hover:border-white/30 bg-paper-light/30 dark:bg-white/[0.01]"
                      )}
                    >
                      <div className="relative aspect-[4/5] w-full rounded-lg bg-black/5 dark:bg-white/5 overflow-hidden">
                        <img
                          src={getProductStudioImageDisplayUrl(img)}
                          alt={`Фото ${idx + 1}`}
                          className="w-full h-full object-cover"
                        />
                        {isCover && (
                          <span className="absolute top-1 left-1 px-1.5 py-0.5 rounded text-[9px] font-semibold bg-gray-900 text-white dark:bg-white dark:text-gray-900 shadow-xs">
                            Обложка
                          </span>
                        )}
                      </div>

                      <div className="mt-1.5 px-0.5 flex items-center justify-between gap-1 text-[11px]">
                        <span className="font-medium text-graphite dark:text-white truncate">
                          Фото {idx + 1}
                        </span>

                        {boundColorName ? (
                          <span
                            data-testid={`photo-assigned-badge-${idx}`}
                            className="flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-indigo-50 dark:bg-indigo-950/50 text-indigo-700 dark:text-indigo-300 shrink-0"
                            title={`Привязано: ${boundColorName}`}
                          >
                            <span
                              className="w-1.5 h-1.5 rounded-full shrink-0"
                              style={{ backgroundColor: boundColor?.hex || '#000000' }}
                            />
                            <span className="truncate max-w-[55px]">{boundColorName}</span>
                          </span>
                        ) : (
                          <span
                            data-testid={`photo-assigned-badge-${idx}`}
                            className="px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-gray-100 dark:bg-white/10 text-ash shrink-0"
                          >
                            Общее
                          </span>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* RIGHT COLUMN: Color Binding Targets */}
          <div className="w-full md:w-5/12 p-5 overflow-y-auto flex flex-col bg-gray-50/50 dark:bg-white/[0.02]">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-ash mb-3">
              Куда относится фото
            </span>

            {activePhoto ? (
              <div className="flex flex-col flex-1">
                {/* Active Photo Snippet */}
                <div className="p-2.5 rounded-xl border border-border-soft dark:border-white/10 bg-white dark:bg-[#202024] flex items-center gap-3 mb-4">
                  <div className="w-10 h-12 rounded-lg overflow-hidden bg-black/5 dark:bg-white/5 shrink-0 border border-border-soft dark:border-white/10">
                    <img
                      src={getProductStudioImageDisplayUrl(activePhoto)}
                      alt=""
                      className="w-full h-full object-cover"
                    />
                  </div>
                  <div className="min-w-0">
                    <div className="text-xs font-semibold text-graphite dark:text-white truncate">
                      Фото {selectedPhotoIndex + 1}
                      {selectedPhotoIndex === 0 && ' (Обложка)'}
                    </div>
                    <div className="text-[11px] text-ash mt-0.5 truncate">
                      {activeColor ? `Привязано к: ${getColorDisplayName(activeColor)}` : 'Общее фото'}
                    </div>
                  </div>
                </div>

                {/* Target Options */}
                <span className="text-xs font-medium text-graphite dark:text-white mb-2">
                  Выберите вариант привязки:
                </span>

                <div className="space-y-2 flex-1">
                  {/* Option: General */}
                  <button
                    type="button"
                    data-testid="photo-bind-target-general"
                    onClick={() => {
                      setPendingAssignments((prev) => ({
                        ...prev,
                        [selectedPhotoIndex]: null,
                      }));
                    }}
                    className={cn(
                      "w-full flex items-center justify-between p-3 rounded-xl border text-left transition-all cursor-pointer",
                      activeColorId === null
                        ? "border-indigo-500 bg-indigo-50/60 dark:bg-indigo-950/40 text-indigo-950 dark:text-indigo-100 ring-1 ring-indigo-500 shadow-xs"
                        : "border-border-soft dark:border-white/10 hover:border-gray-300 dark:hover:border-white/20 bg-white dark:bg-[#202024] text-graphite dark:text-white"
                    )}
                  >
                    <div>
                      <div className="text-xs font-semibold">Общее (все цвета)</div>
                      <div className="text-[11px] text-ash mt-0.5">
                        Показывается для всех вариантов
                      </div>
                    </div>

                    <div className="flex items-center gap-2 shrink-0 ml-2">
                      <span className="text-[11px] text-ash font-medium">
                        {generalCount} фото
                      </span>
                      {activeColorId === null && (
                        <span className="w-5 h-5 rounded-full bg-indigo-600 text-white flex items-center justify-center shrink-0">
                          <Check className="w-3 h-3 stroke-[2.5]" />
                        </span>
                      )}
                    </div>
                  </button>

                  {/* Options: Colors */}
                  {colors.map((c) => {
                    const isSelectedColor = activeColorId === c.id;
                    const colorName = getColorDisplayName(c);
                    const count = colorCounts[c.id] || 0;

                    return (
                      <button
                        key={c.id}
                        type="button"
                        data-testid={`photo-bind-target-${c.id}`}
                        onClick={() => {
                          setPendingAssignments((prev) => ({
                            ...prev,
                            [selectedPhotoIndex]: c.id,
                          }));
                        }}
                        className={cn(
                          "w-full flex items-center justify-between p-3 rounded-xl border text-left transition-all cursor-pointer",
                          isSelectedColor
                            ? "border-indigo-500 bg-indigo-50/60 dark:bg-indigo-950/40 text-indigo-950 dark:text-indigo-100 ring-1 ring-indigo-500 shadow-xs"
                            : "border-border-soft dark:border-white/10 hover:border-gray-300 dark:hover:border-white/20 bg-white dark:bg-[#202024] text-graphite dark:text-white"
                        )}
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <span
                            className="w-3.5 h-3.5 rounded-full border border-black/10 dark:border-white/20 shrink-0 inline-block"
                            style={{ backgroundColor: c.hex || '#000000' }}
                          />
                          <span className="text-xs font-semibold truncate">
                            {colorName}
                          </span>
                        </div>

                        <div className="flex items-center gap-2 shrink-0 ml-2">
                          <span className="text-[11px] text-ash font-medium">
                            {count} фото
                          </span>
                          {isSelectedColor && (
                            <span className="w-5 h-5 rounded-full bg-indigo-600 text-white flex items-center justify-center shrink-0">
                              <Check className="w-3 h-3 stroke-[2.5]" />
                            </span>
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>

                {/* Helper text */}
                <div className="mt-4 p-3 rounded-xl bg-gray-100/70 dark:bg-white/5 text-[11px] text-ash leading-relaxed flex items-start gap-2">
                  <HelpCircle className="w-4 h-4 text-ash shrink-0 mt-0.5" />
                  <span>
                    Если фото подходит для всех вариантов (например, общая посадка или детали кроя), оставьте его в разделе «Общее».
                  </span>
                </div>
              </div>
            ) : (
              <div className="flex-1 flex items-center justify-center text-xs text-ash">
                Выберите фотографию для настройки привязки
              </div>
            )}
          </div>
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

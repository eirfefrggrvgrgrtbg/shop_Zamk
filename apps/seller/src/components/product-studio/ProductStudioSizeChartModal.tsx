import { useState, useEffect } from 'react';
import { X, Ruler, CheckCircle2, AlertCircle } from 'lucide-react';
import type { SellerCategorySchema } from '@zamk/api-client/src/seller';
import type { ProductPresentationSizeChart } from '@zamk/shared/src/catalog/product-presentation/types';

export interface ProductStudioSizeChartModalProps {
  isOpen: boolean;
  onClose: () => void;
  schema: SellerCategorySchema | null;
  categoryName?: string;
  draftSizes?: Array<{ id: string; label: string }>;
  sizeChart?: ProductPresentationSizeChart;
  onSaveSizeChart: (sizeChart: ProductPresentationSizeChart) => void;
}

export function ProductStudioSizeChartModal({
  isOpen,
  onClose,
  schema,
  categoryName,
  draftSizes = [],
  sizeChart,
  onSaveSizeChart,
}: ProductStudioSizeChartModalProps) {
  // Local pending measurements state: { [sizeValueId]: { [fieldCode]: string | number } }
  const [pendingMeasurements, setPendingMeasurements] = useState<
    Record<string, Record<string, string | number>>
  >({});

  const fields = [...(schema?.sizeChartFields || [])].sort(
    (a, b) => a.sortOrder - b.sortOrder
  );
  const requiredFields = fields.filter((f) => f.isRequired);

  // Sync / hydrate pending state from existing sizeChart when opening
  useEffect(() => {
    if (isOpen) {
      const initial: Record<string, Record<string, string | number>> = {};
      for (const s of draftSizes) {
        const existingRow = sizeChart?.rows?.find(
          (r: any) =>
            (r.sizeValueId && r.sizeValueId === s.id) ||
            (r.sizeValueName && r.sizeValueName === s.label) ||
            (r.size && (r.size === s.label || r.size === s.id))
        );
        initial[s.id] = existingRow?.measurements ? { ...existingRow.measurements } : {};
      }
      setPendingMeasurements(initial);
    }
  }, [isOpen, sizeChart, draftSizes]);

  // Escape key handler
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

  const handleCellChange = (sizeId: string, fieldCode: string, rawVal: string) => {
    setPendingMeasurements((prev) => {
      const rowData = { ...(prev[sizeId] || {}) };
      if (rawVal === '') {
        delete rowData[fieldCode];
      } else {
        rowData[fieldCode] = rawVal;
      }
      return {
        ...prev,
        [sizeId]: rowData,
      };
    });
  };

  const isCellValid = (val: string | number | undefined): boolean => {
    if (val === undefined || val === null || val === '') return false;
    const num = typeof val === 'number' ? val : parseFloat(String(val).replace(',', '.'));
    return !isNaN(num) && num > 0;
  };

  // Compute live progress in pending modal
  let filledRequiredCount = 0;
  const totalRequiredCount = draftSizes.length * requiredFields.length;
  for (const s of draftSizes) {
    const row = pendingMeasurements[s.id] || {};
    for (const f of requiredFields) {
      if (isCellValid(row[f.code])) {
        filledRequiredCount++;
      }
    }
  }

  const isComplete = totalRequiredCount > 0 && filledRequiredCount === totalRequiredCount;

  const handleSave = () => {
    const newSizeChart: ProductPresentationSizeChart = {
      fields: fields.map((f) => ({
        code: f.code,
        name: f.name,
        unit: f.unit,
        isRequired: f.isRequired,
        sortOrder: f.sortOrder,
      })),
      rows: draftSizes.map((s) => {
        const rowMeasurements: Record<string, number> = {};
        const rawRow = pendingMeasurements[s.id] || {};
        for (const f of fields) {
          const raw = rawRow[f.code];
          if (raw !== undefined && raw !== null && raw !== '') {
            const parsed = typeof raw === 'number' ? raw : parseFloat(String(raw).replace(',', '.'));
            if (!isNaN(parsed) && parsed > 0) {
              rowMeasurements[f.code] = parsed;
            }
          }
        }
        return {
          sizeValueId: s.id,
          sizeValueName: s.label,
          measurements: rowMeasurements,
        };
      }),
    };

    onSaveSizeChart(newSizeChart);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="size-chart-modal"
      onClick={onClose}
    >
      <div
        className="w-full max-w-3xl bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft dark:border-white/10 shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-amber-50 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800/40 flex items-center justify-center text-amber-600 dark:text-amber-400 shrink-0">
              <Ruler className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-graphite dark:text-white">
                Таблица размеров и мерки
              </h2>
              <p className="text-xs text-ash mt-0.5">
                {(categoryName || schema?.name)
                  ? `Категория: ${categoryName || schema?.name}. Заполните мерки готового изделия в сантиметрах.`
                  : 'Параметры изделия в сантиметрах.'}
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

        {/* Progress Bar / Summary */}
        <div className="px-6 py-3 bg-[#fafafc] dark:bg-white/[0.02] border-b border-border-soft dark:border-white/10 flex items-center justify-between flex-wrap gap-2 shrink-0">
          <div className="flex items-center gap-2">
            {isComplete ? (
              <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="w-4 h-4" />
                Все обязательные мерки заполнены ({filledRequiredCount} из {totalRequiredCount})
              </span>
            ) : totalRequiredCount > 0 ? (
              <span className="inline-flex items-center gap-1.5 text-xs font-medium text-amber-600 dark:text-amber-400">
                <AlertCircle className="w-4 h-4" />
                Заполнено {filledRequiredCount} из {totalRequiredCount} обязательных мерок
              </span>
            ) : (
              <span className="text-xs text-ash">
                Все мерки опциональны
              </span>
            )}
          </div>
          <span className="text-[11px] text-ash">
            * Обязательные для заполнения поля
          </span>
        </div>

        {/* Body / Content */}
        <div className="p-6 overflow-y-auto flex-1">
          {draftSizes.length === 0 ? (
            <div className="p-8 text-center text-ash text-sm space-y-2">
              <p className="font-medium text-graphite dark:text-white">
                Размеры не выбраны
              </p>
              <p className="text-xs">
                Сначала выберите размеры товара в блоке «Размеры», чтобы настроить для них мерки.
              </p>
            </div>
          ) : fields.length === 0 ? (
            <div className="p-8 text-center text-ash text-sm">
              Для данной категории нет канонических параметров таблицы размеров.
            </div>
          ) : (
            <div className="border border-border-soft dark:border-white/10 rounded-xl overflow-hidden overflow-x-auto shadow-xs">
              <table className="w-full text-sm border-collapse">
                <thead className="bg-[#f5f5f7] dark:bg-white/5 border-b border-border-soft dark:border-white/10">
                  <tr>
                    <th className="py-3 px-4 text-left font-semibold text-graphite dark:text-white whitespace-nowrap min-w-[100px]">
                      Размер
                    </th>
                    {fields.map((field) => (
                      <th
                        key={field.code}
                        className="py-3 px-3 text-left font-semibold text-graphite dark:text-white whitespace-nowrap min-w-[130px]"
                      >
                        <span>{field.name}</span>
                        {field.unit && (
                          <span className="text-xs font-normal text-ash ml-1">
                            ({field.unit})
                          </span>
                        )}
                        {field.isRequired && (
                          <span className="text-red-500 font-bold ml-1">*</span>
                        )}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-soft dark:divide-white/5 bg-white dark:bg-[#1a1a1c]">
                  {draftSizes.map((s) => {
                    const rowData = pendingMeasurements[s.id] || {};
                    return (
                      <tr
                        key={s.id}
                        className="hover:bg-black/[0.01] dark:hover:bg-white/[0.01] transition-colors"
                      >
                        <td className="py-3 px-4 font-semibold text-graphite dark:text-white whitespace-nowrap">
                          {s.label}
                        </td>
                        {fields.map((field) => {
                          const val = rowData[field.code] ?? '';
                          const isFieldReq = field.isRequired;
                          const cellValid = isCellValid(val);
                          const isMissing = isFieldReq && !cellValid;

                          return (
                            <td key={field.code} className="py-2.5 px-3">
                              <input
                                type="number"
                                step="any"
                                min="0"
                                data-testid={`measurement-input-${s.id}-${field.code}`}
                                placeholder={isFieldReq ? 'Обязательно' : 'Опционально'}
                                value={val}
                                onChange={(e) =>
                                  handleCellChange(s.id, field.code, e.target.value)
                                }
                                className={`w-full max-w-[140px] px-3 py-1.5 text-sm font-mono text-center rounded-lg border transition-colors focus:outline-hidden ${
                                  isMissing
                                    ? 'border-red-300 dark:border-red-700/80 bg-red-50/30 dark:bg-red-950/20 text-red-900 dark:text-red-200 placeholder-red-400 focus:border-red-500'
                                    : 'border-border-soft dark:border-white/10 bg-white dark:bg-zinc-900 text-graphite dark:text-white placeholder-ash/50 focus:border-graphite dark:focus:border-white'
                                }`}
                              />
                            </td>
                          );
                        })}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-5 border-t border-border-soft dark:border-white/10 flex items-center justify-between gap-3 bg-[#fafafc] dark:bg-white/[0.01] shrink-0">
          <p className="text-xs text-ash">
            {totalRequiredCount > 0 && !isComplete
              ? 'Заполните обязательные поля для всех выбранных размеров'
              : 'Мерки сохраняются в карточке изделия'}
          </p>
          <div className="flex items-center gap-2.5">
            <button
              type="button"
              data-testid="size-chart-modal-cancel"
              onClick={onClose}
              className="px-4 py-2 text-xs font-medium text-graphite dark:text-white bg-white dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-xl hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            >
              Отмена
            </button>
            <button
              type="button"
              data-testid="size-chart-modal-save"
              onClick={handleSave}
              className="px-5 py-2 text-xs font-semibold text-white bg-graphite dark:bg-white dark:text-graphite rounded-xl hover:opacity-90 transition-opacity cursor-pointer shadow-xs"
            >
              Сохранить
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

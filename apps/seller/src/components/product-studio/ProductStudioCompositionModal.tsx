import React, { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { X, Plus, Trash2, Search, ChevronDown, Check } from 'lucide-react';
import { getSellerMaterials, type SellerMaterial } from '@zamk/api-client/src/seller';

export interface ProductStudioCompositionRow {
  materialId?: string;
  materialName?: string;
  percentage?: number;
}

export interface ProductStudioCompositionModalProps {
  isOpen: boolean;
  onClose: () => void;
  materialComposition?: ProductStudioCompositionRow[];
  onSave: (data: {
    materialComposition: ProductStudioCompositionRow[];
    material?: string;
    careInstructions?: string;
  }) => void;
}

interface InternalRow {
  materialId: string;
  materialName: string;
  percentage: string;
}

const DEFAULT_MATERIAL_COMPOSITION: ProductStudioCompositionRow[] = [];

export function ProductStudioCompositionModal({
  isOpen,
  onClose,
  materialComposition = DEFAULT_MATERIAL_COMPOSITION,
  onSave,
}: ProductStudioCompositionModalProps) {
  const [availableMaterials, setAvailableMaterials] = useState<SellerMaterial[]>([]);
  const [pendingRows, setPendingRows] = useState<InternalRow[]>([]);
  const [openRowIndex, setOpenRowIndex] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [popoverCoords, setPopoverCoords] = useState<{ top: number; left: number; width: number }>({
    top: 100,
    left: 50,
    width: 368,
  });
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const triggerRefs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    if (!isOpen) return;
    getSellerMaterials()
      .then((mats) => setAvailableMaterials(mats || []))
      .catch(() => setAvailableMaterials([]));
  }, [isOpen]);

  useEffect(() => {
    if (isOpen) {
      if (materialComposition && materialComposition.length > 0) {
        setPendingRows(
          materialComposition.map((r) => ({
            materialId: r.materialId || '',
            materialName: r.materialName || '',
            percentage: r.percentage !== undefined && r.percentage !== null ? String(r.percentage) : '',
          }))
        );
      } else {
        // First row must exist by default in empty modal
        setPendingRows([{ materialId: '', materialName: '', percentage: '' }]);
      }
      setOpenRowIndex(null);
      setSearchQuery('');
    }
  }, [isOpen, materialComposition]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        if (openRowIndex !== null) {
          setOpenRowIndex(null);
          setSearchQuery('');
        } else {
          onClose();
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, openRowIndex, onClose]);

  // Floating popover repositioning on resize / scroll
  useEffect(() => {
    if (openRowIndex === null) return;

    const updateCoords = () => {
      const el = triggerRefs.current[openRowIndex];
      if (el) {
        const rect = el.getBoundingClientRect();
        setPopoverCoords({
          top: rect.bottom > 0 ? rect.bottom + 4 : 100,
          left: rect.left > 0 ? rect.left : 50,
          width: rect.width > 0 ? rect.width : 368,
        });
      }
    };

    window.addEventListener('resize', updateCoords);
    window.addEventListener('scroll', updateCoords, true);
    return () => {
      window.removeEventListener('resize', updateCoords);
      window.removeEventListener('scroll', updateCoords, true);
    };
  }, [openRowIndex]);

  // Handle click outside floating popover
  useEffect(() => {
    if (openRowIndex === null) return;

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node;
      const currentTrigger = triggerRefs.current[openRowIndex];
      if (
        popoverRef.current &&
        !popoverRef.current.contains(target) &&
        (!currentTrigger || !currentTrigger.contains(target))
      ) {
        setOpenRowIndex(null);
        setSearchQuery('');
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [openRowIndex]);

  if (!isOpen) return null;

  const totalPercentage = pendingRows.reduce((acc, r) => {
    const trimmed = String(r.percentage).trim();
    const n = parseInt(trimmed, 10);
    return acc + (!isNaN(n) && n > 0 ? n : 0);
  }, 0);

  const selectedMaterialIds = new Set(
    pendingRows.map((r) => r.materialId).filter(Boolean)
  );
  const allMaterialsUsed =
    availableMaterials.length > 0 &&
    selectedMaterialIds.size >= availableMaterials.length;

  const canAddMaterial = totalPercentage < 100 && !allMaterialsUsed;

  let addMaterialTitle: string | undefined = undefined;
  if (!canAddMaterial) {
    if (totalPercentage >= 100) {
      addMaterialTitle = 'Состав уже составляет 100%';
    } else if (allMaterialsUsed) {
      addMaterialTitle = 'Все доступные материалы уже добавлены';
    }
  }

  const isRowValid = (row: InternalRow): boolean => {
    if (!row.materialId) return false;
    const trimmed = String(row.percentage).trim();
    if (trimmed === '') return false;
    const n = parseInt(trimmed, 10);
    return !isNaN(n) && n > 0 && n <= 100 && String(n) === trimmed;
  };

  const isRowInvalid = (row: InternalRow): boolean => {
    const trimmed = String(row.percentage).trim();
    if (trimmed === '') return false;
    const n = parseInt(trimmed, 10);
    return isNaN(n) || n <= 0 || n > 100 || String(n) !== trimmed;
  };

  const handleAddRow = () => {
    if (!canAddMaterial) return;
    setPendingRows((prev) => [
      ...prev,
      { materialId: '', materialName: '', percentage: '' },
    ]);
    setOpenRowIndex(null);
    setSearchQuery('');
  };

  const handleToggleCombobox = (idx: number) => {
    if (openRowIndex === idx) {
      setOpenRowIndex(null);
      setSearchQuery('');
    } else {
      const el = triggerRefs.current[idx];
      if (el) {
        const rect = el.getBoundingClientRect();
        setPopoverCoords({
          top: rect.bottom > 0 ? rect.bottom + 4 : 100,
          left: rect.left > 0 ? rect.left : 50,
          width: rect.width > 0 ? rect.width : 368,
        });
      }
      setOpenRowIndex(idx);
      setSearchQuery('');
    }
  };

  const handleMaterialSelect = (index: number, selectedId: string, selectedName: string) => {
    setPendingRows((prev) => {
      const updated = [...prev];
      updated[index] = {
        ...updated[index],
        materialId: selectedId,
        materialName: selectedName,
      };
      return updated;
    });
    setOpenRowIndex(null);
    setSearchQuery('');
  };

  const handlePercentageChange = (index: number, rawVal: string) => {
    const cleaned = rawVal.replace(/[^\d]/g, '');
    setPendingRows((prev) => {
      const updated = [...prev];
      updated[index] = {
        ...updated[index],
        percentage: cleaned,
      };
      return updated;
    });
  };

  const handlePercentageKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      // If pressing Enter on the last row and sum < 100, add next row
      if (index === pendingRows.length - 1 && canAddMaterial) {
        handleAddRow();
      }
    }
  };

  const handleRemoveRow = (index: number) => {
    setPendingRows((prev) => {
      if (prev.length <= 1) {
        // Keep single row ready for entry
        return [{ materialId: '', materialName: '', percentage: '' }];
      }
      return prev.filter((_, idx) => idx !== index);
    });
    if (openRowIndex === index) {
      setOpenRowIndex(null);
      setSearchQuery('');
    } else if (openRowIndex !== null && openRowIndex > index) {
      setOpenRowIndex(openRowIndex - 1);
    }
  };

  const allRowsFilledAndValid =
    pendingRows.length > 0 &&
    pendingRows.every((r) => isRowValid(r));

  const hasAnyInvalidRow = pendingRows.some((r) => isRowInvalid(r));

  const isCompositionValid = allRowsFilledAndValid && totalPercentage === 100 && !hasAnyInvalidRow;

  const handleApply = () => {
    if (!isCompositionValid) return;

    const validatedRows: ProductStudioCompositionRow[] = pendingRows.map((r) => {
      const mat = availableMaterials.find((m) => m.id === r.materialId);
      return {
        materialId: r.materialId,
        materialName: mat ? mat.nameRu : r.materialName,
        percentage: parseInt(String(r.percentage).trim(), 10),
      };
    });

    const formattedMaterial = validatedRows
      .map((r) => `${r.materialName} — ${r.percentage}%`)
      .join(', ');

    onSave({
      materialComposition: validatedRows,
      material: formattedMaterial,
    });
    onClose();
  };

  // Helper text for sum
  let sumMessage = '';
  let sumColorClass = 'text-ash';
  if (pendingRows.length > 0) {
    if (hasAnyInvalidRow) {
      sumMessage = 'Исправьте некорректные доли материалов';
      sumColorClass = 'text-red-500 dark:text-red-400 font-medium';
    } else if (totalPercentage === 100) {
      if (allRowsFilledAndValid) {
        sumMessage = 'Состав заполнен';
        sumColorClass = 'text-emerald-600 dark:text-emerald-400 font-medium';
      } else {
        sumMessage = 'Заполните все строки состава';
        sumColorClass = 'text-red-500 dark:text-red-400 font-medium';
      }
    } else if (totalPercentage < 100) {
      const remaining = 100 - totalPercentage;
      sumMessage = `Нужно добавить ещё ${remaining}%`;
      sumColorClass = 'text-ash';
    } else {
      const overflow = totalPercentage - 100;
      sumMessage = `Сумма превышает 100% на ${overflow}%`;
      sumColorClass = 'text-red-500 dark:text-red-400 font-medium';
    }
  }

  const filteredMaterials = availableMaterials.filter((m) => {
    if (!searchQuery.trim()) return true;
    return m.nameRu.toLowerCase().includes(searchQuery.trim().toLowerCase());
  });

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="composition-modal-backdrop"
      onClick={onClose}
    >
      <div
        className="w-full max-w-[560px] bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col"
        data-testid="composition-modal"
        style={{ width: 560, maxWidth: 'calc(100vw - 32px)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft/60 dark:border-white/10 shrink-0">
          <div>
            <h2 className="text-base font-semibold text-graphite dark:text-white">
              Состав
            </h2>
            <p className="text-xs text-ash mt-0.5">
              Укажите материалы и их доли
            </p>
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
        <div className="p-5 space-y-3.5 flex-1">
          {/* Material Rows */}
          <div className="space-y-2">
            {pendingRows.map((row, idx) => {
              const isInvalid = isRowInvalid(row);
              const isOpenCombobox = openRowIndex === idx;

              return (
                <div
                  key={idx}
                  data-testid={`composition-row-${idx}`}
                  className="grid gap-2 items-center"
                  style={{
                    gridTemplateColumns: 'minmax(0, 1fr) 96px 32px',
                    height: 42,
                  }}
                >
                  {/* Material Trigger */}
                  <button
                    type="button"
                    ref={(el) => {
                      triggerRefs.current[idx] = el;
                    }}
                    role="combobox"
                    aria-expanded={isOpenCombobox}
                    aria-haspopup="listbox"
                    onClick={() => handleToggleCombobox(idx)}
                    data-testid={`composition-material-trigger-${idx}`}
                    data-material-select-id={`composition-material-select-${idx}`}
                    className={`w-full h-10 px-3 flex items-center justify-between text-sm bg-white dark:bg-zinc-900 border rounded-xl transition-colors cursor-pointer text-left ${
                      isOpenCombobox
                        ? 'border-graphite dark:border-white ring-1 ring-graphite/20 dark:ring-white/20'
                        : 'border-border-soft dark:border-white/10 hover:border-border-medium'
                    }`}
                  >
                    <span className={row.materialName ? 'text-graphite dark:text-white font-medium truncate' : 'text-ash truncate'}>
                      {row.materialName || 'Выберите материал'}
                    </span>
                    <ChevronDown
                      className={`w-4 h-4 text-ash transition-transform shrink-0 ml-2 ${
                        isOpenCombobox ? 'rotate-180' : ''
                      }`}
                    />
                  </button>

                  {/* Percentage Input */}
                  <div className="relative w-full h-10 flex items-center">
                    <input
                      type="text"
                      inputMode="numeric"
                      value={row.percentage}
                      onChange={(e) => handlePercentageChange(idx, e.target.value)}
                      onKeyDown={(e) => handlePercentageKeyDown(idx, e)}
                      placeholder="0"
                      data-testid={`composition-percentage-input-${idx}`}
                      className={`w-full h-10 pl-2.5 pr-6 text-sm font-mono text-right bg-white dark:bg-zinc-900 border rounded-xl outline-none text-graphite dark:text-white transition-colors ${
                        isInvalid
                          ? 'border-red-500 bg-red-50/20 text-red-700 dark:text-red-400 focus:border-red-500'
                          : 'border-border-soft dark:border-white/10 focus:border-graphite dark:focus:border-white'
                      }`}
                    />
                    <span className="absolute right-2 text-xs text-ash pointer-events-none">
                      %
                    </span>
                  </div>

                  {/* Remove Button */}
                  <button
                    type="button"
                    onClick={() => handleRemoveRow(idx)}
                    data-testid={`composition-remove-row-${idx}`}
                    aria-label="Удалить материал"
                    className="w-8 h-10 flex items-center justify-center text-ash hover:text-red-500 hover:bg-red-50/50 dark:hover:bg-red-950/20 rounded-lg transition-colors cursor-pointer"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              );
            })}
          </div>

          {/* Add Material Action */}
          <div>
            <button
              type="button"
              onClick={handleAddRow}
              disabled={!canAddMaterial}
              title={addMaterialTitle}
              data-testid="add-material-row-button"
              className={`inline-flex items-center gap-1.5 py-1.5 px-2.5 text-xs font-medium rounded-lg transition-colors ${
                canAddMaterial
                  ? 'text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 cursor-pointer'
                  : 'text-ash/40 dark:text-white/20 cursor-not-allowed'
              }`}
            >
              <Plus className="w-3.5 h-3.5" />
              Добавить материал
            </button>
          </div>

          {/* Sum Area */}
          <div className="pt-3 border-t border-border-soft/60 dark:border-white/10 flex items-center justify-between">
            <div>
              <span className="text-xs text-ash block">Сумма</span>
              <span
                data-testid="composition-sum-status"
                className={`text-xs ${sumColorClass}`}
              >
                {sumMessage}
              </span>
            </div>
            <div className="text-right">
              <span
                data-testid="composition-sum-value"
                className={`text-base font-semibold font-mono ${
                  totalPercentage === 100 && !hasAnyInvalidRow
                    ? 'text-emerald-600 dark:text-emerald-400'
                    : totalPercentage > 100 || hasAnyInvalidRow
                    ? 'text-red-500 dark:text-red-400'
                    : 'text-graphite dark:text-white'
                }`}
              >
                {totalPercentage}%
              </span>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border-soft/60 dark:border-white/10 flex items-center justify-end gap-2.5 shrink-0">
          <button
            type="button"
            data-testid="composition-modal-cancel"
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 rounded-xl transition-colors cursor-pointer"
          >
            Отмена
          </button>
          <button
            type="button"
            data-testid="composition-modal-apply"
            onClick={handleApply}
            disabled={!isCompositionValid}
            className={`px-5 py-2 text-xs font-medium rounded-xl transition-all ${
              isCompositionValid
                ? 'text-white bg-graphite dark:bg-white dark:text-graphite hover:opacity-90 cursor-pointer shadow-xs'
                : 'text-ash bg-black/5 dark:bg-white/5 cursor-not-allowed opacity-60'
            }`}
          >
            Применить
          </button>
        </div>
      </div>

      {/* Floating Material Picker Overlay */}
      {openRowIndex !== null && typeof document !== 'undefined' && createPortal(
        <div
          ref={popoverRef}
          data-testid={`composition-material-popover-${openRowIndex}`}
          style={{
            position: 'fixed',
            top: popoverCoords.top,
            left: popoverCoords.left,
            width: popoverCoords.width,
            maxHeight: 260,
            zIndex: 9999,
          }}
          className="bg-white dark:bg-[#1c1c1e] border border-border-soft dark:border-white/15 rounded-xl shadow-xl overflow-hidden flex flex-col"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Integrated Search */}
          <div className="relative p-2 border-b border-border-soft dark:border-white/10 shrink-0">
            <Search className="w-3.5 h-3.5 absolute left-4 top-1/2 -translate-y-1/2 text-ash pointer-events-none" />
            <input
              type="text"
              autoFocus
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Поиск материала..."
              data-testid="material-search-input"
              className="w-full pl-7 pr-2.5 py-1.5 text-xs bg-paper-light dark:bg-white/5 border border-border-soft/60 dark:border-white/10 rounded-lg text-graphite dark:text-white placeholder:text-ash outline-none focus:border-graphite dark:focus:border-white transition-colors"
            />
          </div>

          {/* Material Options List */}
          <div className="overflow-y-auto flex-1 p-1 space-y-0.5" role="listbox">
            {filteredMaterials.map((mat) => {
              const isChosenElsewhere = pendingRows.some(
                (other, oIdx) => oIdx !== openRowIndex && other.materialId === mat.id
              );
              const isSelected = pendingRows[openRowIndex]?.materialId === mat.id;

              return (
                <button
                  key={mat.id}
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  disabled={isChosenElsewhere}
                  data-testid={`material-option-${mat.id}`}
                  onClick={() => handleMaterialSelect(openRowIndex, mat.id, mat.nameRu)}
                  className={`w-full h-9 px-2.5 text-xs rounded-lg flex items-center justify-between transition-colors text-left ${
                    isChosenElsewhere
                      ? 'text-ash/40 dark:text-white/20 cursor-not-allowed bg-transparent'
                      : isSelected
                      ? 'text-graphite dark:text-white bg-black/5 dark:bg-white/10 font-medium cursor-pointer'
                      : 'text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 cursor-pointer'
                  }`}
                >
                  <span className="truncate">{mat.nameRu}</span>
                  {isChosenElsewhere && (
                    <span className="text-[11px] text-ash/50 dark:text-white/30 shrink-0 ml-2">
                      Уже добавлен
                    </span>
                  )}
                  {isSelected && !isChosenElsewhere && (
                    <Check className="w-3.5 h-3.5 text-graphite dark:text-white shrink-0 ml-2" />
                  )}
                </button>
              );
            })}
            {filteredMaterials.length === 0 && (
              <div className="text-xs text-ash text-center py-4">
                Материал не найден
              </div>
            )}
          </div>
        </div>,
        document.body
      )}
    </div>
  );
}

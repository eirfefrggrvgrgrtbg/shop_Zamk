import React, { useState, useEffect, useMemo } from 'react';
import { flushSync } from 'react-dom';
import { useParams, Link } from 'react-router-dom';
import { Printer, ArrowLeft, Loader2, AlertCircle, Info, X, ChevronLeft, ChevronRight, CheckSquare } from 'lucide-react';
import { getSellerSupplyUnitLabels, SellerSupplyUnitLabelsResponse, SellerSupplyUnitLabel } from '@zamk/api-client';
import { Code128Barcode } from '../components/Code128Barcode';

function getLabelsWord(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'этикеток';
  if (mod10 === 1) return 'этикетка';
  if (mod10 >= 2 && mod10 <= 4) return 'этикетки';
  return 'этикеток';
}

interface GroupedVariant {
  variantId: string;
  colorName?: string;
  sizeName?: string;
  sellerSku?: string;
  variantBarcode?: string;
  variantDetails: string;
  units: SellerSupplyUnitLabel[];
}

interface GroupedProduct {
  productTitle: string;
  variants: GroupedVariant[];
  totalUnits: number;
}

interface PrintTargetUnit {
  unit: SellerSupplyUnitLabel;
  unitIndex: number;
  variantTotal: number;
}

interface UnitLabelCardProps {
  unit: SellerSupplyUnitLabel;
  supplyNumber: string;
  unitIndex: number;
  variantTotal: number;
  onClick?: () => void;
  isInteractive?: boolean;
  size?: 'normal' | 'large';
  isSelectionMode?: boolean;
  isSelected?: boolean;
  onToggleSelect?: () => void;
  isViewed?: boolean;
  isPrintRequested?: boolean;
}

const UnitLabelCard: React.FC<UnitLabelCardProps> = ({
  unit,
  supplyNumber,
  unitIndex,
  variantTotal,
  onClick,
  isInteractive = false,
  size = 'normal',
  isSelectionMode = false,
  isSelected = false,
  onToggleSelect,
  isViewed = false,
  isPrintRequested = false,
}) => {
  const variantDetails = [unit.colorName, unit.sizeName].filter(Boolean).join(' · ');

  if (size === 'large') {
    return (
      <div
        className="w-full max-w-sm bg-white border-2 border-slate-900 rounded-2xl p-5 shadow-lg flex flex-col justify-between aspect-[58/40]"
        style={{ minHeight: '220px' }}
      >
        {/* Top Header */}
        <div>
          <div className="flex items-center justify-between border-b-2 border-black pb-1 mb-2">
            <span className="text-base font-black tracking-tight text-black">ZAMK</span>
            <span className="text-xs font-mono font-semibold text-gray-600">
              {supplyNumber} · {unitIndex + 1} из {variantTotal}
            </span>
          </div>

          {/* Product Title */}
          <h4 className="text-sm font-bold text-black line-clamp-1 leading-snug" title={unit.productTitle}>
            {unit.productTitle}
          </h4>

          {/* Color / Size & SKU */}
          <div className="flex items-baseline justify-between text-xs text-gray-800 mt-1">
            {variantDetails && <span className="font-bold truncate max-w-[60%]">{variantDetails}</span>}
            {unit.sellerSku && <span className="font-mono text-gray-600 truncate">Арт: {unit.sellerSku}</span>}
          </div>

          {/* Secondary Variant Barcode (ZMK) */}
          {unit.variantBarcode && (
            <div className="text-[11px] font-mono text-gray-500 mt-0.5">
              ZMK: {unit.variantBarcode}
            </div>
          )}
        </div>

        {/* Primary Code128 Barcode & ZMU */}
        <div className="flex flex-col items-center justify-center my-1 pt-2 border-t border-dashed border-gray-300">
          <div className="w-full flex justify-center overflow-hidden py-1">
            <Code128Barcode value={unit.unitCode} width={1.8} height={42} className="max-w-full" />
          </div>
          <p className="font-mono font-black text-sm tracking-widest text-black mt-1">
            {unit.unitCode}
          </p>
        </div>
      </div>
    );
  }

  const handleClick = (e: React.MouseEvent) => {
    if (isSelectionMode) {
      e.stopPropagation();
      onToggleSelect?.();
    } else if (isInteractive) {
      onClick?.();
    }
  };

  return (
    <div
      onClick={handleClick}
      className={`unit-label-card bg-white border rounded-xl p-3 shadow-xs flex flex-col justify-between relative ${
        isSelected
          ? 'border-black ring-2 ring-black bg-slate-50/50'
          : 'border-gray-300'
      } ${
        isInteractive
          ? 'cursor-pointer hover:border-black hover:shadow-md hover:scale-[1.01] transition-all'
          : ''
      }`}
      style={{ minHeight: '160px' }}
      title={
        isSelectionMode
          ? isSelected ? 'Снять выбор' : 'Выбрать этикетку'
          : isInteractive ? 'Нажмите для предпросмотра и печати' : undefined
      }
    >
      {/* Selection Checkbox (Screen Only) */}
      {isSelectionMode && (
        <div className="no-print absolute top-2 right-2 z-10">
          <input
            type="checkbox"
            checked={isSelected}
            onChange={(e) => {
              e.stopPropagation();
              onToggleSelect?.();
            }}
            aria-label={`Выбрать ${unit.unitCode}`}
            className="w-4 h-4 rounded text-black border-gray-400 focus:ring-black cursor-pointer"
          />
        </div>
      )}

      {/* Top Brand & Sequence */}
      <div>
        <div className="flex items-center justify-between border-b border-black pb-0.5 mb-1">
          <span className="text-[11px] font-black tracking-tight text-black">ZAMK</span>
          <span className={`text-[8px] font-mono text-gray-600 ${isSelectionMode ? 'pr-5' : ''}`}>
            {supplyNumber} · {unitIndex + 1} из {variantTotal}
          </span>
        </div>

        {/* Product Title */}
        <h4 className="text-[10px] font-bold text-black truncate leading-tight" title={unit.productTitle}>
          {unit.productTitle}
        </h4>

        {/* Color / Size & SKU */}
        <div className="flex items-baseline justify-between text-[8px] text-gray-800 mt-0.5">
          {variantDetails && <span className="font-semibold truncate max-w-[55%]">{variantDetails}</span>}
          {unit.sellerSku && <span className="font-mono text-gray-600 truncate">Арт: {unit.sellerSku}</span>}
        </div>

        {/* Secondary Variant Barcode (ZMK) */}
        {unit.variantBarcode && (
          <div className="text-[7px] font-mono text-gray-500 mt-0.5">
            ZMK: {unit.variantBarcode}
          </div>
        )}
      </div>

      {/* Primary Code128 Barcode & ZMU + Progress Marks */}
      <div>
        <div className="flex flex-col items-center justify-center my-1 pt-1 border-t border-dashed border-gray-300">
          <div className="w-full flex justify-center overflow-hidden py-0.5">
            <Code128Barcode value={unit.unitCode} width={1.2} height={28} className="max-w-full" />
          </div>
          <p className="font-mono font-black text-[9px] tracking-wider text-black mt-0.5">
            {unit.unitCode}
          </p>
        </div>

        {/* Screen-Only Progress Badges */}
        {(isViewed || isPrintRequested) && (
          <div className="no-print flex flex-wrap items-center gap-1 pt-1 border-t border-gray-100 mt-1">
            {isViewed && (
              <span
                title="Просмотрено"
                className="inline-flex items-center text-[8px] font-medium text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-200/50"
              >
                ✓ Просмотрено
              </span>
            )}
            {isPrintRequested && (
              <span
                title="Отправлено на печать"
                className="inline-flex items-center text-[8px] font-medium text-indigo-700 bg-indigo-50 px-1.5 py-0.5 rounded border border-indigo-200/50"
              >
                🖨 Отправлено на печать
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export const SellerSupplyUnitLabels: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<SellerSupplyUnitLabelsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<{ code?: string; message: string } | null>(null);

  // Modal State
  const [selectedModalState, setSelectedModalState] = useState<{
    variant: GroupedVariant;
    unitIndex: number;
  } | null>(null);

  // Selection Mode State (Arbitrary ZMU selection)
  const [isSelectionMode, setIsSelectionMode] = useState(false);
  const [selectedUnitIds, setSelectedUnitIds] = useState<Set<string>>(new Set());

  // Session-Only Progress Marks
  const [viewedUnitIds, setViewedUnitIds] = useState<Set<string>>(new Set());
  const [printRequestedUnitIds, setPrintRequestedUnitIds] = useState<Set<string>>(new Set());

  // Active Print Target Container (null = Bulk print all; array = Target specific units)
  const [activePrintTargets, setActivePrintTargets] = useState<PrintTargetUnit[] | null>(null);

  useEffect(() => {
    if (id) {
      fetchUnitLabels(id);
    }
  }, [id]);

  const fetchUnitLabels = async (supplyId: string) => {
    try {
      setLoading(true);
      setError(null);
      const res = await getSellerSupplyUnitLabels(supplyId);
      setData(res);
    } catch (err: any) {
      const code = err?.response?.data?.error?.code || err?.code || '';
      let message = err?.response?.data?.error?.message || err?.message || 'Не удалось загрузить этикетки товаров';
      if (code === 'supply_unit_identity_mismatch') {
        message = 'Количество товарных этикеток не совпадает с составом поставки. Обновите страницу или обратитесь к администратору.';
      } else if (code === 'supply_not_found') {
        message = 'Поставка не найдена.';
      } else if (code === 'supply_forbidden') {
        message = 'У вас нет доступа к этой поставке.';
      }
      setError({ code, message });
    } finally {
      setLoading(false);
    }
  };

  const groupedProducts = useMemo<GroupedProduct[]>(() => {
    if (!data?.units || data.units.length === 0) return [];

    const productsMap = new Map<string, {
      productTitle: string;
      variantsMap: Map<string, GroupedVariant>;
    }>();

    for (const unit of data.units) {
      const productKey = unit.productTitle || 'Товар';
      let productEntry = productsMap.get(productKey);
      if (!productEntry) {
        productEntry = {
          productTitle: productKey,
          variantsMap: new Map(),
        };
        productsMap.set(productKey, productEntry);
      }

      const variantKey = unit.productVariantId || `${unit.colorName || ''}-${unit.sizeName || ''}-${unit.sellerSku || ''}`;
      let variantEntry = productEntry.variantsMap.get(variantKey);
      if (!variantEntry) {
        const variantDetails = [unit.colorName, unit.sizeName].filter(Boolean).join(' · ');
        variantEntry = {
          variantId: variantKey,
          colorName: unit.colorName,
          sizeName: unit.sizeName,
          sellerSku: unit.sellerSku,
          variantBarcode: unit.variantBarcode,
          variantDetails: variantDetails || 'Основной вариант',
          units: [],
        };
        productEntry.variantsMap.set(variantKey, variantEntry);
      }

      variantEntry.units.push(unit);
    }

    const result: GroupedProduct[] = [];
    for (const productEntry of productsMap.values()) {
      const variants = Array.from(productEntry.variantsMap.values());
      const totalUnits = variants.reduce((sum, v) => sum + v.units.length, 0);
      result.push({
        productTitle: productEntry.productTitle,
        variants,
        totalUnits,
      });
    }

    return result;
  }, [data?.units]);

  // Mark viewed when modal opens or active preview changes
  useEffect(() => {
    if (!selectedModalState) return;
    const currentUnit = selectedModalState.variant.units[selectedModalState.unitIndex];
    if (currentUnit) {
      setViewedUnitIds(prev => {
        if (prev.has(currentUnit.inventoryUnitId)) return prev;
        const next = new Set(prev);
        next.add(currentUnit.inventoryUnitId);
        return next;
      });
    }
  }, [selectedModalState]);

  // Keyboard navigation & escape listener for modal
  useEffect(() => {
    if (!selectedModalState) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setSelectedModalState(null);
      } else if (e.key === 'ArrowLeft') {
        setSelectedModalState(prev => {
          if (!prev || prev.unitIndex <= 0) return prev;
          return { ...prev, unitIndex: prev.unitIndex - 1 };
        });
      } else if (e.key === 'ArrowRight') {
        setSelectedModalState(prev => {
          if (!prev || prev.unitIndex >= prev.variant.units.length - 1) return prev;
          return { ...prev, unitIndex: prev.unitIndex + 1 };
        });
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedModalState]);

  // 1. Bulk Print: All supply units
  const handleBulkPrint = () => {
    if (!data?.units) return;
    setPrintRequestedUnitIds(prev => {
      const next = new Set(prev);
      data.units.forEach(u => next.add(u.inventoryUnitId));
      return next;
    });
    window.print();
  };

  // 2. Single Label Print: Exactly the active previewed unit
  const handleSinglePrint = () => {
    if (!selectedModalState) return;
    const currentUnit = selectedModalState.variant.units[selectedModalState.unitIndex];
    if (!currentUnit) return;

    setPrintRequestedUnitIds(prev => {
      const next = new Set(prev);
      next.add(currentUnit.inventoryUnitId);
      return next;
    });

    flushSync(() => {
      setActivePrintTargets([{
        unit: currentUnit,
        unitIndex: selectedModalState.unitIndex,
        variantTotal: selectedModalState.variant.units.length,
      }]);
    });

    try {
      window.print();
    } finally {
      setActivePrintTargets(null);
    }
  };

  // 3. Variant Group Print: Exactly the units in that variant
  const handleVariantPrint = (variant: GroupedVariant) => {
    setPrintRequestedUnitIds(prev => {
      const next = new Set(prev);
      variant.units.forEach(u => next.add(u.inventoryUnitId));
      return next;
    });

    const targets: PrintTargetUnit[] = variant.units.map((unit, idx) => ({
      unit,
      unitIndex: idx,
      variantTotal: variant.units.length,
    }));

    flushSync(() => {
      setActivePrintTargets(targets);
    });

    try {
      window.print();
    } finally {
      setActivePrintTargets(null);
    }
  };

  // 4. Selected Labels Print: Exactly selected units
  const handleSelectedPrint = () => {
    if (selectedUnitIds.size === 0) return;

    setPrintRequestedUnitIds(prev => {
      const next = new Set(prev);
      selectedUnitIds.forEach(id => next.add(id));
      return next;
    });

    const targets: PrintTargetUnit[] = [];
    for (const product of groupedProducts) {
      for (const variant of product.variants) {
        variant.units.forEach((unit, idx) => {
          if (selectedUnitIds.has(unit.inventoryUnitId)) {
            targets.push({
              unit,
              unitIndex: idx,
              variantTotal: variant.units.length,
            });
          }
        });
      }
    }

    flushSync(() => {
      setActivePrintTargets(targets);
    });

    try {
      window.print();
    } finally {
      setActivePrintTargets(null);
    }
  };

  // Selection Mode helpers
  const handleToggleUnitSelect = (unitId: string) => {
    setSelectedUnitIds(prev => {
      const next = new Set(prev);
      if (next.has(unitId)) {
        next.delete(unitId);
      } else {
        next.add(unitId);
      }
      return next;
    });
  };

  const handleSelectAll = () => {
    if (!data?.units) return;
    setSelectedUnitIds(new Set(data.units.map(u => u.inventoryUnitId)));
  };

  const handleClearSelection = () => {
    setSelectedUnitIds(new Set());
  };

  const handleExitSelectionMode = () => {
    setSelectedUnitIds(new Set());
    setIsSelectionMode(false);
  };

  const handleToggleVariantSelect = (variant: GroupedVariant) => {
    setSelectedUnitIds(prev => {
      const next = new Set(prev);
      const allSelected = variant.units.every(u => next.has(u.inventoryUnitId));
      if (allSelected) {
        variant.units.forEach(u => next.delete(u.inventoryUnitId));
      } else {
        variant.units.forEach(u => next.add(u.inventoryUnitId));
      }
      return next;
    });
  };

  const handleCloseModal = () => {
    setSelectedModalState(null);
  };

  const handlePrevUnit = () => {
    setSelectedModalState(prev => {
      if (!prev || prev.unitIndex <= 0) return prev;
      return { ...prev, unitIndex: prev.unitIndex - 1 };
    });
  };

  const handleNextUnit = () => {
    setSelectedModalState(prev => {
      if (!prev || prev.unitIndex >= prev.variant.units.length - 1) return prev;
      return { ...prev, unitIndex: prev.unitIndex + 1 };
    });
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex flex-col items-center justify-center p-6">
        <Loader2 className="w-8 h-8 animate-spin text-gray-500 mb-3" />
        <p className="text-sm font-medium text-gray-600">Загружаем этикетки...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-200 max-w-md w-full text-center">
          <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">Ошибка</h2>
          <p className="text-gray-600 mb-6">{error?.message || 'Не удалось загрузить этикетки'}</p>
          <Link
            to={`/supplies/${id || ''}`}
            className="inline-flex items-center justify-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 transition-colors"
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Назад к поставке
          </Link>
        </div>
      </div>
    );
  }

  // Legacy Supply without serialized units
  if (!data.serialized || data.totalUnits === 0 || !data.units || data.units.length === 0) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-200 max-w-lg w-full text-center">
          <Info className="w-12 h-12 text-blue-500 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">Индивидуальные этикетки недоступны</h2>
          <p className="text-gray-700 font-medium mb-2">
            Для этой поставки индивидуальные этикетки ZAMK не создавались.
          </p>
          <p className="text-sm text-gray-500 mb-6">
            Эта поставка была создана до перехода на индивидуальную маркировку товаров.
          </p>
          <Link
            to={`/supplies/${data.supplyId}`}
            className="inline-flex items-center justify-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 transition-colors"
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Назад к поставке
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-100 py-8 px-4 sm:px-6">
      <style>{`
        @media print {
          @page {
            size: 58mm 40mm;
            margin: 0;
          }
          html, body {
            margin: 0 !important;
            padding: 0 !important;
            background: #ffffff !important;
            width: 58mm !important;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
          }
          .no-print {
            display: none !important;
          }
          .print-hidden {
            display: none !important;
          }
          .print-target-container {
            display: block !important;
            margin: 0 !important;
            padding: 0 !important;
          }
          section, div {
            box-shadow: none !important;
            border-color: transparent !important;
            background: transparent !important;
          }
          .unit-labels-grid {
            display: block !important;
            margin: 0 !important;
            padding: 0 !important;
          }
          .unit-label-card {
            width: 58mm !important;
            height: 40mm !important;
            max-width: 58mm !important;
            max-height: 40mm !important;
            margin: 0 !important;
            padding: 2mm 2.5mm !important;
            box-sizing: border-box !important;
            box-shadow: none !important;
            border: none !important;
            border-radius: 0 !important;
            page-break-after: always !important;
            break-after: page !important;
            page-break-inside: avoid !important;
            break-inside: avoid !important;
            display: flex !important;
            flex-direction: column !important;
            justify-content: space-between !important;
            background: #ffffff !important;
            overflow: hidden !important;
          }
        }
        @media screen {
          .print-target-container {
            display: none !important;
          }
        }
      `}</style>

      {/* Screen Toolbar */}
      <div className="no-print max-w-5xl mx-auto mb-8 bg-white p-6 rounded-2xl shadow-sm border border-gray-200">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <Link
                to={`/supplies/${data.supplyId}`}
                className="inline-flex items-center text-sm font-bold text-gray-600 hover:text-gray-900 bg-gray-100 px-3 py-1.5 rounded-lg transition-colors"
              >
                <ArrowLeft className="w-4 h-4 mr-1.5" />
                Назад к поставке
              </Link>
              <h1 className="text-xl font-bold text-gray-900">Этикетки товаров</h1>
            </div>
            <p className="text-sm font-medium text-gray-500 mt-2">
              Поставка <span className="font-mono font-bold text-gray-800">{data.supplyNumber}</span> · {data.totalUnits} {getLabelsWord(data.totalUnits)}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2 sm:gap-3">
            {!isSelectionMode ? (
              <>
                <button
                  onClick={handleBulkPrint}
                  className="inline-flex items-center px-5 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 shadow-sm transition-colors cursor-pointer"
                >
                  <Printer className="w-4 h-4 mr-2" />
                  Печать всех ({data.totalUnits})
                </button>
                <button
                  onClick={() => setIsSelectionMode(true)}
                  className="inline-flex items-center px-4 py-2.5 bg-white border border-gray-300 text-gray-800 font-bold rounded-xl text-sm hover:bg-gray-50 shadow-xs transition-colors cursor-pointer"
                >
                  <CheckSquare className="w-4 h-4 mr-1.5" />
                  Выбрать
                </button>
              </>
            ) : (
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-bold text-gray-800 mr-1">
                  Выбрано: {selectedUnitIds.size}
                </span>
                <button
                  onClick={handleSelectedPrint}
                  disabled={selectedUnitIds.size === 0}
                  className="inline-flex items-center px-4 py-2 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 disabled:opacity-30 disabled:cursor-not-allowed shadow-sm transition-colors cursor-pointer"
                >
                  <Printer className="w-4 h-4 mr-1.5" />
                  Печать выбранных ({selectedUnitIds.size})
                </button>
                <button
                  onClick={handleSelectAll}
                  className="px-3 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 font-semibold rounded-xl text-xs transition-colors cursor-pointer"
                >
                  Выбрать все
                </button>
                <button
                  onClick={handleClearSelection}
                  className="px-3 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 font-semibold rounded-xl text-xs transition-colors cursor-pointer"
                >
                  Снять выделение
                </button>
                <button
                  onClick={handleExitSelectionMode}
                  className="px-4 py-2 bg-white border border-gray-300 hover:bg-gray-50 text-gray-800 font-bold rounded-xl text-xs transition-colors cursor-pointer"
                >
                  Готово
                </button>
              </div>
            )}
          </div>
        </div>

        <div className="mt-4 pt-4 border-t border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs text-gray-500">
          <p>
            {isSelectionMode
              ? 'Выберите конкретные этикетки для печати или используйте быстрый выбор групп.'
              : 'Наклейте по одной этикетке на каждую физическую единицу товара. Нажмите на этикетку для предпросмотра.'}
          </p>
          <p className="font-medium text-gray-400">Повторная печать не создаёт новые ZMU.</p>
        </div>
      </div>

      {/* Dedicated Clean Print Target Container for Single, Variant, and Selected Print Scopes */}
      {activePrintTargets && (
        <div className="print-target-container">
          {activePrintTargets.map((target) => (
            <UnitLabelCard
              key={target.unit.inventoryUnitId}
              unit={target.unit}
              supplyNumber={data.supplyNumber}
              unitIndex={target.unitIndex}
              variantTotal={target.variantTotal}
            />
          ))}
        </div>
      )}

      {/* Grouped Products & Variants */}
      <div className={`max-w-5xl mx-auto space-y-10 ${activePrintTargets ? 'print-hidden' : ''}`}>
        {groupedProducts.map((product) => (
          <section key={product.productTitle} className="space-y-6">
            {/* Product Header */}
            <div className="no-print border-b border-gray-200 pb-3 flex items-baseline justify-between">
              <h2 className="text-xl font-bold text-gray-900 tracking-tight">
                {product.productTitle}
              </h2>
              <span className="text-xs text-gray-500 font-medium">
                {product.totalUnits} {getLabelsWord(product.totalUnits)}
              </span>
            </div>

            {/* Variant Sections */}
            <div className="space-y-6">
              {product.variants.map((variant) => {
                const variantTotal = variant.units.length;

                return (
                  <div
                    key={variant.variantId}
                    className="bg-white rounded-2xl p-5 sm:p-6 border border-gray-200 shadow-xs space-y-4"
                  >
                    {/* Variant Header */}
                    <div className="no-print flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-gray-100 pb-3">
                      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                        <h3 className="text-base font-bold text-gray-900">
                          {variant.variantDetails}
                        </h3>
                        {variant.sellerSku && (
                          <span className="text-xs font-mono text-gray-600 bg-gray-100 px-2 py-0.5 rounded">
                            Арт: {variant.sellerSku}
                          </span>
                        )}
                        {variant.variantBarcode && (
                          <span className="text-xs font-mono text-gray-400">
                            ZMK: {variant.variantBarcode}
                          </span>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-xs font-semibold text-gray-600 bg-gray-100 px-3 py-1 rounded-full w-fit">
                          {variantTotal} {getLabelsWord(variantTotal)}
                        </span>

                        {isSelectionMode && (
                          <button
                            onClick={() => handleToggleVariantSelect(variant)}
                            className="text-xs font-semibold text-gray-700 bg-gray-100 hover:bg-gray-200 px-2.5 py-1 rounded-lg transition-colors cursor-pointer"
                          >
                            {variant.units.every(u => selectedUnitIds.has(u.inventoryUnitId))
                              ? 'Снять с группы'
                              : 'Выбрать группу'}
                          </button>
                        )}

                        <button
                          onClick={() => handleVariantPrint(variant)}
                          title={`Печать: ${variant.variantDetails}`}
                          aria-label={`Печать группы: ${variant.variantDetails}`}
                          className="inline-flex items-center text-xs font-bold text-gray-800 bg-gray-100 hover:bg-gray-200 border border-gray-200 px-3 py-1 rounded-lg transition-colors cursor-pointer"
                        >
                          <Printer className="w-3.5 h-3.5 mr-1.5" />
                          Печать «{variant.variantDetails}» ({variantTotal})
                        </button>
                      </div>
                    </div>

                    {/* Variant Units Grid */}
                    <div className="unit-labels-grid grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
                      {variant.units.map((unit, unitIdx) => (
                        <UnitLabelCard
                          key={unit.inventoryUnitId}
                          unit={unit}
                          supplyNumber={data.supplyNumber}
                          unitIndex={unitIdx}
                          variantTotal={variantTotal}
                          isInteractive={!isSelectionMode}
                          isSelectionMode={isSelectionMode}
                          isSelected={selectedUnitIds.has(unit.inventoryUnitId)}
                          onToggleSelect={() => handleToggleUnitSelect(unit.inventoryUnitId)}
                          isViewed={viewedUnitIds.has(unit.inventoryUnitId)}
                          isPrintRequested={printRequestedUnitIds.has(unit.inventoryUnitId)}
                          onClick={() => setSelectedModalState({ variant, unitIndex: unitIdx })}
                        />
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </section>
        ))}
      </div>

      {/* Single Label Preview Modal */}
      {selectedModalState && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs no-print"
          role="dialog"
          aria-modal="true"
          aria-label="Предпросмотр этикетки"
          onClick={handleCloseModal}
        >
          <div
            className="bg-white rounded-3xl p-6 sm:p-8 max-w-lg w-full shadow-2xl space-y-6"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Top Bar */}
            <div className="flex items-center justify-between border-b border-gray-100 pb-4">
              <div>
                <h3 className="text-lg font-bold text-gray-900">Этикетка товара</h3>
                <p className="text-xs text-gray-500 mt-0.5">
                  {selectedModalState.variant.variantDetails} · Поставка {data.supplyNumber}
                </p>
              </div>
              <button
                onClick={handleCloseModal}
                aria-label="Закрыть"
                className="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Large Label Preview */}
            <div className="flex justify-center py-2">
              <UnitLabelCard
                unit={selectedModalState.variant.units[selectedModalState.unitIndex]}
                supplyNumber={data.supplyNumber}
                unitIndex={selectedModalState.unitIndex}
                variantTotal={selectedModalState.variant.units.length}
                size="large"
              />
            </div>

            {/* Navigation Controls (Within Variant Group) */}
            <div className="flex items-center justify-between bg-gray-50 rounded-2xl p-3 border border-gray-100">
              <button
                onClick={handlePrevUnit}
                disabled={selectedModalState.unitIndex === 0}
                aria-label="Предыдущая этикетка"
                className="inline-flex items-center px-4 py-2 rounded-xl text-sm font-bold text-gray-700 bg-white border border-gray-200 shadow-xs hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all cursor-pointer"
              >
                <ChevronLeft className="w-4 h-4 mr-1" />
                Назад
              </button>

              <span className="text-sm font-bold font-mono text-gray-800">
                {selectedModalState.unitIndex + 1} из {selectedModalState.variant.units.length}
              </span>

              <button
                onClick={handleNextUnit}
                disabled={selectedModalState.unitIndex === selectedModalState.variant.units.length - 1}
                aria-label="Следующая этикетка"
                className="inline-flex items-center px-4 py-2 rounded-xl text-sm font-bold text-gray-700 bg-white border border-gray-200 shadow-xs hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-all cursor-pointer"
              >
                Вперёд
                <ChevronRight className="w-4 h-4 ml-1" />
              </button>
            </div>

            {/* Actions */}
            <div className="flex flex-col sm:flex-row items-center justify-end gap-3 pt-2">
              <button
                onClick={handleCloseModal}
                className="w-full sm:w-auto px-5 py-2.5 rounded-xl border border-gray-300 text-sm font-bold text-gray-700 hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Закрыть
              </button>
              <button
                onClick={handleSinglePrint}
                className="w-full sm:w-auto inline-flex items-center justify-center px-6 py-2.5 rounded-xl bg-black text-white text-sm font-bold shadow-sm hover:bg-gray-800 transition-colors cursor-pointer"
              >
                <Printer className="w-4 h-4 mr-2" />
                Печать этой этикетки
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

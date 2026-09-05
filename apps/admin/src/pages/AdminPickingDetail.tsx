import React, { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Scan,
  CheckCircle2,
  AlertCircle,
  AlertTriangle,
  Package,
  PackageCheck,
  RefreshCw,
  Tag,
  Check,
  Circle,
  X,
} from 'lucide-react';
import {
  getAdminPickingOrder,
  scanPickingCode,
  getCompatibleUnits,
  getPickingErrorMessage,
  isCanonicalScannerCode,
  PickingOrder,
  PickingScanResult,
  CompatibleUnit,
} from '../api/adminPicking';
import { formatOrderNumber } from '../utils/orderFormatters';
import { useAdminAuth } from '../contexts/AdminAuthContext';

interface FeedbackMessage {
  type: 'success' | 'warning' | 'error';
  title: string;
  detail?: string;
}

export const isPlaceholderBarcode = (barcode?: string | null): boolean => {
  if (!barcode) return true;
  const trimmed = barcode.trim();
  if (!trimmed) return true;
  return /^0+$/.test(trimmed);
};

export interface LegacyTargetInfo {
  code: string;
  type: 'barcode' | 'sku';
  label: string;
  instruction: string;
}

export const getActionableLegacyTarget = (
  barcode?: string | null,
  sku?: string | null
): LegacyTargetInfo | null => {
  if (barcode && !isPlaceholderBarcode(barcode)) {
    return {
      code: barcode.trim(),
      type: 'barcode',
      label: 'ОЖИДАЕМЫЙ ШТРИХКОД (ZMK)',
      instruction: 'Отсканируйте штрихкод товара',
    };
  }
  if (sku && sku.trim()) {
    return {
      code: sku.trim(),
      type: 'sku',
      label: 'ОЖИДАЕМЫЙ АРТИКУЛ (SKU)',
      instruction: 'Отсканируйте артикул (SKU) товара',
    };
  }
  return null;
};

export function AdminPickingDetail() {
  const { id } = useParams<{ id: string }>();
  const { hasPermission } = useAdminAuth();
  const canPick = hasPermission('warehouse.picking');

  const [pickingOrder, setPickingOrder] = useState<PickingOrder | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [scanInput, setScanInput] = useState('');
  const [isScanning, setIsScanning] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackMessage | null>(null);

  // Compatible units drawer state
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [isLoadingUnits, setIsLoadingUnits] = useState(false);
  const [compatibleUnits, setCompatibleUnits] = useState<CompatibleUnit[]>([]);
  const [drawerError, setDrawerError] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  const fetchOrder = async (showLoading = true) => {
    if (!id) return;
    try {
      if (showLoading) setIsLoading(true);
      setError(null);
      const data = await getAdminPickingOrder(id);
      setPickingOrder(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить заказ для сборки.');
    } finally {
      if (showLoading) setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchOrder(true);
  }, [id]);

  useEffect(() => {
    // Focus scanner input on load or when order updates
    inputRef.current?.focus();
  }, [isLoading, pickingOrder]);

  const currentActiveItem = pickingOrder?.items.find((i) => i.remainingQuantity > 0);

  const handleScanSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = scanInput.trim();
    if (!code || isScanning || !id) return;

    setIsScanning(true);
    setFeedback(null);

    try {
      const res: PickingScanResult = await scanPickingCode(id, code, currentActiveItem?.orderItemId);

      if (res.scanResult.substituted) {
        setFeedback({
          type: 'success',
          title: 'Единица выбрана и добавлена в сборку',
          detail: `${code} привязана к заказу и отмечена как собранная`,
        });
      } else if (res.scanResult.newlyPicked) {
        setFeedback({
          type: 'success',
          title: 'Товар добавлен в сборку',
          detail: `Отсканирован: ${code}`,
        });
      } else if (res.scanResult.alreadyPicked) {
        setFeedback({
          type: 'warning',
          title: 'Эта единица уже была отобрана.',
          detail: `ZMU: ${code}`,
        });
      } else if (res.scanResult.alreadyComplete) {
        setFeedback({
          type: 'warning',
          title: 'Нужное количество этого товара уже собрано.',
          detail: `Штрихкод: ${code}`,
        });
      }

      // Refresh real picking order data from backend
      await fetchOrder(false);
    } catch (err: any) {
      const msg = getPickingErrorMessage(err);
      const isCanonical = isCanonicalScannerCode(code);
      setFeedback({
        type: 'error',
        title: msg,
        detail: isCanonical ? `Код: ${code}` : undefined,
      });
    } finally {
      setIsScanning(false);
      setScanInput('');
      setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
    }
  };

  const openCompatibleUnitsDrawer = async () => {
    if (!activeItem || !id) return;
    setIsDrawerOpen(true);
    setIsLoadingUnits(true);
    setDrawerError(null);
    try {
      const units = await getCompatibleUnits(id, activeItem.orderItemId);
      setCompatibleUnits(units);
    } catch (err: any) {
      setDrawerError(err.message || 'Не удалось загрузить список единиц');
    } finally {
      setIsLoadingUnits(false);
    }
  };

  if (isLoading) {
    return (
      <div className="p-12 text-center text-gray-500 bg-white rounded-xl border border-gray-200 shadow-sm max-w-4xl mx-auto mt-6">
        <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-600 mb-3" />
        <p className="text-sm font-medium">Загрузка данных заказа...</p>
      </div>
    );
  }

  if (error || !pickingOrder) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-6">
        <div className="p-6 rounded-2xl bg-rose-50 border border-rose-200 text-rose-800 space-y-4 shadow-sm">
          <div className="flex items-center space-x-3">
            <AlertCircle className="w-6 h-6 text-rose-600 shrink-0" />
            <h2 className="text-lg font-bold text-gray-900">Ошибка загрузки заказа</h2>
          </div>
          <p className="text-sm text-gray-700">{error || 'Заказ не найден или недоступен для сборки.'}</p>
          <div className="flex space-x-4 pt-2">
            <button
              onClick={() => fetchOrder(true)}
              className="px-4 py-2 bg-rose-600 hover:bg-rose-700 rounded-xl text-sm font-semibold text-white transition-colors"
            >
              Попробовать снова
            </button>
            <Link
              to="/fulfillment/picking"
              className="px-4 py-2 bg-white hover:bg-gray-50 border border-gray-300 rounded-xl text-sm font-semibold text-gray-700 transition-colors"
            >
              Вернуться в очередь
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const totalQuantity = pickingOrder.items.reduce((sum, i) => sum + i.quantity, 0);
  const pickedQuantity = pickingOrder.items.reduce((sum, i) => sum + i.pickedQuantity, 0);
  const remainingQuantity = Math.max(0, totalQuantity - pickedQuantity);
  const isComplete = totalQuantity > 0 && pickedQuantity === totalQuantity;
  const progressPercent = totalQuantity > 0 ? Math.round((pickedQuantity / totalQuantity) * 100) : 0;

  // Active target unit derivation
  const activeItem = pickingOrder.items.find((i) => i.remainingQuantity > 0);
  const legacyTarget = activeItem ? getActionableLegacyTarget(activeItem.barcode, activeItem.sku) : null;

  return (
    <div data-testid="picking-detail-page" className="max-w-5xl mx-auto space-y-6 pb-16 px-4 sm:px-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-gray-200 pb-5">
        <div className="space-y-1">
          <Link
            to="/fulfillment/picking"
            className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900 transition-colors mb-2"
          >
            <ArrowLeft className="w-4 h-4 mr-1.5" />
            К очереди сборок
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-bold text-gray-900 tracking-tight">
              Заказ #{formatOrderNumber({ id: pickingOrder.orderId, orderNumber: pickingOrder.orderNumber })}
            </h1>
            <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">
              Сборка
            </span>
          </div>
        </div>

        <div className="flex items-center gap-4 bg-white border border-gray-200 rounded-xl px-5 py-3 shadow-sm">
          <div className="text-right">
            <div className="text-xs text-gray-500 font-medium">Прогресс сборки</div>
            <div className="text-xl font-bold text-gray-900">
              {pickedQuantity} / {totalQuantity}
            </div>
          </div>
          <div className="w-12 h-12 relative flex items-center justify-center bg-indigo-50 rounded-xl border border-indigo-100">
            <div className="text-xs font-bold text-indigo-700">{progressPercent}%</div>
          </div>
        </div>
      </div>

      {/* Completion Banner */}
      {isComplete && (
        <div className="p-6 rounded-2xl bg-emerald-50 border border-emerald-200 text-emerald-900 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm">
          <div className="flex items-center space-x-3.5">
            <CheckCircle2 className="w-8 h-8 text-emerald-600 shrink-0" />
            <div>
              <h3 className="text-lg font-bold text-gray-900">
                Сборка завершена ({pickedQuantity} / {totalQuantity})
              </h3>
              <p className="text-sm text-emerald-800 mt-0.5 font-medium">
                Все позиции заказа успешно укомплектованы на складе.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Link
              to={`/fulfillment/packing/${pickingOrder.fulfillmentId}`}
              className="inline-flex items-center gap-2 px-5 py-3 rounded-xl text-sm font-bold bg-emerald-600 hover:bg-emerald-700 text-white transition-colors shadow-sm"
            >
              <PackageCheck className="w-5 h-5" />
              Перейти к упаковке
            </Link>
          </div>
        </div>
      )}

      {/* GUIDED PICKING TARGET (TASK 4) */}
      {!isComplete && activeItem && (
        <div className="bg-white border-2 border-indigo-500/30 rounded-2xl p-6 shadow-sm space-y-5" data-testid="guided-picking-target">
          <div className="flex items-center justify-between border-b border-gray-100 pb-3">
            <span className="text-xs font-bold uppercase tracking-wider text-indigo-700 bg-indigo-50 px-3 py-1 rounded-full flex items-center gap-1.5">
              <Scan className="w-3.5 h-3.5" />
              СЕЙЧАС НУЖНО ВЗЯТЬ
            </span>
            <span className="text-xs text-gray-500 font-medium">
              Осталось собрать: <strong className="text-gray-900 font-bold">{remainingQuantity} шт.</strong>
            </span>
          </div>

          <div className="flex flex-col sm:flex-row items-start sm:items-center gap-5">
            {activeItem.imageUrl ? (
              <img
                src={activeItem.imageUrl}
                alt={activeItem.title}
                className="w-20 h-20 sm:w-24 sm:h-24 object-cover rounded-xl border border-gray-200 shrink-0 bg-gray-50"
              />
            ) : (
              <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-xl border border-gray-200 bg-gray-100 flex items-center justify-center shrink-0">
                <Package className="w-8 h-8 text-gray-400" />
              </div>
            )}

            <div className="space-y-1.5 flex-1 min-w-0">
              <h3 className="text-lg sm:text-xl font-bold text-gray-900 tracking-tight">
                {activeItem.title}
              </h3>
              {(activeItem.variantSize || activeItem.variantColor) && (
                <p className="text-sm font-semibold text-gray-600">
                  {[activeItem.variantSize, activeItem.variantColor].filter(Boolean).join(' · ')}
                </p>
              )}
              <div className="flex items-center gap-2 pt-1">
                <span className="inline-flex items-center px-2.5 py-0.5 rounded text-xs font-semibold bg-gray-100 text-gray-700">
                  Позиция: {activeItem.pickedQuantity} / {activeItem.quantity} собрано
                </span>
                {activeItem.allocationMode === 'serialized' ? (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                    Маркированный (ZMU)
                  </span>
                ) : (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-gray-100 text-gray-700 border border-gray-200">
                    Штрихкод (Legacy)
                  </span>
                )}
              </div>
            </div>
          </div>

          {/* Expected Code Box */}
          <div className="p-4 rounded-xl bg-slate-900 text-white space-y-2">
            {activeItem.allocationMode === 'serialized' ? (
              <>
                <div className="flex items-center justify-between text-xs font-semibold text-slate-400 uppercase tracking-wider">
                  <span>
                    ПОДХОДЯЩИХ ЕДИНИЦ НА СКЛАДЕ: {activeItem.compatibleUnitsCount ?? activeItem.remainingQuantity}
                  </span>
                  <button
                    type="button"
                    onClick={openCompatibleUnitsDrawer}
                    className="text-xs font-bold text-indigo-400 hover:text-indigo-300 underline underline-offset-2 flex items-center gap-1 transition-colors cursor-pointer"
                    data-testid="view-compatible-units-btn"
                  >
                    Посмотреть подходящие единицы &rarr;
                  </button>
                </div>
                <div
                  className="text-lg sm:text-xl font-bold tracking-tight text-indigo-200"
                  data-testid="expected-unit-code"
                >
                  Возьмите любую свободную единицу
                </div>
                <p className="text-xs text-slate-300">
                  Возьмите любую свободную единицу этого варианта и отсканируйте её ZMU.
                </p>
              </>
            ) : (
              <>
                <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider">
                  {legacyTarget ? legacyTarget.label : 'ОЖИДАЕМЫЙ КОД'}
                </div>
                <div
                  className={`text-xl sm:text-2xl font-mono font-bold tracking-wider break-all ${
                    !legacyTarget ? 'text-amber-400 text-base sm:text-lg font-sans' : 'text-indigo-300'
                  }`}
                  data-testid="expected-unit-code"
                >
                  {legacyTarget ? legacyTarget.code : 'У позиции нет сканируемого складского кода'}
                </div>
                <p className="text-xs text-slate-300">
                  {legacyTarget
                    ? legacyTarget.instruction
                    : 'У товара отсутствует штрихкод и артикул для сканирования. Обратитесь к старшему смены или администратору.'}
                </p>
              </>
            )}
          </div>

          {/* Scanner Input Form */}
          <form onSubmit={handleScanSubmit} className="relative pt-1">
            <input
              id="scanner-input"
              ref={inputRef}
              type="text"
              value={scanInput}
              onChange={(e) => setScanInput(e.target.value)}
              disabled={!canPick || isScanning || (activeItem.allocationMode === 'legacy' && !legacyTarget)}
              placeholder={
                activeItem.allocationMode === 'serialized'
                  ? 'Отсканируйте ZMU подходящей единицы...'
                  : legacyTarget
                  ? legacyTarget.type === 'barcode'
                    ? 'Отсканируйте штрихкод товара...'
                    : 'Отсканируйте артикул (SKU) товара...'
                  : 'Сканирование недоступно: нет кода позиции'
              }
              autoComplete="off"
              className="w-full text-base sm:text-lg bg-gray-50 text-gray-900 placeholder-gray-400 px-4 py-3.5 pl-12 rounded-xl border-2 border-indigo-200 focus:bg-white focus:outline-none focus:border-indigo-600 focus:ring-1 focus:ring-indigo-600 transition-all font-mono disabled:opacity-50"
            />
            <Scan className="w-5 h-5 text-indigo-500 absolute left-4 top-1/2 -translate-y-1/2 mt-0.5" />
            <button
              type="submit"
              disabled={!canPick || isScanning || !scanInput.trim() || (activeItem.allocationMode === 'legacy' && !legacyTarget)}
              className="absolute right-2 top-1/2 -translate-y-1/2 mt-0.5 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-100 disabled:text-gray-400 text-white text-xs font-semibold rounded-lg transition-colors shadow-sm"
            >
              {isScanning ? <RefreshCw className="w-4 h-4 animate-spin" /> : 'Ввод'}
            </button>
          </form>

          {/* Scan Feedback Banner */}
          {feedback && (
            <div
              data-testid="scan-feedback-banner"
              className={`p-4 rounded-xl border flex items-start space-x-3 transition-all shadow-sm ${
                feedback.type === 'success'
                  ? 'bg-emerald-50 border-emerald-200 text-emerald-800'
                  : feedback.type === 'warning'
                  ? 'bg-amber-50 border-amber-200 text-amber-800'
                  : 'bg-rose-50 border-rose-200 text-rose-800'
              }`}
            >
              {feedback.type === 'success' && <CheckCircle2 className="w-5 h-5 text-emerald-600 shrink-0 mt-0.5" />}
              {feedback.type === 'warning' && <AlertTriangle className="w-5 h-5 text-amber-600 shrink-0 mt-0.5" />}
              {feedback.type === 'error' && <AlertCircle className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />}
              <div className="space-y-0.5">
                <div className="text-sm font-semibold">{feedback.title}</div>
                {feedback.detail && <div className="text-xs opacity-90 font-mono">{feedback.detail}</div>}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Items Section */}
      <div className="space-y-4">
        <div className="flex items-center justify-between px-1">
          <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
            <Package className="w-5 h-5 text-gray-500" />
            Состав заказа ({pickingOrder.items.length} {pickingOrder.items.length === 1 ? 'позиция' : 'позиций'})
          </h2>
          <span className="text-xs font-medium text-gray-500">
            Осталось собрать: <span className="text-gray-900 font-bold">{remainingQuantity} шт.</span>
          </span>
        </div>

        <div className="space-y-3">
          {pickingOrder.items.map((item) => {
            const isSerialized = item.allocationMode === 'serialized';
            const itemComplete = item.pickedQuantity === item.quantity;

            return (
              <div
                key={item.orderItemId}
                className={`p-5 rounded-xl border transition-all shadow-sm ${
                  itemComplete
                    ? 'bg-emerald-50/40 border-emerald-200'
                    : 'bg-white border-gray-200'
                }`}
              >
                <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                  <div className="space-y-1.5 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-base font-bold text-gray-900">{item.title}</span>
                      {(item.variantSize || item.variantColor) && (
                        <span className="text-xs text-gray-500 font-medium">
                          {[item.variantSize, item.variantColor].filter(Boolean).join(' · ')}
                        </span>
                      )}
                      {isSerialized ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                          Маркированный (ZMU)
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-gray-100 text-gray-700 border border-gray-200">
                          Без индивидуальной маркировки
                        </span>
                      )}
                    </div>

                    <div className="text-xs text-gray-500 flex items-center gap-3">
                      <span>Количество: <strong className="text-gray-800">{item.quantity} шт.</strong></span>
                      <span>·</span>
                      <span>Собрано: <strong className={itemComplete ? 'text-emerald-700' : 'text-indigo-600'}>{item.pickedQuantity} / {item.quantity}</strong></span>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <div className="text-right">
                      <div className="text-xs font-semibold text-gray-500">
                        {itemComplete ? (
                          <span className="text-emerald-700 flex items-center gap-1 font-bold">
                            <Check className="w-4 h-4 text-emerald-600" /> Собрано полностью
                          </span>
                        ) : (
                          <span>Осталось: <strong className="text-gray-900">{item.remainingQuantity} шт.</strong></span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Serialized ZMU Units List */}
                {isSerialized && (
                  <div className="mt-4 pt-4 border-t border-gray-100 space-y-2">
                    <div className="text-xs font-semibold text-gray-500">
                      Назначенные физические единицы (ZMU):
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
                      {item.allocatedUnits.map((unit) => {
                        const isPicked = unit.pickedAt !== null && unit.pickedAt !== undefined;
                        return (
                          <div
                            key={unit.inventoryUnitId}
                            className={`px-3.5 py-2 rounded-xl border flex items-center justify-between text-xs font-mono transition-all ${
                              isPicked
                                ? 'bg-emerald-50 border-emerald-200 text-emerald-800'
                                : 'bg-gray-50 border-gray-200 text-gray-600'
                            }`}
                          >
                            <span className="font-semibold">{unit.unitCode}</span>
                            {isPicked ? (
                              <span className="inline-flex items-center gap-1 text-[11px] font-bold text-emerald-700">
                                <Check className="w-3.5 h-3.5" /> Собрана
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-[11px] text-gray-500 font-sans">
                                <Circle className="w-3 h-3 text-gray-400" /> Ожидает
                              </span>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {/* Legacy Helper Note */}
                {!isSerialized && (() => {
                  const itemLegacy = getActionableLegacyTarget(item.barcode, item.sku);
                  return (
                    <div className="mt-3 pt-3 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
                      <div className="flex items-center gap-1.5 text-gray-600">
                        <Tag className="w-3.5 h-3.5 text-gray-400" />
                        <span>
                          {itemLegacy
                            ? itemLegacy.type === 'barcode'
                              ? 'Штрихкод товара: '
                              : 'Артикул (SKU): '
                            : 'Код позиции: '}
                          <strong className="font-mono text-gray-800">
                            {itemLegacy ? itemLegacy.code : 'Не указан'}
                          </strong>
                        </span>
                      </div>
                      <div className="text-[11px] text-gray-500">
                        Собрано {item.pickedQuantity} из {item.quantity}
                      </div>
                    </div>
                  );
                })()}
              </div>
            );
          })}
        </div>
      </div>

      {/* COMPATIBLE UNITS DRAWER / MODAL */}
      {isDrawerOpen && activeItem && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs transition-opacity"
          data-testid="compatible-units-drawer"
        >
          <div
            className="bg-white w-full max-w-lg rounded-2xl shadow-xl border border-gray-200 overflow-hidden flex flex-col max-h-[85vh] animate-in fade-in zoom-in-95 duration-150"
            role="dialog"
            aria-modal="true"
          >
            {/* Header */}
            <div className="flex items-start justify-between p-5 border-b border-gray-100 bg-gray-50/50">
              <div>
                <h3 className="text-lg font-bold text-gray-900">Подходящие единицы</h3>
                <p className="text-xs font-medium text-gray-500 mt-0.5">
                  {activeItem.title} · {[activeItem.variantSize, activeItem.variantColor].filter(Boolean).join(' · ')}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsDrawerOpen(false)}
                className="p-1.5 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 transition-colors"
                data-testid="close-compatible-units-btn"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Content List */}
            <div className="p-5 overflow-y-auto space-y-3 flex-1">
              {isLoadingUnits ? (
                <div className="py-8 text-center text-gray-400 flex flex-col items-center gap-2">
                  <RefreshCw className="w-6 h-6 animate-spin text-indigo-500" />
                  <span className="text-xs font-medium">Загрузка подходящих единиц...</span>
                </div>
              ) : drawerError ? (
                <div className="p-4 rounded-xl bg-rose-50 border border-rose-200 text-rose-700 text-xs flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 shrink-0" />
                  <span>{drawerError}</span>
                </div>
              ) : compatibleUnits.length === 0 ? (
                <div className="py-8 text-center text-gray-400 text-xs">
                  Нет доступных единиц для сканирования
                </div>
              ) : (
                <div className="space-y-2">
                  {compatibleUnits.map((u) => (
                    <div
                      key={u.inventoryUnitId}
                      className="p-3.5 rounded-xl border border-gray-200 bg-gray-50/60 flex items-center justify-between hover:bg-indigo-50/30 hover:border-indigo-200 transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <Tag className="w-4 h-4 text-gray-400" />
                        <span className="font-mono font-bold text-sm text-gray-900">{u.unitCode}</span>
                      </div>
                      {u.availability === 'allocated_to_current_item' ? (
                        <span className="px-2.5 py-1 text-xs font-semibold rounded-full bg-indigo-50 text-indigo-700 border border-indigo-200">
                          Назначена этому заказу
                        </span>
                      ) : (
                        <span className="px-2.5 py-1 text-xs font-semibold rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200">
                          Свободна
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="p-4 border-t border-gray-100 bg-gray-50/80 text-xs text-gray-500 flex items-center justify-between">
              <span>Возьмите любую из этих единиц и отсканируйте её код.</span>
              <button
                type="button"
                onClick={() => setIsDrawerOpen(false)}
                className="px-4 py-2 bg-gray-900 hover:bg-gray-800 text-white font-medium rounded-lg transition-colors shrink-0 text-xs"
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

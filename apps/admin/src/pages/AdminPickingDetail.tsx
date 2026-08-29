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
} from 'lucide-react';
import {
  getAdminPickingOrder,
  scanPickingCode,
  getPickingErrorMessage,
  PickingOrder,
  PickingScanResult,
} from '../api/adminPicking';
import { formatOrderNumber } from '../utils/orderFormatters';

interface FeedbackMessage {
  type: 'success' | 'warning' | 'error';
  title: string;
  detail?: string;
}

export function AdminPickingDetail() {
  const { id } = useParams<{ id: string }>();

  const [pickingOrder, setPickingOrder] = useState<PickingOrder | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [scanInput, setScanInput] = useState('');
  const [isScanning, setIsScanning] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackMessage | null>(null);
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
    // Focus scanner input on load
    inputRef.current?.focus();
  }, [isLoading]);

  const handleScanSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = scanInput.trim();
    if (!code || isScanning || !id) return;

    setIsScanning(true);
    setFeedback(null);

    try {
      const res: PickingScanResult = await scanPickingCode(id, code);

      if (res.scanResult.newlyPicked) {
        setFeedback({
          type: 'success',
          title: 'Товар добавлен в сборку',
          detail: `Отсканирован: ${code}`,
        });
      } else if (res.scanResult.alreadyPicked) {
        setFeedback({
          type: 'warning',
          title: 'Эта единица уже собрана',
          detail: `ZMU: ${code}`,
        });
      } else if (res.scanResult.alreadyComplete) {
        setFeedback({
          type: 'warning',
          title: 'Нужное количество этого товара уже собрано',
          detail: `Штрихкод: ${code}`,
        });
      }

      // Refresh real picking order data from backend
      await fetchOrder(false);
    } catch (err: any) {
      const msg = getPickingErrorMessage(err);
      setFeedback({
        type: 'error',
        title: msg,
        detail: `Код: ${code}`,
      });
    } finally {
      setIsScanning(false);
      setScanInput('');
      setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
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
        <div className="p-5 rounded-xl bg-emerald-50 border border-emerald-200 text-emerald-900 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-sm">
          <div className="flex items-center space-x-3.5">
            <CheckCircle2 className="w-7 h-7 text-emerald-600 shrink-0" />
            <div>
              <h3 className="text-base font-bold text-gray-900">
                Сборка завершена ({pickedQuantity} / {totalQuantity})
              </h3>
              <p className="text-xs text-emerald-800 mt-0.5 font-medium">
                Все позиции заказа успешно укомплектованы на складе.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Link
              to={`/fulfillment/packing/${pickingOrder.fulfillmentId}`}
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold bg-emerald-600 hover:bg-emerald-700 text-white transition-colors shadow-sm"
            >
              <PackageCheck className="w-4 h-4" />
              Перейти к упаковке
            </Link>
          </div>
        </div>
      )}

      {/* Scanner Input Card */}
      <div className="bg-white border border-gray-200 rounded-xl p-6 shadow-sm space-y-4">
        <div className="flex items-center justify-between">
          <label htmlFor="scanner-input" className="text-sm font-semibold text-gray-900 flex items-center gap-2">
            <Scan className="w-4 h-4 text-indigo-600" />
            Отсканируйте ZMU или штрихкод
          </label>
          <span className="text-xs text-gray-500">USB / Bluetooth сканер или ручной ввод</span>
        </div>

        <form onSubmit={handleScanSubmit} className="relative">
          <input
            id="scanner-input"
            ref={inputRef}
            type="text"
            value={scanInput}
            onChange={(e) => setScanInput(e.target.value)}
            disabled={isScanning}
            placeholder="Отсканируйте ZMU или штрихкод товара и нажмите Enter..."
            autoComplete="off"
            className="w-full text-base sm:text-lg bg-gray-50 text-gray-900 placeholder-gray-400 px-4 py-3.5 pl-12 rounded-xl border border-gray-300 focus:bg-white focus:outline-none focus:border-indigo-600 focus:ring-1 focus:ring-indigo-600 transition-all font-mono disabled:opacity-50"
          />
          <Scan className="w-5 h-5 text-gray-400 absolute left-4 top-1/2 -translate-y-1/2" />
          <button
            type="submit"
            disabled={isScanning || !scanInput.trim()}
            className="absolute right-2 top-1/2 -translate-y-1/2 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-100 disabled:text-gray-400 text-white text-xs font-semibold rounded-lg transition-colors shadow-sm"
          >
            {isScanning ? <RefreshCw className="w-4 h-4 animate-spin" /> : 'Ввод'}
          </button>
        </form>

        {/* Scan Feedback Banner */}
        {feedback && (
          <div
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
                {!isSerialized && (
                  <div className="mt-3 pt-3 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
                    <div className="flex items-center gap-1.5 text-gray-600">
                      <Tag className="w-3.5 h-3.5 text-gray-400" />
                      <span>Сканируйте ZMK / штрихкод товара</span>
                    </div>
                    <div className="text-[11px] text-gray-500">
                      Собрано {item.pickedQuantity} из {item.quantity}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

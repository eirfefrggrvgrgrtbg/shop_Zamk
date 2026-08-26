import React, { useEffect, useState, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Scan,
  CheckCircle2,
  AlertCircle,
  AlertTriangle,
  Package,
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
      <div className="p-12 text-center text-slate-400">
        <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-400 mb-3" />
        <p className="text-sm font-medium">Загрузка данных заказа...</p>
      </div>
    );
  }

  if (error || !pickingOrder) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-6">
        <div className="p-6 rounded-2xl bg-rose-950/50 border border-rose-800 text-rose-300 space-y-4">
          <div className="flex items-center space-x-3">
            <AlertCircle className="w-6 h-6 text-rose-400 shrink-0" />
            <h2 className="text-lg font-bold">Ошибка загрузки заказа</h2>
          </div>
          <p className="text-sm">{error || 'Заказ не найден или недоступен для сборки.'}</p>
          <div className="flex space-x-4 pt-2">
            <button
              onClick={() => fetchOrder(true)}
              className="px-4 py-2 bg-rose-900/60 hover:bg-rose-900 rounded-xl text-sm font-semibold text-rose-200 transition-colors"
            >
              Попробовать снова
            </button>
            <Link
              to="/fulfillment/picking"
              className="px-4 py-2 bg-slate-800 hover:bg-slate-700 rounded-xl text-sm font-semibold text-slate-300 transition-colors"
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
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-slate-800 pb-5">
        <div className="space-y-1">
          <Link
            to="/fulfillment/picking"
            className="inline-flex items-center text-xs font-semibold text-slate-400 hover:text-white transition-colors mb-2"
          >
            <ArrowLeft className="w-4 h-4 mr-1.5" />
            К очереди сборок
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-bold text-white tracking-tight">
              Заказ #{pickingOrder.orderNumber || pickingOrder.orderId.slice(0, 8)}
            </h1>
            <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-bold bg-blue-950/80 text-blue-400 border border-blue-800/60">
              Сборка
            </span>
          </div>
        </div>

        <div className="flex items-center gap-4 bg-slate-900/80 border border-slate-800 rounded-2xl px-5 py-3.5">
          <div className="text-right">
            <div className="text-xs text-slate-400 font-medium">Прогресс сборки</div>
            <div className="text-xl font-bold text-white">
              {pickedQuantity} / {totalQuantity}
            </div>
          </div>
          <div className="w-16 h-16 relative flex items-center justify-center">
            <div className="text-xs font-bold text-indigo-400">{progressPercent}%</div>
          </div>
        </div>
      </div>

      {/* Completion Banner */}
      {isComplete && (
        <div className="p-5 rounded-2xl bg-emerald-950/60 border border-emerald-700/80 text-emerald-200 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-lg shadow-emerald-950/30">
          <div className="flex items-center space-x-3.5">
            <CheckCircle2 className="w-7 h-7 text-emerald-400 shrink-0" />
            <div>
              <h3 className="text-base font-bold text-white">
                Сборка завершена ({pickedQuantity} / {totalQuantity})
              </h3>
              <p className="text-xs text-emerald-300 mt-0.5 font-medium">
                Все товары заказа успешно собраны.
              </p>
            </div>
          </div>
          <div className="inline-flex items-center px-3.5 py-1.5 rounded-xl text-xs font-semibold bg-emerald-900/80 text-emerald-100 border border-emerald-700">
            Следующий этап — упаковка
          </div>
        </div>
      )}

      {/* Scanner Input Card */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-6 shadow-md space-y-4">
        <div className="flex items-center justify-between">
          <label htmlFor="scanner-input" className="text-sm font-semibold text-slate-200 flex items-center gap-2">
            <Scan className="w-4 h-4 text-indigo-400" />
            Отсканируйте ZMU или штрихкод
          </label>
          <span className="text-xs text-slate-400">USB / Bluetooth сканер или ручной ввод</span>
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
            className="w-full text-base sm:text-lg bg-slate-950 text-white placeholder-slate-500 px-4 py-3.5 pl-12 rounded-xl border border-slate-700 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all font-mono disabled:opacity-50"
          />
          <Scan className="w-5 h-5 text-slate-400 absolute left-4 top-1/2 -translate-y-1/2" />
          <button
            type="submit"
            disabled={isScanning || !scanInput.trim()}
            className="absolute right-2 top-1/2 -translate-y-1/2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:bg-slate-800 disabled:text-slate-500 text-white text-xs font-semibold rounded-lg transition-colors"
          >
            {isScanning ? <RefreshCw className="w-4 h-4 animate-spin" /> : 'Ввод'}
          </button>
        </form>

        {/* Scan Feedback Banner */}
        {feedback && (
          <div
            className={`p-4 rounded-xl border flex items-start space-x-3 transition-all ${
              feedback.type === 'success'
                ? 'bg-emerald-950/50 border-emerald-800 text-emerald-200'
                : feedback.type === 'warning'
                ? 'bg-amber-950/50 border-amber-800 text-amber-200'
                : 'bg-rose-950/50 border-rose-800 text-rose-200'
            }`}
          >
            {feedback.type === 'success' && <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />}
            {feedback.type === 'warning' && <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />}
            {feedback.type === 'error' && <AlertCircle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />}
            <div className="space-y-0.5">
              <div className="text-sm font-semibold">{feedback.title}</div>
              {feedback.detail && <div className="text-xs opacity-80 font-mono">{feedback.detail}</div>}
            </div>
          </div>
        )}
      </div>

      {/* Items Section */}
      <div className="space-y-4">
        <div className="flex items-center justify-between px-1">
          <h2 className="text-lg font-bold text-white flex items-center gap-2">
            <Package className="w-5 h-5 text-slate-400" />
            Состав заказа ({pickingOrder.items.length} {pickingOrder.items.length === 1 ? 'позиция' : 'позиций'})
          </h2>
          <span className="text-xs font-medium text-slate-400">
            Осталось собрать: <span className="text-white font-bold">{remainingQuantity} шт.</span>
          </span>
        </div>

        <div className="space-y-4">
          {pickingOrder.items.map((item) => {
            const isSerialized = item.allocationMode === 'serialized';
            const itemComplete = item.pickedQuantity === item.quantity;

            return (
              <div
                key={item.orderItemId}
                className={`p-5 rounded-2xl border transition-all ${
                  itemComplete
                    ? 'bg-slate-900/40 border-emerald-900/40'
                    : 'bg-slate-900/80 border-slate-800'
                }`}
              >
                <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                  <div className="space-y-1.5 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-base font-bold text-white">{item.title}</span>
                      {isSerialized ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-indigo-950/80 text-indigo-300 border border-indigo-800/60">
                          Маркированный (ZMU)
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-slate-800 text-slate-300 border border-slate-700">
                          Без индивидуальной маркировки
                        </span>
                      )}
                    </div>

                    <div className="text-xs text-slate-400 flex items-center gap-3">
                      <span>Количество: <strong className="text-slate-200">{item.quantity} шт.</strong></span>
                      <span>·</span>
                      <span>Собрано: <strong className={itemComplete ? 'text-emerald-400' : 'text-indigo-400'}>{item.pickedQuantity} / {item.quantity}</strong></span>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <div className="text-right">
                      <div className="text-xs font-semibold text-slate-400">
                        {itemComplete ? (
                          <span className="text-emerald-400 flex items-center gap-1 font-bold">
                            <Check className="w-4 h-4" /> Собрано полностью
                          </span>
                        ) : (
                          <span>Осталось: <strong className="text-white">{item.remainingQuantity} шт.</strong></span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Serialized ZMU Units List */}
                {isSerialized && (
                  <div className="mt-4 pt-4 border-t border-slate-800/80 space-y-2">
                    <div className="text-xs font-semibold text-slate-400">
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
                                ? 'bg-emerald-950/40 border-emerald-800/70 text-emerald-300'
                                : 'bg-slate-950 border-slate-800 text-slate-400'
                            }`}
                          >
                            <span className="font-semibold">{unit.unitCode}</span>
                            {isPicked ? (
                              <span className="inline-flex items-center gap-1 text-[11px] font-bold text-emerald-400">
                                <Check className="w-3.5 h-3.5" /> Собрана
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-[11px] text-slate-500 font-sans">
                                <Circle className="w-3 h-3" /> Ожидает
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
                  <div className="mt-3 pt-3 border-t border-slate-800/60 flex items-center justify-between text-xs text-slate-400">
                    <div className="flex items-center gap-1.5 text-slate-400">
                      <Tag className="w-3.5 h-3.5 text-slate-500" />
                      <span>Сканируйте ZMK / штрихкод товара</span>
                    </div>
                    <div className="text-[11px] text-slate-500">
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

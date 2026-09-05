import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  Package,
  PackageCheck,
  RefreshCw,
  Clock,
  Truck,
  Check,
  Tag,
  ExternalLink,
} from 'lucide-react';
import {
  getAdminPickingOrder,
  packFulfillment,
  getPackingErrorMessage,
  PickingOrder,
  PackResult,
} from '../api/adminPicking';
import { getAdminFulfillment } from '../api/adminOrders';
import { formatOrderNumber } from '../utils/orderFormatters';
import type { AdminFulfillment } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';

export function AdminPackingDetail() {
  const { id } = useParams<{ id: string }>();
  const { hasPermission } = useAdminAuth();
  const canPack = hasPermission('warehouse.packing');

  const [pickingOrder, setPickingOrder] = useState<PickingOrder | null>(null);
  const [fulfillmentData, setFulfillmentData] = useState<AdminFulfillment | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [isPacking, setIsPacking] = useState(false);
  const [packError, setPackError] = useState<string | null>(null);
  const [packResult, setPackResult] = useState<PackResult | null>(null);

  const loadData = async (showLoading = true) => {
    if (!id) return;
    try {
      if (showLoading) setIsLoading(true);
      setError(null);
      setPackError(null);

      // Attempt to load picking order details
      try {
        const po = await getAdminPickingOrder(id);
        setPickingOrder(po);
        if (po.fulfillmentStatus === 'packed') {
          // If already packed, also fetch fulfillment for packedAt metadata
          const f = await getAdminFulfillment(id).catch(() => null);
          if (f) setFulfillmentData(f);
        }
      } catch (err: any) {
        // If picking order is rejected because it's already packed, fetch fulfillment directly
        const f = await getAdminFulfillment(id);
        setFulfillmentData(f);
      }
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить данные упаковки.');
    } finally {
      if (showLoading) setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData(true);
  }, [id]);

  const handleConfirmPacking = async () => {
    if (!id || isPacking) return;

    setIsPacking(true);
    setPackError(null);

    try {
      const res = await packFulfillment(id);
      setPackResult(res);
      // Refresh fulfillment data
      const f = await getAdminFulfillment(id).catch(() => null);
      if (f) setFulfillmentData(f);
    } catch (err: any) {
      const msg = getPackingErrorMessage(err);
      setPackError(msg);
    } finally {
      setIsPacking(false);
    }
  };

  if (isLoading) {
    return (
      <div className="p-12 text-center text-gray-500 bg-white rounded-xl border border-gray-200 shadow-sm max-w-4xl mx-auto mt-6">
        <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-600 mb-3" />
        <p className="text-sm font-medium">Загрузка данных упаковки...</p>
      </div>
    );
  }

  if (error && !pickingOrder && !fulfillmentData) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-6">
        <div className="p-6 rounded-2xl bg-rose-50 border border-rose-200 text-rose-800 space-y-4 shadow-sm">
          <div className="flex items-center space-x-3">
            <AlertCircle className="w-6 h-6 text-rose-600 shrink-0" />
            <h2 className="text-lg font-bold text-gray-900">Ошибка загрузки заказа</h2>
          </div>
          <p className="text-sm text-gray-700">{error || 'Сборка не найдена или недоступна.'}</p>
          <div className="flex space-x-4 pt-2">
            <button
              onClick={() => loadData(true)}
              className="px-4 py-2 bg-rose-600 hover:bg-rose-700 rounded-xl text-sm font-semibold text-white transition-colors"
            >
              Попробовать снова
            </button>
            <Link
              to="/fulfillment/picking"
              className="px-4 py-2 bg-white hover:bg-gray-50 border border-gray-300 rounded-xl text-sm font-semibold text-gray-700 transition-colors"
            >
              К очереди сборок
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const orderId = pickingOrder?.orderId || fulfillmentData?.orderId || '';
  const orderNumber = pickingOrder?.orderNumber || fulfillmentData?.orderNumber;
  const isPacked =
    packResult?.fulfillmentStatus === 'packed' ||
    pickingOrder?.fulfillmentStatus === 'packed' ||
    fulfillmentData?.status === 'packed';

  const packedAtTimestamp =
    packResult?.packedAt ||
    fulfillmentData?.packedAt ||
    (fulfillmentData as any)?.packed_at;

  const formattedPackedAt = packedAtTimestamp
    ? new Date(packedAtTimestamp).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : null;

  const items = pickingOrder?.items || [];
  const totalQuantity = items.reduce((sum, i) => sum + i.quantity, 0);
  const pickedQuantity = items.reduce((sum, i) => sum + i.pickedQuantity, 0);
  const isPickingComplete = totalQuantity > 0 && pickedQuantity === totalQuantity;

  return (
    <div data-testid="packing-detail-page" className="max-w-5xl mx-auto space-y-6 pb-16 px-4 sm:px-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-gray-200 pb-5">
        <div className="space-y-1">
          <div className="flex items-center gap-3 mb-2 text-xs font-semibold text-gray-500">
            <Link
              to={id ? `/fulfillment/picking/${id}` : '/fulfillment/picking'}
              className="inline-flex items-center hover:text-gray-900 transition-colors"
            >
              <ArrowLeft className="w-4 h-4 mr-1.5" />
              К сборке заказа
            </Link>
            <span>·</span>
            <Link
              to="/fulfillment/picking"
              className="hover:text-gray-900 transition-colors"
            >
              К очереди сборок
            </Link>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-bold text-gray-900 tracking-tight">
              Заказ #{formatOrderNumber({ id: orderId, orderNumber })}
            </h1>
            {isPacked ? (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                <Check className="w-3.5 h-3.5" />
                Упакован
              </span>
            ) : (
              <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                Готов к упаковке
              </span>
            )}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => loadData(true)}
            disabled={isLoading || isPacking}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      {/* Status & Next Step Banner */}
      {isPacked ? (
        <div className="p-6 rounded-2xl bg-emerald-50 border border-emerald-200 text-emerald-900 space-y-3 shadow-sm">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-center space-x-3.5">
              <CheckCircle2 className="w-8 h-8 text-emerald-600 shrink-0" />
              <div>
                <h3 className="text-lg font-bold text-gray-900">Упаковка завершена</h3>
                <p className="text-xs text-emerald-800 mt-0.5 font-medium">
                  Сборка переведена в статус «Упакован».
                  {formattedPackedAt && ` Время упаковки: ${formattedPackedAt}`}
                </p>
              </div>
            </div>
            <Link
              to={`/fulfillment/dispatch/${id}`}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-indigo-600 hover:bg-indigo-700 text-white shadow-sm transition-colors shrink-0"
            >
              <Truck className="w-4 h-4" />
              Перейти к отгрузке
            </Link>
          </div>

          <div className="pt-3 border-t border-emerald-200/60 flex flex-wrap items-center gap-3 text-xs">
            <Link
              to={`/fulfillment/dispatch/${id}`}
              className="inline-flex items-center gap-1 font-semibold text-emerald-800 hover:text-emerald-950 bg-emerald-100 hover:bg-emerald-200 px-3 py-1.5 rounded-lg transition-colors"
            >
              <span>Отгрузка со склада</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>
            <Link
              to={`/orders/${orderId}`}
              className="inline-flex items-center gap-1 font-semibold text-emerald-800 hover:text-emerald-950 bg-emerald-100 hover:bg-emerald-200 px-3 py-1.5 rounded-lg transition-colors"
            >
              <span>Перейти в карточку заказа</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>
            <Link
              to="/fulfillment/picking"
              className="font-medium text-emerald-700 hover:underline"
            >
              Вернуться в очередь сборок
            </Link>
          </div>
        </div>
      ) : (
        <div className="p-5 rounded-xl bg-blue-50 border border-blue-200 text-blue-900 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm">
          <div className="flex items-center space-x-3.5">
            <CheckCircle2 className="w-7 h-7 text-blue-600 shrink-0" />
            <div>
              <h3 className="text-base font-bold text-gray-900">
                Сборка завершена ({pickedQuantity} / {totalQuantity} шт.)
              </h3>
              <p className="text-xs text-blue-800 mt-0.5 font-medium">
                Все товары укомплектованы. Проверьте физический состав и подтвердите упаковку.
              </p>
            </div>
          </div>
          <div className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-blue-100 text-blue-800 border border-blue-200">
            <Clock className="w-3.5 h-3.5" />
            Ожидает упаковки
          </div>
        </div>
      )}

      {/* Items Section */}
      <div className="bg-white border border-gray-200 rounded-xl p-6 shadow-sm space-y-4">
        <div className="flex items-center justify-between pb-3 border-b border-gray-100">
          <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
            <Package className="w-5 h-5 text-gray-500" />
            Состав сборки ({items.length} {items.length === 1 ? 'позиция' : 'позиций'})
          </h2>
          <span className="text-xs text-gray-500 font-medium">
            Всего: <strong className="text-gray-900 font-bold">{totalQuantity} шт.</strong>
          </span>
        </div>

        <div className="space-y-3">
          {items.map((item) => {
            const isSerialized = item.allocationMode === 'serialized';

            return (
              <div
                key={item.orderItemId}
                className="p-4 rounded-xl border border-gray-200 bg-gray-50/50 space-y-3"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-gray-900">{item.title}</span>
                      {isSerialized ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                          ZMU
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-gray-100 text-gray-600 border border-gray-200">
                          Штрихкод
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="text-right text-xs font-semibold text-gray-700">
                    Количество: <span className="text-gray-900 font-bold">{item.quantity} шт.</span>
                  </div>
                </div>

                {/* Serialized ZMU units display */}
                {isSerialized && item.allocatedUnits.length > 0 && (
                  <div className="pt-2 border-t border-gray-200/60">
                    <div className="text-[11px] font-semibold text-gray-500 mb-1.5">
                      Собранные единицы ZMU:
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
                      {item.allocatedUnits.map((unit) => (
                        <div
                          key={unit.inventoryUnitId}
                          className="px-3 py-1.5 rounded-lg border border-emerald-200 bg-emerald-50 text-emerald-800 flex items-center justify-between text-xs font-mono"
                        >
                          <span className="font-semibold">{unit.unitCode}</span>
                          <span className="inline-flex items-center gap-1 text-[11px] font-bold text-emerald-700">
                            <Check className="w-3 h-3" /> Собрана
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        <div className="p-3.5 rounded-xl bg-gray-50 border border-gray-200 text-xs text-gray-600 flex items-start gap-2.5">
          <Tag className="w-4 h-4 text-gray-400 shrink-0 mt-0.5" />
          <span>
            Физическая идентификация единиц установлена на этапе сборки. Повторное сканирование ZMU при упаковке не требуется.
          </span>
        </div>
      </div>

      {/* Confirmation Action Card */}
      {!isPacked && (
        <div className="bg-white border border-gray-200 rounded-xl p-6 shadow-sm space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="space-y-1">
              <h3 className="text-base font-bold text-gray-900">Подтверждение упаковки</h3>
              <p className="text-xs text-gray-500">
                После подтверждения статус сборки изменится на «Упакован». Отгрузка не начнется автоматически.
              </p>
            </div>

            <button
              onClick={handleConfirmPacking}
              disabled={!canPack || isPacking || !isPickingComplete}
              className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-200 disabled:text-gray-400 text-white text-sm font-bold rounded-xl transition-all shadow-sm shrink-0"
            >
              {isPacking ? (
                <>
                  <RefreshCw className="w-4 h-4 animate-spin" />
                  <span>Упаковка...</span>
                </>
              ) : (
                <>
                  <PackageCheck className="w-4 h-4" />
                  <span>Подтвердить упаковку</span>
                </>
              )}
            </button>
          </div>

          {/* Pack Error Banner */}
          {packError && (
            <div className="p-4 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 flex items-center gap-3 text-sm">
              <AlertCircle className="w-5 h-5 text-rose-600 shrink-0" />
              <span>{packError}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

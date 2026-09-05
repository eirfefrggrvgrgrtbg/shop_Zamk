import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  AlertTriangle,
  Package,
  Truck,
  RefreshCw,
  Clock,
  Check,
  Tag,
  ExternalLink,
  MapPin,
  User,
  Phone,
  Building2,
  X,
} from 'lucide-react';
import {
  getAdminPickingOrder,
  dispatchFulfillment,
  getDispatchErrorMessage,
  PickingOrder,
  DispatchResult,
} from '../api/adminPicking';
import { getAdminFulfillment, getAdminOrder, AdminOrderView } from '../api/adminOrders';
import { formatOrderNumber } from '../utils/orderFormatters';
import type { AdminFulfillment } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';

interface DispatchDisplayItem {
  orderItemId: string;
  title: string;
  quantity: number;
  allocationMode: 'serialized' | 'legacy';
  allocatedUnits: {
    inventoryUnitId: string;
    unitCode: string;
    pickedAt?: string | null;
  }[];
}

export function AdminDispatchDetail() {
  const { id } = useParams<{ id: string }>();
  const { hasPermission } = useAdminAuth();
  const canDispatch = hasPermission('warehouse.dispatch');

  const [pickingOrder, setPickingOrder] = useState<PickingOrder | null>(null);
  const [fulfillmentData, setFulfillmentData] = useState<AdminFulfillment | null>(null);
  const [orderData, setOrderData] = useState<AdminOrderView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [isDispatching, setIsDispatching] = useState(false);
  const [dispatchError, setDispatchError] = useState<string | null>(null);
  const [dispatchResult, setDispatchResult] = useState<DispatchResult | null>(null);
  const [showConfirmModal, setShowConfirmModal] = useState(false);

  const loadData = async (showLoading = true) => {
    if (!id) return;
    try {
      if (showLoading) setIsLoading(true);
      setError(null);
      setDispatchError(null);

      // Primary canonical read model: Fulfillment
      let f: AdminFulfillment | null = null;
      try {
        f = await getAdminFulfillment(id);
        setFulfillmentData(f);
      } catch (fErr: any) {
        console.warn('getAdminFulfillment failed:', fErr);
      }

      // Supplementary read model: Picking Order (if still in picking workflow)
      let po: PickingOrder | null = null;
      try {
        po = await getAdminPickingOrder(id);
        setPickingOrder(po);
      } catch (poErr: any) {
        // Expected to fail with 409 if already packed/shipped
      }

      const oId = f?.orderId || po?.orderId;
      if (oId) {
        const ord = await getAdminOrder(oId).catch(() => null);
        if (ord) setOrderData(ord);
      }

      if (!f && !po) {
        throw new Error('Не удалось загрузить данные сборки.');
      }
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить данные для отгрузки.');
    } finally {
      if (showLoading) setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData(true);
  }, [id]);

  const handleConfirmDispatch = async () => {
    if (!id || isDispatching) return;

    setIsDispatching(true);
    setDispatchError(null);

    try {
      const res = await dispatchFulfillment(id);
      setDispatchResult(res);
      setShowConfirmModal(false);

      // Refresh fulfillment and order data
      const f = await getAdminFulfillment(id).catch(() => null);
      if (f) setFulfillmentData(f);
      if (res.orderId) {
        const ord = await getAdminOrder(res.orderId).catch(() => null);
        if (ord) setOrderData(ord);
      }
    } catch (err: any) {
      const msg = getDispatchErrorMessage(err);
      setDispatchError(msg);
      setShowConfirmModal(false);
    } finally {
      setIsDispatching(false);
    }
  };

  if (isLoading) {
    return (
      <div className="p-12 text-center text-gray-500 bg-white rounded-xl border border-gray-200 shadow-sm max-w-4xl mx-auto mt-6">
        <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-600 mb-3" />
        <p className="text-sm font-medium">Загрузка данных отгрузки...</p>
      </div>
    );
  }

  if (error && !pickingOrder && !fulfillmentData) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-6">
        <div className="p-6 rounded-2xl bg-rose-50 border border-rose-200 text-rose-800 space-y-4 shadow-sm">
          <div className="flex items-center space-x-3">
            <AlertCircle className="w-6 h-6 text-rose-600 shrink-0" />
            <h2 className="text-lg font-bold text-gray-900">Ошибка загрузки данных</h2>
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
              to="/shipments"
              className="px-4 py-2 bg-white hover:bg-gray-50 border border-gray-300 rounded-xl text-sm font-semibold text-gray-700 transition-colors"
            >
              К очереди отгрузок
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const orderId = fulfillmentData?.orderId || pickingOrder?.orderId || orderData?.id || '';
  const orderNumber = fulfillmentData?.orderNumber || pickingOrder?.orderNumber || orderData?.orderNumber;

  const currentFulfillmentStatus =
    dispatchResult?.fulfillmentStatus ||
    fulfillmentData?.status ||
    pickingOrder?.fulfillmentStatus ||
    '';

  const isShipped = currentFulfillmentStatus === 'shipped';
  const isPacked = currentFulfillmentStatus === 'packed' || (!isShipped && Boolean(fulfillmentData?.packedAt));

  const shippedAtTimestamp =
    dispatchResult?.shippedAt ||
    (fulfillmentData as any)?.shippedAt ||
    (fulfillmentData as any)?.shipped_at;

  const formattedShippedAt = shippedAtTimestamp
    ? new Date(shippedAtTimestamp).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : null;

  const packedAtTimestamp = fulfillmentData?.packedAt;
  const formattedPackedAt = packedAtTimestamp
    ? new Date(packedAtTimestamp).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : null;

  const items: DispatchDisplayItem[] = (
    fulfillmentData?.items && fulfillmentData.items.length > 0
      ? fulfillmentData.items.map((i) => ({
          orderItemId: i.orderItemId,
          title: i.productTitle || (i as any).title || 'Товар',
          quantity: i.quantity,
          allocationMode:
            (i.allocationMode as 'serialized' | 'legacy') ||
            (i.allocatedUnits && i.allocatedUnits.length > 0 ? 'serialized' : 'legacy'),
          allocatedUnits: (i.allocatedUnits || []).map((u) => ({
            inventoryUnitId: u.inventoryUnitId,
            unitCode: u.unitCode,
            pickedAt: u.pickedAt,
          })),
        }))
      : pickingOrder?.items && pickingOrder.items.length > 0
      ? pickingOrder.items.map((i) => ({
          orderItemId: i.orderItemId,
          title: i.title,
          quantity: i.quantity,
          allocationMode: (i.allocationMode as 'serialized' | 'legacy') || 'legacy',
          allocatedUnits: (i.allocatedUnits || []).map((u) => ({
            inventoryUnitId: u.inventoryUnitId,
            unitCode: u.unitCode,
            pickedAt: u.pickedAt,
          })),
        }))
      : []
  );

  const totalQuantity = items.reduce((sum, i) => sum + i.quantity, 0);

  const deliveryAddress =
    fulfillmentData?.deliveryAddress ||
    orderData?.deliveryAddress ||
    'Самовывоз / Не указан';

  const customerName =
    fulfillmentData?.customerName ||
    orderData?.customerName ||
    '—';

  const customerPhone =
    fulfillmentData?.customerPhone ||
    orderData?.customerPhone ||
    null;

  const customerEmail =
    orderData?.customerEmail ||
    null;

  const deliveryMethodName =
    orderData?.deliveryMethodName ||
    null;

  return (
    <div data-testid="dispatch-detail-page" className="max-w-5xl mx-auto space-y-6 pb-16 px-4 sm:px-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b border-gray-200 pb-5">
        <div className="space-y-1">
          <div className="flex items-center gap-3 mb-2 text-xs font-semibold text-gray-500">
            <Link
              to="/shipments"
              className="inline-flex items-center gap-1.5 hover:text-gray-900 transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Очередь отгрузок</span>
            </Link>
            <span>/</span>
            <Link
              to={`/fulfillment/packing/${id}`}
              className="hover:text-gray-900 transition-colors"
            >
              Упаковка
            </Link>
            <span>/</span>
            <span className="text-gray-900 font-bold">Отгрузка со склада</span>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-2xl font-black text-gray-900 tracking-tight">
              Отгрузка заказа #{formatOrderNumber({ id: orderId, orderNumber })}
            </h1>
            {isShipped ? (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                <Check className="w-3.5 h-3.5" />
                Отгружен
              </span>
            ) : isPacked ? (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                <Package className="w-3.5 h-3.5" />
                Упакован (готов к отгрузке)
              </span>
            ) : (
              <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-gray-100 text-gray-700 border border-gray-200">
                {currentFulfillmentStatus || 'Не готов к отгрузке'}
              </span>
            )}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => loadData(true)}
            disabled={isLoading || isDispatching}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      {/* Status & Result Banner */}
      {isShipped ? (
        <div className="p-6 rounded-2xl bg-emerald-50 border border-emerald-200 text-emerald-900 space-y-4 shadow-sm">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-center space-x-3.5">
              <CheckCircle2 className="w-9 h-9 text-emerald-600 shrink-0" />
              <div>
                <h3 className="text-lg font-bold text-gray-900">Отгрузка со склада выполнена</h3>
                <p className="text-xs text-emerald-800 mt-0.5 font-medium">
                  Сборка переведена в статус «Отгружен». Складские остатки списаны, товары переданы в доставку.
                  {formattedShippedAt && ` Время отгрузки: ${formattedShippedAt}.`}
                </p>
                {dispatchResult?.shipmentId && (
                  <p className="text-xs text-emerald-700 mt-1 font-mono">
                    ID отгрузки: {dispatchResult.shipmentId}
                  </p>
                )}
              </div>
            </div>
            <div className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-white text-emerald-800 border border-emerald-300 shadow-sm shrink-0">
              <Truck className="w-4 h-4 text-emerald-600" />
              Следующий этап — Доставка
            </div>
          </div>

          <div className="pt-3 border-t border-emerald-200/60 flex flex-wrap items-center gap-3 text-xs">
            <Link
              to={`/orders/${orderId}`}
              className="inline-flex items-center gap-1 font-semibold text-emerald-800 hover:text-emerald-950 bg-emerald-100 hover:bg-emerald-200 px-3 py-1.5 rounded-lg transition-colors"
            >
              <span>Карточка заказа</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>
            <Link
              to="/shipments"
              className="inline-flex items-center gap-1 font-semibold text-emerald-800 hover:text-emerald-950 bg-emerald-100 hover:bg-emerald-200 px-3 py-1.5 rounded-lg transition-colors"
            >
              <span>Журнал отгрузок</span>
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
      ) : isPacked ? (
        <div className="p-5 rounded-xl bg-blue-50 border border-blue-200 text-blue-900 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm">
          <div className="flex items-center space-x-3.5">
            <CheckCircle2 className="w-7 h-7 text-blue-600 shrink-0" />
            <div>
              <h3 className="text-base font-bold text-gray-900">
                Заказ упакован и готов к физической отгрузке
              </h3>
              <p className="text-xs text-blue-800 mt-0.5 font-medium">
                Физическая идентификация завершена. Проверьте параметры доставки и подтвердите передачу курьеру.
                {formattedPackedAt && ` Время упаковки: ${formattedPackedAt}.`}
              </p>
            </div>
          </div>
          <div className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-blue-100 text-blue-800 border border-blue-200 shrink-0">
            <Clock className="w-3.5 h-3.5" />
            Ожидает отгрузки
          </div>
        </div>
      ) : (
        <div className="p-5 rounded-xl bg-amber-50 border border-amber-200 text-amber-900 flex items-center gap-3 text-sm">
          <AlertCircle className="w-5 h-5 text-amber-600 shrink-0" />
          <span>
            Сборка находится в статусе «{currentFulfillmentStatus || 'Не определен'}». Для выполнения отгрузки сборка должна быть предварительно упакована.
          </span>
        </div>
      )}

      {/* Delivery & Customer Info Card */}
      <div className="bg-white border border-gray-200 rounded-xl p-5 shadow-sm space-y-4">
        <h3 className="text-sm font-bold text-gray-900 flex items-center gap-2">
          <Truck className="w-4 h-4 text-indigo-600" />
          Параметры доставки и получатель
        </h3>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
          <div className="space-y-1.5 p-3 rounded-lg bg-gray-50 border border-gray-100">
            <div className="text-gray-500 font-medium flex items-center gap-1.5">
              <MapPin className="w-3.5 h-3.5 text-gray-400" />
              Адрес доставки
            </div>
            <div className="font-semibold text-gray-900 break-words">
              {deliveryAddress}
            </div>
            {deliveryMethodName && (
              <div className="text-gray-500 text-[11px]">
                Способ: {deliveryMethodName}
              </div>
            )}
          </div>

          <div className="space-y-1.5 p-3 rounded-lg bg-gray-50 border border-gray-100">
            <div className="text-gray-500 font-medium flex items-center gap-1.5">
              <User className="w-3.5 h-3.5 text-gray-400" />
              Получатель
            </div>
            <div className="font-semibold text-gray-900">
              {customerName}
            </div>
            {(customerPhone || customerEmail) && (
              <div className="text-gray-500 text-[11px] flex flex-wrap items-center gap-2">
                {customerPhone && (
                  <span className="flex items-center gap-1">
                    <Phone className="w-3 h-3 text-gray-400" />
                    {customerPhone}
                  </span>
                )}
                {customerEmail && <span>{customerEmail}</span>}
              </div>
            )}
          </div>

          <div className="space-y-1.5 p-3 rounded-lg bg-gray-50 border border-gray-100">
            <div className="text-gray-500 font-medium flex items-center gap-1.5">
              <Building2 className="w-3.5 h-3.5 text-gray-400" />
              Продавец / Склад
            </div>
            <div className="font-semibold text-gray-900">
              {fulfillmentData?.sellerName || 'Склад ZAMK'}
            </div>
            <div className="text-gray-500 text-[11px]">
              Идентификатор сборки: <span className="font-mono">{id?.substring(0, 8)}...</span>
            </div>
          </div>
        </div>
      </div>

      {/* Items Section */}
      <div className="bg-white border border-gray-200 rounded-xl p-6 shadow-sm space-y-4">
        <div className="flex items-center justify-between border-b border-gray-100 pb-3">
          <h2 className="text-base font-bold text-gray-900 flex items-center gap-2">
            <Package className="w-5 h-5 text-indigo-600" />
            Состав отгрузки ({items.length} поз., {totalQuantity} шт.)
          </h2>
          <span className="text-xs font-semibold text-emerald-700 bg-emerald-50 border border-emerald-200 px-2.5 py-1 rounded-lg">
            Упаковка завершена
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
                  <div className="space-y-0.5">
                    <div className="text-sm font-bold text-gray-900">{item.title}</div>
                    <div className="text-xs text-gray-500">
                      Тип учета: <span className="font-medium text-gray-700">{isSerialized ? 'Сериализованный (ZMU)' : 'Штучный (Legacy)'}</span>
                    </div>
                  </div>
                  <div className="text-xs font-semibold text-gray-700 bg-white px-3 py-1.5 rounded-lg border border-gray-200 shrink-0">
                    Количество: <span className="text-gray-900 font-bold">{item.quantity} шт.</span>
                  </div>
                </div>

                {/* Serialized ZMU units display */}
                {isSerialized && item.allocatedUnits.length > 0 && (
                  <div className="pt-2 border-t border-gray-200/60">
                    <div className="text-[11px] font-semibold text-gray-500 mb-1.5">
                      Идентифицированные единицы ZMU:
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
                      {item.allocatedUnits.map((unit) => (
                        <div
                          key={unit.inventoryUnitId}
                          className="px-3 py-1.5 rounded-lg border border-emerald-200 bg-emerald-50 text-emerald-800 flex items-center justify-between text-xs font-mono"
                        >
                          <span className="font-semibold">{unit.unitCode}</span>
                          <span className="inline-flex items-center gap-1 text-[11px] font-bold text-emerald-700">
                            <Check className="w-3 h-3" /> {isShipped ? 'Отгружена' : 'Готова'}
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
            Физическая идентификация завершена на этапах сборки и упаковки. Повторное сканирование при отгрузке не требуется.
          </span>
        </div>
      </div>

      {/* Confirmation Action Card (when not yet shipped) */}
      {!isShipped && (
        <div className="bg-white border border-amber-200 rounded-xl p-6 shadow-sm space-y-4 bg-gradient-to-br from-white to-amber-50/30">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-6 h-6 text-amber-600 shrink-0 mt-0.5" />
            <div className="space-y-1">
              <h3 className="text-base font-bold text-gray-900">Подтверждение физической отгрузки</h3>
              <p className="text-xs text-gray-600 leading-relaxed">
                При подтверждении товары <strong>физически покидают склад ZAMK</strong>. Будут списаны складские остатки (total_stock, reserved_stock), и единицы перейдут в статус «Отгружен». Отменить действие невозможно.
              </p>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pt-2 border-t border-amber-200/60">
            <div className="text-xs text-gray-500">
              Действие доступно администраторам склада с правами управления статусами заказов.
            </div>

            <button
              onClick={() => setShowConfirmModal(true)}
              disabled={!canDispatch || isDispatching || !isPacked}
              className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-amber-600 hover:bg-amber-700 disabled:bg-gray-200 disabled:text-gray-400 text-white text-sm font-bold rounded-xl transition-all shadow-sm shrink-0"
            >
              <Truck className="w-4 h-4" />
              <span>Подтвердить отгрузку</span>
            </button>
          </div>

          {/* Dispatch Error Banner */}
          {dispatchError && (
            <div className="p-4 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 flex items-center gap-3 text-sm">
              <AlertCircle className="w-5 h-5 text-rose-600 shrink-0" />
              <span>{dispatchError}</span>
            </div>
          )}
        </div>
      )}

      {/* Confirmation Modal */}
      {showConfirmModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-2xl space-y-5 border border-gray-100">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-amber-100 text-amber-700 flex items-center justify-center shrink-0">
                  <Truck className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="text-lg font-bold text-gray-900">Подтвердить отгрузку?</h3>
                  <p className="text-xs text-gray-500">Заказ #{formatOrderNumber({ id: orderId, orderNumber })}</p>
                </div>
              </div>
              <button
                onClick={() => setShowConfirmModal(false)}
                className="text-gray-400 hover:text-gray-600 p-1 rounded-lg transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-4 rounded-xl bg-amber-50 border border-amber-200 text-xs text-amber-900 space-y-2">
              <p className="font-bold">Вы подтверждаете передачу товаров со склада ZAMK в доставку:</p>
              <ul className="list-disc list-inside space-y-1 text-amber-800">
                <li>Складские остатки будут окончательно списаны</li>
                <li>Статус сборки и отгрузки изменится на «Отгружен»</li>
                <li>Товары будут считаться находящимися в пути к покупателю</li>
              </ul>
            </div>

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                onClick={() => setShowConfirmModal(false)}
                disabled={isDispatching}
                className="px-4 py-2.5 rounded-xl border border-gray-300 text-sm font-semibold text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                onClick={handleConfirmDispatch}
                disabled={isDispatching}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-amber-600 hover:bg-amber-700 text-sm font-bold text-white transition-colors shadow-sm disabled:opacity-50"
              >
                {isDispatching ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    <span>Отгрузка...</span>
                  </>
                ) : (
                  <>
                    <Truck className="w-4 h-4" />
                    <span>Да, подтвердить отгрузку</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

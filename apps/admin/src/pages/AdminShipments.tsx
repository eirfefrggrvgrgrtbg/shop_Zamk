import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertCircle,
  Truck,
  Package,
  Clock,
  ExternalLink,
  RefreshCw,
  CheckCircle2,
  ChevronRight,
} from 'lucide-react';
import {
  getAdminShipment,
  getAdminShipmentErrorMessage,
  getAdminShipments,
  getGenericEditableShipmentStatuses,
  getShipmentStatusLabel,
  updateAdminShipmentStatus,
} from '../api/adminShipments';
import type { AdminShipmentView } from '../api/adminShipments';
import { getAdminFulfillments } from '../api/adminOrders';
import type { AdminFulfillment } from '@zamk/api-client/src/types';
import { formatOrderNumber } from '../utils/orderFormatters';
import { PermissionGuard } from '../components/PermissionGuard';

export function AdminShipments() {
  const [packedFulfillments, setPackedFulfillments] = useState<AdminFulfillment[]>([]);
  const [shipments, setShipments] = useState<AdminShipmentView[]>([]);
  const [selectedShipment, setSelectedShipment] = useState<AdminShipmentView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [carrier, setCarrier] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingUrl, setTrackingUrl] = useState('');
  const [statusDraft, setStatusDraft] = useState('');
  const [comment, setComment] = useState('');

  const fetchData = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const [fulfillmentsData, shipmentsData] = await Promise.all([
        getAdminFulfillments({ status: 'packed' }),
        getAdminShipments(),
      ]);
      setPackedFulfillments(fulfillmentsData);
      setShipments(shipmentsData);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось загрузить данные отгрузок.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchShipmentDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      const shipment = await getAdminShipment(id);
      setSelectedShipment(shipment);
      setStatusDraft(shipment.status);
      setCarrier(shipment.carrier || '');
      setTrackingNumber(shipment.trackingNumber || '');
      setTrackingUrl(shipment.trackingUrl || '');
      setComment('');
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось загрузить детали отгрузки.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleUpdateShipment = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedShipment || !statusDraft) return;

    try {
      setIsSubmitting(true);
      setError(null);
      await updateAdminShipmentStatus(selectedShipment.id, {
        status: statusDraft,
        carrier,
        trackingNumber,
        trackingUrl,
        comment,
      });
      await fetchData();
      await fetchShipmentDetail(selectedShipment.id);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось обновить отгрузку.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'delivered':
        return 'bg-green-100 text-green-800 border-green-200';
      case 'shipped':
        return 'bg-emerald-100 text-emerald-800 border-emerald-200';
      case 'failed':
      case 'cancelled':
        return 'bg-red-100 text-red-800 border-red-200';
      case 'pending':
      case 'assembling':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      default:
        return 'bg-blue-100 text-blue-800 border-blue-200';
    }
  };

  const formatDate = (value?: string | null) =>
    value
      ? new Date(value).toLocaleString('ru-RU', {
          day: '2-digit',
          month: '2-digit',
          year: 'numeric',
          hour: '2-digit',
          minute: '2-digit',
        })
      : '—';

  return (
    <div className="space-y-8 pb-16">
      {/* Page Header */}
      <div className="sm:flex sm:items-center sm:justify-between border-b border-gray-200 pb-5">
        <div>
          <h1 className="text-2xl font-black text-gray-900 tracking-tight">Доставка / Отгрузки</h1>
          <p className="mt-1 text-sm text-gray-500">
            Очередь физической отгрузки упакованных заказов со склада ZAMK и журнал отправлений
          </p>
        </div>
        <button
          onClick={fetchData}
          disabled={isLoading}
          className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 rounded-xl text-xs font-semibold bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
          Обновить данные
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 rounded-xl flex items-center shadow-sm">
          <AlertCircle className="h-5 w-5 mr-2 shrink-0 text-red-600" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* SECTION 1: ОЖИДАЮТ ОТГРУЗКИ (PACKED FULFILLMENTS QUEUE) */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-amber-100 text-amber-700 flex items-center justify-center">
              <Truck className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                Ожидают отгрузки
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-bold bg-amber-100 text-amber-800">
                  {packedFulfillments.length}
                </span>
              </h2>
              <p className="text-xs text-gray-500">
                Упакованные заказы, готовые к передаче курьеру и физическому списанию со склада
              </p>
            </div>
          </div>
        </div>

        {isLoading ? (
          <div className="p-8 text-center bg-white rounded-2xl border border-gray-200 shadow-sm">
            <RefreshCw className="w-6 h-6 animate-spin mx-auto text-indigo-600 mb-2" />
            <p className="text-xs text-gray-500 font-medium">Загрузка очереди сборок...</p>
          </div>
        ) : packedFulfillments.length === 0 ? (
          <div className="p-6 text-center bg-white rounded-2xl border border-dashed border-gray-200 shadow-sm">
            <CheckCircle2 className="w-8 h-8 text-emerald-500 mx-auto mb-2" />
            <h3 className="text-sm font-bold text-gray-900">Все упакованные заказы отгружены</h3>
            <p className="text-xs text-gray-500 mt-0.5">
              В данный момент нет сборок, ожидающих передачи в доставку.
            </p>
          </div>
        ) : (
          <div className="bg-white shadow-sm border border-amber-200/80 rounded-2xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-amber-50/50">
                  <tr>
                    <th className="px-6 py-3.5 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Заказ / Сборка
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Продавец / Склад
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Состав заказа
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Получатель / Адрес
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Упакован
                    </th>
                    <th className="px-6 py-3.5 text-right text-xs font-bold text-gray-700 uppercase tracking-wider">
                      Действие
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-100">
                  {packedFulfillments.map((f) => {
                    const totalQty = (f.items || []).reduce((sum, it) => sum + it.quantity, 0);
                    const itemsSummary = f.items && f.items.length > 0
                      ? `${f.items.length} поз., ${totalQty} шт.`
                      : '—';

                    return (
                      <tr key={f.id} className="hover:bg-amber-50/20 transition-colors">
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm font-bold text-gray-900">
                            #{formatOrderNumber({ id: f.orderId, orderNumber: f.orderNumber })}
                          </div>
                          <div className="text-[11px] text-gray-400 font-mono mt-0.5">
                            Сборка: {f.id.substring(0, 8)}...
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-700 font-medium">
                          {f.sellerName || 'Склад ZAMK'}
                        </td>
                        <td className="px-6 py-4 text-xs text-gray-900">
                          <div className="font-semibold">{itemsSummary}</div>
                          {f.items && f.items[0] && (
                            <div className="text-[11px] text-gray-500 truncate max-w-xs mt-0.5">
                              {f.items[0].productTitle}
                              {f.items.length > 1 && ` + еще ${f.items.length - 1}`}
                            </div>
                          )}
                        </td>
                        <td className="px-6 py-4 text-xs text-gray-700 max-w-xs">
                          <div className="font-medium text-gray-900 truncate">
                            {f.customerName || '—'}
                          </div>
                          <div className="text-[11px] text-gray-500 truncate mt-0.5">
                            {f.deliveryAddress || 'Самовывоз / Не указан'}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-600">
                          <div className="flex items-center gap-1.5">
                            <Clock className="w-3.5 h-3.5 text-gray-400" />
                            <span>{formatDate(f.packedAt || (f as any).packed_at)}</span>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-xs">
                          <Link
                            to={`/fulfillment/dispatch/${f.id}`}
                            className="inline-flex items-center gap-1.5 px-4 py-2 bg-amber-600 hover:bg-amber-700 text-white font-bold rounded-xl transition-all shadow-sm"
                          >
                            <Truck className="w-3.5 h-3.5" />
                            <span>Перейти к отгрузке</span>
                            <ChevronRight className="w-3.5 h-3.5" />
                          </Link>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {/* SECTION 2: ЖУРНАЛ И ИСТОРИЯ ОТГРУЗОК */}
      <div className="space-y-4 pt-4 border-t border-gray-200">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-gray-100 text-gray-700 flex items-center justify-center">
              <Package className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-gray-900 flex items-center gap-2">
                История и журнал отгрузок
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-700">
                  {shipments.length}
                </span>
              </h2>
              <p className="text-xs text-gray-500">
                Записи об отправлениях, созданные после подтверждения физической отгрузки
              </p>
            </div>
          </div>
        </div>

        {isLoading ? (
          <div className="text-center py-10">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
            <p className="mt-2 text-xs text-gray-500">Загрузка журнала отгрузок...</p>
          </div>
        ) : shipments.length === 0 ? (
          <div className="text-center py-10 bg-white rounded-2xl border border-gray-200 shadow-sm">
            <Truck className="mx-auto h-10 w-10 text-gray-400" />
            <h3 className="mt-2 text-sm font-bold text-gray-900">Отгрузок пока нет</h3>
            <p className="mt-1 text-xs text-gray-500">
              Записи отгрузок формируются автоматически при подтверждении физической отгрузки сборок.
            </p>
          </div>
        ) : (
          <div className="bg-white shadow-sm border border-gray-200 rounded-2xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      ID отгрузки
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Основание
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Статус
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Служба / Трекинг
                    </th>
                    <th className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Дата отправки
                    </th>
                    <th className="px-6 py-3.5 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Действия
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-100">
                  {shipments.map((shipment) => (
                    <tr key={shipment.id} className="hover:bg-gray-50/60 transition-colors">
                      <td className="px-6 py-4 whitespace-nowrap text-xs font-mono font-medium text-gray-900">
                        {shipment.id.substring(0, 8)}...
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-500">
                        {shipment.fulfillmentId ? (
                          <>
                            <span className="font-semibold text-gray-900">
                              Сборка: {shipment.fulfillmentId.substring(0, 8)}...
                            </span>
                            <br />
                            <span className="text-[11px] text-gray-400">
                              Заказ: {shipment.orderId.substring(0, 8)}...
                            </span>
                          </>
                        ) : (
                          <>
                            <span className="bg-yellow-100 text-yellow-800 text-[10px] px-1.5 py-0.5 rounded font-medium">
                              Старая отгрузка заказа
                            </span>
                            <br />
                            <span className="text-[11px] text-gray-400">
                              Заказ: {shipment.orderId.substring(0, 8)}...
                            </span>
                          </>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border ${getStatusBadgeClass(
                            shipment.status
                          )}`}
                        >
                          {shipment.statusLabel}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-700">
                        {[shipment.carrier, shipment.trackingNumber].filter(Boolean).join(' / ') || '—'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-500">
                        {formatDate(shipment.shippedAt)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-xs font-medium space-x-3">
                        {shipment.fulfillmentId && (
                          <Link
                            to={`/fulfillment/dispatch/${shipment.fulfillmentId}`}
                            className="inline-flex items-center gap-1 text-indigo-600 hover:text-indigo-900 font-semibold"
                          >
                            <span>Карточка отгрузки</span>
                            <ExternalLink className="w-3 h-3" />
                          </Link>
                        )}
                        <button
                          onClick={() => fetchShipmentDetail(shipment.id)}
                          className="text-gray-600 hover:text-gray-900 font-medium"
                        >
                          Детали / Трекинг
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {/* SECTION 3: ДЕТАЛИ И РЕДАКТИРОВАНИЕ ВЫБРАННОЙ ОТГРУЗКИ */}
      {selectedShipment && (
        <div className="bg-white shadow-sm border border-gray-200 rounded-2xl p-6 space-y-5">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-base font-bold text-gray-900">Детали отгрузки</h2>
              <p className="mt-0.5 text-xs text-gray-500 font-mono">{selectedShipment.id}</p>
            </div>
            {isDetailLoading && (
              <span className="text-xs text-gray-500 flex items-center gap-1">
                <RefreshCw className="w-3.5 h-3.5 animate-spin" /> Загрузка...
              </span>
            )}
          </div>

          {selectedShipment.fulfillmentId && (
            <div className="p-4 rounded-xl bg-blue-50 border border-blue-200 text-blue-900 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
              <div>
                <strong className="block text-sm font-bold text-gray-900">
                  {selectedShipment.status === 'shipped'
                    ? 'Сборка отгружена со склада'
                    : 'Физическая отгрузка сборки'}
                </strong>
                <span>
                  {selectedShipment.status === 'shipped'
                    ? 'Сборка переведена в статус «Отгружен» и передана в доставку.'
                    : 'Списание складских остатков и физическая передача товаров выполняются через процесс отгрузки сборки.'}
                </span>
              </div>
              <Link
                to={`/fulfillment/dispatch/${selectedShipment.fulfillmentId}`}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl font-bold shrink-0 transition-colors shadow-sm"
              >
                <Truck className="w-3.5 h-3.5" />
                <span>
                  {selectedShipment.status === 'shipped' ? 'Карточка отгрузки' : 'Перейти к отгрузке'}
                </span>
              </Link>
            </div>
          )}

          <dl className="grid gap-4 md:grid-cols-4 p-4 rounded-xl bg-gray-50 text-xs">
            <div>
              <dt className="font-semibold text-gray-500">Основание</dt>
              <dd className="mt-1 text-gray-900">
                {selectedShipment.fulfillmentId ? (
                  <>
                    Сборка: <span className="font-mono">{selectedShipment.fulfillmentId.substring(0, 8)}...</span>
                    <br />
                    <span className="text-[11px] text-gray-500">
                      Заказ: {selectedShipment.orderId.substring(0, 8)}...
                    </span>
                  </>
                ) : (
                  <>
                    <span className="text-yellow-700 font-medium">Старая отгрузка</span>
                    <br />
                    <span className="text-[11px] text-gray-500">
                      Заказ: {selectedShipment.orderId.substring(0, 8)}...
                    </span>
                  </>
                )}
              </dd>
            </div>
            <div>
              <dt className="font-semibold text-gray-500">Отправлен</dt>
              <dd className="mt-1 text-gray-900">{formatDate(selectedShipment.shippedAt)}</dd>
            </div>
            <div>
              <dt className="font-semibold text-gray-500">Доставлен</dt>
              <dd className="mt-1 text-gray-900">{formatDate(selectedShipment.deliveredAt)}</dd>
            </div>
            <div>
              <dt className="font-semibold text-gray-500">Обновлён</dt>
              <dd className="mt-1 text-gray-900">{formatDate(selectedShipment.updatedAt)}</dd>
            </div>
          </dl>

          <PermissionGuard
            permission="shipments.update_status"
            fallback={
              <p className="text-xs text-gray-500">
                У вас нет прав для изменения параметров отгрузки.
              </p>
            }
          >
            <form onSubmit={handleUpdateShipment} className="grid gap-4 md:grid-cols-2 pt-2 border-t border-gray-100">
              <div className="space-y-1">
                <label className="block text-xs font-bold text-gray-700">Статус отгрузки</label>
                <select
                  required
                  disabled={selectedShipment.status === 'shipped' || selectedShipment.status === 'delivered'}
                  value={statusDraft}
                  onChange={(event) => setStatusDraft(event.target.value)}
                  className="w-full rounded-xl border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 disabled:bg-gray-100 disabled:text-gray-500 text-xs py-2"
                >
                  {getGenericEditableShipmentStatuses(selectedShipment.status).map((status) => (
                    <option key={status} value={status}>
                      {getShipmentStatusLabel(status)}
                    </option>
                  ))}
                </select>
                {(selectedShipment.status === 'shipped' || selectedShipment.status === 'delivered') && (
                  <p className="text-[11px] text-gray-500">
                    Статус зафиксирован физическим складским процессом.
                  </p>
                )}
              </div>
              <div className="space-y-1">
                <label className="block text-xs font-bold text-gray-700">Служба доставки</label>
                <input
                  value={carrier}
                  onChange={(event) => setCarrier(event.target.value)}
                  placeholder="Служба доставки"
                  className="w-full rounded-xl border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-xs py-2"
                />
              </div>
              <div className="space-y-1">
                <label className="block text-xs font-bold text-gray-700">Номер отслеживания</label>
                <input
                  value={trackingNumber}
                  onChange={(event) => setTrackingNumber(event.target.value)}
                  placeholder="Номер отслеживания"
                  className="w-full rounded-xl border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-xs py-2"
                />
              </div>
              <div className="space-y-1">
                <label className="block text-xs font-bold text-gray-700">Ссылка для отслеживания</label>
                <input
                  value={trackingUrl}
                  onChange={(event) => setTrackingUrl(event.target.value)}
                  placeholder="Ссылка для отслеживания"
                  className="w-full rounded-xl border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-xs py-2"
                />
              </div>
              <div className="md:col-span-2 space-y-1">
                <label className="block text-xs font-bold text-gray-700">Комментарий</label>
                <textarea
                  rows={2}
                  value={comment}
                  onChange={(event) => setComment(event.target.value)}
                  placeholder="Комментарий (необязательно)"
                  className="w-full rounded-xl border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-xs"
                />
              </div>
              <div className="md:col-span-2">
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="rounded-xl bg-indigo-600 px-5 py-2.5 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors shadow-sm"
                >
                  {isSubmitting ? 'Сохранение...' : 'Обновить данные отгрузки'}
                </button>
              </div>
            </form>
          </PermissionGuard>
        </div>
      )}
    </div>
  );
}

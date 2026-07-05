import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, ShoppingCart, X } from 'lucide-react';
import {
  getAdminOrder,
  getAdminOrderErrorMessage,
  getAdminOrders,
  getAllowedOrderStatusTargets,
  getOrderStatusLabel,
  updateAdminOrderStatus,
  getAdminOrderFulfillments,
} from '../api/adminOrders';
import type { AdminOrderView } from '../api/adminOrders';
import type { AdminFulfillment } from '@zamk/api-client/src/types';
import {
  createAdminShipment,
  updateAdminOrderFulfillmentStatus,
  createAdminFulfillmentShipment,
  getAdminShipmentErrorMessage,
} from '../api/adminShipments';
import { PermissionGuard } from '../components/PermissionGuard';
import { HelpTooltip } from '../components/HelpTooltip';

export function AdminOrders() {
  const navigate = useNavigate();
  const [orders, setOrders] = useState<AdminOrderView[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<AdminOrderView | null>(null);
  const [fulfillments, setFulfillments] = useState<AdminFulfillment[]>([]);
  const [isPanelOpen, setIsPanelOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isFulfillmentsLoading, setIsFulfillmentsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fulfillmentsError, setFulfillmentsError] = useState<string | null>(null);
  const [statusDraft, setStatusDraft] = useState('');
  const [fulfillmentStatusDraft, setFulfillmentStatusDraft] = useState('');
  const [fulfillmentStatusReason, setFulfillmentStatusReason] = useState('');
  const [statusComment, setStatusComment] = useState('');
  const [shipmentCarrier, setShipmentCarrier] = useState('');
  const [shipmentTrackingNumber, setShipmentTrackingNumber] = useState('');
  const [shipmentTrackingUrl, setShipmentTrackingUrl] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const [totalCount, setTotalCount] = useState(0);
  const [page, setPage] = useState(1);
  const [limit] = useState(20);
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [fulfillmentStatusFilter, setFulfillmentStatusFilter] = useState('');

  const fulfillmentStatusLabels: Record<string, string> = {
    pending: 'Ожидает',
    awaiting_payment: 'Ожидает оплаты',
    paid: 'Оплачен',
    assembling: 'Собирается',
    packed: 'Упакован',
    shipped: 'Отправлен',
    delivered: 'Доставлен',
    cancelled: 'Отменён',
    returned: 'Возврат',
    refunded: 'Возмещён'
  };

  const shipmentStatusLabels: Record<string, string> = {
    pending: 'Ожидает подготовки',
    assembling: 'Собирается',
    packed: 'Упакован',
    shipped: 'Отправлен',
    delivered: 'Доставлен',
    cancelled: 'Отменён'
  };

  const fetchOrders = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminOrders({
        limit,
        offset: (page - 1) * limit,
        q: debouncedQuery,
        status: statusFilter,
        sourceType: sourceFilter,
        fulfillmentStatus: fulfillmentStatusFilter,
      });
      setOrders(data.items);
      setTotalCount(data.totalCount);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось загрузить заказы.'));
    } finally {
      setIsLoading(false);
    }
  };

  const openOrderPanel = async (id: string) => {
    setIsPanelOpen(true);
    setSelectedOrder(null);
    setFulfillments([]);
    setIsDetailLoading(true);
    setIsFulfillmentsLoading(true);
    setError(null);
    setFulfillmentsError(null);
    setStatusDraft('');
    setStatusComment('');
    try {
      const data = await getAdminOrder(id);
      setSelectedOrder(data);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось загрузить детали заказа.'));
    } finally {
      setIsDetailLoading(false);
    }
    
    try {
      const fData = await getAdminOrderFulfillments(id);
      setFulfillments(fData);
    } catch (err: unknown) {
      setFulfillmentsError('Не удалось загрузить сборки продавцов.');
    } finally {
      setIsFulfillmentsLoading(false);
    }
  };

  const closePanel = () => {
    setIsPanelOpen(false);
    setSelectedOrder(null);
  };

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(searchQuery);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  useEffect(() => {
    fetchOrders();
  }, [page, debouncedQuery, statusFilter, sourceFilter, fulfillmentStatusFilter]);

  const handleStatusUpdate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedOrder || !statusDraft) return;
    if (statusDraft === 'paid') {
      setError('Статус «Оплачен» устанавливается только платёжным вебхуком. Ручное изменение недоступно.');
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      await updateAdminOrderStatus(selectedOrder.id, {
        status: statusDraft,
        comment: statusComment || undefined,
      });
      await fetchOrders();
      await openOrderPanel(selectedOrder.id);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось обновить статус заказа.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  
  const handleFulfillmentStatusUpdate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedOrder || !fulfillmentStatusDraft) return;

    try {
      setIsSubmitting(true);
      setError(null);
      await updateAdminOrderFulfillmentStatus(selectedOrder.id, {
        status: fulfillmentStatusDraft,
        reason: fulfillmentStatusReason || undefined,
      });
      await fetchOrders();
      await openOrderPanel(selectedOrder.id);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось обновить статус сборки.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCreateShipment = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedOrder) return;

    try {
      setIsSubmitting(true);
      setError(null);
      await createAdminShipment({
        orderId: selectedOrder.id,
        carrier: shipmentCarrier,
        trackingNumber: shipmentTrackingNumber,
        trackingUrl: shipmentTrackingUrl,
      });
      setShipmentCarrier('');
      setShipmentTrackingNumber('');
      setShipmentTrackingUrl('');
      await fetchOrders();
      await openOrderPanel(selectedOrder.id);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось создать отгрузку.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCreateFulfillmentShipment = async (fulfillmentId: string) => {
    try {
      setIsSubmitting(true);
      setFulfillmentsError(null);
      await createAdminFulfillmentShipment(fulfillmentId, {});
      await fetchOrders();
      if (selectedOrder) {
        await openOrderPanel(selectedOrder.id);
      }
    } catch (err: unknown) {
      setFulfillmentsError(getAdminShipmentErrorMessage(err, 'Не удалось создать отгрузку для сборки.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'paid':
      case 'delivered':
        return 'bg-green-100 text-green-800';
      case 'cancelled':
      case 'failed':
        return 'bg-red-100 text-red-800';
      case 'awaiting_payment':
        return 'bg-yellow-100 text-yellow-800';
      default:
        return 'bg-blue-100 text-blue-800';
    }
  };

  const formatDate = (value?: string) =>
    value ? new Date(value).toLocaleDateString('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' }) : '—';
  const formatMoney = (amount: number, currency: string) =>
    `${amount.toFixed(2)} ${currency === 'RUB' ? '₽' : currency}`;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Заказы</h1>
      </div>

      <div className="flex flex-col sm:flex-row gap-4 bg-white p-4 rounded-lg shadow-sm">
        <div className="flex-1">
          <input
            type="text"
            placeholder="Поиск по ID, имени, email..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
          />
        </div>
        <div className="w-full sm:w-48">
          <select
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
            className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
          >
            <option value="">Все статусы</option>
            <option value="awaiting_payment">Ожидает оплаты</option>
            <option value="paid">Оплачен</option>
            <option value="cancelled">Отменён</option>
          </select>
        </div>
        <div className="w-full sm:w-48">
          <select
            value={fulfillmentStatusFilter}
            onChange={(e) => { setFulfillmentStatusFilter(e.target.value); setPage(1); }}
            className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
          >
            <option value="">Все сборки</option>
            <option value="pending">Ожидает</option>
            <option value="assembling">Собирается</option>
            <option value="packed">Упакован</option>
            <option value="shipped">Отгружен</option>
            <option value="delivered">Доставлен</option>
          </select>
        </div>
        <div className="w-full sm:w-48">
          <select
            value={sourceFilter}
            onChange={(e) => { setSourceFilter(e.target.value); setPage(1); }}
            className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
          >
            <option value="">Все типы</option>
            <option value="normal">Обычные</option>
            <option value="auction">Аукционы</option>
            <option value="direct_sale">ZAMK</option>
          </select>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-2 text-sm text-gray-500">Загрузка заказов...</p>
        </div>
      ) : orders.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ShoppingCart className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Заказов нет</h3>
          <p className="mt-1 text-sm text-gray-500">Заказы покупателей пока отсутствуют.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID заказа</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Покупатель</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Дата</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сумма</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Тип</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сборка</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {orders.map((order) => (
                      <tr key={order.id} className="hover:bg-gray-50 cursor-pointer" onClick={() => openOrderPanel(order.id)}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                          {order.id.substring(0, 8)}…
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {order.customerName || '—'}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {formatDate(order.createdAt)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                          {formatMoney(order.totalAmount, order.currency)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {order.sourceType === 'auction' && <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">Аукцион</span>}
                          {order.sourceType === 'direct_sale' && <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">ZAMK</span>}
                          {order.sourceType === 'normal' && <span className="text-gray-500">Обычный</span>}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(order.fulfillmentStatus)}`}>
                            {fulfillmentStatusLabels[order.fulfillmentStatus] || order.fulfillmentStatus}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(order.status)}`}>
                            {order.statusLabel}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button
                            onClick={(e) => { e.stopPropagation(); openOrderPanel(order.id); }}
                            className="text-indigo-600 hover:text-indigo-900"
                          >
                            Открыть
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
          
          {totalCount > limit && (
            <div className="mt-4 flex items-center justify-between bg-white px-4 py-3 sm:px-6 rounded-lg shadow-sm border border-gray-200">
              <div className="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
                <div>
                  <p className="text-sm text-gray-700">
                    Показано от <span className="font-medium">{(page - 1) * limit + 1}</span> до <span className="font-medium">{Math.min(page * limit, totalCount)}</span> из <span className="font-medium">{totalCount}</span> заказов
                  </p>
                </div>
                <div>
                  <nav className="isolate inline-flex -space-x-px rounded-md shadow-sm" aria-label="Pagination">
                    <button
                      onClick={() => setPage(p => Math.max(1, p - 1))}
                      disabled={page === 1}
                      className="relative inline-flex items-center rounded-l-md px-2 py-2 text-gray-400 ring-1 ring-inset ring-gray-300 hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:opacity-50"
                    >
                      Назад
                    </button>
                    <button
                      onClick={() => setPage(p => p + 1)}
                      disabled={page * limit >= totalCount}
                      className="relative inline-flex items-center rounded-r-md px-2 py-2 text-gray-400 ring-1 ring-inset ring-gray-300 hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:opacity-50"
                    >
                      Вперёд
                    </button>
                  </nav>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Side panel overlay */}
      {isPanelOpen && (
        <div className="fixed inset-0 z-50 flex">
          {/* Overlay */}
          <div className="fixed inset-0 bg-black bg-opacity-30" onClick={closePanel} />
          {/* Panel */}
          <div className="fixed right-0 top-0 h-full w-full max-w-xl bg-white shadow-xl overflow-y-auto flex flex-col">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 shrink-0">
              <h2 className="text-lg font-semibold text-gray-900">
                {selectedOrder ? `Заказ № ${selectedOrder.id.substring(0, 8)}` : 'Загрузка...'}
              </h2>
              <button onClick={closePanel} className="text-gray-400 hover:text-gray-600">
                <X className="h-6 w-6" />
              </button>
            </div>

            <div className="flex-1 px-6 py-4 space-y-6">
              {isDetailLoading ? (
                <div className="text-center py-10">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
                  <p className="mt-2 text-sm text-gray-500">Загрузка...</p>
                </div>
              ) : selectedOrder ? (
                <>
                  {error && (
                    <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center text-sm">
                      <AlertCircle className="h-5 w-5 mr-2 shrink-0" />
                      {error}
                    </div>
                  )}

                  {/* Status */}
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-xs font-medium text-gray-500 uppercase">Статус</p>
                      <span className={`mt-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(selectedOrder.status)}`}>
                        {selectedOrder.statusLabel}
                      </span>
                    </div>
                    <div>
                      <p className="text-xs font-medium text-gray-500 uppercase">Сборка</p>
                      <span className={`mt-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(selectedOrder.fulfillmentStatus)}`}>
                        {fulfillmentStatusLabels[selectedOrder.fulfillmentStatus] || selectedOrder.fulfillmentStatus}
                      </span>
                    </div>
                    <div>
                      <p className="text-xs font-medium text-gray-500 uppercase">Тип заказа</p>
                      <p className="mt-1 text-sm text-gray-900">
                        {selectedOrder.sourceType === 'auction' ? 'Аукцион' : selectedOrder.sourceType === 'direct_sale' ? 'ZAMK (Прямая продажа)' : 'Обычный'}
                      </p>
                    </div>
                    <div>
                      <p className="text-xs font-medium text-gray-500 uppercase">Создан</p>
                      <p className="mt-1 text-sm text-gray-900">{formatDate(selectedOrder.createdAt)}</p>
                    </div>
                  </div>

                  {/* Customer */}
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase mb-2">Покупатель</p>
                    <dl className="grid grid-cols-1 gap-1 text-sm">
                      <div className="flex"><dt className="w-24 text-gray-500 shrink-0">Имя:</dt><dd className="text-gray-900">{selectedOrder.customerName || 'Не указано'}</dd></div>
                      <div className="flex"><dt className="w-24 text-gray-500 shrink-0">Email:</dt><dd className="text-gray-900">{selectedOrder.customerEmail || 'Не указано'}</dd></div>
                      <div className="flex"><dt className="w-24 text-gray-500 shrink-0">Телефон:</dt><dd className="text-gray-900">{selectedOrder.customerPhone || 'Не указано'}</dd></div>
                    </dl>
                  </div>

                  {/* Delivery */}
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase mb-1">Адрес доставки</p>
                    <p className="text-sm text-gray-900">{selectedOrder.deliveryAddress || 'Не указано'}</p>
                  </div>

                  {/* Items */}
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase mb-2">Товары</p>
                    {selectedOrder.items.length === 0 ? (
                      <p className="text-sm text-gray-500">Нет данных</p>
                    ) : (
                      <div className="overflow-hidden border border-gray-200 rounded-lg">
                        <table className="min-w-full divide-y divide-gray-200 text-sm">
                          <thead className="bg-gray-50">
                            <tr>
                              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500">Товар</th>
                              <th className="px-3 py-2 text-center text-xs font-medium text-gray-500">Кол-во</th>
                              <th className="px-3 py-2 text-right text-xs font-medium text-gray-500">Сумма</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-gray-200 bg-white">
                            {selectedOrder.items.map((item) => (
                              <tr key={item.id}>
                                <td className="px-3 py-2 text-gray-900">{item.title}</td>
                                <td className="px-3 py-2 text-center text-gray-500">×{item.quantity}</td>
                                <td className="px-3 py-2 text-right text-gray-900">
                                  {(item.subtotalPriceCents / 100).toFixed(2)} ₽
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                    <div className="mt-2 text-right text-sm font-semibold text-gray-900">
                      Итого: {(selectedOrder.totalPriceCents / 100).toFixed(2)} ₽
                    </div>
                  </div>

                  {/* Fulfillments */}
                  <div className="border-t pt-4">
                    <p className="text-xs font-medium text-gray-500 uppercase mb-3">Сборки по продавцам</p>
                    {isFulfillmentsLoading ? (
                      <p className="text-sm text-gray-500">Загружаем сборки продавцов...</p>
                    ) : fulfillmentsError ? (
                      <p className="text-sm text-red-600">{fulfillmentsError}</p>
                    ) : fulfillments.length === 0 ? (
                      <p className="text-sm text-gray-500">Сборки по продавцам не найдены.</p>
                    ) : (
                      <div className="space-y-4">
                        {fulfillments.map(f => (
                          <div key={f.id} className="border border-gray-200 rounded-lg p-4 bg-gray-50">
                            <div className="flex justify-between items-start mb-2">
                              <div>
                                <p className="text-sm font-medium text-gray-900">{f.sellerName || 'Неизвестный продавец'}</p>
                                <p className="text-xs text-gray-500">Сборка #{f.id.substring(0, 8)}</p>
                              </div>
                              <div className="text-right">
                                <p className="text-xs font-medium bg-blue-100 text-blue-800 px-2 py-0.5 rounded">
                                  {fulfillmentStatusLabels[f.status] || f.status}
                                </p>
                                {f.shipmentStatus ? (
                                  <p className="mt-1 text-xs text-gray-600">
                                    Доставка: {shipmentStatusLabels[f.shipmentStatus] || f.shipmentStatus}
                                  </p>
                                ) : (
                                  ['paid', 'assembling', 'packed', 'shipped'].includes(selectedOrder.status) && f.status !== 'cancelled' && (
                                    <PermissionGuard
                                      permission="shipments.create"
                                      fallback={null}
                                    >
                                      <button
                                        onClick={(e) => { e.preventDefault(); handleCreateFulfillmentShipment(f.id); }}
                                        disabled={isSubmitting}
                                        className="mt-1 text-xs text-indigo-600 hover:text-indigo-800 font-medium"
                                      >
                                        Создать отгрузку
                                      </button>
                                    </PermissionGuard>
                                  )
                                )}
                              </div>
                            </div>
                            
                            <div className="mt-3">
                              <table className="min-w-full divide-y divide-gray-200 text-xs">
                                <thead className="bg-white">
                                  <tr>
                                    <th className="px-2 py-1 text-left font-medium text-gray-500">Товар</th>
                                    <th className="px-2 py-1 text-center font-medium text-gray-500">Кол-во</th>
                                    <th className="px-2 py-1 text-right font-medium text-gray-500">Сумма</th>
                                  </tr>
                                </thead>
                                <tbody className="divide-y divide-gray-200 bg-white">
                                  {f.items.map(item => (
                                    <tr key={item.orderItemId}>
                                      <td className="px-2 py-1 text-gray-900">{item.productTitle} {item.sku ? `(${item.sku})` : ''}</td>
                                      <td className="px-2 py-1 text-center text-gray-500">×{item.quantity}</td>
                                      <td className="px-2 py-1 text-right text-gray-900">{(item.lineTotalCents / 100).toFixed(2)} ₽</td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                            
                            <div className="mt-2 text-right space-y-1">
                              <p className="text-xs text-gray-500">
                                Подытог: {(f.subtotalCents / 100).toFixed(2)} ₽
                              </p>
                              {f.commissionBps > 0 && (
                                <p className="text-xs text-gray-500">
                                  Комиссия ({f.commissionBps / 100}%): {((f.subtotalCents - f.sellerAmountCents) / 100).toFixed(2)} ₽
                                </p>
                              )}
                              <p className="text-xs font-medium text-gray-900">
                                Выручка продавца: {(f.sellerAmountCents / 100).toFixed(2)} ₽
                              </p>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Status update */}
                  {getAllowedOrderStatusTargets(selectedOrder.status).length > 0 && (
                    <PermissionGuard
                      permission="orders.update_status"
                      fallback={<p className="text-xs text-gray-400 border-t pt-4">У вас нет прав для изменения статуса заказа.</p>}
                    >
                      <form onSubmit={handleStatusUpdate} className="space-y-3 border-t pt-4">
                        <p className="text-xs font-medium text-gray-500 uppercase">Изменить статус</p>
                        <select
                          required
                          value={statusDraft}
                          onChange={(e) => setStatusDraft(e.target.value)}
                          className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                        >
                          <option value="">Выберите статус</option>
                          {getAllowedOrderStatusTargets(selectedOrder.status).map((status) => (
                            <option key={status} value={status}>{getOrderStatusLabel(status)}</option>
                          ))}
                        </select>
                        <textarea
                          rows={2}
                          value={statusComment}
                          onChange={(e) => setStatusComment(e.target.value)}
                          placeholder="Комментарий (необязательно)"
                          className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                        />
                        <p className="text-xs text-gray-500">Статус «Оплачен» устанавливается только платёжным вебхуком и не отображается здесь.</p>
                        <button
                          type="submit"
                          disabled={isSubmitting || !statusDraft}
                          className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                        >
                          {isSubmitting ? 'Обновление...' : 'Обновить статус'}
                        </button>
                      </form>
                    </PermissionGuard>
                  )}

                  
                  
                  {/* Fulfillment Status update */}
                  <PermissionGuard
                    permission="orders.update_status"
                    fallback={<p className="text-xs text-gray-400 border-t pt-4">У вас нет прав для изменения статуса сборки.</p>}
                  >
                    <form onSubmit={handleFulfillmentStatusUpdate} className="space-y-3 border-t pt-4">
                      <p className="text-xs font-medium text-gray-500 uppercase flex items-center gap-1">
                        Изменить статус сборки
                        <HelpTooltip content="Показывает, на каком этапе находится подготовка заказа." />
                      </p>
                      <select
                        required
                        value={fulfillmentStatusDraft}
                        onChange={(e) => setFulfillmentStatusDraft(e.target.value)}
                        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                      >
                        <option value="">Выберите статус сборки</option>
                        {Object.entries(fulfillmentStatusLabels).map(([val, label]) => (
                          <option key={val} value={val}>{label}</option>
                        ))}
                      </select>
                      <textarea
                        rows={2}
                        value={fulfillmentStatusReason}
                        onChange={(e) => setFulfillmentStatusReason(e.target.value)}
                        placeholder="Причина (необязательно)"
                        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                      />
                      <button
                        type="submit"
                        disabled={isSubmitting || !fulfillmentStatusDraft}
                        className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                      >
                        {isSubmitting ? 'Обновление...' : 'Обновить сборку'}
                      </button>
                    </form>
                  </PermissionGuard>

                  {/* Create shipment */}
                  {selectedOrder.status === 'paid' && fulfillments.length === 0 && (
                    <PermissionGuard
                      permission="shipments.create"
                      fallback={<p className="text-xs text-gray-400 border-t pt-4">У вас нет прав для создания отгрузки.</p>}
                    >
                      <form onSubmit={handleCreateShipment} className="space-y-3 border-t pt-4">
                        <p className="text-xs font-medium text-gray-500 uppercase flex items-center gap-1">
                          Создать отгрузку
                          <HelpTooltip content="Посылка, созданная для доставки заказа." />
                        </p>
                        <input
                          value={shipmentCarrier}
                          onChange={(e) => setShipmentCarrier(e.target.value)}
                          placeholder="Служба доставки (опционально)"
                          className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                        />
                        <input
                          required
                          value={shipmentTrackingNumber}
                          onChange={(e) => setShipmentTrackingNumber(e.target.value)}
                          placeholder="Трек-номер"
                          className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                        />
                        <input
                          value={shipmentTrackingUrl}
                          onChange={(e) => setShipmentTrackingUrl(e.target.value)}
                          placeholder="Ссылка для отслеживания (опционально)"
                          className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm"
                        />
                        <button
                          type="submit"
                          disabled={isSubmitting || !shipmentTrackingNumber}
                          className="rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50"
                        >
                          {isSubmitting ? 'Создание...' : 'Создать отгрузку'}
                        </button>
                      </form>
                    </PermissionGuard>
                  )}

                
                  {/* Navigate to shipments */}
                  <div className="border-t pt-4">
                    <button
                      onClick={() => navigate('/shipments')}
                      className="text-sm text-indigo-600 hover:text-indigo-800"
                    >
                      Перейти в раздел «Доставка / Отгрузки» →
                    </button>
                    <p className="mt-1 text-xs text-gray-400">В следующей версии создание отгрузки будет встроено прямо здесь.</p>
                  </div>

                </>
              ) : null}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

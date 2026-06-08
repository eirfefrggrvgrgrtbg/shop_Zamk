import { useEffect, useState } from 'react';
import { AlertCircle, ShoppingCart } from 'lucide-react';
import {
  getAdminOrder,
  getAdminOrderErrorMessage,
  getAdminOrders,
  getAllowedOrderStatusTargets,
  getOrderStatusLabel,
  updateAdminOrderStatus,
} from '../api/adminOrders';
import type { AdminOrderView } from '../api/adminOrders';
import {
  createAdminShipment,
  getAdminShipmentErrorMessage,
} from '../api/adminShipments';

export function AdminOrders() {
  const [orders, setOrders] = useState<AdminOrderView[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<AdminOrderView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusDraft, setStatusDraft] = useState('');
  const [statusComment, setStatusComment] = useState('');
  const [shipmentCarrier, setShipmentCarrier] = useState('');
  const [shipmentTrackingNumber, setShipmentTrackingNumber] = useState('');
  const [shipmentTrackingUrl, setShipmentTrackingUrl] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const fetchOrders = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminOrders();
      setOrders(data);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось загрузить заказы.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchOrderDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      const data = await getAdminOrder(id);
      setSelectedOrder(data);
      setStatusDraft('');
      setStatusComment('');
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось загрузить детали заказа.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchOrders();
  }, []);

  const handleStatusUpdate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedOrder || !statusDraft) return;
    if (statusDraft === 'paid') {
      setError('Admin cannot manually set paid status. Payment webhook owns that transition.');
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
      await fetchOrderDetail(selectedOrder.id);
    } catch (err: unknown) {
      setError(getAdminOrderErrorMessage(err, 'Не удалось обновить статус заказа.'));
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
      await fetchOrderDetail(selectedOrder.id);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось создать отгрузку.'));
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

  const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString('ru-RU') : '-';
  const formatMoney = (amount: number, currency: string) => `${amount.toFixed(2)} ${currency}`;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Заказы</h1>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Loading orders...</p>
        </div>
      ) : orders.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ShoppingCart className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No orders</h3>
          <p className="mt-1 text-sm text-gray-500">No customer orders are available yet.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Order ID</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Customer</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Date</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Total</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status / Payment</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {orders.map((order) => (
                    <tr key={order.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {order.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {order.customerName}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(order.createdAt)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {formatMoney(order.totalAmount, order.currency)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex flex-col space-y-1 items-start">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(order.status)}`}>
                            {order.statusLabel}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <button onClick={() => fetchOrderDetail(order.id)} className="text-indigo-600 hover:text-indigo-900 mr-4">View</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
      )}

      {selectedOrder && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Order details</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedOrder.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Loading...</span>}
          </div>

          <div className="mt-4 grid gap-4 md:grid-cols-3">
            <div>
              <h3 className="text-sm font-medium text-gray-700">Customer</h3>
              <p className="mt-1 text-sm text-gray-900">{selectedOrder.customerName || '-'}</p>
              <p className="text-sm text-gray-500">{selectedOrder.customerEmail || '-'}</p>
              <p className="text-sm text-gray-500">{selectedOrder.customerPhone || '-'}</p>
            </div>
            <div>
              <h3 className="text-sm font-medium text-gray-700">Delivery address</h3>
              <p className="mt-1 text-sm text-gray-900">{selectedOrder.deliveryAddress || '-'}</p>
            </div>
            <div>
              <h3 className="text-sm font-medium text-gray-700">Статус</h3>
              <span className={`mt-1 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(selectedOrder.status)}`}>
                {selectedOrder.statusLabel}
              </span>
            </div>
          </div>

          <div className="mt-6">
            <h3 className="text-sm font-medium text-gray-700">Items</h3>
            {selectedOrder.items.length === 0 ? (
              <p className="mt-2 text-sm text-gray-500">No items returned for this order.</p>
            ) : (
              <div className="mt-2 overflow-hidden border border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <tbody className="divide-y divide-gray-200 bg-white">
                    {selectedOrder.items.map((item) => (
                      <tr key={item.id}>
                        <td className="px-4 py-2 text-sm text-gray-900">{item.title}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{[item.variantSize, item.variantColor, item.sku].filter(Boolean).join(' / ') || '-'}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">x{item.quantity}</td>
                        <td className="px-4 py-2 text-sm text-gray-900">{(item.subtotalPriceCents / 100).toFixed(2)} {selectedOrder.currency}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="mt-6 grid gap-6 md:grid-cols-2">
            <form onSubmit={handleStatusUpdate} className="space-y-4">
              <h3 className="text-sm font-medium text-gray-700">Operational status update</h3>
              <select required value={statusDraft} onChange={(event) => setStatusDraft(event.target.value)} className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                <option value="">Select next status</option>
                {getAllowedOrderStatusTargets(selectedOrder.status).map((status) => (
                  <option key={status} value={status}>{getOrderStatusLabel(status)}</option>
                ))}
              </select>
              <textarea rows={2} value={statusComment} onChange={(event) => setStatusComment(event.target.value)} placeholder="Комментарий, необязательно" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting || !statusDraft} className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
                Update status
              </button>
              <p className="text-xs text-gray-500">Paid status is never offered here; payment webhook owns that transition.</p>
            </form>

            <form onSubmit={handleCreateShipment} className="space-y-4">
              <h3 className="text-sm font-medium text-gray-700">Create shipment</h3>
              <input value={shipmentCarrier} onChange={(event) => setShipmentCarrier(event.target.value)} placeholder="Перевозчик" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <input value={shipmentTrackingNumber} onChange={(event) => setShipmentTrackingNumber(event.target.value)} placeholder="Трек-номер" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <input value={shipmentTrackingUrl} onChange={(event) => setShipmentTrackingUrl(event.target.value)} placeholder="Ссылка отслеживания" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting || selectedOrder.status !== 'paid'} className="rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50">
                Create shipment
              </button>
              <p className="text-xs text-gray-500">Backend only allows shipment creation for paid orders.</p>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

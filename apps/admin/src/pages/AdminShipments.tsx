import { useEffect, useState } from 'react';
import { AlertCircle, Truck } from 'lucide-react';
import {
  createAdminShipment,
  getAdminShipment,
  getAdminShipmentErrorMessage,
  getAdminShipments,
  getShipmentStatuses,
  getShipmentStatusLabel,
  updateAdminShipmentStatus,
} from '../api/adminShipments';
import type { AdminShipmentView } from '../api/adminShipments';
import { PermissionGuard } from '../components/PermissionGuard';

export function AdminShipments() {
  const [shipments, setShipments] = useState<AdminShipmentView[]>([]);
  const [selectedShipment, setSelectedShipment] = useState<AdminShipmentView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createOrderId, setCreateOrderId] = useState('');
  const [carrier, setCarrier] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingUrl, setTrackingUrl] = useState('');
  const [statusDraft, setStatusDraft] = useState('');
  const [comment, setComment] = useState('');

  const fetchShipments = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setShipments(await getAdminShipments());
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось загрузить отгрузки.'));
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
    fetchShipments();
  }, []);

  const handleCreateShipment = async (event: React.FormEvent) => {
    event.preventDefault();
    try {
      setIsSubmitting(true);
      setError(null);
      const shipment = await createAdminShipment({
        orderId: createOrderId,
        carrier,
        trackingNumber,
        trackingUrl,
      });
      setCreateOrderId('');
      setCarrier('');
      setTrackingNumber('');
      setTrackingUrl('');
      await fetchShipments();
      await fetchShipmentDetail(shipment.id);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось создать отгрузку.'));
    } finally {
      setIsSubmitting(false);
    }
  };

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
      await fetchShipments();
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
        return 'bg-green-100 text-green-800';
      case 'failed':
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      case 'pending':
        return 'bg-yellow-100 text-yellow-800';
      default:
        return 'bg-blue-100 text-blue-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Доставка / Отгрузки</h1>
          <p className="mt-1 text-sm text-gray-500">Отслеживание статуса доставки по каждому заказу</p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}

      <div className="bg-amber-50 border-l-4 border-amber-400 p-4">
        <p className="text-sm text-amber-800">
          Создание отгрузки вручную — временное решение. В следующей версии отгрузки будут создаваться из детали заказа.
        </p>
      </div>

      <PermissionGuard permission="shipments.create">
        <div className="bg-white shadow sm:rounded-lg p-6">
          <h2 className="text-lg font-medium text-gray-900">Создать отгрузку вручную</h2>
          <p className="mt-1 text-sm text-gray-500">Только ручной ввод перевозчика и трекинга. Бэкенд проверяет, что заказ оплачен и подходит для отправки.</p>
          <form onSubmit={handleCreateShipment} className="mt-4 grid gap-4 md:grid-cols-4">
            <input required value={createOrderId} onChange={(event) => setCreateOrderId(event.target.value)} placeholder="ID заказа" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
            <input value={carrier} onChange={(event) => setCarrier(event.target.value)} placeholder="Служба доставки" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
            <input value={trackingNumber} onChange={(event) => setTrackingNumber(event.target.value)} placeholder="Номер отслеживания" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
            <button type="submit" disabled={isSubmitting} className="rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50">
              Создать
            </button>
            <input value={trackingUrl} onChange={(event) => setTrackingUrl(event.target.value)} placeholder="Ссылка для отслеживания" className="md:col-span-3 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
          </form>
        </div>
      </PermissionGuard>

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Загрузка отгрузок...</p>
        </div>
      ) : shipments.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Truck className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Отгрузок нет</h3>
          <p className="mt-1 text-sm text-gray-500">Отгрузки появятся после первых оплаченных и отправленных заказов.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID отгрузки</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Заказ</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Трекинг</th>
                      <th className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {shipments.map((shipment) => (
                      <tr key={shipment.id}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{shipment.id}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{shipment.orderId}</td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(shipment.status)}`}>
                            {shipment.statusLabel}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{[shipment.carrier, shipment.trackingNumber].filter(Boolean).join(' / ') || '-'}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button onClick={() => fetchShipmentDetail(shipment.id)} className="text-indigo-600 hover:text-indigo-900">Детали / Изменить</button>
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

      {selectedShipment && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Детали отгрузки</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedShipment.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Загрузка...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Заказ</dt><dd className="mt-1 text-sm text-gray-900">{selectedShipment.orderId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Отправлен</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedShipment.shippedAt)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Доставлен</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedShipment.deliveredAt)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Обновлён</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedShipment.updatedAt)}</dd></div>
          </dl>

          <PermissionGuard
            permission="shipments.update_status"
            fallback={<p className="mt-6 text-sm text-gray-500">У вас нет прав для изменения статуса отгрузки.</p>}
          >
            <form onSubmit={handleUpdateShipment} className="mt-6 grid gap-4 md:grid-cols-2">
              <select required value={statusDraft} onChange={(event) => setStatusDraft(event.target.value)} className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                {getShipmentStatuses().map((status) => (
                  <option key={status} value={status}>{getShipmentStatusLabel(status)}</option>
                ))}
              </select>
              <input value={carrier} onChange={(event) => setCarrier(event.target.value)} placeholder="Служба доставки" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <input value={trackingNumber} onChange={(event) => setTrackingNumber(event.target.value)} placeholder="Номер отслеживания" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <input value={trackingUrl} onChange={(event) => setTrackingUrl(event.target.value)} placeholder="Ссылка для отслеживания" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <textarea rows={2} value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Комментарий (необязательно)" className="md:col-span-2 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting} className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
                Обновить отгрузку
              </button>
            </form>
          </PermissionGuard>
        </div>
      )}
    </div>
  );
}

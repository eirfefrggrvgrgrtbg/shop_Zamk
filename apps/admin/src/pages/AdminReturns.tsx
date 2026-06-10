import { useEffect, useState } from 'react';
import { AlertCircle, RotateCcw } from 'lucide-react';
import {
  createAdminRefund,
  getAdminReturn,
  getAdminReturnErrorMessage,
  getAdminReturns,
  getReturnStatusLabel,
  getReturnStatusTargets,
  updateAdminReturnStatus,
} from '../api/adminReturns';
import type { AdminReturnView } from '../api/adminReturns';

export function AdminReturns() {
  const [returns, setReturns] = useState<AdminReturnView[]>([]);
  const [selectedReturn, setSelectedReturn] = useState<AdminReturnView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusDraft, setStatusDraft] = useState('');
  const [adminComment, setAdminComment] = useState('');
  const [refundReason, setRefundReason] = useState('');
  const [success, setSuccess] = useState<string | null>(null);

  const fetchReturns = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setReturns(await getAdminReturns());
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить возвраты.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchReturnDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      const detail = await getAdminReturn(id);
      setSelectedReturn(detail);
      setStatusDraft('');
      setAdminComment('');
      setRefundReason('');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить детали возврата.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchReturns();
  }, []);

  const handleStatusUpdate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedReturn || !statusDraft) return;

    try {
      setIsSubmitting(true);
      setError(null);
      setSuccess(null);
      await updateAdminReturnStatus(selectedReturn.id, {
        status: statusDraft,
        adminComment: adminComment || undefined,
      });
      await fetchReturns();
      await fetchReturnDetail(selectedReturn.id);
      setSuccess('Статус возврата обновлён.');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось обновить статус возврата.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCreateRefund = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedReturn) return;
    if (!window.confirm('Создать возмещение для этого возврата? Бэкенд рассчитает сумму.')) return;

    try {
      setIsSubmitting(true);
      setError(null);
      setSuccess(null);
      await createAdminRefund(selectedReturn.id, refundReason);
      await fetchReturns();
      await fetchReturnDetail(selectedReturn.id);
      setSuccess('Возмещение создано бэкендом.');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось создать возмещение.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'completed':
      case 'refunded':
        return 'bg-green-100 text-green-800';
      case 'approved':
      case 'item_received':
        return 'bg-blue-100 text-blue-800';
      case 'rejected':
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Возвраты</h1>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}
      {success && <div className="p-4 bg-green-50 text-green-700 rounded-md">{success}</div>}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Загрузка возвратов...</p>
        </div>
      ) : returns.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <RotateCcw className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Возвратов нет</h3>
          <p className="mt-1 text-sm text-gray-500">Заявок на возврат пока нет.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID возврата</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID заказа</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Покупатель</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Причина</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {returns.map((req) => (
                    <tr key={req.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {req.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {req.orderId}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {req.customerName || req.userId || '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {req.reason}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(req.status)}`}>
                          {req.statusLabel}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <button onClick={() => fetchReturnDetail(req.id)} className="text-indigo-600 hover:text-indigo-900">Открыть</button>
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

      {selectedReturn && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Детали возврата</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedReturn.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Загрузка...</span>}
          </div>

          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Заказ</dt><dd className="mt-1 text-sm text-gray-900">{selectedReturn.orderId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Статус</dt><dd className="mt-1 text-sm text-gray-900">{selectedReturn.statusLabel}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Причина</dt><dd className="mt-1 text-sm text-gray-900">{selectedReturn.reason || '—'}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Создан</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedReturn.createdAt)}</dd></div>
          </dl>

          <div className="mt-6">
            <h3 className="text-sm font-medium text-gray-700">Позиции</h3>
            {selectedReturn.items.length === 0 ? (
              <p className="mt-2 text-sm text-gray-500">Нет данных</p>
            ) : (
              <div className="mt-2 overflow-hidden border border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <tbody className="divide-y divide-gray-200 bg-white">
                    {selectedReturn.items.map((item) => (
                      <tr key={item.id}>
                        <td className="px-4 py-2 text-sm text-gray-900">{item.productTitle || item.orderItemId}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">x{item.quantity}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{item.reason || '-'}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{item.condition || '-'}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">Возврат на склад: {item.restock ? 'да' : 'нет'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="mt-6 grid gap-6 md:grid-cols-2">
            <form onSubmit={handleStatusUpdate} className="space-y-4">
              <h3 className="text-sm font-medium text-gray-700">Изменить статус возврата</h3>
              <select required value={statusDraft} onChange={(event) => setStatusDraft(event.target.value)} className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                <option value="">Выберите статус</option>
                {getReturnStatusTargets(selectedReturn.status).map((status) => (
                  <option key={status} value={status}>{getReturnStatusLabel(status)}</option>
                ))}
              </select>
              <textarea rows={3} required={statusDraft === 'rejected'} value={adminComment} onChange={(event) => setAdminComment(event.target.value)} placeholder="Комментарий администратора / причина отклонения" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting || !statusDraft} className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
                Обновить статус
              </button>
            </form>

            <form onSubmit={handleCreateRefund} className="space-y-4">
              <h3 className="text-sm font-medium text-gray-700">Создать возмещение</h3>
              <p className="text-xs text-gray-500">Сумма возмещения рассчитывается бэкендом на основе позиций возврата и оплаченного заказа.</p>
              <textarea rows={3} value={refundReason} onChange={(event) => setRefundReason(event.target.value)} placeholder="Причина возмещения (необязательно)" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting || selectedReturn.status !== 'item_received'} className="rounded-md bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50">
                Создать возмещение
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

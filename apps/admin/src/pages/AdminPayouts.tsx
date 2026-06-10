import { useEffect, useState } from 'react';
import { AlertCircle, Wallet } from 'lucide-react';
import {
  getAdminPayout,
  getAdminPayoutErrorMessage,
  getAdminPayouts,
  getPayoutStatusLabel,
  getPayoutStatusTargets,
  updateAdminPayoutStatus,
} from '../api/adminPayouts';
import type { AdminPayoutView } from '../api/adminPayouts';
import { PermissionGuard } from '../components/PermissionGuard';

export function AdminPayouts() {
  const [payouts, setPayouts] = useState<AdminPayoutView[]>([]);
  const [selectedPayout, setSelectedPayout] = useState<AdminPayoutView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statusDraft, setStatusDraft] = useState('');
  const [comment, setComment] = useState('');

  const fetchPayouts = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setPayouts(await getAdminPayouts());
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось загрузить выплаты.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchPayoutDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      const detail = await getAdminPayout(id);
      setSelectedPayout(detail);
      setStatusDraft('');
      setComment('');
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось загрузить детали выплаты.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchPayouts();
  }, []);

  const handleStatusUpdate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedPayout || !statusDraft) return;
    if (statusDraft === 'paid' && !window.confirm('Отметить выплату как выплаченную? Это ручное подтверждение. Реальный банковский перевод выполняется вне системы.')) return;

    try {
      setIsSubmitting(true);
      setError(null);
      await updateAdminPayoutStatus(selectedPayout.id, statusDraft, comment);
      await fetchPayouts();
      await fetchPayoutDetail(selectedPayout.id);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось обновить статус выплаты.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'paid':
        return 'bg-green-100 text-green-800';
      case 'approved':
        return 'bg-blue-100 text-blue-800';
      case 'rejected':
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString('ru-RU') : '-';
  const formatMoney = (payout: AdminPayoutView) => `${payout.amount.toFixed(2)} ${payout.currency}`;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Выплаты продавцам</h1>
          <p className="mt-1 text-sm text-gray-500">Выплаты продавцам за реализованные товары. Базовая комиссия ZAMK — 9%. При 2 нарушениях комиссия может быть повышена до 18% на 1 месяц (система нарушений будет подключена отдельно).</p>
        </div>
      </div>

      <div className="bg-amber-50 border-l-4 border-amber-400 p-4">
        <p className="text-sm text-amber-800">
          Кнопка «Отметить как выплачено» — ручное подтверждение. Реальный банковский перевод выполняется вне системы.
        </p>
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
          <p className="mt-2 text-sm text-gray-500">Загрузка выплат...</p>
        </div>
      ) : payouts.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Wallet className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Заявок на выплату нет</h3>
          <p className="mt-1 text-sm text-gray-500">Продавцы ещё не запрашивали выплату средств.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID выплаты</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Продавец</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Запрошено</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сумма</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {payouts.map((payout) => (
                    <tr key={payout.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {payout.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {payout.sellerName || payout.sellerId}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(payout.requestedAt)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {formatMoney(payout)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(payout.status)}`}>
                          {payout.statusLabel}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <button onClick={() => fetchPayoutDetail(payout.id)} className="text-indigo-600 hover:text-indigo-900">Детали / Управление</button>
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

      {selectedPayout && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Детали выплаты</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedPayout.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Загрузка...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Продавец</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayout.sellerName || selectedPayout.sellerId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Сумма</dt><dd className="mt-1 text-sm text-gray-900">{formatMoney(selectedPayout)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Статус</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayout.statusLabel}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Дата запроса</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedPayout.requestedAt)}</dd></div>
          </dl>

          <PermissionGuard
            permission={['payouts.approve', 'payouts.reject', 'payouts.mark_paid']}
            fallback={<p className="mt-6 text-sm text-gray-500">У вас нет прав для изменения статуса выплаты.</p>}
          >
            <form onSubmit={handleStatusUpdate} className="mt-6 grid gap-4 md:grid-cols-2">
              <select required value={statusDraft} onChange={(event) => setStatusDraft(event.target.value)} className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                <option value="">Выберите новый статус</option>
                {getPayoutStatusTargets(selectedPayout.status).map((status) => (
                  <option key={status} value={status}>{getPayoutStatusLabel(status)}</option>
                ))}
              </select>
              <input value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Комментарий (необязательно)" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <button type="submit" disabled={isSubmitting || !statusDraft} className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
                Обновить статус выплаты
              </button>
            </form>
          </PermissionGuard>
        </div>
      )}
    </div>
  );
}

import { useEffect, useState } from 'react';
import { AlertCircle, ReceiptText } from 'lucide-react';
import {
  getAdminRefund,
  getAdminRefundErrorMessage,
  getAdminRefunds,
} from '../api/adminRefunds';
import type { AdminRefundView } from '../api/adminRefunds';

export function AdminRefunds() {
  const [refunds, setRefunds] = useState<AdminRefundView[]>([]);
  const [selectedRefund, setSelectedRefund] = useState<AdminRefundView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchRefunds = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setRefunds(await getAdminRefunds());
    } catch (err: unknown) {
      setError(getAdminRefundErrorMessage(err, 'Не удалось загрузить возмещения.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchRefundDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      setSelectedRefund(await getAdminRefund(id));
    } catch (err: unknown) {
      setError(getAdminRefundErrorMessage(err, 'Не удалось загрузить детали возмещения.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchRefunds();
  }, []);

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'succeeded':
        return 'bg-green-100 text-green-800';
      case 'failed':
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      case 'processing':
        return 'bg-blue-100 text-blue-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';
  const formatMoney = (refund: AdminRefundView) => `${refund.amount.toFixed(2)} ${refund.currency}`;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Возмещения</h1>
      </div>

      <div className="bg-blue-50 border-l-4 border-blue-400 p-4">
        <p className="text-sm text-blue-700">Возмещения доступны только для просмотра. Backend рассчитывает суммы и данные провайдера.</p>
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
          <p className="mt-2 text-sm text-gray-500">Загрузка возмещений...</p>
        </div>
      ) : refunds.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ReceiptText className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Нет возмещений</h3>
          <p className="mt-1 text-sm text-gray-500">Записи о возмещениях пока отсутствуют.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Возмещение</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Return / Order</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Provider</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Amount</th>
                      <th className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {refunds.map((refund) => (
                      <tr key={refund.id}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{refund.id}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{refund.returnId || '-'} / {refund.orderId}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{refund.provider || '-'}</td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(refund.status)}`}>
                            {refund.status}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{formatMoney(refund)}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button onClick={() => fetchRefundDetail(refund.id)} className="text-indigo-600 hover:text-indigo-900">View</button>
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

      {selectedRefund && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Refund details</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedRefund.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Loading...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Amount</dt><dd className="mt-1 text-sm text-gray-900">{formatMoney(selectedRefund)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Provider refund ID</dt><dd className="mt-1 text-sm text-gray-900">{selectedRefund.providerRefundId || '-'}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Reason</dt><dd className="mt-1 text-sm text-gray-900">{selectedRefund.reason || '-'}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Processed / Failed</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedRefund.processedAt || selectedRefund.failedAt)}</dd></div>
          </dl>
        </div>
      )}
    </div>
  );
}

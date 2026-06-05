import { useEffect, useState } from 'react';
import { AlertCircle, CreditCard } from 'lucide-react';
import {
  getAdminPayment,
  getAdminPaymentErrorMessage,
  getAdminPayments,
} from '../api/adminPayments';
import type { AdminPaymentView } from '../api/adminPayments';

export function AdminPayments() {
  const [payments, setPayments] = useState<AdminPaymentView[]>([]);
  const [selectedPayment, setSelectedPayment] = useState<AdminPaymentView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchPayments = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setPayments(await getAdminPayments());
    } catch (err: unknown) {
      setError(getAdminPaymentErrorMessage(err, 'Failed to load payments.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchPaymentDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      setSelectedPayment(await getAdminPayment(id));
    } catch (err: unknown) {
      setError(getAdminPaymentErrorMessage(err, 'Failed to load payment details.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchPayments();
  }, []);

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'succeeded':
      case 'paid':
        return 'bg-green-100 text-green-800';
      case 'failed':
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';
  const formatMoney = (payment: AdminPaymentView) => `${payment.amount.toFixed(2)} ${payment.currency}`;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Payments</h1>
      </div>

      <div className="bg-blue-50 border-l-4 border-blue-400 p-4">
        <p className="text-sm text-blue-700">Payments are read-only in admin. Paid/succeeded transitions are owned by payment webhooks.</p>
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
          <p className="mt-2 text-sm text-gray-500">Loading payments...</p>
        </div>
      ) : payments.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <CreditCard className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No payments</h3>
          <p className="mt-1 text-sm text-gray-500">No payment records are available yet.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Payment</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Order</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Provider</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Amount</th>
                      <th className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {payments.map((payment) => (
                      <tr key={payment.id}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{payment.id}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{payment.orderId}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{payment.provider}</td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(payment.status)}`}>
                            {payment.status}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{formatMoney(payment)}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button onClick={() => fetchPaymentDetail(payment.id)} className="text-indigo-600 hover:text-indigo-900">View</button>
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

      {selectedPayment && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Payment details</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedPayment.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Loading...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-3">
            <div><dt className="text-sm font-medium text-gray-500">Order</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayment.orderId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Provider</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayment.provider}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Provider payment ID</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayment.providerPaymentId || '-'}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Amount</dt><dd className="mt-1 text-sm text-gray-900">{formatMoney(selectedPayment)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Created</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedPayment.createdAt)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Paid / Failed / Cancelled</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedPayment.paidAt || selectedPayment.failedAt || selectedPayment.cancelledAt)}</dd></div>
          </dl>
        </div>
      )}
    </div>
  );
}

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
      setError(getAdminPayoutErrorMessage(err, 'Failed to load payouts.'));
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
      setError(getAdminPayoutErrorMessage(err, 'Failed to load payout details.'));
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
    if (statusDraft === 'paid' && !window.confirm('Mark payout as paid? This should only be used after offline transfer is complete.')) return;

    try {
      setIsSubmitting(true);
      setError(null);
      await updateAdminPayoutStatus(selectedPayout.id, statusDraft, comment);
      await fetchPayouts();
      await fetchPayoutDetail(selectedPayout.id);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Failed to update payout status.'));
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
        <h1 className="text-2xl font-bold text-gray-900">Seller Payouts</h1>
      </div>

      <div className="bg-blue-50 border-l-4 border-blue-400 p-4">
        <p className="text-sm text-blue-700">No bank payout is triggered here. Mark paid only after an offline/manual transfer is already complete.</p>
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
          <p className="mt-2 text-sm text-gray-500">Loading payouts...</p>
        </div>
      ) : payouts.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Wallet className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No payouts</h3>
          <p className="mt-1 text-sm text-gray-500">No payout requests are available yet.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Payout ID</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Seller</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Requested</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Amount</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
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
                        <button onClick={() => fetchPayoutDetail(payout.id)} className="text-indigo-600 hover:text-indigo-900">View / Manage</button>
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
              <h2 className="text-lg font-medium text-gray-900">Payout details</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedPayout.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Loading...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Seller</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayout.sellerName || selectedPayout.sellerId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Amount</dt><dd className="mt-1 text-sm text-gray-900">{formatMoney(selectedPayout)}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Status</dt><dd className="mt-1 text-sm text-gray-900">{selectedPayout.statusLabel}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Requested</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedPayout.requestedAt)}</dd></div>
          </dl>

          <form onSubmit={handleStatusUpdate} className="mt-6 grid gap-4 md:grid-cols-2">
            <select required value={statusDraft} onChange={(event) => setStatusDraft(event.target.value)} className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
              <option value="">Select next status</option>
              {getPayoutStatusTargets(selectedPayout.status).map((status) => (
                <option key={status} value={status}>{getPayoutStatusLabel(status)}</option>
              ))}
            </select>
            <input value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Comment, optional" className="rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
            <button type="submit" disabled={isSubmitting || !statusDraft} className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50">
              Update payout
            </button>
          </form>
        </div>
      )}
    </div>
  );
}

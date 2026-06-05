import { useEffect, useState } from 'react';
import { getSellerBalance, getSellerPayouts, requestSellerPayout } from '@zamk/api-client/src/seller';
import { adaptBalance, adaptPayouts } from '../api/sellerFinance';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

export function SellerPayouts() {
  const [balance, setBalance] = useState<any>(null);
  const [payouts, setPayouts] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  
  const [amountInput, setAmountInput] = useState('');
  const [commentInput, setCommentInput] = useState('');
  const [requestError, setRequestError] = useState('');
  const [isRequesting, setIsRequesting] = useState(false);

  const fetchData = async () => {
    setIsLoading(true);
    setError('');
    try {
      const [balData, payData] = await Promise.all([
        getSellerBalance(),
        getSellerPayouts()
      ]);
      setBalance(adaptBalance(balData));
      setPayouts(adaptPayouts(payData));
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки финансов');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleRequestPayout = async (e: React.FormEvent) => {
    e.preventDefault();
    setRequestError('');
    
    const amountCents = parseInt(amountInput, 10) * 100;
    if (isNaN(amountCents) || amountCents <= 0) {
      setRequestError('Введите корректную сумму');
      return;
    }

    if (balance && amountCents / 100 > balance.availableBalance) {
      setRequestError('Запрошенная сумма превышает доступный баланс');
      return;
    }

    setIsRequesting(true);
    try {
      await requestSellerPayout(amountCents, commentInput);
      setAmountInput('');
      setCommentInput('');
      await fetchData(); // Refetch to update balance and payouts list
    } catch (err: any) {
      setRequestError(err.message || 'Ошибка при запросе выплаты');
    } finally {
      setIsRequesting(false);
    }
  };

  if (isLoading && !balance) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  if (error && !balance) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="p-8 max-w-6xl mx-auto space-y-8">
      <div>
        <h1 className="text-2xl font-bold mb-2">Финансы и выплаты</h1>
        <p className="text-gray-600">Здесь вы можете управлять балансом и запрашивать выплаты.</p>
      </div>

      {/* Balance Cards */}
      {balance && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
            <p className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">Доступно к выплате</p>
            <p className="text-3xl font-bold text-gray-900">{currencyFormatter.format(balance.availableBalance)}</p>
          </div>
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
            <p className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">В ожидании</p>
            <p className="text-3xl font-bold text-gray-900">{currencyFormatter.format(balance.pendingBalance)}</p>
          </div>
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
            <p className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">Запрошено</p>
            <p className="text-3xl font-bold text-gray-900">{currencyFormatter.format(balance.requestedPayouts)}</p>
          </div>
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
            <p className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-2">Уже выплачено</p>
            <p className="text-3xl font-bold text-gray-900">{currencyFormatter.format(balance.paidPayouts)}</p>
          </div>
        </div>
      )}

      {/* Request Payout Form */}
      <div className="bg-white p-6 rounded-2xl shadow-sm border border-gray-100">
        <h2 className="text-lg font-bold mb-4">Запросить выплату</h2>
        <form onSubmit={handleRequestPayout} className="flex flex-col sm:flex-row gap-4 items-start sm:items-end">
          <div className="flex-1 w-full">
            <label className="block text-sm font-medium text-gray-700 mb-1">Сумма (₽)</label>
            <input
              type="number"
              min="1"
              value={amountInput}
              onChange={(e) => setAmountInput(e.target.value)}
              className="w-full rounded-xl border border-gray-200 p-3 outline-none focus:border-black focus:ring-1 focus:ring-black"
              placeholder="Сумма к выплате"
            />
          </div>
          <div className="flex-1 w-full">
            <label className="block text-sm font-medium text-gray-700 mb-1">Комментарий (необязательно)</label>
            <input
              type="text"
              value={commentInput}
              onChange={(e) => setCommentInput(e.target.value)}
              className="w-full rounded-xl border border-gray-200 p-3 outline-none focus:border-black focus:ring-1 focus:ring-black"
              placeholder="Назначение платежа..."
            />
          </div>
          <button
            type="submit"
            disabled={isRequesting || !amountInput}
            className="w-full sm:w-auto h-12 px-6 bg-black text-white rounded-xl font-semibold hover:bg-gray-800 disabled:opacity-50 transition-colors"
          >
            {isRequesting ? 'Запрашиваем...' : 'Создать заявку'}
          </button>
        </form>
        {requestError && <p className="mt-2 text-sm text-red-600 font-medium">{requestError}</p>}
      </div>

      {/* Payouts List */}
      <div>
        <h2 className="text-lg font-bold mb-4">История выплат</h2>
        <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Дата заявки</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Сумма</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Статус</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Комментарий</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {payouts.map((p) => (
                <tr key={p.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{p.requestedAt}</td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{currencyFormatter.format(p.amount)}</td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <span className={`px-3 py-1 inline-flex text-xs leading-5 font-semibold rounded-full 
                      ${p.status === 'paid' ? 'bg-green-100 text-green-800' : 
                        p.status === 'requested' ? 'bg-yellow-100 text-yellow-800' : 
                        p.status === 'rejected' ? 'bg-red-100 text-red-800' :
                        'bg-gray-100 text-gray-800'}`}>
                      {p.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{p.comment || '-'}</td>
                </tr>
              ))}
              {payouts.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center text-sm text-gray-500">
                    У вас пока нет заявок на выплату
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

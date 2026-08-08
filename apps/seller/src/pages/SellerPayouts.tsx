import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getSellerBalance, getSellerLedger, getSellerPayouts } from '@zamk/api-client/src/seller';
import { adaptBalance, adaptLedger, adaptPayoutBatches } from '../api/sellerFinance';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

export function SellerPayouts() {
  const [balance, setBalance] = useState<any>(null);
  const [ledger, setLedger] = useState<any[]>([]);
  const [payouts, setPayouts] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null);

  const fetchData = async () => {
    setIsLoading(true);
    setError('');
    try {
      const [balData, ledData, payData] = await Promise.all([
        getSellerBalance(),
        getSellerLedger(),
        getSellerPayouts()
      ]);
      setBalance(adaptBalance(balData));
      setLedger(adaptLedger(ledData));
      setPayouts(adaptPayoutBatches(payData));
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки финансов');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

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
        <p className="text-gray-600 mb-4">Здесь вы можете управлять балансом и просматривать операции.</p>
        <div className="rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800">
          <p className="font-medium">Выплаты формируются автоматически один раз в неделю. Суммы замораживаются на 14 дней с момента доставки покупателю.</p>
        </div>
      </div>

      {/* Balance Cards (Global Money Strip) */}
      {balance && (
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100 flex flex-col justify-between">
            <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Продажи (Gross)</p>
            <p className="text-2xl font-bold text-gray-900">{currencyFormatter.format(balance.grossSales)}</p>
          </div>
          <div className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100 flex flex-col justify-between">
            <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Удержания (ZAMK)</p>
            <p className="text-2xl font-bold text-red-600">-{currencyFormatter.format(balance.commission)}</p>
          </div>
          <div className="bg-white p-4 rounded-2xl shadow-sm border border-orange-200 flex flex-col justify-between relative overflow-hidden">
            <div className="absolute top-0 left-0 w-1 h-full bg-orange-400"></div>
            <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Заморожено</p>
            <p className="text-2xl font-bold text-gray-900">{currencyFormatter.format(balance.frozen)}</p>
          </div>
          <div className="bg-white p-4 rounded-2xl shadow-sm border border-green-200 flex flex-col justify-between relative overflow-hidden">
            <div className="absolute top-0 left-0 w-1 h-full bg-green-500"></div>
            <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Доступно к выплате</p>
            <p className="text-2xl font-bold text-green-700">{currencyFormatter.format(balance.available)}</p>
          </div>
          <div className="bg-white p-4 rounded-2xl shadow-sm border border-gray-100 flex flex-col justify-between">
            <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Уже выплачено</p>
            <p className="text-2xl font-bold text-gray-900">{currencyFormatter.format(balance.paid)}</p>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Ledger List */}
        <div>
          <h2 className="text-lg font-bold mb-4">История операций</h2>
          <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Дата</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Тип</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Сумма</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Разморозка</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {ledger.map((l) => (
                  <tr 
                    key={l.id} 
                    className="hover:bg-gray-50 text-sm cursor-pointer transition-colors"
                    onClick={() => l.orderId && setSelectedOrderId(l.orderId)}
                  >
                    <td className="px-4 py-3 whitespace-nowrap text-gray-900">{l.createdAt}</td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <span className="font-medium text-gray-700">
                        {l.type === 'return_deduction' ? 'Возврат покупателю' : l.type}
                      </span>
                      {l.orderId && <div className="text-xs text-gray-400 mt-0.5">Заказ</div>}
                    </td>
                    <td className={`px-4 py-3 whitespace-nowrap font-medium ${l.amount < 0 ? 'text-red-600' : 'text-green-600'}`}>
                      {l.amount > 0 ? '+' : ''}{currencyFormatter.format(l.amount)}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-gray-500">{l.availableAt || '-'}</td>
                  </tr>
                ))}
                {ledger.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-gray-500">
                      У вас пока нет операций
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Payouts Batches */}
        <div>
          <h2 className="text-lg font-bold mb-4">Автоматические выплаты</h2>
          <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Назначено на</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Сумма</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Статус</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {payouts.map((p) => (
                  <tr key={p.id} className="hover:bg-gray-50 text-sm">
                    <td className="px-4 py-3 whitespace-nowrap text-gray-900">{p.scheduledFor}</td>
                    <td className="px-4 py-3 whitespace-nowrap font-medium text-gray-900">{currencyFormatter.format(p.amount)}</td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <span className={`px-3 py-1 inline-flex text-xs leading-5 font-semibold rounded-full 
                        ${p.status === 'paid' ? 'bg-green-100 text-green-800' : 
                          p.status === 'scheduled' ? 'bg-blue-100 text-blue-800' : 
                          p.status === 'held' ? 'bg-orange-100 text-orange-800' :
                          'bg-gray-100 text-gray-800'}`}>
                        {p.status}
                      </span>
                    </td>
                  </tr>
                ))}
                {payouts.length === 0 && (
                  <tr>
                    <td colSpan={3} className="px-4 py-8 text-center text-gray-500">
                      Выплат пока нет
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Ledger Details Modal */}
      {selectedOrderId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg p-6 overflow-hidden flex flex-col max-h-full">
            <div className="flex justify-between items-center mb-4">
              <h3 className="text-lg font-bold text-gray-900">Детали начисления</h3>
              <button 
                onClick={() => setSelectedOrderId(null)}
                className="text-gray-400 hover:text-gray-500 transition-colors"
              >
                ✕
              </button>
            </div>
            
            <div className="mb-6">
              <p className="text-sm text-gray-500 mb-1">Связанный заказ</p>
              <Link 
                to={`/orders/${selectedOrderId}`}
                className="text-blue-600 hover:text-blue-700 font-medium text-sm flex items-center gap-1"
              >
                {selectedOrderId}
                <span className="text-xs">↗</span>
              </Link>
            </div>

            <div>
              <h4 className="text-sm font-semibold text-gray-900 mb-3 uppercase tracking-wider">Структура начисления</h4>
              <div className="bg-gray-50 rounded-xl p-4 border border-gray-100 space-y-3">
                {ledger.filter(entry => entry.orderId === selectedOrderId).map(entry => (
                  <div key={entry.id} className="flex justify-between items-center text-sm">
                    <span className="text-gray-600 font-medium">
                      {entry.type === 'seller_earning' ? 'Чистый доход' : 
                       entry.type === 'sale_gross' ? 'Сумма продажи' :
                       entry.type === 'zamk_commission' ? 'Комиссия ZAMK' : 
                       entry.type === 'adjustment' ? 'Корректировка' : entry.type}
                    </span>
                    <span className={`font-bold ${entry.amount < 0 ? 'text-red-600' : 'text-gray-900'}`}>
                      {entry.amount > 0 ? '+' : ''}{currencyFormatter.format(entry.amount)}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            <div className="mt-6">
              <button 
                onClick={() => setSelectedOrderId(null)}
                className="w-full bg-gray-100 hover:bg-gray-200 text-gray-900 font-medium py-2 px-4 rounded-lg transition-colors"
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, CheckCircle, XCircle, AlertTriangle, RefreshCw } from 'lucide-react';
import { CheckoutPanel } from '../components/editorial/StudioKit';
import { API_URL } from '../lib/api';

interface PaymentDetails {
  id: string;
  orderId: string;
  paymentNumber: string;
  provider: string;
  paymentMethod: string;
  integrationMode: string;
  status: string;
  amountCents: number;
  currency: string;
}

export function DevMockPayment() {
  const { paymentId } = useParams<{ paymentId: string }>();
  const [payment, setPayment] = useState<PaymentDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionProcessing, setActionProcessing] = useState(false);

  const fetchPayment = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await fetch(`${API_URL}/dev/payments/mock/${paymentId}`);
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data?.error?.message || ` Ошибка получения платежа (${res.status})`);
      }
      const data = await res.json();
      setPayment(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить данные тестового платежа.');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (paymentId) {
      fetchPayment();
    }
  }, [paymentId]);

  const handleAction = async (action: 'confirm' | 'reject' | 'cancel') => {
    if (!paymentId || actionProcessing) return;
    try {
      setActionProcessing(true);
      setError(null);
      const res = await fetch(`${API_URL}/dev/payments/mock/${paymentId}/${action}`, {
        method: 'POST',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data?.error?.message || `Ошибка выполнения действия (${res.status})`);
      }
      await fetchPayment();
    } catch (err: any) {
      setError(err.message || 'Ошибка обработки решения.');
    } finally {
      setActionProcessing(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen pt-20 flex items-center justify-center">
        <div className="text-center space-y-3">
          <RefreshCw className="w-8 h-8 animate-spin mx-auto text-yellow-500" />
          <p className="text-sm text-graphite-light dark:text-white/60">Загрузка тестового платежа T-Pay...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen pt-16 md:pt-20 pb-20 z-10 relative">
      <div className="container mx-auto px-4 max-w-2xl">
        <Link to="/checkout" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white mb-6">
          <ArrowLeft className="w-4 h-4" /> Назад к оформлению
        </Link>

        <CheckoutPanel>
          <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 mb-6 flex items-start gap-3">
            <AlertTriangle className="w-6 h-6 text-yellow-600 dark:text-yellow-400 shrink-0 mt-0.5" />
            <div>
              <h3 className="font-semibold text-yellow-800 dark:text-yellow-300 text-sm">Эмулятор T-Pay (Development Only)</h3>
              <p className="text-xs text-yellow-700 dark:text-yellow-400 mt-0.5">
                Тестовая операция. Реального списания не будет.
              </p>
            </div>
          </div>

          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/30 text-red-600 dark:text-red-400 text-xs rounded-lg mb-6">
              {error}
            </div>
          )}

          {payment && (
            <div className="space-y-6">
              <div className="border-b border-border-lighter dark:border-white/10 pb-4">
                <span className="text-xs uppercase font-semibold text-ash tracking-wider">Информация о платеже</span>
                <div className="mt-2 grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-ash text-xs">Номер платежа</p>
                    <p className="font-mono font-medium text-graphite dark:text-white">{payment.paymentNumber}</p>
                  </div>
                  <div>
                    <p className="text-ash text-xs">ID Заказа</p>
                    <p className="font-mono text-xs text-graphite dark:text-white/80">{payment.orderId.substring(0, 13)}...</p>
                  </div>
                  <div>
                    <p className="text-ash text-xs">Сумма к оплате</p>
                    <p className="font-serif text-lg font-bold text-graphite dark:text-white">
                      {(payment.amountCents / 100).toLocaleString('ru-RU')} ₽
                    </p>
                  </div>
                  <div>
                    <p className="text-ash text-xs">Текущий статус</p>
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium mt-1 ${
                      payment.status === 'succeeded' ? 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300' :
                      payment.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300' :
                      payment.status === 'cancelled' ? 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300' :
                      'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300'
                    }`}>
                      {payment.status === 'pending' ? 'Ожидает решения' : 
                       payment.status === 'succeeded' ? 'Тестовая оплата подтверждена' : payment.status}
                    </span>
                  </div>
                </div>
              </div>

              {payment.status === 'pending' ? (
                <div className="space-y-3 pt-2">
                  <p className="text-xs font-medium text-graphite dark:text-white mb-2">Выберите результат эмуляции:</p>
                  
                  <button
                    type="button"
                    onClick={() => handleAction('confirm')}
                    disabled={actionProcessing}
                    className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-lg bg-green-600 hover:bg-green-700 text-white font-medium text-sm transition-colors disabled:opacity-50"
                  >
                    <CheckCircle className="w-4 h-4" />
                    Подтвердить тестовую оплату
                  </button>

                  <div className="grid grid-cols-2 gap-3">
                    <button
                      type="button"
                      onClick={() => handleAction('reject')}
                      disabled={actionProcessing}
                      className="flex items-center justify-center gap-2 py-2.5 px-4 rounded-lg bg-red-600 hover:bg-red-700 text-white font-medium text-xs transition-colors disabled:opacity-50"
                    >
                      <XCircle className="w-4 h-4" />
                      Отклонить
                    </button>

                    <button
                      type="button"
                      onClick={() => handleAction('cancel')}
                      disabled={actionProcessing}
                      className="flex items-center justify-center gap-2 py-2.5 px-4 rounded-lg bg-gray-600 hover:bg-gray-700 text-white font-medium text-xs transition-colors disabled:opacity-50"
                    >
                      Отменить
                    </button>
                  </div>
                </div>
              ) : (
                <div className="text-center py-4 space-y-4">
                  <p className="text-sm text-graphite dark:text-white">
                    {payment.status === 'succeeded' ? (
                      <>Статус: <strong className="font-semibold">Тестовая оплата подтверждена</strong></>
                    ) : (
                      <>Платёж переведён в статус <strong className="font-semibold">{payment.status}</strong>.</>
                    )}
                  </p>
                  <p className="text-xs text-ash font-mono mt-1">Dev Diagnostic: status={payment.status}</p>
                  <div className="flex justify-center gap-3">
                    <Link to="/orders" className="px-4 py-2 bg-black text-white dark:bg-white dark:text-black text-sm rounded-lg font-medium">
                      Мои заказы
                    </Link>
                    <Link to="/checkout" className="px-4 py-2 border border-border-lighter text-graphite dark:text-white text-sm rounded-lg">
                      Вернуться в оформление
                    </Link>
                  </div>
                </div>
              )}
            </div>
          )}
        </CheckoutPanel>
      </div>
    </div>
  );
}

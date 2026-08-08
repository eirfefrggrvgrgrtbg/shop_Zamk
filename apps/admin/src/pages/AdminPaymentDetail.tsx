import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate, useLocation, Link } from 'react-router-dom';
import { AlertCircle, ArrowLeft, AlertTriangle, RefreshCw, ChevronDown, ChevronUp, Copy, Check } from 'lucide-react';
import type { AdminPaymentDetail } from '@zamk/api-client/src/types';

const PAYMENT_STATUS_LABELS: Record<string, string> = {
  created: 'Создан',
  pending: 'Ожидает',
  succeeded: 'Успешно',
  failed: 'Ошибка',
  cancelled: 'Отменён',
};

const REFUND_STATE_LABELS: Record<string, string> = {
  none: 'Нет',
  pending: 'Ожидает',
  partial: 'Частичный',
  full: 'Полный',
  partial_pending: 'Част. (ожид)',
  full_pending: 'Полный (ожид)',
};

const PROVIDER_LABELS: Record<string, string> = {
  tbank: 'Т-Банк',
};

const PAYMENT_METHOD_LABELS: Record<string, string> = {
  tpay: 'T-Pay',
  spb: 'СБП',
  card: 'Карта',
};



const PROBLEM_CODE_LABELS: Record<string, string> = {
  PAID_ORDER_WITHOUT_SUCCEEDED_PAYMENT: 'Заказ оплачен, но платеж не успешен',
  SUCCEEDED_PAYMENT_ORDER_NOT_PAID: 'Платеж успешен, но заказ не оплачен',
  MULTIPLE_SUCCEEDED_PAYMENTS: 'Несколько успешных платежей',
  AMOUNT_MISMATCH: 'Несовпадение суммы',
  STUCK_PENDING: 'Завис в ожидании',
  INVALID_WEBHOOK_SIGNATURE: 'Неверная подпись webhook',
  UNPROCESSED_WEBHOOK: 'Необработанный webhook',
};

export function AdminPaymentDetail() {
  const { paymentId } = useParams<{ paymentId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const [data, setData] = useState<AdminPaymentDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showTechInfo, setShowTechInfo] = useState(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const backSearchParams = location.state?.from || '';

  const fetchDetail = async () => {
    if (!paymentId) return;
    setLoading(true);
    setError(null);
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    abortControllerRef.current = new AbortController();
    try {
      // Temporary cast as the api method handles this correctly in our case
      // We had to use getAdminPaymentDetail instead of getAdminPayment from api-client to get the extra fields
      // But we just need to import it properly. Wait, it's actually in @zamk/api-client/src/admin. Let me import it from there.
      const { getAdminPaymentDetail } = await import('@zamk/api-client/src/admin');
      const res = await getAdminPaymentDetail(paymentId, abortControllerRef.current.signal);
      setData(res);
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      if (err.status === 404) {
        setError('Платёж не найден');
      } else {
        setError(err.message || 'Ошибка загрузки деталей платежа');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDetail();
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [paymentId]);

  const handleBack = () => {
    if (backSearchParams) {
      navigate(`/payments?${backSearchParams}`);
    } else {
      navigate('/payments');
    }
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const formatMoney = (cents: number, currency: string) => {
    return new Intl.NumberFormat('ru-RU', {
      style: 'currency',
      currency: currency || 'RUB',
    }).format(cents / 100);
  };

  const formatDate = (isoStr?: string | null) => {
    if (!isoStr) return '—';
    return new Date(isoStr).toLocaleString('ru-RU', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  if (loading) {
    return (
      <div className="p-6 max-w-7xl mx-auto animate-pulse">
        <div className="h-8 bg-gray-200 rounded w-1/4 mb-6"></div>
        <div className="bg-white shadow rounded-lg p-6 mb-6">
          <div className="h-6 bg-gray-200 rounded w-1/3 mb-4"></div>
          <div className="space-y-3">
            <div className="h-4 bg-gray-200 rounded w-full"></div>
            <div className="h-4 bg-gray-200 rounded w-5/6"></div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 max-w-7xl mx-auto">
        <button onClick={handleBack} className="mb-4 inline-flex items-center text-sm text-gray-500 hover:text-gray-700">
          <ArrowLeft className="w-4 h-4 mr-1" /> Вернуться к списку
        </button>
        <div className="bg-white shadow rounded-lg p-10 flex flex-col items-center justify-center text-center">
          {error === 'Платёж не найден' ? (
             <div className="flex flex-col items-center">
               <AlertCircle className="w-12 h-12 text-gray-400 mb-4" />
               <h3 className="text-lg font-medium text-gray-900">По вашему запросу ничего не найдено</h3>
               <p className="text-sm text-gray-500 mt-2">Платёж не существует или был удалён.</p>
             </div>
          ) : (
            <div className="flex flex-col items-center">
              <AlertCircle className="w-12 h-12 text-red-500 mb-4" />
              <h3 className="text-lg font-medium text-gray-900">Ошибка загрузки платежа</h3>
              <p className="text-sm text-red-600 mt-2">{error}</p>
              <button onClick={fetchDetail} className="mt-4 px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 flex items-center">
                <RefreshCw className="w-4 h-4 mr-2" /> Повторить
              </button>
            </div>
          )}
        </div>
      </div>
    );
  }

  if (!data) return null;

  const { payment, order, attempts, providerEvents, refunds, problems } = data;

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'succeeded': return 'bg-green-100 text-green-800 border-green-200';
      case 'pending': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      case 'failed': return 'bg-red-100 text-red-800 border-red-200';
      case 'cancelled': return 'bg-gray-100 text-gray-800 border-gray-200';
      case 'created': return 'bg-blue-100 text-blue-800 border-blue-200';
      default: return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  const getRefundBadgeClass = (state: string) => {
    switch (state) {
      case 'none': return 'bg-gray-100 text-gray-800 border-gray-200';
      case 'partial':
      case 'partial_pending': return 'bg-yellow-100 text-yellow-800 border-yellow-200';
      case 'full':
      case 'full_pending': return 'bg-red-100 text-red-800 border-red-200';
      default: return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* Header section */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <button onClick={handleBack} className="p-2 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 text-gray-500 shadow-sm transition-colors">
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-3">
              Платёж {payment.paymentNumber}
              {payment.integrationMode === 'mock' && (
                <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold bg-amber-400 text-black">ТЕСТОВЫЙ</span>
              )}
            </h1>
            <p className="text-sm text-gray-500 mt-1 flex items-center gap-2">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${getStatusBadgeClass(payment.status)}`}>
                {PAYMENT_STATUS_LABELS[payment.status] || payment.status}
              </span>
              <span>•</span>
              <span className="font-medium text-gray-700">{PROVIDER_LABELS[payment.provider || ''] || payment.provider || 'Нет провайдера'}</span>
              <span>•</span>
              <span className="uppercase">{PAYMENT_METHOD_LABELS[payment.paymentMethod || ''] || payment.paymentMethod || 'Не выбран'}</span>
            </p>
          </div>
        </div>
      </div>

      {/* Problems Section */}
      {problems && problems.length > 0 && (
        <div className="bg-white rounded-xl shadow-sm border border-red-200 overflow-hidden">
          <div className="px-6 py-4 bg-red-50 border-b border-red-100 flex items-center">
            <AlertTriangle className="w-5 h-5 text-red-600 mr-2" />
            <h2 className="text-sm font-bold text-red-800 uppercase tracking-wide">Требует внимания</h2>
          </div>
          <div className="p-6 space-y-3">
            {problems.map((prob, idx) => (
              <div key={idx} className={`border rounded-lg p-4 flex items-start ${prob.severity === 'critical' ? 'bg-red-50 border-red-200' : 'bg-amber-50 border-amber-200'}`}>
                <AlertTriangle className={`w-5 h-5 mr-3 mt-0.5 ${prob.severity === 'critical' ? 'text-red-500' : 'text-amber-500'}`} />
                <div>
                  <div className={`font-bold ${prob.severity === 'critical' ? 'text-red-900' : 'text-amber-900'}`}>
                    {PROBLEM_CODE_LABELS[prob.code] || prob.code}
                  </div>
                  <div className={`text-sm mt-1 ${prob.severity === 'critical' ? 'text-red-700' : 'text-amber-700'}`}>
                    Код: <code className="bg-white/50 px-1 py-0.5 rounded">{prob.code}</code> | Уровень: {prob.severity}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          {/* Main Info */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Сводка по платежу</h2>
            </div>
            <div className="p-6 grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <dt className="text-sm font-medium text-gray-500">Номер платежа</dt>
                <dd className="mt-1 text-sm font-bold text-gray-900">{payment.paymentNumber}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Сумма к оплате</dt>
                <dd className="mt-1 text-lg font-bold text-gray-900">{formatMoney(payment.amountCents, payment.currency)}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Создан</dt>
                <dd className="mt-1 text-sm text-gray-900">{formatDate(payment.createdAt)}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Оплачен</dt>
                <dd className="mt-1 text-sm text-gray-900">{formatDate(payment.paidAt)}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Отменён / Ошибка</dt>
                <dd className="mt-1 text-sm text-gray-900">{formatDate(payment.cancelledAt || payment.failedAt)}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">ID Провайдера</dt>
                <dd className="mt-1 text-sm text-gray-900 font-mono">{payment.providerPaymentId || '—'}</dd>
              </div>
            </div>
          </div>

          {/* Attempts */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Попытки оплаты ({payment.attemptsCount})</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">№</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Номер</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Метод</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сумма</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Время</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {attempts.length === 0 ? (
                    <tr><td colSpan={6} className="px-6 py-4 text-center text-sm text-gray-500">Нет попыток</td></tr>
                  ) : attempts.map((attempt) => (
                    <tr key={attempt.paymentId} className={attempt.paymentId === payment.paymentId ? "bg-indigo-50/50" : ""}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{attempt.attemptNumber}</td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-indigo-600">
                        {attempt.paymentId !== payment.paymentId ? (
                          <Link to={`/payments/${attempt.paymentId}`} className="hover:underline">{attempt.paymentNumber}</Link>
                        ) : (
                          <span>{attempt.paymentNumber} (Текущий)</span>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${getStatusBadgeClass(attempt.status)}`}>
                          {PAYMENT_STATUS_LABELS[attempt.status] || attempt.status}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {PAYMENT_METHOD_LABELS[attempt.paymentMethod || ''] || attempt.paymentMethod || '—'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {formatMoney(attempt.amountCents, payment.currency)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(attempt.createdAt)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Provider Events */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">События провайдера (Webhooks)</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Событие</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Получено</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Обработано</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Подпись</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Ключ идемпотентности</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {providerEvents.length === 0 ? (
                    <tr><td colSpan={5} className="px-6 py-4 text-center text-sm text-gray-500">Нет событий</td></tr>
                  ) : providerEvents.map((evt) => (
                    <tr key={evt.eventId}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        <div className="flex flex-col">
                          <span>{evt.eventType}</span>
                          {evt.safePayloadSummary && Object.keys(evt.safePayloadSummary).length > 0 && (
                            <span className="text-[10px] text-gray-500 mt-1 font-mono">
                              {JSON.stringify(evt.safePayloadSummary).slice(0, 50)}...
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{formatDate(evt.createdAt)}</td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{formatDate(evt.processedAt)}</td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {evt.signatureValid ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">Валидна</span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">Неверна</span>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-400 font-mono">
                        {evt.eventKey.split('-')[0]}...
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Refunds */}
          {refunds.length > 0 && (
             <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
               <div className="px-6 py-4 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
                 <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Возвраты</h2>
               </div>
               <div className="overflow-x-auto">
                 <table className="min-w-full divide-y divide-gray-200">
                   <thead className="bg-gray-50">
                     <tr>
                       <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                       <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сумма</th>
                       <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Создан</th>
                       <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Обработан</th>
                       <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID Провайдера</th>
                     </tr>
                   </thead>
                   <tbody className="bg-white divide-y divide-gray-200">
                     {refunds.map((refund) => (
                       <tr key={refund.refundId}>
                         <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                           {refund.status}
                         </td>
                         <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-red-600">
                           {formatMoney(refund.amountCents, payment.currency)}
                         </td>
                         <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{formatDate(refund.createdAt)}</td>
                         <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{formatDate(refund.processedAt)}</td>
                         <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-400 font-mono">
                           {refund.providerRefundId || '—'}
                         </td>
                       </tr>
                     ))}
                   </tbody>
                 </table>
               </div>
             </div>
          )}
        </div>

        <div className="space-y-6">
          {/* Financial Summary */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 bg-gray-50">
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Финансовая сводка</h2>
            </div>
            <div className="p-6">
              <div className="mb-4">
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${getRefundBadgeClass(payment.refundState)}`}>
                  Статус возвратов: {REFUND_STATE_LABELS[payment.refundState] || payment.refundState}
                </span>
              </div>
              <dl className="space-y-4">
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <dt className="text-sm text-gray-500">Итого оплачено</dt>
                  <dd className="text-sm font-medium text-gray-900">{formatMoney(payment.paidAmountCents, payment.currency)}</dd>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <dt className="text-sm text-gray-500">Успешные возвраты</dt>
                  <dd className="text-sm font-medium text-red-600">{formatMoney(payment.succeededRefundedAmountCents, payment.currency)}</dd>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <dt className="text-sm text-gray-500">Возвраты в обработке</dt>
                  <dd className="text-sm font-medium text-yellow-600">{formatMoney(payment.pendingRefundAmountCents, payment.currency)}</dd>
                </div>
                <div className="flex justify-between items-center py-2 border-b border-gray-100">
                  <dt className="text-sm text-gray-500">Зарезервировано</dt>
                  <dd className="text-sm font-medium text-gray-500">{formatMoney(payment.reservedRefundAmountCents, payment.currency)}</dd>
                </div>
                <div className="flex justify-between items-center py-3 border-b border-gray-200">
                  <dt className="text-base font-medium text-gray-700">Нетто сумма (выручка)</dt>
                  <dd className="text-lg font-bold text-gray-900">{formatMoney(payment.netAmountCents, payment.currency)}</dd>
                </div>
                <div className="flex justify-between items-center pt-2">
                  <dt className="text-sm font-medium text-gray-500">Доступно к возврату</dt>
                  <dd className="text-sm font-bold text-green-600">{formatMoney(payment.availableToRefundCents, payment.currency)}</dd>
                </div>
              </dl>
            </div>
          </div>

          {/* Order & Customer */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 bg-gray-50">
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Заказ и Клиент</h2>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <dt className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-1">Заказ</dt>
                <dd>
                  <Link to={`/orders/${order.orderId}`} className="text-indigo-600 hover:text-indigo-900 font-medium text-lg flex items-center group">
                    {order.orderNumber}
                  </Link>
                  <div className="text-sm text-gray-500 mt-1">
                    Сумма заказа: {formatMoney(order.orderTotalCents, payment.currency)}
                  </div>
                  <div className="text-sm text-gray-500">
                    Статус: <span className="font-medium text-gray-700">{order.orderStatus}</span>
                  </div>
                </dd>
              </div>
              <div className="pt-4 border-t border-gray-100">
                <dt className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2">Покупатель</dt>
                {order.customer ? (
                  <dd className="space-y-1">
                    <div className="text-sm font-medium text-gray-900">{order.customer.name}</div>
                    <div className="text-sm text-gray-500">{order.customer.email}</div>
                    <div className="text-sm text-gray-500">{order.customer.phone}</div>
                  </dd>
                ) : (
                  <dd className="text-sm text-gray-500 italic">Неизвестен</dd>
                )}
              </div>
            </div>
          </div>

          {/* Technical Info */}
          <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
            <button
              onClick={() => setShowTechInfo(!showTechInfo)}
              className="w-full px-6 py-4 bg-gray-50 hover:bg-gray-100 flex items-center justify-between transition-colors focus:outline-none"
            >
              <h2 className="text-sm font-semibold text-gray-900 uppercase tracking-wider">Техническая информация</h2>
              {showTechInfo ? <ChevronUp className="w-4 h-4 text-gray-500" /> : <ChevronDown className="w-4 h-4 text-gray-500" />}
            </button>
            
            {showTechInfo && (
              <div className="p-6 border-t border-gray-100 bg-gray-50 space-y-4">
                <div>
                  <dt className="text-xs font-medium text-gray-500 mb-1">ID Платежа (UUID)</dt>
                  <dd className="flex items-center">
                    <code className="text-xs text-gray-800 bg-white px-2 py-1 rounded border border-gray-200 flex-1">{payment.paymentId}</code>
                    <button onClick={() => copyToClipboard(payment.paymentId, 'paymentId')} className="ml-2 p-1.5 text-gray-400 hover:text-gray-600 rounded">
                      {copiedId === 'paymentId' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                    </button>
                  </dd>
                </div>
                <div>
                  <dt className="text-xs font-medium text-gray-500 mb-1">ID Заказа (UUID)</dt>
                  <dd className="flex items-center">
                    <code className="text-xs text-gray-800 bg-white px-2 py-1 rounded border border-gray-200 flex-1">{order.orderId}</code>
                    <button onClick={() => copyToClipboard(order.orderId, 'orderId')} className="ml-2 p-1.5 text-gray-400 hover:text-gray-600 rounded">
                      {copiedId === 'orderId' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                    </button>
                  </dd>
                </div>
                {order.customer && (
                  <div>
                    <dt className="text-xs font-medium text-gray-500 mb-1">ID Покупателя (UUID)</dt>
                    <dd className="flex items-center">
                      <code className="text-xs text-gray-800 bg-white px-2 py-1 rounded border border-gray-200 flex-1">{order.customer?.id}</code>
                      <button onClick={() => copyToClipboard(order.customer?.id || '', 'customerId')} className="ml-2 p-1.5 text-gray-400 hover:text-gray-600 rounded">
                        {copiedId === 'customerId' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                      </button>
                    </dd>
                  </div>
                )}
              </div>
            )}
          </div>

        </div>
      </div>
    </div>
  );
}

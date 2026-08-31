import { useEffect, useState, useRef, useCallback } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Clock,
  ExternalLink,
  FileText,
  Image as ImageIcon,
  Package,
  RotateCcw,
  User,
  X,
  XCircle,
  MessageSquare,

} from 'lucide-react';
import { SellerContextBanner } from '../components/SellerContextBanner';
import {
  approveAdminReturn,
  getAdminReturn,
  getAdminReturnErrorMessage,
  getAdminReturns,
  getAdminReturnRefundQuote,
  createAdminRefundForReturn,
  getReturnReasonLabel,
  getReturnStatusLabel,
  getStatusBadgeClass,
  rejectAdminReturn,
  formatReturnShipmentStatus,
  formatReturnShipmentMethod,
} from '../api/adminReturns';
import { ReturnConversationDrawer } from '../components/returns/ReturnConversationDrawer';
import type { AdminReturn, AdminReturnItem, AdminReturnRefundQuote } from '../api/adminReturns';
import { PermissionGuard } from '../components/PermissionGuard';
import { getAdminReturnTimeline } from '../api/adminTimeline';
import { EntityTimeline } from '../components/EntityTimeline';


export function AdminReturns() {
  const [returns, setReturns] = useState<AdminReturn[]>([]);
  const [selectedReturn, setSelectedReturn] = useState<AdminReturn | null>(null);
  const [refundQuote, setRefundQuote] = useState<AdminReturnRefundQuote | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isQuoteLoading, setIsQuoteLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Modals state
  const [isApproveModalOpen, setIsApproveModalOpen] = useState(false);
  const [isRejectModalOpen, setIsRejectModalOpen] = useState(false);
  const [isRefundModalOpen, setIsRefundModalOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [refundReason, setRefundReason] = useState('');
  const [previewImageUrl, setPreviewImageUrl] = useState<string | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [timelineRefreshKey, setTimelineRefreshKey] = useState(0);

  const [searchParams, setSearchParams] = useSearchParams();
  const sellerId = searchParams.get('sellerId');
  const urlReturnId = searchParams.get('id');
  const initialReturnIdHandledRef = useRef<string | null>(null);

  const fetchReturns = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminReturns();
      setReturns(data || []);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить возвраты.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchRefundQuote = async (id: string) => {
    try {
      setIsQuoteLoading(true);
      const quote = await getAdminReturnRefundQuote(id);
      setRefundQuote(quote);
    } catch {
      setRefundQuote(null);
    } finally {
      setIsQuoteLoading(false);
    }
  };

  const fetchReturnDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      const detail = await getAdminReturn(id);
      setSelectedReturn(detail);
      await fetchRefundQuote(id);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить детали возврата.'));
      setSelectedReturn(null);
      setRefundQuote(null);
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchReturns();
  }, []);

  // Handle URL return ID context handoff from Global Search
  useEffect(() => {
    if (urlReturnId && initialReturnIdHandledRef.current !== urlReturnId) {
      initialReturnIdHandledRef.current = urlReturnId;
      fetchReturnDetail(urlReturnId);
    }
  }, [urlReturnId]);

  const handleBackToList = () => {
    setSelectedReturn(null);
    setRefundQuote(null);
    if (searchParams.has('id') || searchParams.has('orderNumber')) {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.delete('id');
      nextParams.delete('orderNumber');
      setSearchParams(nextParams, { replace: true });
    }
  };

  const handleApprove = async () => {
    if (!selectedReturn) return;
    try {
      setIsSubmitting(true);
      setError(null);
      setSuccess(null);
      await approveAdminReturn(selectedReturn.id);
      await fetchReturns();
      await fetchReturnDetail(selectedReturn.id);
      setIsApproveModalOpen(false);
      setSuccess('Возврат успешно одобрен.');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось одобрить возврат.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleReject = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedReturn || !rejectReason.trim()) return;
    try {
      setIsSubmitting(true);
      setError(null);
      setSuccess(null);
      await rejectAdminReturn(selectedReturn.id, rejectReason.trim());
      await fetchReturns();
      await fetchReturnDetail(selectedReturn.id);
      setIsRejectModalOpen(false);
      setRejectReason('');
      setSuccess('Заявка на возврат отклонена.');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось отклонить возврат.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRefund = async () => {
    if (!selectedReturn) return;
    try {
      setIsSubmitting(true);
      setError(null);
      setSuccess(null);
      await createAdminRefundForReturn(selectedReturn.id, refundReason.trim() || undefined);
      setIsRefundModalOpen(false);
      setRefundReason('');
      setSuccess('Возврат средств поставлен в обработку.');
      await fetchReturns();
      await fetchReturnDetail(selectedReturn.id);
      setTimelineRefreshKey((k) => k + 1);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось запустить возврат средств.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatDate = (value?: string) => {
    if (!value) return '—';
    const d = new Date(value);
    return d.toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatPrice = (cents?: number) => {
    if (cents === undefined || cents === null) return '—';
    return (cents / 100).toLocaleString('ru-RU', {
      style: 'currency',
      currency: 'RUB',
      maximumFractionDigits: 0,
    });
  };

  // Stable fetcher for EntityTimeline — re-created only when the selected return changes
  const returnTimelineFetcher = useCallback(
    () => getAdminReturnTimeline(selectedReturn!.id),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedReturn?.id],
  );

  const filteredReturns = sellerId
    ? returns.filter((r) => r.sellerId === sellerId)
    : returns;

  // Flatten evidence across all items in selected return
  const allEvidence = (selectedReturn?.items || []).flatMap((item) => item.evidence || []);

  return (
    <PermissionGuard permission="returns.read">
      <div className="space-y-6">
        <SellerContextBanner />

        {error && (
          <div className="p-4 bg-red-50 text-red-700 rounded-lg flex items-center border border-red-200 shadow-sm">
            <AlertCircle className="h-5 w-5 mr-2 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}
        {success && (
          <div className="p-4 bg-green-50 text-green-700 rounded-lg flex items-center border border-green-200 shadow-sm">
            <CheckCircle2 className="h-5 w-5 mr-2 flex-shrink-0" />
            <span>{success}</span>
          </div>
        )}

        {/* ------------------------------------------------------------------ */}
        {/* DOSSIER / CLAIM REVIEW VIEW                                        */}
        {/* ------------------------------------------------------------------ */}
        {selectedReturn ? (
          <div className="space-y-6">
            {/* Dossier Header */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between bg-white p-5 rounded-xl border border-gray-200 shadow-sm gap-4">
              <div className="flex items-center space-x-4">
                <button
                  type="button"
                  onClick={handleBackToList}
                  className="inline-flex items-center text-sm font-medium text-gray-600 hover:text-gray-900 bg-gray-100 hover:bg-gray-200 px-3 py-1.5 rounded-lg transition-colors"
                >
                  <ArrowLeft className="h-4 w-4 mr-1.5" />
                  Назад к списку
                </button>
                <div>
                  <div className="flex items-center space-x-3">
                    <h1 className="text-xl font-bold text-gray-900">Заявка на возврат</h1>
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${getStatusBadgeClass(
                        selectedReturn.status
                      )}`}
                    >
                      {getReturnStatusLabel(selectedReturn.status)}
                    </span>
                  </div>
                  <p className="text-sm text-gray-500 mt-0.5">
                    Заказ{' '}
                    <span className="font-semibold text-gray-700">
                      {selectedReturn.orderNumber || selectedReturn.orderId}
                    </span>
                  </p>
                </div>
              </div>
              {isDetailLoading && <span className="text-sm text-gray-500">Обновление...</span>}
            </div>

            {/* Dossier Body: 2 Columns */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              {/* Left Column: Product, Claim & Evidence (2 cols wide on desktop) */}
              <div className="lg:col-span-2 space-y-6">
                {/* A. Product Card */}
                <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center space-x-2 border-b border-gray-100 pb-3">
                    <Package className="h-5 w-5 text-gray-500" />
                    <h2 className="text-base font-semibold text-gray-900">Товары к возврату</h2>
                  </div>

                  {!selectedReturn.items || selectedReturn.items.length === 0 ? (
                    <p className="text-sm text-gray-500">Нет позиций</p>
                  ) : (
                    <div className="divide-y divide-gray-100">
                      {selectedReturn.items.map((item: AdminReturnItem) => (
                        <div key={item.id} className="py-3 first:pt-0 last:pb-0 flex items-start space-x-4">
                          <div className="w-16 h-16 rounded-lg bg-gray-100 border border-gray-200 overflow-hidden flex-shrink-0 flex items-center justify-center">
                            {item.productImageUrl ? (
                              <img
                                src={item.productImageUrl}
                                alt={item.productTitle || ''}
                                className="w-full h-full object-cover"
                              />
                            ) : (
                              <ImageIcon className="h-6 w-6 text-gray-400" />
                            )}
                          </div>
                          <div className="flex-1 min-w-0">
                            <h3 className="text-sm font-semibold text-gray-900 truncate">
                              {item.productTitle || 'Товар'}
                            </h3>
                            <div className="text-xs text-gray-500 mt-1 flex flex-wrap gap-2">
                              {item.variantSize && <span>Размер: <span className="font-medium text-gray-700">{item.variantSize}</span></span>}
                              {item.variantColor && <span>Цвет: <span className="font-medium text-gray-700">{item.variantColor}</span></span>}
                              {item.sku && <span>Артикул: <span className="font-mono text-gray-700">{item.sku}</span></span>}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">
                              Количество: <span className="font-medium text-gray-900">{item.quantity} шт.</span>
                              {item.priceCents > 0 && (
                                <span className="ml-3">
                                  Цена: <span className="font-medium text-gray-900">{formatPrice(item.priceCents)}</span>
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  {selectedReturn.sellerName && (
                    <div className="pt-2 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
                      <span>Продавец: <span className="font-medium text-gray-700">{selectedReturn.sellerName}</span></span>
                    </div>
                  )}
                </div>

                {/* B. Financial Refund Card — Возврат средств */}
                <div data-testid="return-refund-card" className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center justify-between border-b border-gray-100 pb-3">
                    <div className="flex items-center space-x-2">
                      <RotateCcw className="h-5 w-5 text-gray-500" />
                      <h2 className="text-base font-semibold text-gray-900">Возврат средств</h2>
                    </div>
                    {refundQuote && (() => {
                      const isSucceeded = selectedReturn.status === 'refunded' || refundQuote.latestRefundStatus === 'succeeded';
                      const isPending = refundQuote.latestRefundStatus === 'pending';
                      const isAvailable = refundQuote.canRefund && refundQuote.remainingRefundableCents > 0;
                      return (
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                          isSucceeded
                            ? 'bg-green-50 text-green-800 border border-green-200'
                            : isPending
                            ? 'bg-amber-50 text-amber-800 border border-amber-200'
                            : isAvailable
                            ? 'bg-blue-50 text-blue-800 border border-blue-200'
                            : 'bg-gray-50 text-gray-700 border border-gray-200'
                        }`}>
                          {isSucceeded
                            ? 'Выполнен'
                            : isPending
                            ? 'Обрабатывается'
                            : isAvailable
                            ? 'Доступен'
                            : 'Недоступен'}
                        </span>
                      );
                    })()}
                  </div>

                  {isQuoteLoading ? (
                    <div className="py-6 text-center text-sm text-gray-400">
                      <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-indigo-600 mx-auto mb-2" />
                      Загрузка расчета возврата средств...
                    </div>
                  ) : !refundQuote ? (
                    <div className="py-4 text-sm text-gray-500">Информация о возврате средств недоступна</div>
                  ) : (
                    <div className="space-y-4">
                      {/* Items table */}
                      <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-gray-100 text-xs">
                          <thead>
                            <tr className="text-gray-500 text-left">
                              <th className="py-2 pr-3 font-medium">Товар</th>
                              <th className="py-2 px-3 font-medium text-center">Запрошено</th>
                              <th className="py-2 px-3 font-medium text-center">К возврату</th>
                              <th className="py-2 px-3 font-medium text-right">Цена за шт.</th>
                              <th className="py-2 pl-3 font-medium text-right">Сумма возврата</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-gray-100">
                            {refundQuote.items.map((item) => (
                              <tr key={item.orderItemId} className="text-gray-900">
                                <td className="py-2.5 pr-3">
                                  <div className="font-medium text-gray-900">{item.productTitle}</div>
                                  <div className="text-[11px] text-gray-500 mt-0.5">
                                    {item.mode === 'serialized' ? (
                                      <span className="inline-flex items-center text-purple-700 bg-purple-50 px-1.5 py-0.5 rounded font-medium">Поштучный учёт</span>
                                    ) : item.mode === 'legacy' ? (
                                      <span className="inline-flex items-center text-gray-600 bg-gray-100 px-1.5 py-0.5 rounded font-medium">Количественный учёт</span>
                                    ) : null}
                                  </div>
                                </td>
                                <td className="py-2.5 px-3 text-center text-gray-700 font-medium whitespace-nowrap">{item.requestedQuantity} шт.</td>
                                <td className="py-2.5 px-3 text-center font-semibold whitespace-nowrap">
                                  <span className={item.refundableQuantity > 0 ? 'text-green-700' : 'text-gray-400'}>
                                    {item.refundableQuantity} шт.
                                  </span>
                                </td>
                                <td className="py-2.5 px-3 text-right text-gray-700 whitespace-nowrap">{formatPrice(item.unitPriceCents)}</td>
                                <td className="py-2.5 pl-3 text-right font-semibold text-gray-900 whitespace-nowrap">{formatPrice(item.refundCents)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>

                      {/* Totals breakdown */}
                      <div className="bg-gray-50 rounded-lg p-3.5 border border-gray-100 text-xs space-y-1.5">
                        <div className="flex justify-between text-gray-600">
                          <span>Товары:</span>
                          <span className="font-medium text-gray-900">{formatPrice(refundQuote.productsRefundCents)}</span>
                        </div>
                        <div className="flex justify-between text-gray-600">
                          <span>Доставка:</span>
                          <span className="font-medium text-gray-900">{formatPrice(refundQuote.deliveryRefundCents)}</span>
                        </div>
                        {refundQuote.alreadyRefundedCents > 0 && (
                          <div className="flex justify-between text-gray-600">
                            <span>Ранее возвращено:</span>
                            <span className="font-medium text-gray-700">{formatPrice(refundQuote.alreadyRefundedCents)}</span>
                          </div>
                        )}
                        <div className="flex justify-between pt-1.5 border-t border-gray-200 text-sm font-semibold text-gray-900">
                          <span>Итого к возврату:</span>
                          <span>{formatPrice(refundQuote.totalRefundCents)}</span>
                        </div>
                      </div>

                      {/* Status States & Action */}
                      {selectedReturn.status === 'refunded' || refundQuote.latestRefundStatus === 'succeeded' ? (
                        <div className="p-3.5 bg-green-50 border border-green-200 rounded-lg text-xs text-green-900 space-y-1">
                          <div className="flex items-center space-x-2 font-semibold">
                            <CheckCircle2 className="h-4 w-4 text-green-600 flex-shrink-0" />
                            <span>Возврат средств выполнен</span>
                          </div>
                          <p className="text-green-700 pl-6">
                            Возврат средств выполнен.
                          </p>
                        </div>
                      ) : refundQuote.latestRefundStatus === 'pending' ? (
                        <div className="p-3.5 bg-amber-50 border border-amber-200 rounded-lg text-xs text-amber-900 space-y-1">
                          <div className="flex items-center space-x-2 font-semibold">
                            <Clock className="h-4 w-4 text-amber-600 flex-shrink-0" />
                            <span>Возврат средств обрабатывается</span>
                          </div>
                          <p className="text-amber-700 pl-6">
                            Возврат зарегистрирован и ожидает обработки платежной системой.
                          </p>
                        </div>
                      ) : refundQuote.canRefund && refundQuote.remainingRefundableCents > 0 ? (
                        <PermissionGuard
                          permission="refunds.create"
                          fallback={<p className="text-xs text-gray-500">У вас нет прав для создания возвратов средств.</p>}
                        >
                          <button
                            type="button"
                            onClick={() => setIsRefundModalOpen(true)}
                            className="w-full inline-flex items-center justify-center px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold rounded-lg shadow-sm transition-colors"
                          >
                            <RotateCcw className="h-4 w-4 mr-2" />
                            {refundQuote.latestRefundStatus === 'failed' ? 'Повторить возврат средств' : 'Запустить возврат средств'}
                          </button>
                        </PermissionGuard>
                      ) : (
                        <div className="p-3.5 bg-gray-50 border border-gray-200 rounded-lg text-xs text-gray-700 flex items-start space-x-2">
                          <AlertCircle className="h-4 w-4 text-gray-400 mt-0.5 flex-shrink-0" />
                          <div>
                            <span className="font-semibold block text-gray-900">Возврат средств недоступен</span>
                            <span className="text-gray-600 mt-0.5 block">{refundQuote.blockingReason || 'Условия возврата средств не выполнены.'}</span>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* C. Customer Claim Details */}
                <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center space-x-2 border-b border-gray-100 pb-3">
                    <FileText className="h-5 w-5 text-gray-500" />
                    <h2 className="text-base font-semibold text-gray-900">Претензия покупателя</h2>
                  </div>

                  <div>
                    <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">Причина возврата</span>
                    <p className="mt-1 text-base font-semibold text-gray-900">
                      {getReturnReasonLabel(selectedReturn.reason)}
                    </p>
                  </div>

                  <div>
                    <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">Комментарий покупателя</span>
                    <div className="mt-1.5 p-3.5 bg-gray-50 rounded-lg border border-gray-200 text-sm text-gray-800 whitespace-pre-wrap leading-relaxed">
                      {selectedReturn.comment || <span className="text-gray-400 italic">Комментарий не указан</span>}
                    </div>
                  </div>
                </div>

                {/* D. Evidence Photos Gallery */}
                <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center justify-between border-b border-gray-100 pb-3">
                    <div className="flex items-center space-x-2">
                      <ImageIcon className="h-5 w-5 text-gray-500" />
                      <h2 className="text-base font-semibold text-gray-900">Фотографии покупателя</h2>
                    </div>
                    <span className="text-xs text-gray-500 font-medium">
                      {allEvidence.length} {allEvidence.length === 1 ? 'фотография' : 'фото'}
                    </span>
                  </div>

                  {allEvidence.length === 0 ? (
                    <div className="py-6 text-center text-sm text-gray-400 bg-gray-50 rounded-lg border border-dashed border-gray-200">
                      Фотографии не приложены
                    </div>
                  ) : (
                    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                      {allEvidence.map((ev, index) => (
                        <button
                          key={ev.id}
                          type="button"
                          onClick={() => setPreviewImageUrl(ev.url)}
                          className="group relative aspect-square rounded-lg overflow-hidden border border-gray-200 bg-gray-100 hover:ring-2 hover:ring-indigo-500 focus:outline-none transition-all"
                        >
                          <img
                            src={ev.url}
                            alt={`Фото доказательства ${index + 1}`}
                            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
                          />
                          <div className="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 flex items-center justify-center text-white transition-opacity">
                            <ExternalLink className="h-5 w-5" />
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                </div>

                {/* E. Return Timeline — История возврата */}
                <div data-testid="return-timeline-card" className="bg-white rounded-xl border border-gray-200 shadow-sm p-6">
                  <EntityTimeline
                    key={timelineRefreshKey}
                    fetcher={returnTimelineFetcher}
                    title="История возврата"
                  />
                </div>
              </div>

              {/* Right Column: Order/Customer Context & Moderation Decisions */}
              <div className="space-y-6">
                <button
                  type="button"
                  onClick={() => setIsDrawerOpen(true)}
                  className="w-full inline-flex items-center justify-center px-4 py-3 bg-white hover:bg-gray-50 text-gray-900 text-sm font-semibold rounded-xl border border-gray-200 shadow-sm transition-colors"
                >
                  <MessageSquare className="h-5 w-5 mr-2 text-gray-500" />
                  Переписка с покупателем
                </button>

                {/* D. Order & Customer Context */}
                <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center space-x-2 border-b border-gray-100 pb-3">
                    <User className="h-5 w-5 text-gray-500" />
                    <h2 className="text-base font-semibold text-gray-900">Заказ и покупатель</h2>
                  </div>

                  <dl className="space-y-3 text-sm">
                    <div>
                      <dt className="text-xs text-gray-500">Номер заказа</dt>
                      <dd className="font-semibold text-gray-900">
                        {selectedReturn.orderNumber || selectedReturn.orderId}
                      </dd>
                    </div>

                    <div>
                      <dt className="text-xs text-gray-500">Покупатель</dt>
                      <dd className="font-medium text-gray-900">
                        {selectedReturn.customerName || 'Покупатель'}
                      </dd>
                    </div>

                    {selectedReturn.customerEmail && (
                      <div>
                        <dt className="text-xs text-gray-500">Email</dt>
                        <dd className="text-gray-800 font-mono text-xs">{selectedReturn.customerEmail}</dd>
                      </div>
                    )}

                    {selectedReturn.customerPhone && (
                      <div>
                        <dt className="text-xs text-gray-500">Телефон</dt>
                        <dd className="text-gray-800">{selectedReturn.customerPhone}</dd>
                      </div>
                    )}

                    {selectedReturn.deliveredAt && (
                      <div>
                        <dt className="text-xs text-gray-500">Дата доставки заказа</dt>
                        <dd className="text-gray-800">{formatDate(selectedReturn.deliveredAt)}</dd>
                      </div>
                    )}

                    <div>
                      <dt className="text-xs text-gray-500">Дата создания заявки</dt>
                      <dd className="text-gray-800">{formatDate(selectedReturn.createdAt)}</dd>
                    </div>
                  </dl>
                </div>

                {/* E. Moderation Actions / Status Box */}
                <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-4">
                  <div className="flex items-center space-x-2 border-b border-gray-100 pb-3">
                    <Clock className="h-5 w-5 text-gray-500" />
                    <h2 className="text-base font-semibold text-gray-900">Решение по заявке</h2>
                  </div>

                  {/* For requested status: Action Buttons */}
                  {selectedReturn.status === 'requested' && (
                    <PermissionGuard
                      permission="returns.update_status"
                      fallback={<p className="text-sm text-gray-500">У вас нет прав для модерации возвратов.</p>}
                    >
                      <div className="space-y-3 pt-1">
                        <p className="text-xs text-gray-500">
                          Ознакомьтесь с причиной и фотографиями перед принятием решения.
                        </p>

                        <button
                          type="button"
                          onClick={() => setIsApproveModalOpen(true)}
                          disabled={isSubmitting}
                          className="w-full inline-flex items-center justify-center px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold rounded-lg shadow-sm transition-colors disabled:opacity-50"
                        >
                          <CheckCircle2 className="h-4 w-4 mr-2" />
                          Одобрить возврат
                        </button>
                        <button
                          type="button"
                          onClick={() => setIsRejectModalOpen(true)}
                          disabled={isSubmitting}
                          className="w-full inline-flex items-center justify-center px-4 py-2.5 bg-white hover:bg-red-50 text-red-700 text-sm font-semibold rounded-lg border border-red-300 shadow-sm transition-colors disabled:opacity-50"
                        >
                          <XCircle className="h-4 w-4 mr-2" />
                          Отклонить заявку
                        </button>
                      </div>
                    </PermissionGuard>
                  )}

                  {/* Needs Info State */}
                  {selectedReturn.status === 'needs_info' && (
                    <div className="p-4 bg-yellow-50 border border-yellow-200 rounded-lg text-sm text-yellow-900 space-y-2">
                      <div className="flex items-center space-x-2 font-semibold text-yellow-800">
                        <MessageSquare className="h-5 w-5 text-yellow-600" />
                        <span>Ожидает ответа покупателя</span>
                      </div>
                      <p className="text-xs text-yellow-700 leading-relaxed">
                        Покупателю отправлен запрос на уточнение информации. Модерация возобновится после получения ответа.
                      </p>
                    </div>
                  )}

                  {/* Read-Only Status States */}
                  {selectedReturn.status === 'approved' && (
                    <div className="space-y-4">
                      <div className="p-4 bg-blue-50 border border-blue-200 rounded-lg text-sm text-blue-900 space-y-2">
                        <div className="flex items-center space-x-2 font-semibold">
                          <CheckCircle2 className="h-5 w-5 text-blue-600" />
                          <span>Возврат одобрен</span>
                        </div>
                        {selectedReturn.shipment ? (
                          selectedReturn.shipment.status === 'arrived_at_zamk' ? (
                            <p className="text-xs text-blue-700 font-medium">Возврат прибыл на склад</p>
                          ) : (
                            <p className="text-xs text-blue-700">Ожидает прибытия на склад (Статус: {formatReturnShipmentStatus(selectedReturn.shipment.status)})</p>
                          )
                        ) : (
                          <p className="text-xs text-blue-700">Ожидает выбора способа отправки покупателем</p>
                        )}
                        {selectedReturn.approvedAt && (
                          <p className="text-xs text-blue-500 pt-1">
                            Одобрено: {formatDate(selectedReturn.approvedAt)}
                          </p>
                        )}
                      </div>

                      {selectedReturn.shipment && (
                        <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg text-sm space-y-3">
                          <div className="font-semibold text-gray-900">Возвратная логистика</div>
                          <div className="grid grid-cols-2 gap-2 text-xs">
                            <span className="text-gray-500">Служба:</span>
                            <span className="font-medium">{selectedReturn.shipment.provider === 'cdek' ? 'СДЭК' : selectedReturn.shipment.provider}</span>
                            <span className="text-gray-500">Способ:</span>
                            <span className="font-medium">{formatReturnShipmentMethod(selectedReturn.shipment.method)}</span>
                            <span className="text-gray-500">Трек-номер:</span>
                            <span className="font-medium">{selectedReturn.shipment.trackingNumber || 'Ожидается'}</span>
                            <span className="text-gray-500">Статус:</span>
                            <span className="font-medium">{formatReturnShipmentStatus(selectedReturn.shipment.status)}</span>
                            {selectedReturn.shipment.selectedCdekOfficeCode && (
                              <>
                                <span className="text-gray-500">Отделение:</span>
                                <span className="font-medium">{selectedReturn.shipment.selectedCdekOfficeCode}</span>
                              </>
                            )}
                            {selectedReturn.shipment.customerName && (
                              <>
                                <span className="text-gray-500">Отправитель:</span>
                                <span className="font-medium">{selectedReturn.shipment.customerName}</span>
                              </>
                            )}
                          </div>

                          {selectedReturn.shipment.status === 'arrived_at_zamk' && (
                            <div className="pt-2">
                              <Link
                                to={`/returns/${selectedReturn.id}/receiving`}
                                className="w-full inline-flex justify-center items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                              >
                                Начать приёмку на складе
                              </Link>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  )}

                  {selectedReturn.status === 'rejected' && (
                    <div className="p-4 bg-red-50 border border-red-200 rounded-lg text-sm text-red-900 space-y-2">
                      <div className="flex items-center space-x-2 font-semibold">
                        <XCircle className="h-5 w-5 text-red-600" />
                        <span>Заявка отклонена</span>
                      </div>
                      {selectedReturn.adminComment && (
                        <div>
                          <span className="text-xs font-medium text-red-700 block">Причина отказа:</span>
                          <p className="text-xs text-red-800 mt-0.5 whitespace-pre-wrap">{selectedReturn.adminComment}</p>
                        </div>
                      )}
                      {selectedReturn.rejectedAt && (
                        <p className="text-xs text-red-500 pt-1">
                          Отклонено: {formatDate(selectedReturn.rejectedAt)}
                        </p>
                      )}
                    </div>
                  )}

                  {selectedReturn.status === 'receiving' && (
                    <div className="p-4 bg-purple-50 border border-purple-200 rounded-lg text-sm text-purple-900 space-y-1">
                      <div className="font-semibold">Идёт приёмка на складе</div>
                      <p className="text-xs text-purple-700">Товары сканируются на складе.</p>
                    </div>
                  )}

                  {selectedReturn.status === 'item_received' && (
                    <div className="p-4 bg-teal-50 border border-teal-200 rounded-lg text-sm text-teal-900 space-y-1">
                      <div className="font-semibold">Товар принят на складе</div>
                      <p className="text-xs text-teal-700">Приёмка завершена сотрудником склада.</p>
                    </div>
                  )}

                  {selectedReturn.status === 'refunded' && (
                    <div className="p-4 bg-green-50 border border-green-200 rounded-lg text-sm text-green-900 space-y-1">
                      <div className="font-semibold">Деньги возвращены</div>
                      <p className="text-xs text-green-700">Возмещение средств покупателю выполнено.</p>
                    </div>
                  )}

                  {selectedReturn.status === 'completed' && (
                    <div className="p-4 bg-green-50 border border-green-200 rounded-lg text-sm text-green-900 space-y-1">
                      <div className="font-semibold">Возврат завершён</div>
                      <p className="text-xs text-green-700">Все операции по возврату успешно закрыты.</p>
                      {selectedReturn.completedAt && (
                        <p className="text-xs text-green-600 pt-1">
                          Завершено: {formatDate(selectedReturn.completedAt)}
                        </p>
                      )}
                    </div>
                  )}

                  {selectedReturn.status === 'cancelled' && (
                    <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-800 space-y-1">
                      <div className="font-semibold">Заявка отменена</div>
                      <p className="text-xs text-gray-600">Заявка на возврат была отменена покупателем.</p>
                    </div>
                  )}
                </div>
              </div>
            </div>

            {selectedReturn && (
              <ReturnConversationDrawer
                returnId={selectedReturn.id}
                isOpen={isDrawerOpen}
                onClose={() => setIsDrawerOpen(false)}
                status={selectedReturn.status}
                onStatusChange={() => fetchReturnDetail(selectedReturn.id)}
              />
            )}
          </div>
        ) : (
          /* ------------------------------------------------------------------ */
          /* RETURNS LIST VIEW                                                  */
          /* ------------------------------------------------------------------ */
          <div className="space-y-4">
            <div className="sm:flex sm:items-center sm:justify-between">
              <div>
                <h1 className="text-2xl font-bold text-gray-900">Возвраты</h1>
                <p className="text-sm text-gray-500 mt-1">
                  Модерация претензий покупателей и контроль возвратов
                </p>
              </div>
            </div>

            {isLoading ? (
              <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
                <p className="mt-2 text-sm text-gray-500">Загрузка возвратов...</p>
              </div>
            ) : filteredReturns.length === 0 ? (
              <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
                <RotateCcw className="mx-auto h-12 w-12 text-gray-300" />
                <h3 className="mt-2 text-base font-semibold text-gray-900">Возвратов нет</h3>
                <p className="mt-1 text-sm text-gray-500">Заявок на возврат пока нет.</p>
              </div>
            ) : (
              <div className="bg-white shadow-sm border border-gray-200 rounded-xl overflow-hidden">
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Товар
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Заказ
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Покупатель
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Причина
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Материалы
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Создана
                        </th>
                        <th scope="col" className="px-6 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Статус
                        </th>
                        <th scope="col" className="relative px-6 py-3.5">
                          <span className="sr-only">Действия</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {filteredReturns.map((req) => {
                        const primaryItem = req.items?.[0];
                        return (
                          <tr key={req.id} className="hover:bg-gray-50 transition-colors">
                            {/* Product column with photo */}
                            <td className="px-6 py-4">
                              <div className="flex items-center space-x-3">
                                <div className="w-10 h-10 rounded-lg bg-gray-100 border border-gray-200 overflow-hidden flex-shrink-0 flex items-center justify-center">
                                  {primaryItem?.productImageUrl ? (
                                    <img
                                      src={primaryItem.productImageUrl}
                                      alt=""
                                      className="w-full h-full object-cover"
                                    />
                                  ) : (
                                    <Package className="h-5 w-5 text-gray-400" />
                                  )}
                                </div>
                                <div className="min-w-0 max-w-xs">
                                  <div className="text-sm font-semibold text-gray-900 truncate">
                                    {primaryItem?.productTitle || 'Товар'}
                                  </div>
                                  {(primaryItem?.variantSize || primaryItem?.variantColor) && (
                                    <div className="text-xs text-gray-500 truncate">
                                      {[primaryItem.variantSize, primaryItem.variantColor].filter(Boolean).join(' · ')}
                                    </div>
                                  )}
                                </div>
                              </div>
                            </td>

                            {/* Order column */}
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-semibold text-gray-900">
                              {req.orderNumber || req.orderId}
                            </td>

                            {/* Customer column */}
                            <td className="px-6 py-4 whitespace-nowrap">
                              <div className="text-sm font-medium text-gray-900">
                                {req.customerName || 'Покупатель'}
                              </div>
                              {req.customerEmail && (
                                <div className="text-xs text-gray-500">{req.customerEmail}</div>
                              )}
                            </td>

                            {/* Reason column */}
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-700">
                              {getReturnReasonLabel(req.reason)}
                            </td>

                            {/* Materials / Evidence column */}
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                              {(req.evidenceCount || 0) > 0 ? (
                                <span className="inline-flex items-center text-xs font-medium text-gray-700 bg-gray-100 px-2 py-0.5 rounded">
                                  <ImageIcon className="h-3.5 w-3.5 mr-1 text-gray-500" />
                                  {req.evidenceCount} фото
                                </span>
                              ) : (
                                '—'
                              )}
                            </td>

                            {/* Created date column */}
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                              {formatDate(req.createdAt)}
                            </td>

                            {/* Status badge column */}
                            <td className="px-6 py-4 whitespace-nowrap">
                              <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${getStatusBadgeClass(
                                  req.status
                                )}`}
                              >
                                {getReturnStatusLabel(req.status)}
                              </span>
                            </td>

                            {/* Action column */}
                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                              <button
                                type="button"
                                onClick={() => fetchReturnDetail(req.id)}
                                className="inline-flex items-center px-3 py-1.5 bg-indigo-50 hover:bg-indigo-100 text-indigo-700 text-xs font-semibold rounded-lg transition-colors"
                              >
                                Рассмотреть
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ------------------------------------------------------------------ */}
        {/* REFUND CONFIRMATION MODAL                                          */}
        {/* ------------------------------------------------------------------ */}
        {isRefundModalOpen && selectedReturn && refundQuote && (
          <div className="fixed inset-0 z-50 overflow-y-auto flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
            <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-gray-200 space-y-4">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center flex-shrink-0">
                  <RotateCcw className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="text-lg font-bold text-gray-900">
                    {refundQuote.latestRefundStatus === 'failed' ? 'Повторить возврат средств?' : 'Запустить возврат средств?'}
                  </h3>
                  <p className="text-xs text-gray-500">Заказ {selectedReturn.orderNumber || selectedReturn.orderId}</p>
                </div>
              </div>

              <div className="bg-gray-50 rounded-lg p-3.5 border border-gray-200 text-xs space-y-1.5">
                <div className="flex justify-between text-gray-600">
                  <span>К возврату (товары):</span>
                  <span className="font-medium text-gray-900">{formatPrice(refundQuote.productsRefundCents)}</span>
                </div>
                <div className="flex justify-between text-gray-600">
                  <span>Доставка:</span>
                  <span className="font-medium text-gray-900">{formatPrice(refundQuote.deliveryRefundCents)}</span>
                </div>
                <div className="flex justify-between pt-1.5 border-t border-gray-200 text-sm font-semibold text-gray-900">
                  <span>Итого к возврату:</span>
                  <span className="text-indigo-600">{formatPrice(refundQuote.totalRefundCents)}</span>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">
                  Причина возврата (опционально)
                </label>
                <input
                  type="text"
                  value={refundReason}
                  onChange={(e) => setRefundReason(e.target.value)}
                  placeholder="Например: брак при производстве"
                  className="w-full text-xs rounded-lg border border-gray-300 px-3 py-2 text-gray-900 placeholder-gray-400 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>

              <p className="text-xs text-gray-500 leading-relaxed">
                После подтверждения возврат будет поставлен в обработку.
              </p>

              <div className="pt-2 flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => {
                    setIsRefundModalOpen(false);
                    setRefundReason('');
                  }}
                  disabled={isSubmitting}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
                >
                  Отмена
                </button>
                <button
                  type="button"
                  onClick={handleRefund}
                  disabled={isSubmitting}
                  className="inline-flex items-center px-4 py-2 text-sm font-semibold text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg shadow-sm transition-colors disabled:opacity-50"
                >
                  {isSubmitting ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
                      Обработка...
                    </>
                  ) : refundQuote.latestRefundStatus === 'failed' ? (
                    'Повторить возврат'
                  ) : (
                    'Запустить возврат'
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* ------------------------------------------------------------------ */}
        {/* APPROVE CONFIRMATION MODAL                                         */}
        {/* ------------------------------------------------------------------ */}
        {isApproveModalOpen && selectedReturn && (
          <div className="fixed inset-0 z-50 overflow-y-auto flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
            <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-gray-200 space-y-4">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 rounded-full bg-green-100 text-green-600 flex items-center justify-center flex-shrink-0">
                  <CheckCircle2 className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="text-lg font-bold text-gray-900">Одобрить возврат?</h3>
                  <p className="text-xs text-gray-500">Заказ {selectedReturn.orderNumber || selectedReturn.orderId}</p>
                </div>
              </div>

              <p className="text-sm text-gray-600 leading-relaxed">
                После одобрения товар можно будет принять на складе.
              </p>

              <div className="pt-2 flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={() => setIsApproveModalOpen(false)}
                  disabled={isSubmitting}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
                >
                  Отмена
                </button>
                <button
                  type="button"
                  onClick={handleApprove}
                  disabled={isSubmitting}
                  className="px-4 py-2 text-sm font-semibold text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg shadow-sm transition-colors disabled:opacity-50"
                >
                  {isSubmitting ? 'Одобрение...' : 'Подтвердить одобрение'}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* ------------------------------------------------------------------ */}
        {/* REJECT MODAL                                                       */}
        {/* ------------------------------------------------------------------ */}
        {isRejectModalOpen && selectedReturn && (
          <div className="fixed inset-0 z-50 overflow-y-auto flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
            <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-gray-200 space-y-4">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 rounded-full bg-red-100 text-red-600 flex items-center justify-center flex-shrink-0">
                  <XCircle className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="text-lg font-bold text-gray-900">Отклонение заявки</h3>
                  <p className="text-xs text-gray-500">Заказ {selectedReturn.orderNumber || selectedReturn.orderId}</p>
                </div>
              </div>

              <form onSubmit={handleReject} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Причина отказа <span className="text-red-500">*</span>
                  </label>
                  <p className="text-xs text-gray-500 mb-2">
                    Укажите причину отказа. Она будет сохранена в системе.
                  </p>
                  <textarea
                    rows={3}
                    required
                    value={rejectReason}
                    onChange={(e) => setRejectReason(e.target.value)}
                    placeholder="Например: товар со следами носки, сорваны бирки..."
                    className="w-full rounded-lg border-gray-300 shadow-sm focus:border-red-500 focus:ring-red-500 text-sm"
                  />
                </div>

                <div className="pt-2 flex justify-end space-x-3">
                  <button
                    type="button"
                    onClick={() => {
                      setIsRejectModalOpen(false);
                      setRejectReason('');
                    }}
                    disabled={isSubmitting}
                    className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
                  >
                    Отмена
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmitting || !rejectReason.trim()}
                    className="px-4 py-2 text-sm font-semibold text-white bg-red-600 hover:bg-red-700 rounded-lg shadow-sm transition-colors disabled:opacity-50"
                  >
                    {isSubmitting ? 'Отклонение...' : 'Отклонить заявку'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}


        {/* ------------------------------------------------------------------ */}
        {/* LIGHTBOX IMAGE PREVIEW MODAL                                       */}
        {/* ------------------------------------------------------------------ */}
        {previewImageUrl && (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
            onClick={() => setPreviewImageUrl(null)}
          >
            <div className="relative max-w-4xl max-h-[90vh] bg-transparent flex flex-col items-center">
              <button
                type="button"
                onClick={() => setPreviewImageUrl(null)}
                className="absolute -top-10 right-0 text-white hover:text-gray-300 p-1 rounded-full bg-black/50"
              >
                <X className="h-6 w-6" />
              </button>
              <img
                src={previewImageUrl}
                alt="Просмотр доказательства"
                className="max-h-[85vh] max-w-full rounded-lg object-contain shadow-2xl"
                onClick={(e) => e.stopPropagation()}
              />
            </div>
          </div>
        )}
      </div>
    </PermissionGuard>
  );
}

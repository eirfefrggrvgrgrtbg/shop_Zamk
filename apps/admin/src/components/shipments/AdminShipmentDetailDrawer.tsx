import React, { useEffect, useState, useCallback } from 'react';
import { createPortal } from 'react-dom';
import {
  X,
  Check,
  Copy,
  ExternalLink,
  Clock,
  CheckCircle2,
  AlertCircle,
  Package,
  Truck,
  ChevronDown,
  RefreshCw,
} from 'lucide-react';
import {
  getAdminShipment,
  getShipmentWorkspaceStatusLabel,
  type AdminShipmentView,
} from '../../api/adminShipments';
import { getDisplaySellerName, formatPositionsCount, formatUnitsCount } from '../../pages/AdminShipments';
import { formatOrderNumber } from '../../utils/orderFormatters';

export interface AdminShipmentDetailDrawerProps {
  shipmentId: string | null;
  isOpen: boolean;
  onClose: () => void;
}

export function isSafeExternalUrl(url?: string | null): url is string {
  if (!url) return false;
  return /^https?:\/\//i.test(url.trim());
}

export const formatDate = (value?: string | null): string => {
  if (!value) return 'Не указано';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return 'Не указано';
  return parsed.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

export const getStatusBadgeClass = (status: string): string => {
  switch (status) {
    case 'shipped':
      return 'bg-indigo-50 text-indigo-700 border-indigo-200';
    case 'delivered':
      return 'bg-emerald-50 text-emerald-800 border-emerald-200';
    case 'failed':
      return 'bg-rose-50 text-rose-800 border-rose-200';
    case 'cancelled':
      return 'bg-gray-100 text-gray-700 border-gray-200';
    default:
      return 'bg-gray-100 text-gray-700 border-gray-200';
  }
};

export const getCurrentStateSummary = (status: string): { text: string; icon: React.ReactNode } => {
  switch (status) {
    case 'shipped':
      return {
        text: 'Отправление передано в доставку',
        icon: <Truck className="w-5 h-5 text-indigo-600 shrink-0" />,
      };
    case 'delivered':
      return {
        text: 'Отправление доставлено',
        icon: <CheckCircle2 className="w-5 h-5 text-emerald-600 shrink-0" />,
      };
    case 'failed':
      return {
        text: 'Доставка не завершена',
        icon: <AlertCircle className="w-5 h-5 text-rose-600 shrink-0" />,
      };
    case 'cancelled':
      return {
        text: 'Отправка отменена',
        icon: <AlertCircle className="w-5 h-5 text-gray-500 shrink-0" />,
      };
    default:
      return {
        text: getShipmentWorkspaceStatusLabel(status),
        icon: <Package className="w-5 h-5 text-gray-600 shrink-0" />,
      };
  }
};

export const AdminShipmentDetailDrawer: React.FC<AdminShipmentDetailDrawerProps> = ({
  shipmentId,
  isOpen,
  onClose,
}) => {
  const [detail, setDetail] = useState<AdminShipmentView | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const fetchDetail = useCallback(async () => {
    if (!shipmentId) return;
    try {
      setIsLoading(true);
      setError(null);
      const res = await getAdminShipment(shipmentId);
      setDetail(res);
    } catch {
      setError('Не удалось загрузить данные отправления');
    } finally {
      setIsLoading(false);
    }
  }, [shipmentId]);

  useEffect(() => {
    if (isOpen && shipmentId) {
      fetchDetail();
    } else {
      setDetail(null);
      setError(null);
      setIsLoading(false);
    }
  }, [isOpen, shipmentId, fetchDetail]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => {
      setCopiedKey((curr) => (curr === key ? null : curr));
    }, 1500);
  };

  if (!isOpen || !shipmentId) return null;

  const orderNumber = detail
    ? formatOrderNumber({ id: detail.orderId, orderNumber: detail.orderNumber })
    : '—';
  const sellerName = detail ? getDisplaySellerName(detail.sellerName) : 'Продавец';
  const statusLabel = detail ? getShipmentWorkspaceStatusLabel(detail.status) : '';
  const statusBadge = detail ? getStatusBadgeClass(detail.status) : '';
  const currentState = detail ? getCurrentStateSummary(detail.status) : null;

  const items = detail?.items || [];
  const itemsCount = detail?.itemsCount ?? items.length;
  const unitsCount =
    detail?.unitsCount ?? items.reduce((sum, item) => sum + (item.quantity || 0), 0);

  const carrierDisplay = detail?.carrier?.trim() || 'Не указана';
  const trackingDisplay = detail?.trackingNumber?.trim() || 'Не указан';
  const deliveryMethodDisplay = detail?.deliveryMethodName?.trim() || 'Не указан';

  const drawerContent = (
    <>
      {/* Backdrop */}
      <div
        data-testid="shipment-drawer-backdrop"
        className="fixed inset-0 bg-slate-900/40 backdrop-blur-2xs z-[60] transition-opacity"
        onClick={onClose}
      />

      {/* Drawer */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="shipment-drawer-title"
        data-testid="shipment-detail-drawer"
        className="fixed top-0 right-0 bottom-0 w-full max-w-xl bg-white shadow-2xl z-[70] flex flex-col border-l border-gray-200 min-w-0"
      >
        {/* A. HEADER */}
        <div className="px-6 py-5 border-b border-gray-200 flex items-start justify-between gap-4 bg-white shrink-0">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-3 flex-wrap">
              <h2
                id="shipment-drawer-title"
                className="text-xl font-black text-gray-900 tracking-tight"
              >
                {orderNumber}
              </h2>
              {detail && (
                <span
                  data-testid="shipment-drawer-status-badge"
                  className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border ${statusBadge}`}
                >
                  {statusLabel}
                </span>
              )}
            </div>
            <p className="text-xs text-gray-500 font-medium mt-1">{sellerName}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            data-testid="shipment-drawer-close"
            aria-label="Закрыть"
            className="text-gray-400 hover:text-gray-600 p-2 rounded-xl hover:bg-gray-100 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* BODY */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {isLoading ? (
            <div
              data-testid="shipment-drawer-loading"
              className="py-16 flex flex-col items-center justify-center text-center space-y-2"
            >
              <RefreshCw className="w-7 h-7 animate-spin text-indigo-600" />
              <p className="text-xs font-medium text-gray-500">Загрузка данных отправления...</p>
            </div>
          ) : error ? (
            <div
              data-testid="shipment-drawer-error"
              className="py-12 flex flex-col items-center justify-center text-center space-y-3"
            >
              <AlertCircle className="w-9 h-9 text-rose-500" />
              <div className="space-y-1">
                <h4 className="text-sm font-bold text-gray-900">{error}</h4>
                <p className="text-xs text-gray-500 max-w-xs">
                  Проверьте подключение к серверу и попробуйте снова.
                </p>
              </div>
              <button
                type="button"
                onClick={fetchDetail}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-indigo-50 hover:bg-indigo-100 text-indigo-700 text-xs font-bold rounded-xl transition-colors"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Повторить попытку</span>
              </button>
            </div>
          ) : detail ? (
            <>
              {/* B. CURRENT STATE */}
              {currentState && (
                <div
                  data-testid="shipment-drawer-current-state"
                  className="p-4 rounded-xl bg-gray-50 border border-gray-200/80 flex items-center gap-3"
                >
                  {currentState.icon}
                  <div>
                    <div className="text-sm font-bold text-gray-900">{currentState.text}</div>
                  </div>
                </div>
              )}

              {/* C. DELIVERY */}
              <div data-testid="shipment-drawer-delivery" className="space-y-3">
                <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
                  Доставка
                </h3>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 p-4 bg-gray-50 rounded-xl border border-gray-100 text-xs">
                  <div>
                    <dt className="text-gray-400 font-medium">Способ доставки</dt>
                    <dd className="font-semibold text-gray-900 mt-0.5">{deliveryMethodDisplay}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-400 font-medium">Служба доставки</dt>
                    <dd className="font-semibold text-gray-900 mt-0.5">{carrierDisplay}</dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-gray-400 font-medium">Трек-номер</dt>
                    <dd className="font-mono font-semibold text-gray-900 mt-0.5 flex items-center gap-2 flex-wrap">
                      <span>{trackingDisplay}</span>
                      {isSafeExternalUrl(detail.trackingUrl) && (
                        <a
                          href={detail.trackingUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-[11px] font-sans font-semibold text-indigo-600 hover:text-indigo-800 transition-colors"
                        >
                          <span>Отследить</span>
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-gray-400 font-medium">Упаковано</dt>
                    <dd className="text-gray-700 mt-0.5">{formatDate(detail.packedAt)}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-400 font-medium">Передано в доставку</dt>
                    <dd className="text-gray-700 mt-0.5">{formatDate(detail.shippedAt)}</dd>
                  </div>
                  {detail.status === 'delivered' || detail.deliveredAt ? (
                    <div>
                      <dt className="text-gray-400 font-medium">Доставлено</dt>
                      <dd className="font-semibold text-emerald-800 mt-0.5">
                        {formatDate(detail.deliveredAt)}
                      </dd>
                    </div>
                  ) : null}
                </dl>
              </div>

              {/* D. CONTENTS */}
              <div data-testid="shipment-drawer-contents" className="space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
                    Состав отправления
                  </h3>
                  <span className="text-xs font-semibold text-gray-500">
                    {formatPositionsCount(itemsCount)} · {formatUnitsCount(unitsCount)}
                  </span>
                </div>

                {items.length === 0 ? (
                  <div className="p-4 text-center rounded-xl bg-gray-50 border border-gray-100 text-xs text-gray-500">
                    Состав не указан
                  </div>
                ) : (
                  <div className="space-y-2">
                    {items.map((item, idx) => {
                      const color = item.variantColor?.trim();
                      const size = item.variantSize?.trim();
                      const variantDetails = [color, size].filter(Boolean).join(' · ');

                      return (
                        <div
                          key={item.orderItemId || `${item.productId}-${idx}`}
                          className="flex items-center gap-3.5 p-3 rounded-xl bg-gray-50/70 border border-gray-100"
                        >
                          <div className="w-12 h-12 rounded-lg bg-white flex items-center justify-center overflow-hidden shrink-0 border border-gray-200">
                            {item.imageUrl ? (
                              <img
                                src={item.imageUrl}
                                alt={item.productTitle}
                                className="w-full h-full object-cover"
                              />
                            ) : (
                              <Package className="w-5 h-5 text-gray-400" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="text-xs sm:text-sm font-bold text-gray-900 truncate">
                              {item.productTitle}
                            </div>
                            {variantDetails ? (
                              <div className="text-xs text-gray-500 mt-0.5">{variantDetails}</div>
                            ) : null}
                            <div className="text-xs font-semibold text-gray-700 mt-1">
                              {item.quantity} шт.
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* E. RECIPIENT */}
              <div data-testid="shipment-drawer-recipient" className="space-y-3">
                <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
                  Получатель
                </h3>
                <dl className="space-y-2.5 p-4 bg-gray-50 rounded-xl border border-gray-100 text-xs">
                  <div>
                    <dt className="text-gray-400 font-medium">Получатель</dt>
                    <dd className="font-semibold text-gray-900 mt-0.5">
                      {detail.customerName?.trim() || 'Не указан'}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-gray-400 font-medium">Телефон</dt>
                    <dd className="font-mono text-gray-900 mt-0.5">
                      {detail.customerPhone?.trim() || 'Не указан'}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-gray-400 font-medium">Адрес доставки</dt>
                    <dd className="text-gray-800 mt-0.5">
                      {detail.deliveryAddress?.trim() || 'Не указан'}
                    </dd>
                  </div>
                </dl>
              </div>

              {/* F. TIMELINE */}
              <div data-testid="shipment-drawer-timeline" className="space-y-3">
                <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
                  Хронология
                </h3>
                <div className="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-4">
                  {/* Step 1: Упаковано */}
                  <div className="flex items-start gap-3">
                    <div className="mt-0.5">
                      {detail.packedAt ? (
                        <CheckCircle2 className="w-4 h-4 text-emerald-600 shrink-0" />
                      ) : (
                        <Clock className="w-4 h-4 text-gray-300 shrink-0" />
                      )}
                    </div>
                    <div className="flex-1">
                      <div className="text-xs font-bold text-gray-900">Упаковано</div>
                      <div className="text-[11px] text-gray-500 mt-0.5">
                        {detail.packedAt ? formatDate(detail.packedAt) : 'Ожидает упаковки'}
                      </div>
                    </div>
                  </div>

                  {/* Step 2: Передано в доставку */}
                  <div className="flex items-start gap-3">
                    <div className="mt-0.5">
                      {detail.shippedAt ? (
                        <CheckCircle2 className="w-4 h-4 text-indigo-600 shrink-0" />
                      ) : (
                        <Clock className="w-4 h-4 text-gray-300 shrink-0" />
                      )}
                    </div>
                    <div className="flex-1">
                      <div className="text-xs font-bold text-gray-900">Передано в доставку</div>
                      <div className="text-[11px] text-gray-500 mt-0.5">
                        {detail.shippedAt ? formatDate(detail.shippedAt) : 'Ожидает передачи'}
                      </div>
                    </div>
                  </div>

                  {/* Step 3: Доставлено */}
                  <div className="flex items-start gap-3">
                    <div className="mt-0.5">
                      {detail.deliveredAt ? (
                        <CheckCircle2 className="w-4 h-4 text-emerald-600 shrink-0" />
                      ) : (
                        <Clock className="w-4 h-4 text-gray-300 shrink-0" />
                      )}
                    </div>
                    <div className="flex-1">
                      <div className="text-xs font-bold text-gray-900">Доставлено</div>
                      <div className="text-[11px] text-gray-500 mt-0.5">
                        {detail.deliveredAt ? formatDate(detail.deliveredAt) : 'Не доставлено'}
                      </div>
                    </div>
                  </div>

                  {/* Exception state if failed or cancelled */}
                  {(detail.status === 'failed' || detail.status === 'cancelled') && (
                    <div
                      data-testid="shipment-drawer-timeline-exception"
                      className="pt-2 border-t border-gray-200/80 flex items-start gap-2.5"
                    >
                      <AlertCircle className="w-4 h-4 text-rose-600 mt-0.5 shrink-0" />
                      <div>
                        <div className="text-xs font-bold text-rose-800">
                          {detail.status === 'failed' ? 'Ошибка доставки' : 'Отправка отменена'}
                        </div>
                        <div className="text-[11px] text-rose-600 mt-0.5">
                          {detail.status === 'failed'
                            ? 'Доставка не завершена'
                            : 'Отправление отменено'}
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* G. TECHNICAL INFORMATION */}
              <div data-testid="shipment-drawer-technical">
                <details className="group border border-gray-200 rounded-xl overflow-hidden bg-gray-50/50">
                  <summary className="px-4 py-3 cursor-pointer flex items-center justify-between text-xs font-bold text-gray-600 hover:text-gray-900 select-none">
                    <span>Техническая информация</span>
                    <ChevronDown className="w-4 h-4 transition-transform group-open:rotate-180" />
                  </summary>
                  <div className="px-4 pb-4 pt-1 space-y-2.5 text-xs text-gray-600 border-t border-gray-100">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-gray-400 shrink-0">ID отправления:</span>
                      <div className="flex items-center gap-1.5 font-mono text-[11px] text-gray-800 truncate">
                        <span className="truncate">{detail.shipmentId || detail.id}</span>
                        <button
                          type="button"
                          onClick={() => handleCopy(detail.shipmentId || detail.id, 'shipmentId')}
                          className="text-gray-400 hover:text-gray-700 p-1 shrink-0 transition-colors"
                          title="Скопировать ID отправления"
                        >
                          {copiedKey === 'shipmentId' ? (
                            <Check className="w-3.5 h-3.5 text-emerald-600" />
                          ) : (
                            <Copy className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                    </div>

                    {detail.fulfillmentId && (
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-gray-400 shrink-0">ID сборки:</span>
                        <div className="flex items-center gap-1.5 font-mono text-[11px] text-gray-800 truncate">
                          <span className="truncate">{detail.fulfillmentId}</span>
                          <button
                            type="button"
                            onClick={() => handleCopy(detail.fulfillmentId!, 'fulfillmentId')}
                            className="text-gray-400 hover:text-gray-700 p-1 shrink-0 transition-colors"
                            title="Скопировать ID сборки"
                          >
                            {copiedKey === 'fulfillmentId' ? (
                              <Check className="w-3.5 h-3.5 text-emerald-600" />
                            ) : (
                              <Copy className="w-3.5 h-3.5" />
                            )}
                          </button>
                        </div>
                      </div>
                    )}

                    <div className="flex items-center justify-between gap-2">
                      <span className="text-gray-400 shrink-0">ID заказа:</span>
                      <div className="flex items-center gap-1.5 font-mono text-[11px] text-gray-800 truncate">
                        <span className="truncate">{detail.orderId}</span>
                        <button
                          type="button"
                          onClick={() => handleCopy(detail.orderId, 'orderId')}
                          className="text-gray-400 hover:text-gray-700 p-1 shrink-0 transition-colors"
                          title="Скопировать ID заказа"
                        >
                          {copiedKey === 'orderId' ? (
                            <Check className="w-3.5 h-3.5 text-emerald-600" />
                          ) : (
                            <Copy className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                    </div>
                  </div>
                </details>
              </div>
            </>
          ) : null}
        </div>
      </div>
    </>
  );

  if (typeof document !== 'undefined' && document.body) {
    return createPortal(drawerContent, document.body);
  }

  return drawerContent;
};

import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  X,
  ArrowLeft,
  Copy,
  Check,
  AlertTriangle,
  Package,
  Layers,
  CheckCircle2,
  RotateCcw,
  ExternalLink,
  QrCode,
  Calendar,
  User,
  ShieldAlert,
} from 'lucide-react';
import {
  getAdminInventoryUnitTraceability,
  type AdminInventoryUnitTraceability,
  type AdminInventoryUnitTimelineEvent,
} from '../../api/adminInventory';

interface ZmuTraceabilityDrawerProps {
  unitCode: string | null;
  isOpen: boolean;
  onClose: () => void;
}

export const ZmuTraceabilityDrawer: React.FC<ZmuTraceabilityDrawerProps> = ({
  unitCode,
  isOpen,
  onClose,
}) => {
  const [data, setData] = useState<AdminInventoryUnitTraceability | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!isOpen || !unitCode) {
      setData(null);
      setError(null);
      return;
    }

    let isMounted = true;
    setLoading(true);
    setError(null);

    getAdminInventoryUnitTraceability(unitCode)
      .then((res) => {
        if (isMounted) {
          setData(res);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setError(err?.message || 'Не удалось загрузить историю единицы');
          setLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, [isOpen, unitCode]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  if (!isOpen || !unitCode) return null;

  const formatDate = (dateStr?: string | null) => {
    if (!dateStr) return '—';
    try {
      const d = new Date(dateStr);
      return new Intl.DateTimeFormat('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }).format(d);
    } catch {
      return dateStr;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'warehouse':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            На складе
          </span>
        );
      case 'expected':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">
            Ожидается
          </span>
        );
      case 'damaged':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
            Брак
          </span>
        );
      case 'written_off':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200">
            Списана
          </span>
        );
      case 'shipped':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
            Отгружена
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-700">
            {status}
          </span>
        );
    }
  };

  const getAvailabilityBadge = (avail: string) => {
    switch (avail) {
      case 'free':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            Свободна
          </span>
        );
      case 'allocated':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200">
            Назначена заказу
          </span>
        );
      case 'picked':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-purple-50 text-purple-700 border border-purple-200">
            Собрана
          </span>
        );
      case 'unavailable_expected':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-600">
            Недоступна — ожидается
          </span>
        );
      case 'unavailable_damaged':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-rose-50 text-rose-600">
            Недоступна — брак
          </span>
        );
      case 'unavailable_shipped':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-600">
            Не на складе
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-600">
            {avail}
          </span>
        );
    }
  };

  const getCategoryVisuals = (category: AdminInventoryUnitTimelineEvent['category']) => {
    switch (category) {
      case 'physical':
        return {
          icon: <Package className="w-4 h-4 text-emerald-600" />,
          bgColor: 'bg-emerald-50',
          borderColor: 'border-emerald-200',
          badgeText: 'ФИЗИЧЕСКОЕ',
          badgeClass: 'bg-emerald-100 text-emerald-800',
        };
      case 'commitment':
        return {
          icon: <Layers className="w-4 h-4 text-amber-600" />,
          bgColor: 'bg-amber-50',
          borderColor: 'border-amber-200',
          badgeText: 'ОБЯЗАТЕЛЬСТВО',
          badgeClass: 'bg-amber-100 text-amber-800',
        };
      case 'operation':
        return {
          icon: <CheckCircle2 className="w-4 h-4 text-purple-600" />,
          bgColor: 'bg-purple-50',
          borderColor: 'border-purple-200',
          badgeText: 'ОПЕРАЦИЯ',
          badgeClass: 'bg-purple-100 text-purple-800',
        };
      case 'order_lifecycle':
        return {
          icon: <RotateCcw className="w-4 h-4 text-indigo-600" />,
          bgColor: 'bg-indigo-50',
          borderColor: 'border-indigo-200',
          badgeText: 'ЗАКАЗ / ВОЗВРАТ',
          badgeClass: 'bg-indigo-100 text-indigo-800',
        };
      case 'diagnostic':
        return {
          icon: <ShieldAlert className="w-4 h-4 text-rose-600" />,
          bgColor: 'bg-rose-50',
          borderColor: 'border-rose-200',
          badgeText: 'ДИАГНОСТИКА',
          badgeClass: 'bg-rose-100 text-rose-800 font-bold',
        };
      default:
        return {
          icon: <Package className="w-4 h-4 text-slate-600" />,
          bgColor: 'bg-slate-50',
          borderColor: 'border-slate-200',
          badgeText: 'ДВИЖЕНИЕ',
          badgeClass: 'bg-slate-100 text-slate-800',
        };
    }
  };

  const formatRussianOrderStatus = (status: string) => {
    switch (status) {
      case 'pending':
        return 'Создан';
      case 'paid':
        return 'Оплачен';
      case 'assembling':
        return 'В сборке';
      case 'assembled':
        return 'Собран';
      case 'packed':
        return 'Упакован';
      case 'shipped':
        return 'В доставке';
      case 'delivered':
        return 'Доставлен';
      case 'cancelled':
        return 'Отменён';
      case 'returned':
        return 'Возвращён';
      case 'refunded':
        return 'Возврат средств';
      default:
        return status;
    }
  };

  const formatActorPresentation = (role?: string, name?: string) => {
    if (!role && !name) return null;

    const roleMap: Record<string, string> = {
      staff: 'Сотрудник',
      system: 'Система',
      customer: 'Покупатель',
      seller: 'Продавец',
      admin: 'Администратор',
    };

    const russianRole = role ? (roleMap[role.toLowerCase()] || role) : '';

    if (name && russianRole) {
      return `${name} · ${russianRole}`;
    }
    if (name) {
      return name;
    }
    return russianRole;
  };

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-slate-900/40 backdrop-blur-2xs z-[60] transition-opacity"
        onClick={onClose}
      />

      {/* Drawer */}
      <div
        data-testid="zmu-traceability-drawer"
        className="fixed inset-y-0 right-0 h-screen h-[100dvh] w-full sm:max-w-xl md:max-w-2xl bg-white shadow-2xl z-[70] flex flex-col border-l border-slate-200 min-w-0"
      >
        {/* Header */}
        <div className="px-6 pt-5 pb-4 border-b border-slate-200 flex items-start justify-between gap-4 bg-white shrink-0">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 mb-2">
              <button
                type="button"
                onClick={onClose}
                className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600 hover:text-slate-900 bg-slate-100 hover:bg-slate-200 px-2.5 py-1 rounded-md transition-colors cursor-pointer"
              >
                <ArrowLeft className="w-3.5 h-3.5" />
                <span>Назад к варианту</span>
              </button>
            </div>

            <div className="flex items-center gap-2.5 flex-wrap">
              <span className="font-mono font-bold text-base text-slate-900">
                {unitCode}
              </span>
              <button
                type="button"
                onClick={() => handleCopy(unitCode)}
                className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-800 bg-slate-100 hover:bg-slate-200 px-2 py-0.5 rounded cursor-pointer transition-colors"
                title="Скопировать ZMU"
              >
                {copied ? (
                  <>
                    <Check className="w-3 h-3 text-emerald-600" />
                    <span className="text-emerald-700 text-[11px]">Скопировано</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3 h-3" />
                    <span className="text-[11px]">Копировать</span>
                  </>
                )}
              </button>
              {data && getStatusBadge(data.currentState.status)}
              {data && getAvailabilityBadge(data.currentState.availability)}
              {data?.currentState.isStaleAllocation && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
                  <AlertTriangle className="w-3.5 h-3.5 text-rose-600" />
                  <span>Старое назначение</span>
                </span>
              )}
            </div>

            {data && (
              <div className="text-sm text-slate-600 font-medium mt-1">
                {data.identity.productTitle}
                {data.identity.variantName && ` · ${data.identity.variantName}`}
              </div>
            )}
          </div>

          <button
            type="button"
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
            title="Закрыть (Esc)"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 min-h-0 overflow-y-auto p-6 space-y-6">
          {loading && (
            <div className="py-12 text-center text-slate-400">
              <div className="inline-block animate-spin w-6 h-6 border-2 border-indigo-600 border-t-transparent rounded-full mb-2" />
              <div className="text-xs">Загрузка истории физической единицы...</div>
            </div>
          )}

          {error && (
            <div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700">
              {error}
            </div>
          )}

          {!loading && data && (
            <>
              {/* CURRENT STATE SUMMARY BLOCK */}
              <div className="bg-slate-50/90 border border-slate-200 rounded-xl p-4 shadow-2xs">
                <div className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3">
                  Текущее состояние единицы
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-xs">
                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Физическое состояние
                    </div>
                    <div>{getStatusBadge(data.currentState.status)}</div>
                  </div>

                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Доступность
                    </div>
                    <div>{getAvailabilityBadge(data.currentState.availability)}</div>
                  </div>

                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Расположение
                    </div>
                    <div className="text-slate-700 font-medium">{data.currentState.location}</div>
                  </div>

                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Происхождение
                    </div>
                    {data.origin ? (
                      <div>
                        <div className="font-mono font-medium text-slate-900 flex items-center gap-1">
                          <span>{data.origin.supplyNumber}</span>
                          <Link
                            to={`/supply-receiving?id=${data.origin.supplyId}`}
                            className="text-indigo-600 hover:text-indigo-800 inline-flex items-center"
                            title="Открыть приёмку поставки"
                          >
                            <ExternalLink className="w-3 h-3" />
                          </Link>
                        </div>
                      </div>
                    ) : (
                      <span className="text-slate-500">—</span>
                    )}
                  </div>

                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Принято
                    </div>
                    <div className="text-slate-700 font-medium">
                      {formatDate(data.origin?.receivedAt)}
                    </div>
                  </div>

                  <div>
                    <div className="text-slate-400 font-medium uppercase text-[10px] tracking-wider mb-1">
                      Текущее назначение
                    </div>
                    {data.currentContext.liveAllocation ? (
                      <div>
                        <Link
                          to={`/orders/${data.currentContext.liveAllocation.orderId}`}
                          className="font-mono font-semibold text-indigo-600 hover:text-indigo-800 flex items-center gap-1"
                        >
                          <span>{data.currentContext.liveAllocation.orderNumber}</span>
                          <ExternalLink className="w-3 h-3" />
                        </Link>
                        <div className="text-[11px] text-slate-600 mt-0.5">
                          <span>{formatRussianOrderStatus(data.currentContext.liveAllocation.orderStatus)}</span>
                          {' · '}
                          <span className={data.currentContext.liveAllocation.pickedAt ? 'text-purple-700 font-medium' : 'text-slate-500'}>
                            {data.currentContext.liveAllocation.pickedAt ? 'Собрана' : 'Ожидает сборки'}
                          </span>
                        </div>
                      </div>
                    ) : data.currentContext.staleAllocation ? (
                      <div>
                        <span className="text-slate-500">—</span>
                        <div className="text-[10px] text-rose-600 font-medium mt-0.5">
                          Историческое: {data.currentContext.staleAllocation.orderNumber}
                        </div>
                      </div>
                    ) : (
                      <span className="text-slate-500">—</span>
                    )}
                  </div>
                </div>

                {/* Secondary Quick Action: Free Scanner */}
                <div className="mt-3 pt-3 border-t border-slate-200/80 flex items-center justify-between text-xs text-slate-500">
                  <span className="text-[11px]">Действия с физической единицей:</span>
                  <div className="flex items-center gap-3">
                    <Link
                      to={`/warehouse/free-scan?q=${encodeURIComponent(unitCode)}`}
                      className="inline-flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800 font-medium"
                    >
                      <QrCode className="w-3.5 h-3.5" />
                      <span>Открыть в сканере</span>
                    </Link>
                  </div>
                </div>
              </div>

              {/* CURRENT DISCREPANCY SECTION */}
              {data.currentState.isStaleAllocation && data.currentContext.staleAllocation && (
                <div className="p-4 bg-rose-50/90 border border-rose-200 rounded-xl flex items-start gap-3 shadow-2xs">
                  <AlertTriangle className="w-5 h-5 text-rose-600 shrink-0 mt-0.5" />
                  <div className="text-xs text-rose-900 flex-1">
                    <div className="font-bold text-rose-950">
                      Текущее расхождение: Старое назначение
                    </div>
                    <div className="mt-1 text-rose-800">
                      Единица находится на складе, но историческое назначение{' '}
                      <Link
                        to={`/orders/${data.currentContext.staleAllocation.orderId}`}
                        className="font-mono font-bold underline hover:text-rose-950"
                      >
                        {data.currentContext.staleAllocation.orderNumber}
                      </Link>{' '}
                      ({formatRussianOrderStatus(data.currentContext.staleAllocation.orderStatus)}) не было закрыто.
                    </div>
                  </div>
                </div>
              )}

              {/* PARTIAL HISTORY NOTICE */}
              {data.hasPartialHistory && (
                <div className="p-3 bg-amber-50/80 border border-amber-200 rounded-lg text-xs text-amber-800 flex items-center gap-2">
                  <Calendar className="w-4 h-4 text-amber-600 shrink-0" />
                  <span>История этой единицы сохранена не полностью. Отображаются только зафиксированные события.</span>
                </div>
              )}

              {/* CANONICAL EVENT TIMELINE */}
              <div>
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-sm font-bold text-slate-900">
                    История единицы ({data.timeline.length})
                  </h3>
                  <span className="text-[11px] text-slate-400 font-medium">
                    Сначала новые
                  </span>
                </div>

                {data.timeline.length === 0 ? (
                  <div className="py-8 text-center text-xs text-slate-400 bg-slate-50 border border-dashed border-slate-200 rounded-xl">
                    Для этой единицы пока не зафиксировано событий жизненного цикла
                  </div>
                ) : (
                  <div className="relative pl-6 space-y-6 before:absolute before:left-2.5 before:top-2 before:bottom-2 before:w-0.5 before:bg-slate-200">
                    {data.timeline.map((event) => {
                      const visual = getCategoryVisuals(event.category);
                      return (
                        <div key={event.id} className="relative group">
                          {/* Dot / Icon */}
                          <div
                            className={`absolute -left-6 top-0.5 w-5 h-5 rounded-full ${visual.bgColor} border ${visual.borderColor} flex items-center justify-center ring-4 ring-white`}
                          >
                            {visual.icon}
                          </div>

                          <div
                            className="bg-white border border-slate-200 hover:border-slate-300 rounded-xl p-3.5 transition-colors shadow-2xs"
                            title={event.sourceEntity ? `Технический источник: ${event.sourceEntity}` : undefined}
                          >
                            <div className="flex items-start justify-between gap-2 flex-wrap mb-1">
                              <div className="flex items-center gap-2">
                                <span className="text-xs font-bold text-slate-900">
                                  {event.eventName}
                                </span>
                                <span
                                  className={`inline-flex items-center px-1.5 py-0.2 rounded text-[10px] font-semibold uppercase tracking-wider ${visual.badgeClass}`}
                                >
                                  {visual.badgeText}
                                </span>
                              </div>
                              <span className="text-[11px] font-mono text-slate-500">
                                {formatDate(event.timestamp)}
                              </span>
                            </div>

                            <p className="text-xs text-slate-700 mb-2">
                              {event.description}
                            </p>

                            {/* Event Metadata Footer */}
                            <div className="flex items-center gap-4 text-[11px] text-slate-500 flex-wrap pt-2 border-t border-slate-100">
                              {event.referenceNumber && (
                                <div className="flex items-center gap-1">
                                  <span className="text-slate-400">Документ:</span>
                                  {event.link ? (
                                    <Link
                                      to={event.link}
                                      className="font-mono font-medium text-indigo-600 hover:text-indigo-800 flex items-center gap-0.5"
                                    >
                                      <span>{event.referenceNumber}</span>
                                      <ExternalLink className="w-2.5 h-2.5" />
                                    </Link>
                                  ) : (
                                    <span className="font-mono font-medium text-slate-700">
                                      {event.referenceNumber}
                                    </span>
                                  )}
                                </div>
                              )}

                              {(event.actorRole || event.actorName) && (
                                <div className="flex items-center gap-1">
                                  <User className="w-3 h-3 text-slate-400" />
                                  <span className="text-slate-400">Инициатор:</span>
                                  <span className="text-slate-700 font-medium">
                                    {formatActorPresentation(event.actorRole, event.actorName)}
                                  </span>
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </>
  );
};

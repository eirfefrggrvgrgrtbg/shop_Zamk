import { useState, useEffect, useRef } from 'react';
import { Bell, Check, CheckCheck, AlertTriangle, AlertCircle, Info } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { notificationsApi, type Notification } from '../../api/notifications';
import { useAuth } from '../../contexts/AuthContext';

interface ForecastVariantMeta {
  variantId?: string;
  sellerSku?: string;
  color?: string;
  size?: string;
  freeSellableStock?: number;
  paidDemandUnits?: number;
  observedDays?: number;
  dailySalesVelocity?: number;
  daysOfCover?: number;
  severity?: string;
}

export function formatDaysRussian(days: number): string {
  if (days < 0) days = 0;
  const mod100 = days % 100;
  const mod10 = days % 10;
  if (mod100 >= 11 && mod100 <= 19) {
    return `${days} дней`;
  }
  switch (mod10) {
    case 1:
      return `${days} день`;
    case 2:
    case 3:
    case 4:
      return `${days} дня`;
    default:
      return `${days} дней`;
  }
}

export function formatVariantDisplayLabel(color?: string | null, size?: string | null): string {
  const c = (color || '').trim();
  const s = (size || '').trim();
  if (c && s) return `${c} · ${s}`;
  if (c) return c;
  if (s) return s;
  return '';
}

function parseForecastVariants(metadata?: Record<string, unknown> | null): ForecastVariantMeta[] | null {
  if (!metadata || !Array.isArray(metadata.variants)) {
    return null;
  }
  const result: ForecastVariantMeta[] = [];
  for (const item of metadata.variants) {
    if (item && typeof item === 'object') {
      const v = item as Record<string, unknown>;
      const daysOfCover = typeof v.daysOfCover === 'number' ? v.daysOfCover : undefined;
      result.push({
        variantId: typeof v.variantId === 'string' ? v.variantId : undefined,
        sellerSku: typeof v.sellerSku === 'string' ? v.sellerSku : undefined,
        color: typeof v.color === 'string' ? v.color : undefined,
        size: typeof v.size === 'string' ? v.size : undefined,
        freeSellableStock: typeof v.freeSellableStock === 'number' ? v.freeSellableStock : undefined,
        paidDemandUnits: typeof v.paidDemandUnits === 'number' ? v.paidDemandUnits : undefined,
        observedDays: typeof v.observedDays === 'number' ? v.observedDays : undefined,
        dailySalesVelocity: typeof v.dailySalesVelocity === 'number' ? v.dailySalesVelocity : undefined,
        daysOfCover,
        severity: typeof v.severity === 'string' ? v.severity : undefined,
      });
    }
  }
  return result.length > 0 ? result : null;
}

export function NotificationBell() {
  const navigate = useNavigate();
  const [isOpen, setIsOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { isAuthenticated } = useAuth();
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isAuthenticated) {
      fetchUnreadCount();
      const interval = setInterval(fetchUnreadCount, 60000); // Polling every minute
      return () => clearInterval(interval);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const fetchUnreadCount = async () => {
    try {
      const count = await notificationsApi.getUnreadCount();
      setUnreadCount(count);
    } catch (error) {
      console.error('Failed to fetch unread count', error);
    }
  };

  const fetchNotifications = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await notificationsApi.getNotifications(20, 0);
      setNotifications(res.items || []);
    } catch (err) {
      console.error('Failed to fetch notifications', err);
      setError('Не удалось загрузить уведомления.');
    } finally {
      setLoading(false);
    }
  };

  const handleOpen = () => {
    if (!isOpen) {
      fetchNotifications();
    }
    setIsOpen(!isOpen);
  };

  const handleMarkRead = async (id: string) => {
    try {
      await notificationsApi.markRead(id);
      setNotifications(notifications.map(n => n.id === id ? { ...n, readAt: new Date().toISOString() } : n));
      setUnreadCount(Math.max(0, unreadCount - 1));
    } catch (error) {
      console.error('Failed to mark read', error);
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await notificationsApi.markAllRead();
      setNotifications(notifications.map(n => ({ ...n, readAt: n.readAt || new Date().toISOString() })));
      setUnreadCount(0);
    } catch (error) {
      console.error('Failed to mark all read', error);
    }
  };

  if (!isAuthenticated) return null;

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={handleOpen}
        className="p-2.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors relative rounded-full hover:bg-gray-100 dark:hover:bg-gray-800"
        aria-label="Уведомления"
      >
        <Bell className="w-5 h-5" />
        {unreadCount > 0 && (
          <span className="absolute top-1 right-1 bg-red-500 text-white text-[10px] font-bold w-[18px] h-[18px] rounded-full flex items-center justify-center shadow-sm">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 bg-white dark:bg-gray-900 rounded-xl shadow-lg border border-gray-200 dark:border-gray-800 overflow-hidden z-[100] flex flex-col max-h-[80vh]">
          <div className="p-4 border-b border-gray-100 dark:border-gray-800 flex justify-between items-center bg-gray-50/50 dark:bg-gray-800/50">
            <h3 className="font-medium text-gray-900 dark:text-white">Уведомления</h3>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 transition-colors flex items-center gap-1"
              >
                <CheckCheck className="w-3.5 h-3.5" />
                Прочитать все
              </button>
            )}
          </div>
          
          <div className="overflow-y-auto flex-1 p-2 space-y-1">
            {loading ? (
              <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">Загружаем уведомления…</div>
            ) : error ? (
              <div className="p-4 text-center text-sm text-red-500">{error}</div>
            ) : notifications.length === 0 ? (
              <div className="p-8 text-center text-sm text-gray-500 dark:text-gray-400">Уведомлений пока нет.</div>
            ) : (
              notifications.map((n) => {
                const isCritical = n.severity === 'critical';
                const isWarning = n.severity === 'warning';
                const isAlert = n.kind === 'alert';
                const isResolved = n.status === 'resolved';
                const isForecastRisk = n.type === 'stock_forecast_risk';

                let icon = <Info className="w-4 h-4 text-blue-500 shrink-0" />;
                if (isCritical) icon = <AlertCircle className="w-4 h-4 text-red-500 shrink-0" />;
                if (isWarning) icon = <AlertTriangle className="w-4 h-4 text-yellow-500 shrink-0" />;

                const forecastVariants = isForecastRisk ? parseForecastVariants(n.metadata) : null;

                const handleAction = () => {
                  if (n.actionUrl) {
                    setIsOpen(false);
                    navigate(n.actionUrl);
                  }
                };

                return (
                  <div
                    key={n.id}
                    className={`p-3 rounded-lg transition-colors ${n.actionUrl ? 'cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800' : 'cursor-default hover:bg-gray-50 dark:hover:bg-gray-800'} ${!n.readAt ? 'bg-blue-50 dark:bg-blue-900/20' : ''}`}
                    onClick={n.actionUrl ? handleAction : undefined}
                  >
                    <div className="flex justify-between items-start gap-2">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-1.5">
                           {icon}
                           <h4 className="text-sm font-medium text-gray-900 dark:text-white leading-tight truncate">{n.title}</h4>
                        </div>

                        {/* Forecast diagnostic details or standard body */}
                        {isForecastRisk && forecastVariants ? (
                          <div className="mt-2 space-y-1.5">
                            {forecastVariants.slice(0, 3).map((v, idx) => {
                              const label = formatVariantDisplayLabel(v.color, v.size);
                              const roundedDays = typeof v.daysOfCover === 'number' ? Math.round(v.daysOfCover) : null;
                              const baseCoverText = roundedDays !== null ? `≈ ${formatDaysRussian(roundedDays)} запаса` : null;
                              const coverText = baseCoverText ? (isResolved ? `Было: ${baseCoverText}` : baseCoverText) : null;

                              return (
                                <div key={v.variantId || idx} className="text-xs bg-gray-50 dark:bg-gray-800/60 p-1.5 rounded flex items-center justify-between gap-2 border border-gray-100 dark:border-gray-800">
                                  {label ? (
                                    <span className="font-medium text-gray-800 dark:text-gray-200 truncate">{label}</span>
                                  ) : (
                                    <span className="font-medium text-gray-800 dark:text-gray-200 truncate">Основной</span>
                                  )}
                                  {coverText && (
                                    <span className="text-[11px] text-gray-500 dark:text-gray-400 shrink-0 font-normal">
                                      {coverText}
                                    </span>
                                  )}
                                </div>
                              );
                            })}
                            {forecastVariants.length > 3 && (
                              <p className="text-[11px] text-gray-400 dark:text-gray-500 italic pl-1">
                                Ещё {forecastVariants.length - 3} {formatDaysRussian(forecastVariants.length - 3).replace(/^\d+\s*/, '') === 'день' ? 'вариант' : (formatDaysRussian(forecastVariants.length - 3).replace(/^\d+\s*/, '') === 'дня' ? 'варианта' : 'вариантов')}
                              </p>
                            )}
                          </div>
                        ) : (
                          <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{n.body}</p>
                        )}

                        <div className="flex items-center gap-2 mt-2">
                          <span className="text-[10px] text-gray-400 dark:text-gray-500 block">
                            {new Date(n.createdAt).toLocaleString('ru-RU')}
                          </span>
                          {isAlert && (
                            <span className={`text-[10px] px-1.5 py-0.5 rounded-sm font-medium ${isResolved ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'}`}>
                              {isResolved ? 'Решено' : 'Активно'}
                            </span>
                          )}
                        </div>
                      </div>
                      {!n.readAt && (
                        <button
                          onClick={(e) => { e.stopPropagation(); handleMarkRead(n.id); }}
                          className="text-blue-600 hover:text-blue-700 p-1 rounded-full hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors shrink-0"
                          title="Отметить как прочитанное"
                        >
                          <Check className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}

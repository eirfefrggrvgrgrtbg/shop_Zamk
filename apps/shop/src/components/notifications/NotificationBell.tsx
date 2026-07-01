import { useState, useEffect, useRef } from 'react';
import { Bell, Check, CheckCheck } from 'lucide-react';
import { notificationsApi, type Notification } from '../../api/notifications';
import { useAuth } from '../../contexts/AuthContext';

export function NotificationBell() {
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
        className="hidden sm:flex p-2.5 text-graphite/50 hover:text-graphite dark:text-white/50 dark:hover:text-white transition-colors relative rounded-full hover:bg-white/40 dark:hover:bg-white/10"
        aria-label="Уведомления"
      >
        <Bell className="w-[17px] h-[17px]" />
        {unreadCount > 0 && (
          <span className="absolute top-1.5 right-1.5 bg-red-500 text-white text-[9px] font-bold w-[16px] h-[16px] rounded-full flex items-center justify-center shadow-sm">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-2 w-80 bg-white dark:bg-[#111214] rounded-xl shadow-[0_12px_36px_rgba(0,0,0,0.12)] border border-gray-100 dark:border-white/10 overflow-hidden z-[100] flex flex-col max-h-[80vh]">
          <div className="p-4 border-b border-gray-100 dark:border-white/10 flex justify-between items-center bg-gray-50/50 dark:bg-white/5">
            <h3 className="font-medium text-graphite dark:text-white">Уведомления</h3>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="text-xs text-primary hover:text-primary/80 transition-colors flex items-center gap-1"
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
              notifications.map((n) => (
                <div
                  key={n.id}
                  className={`p-3 rounded-lg transition-colors cursor-default ${
                    !n.readAt ? 'bg-primary/5 dark:bg-primary/10' : 'hover:bg-gray-50 dark:hover:bg-white/5'
                  }`}
                >
                  <div className="flex justify-between items-start gap-2">
                    <div>
                      <h4 className="text-sm font-medium text-graphite dark:text-white leading-tight">{n.title}</h4>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{n.body}</p>
                      <span className="text-[10px] text-gray-400 dark:text-gray-500 mt-2 block">
                        {new Date(n.createdAt).toLocaleString('ru-RU')}
                      </span>
                    </div>
                    {!n.readAt && (
                      <button
                        onClick={() => handleMarkRead(n.id)}
                        className="text-primary hover:text-primary/80 p-1 rounded-full hover:bg-primary/10 transition-colors"
                        title="Отметить как прочитанное"
                      >
                        <Check className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

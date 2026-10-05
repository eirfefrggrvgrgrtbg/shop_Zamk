import { Search, X, User, Store, AlertCircle } from 'lucide-react';
import type { SupportConversation, SupportRequesterType } from '../../api/adminSupport';

interface SupportInboxProps {
  conversations: SupportConversation[];
  selectedId: string | null;
  onSelect: (conv: SupportConversation) => void;
  filter: 'ALL' | SupportRequesterType;
  onFilterChange: (filter: 'ALL' | SupportRequesterType) => void;
  search: string;
  onSearchChange: (query: string) => void;
  loading: boolean;
}

export function SupportInbox({
  conversations,
  selectedId,
  onSelect,
  filter,
  onFilterChange,
  search,
  onSearchChange,
  loading,
}: SupportInboxProps) {
  const formatTime = (isoString?: string) => {
    if (!isoString) return '';
    try {
      const date = new Date(isoString);
      const now = new Date();
      const isToday =
        date.getDate() === now.getDate() &&
        date.getMonth() === now.getMonth() &&
        date.getFullYear() === now.getFullYear();

      if (isToday) {
        return date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
      }
      return date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' });
    } catch {
      return '';
    }
  };

  return (
    <div className="flex flex-col h-full bg-white border-r border-gray-200 w-80 md:w-96 flex-shrink-0 select-none">
      {/* Top Header */}
      <div className="p-4 border-b border-gray-100 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-gray-900 tracking-tight">Поддержка</h1>
          <span className="text-xs font-semibold px-2 py-0.5 rounded bg-gray-100 text-gray-600">
            {conversations.length}
          </span>
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Поиск по имени, email, магазину..."
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full pl-9 pr-8 py-1.5 text-sm bg-gray-50 border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-black focus:bg-white text-gray-900 placeholder-gray-400 transition-colors"
          />
          {search && (
            <button
              onClick={() => onSearchChange('')}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              title="Очистить поиск"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>

        {/* Filter Tabs */}
        <div className="flex rounded-lg bg-gray-100 p-0.5 text-xs font-medium">
          <button
            onClick={() => onFilterChange('ALL')}
            className={`flex-1 py-1 text-center rounded-md transition-colors ${
              filter === 'ALL'
                ? 'bg-white text-gray-900 shadow-sm font-semibold'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            Все
          </button>
          <button
            onClick={() => onFilterChange('CUSTOMER')}
            className={`flex-1 py-1 text-center rounded-md transition-colors ${
              filter === 'CUSTOMER'
                ? 'bg-white text-gray-900 shadow-sm font-semibold'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            Покупатели
          </button>
          <button
            onClick={() => onFilterChange('SELLER')}
            className={`flex-1 py-1 text-center rounded-md transition-colors ${
              filter === 'SELLER'
                ? 'bg-white text-gray-900 shadow-sm font-semibold'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            Продавцы
          </button>
        </div>
      </div>

      {/* Conversations List */}
      <div className="flex-1 overflow-y-auto divide-y divide-gray-100">
        {loading ? (
          <div className="p-4 space-y-4">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="animate-pulse flex flex-col gap-2">
                <div className="flex justify-between items-center">
                  <div className="h-4 bg-gray-200 rounded w-28" />
                  <div className="h-3 bg-gray-200 rounded w-10" />
                </div>
                <div className="h-3 bg-gray-100 rounded w-48" />
              </div>
            ))}
          </div>
        ) : conversations.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 p-6 text-center text-gray-500">
            <AlertCircle className="w-8 h-8 text-gray-300 mb-2" />
            <p className="text-sm font-medium text-gray-700">Диалогов не найдено</p>
            <p className="text-xs text-gray-400 mt-1">
              {search
                ? 'Попробуйте изменить поисковый запрос'
                : 'В выбранной категории нет обращений'}
            </p>
          </div>
        ) : (
          conversations.map((conv) => {
            const isSelected = conv.id === selectedId;
            const isCustomer = conv.requesterType === 'CUSTOMER';
            const identityName = isCustomer
              ? conv.requesterName || conv.requesterEmail || 'Покупатель'
              : conv.requesterStoreName || 'Магазин продавца';
            const priority = conv.activeSession?.priority;
            const hasUnread = conv.unreadCount > 0;

            return (
              <button
                key={conv.id}
                onClick={() => onSelect(conv)}
                data-testid={`support-conversation-item-${conv.id}`}
                className={`w-full text-left p-3.5 transition-colors flex flex-col gap-1.5 focus:outline-none ${
                  isSelected
                    ? 'bg-blue-50/70 border-l-4 border-blue-600'
                    : 'hover:bg-gray-50 border-l-4 border-transparent'
                }`}
              >
                {/* Header Row: Identity & Time */}
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 min-w-0">
                    {isCustomer ? (
                      <User className="w-3.5 h-3.5 text-gray-400 flex-shrink-0" />
                    ) : (
                      <Store className="w-3.5 h-3.5 text-amber-600 flex-shrink-0" />
                    )}
                    <span
                      className={`text-sm truncate ${
                        hasUnread ? 'font-bold text-gray-900' : 'font-medium text-gray-800'
                      }`}
                    >
                      {identityName}
                    </span>
                  </div>
                  <span className="text-[11px] text-gray-400 whitespace-nowrap flex-shrink-0">
                    {formatTime(conv.latestMessageAt || conv.updatedAt)}
                  </span>
                </div>

                {/* Latest message snippet */}
                <div className="flex items-center justify-between gap-2">
                  <p
                    className={`text-xs truncate max-w-[200px] ${
                      hasUnread ? 'text-gray-900 font-medium' : 'text-gray-500'
                    }`}
                  >
                    {conv.latestMessageText || 'Нет сообщений'}
                  </p>
                  {hasUnread && (
                    <span className="flex-shrink-0 min-w-5 h-5 px-1.5 bg-blue-600 text-white text-[11px] font-bold rounded-full flex items-center justify-center">
                      {conv.unreadCount}
                    </span>
                  )}
                </div>

                {/* Badges: Type & Priority */}
                <div className="flex items-center gap-1.5 pt-0.5">
                  <span
                    className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${
                      isCustomer
                        ? 'bg-gray-100 text-gray-600'
                        : 'bg-amber-100 text-amber-800'
                    }`}
                  >
                    {isCustomer ? 'Покупатель' : 'Продавец'}
                  </span>

                  {priority === 'HIGH' && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded font-medium bg-orange-100 text-orange-800">
                      Высокий
                    </span>
                  )}
                  {priority === 'URGENT' && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded font-medium bg-red-100 text-red-800">
                      Срочный
                    </span>
                  )}
                  {conv.activeSession?.categoryName && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 text-gray-500 truncate max-w-[120px]">
                      {conv.activeSession.categoryName}
                    </span>
                  )}
                </div>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}

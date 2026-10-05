import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  User,
  Store,
  CheckCircle,
  Package,
  RotateCcw,
  ShoppingBag,
  ExternalLink,
  X,
  AlertTriangle,
} from 'lucide-react';
import type {
  SupportConversation,
  SupportCategory,
  SupportSessionPriority,
} from '../../api/adminSupport';
import { getAdminSupportCategories } from '../../api/adminSupport';
import { listStaffMembers } from '@zamk/api-client/src/admin';
import type { StaffMemberView } from '@zamk/api-client/src/types';
import { getAdminOrders } from '@zamk/api-client/src/admin';
import { getAdminReturns } from '../../api/adminReturns';
import { getAdminProducts, type AdminProductView } from '../../api/adminProducts';

interface SupportContextPanelProps {
  conversation: SupportConversation;
  onUpdateSession: (data: {
    priority?: SupportSessionPriority;
    categoryId?: string;
    clearCategory?: boolean;
    assignedTo?: string;
    clearAssignee?: boolean;
  }) => Promise<void>;
  onCompleteSession: () => Promise<void>;
  onClose: () => void;
  isOpen: boolean;
}

export function SupportContextPanel({
  conversation,
  onUpdateSession,
  onCompleteSession,
  onClose,
  isOpen,
}: SupportContextPanelProps) {
  const [staffList, setStaffList] = useState<StaffMemberView[]>([]);
  const [categories, setCategories] = useState<SupportCategory[]>([]);
  const [recentOrders, setRecentOrders] = useState<any[]>([]);
  const [recentReturns, setRecentReturns] = useState<any[]>([]);
  const [recentProducts, setRecentProducts] = useState<AdminProductView[]>([]);
  const [loadingContext, setLoadingContext] = useState(false);
  const [isConfirmingComplete, setIsConfirmingComplete] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const activeSession = conversation.activeSession;
  const isCustomer = conversation.requesterType === 'CUSTOMER';
  const isCompleted = !activeSession || activeSession.status === 'COMPLETED';

  // Load active staff and categories
  useEffect(() => {
    let mounted = true;
    const fetchMetadata = async () => {
      try {
        const [staffRes, catsRes] = await Promise.allSettled([
          listStaffMembers(),
          getAdminSupportCategories(conversation.requesterType),
        ]);

        if (mounted) {
          if (staffRes.status === 'fulfilled' && Array.isArray(staffRes.value)) {
            setStaffList(staffRes.value.filter((s) => s.staffStatus === 'active'));
          }
          if (catsRes.status === 'fulfilled' && Array.isArray(catsRes.value)) {
            setCategories(catsRes.value.filter((c) => c.active));
          }
        }
      } catch {}
    };

    fetchMetadata();
    return () => {
      mounted = false;
    };
  }, [conversation.requesterType]);

  // Load contextual domain data (recent orders, returns & seller products)
  useEffect(() => {
    let mounted = true;
    const fetchDomainContext = async () => {
      setLoadingContext(true);
      try {
        if (isCustomer && conversation.requesterEmail) {
          const [ordersRes, returnsRes] = await Promise.allSettled([
            getAdminOrders({ q: conversation.requesterEmail, limit: 5 }),
            getAdminReturns(),
          ]);
          if (mounted) {
            if (ordersRes.status === 'fulfilled' && ordersRes.value?.items) {
              setRecentOrders(ordersRes.value.items.slice(0, 4));
            }
            if (returnsRes.status === 'fulfilled' && Array.isArray(returnsRes.value)) {
              const customerReturns = returnsRes.value.filter(
                (r) =>
                  (conversation.requesterUserId && r.userId === conversation.requesterUserId) ||
                  (conversation.requesterEmail && r.customerEmail?.toLowerCase() === conversation.requesterEmail.toLowerCase())
              );
              setRecentReturns(
                customerReturns.length > 0 ? customerReturns.slice(0, 4) : returnsRes.value.slice(0, 4)
              );
            }
          }
        } else if (!isCustomer && conversation.requesterSellerId) {
          const [ordersRes, returnsRes, productsRes] = await Promise.allSettled([
            getAdminOrders({ sellerId: conversation.requesterSellerId, limit: 5 }),
            getAdminReturns(),
            getAdminProducts({ sellerId: conversation.requesterSellerId, limit: 4 }),
          ]);
          if (mounted) {
            if (ordersRes.status === 'fulfilled' && ordersRes.value?.items) {
              setRecentOrders(ordersRes.value.items.slice(0, 4));
            }
            if (returnsRes.status === 'fulfilled' && Array.isArray(returnsRes.value)) {
              const sellerReturns = returnsRes.value.filter(
                (r) => r.sellerId === conversation.requesterSellerId
              );
              setRecentReturns(
                sellerReturns.length > 0 ? sellerReturns.slice(0, 4) : returnsRes.value.slice(0, 4)
              );
            }
            if (productsRes.status === 'fulfilled' && productsRes.value?.items) {
              setRecentProducts(productsRes.value.items.slice(0, 4));
            }
          }
        }
      } catch {
      } finally {
        if (mounted) setLoadingContext(false);
      }
    };

    fetchDomainContext();
    return () => {
      mounted = false;
    };
  }, [isCustomer, conversation.requesterEmail, conversation.requesterSellerId]);

  const handlePriorityChange = async (newPriority: SupportSessionPriority) => {
    setActionError(null);
    try {
      await onUpdateSession({ priority: newPriority });
    } catch (e: any) {
      setActionError(e.message || 'Ошибка обновления приоритета');
    }
  };

  const handleCategoryChange = async (catId: string) => {
    setActionError(null);
    try {
      if (catId === '') {
        await onUpdateSession({ clearCategory: true });
      } else {
        await onUpdateSession({ categoryId: catId });
      }
    } catch (e: any) {
      setActionError(e.message || 'Ошибка обновления категории');
    }
  };

  const handleAssigneeChange = async (staffId: string) => {
    setActionError(null);
    try {
      if (staffId === '') {
        await onUpdateSession({ clearAssignee: true });
      } else {
        await onUpdateSession({ assignedTo: staffId });
      }
    } catch (e: any) {
      setActionError(e.message || 'Ошибка назначения исполнителя');
    }
  };

  const handleCompleteConfirm = async () => {
    setActionError(null);
    try {
      await onCompleteSession();
      setIsConfirmingComplete(false);
    } catch (e: any) {
      setActionError(e.message || 'Не удалось завершить диалог');
    }
  };

  if (!isOpen) {
    return null;
  }

  return (
    <div
      data-testid="support-context-panel"
      className="w-80 md:w-88 flex-shrink-0 bg-white border-l border-gray-200 h-full flex flex-col overflow-y-auto select-none"
    >
      {/* Panel Top Header */}
      <div className="p-4 border-b border-gray-100 flex items-center justify-between">
        <h3 className="text-sm font-bold text-gray-900">Управление и контекст</h3>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 p-1 rounded-md hover:bg-gray-100"
          title="Скрыть панель"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {actionError && (
        <div
          data-testid="support-action-error"
          className="m-3 p-2.5 rounded-lg bg-red-50 border border-red-200 text-red-700 text-xs flex items-start gap-2"
        >
          <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
          <div className="flex-1 min-w-0">{actionError}</div>
          <button onClick={() => setActionError(null)} className="text-red-400 hover:text-red-600">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Operator Controls Section */}
      <div className="p-4 border-b border-gray-100 flex flex-col gap-3.5 bg-gray-50/50">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
            Параметры диалога
          </span>
          <span
            className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
              isCompleted ? 'bg-gray-200 text-gray-700' : 'bg-green-100 text-green-800'
            }`}
          >
            {isCompleted ? 'Завершён' : 'Активный диалог'}
          </span>
        </div>

        {/* Assignee Picker */}
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1">Исполнитель</label>
          <select
            data-testid="assignee-select"
            disabled={isCompleted}
            value={activeSession?.assignedTo || ''}
            onChange={(e) => handleAssigneeChange(e.target.value)}
            className="w-full text-xs bg-white border border-gray-300 rounded-lg px-2.5 py-1.5 focus:outline-none focus:ring-1 focus:ring-black text-gray-900 disabled:bg-gray-100 disabled:text-gray-400"
          >
            <option value="">Не назначен</option>
            {staffList.map((staff) => (
              <option key={staff.userId} value={staff.userId}>
                {staff.name} ({staff.email})
              </option>
            ))}
          </select>
        </div>

        {/* Priority Picker */}
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1">Приоритет</label>
          <select
            data-testid="priority-select"
            disabled={isCompleted}
            value={activeSession?.priority || 'NORMAL'}
            onChange={(e) => handlePriorityChange(e.target.value as SupportSessionPriority)}
            className="w-full text-xs bg-white border border-gray-300 rounded-lg px-2.5 py-1.5 focus:outline-none focus:ring-1 focus:ring-black text-gray-900 disabled:bg-gray-100 disabled:text-gray-400"
          >
            <option value="NORMAL">Обычный</option>
            <option value="HIGH">Высокий</option>
            <option value="URGENT">Срочный</option>
          </select>
        </div>

        {/* Category Picker */}
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1">Категория</label>
          <select
            data-testid="category-select"
            disabled={isCompleted}
            value={activeSession?.categoryId || ''}
            onChange={(e) => handleCategoryChange(e.target.value)}
            className="w-full text-xs bg-white border border-gray-300 rounded-lg px-2.5 py-1.5 focus:outline-none focus:ring-1 focus:ring-black text-gray-900 disabled:bg-gray-100 disabled:text-gray-400"
          >
            <option value="">Без категории</option>
            {categories.map((cat) => (
              <option key={cat.id} value={cat.id}>
                {cat.name}
              </option>
            ))}
          </select>
        </div>

        {/* Complete Dialogue Action */}
        {!isCompleted && (
          <div className="pt-1">
            {!isConfirmingComplete ? (
              <button
                type="button"
                data-testid="complete-dialogue-button"
                onClick={() => setIsConfirmingComplete(true)}
                className="w-full flex items-center justify-center gap-1.5 py-2 px-3 text-xs font-semibold text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 rounded-lg transition-colors shadow-sm"
              >
                <CheckCircle className="w-3.5 h-3.5 text-gray-500" />
                <span>Завершить диалог</span>
              </button>
            ) : (
              <div className="p-3 bg-red-50 border border-red-200 rounded-lg flex flex-col gap-2">
                <p className="text-xs text-red-900 font-medium">
                  Завершить текущий диалог с пользователем?
                </p>
                <div className="flex gap-2">
                  <button
                    type="button"
                    data-testid="confirm-complete-button"
                    onClick={handleCompleteConfirm}
                    className="flex-1 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold rounded-md shadow-sm transition-colors"
                  >
                    Да, завершить
                  </button>
                  <button
                    type="button"
                    onClick={() => setIsConfirmingComplete(false)}
                    className="flex-1 py-1.5 bg-white border border-gray-300 text-gray-700 hover:bg-gray-50 text-xs font-medium rounded-md transition-colors"
                  >
                    Отмена
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Requester Profile Section */}
      <div className="p-4 border-b border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          {isCustomer ? 'Информация о покупателе' : 'Информация о продавце'}
        </span>

        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-full bg-gray-100 flex items-center justify-center flex-shrink-0">
            {isCustomer ? (
              <User className="w-4 h-4 text-gray-600" />
            ) : (
              <Store className="w-4 h-4 text-amber-600" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <h4 className="text-sm font-semibold text-gray-900 truncate">
              {isCustomer
                ? conversation.requesterName || 'Покупатель ZAMK'
                : conversation.requesterStoreName || 'Магазин'}
            </h4>
            {isCustomer ? (
              <>
                {conversation.requesterEmail && (
                  <p className="text-xs text-gray-500 truncate mt-0.5">
                    {conversation.requesterEmail}
                  </p>
                )}
                {conversation.requesterPhone && (
                  <p className="text-xs text-gray-400 truncate mt-0.5">
                    {conversation.requesterPhone}
                  </p>
                )}
              </>
            ) : (
              <>
                {conversation.requesterSellerId && (
                  <Link
                    to={`/sellers/${conversation.requesterSellerId}`}
                    className="inline-flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800 font-medium mt-1"
                  >
                    <span>Карточка продавца</span>
                    <ExternalLink className="w-3 h-3" />
                  </Link>
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* Domain Context: Recent Orders */}
      <div className="p-4 border-b border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          Последние заказы
        </span>
        {loadingContext ? (
          <div className="space-y-2">
            <div className="h-4 bg-gray-100 rounded w-full animate-pulse" />
            <div className="h-4 bg-gray-100 rounded w-3/4 animate-pulse" />
          </div>
        ) : recentOrders.length === 0 ? (
          <p className="text-xs text-gray-400">Нет данных о заказах</p>
        ) : (
          <div className="space-y-2">
            {recentOrders.map((ord) => (
              <Link
                key={ord.id}
                to={`/orders/${ord.id}`}
                className="flex items-center justify-between p-2 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors text-xs group"
              >
                <div className="flex items-center gap-2 min-w-0">
                  <Package className="w-3.5 h-3.5 text-gray-500 flex-shrink-0" />
                  <span className="font-semibold text-gray-900 group-hover:text-blue-600 truncate">
                    {ord.orderNumber || ord.id.slice(0, 8)}
                  </span>
                </div>
                <span className="text-[11px] text-gray-400 flex-shrink-0">
                  {ord.totalAmountCents ? `${ord.totalAmountCents / 100} ₽` : ''}
                </span>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Domain Context: Recent Returns */}
      <div className="p-4 border-b border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          Возвраты
        </span>
        {loadingContext ? (
          <div className="space-y-2">
            <div className="h-4 bg-gray-100 rounded w-full animate-pulse" />
          </div>
        ) : recentReturns.length === 0 ? (
          <p className="text-xs text-gray-400">Нет зарегистрированных возвратов</p>
        ) : (
          <div className="space-y-2">
            {recentReturns.map((ret) => (
              <Link
                key={ret.id}
                to={`/returns?id=${ret.id}`}
                className="flex items-center justify-between p-2 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors text-xs group"
              >
                <div className="flex items-center gap-2 min-w-0">
                  <RotateCcw className="w-3.5 h-3.5 text-purple-600 flex-shrink-0" />
                  <span className="font-medium text-gray-800 group-hover:text-purple-600 truncate">
                    {ret.orderNumber ? `Возврат #${ret.orderNumber}` : `Возврат #${ret.id.slice(0, 8)}`}
                  </span>
                </div>
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-200 text-gray-700">
                  {ret.status || 'Новый'}
                </span>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Domain Context: Seller Products */}
      {!isCustomer && (
        <div className="p-4">
          <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
            Товары продавца
          </span>
          {loadingContext ? (
            <div className="space-y-2">
              <div className="h-4 bg-gray-100 rounded w-full animate-pulse" />
            </div>
          ) : recentProducts.length === 0 ? (
            <p className="text-xs text-gray-400">Нет товаров продавца</p>
          ) : (
            <div className="space-y-2">
              {recentProducts.map((prod) => (
                <Link
                  key={prod.id}
                  to={`/products/${prod.id}`}
                  className="flex items-center justify-between p-2 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors text-xs group"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <ShoppingBag className="w-3.5 h-3.5 text-emerald-600 flex-shrink-0" />
                    <span className="font-medium text-gray-800 group-hover:text-emerald-600 truncate">
                      {prod.title}
                    </span>
                  </div>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-200 text-gray-700 flex-shrink-0">
                    {prod.status}
                  </span>
                </Link>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

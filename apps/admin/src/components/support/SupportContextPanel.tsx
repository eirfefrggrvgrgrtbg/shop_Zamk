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
  ChevronRight,
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
  onContextClick?: (type: 'ORDER' | 'RETURN' | 'PRODUCT', id: string) => void;
}

export function SupportContextPanel({
  conversation,
  onUpdateSession,
  onCompleteSession,
  onClose,
  isOpen,
  onContextClick,
}: SupportContextPanelProps) {
  const [staffList, setStaffList] = useState<StaffMemberView[]>([]);
  const [, setCategories] = useState<SupportCategory[]>([]);
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
      className="w-80 sm:w-96 md:w-[410px] flex-shrink-0 bg-white border-l border-gray-200 h-full flex flex-col overflow-y-auto select-none"
    >
      {/* Panel Top Header */}
      <div className="p-4 border-b border-gray-100 flex items-center justify-between sticky top-0 bg-white z-10">
        <div>
          <h3 className="text-sm font-bold text-gray-900">Детали</h3>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors cursor-pointer"
          title="Скрыть панель"
          aria-label="Скрыть панель"
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

      {/* Section 1: Requester Profile */}
      <div className="p-4 border-b border-gray-100 bg-white">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          {isCustomer ? 'Информация о покупателе' : 'Информация о продавце'}
        </span>

        <div className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/70 border border-gray-100">
          <div className="w-10 h-10 rounded-full bg-white border border-gray-200/80 shadow-sm flex items-center justify-center flex-shrink-0">
            {isCustomer ? (
              <User className="w-5 h-5 text-gray-600" />
            ) : (
              <Store className="w-5 h-5 text-amber-600" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h4 className="text-xs font-semibold text-gray-900 truncate">
                {isCustomer
                  ? conversation.requesterName || 'Покупатель ZAMK'
                  : conversation.requesterStoreName || 'Магазин'}
              </h4>
              <span
                className={`text-[10px] font-medium px-1.5 py-0.5 rounded ${
                  isCustomer ? 'bg-blue-50 text-blue-700' : 'bg-amber-50 text-amber-700'
                }`}
              >
                {isCustomer ? 'Покупатель' : 'Продавец'}
              </span>
            </div>
            {isCustomer ? (
              <>
                {conversation.requesterEmail && (
                  <p className="text-[11px] text-gray-600 truncate mt-0.5 font-mono">
                    {conversation.requesterEmail}
                  </p>
                )}
                {conversation.requesterPhone && (
                  <p className="text-[11px] text-gray-500 truncate mt-0.5">
                    {conversation.requesterPhone}
                  </p>
                )}
              </>
            ) : (
              <>
                {conversation.requesterSellerId && (
                  <Link
                    to={`/sellers/${conversation.requesterSellerId}`}
                    className="inline-flex items-center gap-1 text-[11px] text-blue-600 hover:text-blue-800 font-medium mt-1 group"
                  >
                    <span>Карточка продавца</span>
                    <ExternalLink className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" />
                  </Link>
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* Section 2: Dialogue Parameters */}
      <div className="p-4 border-b border-gray-100 flex flex-col gap-3.5 bg-gray-50/40">
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
          <label className="block text-[11px] font-medium text-gray-600 mb-1">Исполнитель</label>
          <div className="relative">
            <select
              data-testid="assignee-select"
              disabled={isCompleted}
              value={activeSession?.assignedTo || ''}
              onChange={(e) => handleAssigneeChange(e.target.value)}
              className="w-full text-xs bg-white border border-gray-200 rounded-lg px-2.5 py-2 focus:outline-none focus:ring-1 focus:ring-black text-gray-900 disabled:bg-gray-100 disabled:text-gray-400 shadow-sm transition-colors hover:border-gray-300"
            >
              <option value="">Не назначен</option>
              {staffList.map((staff) => (
                <option key={staff.userId} value={staff.userId}>
                  {staff.name} ({staff.email})
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Priority Picker */}
        <div>
          <label className="block text-[11px] font-medium text-gray-600 mb-1">Приоритет</label>
          <div className="relative">
            <select
              data-testid="priority-select"
              disabled={isCompleted}
              value={activeSession?.priority || 'NORMAL'}
              onChange={(e) => handlePriorityChange(e.target.value as SupportSessionPriority)}
              className="w-full text-xs bg-white border border-gray-200 rounded-lg px-2.5 py-2 focus:outline-none focus:ring-1 focus:ring-black text-gray-900 disabled:bg-gray-100 disabled:text-gray-400 shadow-sm transition-colors hover:border-gray-300"
            >
              <option value="NORMAL">Обычный</option>
              <option value="HIGH">Высокий</option>
              <option value="URGENT">Срочный</option>
            </select>
          </div>
        </div>
      </div>

      {/* Section 3: Domain Context - Recent Orders */}
      <div className="p-4 border-b border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          Последние заказы
        </span>
        {loadingContext ? (
          <div className="space-y-2">
            <div className="h-9 bg-gray-100 rounded-lg w-full animate-pulse" />
            <div className="h-9 bg-gray-100 rounded-lg w-3/4 animate-pulse" />
          </div>
        ) : recentOrders.length === 0 ? (
          <p className="text-xs text-gray-400">Нет данных о заказах</p>
        ) : (
          <div className="space-y-1.5">
            {recentOrders.map((ord) => {
              if (onContextClick) {
                return (
                  <button
                    key={ord.id}
                    type="button"
                    onClick={() => onContextClick('ORDER', ord.id)}
                    className="w-full text-left flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 active:bg-gray-200 transition-all text-xs group cursor-pointer"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center flex-shrink-0 group-hover:border-blue-300 transition-colors">
                        <Package className="w-3.5 h-3.5 text-gray-600 group-hover:text-blue-600 transition-colors" />
                      </div>
                      <div className="min-w-0">
                        <span className="font-semibold text-gray-900 group-hover:text-blue-600 truncate block">
                          {ord.orderNumber || ord.id.slice(0, 8)}
                        </span>
                        {ord.totalAmountCents ? (
                          <span className="text-[10px] text-gray-500 font-mono">
                            {ord.totalAmountCents / 100} ₽
                          </span>
                        ) : null}
                      </div>
                    </div>
                    <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all flex-shrink-0 ml-2" />
                  </button>
                );
              }
              return (
                <Link
                  key={ord.id}
                  to={`/orders/${ord.id}`}
                  className="flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 active:bg-gray-200 transition-all text-xs group cursor-pointer"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center flex-shrink-0 group-hover:border-blue-300 transition-colors">
                      <Package className="w-3.5 h-3.5 text-gray-600 group-hover:text-blue-600 transition-colors" />
                    </div>
                    <div className="min-w-0">
                      <span className="font-semibold text-gray-900 group-hover:text-blue-600 truncate block">
                        {ord.orderNumber || ord.id.slice(0, 8)}
                      </span>
                      {ord.totalAmountCents ? (
                        <span className="text-[10px] text-gray-500 font-mono">
                          {ord.totalAmountCents / 100} ₽
                        </span>
                      ) : null}
                    </div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all flex-shrink-0 ml-2" />
                </Link>
              );
            })}
          </div>
        )}
      </div>

      {/* Section 3: Domain Context - Recent Returns */}
      <div className="p-4 border-b border-gray-100">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
          Возвраты
        </span>
        {loadingContext ? (
          <div className="space-y-2">
            <div className="h-9 bg-gray-100 rounded-lg w-full animate-pulse" />
          </div>
        ) : recentReturns.length === 0 ? (
          <p className="text-xs text-gray-400">Нет зарегистрированных возвратов</p>
        ) : (
          <div className="space-y-1.5">
            {recentReturns.map((ret) => {
              if (onContextClick) {
                return (
                  <button
                    key={ret.id}
                    type="button"
                    onClick={() => onContextClick('RETURN', ret.id)}
                    className="w-full text-left flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 active:bg-gray-200 transition-all text-xs group cursor-pointer"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center flex-shrink-0 group-hover:border-purple-300 transition-colors">
                        <RotateCcw className="w-3.5 h-3.5 text-purple-600 transition-colors" />
                      </div>
                      <div className="min-w-0">
                        <span className="font-semibold text-gray-900 group-hover:text-purple-600 truncate block">
                          {ret.orderNumber ? `Возврат #${ret.orderNumber}` : `Возврат #${ret.id.slice(0, 8)}`}
                        </span>
                        <span className="text-[10px] text-gray-500 block">
                          {ret.status || 'Новый'}
                        </span>
                      </div>
                    </div>
                    <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all flex-shrink-0 ml-2" />
                  </button>
                );
              }
              return (
                <Link
                  key={ret.id}
                  to={`/returns?id=${ret.id}`}
                  className="flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 active:bg-gray-200 transition-all text-xs group cursor-pointer"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center flex-shrink-0 group-hover:border-purple-300 transition-colors">
                      <RotateCcw className="w-3.5 h-3.5 text-purple-600 transition-colors" />
                    </div>
                    <div className="min-w-0">
                      <span className="font-semibold text-gray-900 group-hover:text-purple-600 truncate block">
                        {ret.orderNumber ? `Возврат #${ret.orderNumber}` : `Возврат #${ret.id.slice(0, 8)}`}
                      </span>
                      <span className="text-[10px] text-gray-500 block">
                        {ret.status || 'Новый'}
                      </span>
                    </div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all flex-shrink-0 ml-2" />
                </Link>
              );
            })}
          </div>
        )}
      </div>

      {/* Section 3: Domain Context - Seller Products */}
      {!isCustomer && (
        <div className="p-4 border-b border-gray-100">
          <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2.5">
            Товары продавца
          </span>
          {loadingContext ? (
            <div className="space-y-2">
              <div className="h-9 bg-gray-100 rounded-lg w-full animate-pulse" />
            </div>
          ) : recentProducts.length === 0 ? (
            <p className="text-xs text-gray-400">Нет товаров продавца</p>
          ) : (
            <div className="space-y-1.5">
              {recentProducts.map((prod) => (
                <Link
                  key={prod.id}
                  to={`/products/${prod.id}`}
                  className="flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 active:bg-gray-200 transition-all text-xs group cursor-pointer"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center flex-shrink-0 group-hover:border-emerald-300 transition-colors">
                      <ShoppingBag className="w-3.5 h-3.5 text-emerald-600 transition-colors" />
                    </div>
                    <div className="min-w-0">
                      <span className="font-semibold text-gray-900 group-hover:text-emerald-600 truncate block">
                        {prod.title}
                      </span>
                      <span className="text-[10px] text-gray-500 block">
                        {prod.status}
                      </span>
                    </div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-400 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all flex-shrink-0 ml-2" />
                </Link>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Section 4: Secondary Bottom Action - Complete Dialogue */}
      {!isCompleted && (
        <div className="p-4 mt-auto bg-gray-50/50 border-t border-gray-100">
          {!isConfirmingComplete ? (
            <button
              type="button"
              data-testid="complete-dialogue-button"
              onClick={() => setIsConfirmingComplete(true)}
              className="w-full flex items-center justify-center gap-1.5 py-2 px-3 text-xs font-medium text-gray-600 bg-white border border-gray-200 hover:bg-gray-50 hover:text-gray-900 rounded-lg transition-colors shadow-2xs cursor-pointer"
            >
              <CheckCircle className="w-3.5 h-3.5 text-gray-400" />
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
                  className="flex-1 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold rounded-md shadow-sm transition-colors cursor-pointer"
                >
                  Да, завершить
                </button>
                <button
                  type="button"
                  onClick={() => setIsConfirmingComplete(false)}
                  className="flex-1 py-1.5 bg-white border border-gray-300 text-gray-700 hover:bg-gray-50 text-xs font-medium rounded-md transition-colors cursor-pointer"
                >
                  Отмена
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

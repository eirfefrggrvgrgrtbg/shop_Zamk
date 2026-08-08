import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useLocation, useSearchParams } from 'react-router-dom';
import {
  getAdminSellerDetail,
  getAdminSellerOverview,
  updateAdminSellerStatus,
  resetAdminSellerOwnerPassword,
  listSellerWarnings,
  createSellerWarning,
  listSellerViolations,
  createSellerViolation,
  listSellerNotes,
  createSellerNote,
  listImprovementPlans,
  createImprovementPlan,
  updateImprovementPlanStatus,
  getAdminOrders,
  getAdminProducts,
  getAdminPayouts,
  getAdminSellerCommissionHistory,
  setAdminSellerCommission,
} from '@zamk/api-client/src/admin';
import type { AdminOrder, AdminProduct, AdminPayout } from '@zamk/api-client/src/types';
import type {
  SellerDetail,
  SellerOverviewData,
  SellerWarning,
  SellerViolation,
  SellerNote,
  SellerImprovementPlan,
} from '@zamk/api-client/src/types';
import {
  ArrowLeft,
  Shield,
  Store as StoreIcon,
  AlertTriangle,
  Key,
  Plus,
  ExternalLink,
  ShoppingBag,
  TrendingUp,
  Package,
  DollarSign,
  Star,
} from 'lucide-react';
import { PermissionGuard } from '../components/PermissionGuard';
import { CustomSelect } from '../components/CustomSelect';
import { formatStatus, getStatusBadgeClass } from '../utils/statusMapper';

const SELLER_STATUS_LABELS: Record<string, string> = {
  pending_setup: 'Ожидает настройки',
  pending_review: 'Ожидает проверки',
  pending: 'Ожидает активации',
  active: 'Активен',
  blocked: 'Заблокирован',
  archived: 'В архиве',
};

const SELLER_STATUS_BADGE: Record<string, string> = {
  active: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300 border border-green-200 dark:border-green-800',
  blocked: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300 border border-red-200 dark:border-red-800',
  pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300 border border-yellow-200 dark:border-yellow-800',
  pending_setup: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300 border border-amber-200 dark:border-amber-800',
  pending_review: 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300 border border-blue-200 dark:border-blue-800',
  archived: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300 border border-gray-200 dark:border-gray-700',
};

const OWNER_STATUS_LABELS: Record<string, string> = {
  active: 'Активен',
  blocked: 'Заблокирован',
  pending: 'Ожидает активации',
};

const OWNER_STATUS_BADGE: Record<string, string> = {
  active: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300',
  blocked: 'bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-300',
  pending: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
};

const WARNING_TYPES = [
  { value: 'late_shipment', label: 'Поздняя отправка' },
  { value: 'wrong_item', label: 'Неверный товар' },
  { value: 'no_shipment', label: 'Нет отправки' },
  { value: 'poor_packaging', label: 'Плохая упаковка' },
  { value: 'customer_complaint', label: 'Жалоба покупателя' },
  { value: 'moderation_issue', label: 'Нарушение модерации' },
  { value: 'return_problem', label: 'Проблема с возвратом' },
  { value: 'other', label: 'Другое' },
];

const VIOLATION_TYPES = [
  { value: 'no_shipment', label: 'Нет отправки' },
  { value: 'late_shipment', label: 'Поздняя отправка' },
  { value: 'wrong_item', label: 'Неверный товар' },
  { value: 'fake_product', label: 'Поддельный товар' },
  { value: 'damaged_item_not_disclosed', label: 'Скрытый дефект' },
  { value: 'repeated_customer_complaints', label: 'Повторные жалобы' },
  { value: 'return_abuse', label: 'Злоупотребление возвратами' },
  { value: 'moderation_violation', label: 'Нарушение правил модерации' },
  { value: 'other', label: 'Другое' },
];

const SEVERITY_OPTIONS = [
  { value: 'low', label: 'Низкая' },
  { value: 'medium', label: 'Средняя' },
  { value: 'high', label: 'Высокая' },
];

function Badge({ label, className }: { label: string; className: string }) {
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${className}`}>
      {label}
    </span>
  );
}

function formatCurrency(cents: number): string {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    maximumFractionDigits: 0,
  }).format(cents / 100);
}

export function AdminSellerDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();

  const activeTab = (searchParams.get('tab') as 'overview' | 'sales' | 'catalog' | 'finance' | 'quality' | 'control' | 'access') || 'overview';
  const period = searchParams.get('period') || '30d';

  const [detail, setDetail] = useState<SellerDetail | null>(null);
  const [overview, setOverview] = useState<SellerOverviewData | null>(null);
  const [warnings, setWarnings] = useState<SellerWarning[]>([]);
  const [violations, setViolations] = useState<SellerViolation[]>([]);
  const [notes, setNotes] = useState<SellerNote[]>([]);
  const [plans, setPlans] = useState<SellerImprovementPlan[]>([]);
  const [recentOrders, setRecentOrders] = useState<AdminOrder[]>([]);
  const [recentProducts, setRecentProducts] = useState<AdminProduct[]>([]);
  const [recentPayouts, setRecentPayouts] = useState<AdminPayout[]>([]);
  const [commissionHistory, setCommissionHistory] = useState<any[]>([]);
  const [showNoteModal, setShowNoteModal] = useState(false);
  const [noteContent, setNoteContent] = useState('');
  const [noteType, setNoteType] = useState('note');
  const [showPlanModal, setShowPlanModal] = useState(false);
  const [planReason, setPlanReason] = useState('');
  const [planInternalComment, setPlanInternalComment] = useState('');
  const [planActions, setPlanActions] = useState<string[]>(['']);
  const [planDeadline, setPlanDeadline] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Status modal
  const [showStatusModal, setShowStatusModal] = useState(false);
  const [newStatus, setNewStatus] = useState<string>('');
  const [statusReason, setStatusReason] = useState<string>('');

  // Password reset modal
  const [tempPasswordModal, setTempPasswordModal] = useState<string | null>(null);

  // Warnings / Violations modals
  const [showWarningModal, setShowWarningModal] = useState(false);
  const [warningType, setWarningType] = useState('other');
  const [warningTitle, setWarningTitle] = useState('');
  const [warningMessage, setWarningMessage] = useState('');
  const [warningSeverity, setWarningSeverity] = useState<'low' | 'medium' | 'high'>('medium');

  // Commission Modal
  const [showCommissionModal, setShowCommissionModal] = useState(false);
  const [commissionRatePercent, setCommissionRatePercent] = useState('8.5');
  const [commissionReason, setCommissionReason] = useState('');

  const [showViolationModal, setShowViolationModal] = useState(false);
  const [violationType, setViolationType] = useState('other');
  const [violationTitle, setViolationTitle] = useState('');
  const [violationDescription, setViolationDescription] = useState('');
  const [violationSeverity, setViolationSeverity] = useState<'low' | 'medium' | 'high'>('medium');

  const updateUrlParam = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams);
    params.set(key, value);
    setSearchParams(params);
  };

  const loadData = useCallback(async () => {
    if (!id) return;
    setIsLoading(true);
    setError(null);
    try {
      const [detailData, overviewData, warnRes, violRes, notesRes, plansRes, ordersRes, productsRes, payoutsRes, commRes] = await Promise.all([
        getAdminSellerDetail(id),
        getAdminSellerOverview(id, period).catch(() => null),
        listSellerWarnings(id).catch(() => ({ items: [] })),
        listSellerViolations(id).catch(() => ({ items: [] })),
        listSellerNotes(id).catch(() => ({ items: [] })),
        listImprovementPlans(id).catch(() => ({ items: [] })),
        getAdminOrders({ sellerId: id, limit: 10 }).catch(() => ({ items: [] })),
        getAdminProducts(1, 10, { sellerId: id }).catch(() => ({ items: [] })),
        getAdminPayouts({ sellerId: id, limit: 5 }).catch(() => ({ items: [] })),
        getAdminSellerCommissionHistory(id).catch(() => []),
      ]);
      setDetail(detailData);
      setOverview(overviewData);
      setWarnings(warnRes.items || []);
      setViolations(violRes.items || []);
      setNotes(notesRes.items || []);
      setPlans(plansRes.items || []);
      setRecentOrders(ordersRes.items || []);
      setRecentProducts(productsRes.items || []);
      setRecentPayouts(payoutsRes.items || []);
      setCommissionHistory(Array.isArray(commRes) ? commRes : []);
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки данных продавца');
    } finally {
      setIsLoading(false);
    }
  }, [id, period]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black dark:border-white"></div>
      </div>
    );
  }

  if (error || !detail) {
    return (
      <div className="p-6">
        <button
          onClick={() => navigate('/sellers' + location.search)}
          className="inline-flex items-center text-sm font-medium text-gray-600 hover:text-black dark:text-gray-400 dark:hover:text-white mb-4"
        >
          <ArrowLeft className="h-4 w-4 mr-1" /> К продавцам
        </button>
        <div className="bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 p-4 rounded-xl text-sm">
          {error || 'Продавец не найден'}
        </div>
      </div>
    );
  }

  const handleUpdateStatus = async () => {
    if (!id || !newStatus) return;
    if ((newStatus === 'blocked' || newStatus === 'archived') && !statusReason.trim()) {
      setError('Укажите причину изменения статуса');
      return;
    }
    try {
      await updateAdminSellerStatus(id, newStatus, statusReason.trim() || undefined);
      setShowStatusModal(false);
      setStatusReason('');
      setSuccess('Статус продавца обновлен');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка обновления статуса');
    }
  };

  const handleResetPassword = async () => {
    if (!id) return;
    if (!confirm('Вы уверены, что хотите сбросить пароль владельца?')) return;
    try {
      const res = await resetAdminSellerOwnerPassword(id);
      setTempPasswordModal(res.temporaryPassword);
    } catch (err: any) {
      setError(err.message || 'Ошибка сброса пароля');
    }
  };

  const handleUpdateCommission = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      const rateBps = Math.round(parseFloat(commissionRatePercent) * 100);
      if (isNaN(rateBps) || rateBps < 0 || rateBps > 10000) {
        setError('Неверный процент комиссии');
        return;
      }
      await setAdminSellerCommission(id, { rateBps, reason: commissionReason });
      setShowCommissionModal(false);
      setCommissionReason('');
      setSuccess('Комиссия обновлена');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка обновления комиссии');
    }
  };
  const handleCreateNote = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      await createSellerNote(id, { noteType, content: noteContent });
      setShowNoteModal(false);
      setNoteContent('');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка создания заметки');
    }
  };

  const handleCreatePlan = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      const validActions = planActions.filter(a => a.trim().length > 0).map(a => ({ title: a.trim(), isCompleted: false }));
      if (validActions.length === 0) {
        setError('Необходимо добавить хотя бы одно действие');
        return;
      }
      await createImprovementPlan(id, {
        reason: planReason,
        internalComment: planInternalComment,
        deadline: planDeadline ? new Date(planDeadline).toISOString() : undefined,
        actions: validActions
      });
      setShowPlanModal(false);
      setPlanReason('');
      setPlanInternalComment('');
      setPlanActions(['']);
      setPlanDeadline('');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка создания плана');
    }
  };

  const handleUpdatePlanStatus = async (planId: string, status: string) => {
    if (!id) return;
    try {
      await updateImprovementPlanStatus(id, planId, status);
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка обновления статуса');
    }
  };

  const handleCreateWarning = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      await createSellerWarning(id, {
        type: warningType,
        title: warningTitle,
        message: warningMessage,
        severity: warningSeverity,
      });
      setShowWarningModal(false);
      setWarningTitle('');
      setWarningMessage('');
      setSuccess('Предупреждение создано');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка создания предупреждения');
    }
  };

  const handleCreateViolation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    try {
      await createSellerViolation(id, {
        type: violationType,
        title: violationTitle,
        description: violationDescription,
        severity: violationSeverity,
        countsForPenalty: true,
      });
      setShowViolationModal(false);
      setViolationTitle('');
      setViolationDescription('');
      setSuccess('Нарушение зафиксировано');
      loadData();
    } catch (err: any) {
      setError(err.message || 'Ошибка создания нарушения');
    }
  };

  // Compute "Requires Attention" items (only items > 0)
  const attentionItems: { label: string; count: number; tab: 'sales' | 'catalog' | 'finance' | 'control' }[] = [];
  if (overview) {
    if (overview.fulfillment.fulfillmentsProblematic > 0) {
      attentionItems.push({ label: 'Проблемные заказы', count: overview.fulfillment.fulfillmentsProblematic, tab: 'sales' });
    }
    if (overview.quality.openReturns > 0) {
      attentionItems.push({ label: 'Открытые возвраты', count: overview.quality.openReturns, tab: 'sales' });
    }
    if (overview.catalog.productsModeration > 0) {
      attentionItems.push({ label: 'Товары на модерации', count: overview.catalog.productsModeration, tab: 'catalog' });
    }
    if (overview.catalog.productsRejected > 0) {
      attentionItems.push({ label: 'Отклонённые товары', count: overview.catalog.productsRejected, tab: 'catalog' });
    }
    if (overview.catalog.productsOutOfStock > 0) {
      attentionItems.push({ label: 'Товары без остатка', count: overview.catalog.productsOutOfStock, tab: 'catalog' });
    }
    if (overview.catalog.productsLowStock > 0) {
      attentionItems.push({ label: 'Низкий остаток на складе', count: overview.catalog.productsLowStock, tab: 'catalog' });
    }
    if (overview.quality.warningsActive > 0) {
      attentionItems.push({ label: 'Активные предупреждения', count: overview.quality.warningsActive, tab: 'control' });
    }
    if (overview.quality.violationsActive > 0) {
      attentionItems.push({ label: 'Активные нарушения', count: overview.quality.violationsActive, tab: 'control' });
    }
  }

  const storeTitle = detail.brandName ? detail.brandName : 'Продавец без магазина';

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-6">
      {/* Notifications */}
      {success && (
        <div className="p-4 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-800 text-green-700 dark:text-green-300 rounded-2xl flex items-center justify-between text-sm">
          <span>{success}</span>
          <button onClick={() => setSuccess(null)} className="font-bold text-xs">✕</button>
        </div>
      )}

      {/* Breadcrumb & Navigation Back */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-2 text-xs text-gray-500 dark:text-gray-400">
          <span>ZAMK Admin</span>
          <span>/</span>
          <button
            onClick={() => navigate('/sellers' + location.search)}
            className="hover:underline hover:text-gray-900 dark:hover:text-white"
          >
            Продавцы
          </button>
          <span>/</span>
          <span className="font-semibold text-gray-900 dark:text-white">{storeTitle}</span>
        </div>

        <button
          onClick={() => navigate('/sellers' + location.search)}
          className="inline-flex items-center px-3 py-1.5 rounded-xl border border-gray-200 dark:border-gray-700 text-xs font-semibold text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
        >
          <ArrowLeft className="h-3.5 w-3.5 mr-1" />
          <span>К продавцам</span>
        </button>
      </div>

      {/* Main Header */}
      <div className="bg-white dark:bg-gray-800 rounded-3xl border border-gray-200 dark:border-gray-700 p-6 shadow-sm">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6">
          <div className="flex items-start space-x-4">
            <div className="w-14 h-14 rounded-2xl bg-gray-100 dark:bg-gray-700 flex items-center justify-center shrink-0 border border-gray-200 dark:border-gray-600 overflow-hidden">
              {detail.logoUrl ? (
                <img src={detail.logoUrl} alt={storeTitle} className="w-full h-full object-cover" />
              ) : (
                <StoreIcon className="w-7 h-7 text-gray-400" />
              )}
            </div>

            <div className="space-y-1">
              <div className="flex items-center space-x-3 flex-wrap gap-y-1">
                <h1 className="text-xl font-bold text-gray-900 dark:text-white">
                  {storeTitle}
                </h1>
                <Badge
                  label={SELLER_STATUS_LABELS[detail.status] || detail.status}
                  className={SELLER_STATUS_BADGE[detail.status] || 'bg-gray-100 text-gray-700'}
                />
              </div>

              <div className="flex items-center space-x-3 text-xs text-gray-500 dark:text-gray-400 flex-wrap">
                <span>Владелец: <strong className="text-gray-700 dark:text-gray-200">{detail.owner?.name || '—'}</strong> ({detail.owner?.email || '—'})</span>
                <span>·</span>
                <span>Доступ владельца: <Badge label={OWNER_STATUS_LABELS[detail.owner?.status] || detail.owner?.status || 'pending'} className={OWNER_STATUS_BADGE[detail.owner?.status] || 'bg-gray-100'} /></span>
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center space-x-2 flex-wrap gap-y-2">
            <PermissionGuard permission="sellers.update_status">
              <button
                onClick={() => {
                  setNewStatus(detail.status);
                  setShowStatusModal(true);
                }}
                className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black rounded-xl text-xs font-bold hover:opacity-90 transition-opacity"
              >
                Изменить статус
              </button>
            </PermissionGuard>

            {detail.slug && (
              <a
                href={`/shop/sellers/${detail.slug}`}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center px-4 py-2 border border-gray-200 dark:border-gray-700 rounded-xl text-xs font-semibold text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
              >
                <span>Открыть магазин</span>
                <ExternalLink className="w-3.5 h-3.5 ml-1.5" />
              </a>
            )}
          </div>
        </div>
      </div>

      {/* Navigation Tabs Bar */}
      <div className="border-b border-gray-200 dark:border-gray-700 overflow-x-auto pb-0 -mx-4 px-4 sm:mx-0 sm:px-0">
        <nav className="flex space-x-6 min-w-max">
          {[
            { id: 'overview', label: 'Обзор', icon: TrendingUp },
            { id: 'sales', label: 'Продажи и заказы', icon: ShoppingBag },
            { id: 'catalog', label: 'Каталог и склад', icon: Package },
            { id: 'finance', label: 'Финансы', icon: DollarSign },
            { id: 'quality', label: 'Качество и эффективность', icon: Star },
            { id: 'control', label: 'Контроль', icon: Shield },
            { id: 'access', label: 'Магазин и доступ', icon: Key },
          ].map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => updateUrlParam('tab', tab.id)}
                className={`flex items-center space-x-2 py-3 px-1 border-b-2 text-xs font-bold transition-all ${
                  isActive
                    ? 'border-black dark:border-white text-gray-900 dark:text-white'
                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
      </div>

      {/* TAB 1: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Period Selector */}
          <div className="flex items-center justify-between bg-gray-50 dark:bg-gray-900/50 p-3 rounded-2xl border border-gray-200 dark:border-gray-700">
            <span className="text-xs font-bold text-gray-500 dark:text-gray-400 uppercase tracking-wider pl-1">
              Период аналитики:
            </span>
            <div className="flex space-x-1 bg-white dark:bg-gray-800 p-1 rounded-xl border border-gray-200 dark:border-gray-700">
              {[
                { id: '7d', label: '7 дней' },
                { id: '30d', label: '30 дней' },
                { id: 'all', label: 'Всё время' },
              ].map((p) => (
                <button
                  key={p.id}
                  onClick={() => updateUrlParam('period', p.id)}
                  className={`px-3 py-1 rounded-lg text-xs font-semibold transition-all ${
                    period === p.id
                      ? 'bg-black text-white dark:bg-white dark:text-black shadow-sm'
                      : 'text-gray-600 dark:text-gray-400 hover:text-black dark:hover:text-white'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          {/* 6 Key Indicators */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Оборот</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.sales.grossSalesCents) : '—'}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Заказы</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? overview.sales.ordersCount : '—'}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Продано товаров</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? overview.sales.itemsSold : '—'}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Средний чек</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.sales.averageOrderValueCents) : '—'}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Возвраты и отмены</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? overview.sales.cancelledOrders + overview.sales.returnedOrders : '—'}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm">
              <div className="text-xs text-gray-500 dark:text-gray-400 font-medium">Ожидает выплаты</div>
              <div className="text-2xl font-black text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.finance.pendingPayoutCents) : '—'}
              </div>
            </div>
          </div>

          {/* Requires Attention Section (only > 0) */}
          {attentionItems.length > 0 && (
            <div className="bg-amber-50 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-900/60 rounded-2xl p-5 space-y-3">
              <div className="flex items-center space-x-2 text-amber-800 dark:text-amber-300 font-bold text-sm">
                <AlertTriangle className="w-4 h-4" />
                <span>Требует внимания ({attentionItems.length})</span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {attentionItems.map((item, idx) => (
                  <button
                    key={idx}
                    onClick={() => updateUrlParam('tab', item.tab)}
                    className="flex items-center justify-between p-3 bg-white dark:bg-gray-800 rounded-xl border border-amber-200 dark:border-amber-900/40 hover:bg-amber-100/50 dark:hover:bg-gray-700 transition-colors text-left"
                  >
                    <span className="text-xs font-semibold text-gray-900 dark:text-white">{item.label}</span>
                    <span className="text-xs font-bold px-2 py-0.5 bg-amber-100 dark:bg-amber-900/60 text-amber-900 dark:text-amber-200 rounded-full">
                      {item.count}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* 4 Summary Blocks */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Sales & Orders */}
            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">Продажи и заказы</h3>
                <button onClick={() => updateUrlParam('tab', 'sales')} className="text-xs text-gray-500 hover:underline">Подробнее →</button>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Доставлено</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.sales.deliveredOrders ?? 0}</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Отменено</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.sales.cancelledOrders ?? 0}</div>
                </div>
              </div>
            </div>

            {/* Catalog & Stock */}
            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">Каталог и склад</h3>
                <button onClick={() => updateUrlParam('tab', 'catalog')} className="text-xs text-gray-500 hover:underline">Подробнее →</button>
              </div>
              <div className="grid grid-cols-3 gap-2 text-xs">
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Всего</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.catalog.productsTotal ?? 0}</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Опубликовано</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.catalog.productsPublished ?? 0}</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Модерация</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.catalog.productsModeration ?? 0}</div>
                </div>
              </div>
            </div>

            {/* Finance */}
            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">Финансы</h3>
                <button onClick={() => updateUrlParam('tab', 'finance')} className="text-xs text-gray-500 hover:underline">Подробнее →</button>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Ожидает выплаты</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview ? formatCurrency(overview.finance.pendingPayoutCents) : '0 ₽'}</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Выплачено</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview ? formatCurrency(overview.finance.paidPayoutCents) : '0 ₽'}</div>
                </div>
              </div>
            </div>

            {/* Quality & Control */}
            <div className="bg-white dark:bg-gray-800 p-5 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">Качество и контроль</h3>
                <button onClick={() => updateUrlParam('tab', 'control')} className="text-xs text-gray-500 hover:underline">Подробнее →</button>
              </div>
              <div className="grid grid-cols-3 gap-2 text-xs">
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Рейтинг</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview ? overview.quality.rating.toFixed(1) : '5.0'} ★</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Предупреждения</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.quality.warningsActive ?? 0}</div>
                </div>
                <div className="p-2.5 bg-gray-50 dark:bg-gray-900/50 rounded-xl">
                  <div className="text-gray-400">Нарушения</div>
                  <div className="font-bold text-gray-900 dark:text-white text-sm">{overview?.quality.violationsActive ?? 0}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 2: SALES & ORDERS */}
      {activeTab === 'sales' && (
        <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="font-bold text-base text-gray-900 dark:text-white">Продажи и заказы</h2>
            <button
              onClick={() => navigate(`/orders?sellerId=${id}`)}
              className="text-xs font-semibold text-blue-600 dark:text-blue-400 hover:underline"
            >
              Перейти к списку заказов →
            </button>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Всего заказов</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.sales.ordersCount ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Доставлено</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.sales.deliveredOrders ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Отменено</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.sales.cancelledOrders ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Возвраты</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.sales.returnedOrders ?? 0}</div>
            </div>
          </div>

          <div className="mt-6">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white mb-3">Последние 10 заказов</h3>
            {recentOrders.length === 0 ? (
              <p className="text-xs text-gray-500">Нет заказов</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700 dark:text-gray-300">
                  <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 uppercase font-semibold">
                    <tr>
                      <th className="px-4 py-2.5 rounded-l-xl">Номер заказа</th>
                      <th className="px-4 py-2.5">Дата</th>
                      <th className="px-4 py-2.5">Сумма</th>
                      <th className="px-4 py-2.5">Fulfillment</th>
                      <th className="px-4 py-2.5">Время сборки</th>
                      <th className="px-4 py-2.5 rounded-r-xl">Действие</th>
                    </tr>
                  </thead>
                  <tbody>
                    {recentOrders.map(order => (
                      <tr key={order.id} className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                        <td className="px-4 py-3 font-mono font-bold">#{order.id.slice(0, 8)}</td>
                        <td className="px-4 py-3 text-gray-500">{new Date(order.createdAt).toLocaleDateString('ru-RU')}</td>
                        <td className="px-4 py-3 font-semibold text-gray-900 dark:text-white">{formatCurrency(order.totalPriceCents)}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium ${getStatusBadgeClass(order.fulfillmentStatus)}`}>
                            {formatStatus(order.fulfillmentStatus)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-gray-500 group relative cursor-help">
                          <span>Нет данных</span>
                          <span className="hidden group-hover:block absolute bottom-full left-0 mb-1 w-56 p-2 bg-black text-white text-[10px] rounded-lg shadow-lg z-20">
                            События начала и завершения сборки не были зафиксированы
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <button onClick={() => navigate(`/orders/${order.id}`)} className="text-blue-600 font-bold hover:underline">Перейти →</button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 3: CATALOG & STOCK */}
      {activeTab === 'catalog' && (
        <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="font-bold text-base text-gray-900 dark:text-white">Каталог и склад</h2>
            <button
              onClick={() => navigate(`/products?sellerId=${id}`)}
              className="text-xs font-semibold text-blue-600 dark:text-blue-400 hover:underline"
            >
              Перейти к товарам продавца →
            </button>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 text-xs">
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Всего товаров</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.catalog.productsTotal ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Опубликовано</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.catalog.productsPublished ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">На модерации</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.catalog.productsModeration ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Без остатка</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.catalog.productsOutOfStock ?? 0}</div>
            </div>
            <div className="p-3 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700">
              <div className="text-gray-500">Низкий остаток</div>
              <div className="text-lg font-bold text-gray-900 dark:text-white">{overview?.catalog.productsLowStock ?? 0}</div>
            </div>
          </div>

          <div className="mt-6">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white mb-3">Последние добавленные товары (10)</h3>
            {recentProducts.length === 0 ? (
              <p className="text-xs text-gray-500">Нет товаров</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700 dark:text-gray-300">
                  <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 uppercase font-semibold">
                    <tr>
                      <th className="px-4 py-2.5 rounded-l-xl">Товар</th>
                      <th className="px-4 py-2.5">Цена</th>
                      <th className="px-4 py-2.5">Сток (в наличии)</th>
                      <th className="px-4 py-2.5">Статус</th>
                      <th className="px-4 py-2.5 rounded-r-xl">Действие</th>
                    </tr>
                  </thead>
                  <tbody>
                    {recentProducts.map(prod => (
                      <tr key={prod.id} className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                        <td className="px-4 py-3 font-semibold text-gray-900 dark:text-white">{prod.title}</td>
                        <td className="px-4 py-3 font-semibold">{formatCurrency(prod.priceCents)}</td>
                        <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
                          {prod.inStock ? 'Остаток в наличии' : 'Остаток недоступен'}
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-medium ${getStatusBadgeClass(prod.status)}`}>
                            {formatStatus(prod.status)}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <button onClick={() => navigate(`/products/${prod.id}`)} className="text-blue-600 font-bold hover:underline">Перейти →</button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 4: FINANCE */}
      {activeTab === 'finance' && (
        <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-6">
          <h2 className="font-bold text-base text-gray-900 dark:text-white">Финансовые показатели</h2>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700">
              <div className="text-xs text-gray-500">Оплачено покупателями</div>
              <div className="text-xl font-bold text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.finance.paidByCustomersCents) : '0 ₽'}
              </div>
            </div>
            <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700">
              <div className="text-xs text-gray-500">Ожидает выплаты</div>
              <div className="text-xl font-bold text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.finance.pendingPayoutCents) : '0 ₽'}
              </div>
            </div>
            <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700">
              <div className="text-xs text-gray-500">Выплачено всего</div>
              <div className="text-xl font-bold text-gray-900 dark:text-white mt-1">
                {overview ? formatCurrency(overview.finance.paidPayoutCents) : '0 ₽'}
              </div>
            </div>
          </div>

          {/* Commission Note */}
          <div className="p-4 bg-gray-50 dark:bg-gray-900/40 rounded-2xl border border-gray-200 dark:border-gray-700 text-xs text-gray-600 dark:text-gray-400 flex items-center justify-between">
            <div>
              {overview?.finance.commissionConfigured ? (
                <span className="text-green-600 font-semibold">✓ Комиссия ZAMK настроена</span>
              ) : (
                <span className="text-amber-600 font-semibold">⚠ Комиссия для продавца не настроена</span>
              )}
              {overview && overview.sales.ordersCount > 0 && overview.finance.pendingPayoutCents === 0 && (
                <p className="text-[11px] text-gray-500 mt-1">
                  Выплата ещё не сформирована: заказы находятся в процессе завершения или не вошли в текущий расчётный период.
                </p>
              )}
            </div>
            <div className="flex items-center space-x-2 shrink-0 ml-4">
              <PermissionGuard permission="commission.manage">
                <button
                  onClick={() => setShowCommissionModal(true)}
                  className="px-3 py-2 bg-gray-200 dark:bg-gray-700 text-gray-900 dark:text-white font-bold rounded-xl hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors text-xs"
                >
                  Настроить комиссию
                </button>
              </PermissionGuard>
              <button
                onClick={() => navigate(`/payouts?sellerId=${id}`)}
                className="px-3 py-2 bg-black dark:bg-white text-white dark:text-black font-bold rounded-xl hover:opacity-80 transition-opacity text-xs"
              >
                Перейти к выплатам Seller →
              </button>
            </div>
          </div>

          <div className="mt-6">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white mb-3">История комиссии ZAMK</h3>
            {commissionHistory.length === 0 ? (
              <p className="text-xs text-gray-500">Нет записей о комиссии</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700 dark:text-gray-300">
                  <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 uppercase">
                    <tr>
                      <th className="px-4 py-2 rounded-l-xl">Ставка</th>
                      <th className="px-4 py-2">С какого периода</th>
                      <th className="px-4 py-2">Причина</th>
                      <th className="px-4 py-2 rounded-r-xl">Кем изменено</th>
                    </tr>
                  </thead>
                  <tbody>
                    {commissionHistory.map((historyItem) => (
                      <tr key={historyItem.id} className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                        <td className="px-4 py-3 font-semibold">{(historyItem.rateBps / 100).toFixed(2)}%</td>
                        <td className="px-4 py-3">{new Date(historyItem.effectiveFrom).toLocaleString('ru-RU')}</td>
                        <td className="px-4 py-3">{historyItem.reason || '—'}</td>
                        <td className="px-4 py-3">{historyItem.adminId || 'System'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="mt-6">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white mb-3">Последние выплаты (5)</h3>
            {recentPayouts.length === 0 ? (
              <p className="text-xs text-gray-500">Нет выплат</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-gray-700 dark:text-gray-300">
                  <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 uppercase">
                    <tr>
                      <th className="px-4 py-2 rounded-l-xl">ID</th>
                      <th className="px-4 py-2">Сумма</th>
                      <th className="px-4 py-2">Статус</th>
                      <th className="px-4 py-2">Дата</th>
                    </tr>
                  </thead>
                  <tbody>
                    {recentPayouts.map(payout => (
                      <tr key={payout.id} className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50">
                        <td className="px-4 py-3 font-mono">{payout.id.split('-')[0]}</td>
                        <td className="px-4 py-3 font-semibold text-green-600">{formatCurrency(payout.amountCents)}</td>
                        <td className="px-4 py-3">
                          <Badge label={payout.status} className={payout.status === 'paid' ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'} />
                        </td>
                        <td className="px-4 py-3 text-gray-500">{payout.createdAt ? new Date(payout.createdAt).toLocaleDateString('ru-RU') : '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

            {/* TAB 5: QUALITY & EFFICIENCY */}
      {activeTab === 'quality' && (
        <div className="space-y-6">
          <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-6">
            <h2 className="font-bold text-base text-gray-900 dark:text-white">Качество и эффективность</h2>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Показатели</h3>
                <div className="grid grid-cols-2 gap-3 text-xs">
                  <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700">
                    <div className="text-gray-500">Рейтинг</div>
                    <div className="text-xl font-bold text-gray-900 dark:text-white mt-1 flex items-center">
                      {overview ? overview.quality.rating.toFixed(1) : '5.0'} 
                      <Star className="w-4 h-4 ml-1 text-amber-400 fill-amber-400" />
                    </div>
                  </div>
                  <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700">
                    <div className="text-gray-500">Отзывы</div>
                    <div className="text-xl font-bold text-gray-900 dark:text-white mt-1">
                      {overview?.quality.reviewsCount ?? 0}
                    </div>
                  </div>
                </div>
              </div>
              
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Эффективность</h3>
                <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl border border-gray-200 dark:border-gray-700 space-y-3">
                  <div className="flex items-center space-x-2">
                    <span className="text-gray-500 text-xs">Статус:</span>
                    {overview?.performance?.category === 'high' && <Badge label="Высокая эффективность" className="bg-emerald-100 text-emerald-800" />}
                    {overview?.performance?.category === 'stable' && <Badge label="Стабильная работа" className="bg-blue-100 text-blue-800" />}
                    {overview?.performance?.category === 'attention' && <Badge label="Требует внимания" className="bg-amber-100 text-amber-800" />}
                    {overview?.performance?.category === 'low' && <Badge label="Низкая эффективность" className="bg-red-100 text-red-800" />}
                    {(!overview?.performance?.category || overview?.performance?.category === 'no_data') && <Badge label="Недостаточно данных" className="bg-gray-100 text-gray-600" />}
                  </div>
                  
                  {overview?.performance?.reasons && overview.performance.reasons.length > 0 && (
                    <div className="space-y-1 mt-2">
                      <span className="text-gray-500 text-xs">Факторы:</span>
                      <ul className="list-disc pl-4 text-xs text-gray-700 dark:text-gray-300 space-y-1">
                        {overview.performance.reasons.map((r, idx) => (
                          <li key={idx}>{r}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              </div>
            </div>
            
          </div>
        </div>
      )}

      {/* TAB 6: CONTROL (SANCTIONS & WARNINGS) */}
      {activeTab === 'control' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700">
            <div>
              <h2 className="font-bold text-base text-gray-900 dark:text-white">Контроль и санкции</h2>
              <p className="text-xs text-gray-500">Управление предупреждениями и нарушениями продавца</p>
            </div>

            <div className="flex space-x-2">
              <PermissionGuard permission="sellers.warn">
                <button
                  onClick={() => setShowWarningModal(true)}
                  className="px-3.5 py-2 bg-amber-500 text-white rounded-xl text-xs font-bold hover:bg-amber-600 transition-colors flex items-center"
                >
                  <Plus className="w-4 h-4 mr-1" />
                  <span>Создать предупреждение</span>
                </button>
              </PermissionGuard>

              <PermissionGuard permission="sellers.warn">
                <button
                  onClick={() => setShowViolationModal(true)}
                  className="px-3.5 py-2 bg-red-600 text-white rounded-xl text-xs font-bold hover:bg-red-700 transition-colors flex items-center"
                >
                  <Plus className="w-4 h-4 mr-1" />
                  <span>Создать нарушение</span>
                </button>
              </PermissionGuard>
            </div>
          </div>

          {/* Warnings List */}
          <div className="bg-white dark:bg-gray-800 rounded-3xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white">Предупреждения ({warnings.length})</h3>
            {warnings.length === 0 ? (
              <p className="text-xs text-gray-400 italic">Активных и прошлых предупреждений нет</p>
            ) : (
              <div className="divide-y divide-gray-100 dark:divide-gray-700">
                {warnings.map((w) => (
                  <div key={w.id} className="py-3 flex items-start justify-between">
                    <div>
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-xs text-gray-900 dark:text-white">{w.title}</span>
                        <Badge label={w.severity} className="bg-amber-100 text-amber-800 text-[10px]" />
                        <Badge label={w.status} className="bg-gray-100 text-gray-600 text-[10px]" />
                      </div>
                      <p className="text-xs text-gray-500 mt-1">{w.message}</p>
                    </div>
                    <span className="text-[10px] text-gray-400">{new Date(w.createdAt).toLocaleDateString('ru-RU')}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Violations List */}
          <div className="bg-white dark:bg-gray-800 rounded-3xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
            <h3 className="font-bold text-sm text-gray-900 dark:text-white">Нарушения ({violations.length})</h3>
            {violations.length === 0 ? (
              <p className="text-xs text-gray-400 italic">Фиксированных нарушений нет</p>
            ) : (
              <div className="divide-y divide-gray-100 dark:divide-gray-700">
                {violations.map((v) => (
                  <div key={v.id} className="py-3 flex items-start justify-between">
                    <div>
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-xs text-gray-900 dark:text-white">{v.title}</span>
                        <Badge label={v.severity} className="bg-red-100 text-red-800 text-[10px]" />
                        <Badge label={v.status} className="bg-gray-100 text-gray-600 text-[10px]" />
                      </div>
                      <p className="text-xs text-gray-500 mt-1">{v.description}</p>
                    </div>
                    <span className="text-[10px] text-gray-400">{new Date(v.createdAt).toLocaleDateString('ru-RU')}</span>
                  </div>
                ))}
              </div>
            )}
          </div>


          {/* Notes Section */}
          <div className="bg-white dark:bg-gray-800 rounded-3xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="font-bold text-sm text-gray-900 dark:text-white">Внутренние заметки ({notes.length})</h3>
              <button
                onClick={() => setShowNoteModal(true)}
                className="text-xs text-blue-600 dark:text-blue-400 font-bold hover:underline flex items-center"
              >
                <Plus className="w-4 h-4 mr-1" /> Добавить заметку
              </button>
            </div>
            {notes.length === 0 ? (
              <p className="text-xs text-gray-400 italic">Нет внутренних заметок</p>
            ) : (
              <div className="space-y-3">
                {notes.map(note => (
                  <div key={note.id} className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700 space-y-2">
                    <div className="flex justify-between items-start">
                      <Badge label={note.noteType} className="bg-gray-200 text-gray-800 text-[10px]" />
                      <span className="text-gray-400 text-[10px]">{new Date(note.createdAt).toLocaleString('ru-RU')}</span>
                    </div>
                    <p className="text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap">{note.content}</p>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Improvement Plans Section */}
          <div className="bg-white dark:bg-gray-800 rounded-3xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="font-bold text-sm text-gray-900 dark:text-white">Планы улучшения ({plans.length})</h3>
              <button
                onClick={() => setShowPlanModal(true)}
                className="text-xs text-blue-600 dark:text-blue-400 font-bold hover:underline flex items-center"
              >
                <Plus className="w-4 h-4 mr-1" /> Создать план
              </button>
            </div>
            {plans.length === 0 ? (
              <p className="text-xs text-gray-400 italic">Нет активных планов улучшения</p>
            ) : (
              <div className="space-y-3">
                {plans.map(plan => (
                  <div key={plan.id} className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-xl border border-gray-200 dark:border-gray-700 space-y-3">
                    <div className="flex justify-between items-start">
                      <Badge label={plan.status} className={plan.status === 'active' ? 'bg-amber-100 text-amber-800 text-[10px]' : 'bg-green-100 text-green-800 text-[10px]'} />
                      <span className="text-gray-400 text-[10px]">
                        {new Date(plan.createdAt).toLocaleString('ru-RU')}
                        {plan.creatorName && ` • Автор: ${plan.creatorName}`}
                      </span>
                    </div>
                    <p className="text-xs text-gray-700 dark:text-gray-300"><strong>Причина:</strong> {plan.reason}</p>
                    {plan.internalComment && (
                      <p className="text-xs text-gray-500 dark:text-gray-400"><strong>Комментарий:</strong> {plan.internalComment}</p>
                    )}
                    {plan.assigneeName && (
                      <p className="text-xs text-gray-500 dark:text-gray-400"><strong>Ответственный:</strong> {plan.assigneeName}</p>
                    )}
                    {plan.deadline && (
                      <p className="text-xs text-gray-500 dark:text-gray-400"><strong>Срок:</strong> {new Date(plan.deadline).toLocaleDateString('ru-RU')}</p>
                    )}
                    
                    {plan.actions && plan.actions.length > 0 && (
                      <div className="pl-3 mt-2 border-l-2 border-blue-500">
                        <p className="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-1">План действий:</p>
                        <ul className="list-disc list-inside text-xs text-gray-700 dark:text-gray-300 space-y-1">
                          {plan.actions.map((action, i) => (
                            <li key={i}>{action.title}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    
                    {plan.status === 'active' && (
                      <div className="pt-2 flex justify-end space-x-2 border-t border-gray-200 dark:border-gray-700 mt-2">
                        <button onClick={() => handleUpdatePlanStatus(plan.id, 'completed')} className="text-xs font-semibold px-3 py-1.5 bg-green-50 text-green-600 rounded-lg hover:bg-green-100">Завершить</button>
                        <button onClick={() => handleUpdatePlanStatus(plan.id, 'cancelled')} className="text-xs font-semibold px-3 py-1.5 bg-gray-100 text-gray-600 rounded-lg hover:bg-gray-200">Отменить</button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

        </div>
      )}

      {/* TAB 7: STORE & ACCESS */}
      {activeTab === 'access' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Store Section */}
          <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-4">
            <h2 className="font-bold text-base text-gray-900 dark:text-white">Информация о магазине</h2>

            {detail.brandName ? (
              <div className="space-y-3 text-xs">
                <div>
                  <span className="text-gray-400">Название магазина:</span>
                  <p className="font-semibold text-gray-900 dark:text-white">{detail.brandName}</p>
                </div>
                {detail.slug && (
                  <div>
                    <span className="text-gray-400">Slug магазина:</span>
                    <p className="font-mono text-gray-700 dark:text-gray-300">{detail.slug}</p>
                  </div>
                )}
                {detail.description && (
                  <div>
                    <span className="text-gray-400">Описание:</span>
                    <p className="text-gray-700 dark:text-gray-300">{detail.description}</p>
                  </div>
                )}
                {detail.contactEmail && (
                  <div>
                    <span className="text-gray-400">Контактный Email:</span>
                    <p className="text-gray-700 dark:text-gray-300">{detail.contactEmail}</p>
                  </div>
                )}
              </div>
            ) : (
              <div className="p-4 bg-gray-50 dark:bg-gray-900/50 rounded-2xl text-xs text-gray-500 italic">
                Магазин ещё не создан. Продавец пока не завершил настройку.
              </div>
            )}
          </div>

          {/* Access Section */}
          <div className="bg-white dark:bg-gray-800 p-6 rounded-3xl border border-gray-200 dark:border-gray-700 space-y-4">
            <h2 className="font-bold text-base text-gray-900 dark:text-white">Доступ владельца</h2>

            <div className="space-y-3 text-xs">
              <div>
                <span className="text-gray-400">Владелец:</span>
                <p className="font-semibold text-gray-900 dark:text-white">{detail.owner?.name || '—'}</p>
              </div>
              <div>
                <span className="text-gray-400">Email:</span>
                <p className="font-semibold text-gray-900 dark:text-white">{detail.owner?.email || '—'}</p>
              </div>
              <div>
                <span className="text-gray-400">Статус аккаунта:</span>
                <div className="mt-1">
                  <Badge label={OWNER_STATUS_LABELS[detail.owner?.status] || detail.owner?.status || 'pending'} className={OWNER_STATUS_BADGE[detail.owner?.status] || 'bg-gray-100'} />
                </div>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-100 dark:border-gray-700">
              <PermissionGuard permission="sellers.create_access">
                <button
                  onClick={handleResetPassword}
                  className="w-full py-2.5 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-900 dark:text-white text-xs font-bold rounded-xl transition-colors flex items-center justify-center space-x-2"
                >
                  <Key className="w-4 h-4" />
                  <span>Сбросить временный пароль</span>
                </button>
              </PermissionGuard>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: STATUS UPDATE */}
      {showStatusModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Изменение статуса продавца</h3>
            <div className="space-y-3 text-xs">
              <label className="block font-semibold text-gray-700 dark:text-gray-300">Новый статус:</label>
              <CustomSelect
                value={newStatus}
                onChange={(val) => setNewStatus(val)}
                options={[
                  { value: 'pending_setup', label: 'Ожидает настройки' },
                  { value: 'pending_review', label: 'Ожидает проверки' },
                  { value: 'active', label: 'Активен' },
                  { value: 'blocked', label: 'Заблокирован' },
                  { value: 'archived', label: 'В архиве' },
                ]}
              />

              <label className="block font-semibold text-gray-700 dark:text-gray-300 pt-2">Причина изменения:</label>
              <textarea
                value={statusReason}
                onChange={(e) => setStatusReason(e.target.value)}
                placeholder="Укажите причину для аудита..."
                className="w-full p-3 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white focus:outline-none"
                rows={3}
              />
            </div>

            <div className="flex justify-end space-x-2 pt-2">
              <button
                onClick={() => setShowStatusModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                onClick={handleUpdateStatus}
                className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black text-xs font-bold rounded-xl"
              >
                Сохранить
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: TEMP PASSWORD */}
      {tempPasswordModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Временный пароль сброшен</h3>
            <p className="text-xs text-gray-500">Передайте этот пароль продавцу для первого входа:</p>
            <div className="p-4 bg-gray-100 dark:bg-gray-900 rounded-2xl text-center font-mono font-bold text-lg text-gray-900 dark:text-white select-all">
              {tempPasswordModal}
            </div>
            <div className="flex justify-end">
              <button
                onClick={() => setTempPasswordModal(null)}
                className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black text-xs font-bold rounded-xl"
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: COMMISSION */}
      {showCommissionModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <form onSubmit={handleUpdateCommission} className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Изменение комиссии продавца</h3>
            <div className="space-y-3 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Ставка комиссии (%)</label>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={commissionRatePercent}
                  onChange={(e) => setCommissionRatePercent(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  placeholder="8.5"
                />
                <p className="text-[10px] text-gray-500 mt-1">Новая ставка применяется к будущим заказам.</p>
              </div>
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Основание (причина)</label>
                <textarea
                  required
                  value={commissionReason}
                  onChange={(e) => setCommissionReason(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={2}
                  placeholder="Например: Переход на базовый тариф..."
                />
              </div>
            </div>
            <div className="flex justify-end space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowCommissionModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black text-xs font-bold rounded-xl"
              >
                Установить
              </button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL: CREATE WARNING */}
      {showWarningModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <form onSubmit={handleCreateWarning} className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Создать предупреждение</h3>
            <div className="space-y-3 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Тип:</label>
                <CustomSelect
                  value={warningType}
                  onChange={(val) => setWarningType(val)}
                  options={WARNING_TYPES}
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Серьезность:</label>
                <CustomSelect
                  value={warningSeverity}
                  onChange={(val) => setWarningSeverity(val as any)}
                  options={SEVERITY_OPTIONS}
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Заголовок:</label>
                <input
                  type="text"
                  required
                  value={warningTitle}
                  onChange={(e) => setWarningTitle(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Сообщение:</label>
                <textarea
                  required
                  value={warningMessage}
                  onChange={(e) => setWarningMessage(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={3}
                />
              </div>
            </div>

            <div className="flex justify-end space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowWarningModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-amber-500 text-white text-xs font-bold rounded-xl"
              >
                Создать
              </button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL: CREATE VIOLATION */}
      {showViolationModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <form onSubmit={handleCreateViolation} className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Зафиксировать нарушение</h3>
            <div className="space-y-3 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Тип:</label>
                <CustomSelect
                  value={violationType}
                  onChange={(val) => setViolationType(val)}
                  options={VIOLATION_TYPES}
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Серьезность:</label>
                <CustomSelect
                  value={violationSeverity}
                  onChange={(val) => setViolationSeverity(val as any)}
                  options={SEVERITY_OPTIONS}
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Заголовок:</label>
                <input
                  type="text"
                  required
                  value={violationTitle}
                  onChange={(e) => setViolationTitle(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Описание:</label>
                <textarea
                  required
                  value={violationDescription}
                  onChange={(e) => setViolationDescription(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={3}
                />
              </div>
            </div>

            <div className="flex justify-end space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowViolationModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-red-600 text-white text-xs font-bold rounded-xl"
              >
                Зафиксировать
              </button>
            </div>
          </form>
        </div>
      )}


      
      {/* MODAL: CREATE NOTE */}
      {showNoteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <form onSubmit={handleCreateNote} className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Добавить внутреннюю заметку</h3>
            <div className="space-y-3 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Тип заметки:</label>
                <CustomSelect
                  value={noteType}
                  onChange={(val) => setNoteType(val)}
                  options={[
                    { value: 'note', label: 'Обычная заметка' },
                    { value: 'recommendation', label: 'Рекомендация' },
                    { value: 'control', label: 'На контроле' },
                    { value: 'critical', label: 'Критично' },
                  ]}
                />
              </div>
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Содержание:</label>
                <textarea
                  required
                  value={noteContent}
                  onChange={(e) => setNoteContent(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={4}
                />
              </div>
            </div>
            <div className="flex justify-end space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowNoteModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black text-xs font-bold rounded-xl"
              >
                Сохранить
              </button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL: CREATE PLAN */}
      {showPlanModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <form onSubmit={handleCreatePlan} className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl max-h-[90vh] overflow-y-auto">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Новый план улучшения</h3>
            <div className="space-y-3 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Причина назначения плана:</label>
                <textarea
                  required
                  value={planReason}
                  onChange={(e) => setPlanReason(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={2}
                />
              </div>
              
              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Действия по улучшению:</label>
                <div className="space-y-2">
                  {planActions.map((action, index) => (
                    <div key={index} className="flex gap-2">
                      <input
                        type="text"
                        value={action}
                        placeholder={`Шаг ${index + 1}`}
                        onChange={(e) => {
                          const newActions = [...planActions];
                          newActions[index] = e.target.value;
                          setPlanActions(newActions);
                        }}
                        className="flex-1 p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                      />
                      {planActions.length > 1 && (
                        <button type="button" onClick={() => setPlanActions(planActions.filter((_, i) => i !== index))} className="p-2 text-red-500 hover:bg-red-50 rounded-xl">✕</button>
                      )}
                    </div>
                  ))}
                  <button type="button" onClick={() => setPlanActions([...planActions, ''])} className="text-blue-600 font-semibold hover:underline flex items-center">
                    <Plus className="w-3 h-3 mr-1" /> Добавить шаг
                  </button>
                </div>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Срок выполнения (опционально):</label>
                <input
                  type="date"
                  value={planDeadline}
                  onChange={(e) => setPlanDeadline(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 dark:text-gray-300 mb-1">Внутренний комментарий:</label>
                <textarea
                  value={planInternalComment}
                  onChange={(e) => setPlanInternalComment(e.target.value)}
                  className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl text-xs text-gray-900 dark:text-white"
                  rows={2}
                />
              </div>
            </div>
            <div className="flex justify-end space-x-2 pt-2">
              <button
                type="button"
                onClick={() => setShowPlanModal(false)}
                className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-blue-600 text-white text-xs font-bold rounded-xl"
              >
                Создать план
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}

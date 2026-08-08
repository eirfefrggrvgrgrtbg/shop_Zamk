import { useState, useEffect, useRef } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import {
  getAdminPaymentErrorMessage,
  getAdminPayments,
} from '../api/adminPayments';
import type { AdminPaymentView, AdminPaymentQueryParams, PaymentSort } from '../api/adminPayments';
import { Search, AlertCircle, CreditCard, ChevronUp, ChevronDown, X, AlertTriangle, RefreshCw } from 'lucide-react';
import { FilterPopover } from '../components/FilterPopover';

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

const INTEGRATION_MODE_LABELS: Record<string, string> = {
  mock: 'Тестовый',
  api: 'API',
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

type SortOrder = 'asc' | 'desc';

interface ColumnDef {
  id: string;
  label: string;
  sortKey?: PaymentSort;
  isNumeric?: boolean;
}

const COLUMNS: ColumnDef[] = [
  { id: 'number', label: 'Номер / Заказ', sortKey: 'paymentNumber' },
  { id: 'created', label: 'Дата и время', sortKey: 'createdAt' },
  { id: 'customer', label: 'Покупатель' },
  { id: 'provider', label: 'Провайдер / способ' },
  { id: 'amount', label: 'Сумма', sortKey: 'amount', isNumeric: true },
  { id: 'refund', label: 'Возврат' },
  { id: 'attempts', label: 'Попытки' },
  { id: 'status', label: 'Статус', sortKey: 'status' },
  { id: 'problems', label: 'Требует внимания' },
];

export function AdminPayments() {
  const [searchParams, setSearchParams] = useSearchParams();
  const abortControllerRef = useRef<AbortController | null>(null);
  const latestRequestIdRef = useRef<number>(0);

  // Parse URL to strictly driven applied filters state
  const q = searchParams.get('q') || '';
  const status = searchParams.get('status') || '';
  const provider = searchParams.get('provider') || '';
  const paymentMethod = searchParams.get('paymentMethod') || '';
  const integrationMode = searchParams.get('integrationMode') || '';
  const refundState = searchParams.get('refundState') || '';
  const hasProblem = searchParams.get('hasProblem') === 'true';
  const problemCode = searchParams.get('problemCode') || '';
  const amountFromCents = searchParams.get('amountFromCents') || '';
  const amountToCents = searchParams.get('amountToCents') || '';
  const dateFrom = searchParams.get('dateFrom') || '';
  const dateTo = searchParams.get('dateTo') || '';
  
  const sortField = (searchParams.get('sort') as PaymentSort) || undefined;
  const sortOrder = (searchParams.get('direction') as SortOrder) || undefined;
  const page = parseInt(searchParams.get('page') || '1', 10);
  const limit = parseInt(searchParams.get('limit') || '25', 10);

  // Safe Date / Amount getters for popovers
  const safeDateString = (iso?: string) => iso ? new Date(iso).toISOString().slice(0, 10) : '';
  const safeAmountString = (cents?: string) => cents ? String(parseInt(cents, 10) / 100) : '';

  // Local state for fetching results
  const [payments, setPayments] = useState<AdminPaymentView[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Local state for debounced search typing
  const [searchQuery, setSearchQuery] = useState(q);



  // Popovers
  const [openPopover, setOpenPopover] = useState<string | null>(null);
  
  // Validation errors
  const [popoverError, setPopoverError] = useState<string | null>(null);

  // Popover Drafts
  const [statusDraft, setStatusDraft] = useState(status);
  const [providerDraft, setProviderDraft] = useState({ provider, paymentMethod, integrationMode });
  const [refundDraft, setRefundDraft] = useState(refundState);
  const [probDraft, setProbDraft] = useState({ hasProblem, problemCode });
  const [periodDraft, setPeriodDraft] = useState({ dateFrom: safeDateString(dateFrom), dateTo: safeDateString(dateTo) });
  const [amountDraft, setAmountDraft] = useState({ from: safeAmountString(amountFromCents), to: safeAmountString(amountToCents) });

  // Sync Drafts whenever URL changes (e.g. Back/Forward button)
  useEffect(() => {
    setStatusDraft(status);
    setProviderDraft({ provider, paymentMethod, integrationMode });
    setRefundDraft(refundState);
    setProbDraft({ hasProblem, problemCode });
    setPeriodDraft({ dateFrom: safeDateString(dateFrom), dateTo: safeDateString(dateTo) });
    setAmountDraft({ from: safeAmountString(amountFromCents), to: safeAmountString(amountToCents) });
    setSearchQuery(q);
  }, [q, status, provider, paymentMethod, integrationMode, refundState, hasProblem, problemCode, amountFromCents, amountToCents, dateFrom, dateTo]);

  // Unified Request Pipeline (Fires whenever searchParams changes)
  useEffect(() => {
    const requestId = ++latestRequestIdRef.current;
    
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    const controller = new AbortController();
    abortControllerRef.current = controller;

    setIsLoading(true);
    setError(null);

    const offset = (page - 1) * limit;

    const queryParams: AdminPaymentQueryParams = {
      q, status, provider, paymentMethod, integrationMode, refundState,
      hasProblem: hasProblem ? 'true' : undefined, problemCode,
      amountFromCents, amountToCents, dateFrom, dateTo,
      sort: sortField, direction: sortOrder, limit, offset,
      signal: controller.signal
    };

    getAdminPayments(queryParams)
      .then(res => {
        if (controller.signal.aborted) return;
        if (latestRequestIdRef.current !== requestId) return;
        
        setPayments(res.items);
        setTotal(res.totalCount);
        setIsLoading(false);
      })
      .catch(err => {
        if (controller.signal.aborted) return;
        if (latestRequestIdRef.current !== requestId) return;
        
        setError(getAdminPaymentErrorMessage(err, 'Ошибка загрузки платежей'));
        setIsLoading(false);
      });

    return () => {
      controller.abort();
    };
  }, [searchParams]); // Strictly dependent on URL changes

  // Debounced Search Sync to URL
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchQuery !== q) {
        updateURL({ q: searchQuery, page: '1' });
      }
    }, 400);
    return () => clearTimeout(timer);
  }, [searchQuery, q]);

  const updateURL = (updates: Record<string, string | undefined>) => {
    const nextParams = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([k, v]) => {
      if (v === undefined || v === '') {
        nextParams.delete(k);
      } else {
        nextParams.set(k, v);
      }
    });
    setSearchParams(nextParams);
  };

  const applyStatusFilter = () => {
    updateURL({ status: statusDraft, page: '1' });
    setOpenPopover(null);
  };

  const applyProviderFilter = () => {
    updateURL({ 
      provider: providerDraft.provider, 
      paymentMethod: providerDraft.paymentMethod, 
      integrationMode: providerDraft.integrationMode, 
      page: '1' 
    });
    setOpenPopover(null);
  };

  const applyRefundFilter = () => {
    updateURL({ refundState: refundDraft, page: '1' });
    setOpenPopover(null);
  };

  const applyProbFilter = () => {
    updateURL({ 
      hasProblem: probDraft.hasProblem ? 'true' : '', 
      problemCode: probDraft.problemCode, 
      page: '1' 
    });
    setOpenPopover(null);
  };

  const applyPeriodFilter = () => {
    let finalFrom = '';
    let finalTo = '';
    
    setPopoverError(null);

    if (periodDraft.dateFrom) {
      const df = new Date(periodDraft.dateFrom);
      if (isNaN(df.getTime())) {
        setPopoverError('Неверный формат даты С');
        return;
      }
      finalFrom = df.toISOString();
    }
    
    if (periodDraft.dateTo) {
      const dt = new Date(periodDraft.dateTo);
      if (isNaN(dt.getTime())) {
        setPopoverError('Неверный формат даты По');
        return;
      }
      dt.setUTCHours(23, 59, 59, 999);
      finalTo = dt.toISOString();
    }

    if (finalFrom && finalTo && new Date(finalFrom) > new Date(finalTo)) {
      setPopoverError('Дата "С" не может быть позже даты "По"');
      return;
    }

    updateURL({ dateFrom: finalFrom, dateTo: finalTo, page: '1' });
    setOpenPopover(null);
  };

  const applyAmountFilter = () => {
    let finalFromCents = '';
    let finalToCents = '';
    
    setPopoverError(null);

    if (amountDraft.from) {
      const f = parseFloat(amountDraft.from.replace(',', '.'));
      if (!isFinite(f) || f < 0) {
        setPopoverError('Неверная минимальная сумма');
        return;
      }
      finalFromCents = Math.floor(f * 100).toString();
    }
    
    if (amountDraft.to) {
      const t = parseFloat(amountDraft.to.replace(',', '.'));
      if (!isFinite(t) || t < 0) {
        setPopoverError('Неверная максимальная сумма');
        return;
      }
      finalToCents = Math.floor(t * 100).toString();
    }

    if (finalFromCents && finalToCents && parseInt(finalFromCents, 10) > parseInt(finalToCents, 10)) {
      setPopoverError('Минимальная сумма не может быть больше максимальной');
      return;
    }

    updateURL({ amountFromCents: finalFromCents, amountToCents: finalToCents, page: '1' });
    setOpenPopover(null);
  };

  const removeFilters = (keys: string[]) => {
    const updates: Record<string, string | undefined> = {};
    keys.forEach(k => updates[k] = undefined);
    updates.page = '1';
    updateURL(updates);
  };

  const handleResetAll = () => {
    setSearchParams(new URLSearchParams(), { replace: true });
    setOpenPopover(null);
  };

  const handleColumnSortClick = (colDef: ColumnDef) => {
    if (!colDef.sortKey) return;
    const key = colDef.sortKey;

    let nextField: string | undefined = sortField;
    let nextOrder: string | undefined = sortOrder;

    const primaryDirection: SortOrder = colDef.isNumeric ? 'desc' : 'asc';
    const reverseDirection: SortOrder = primaryDirection === 'desc' ? 'asc' : 'desc';

    if (sortField !== key) {
      nextField = key;
      nextOrder = primaryDirection;
    } else if (sortOrder === primaryDirection) {
      nextOrder = reverseDirection;
    } else {
      nextField = undefined;
      nextOrder = undefined;
    }

    updateURL({ sort: nextField, direction: nextOrder, page: '1' });
  };

  const handleLimitChange = (newLimit: number) => {
    updateURL({ limit: String(newLimit), page: '1' });
  };

  const handleRetry = () => {
    // Force a re-render/re-fetch by replacing with the same search params
    setSearchParams(new URLSearchParams(searchParams.toString()), { replace: true });
  };

  const renderSortIndicator = (colDef: ColumnDef) => {
    if (!colDef.sortKey) return null;
    if (sortField !== colDef.sortKey) {
      return <span className="ml-1 text-gray-400 opacity-40 hover:opacity-100 font-mono text-xs">↕</span>;
    }
    return <span className="ml-1 text-black font-bold text-xs">{sortOrder === 'asc' ? '↑' : '↓'}</span>;
  };

  const getStatusBadgeClass = (s: string) => {
    switch (s) {
      case 'succeeded': return 'bg-green-100 text-green-800 border-green-200';
      case 'failed':
      case 'cancelled': return 'bg-red-100 text-red-800 border-red-200';
      case 'created': return 'bg-gray-100 text-gray-800 border-gray-200';
      default: return 'bg-yellow-100 text-yellow-800 border-yellow-200';
    }
  };

  const getRefundBadgeClass = (s: string) => {
    switch (s) {
      case 'none': return 'bg-gray-100 text-gray-600 border-gray-200';
      case 'partial': return 'bg-orange-100 text-orange-800 border-orange-200';
      case 'full': return 'bg-purple-100 text-purple-800 border-purple-200';
      case 'partial_pending':
      case 'full_pending': return 'bg-blue-100 text-blue-800 border-blue-200';
      default: return 'bg-gray-100 text-gray-800 border-gray-200';
    }
  };

  const safeText = (val: string | null | undefined) => {
    if (val === null || val === undefined || val === '') return '—';
    return val;
  };

  const formatDateTime = (value?: string | null) => {
    if (!value) return '—';
    const d = new Date(value);
    if (isNaN(d.getTime())) return '—';
    return d.toLocaleString('ru-RU');
  };

  const formatMoneyCents = (cents: number | null | undefined, curr: string | null | undefined) => {
    if (cents === null || cents === undefined || isNaN(cents)) return '—';
    return `${(cents / 100).toFixed(2)} ${curr || 'RUB'}`;
  };

  const hasActiveFilters = q || status || provider || paymentMethod || integrationMode || refundState || hasProblem || problemCode || amountFromCents || amountToCents || dateFrom || dateTo || sortField || page > 1 || limit !== 25;
  const isCompletelyEmpty = payments.length === 0 && !hasActiveFilters && !isLoading && !error;
  const isNoResults = payments.length === 0 && hasActiveFilters && !isLoading && !error;

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-6">
      <div>
        <div className="flex items-center space-x-3">
          <h1 className="text-2xl font-bold text-gray-900">Платежи</h1>
          {!isLoading && (
            <span className="bg-gray-100 text-gray-700 px-3 py-1 rounded-full text-xs font-semibold">
              {total}
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-gray-500">Входящие оплаты от покупателей за заказы</p>
      </div>

      <div className="relative">
        <Search className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
        <input
          type="text"
          placeholder="Поиск по ID платежа, номеру заказа, имени, email, телефону..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full pl-11 pr-4 py-3 bg-white border border-gray-200 rounded-2xl focus:outline-none focus:ring-2 focus:ring-black shadow-sm text-sm text-gray-900"
        />
      </div>

      <div className="flex flex-wrap items-center gap-2 relative z-20">
        {/* Status */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'status' ? null : 'status'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              status ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{status ? `Статус: ${PAYMENT_STATUS_LABELS[status] || status}` : 'Статус'}</span>
            {status ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['status']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'status' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'status'} onClose={() => setOpenPopover(null)} onReset={() => setStatusDraft('')} onApply={applyStatusFilter} widthClass="w-56">
            <div className="space-y-2">
              {Object.entries(PAYMENT_STATUS_LABELS).map(([k, v]) => (
                <label key={k} className="flex items-center space-x-2.5 cursor-pointer">
                  <input type="radio" checked={statusDraft === k} onChange={() => setStatusDraft(k)} className="form-radio text-black focus:ring-black h-4 w-4" />
                  <span className="text-sm font-medium text-gray-700">{v}</span>
                </label>
              ))}
            </div>
          </FilterPopover>
        </div>

        {/* Provider */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'provider' ? null : 'provider'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              provider || paymentMethod || integrationMode ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{provider || paymentMethod || integrationMode ? `Оплата: задана` : 'Оплата'}</span>
            {provider || paymentMethod || integrationMode ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['provider', 'paymentMethod', 'integrationMode']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'provider' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'provider'} onClose={() => setOpenPopover(null)} onReset={() => setProviderDraft({provider: '', paymentMethod: '', integrationMode: ''})} onApply={applyProviderFilter} widthClass="w-64">
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-bold text-gray-500 mb-1 uppercase">Провайдер</label>
                <div className="space-y-1">
                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input type="radio" checked={providerDraft.provider === ''} onChange={() => setProviderDraft({...providerDraft, provider: ''})} className="form-radio text-black focus:ring-black h-4 w-4" />
                    <span className="text-sm font-medium text-gray-700">Любой</span>
                  </label>
                  {Object.entries(PROVIDER_LABELS).map(([k, v]) => (
                    <label key={k} className="flex items-center space-x-2.5 cursor-pointer">
                      <input type="radio" checked={providerDraft.provider === k} onChange={() => setProviderDraft({...providerDraft, provider: k})} className="form-radio text-black focus:ring-black h-4 w-4" />
                      <span className="text-sm font-medium text-gray-700">{v}</span>
                    </label>
                  ))}
                </div>
              </div>
              <div>
                <label className="block text-xs font-bold text-gray-500 mb-1 uppercase">Способ</label>
                <div className="space-y-1">
                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input type="radio" checked={providerDraft.paymentMethod === ''} onChange={() => setProviderDraft({...providerDraft, paymentMethod: ''})} className="form-radio text-black focus:ring-black h-4 w-4" />
                    <span className="text-sm font-medium text-gray-700">Любой</span>
                  </label>
                  {Object.entries(PAYMENT_METHOD_LABELS).map(([k, v]) => (
                    <label key={k} className="flex items-center space-x-2.5 cursor-pointer">
                      <input type="radio" checked={providerDraft.paymentMethod === k} onChange={() => setProviderDraft({...providerDraft, paymentMethod: k})} className="form-radio text-black focus:ring-black h-4 w-4" />
                      <span className="text-sm font-medium text-gray-700">{v}</span>
                    </label>
                  ))}
                </div>
              </div>
              <div>
                <label className="block text-xs font-bold text-gray-500 mb-1 uppercase">Режим</label>
                <div className="space-y-1">
                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input type="radio" checked={providerDraft.integrationMode === ''} onChange={() => setProviderDraft({...providerDraft, integrationMode: ''})} className="form-radio text-black focus:ring-black h-4 w-4" />
                    <span className="text-sm font-medium text-gray-700">Любой</span>
                  </label>
                  {Object.entries(INTEGRATION_MODE_LABELS).map(([k, v]) => (
                    <label key={k} className="flex items-center space-x-2.5 cursor-pointer">
                      <input type="radio" checked={providerDraft.integrationMode === k} onChange={() => setProviderDraft({...providerDraft, integrationMode: k})} className="form-radio text-black focus:ring-black h-4 w-4" />
                      <span className="text-sm font-medium text-gray-700">{v}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          </FilterPopover>
        </div>

        {/* Refund */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'refund' ? null : 'refund'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              refundState ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{refundState ? `Возврат: ${REFUND_STATE_LABELS[refundState] || refundState}` : 'Возврат'}</span>
            {refundState ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['refundState']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'refund' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'refund'} onClose={() => setOpenPopover(null)} onReset={() => setRefundDraft('')} onApply={applyRefundFilter} widthClass="w-56">
            <div className="space-y-2">
              {Object.entries(REFUND_STATE_LABELS).map(([k, v]) => (
                <label key={k} className="flex items-center space-x-2.5 cursor-pointer">
                  <input type="radio" checked={refundDraft === k} onChange={() => setRefundDraft(k)} className="form-radio text-black focus:ring-black h-4 w-4" />
                  <span className="text-sm font-medium text-gray-700">{v}</span>
                </label>
              ))}
            </div>
          </FilterPopover>
        </div>

        {/* Problems */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'problems' ? null : 'problems'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              hasProblem || problemCode ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{hasProblem || problemCode ? `Проблемы: да` : 'Проблемы'}</span>
            {hasProblem || problemCode ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['hasProblem', 'problemCode']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'problems' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'problems'} onClose={() => setOpenPopover(null)} onReset={() => setProbDraft({hasProblem: false, problemCode: ''})} onApply={applyProbFilter} widthClass="w-72">
            <div className="space-y-4 max-h-96 overflow-y-auto pr-1">
              <label className="flex items-center space-x-2.5 cursor-pointer border-b pb-3 border-gray-100">
                <input type="checkbox" checked={probDraft.hasProblem} onChange={(e) => setProbDraft({...probDraft, hasProblem: e.target.checked})} className="form-checkbox text-black rounded h-4 w-4" />
                <span className="text-sm font-medium text-gray-700">Есть любые проблемы</span>
              </label>
              <div>
                <label className="block text-xs font-bold text-gray-500 mb-2 uppercase">Код проблемы</label>
                <div className="space-y-2">
                  <label className="flex items-start space-x-2.5 cursor-pointer">
                    <input type="radio" checked={probDraft.problemCode === ''} onChange={() => setProbDraft({...probDraft, problemCode: ''})} className="form-radio text-black focus:ring-black h-4 w-4 mt-0.5" />
                    <span className="text-sm font-medium text-gray-700">Любой код</span>
                  </label>
                  {Object.entries(PROBLEM_CODE_LABELS).map(([k, v]) => (
                    <label key={k} className="flex items-start space-x-2.5 cursor-pointer">
                      <input type="radio" checked={probDraft.problemCode === k} onChange={() => setProbDraft({...probDraft, problemCode: k})} className="form-radio text-black focus:ring-black h-4 w-4 mt-0.5" />
                      <span className="text-sm font-medium text-gray-700 leading-tight">{v}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          </FilterPopover>
        </div>

        {/* Period */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'period' ? null : 'period'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              dateFrom || dateTo ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{dateFrom || dateTo ? `Период: выбран` : 'Период'}</span>
            {dateFrom || dateTo ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['dateFrom', 'dateTo']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'period' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'period'} onClose={() => setOpenPopover(null)} onReset={() => setPeriodDraft({dateFrom: '', dateTo: ''})} onApply={applyPeriodFilter} widthClass="w-64">
            <div className="space-y-3">
              {popoverError && openPopover === 'period' && (
                <div className="text-xs text-red-600 font-medium bg-red-50 p-2 rounded">{popoverError}</div>
              )}
              <div>
                <label className="block text-xs text-gray-500 mb-1">С (дата создания)</label>
                <input type="date" value={periodDraft.dateFrom} onChange={(e) => setPeriodDraft({...periodDraft, dateFrom: e.target.value})} className="w-full text-sm border-gray-300 rounded-md shadow-sm focus:ring-black focus:border-black" />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">По</label>
                <input type="date" value={periodDraft.dateTo} onChange={(e) => setPeriodDraft({...periodDraft, dateTo: e.target.value})} className="w-full text-sm border-gray-300 rounded-md shadow-sm focus:ring-black focus:border-black" />
              </div>
            </div>
          </FilterPopover>
        </div>

        {/* Amount */}
        <div className="relative">
          <button
            onClick={() => { setOpenPopover(openPopover === 'amount' ? null : 'amount'); setPopoverError(null); }}
            className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white border rounded-xl text-xs font-medium transition-colors ${
              amountFromCents || amountToCents ? 'border-gray-400 font-semibold text-gray-900 shadow-sm' : 'border-gray-200 text-gray-700 hover:border-gray-300'
            }`}
          >
            <span>{amountFromCents || amountToCents ? `Сумма: задана` : 'Сумма'}</span>
            {amountFromCents || amountToCents ? (
              <span onClick={(e) => { e.stopPropagation(); removeFilters(['amountFromCents', 'amountToCents']); }} className="p-0.5 hover:bg-gray-200 rounded-full ml-1"><X className="w-3 h-3 text-gray-400 hover:text-black" /></span>
            ) : openPopover === 'amount' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </button>
          <FilterPopover isOpen={openPopover === 'amount'} onClose={() => setOpenPopover(null)} onReset={() => setAmountDraft({from: '', to: ''})} onApply={applyAmountFilter} widthClass="w-64">
            <div className="space-y-3">
              {popoverError && openPopover === 'amount' && (
                <div className="text-xs text-red-600 font-medium bg-red-50 p-2 rounded">{popoverError}</div>
              )}
              <div>
                <label className="block text-xs text-gray-500 mb-1">От (руб)</label>
                <input type="number" min="0" step="any" value={amountDraft.from} onChange={(e) => setAmountDraft({...amountDraft, from: e.target.value})} className="w-full text-sm border-gray-300 rounded-md shadow-sm focus:ring-black focus:border-black" />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">До (руб)</label>
                <input type="number" min="0" step="any" value={amountDraft.to} onChange={(e) => setAmountDraft({...amountDraft, to: e.target.value})} className="w-full text-sm border-gray-300 rounded-md shadow-sm focus:ring-black focus:border-black" />
              </div>
            </div>
          </FilterPopover>
        </div>

        {hasActiveFilters && (
          <button
            onClick={handleResetAll}
            className="text-xs font-medium text-gray-500 hover:text-black underline decoration-gray-300 hover:decoration-black px-2 transition-colors ml-auto md:ml-0"
          >
            Сбросить фильтры
          </button>
        )}
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 rounded-2xl flex items-start">
          <AlertCircle className="h-5 w-5 mr-3 mt-0.5 flex-shrink-0" />
          <div className="flex-1">
            <h3 className="text-sm font-bold text-red-800 mb-1">Ошибка загрузки</h3>
            <div className="text-sm text-red-700">{error}</div>
            <button onClick={handleRetry} className="mt-3 flex items-center space-x-1.5 px-3 py-1.5 bg-white border border-red-200 text-red-700 rounded-lg text-xs font-semibold hover:bg-red-50 transition-colors">
              <RefreshCw className="w-3.5 h-3.5" />
              <span>Повторить</span>
            </button>
          </div>
        </div>
      )}

      {/* Table */}
      <div className="bg-white rounded-2xl shadow-sm border border-gray-100 overflow-hidden relative z-10">
        {isLoading && (
          <div className="absolute inset-0 bg-white/60 backdrop-blur-[1px] z-10 flex items-center justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div>
          </div>
        )}
        
        {isCompletelyEmpty ? (
          <div className="text-center py-20">
            <CreditCard className="mx-auto h-12 w-12 text-gray-300" />
            <h3 className="mt-3 text-sm font-medium text-gray-900">Платежей пока нет</h3>
            <p className="mt-1 text-sm text-gray-500">Здесь будут отображаться входящие оплаты.</p>
          </div>
        ) : isNoResults ? (
          <div className="text-center py-20">
            <Search className="mx-auto h-12 w-12 text-gray-300" />
            <h3 className="mt-3 text-sm font-medium text-gray-900">По вашему запросу ничего не найдено</h3>
            <p className="mt-1 text-sm text-gray-500">Попробуйте изменить параметры поиска или фильтров.</p>
            <button onClick={handleResetAll} className="mt-4 px-4 py-2 bg-black text-white text-sm font-medium rounded-xl hover:bg-gray-800">
              Сбросить все фильтры
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto min-h-[400px]">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  {COLUMNS.map((col) => (
                    <th
                      key={col.id}
                      onClick={() => handleColumnSortClick(col)}
                      className={`px-4 py-3 text-left text-[11px] font-semibold text-gray-500 uppercase tracking-wider ${
                        col.sortKey ? 'cursor-pointer hover:bg-gray-100 select-none group transition-colors' : ''
                      } ${col.isNumeric ? 'text-right' : ''}`}
                    >
                      <div className={`flex items-center ${col.isNumeric ? 'justify-end' : ''}`}>
                        {col.label}
                        {renderSortIndicator(col)}
                      </div>
                    </th>
                  ))}
                  <th className="px-4 py-3"><span className="sr-only">Действия</span></th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-100">
                {payments.map((payment) => {
                  const problems = payment.problems || [];
                  return (
                    <tr key={payment.id} className="hover:bg-gray-50/50 transition-colors">
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900">{payment.paymentNumber}</div>
                        <div className="text-xs text-gray-500 mt-0.5">Заказ: {safeText(payment.orderId)}</div>
                        {payment.integrationMode === 'mock' && (
                          <div className="mt-1"><span className="inline-flex items-center px-1.5 py-0.5 rounded border border-amber-200 text-[10px] font-bold bg-amber-100 text-amber-800">ТЕСТОВЫЙ</span></div>
                        )}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="text-sm text-gray-900">{formatDateTime(payment.createdAt)}</div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="text-sm text-gray-900 font-medium">{safeText(payment.customer?.name)}</div>
                        <div className="text-xs text-gray-500">{safeText(payment.customer?.email)}</div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="text-sm text-gray-900">{PROVIDER_LABELS[payment.provider] || safeText(payment.provider)}</div>
                        <div className="text-xs text-gray-500 uppercase">{PAYMENT_METHOD_LABELS[payment.paymentMethod || ''] || safeText(payment.paymentMethod)}</div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-right">
                        <div className="text-sm font-bold text-gray-900">{formatMoneyCents(payment.netAmountCents, payment.currency)}</div>
                        <div className="text-[10px] text-gray-500">Брутто: {formatMoneyCents(payment.amountCents, payment.currency)}</div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2 py-0.5 border rounded-full text-xs font-medium ${getRefundBadgeClass(payment.refundState)}`}>
                          {REFUND_STATE_LABELS[payment.refundState] ?? safeText(payment.refundState)}
                        </span>
                        {payment.refundedAmountCents > 0 && (
                          <div className="text-xs text-gray-500 mt-1">
                            -{formatMoneyCents(payment.refundedAmountCents, payment.currency)}
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-center">
                        <div className="text-sm text-gray-900">{(payment.attemptsCount || 0) > 0 ? payment.attemptsCount : '—'}</div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2 py-0.5 border rounded-full text-xs font-medium ${getStatusBadgeClass(payment.status)}`}>
                          {PAYMENT_STATUS_LABELS[payment.status] ?? safeText(payment.status)}
                        </span>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {problems.length > 0 ? (
                          <div className="flex flex-col items-start gap-1">
                            {problems.map((prob, idx) => (
                              <div key={idx} className={`inline-flex flex-col px-2 py-1 rounded-md border ${prob.severity === 'critical' ? 'bg-red-100 border-red-200' : 'bg-amber-50 border-amber-200'}`}>
                                <div className={`flex items-center text-[10px] font-bold ${prob.severity === 'critical' ? 'text-red-800' : 'text-amber-800'}`}>
                                  <AlertTriangle className="w-3 h-3 mr-1" />
                                  {PROBLEM_CODE_LABELS[prob.code] || prob.code}
                                </div>
                                {PROBLEM_CODE_LABELS[prob.code] && (
                                  <div className={`text-[9px] mt-0.5 font-mono ${prob.severity === 'critical' ? 'text-red-600/80' : 'text-amber-700/80'}`}>
                                    {prob.code}
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        ) : (
                          <span className="text-gray-300 text-sm">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-right text-sm font-medium">
                        <Link to={`/payments/${payment.id}`} state={{ from: searchParams.toString() }} className="text-indigo-600 hover:text-indigo-900 font-semibold px-2 py-1 rounded-md hover:bg-indigo-50 transition-colors">Детали</Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {(totalPages > 1 || payments.length > 0) && (
          <div className="px-6 py-4 border-t border-gray-100 bg-gray-50 flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <span className="text-sm text-gray-600 font-medium">
                Показаны {(page - 1) * limit + (total > 0 ? 1 : 0)} – {Math.min(page * limit, total)} из {total}
              </span>
              <div className="flex items-center space-x-2">
                <span className="text-xs text-gray-500">Показывать по:</span>
                <select 
                  value={limit} 
                  onChange={(e) => handleLimitChange(Number(e.target.value))}
                  className="text-xs border-gray-300 rounded-md shadow-sm focus:ring-black focus:border-black py-1 pl-2 pr-6"
                >
                  <option value={25}>25</option>
                  <option value={50}>50</option>
                  <option value={100}>100</option>
                </select>
              </div>
            </div>
            
            {totalPages > 1 && (
              <div className="flex items-center space-x-2">
                <button
                  disabled={page <= 1}
                  onClick={() => updateURL({ page: String(page - 1) })}
                  className="px-4 py-2 border border-gray-200 bg-white rounded-xl text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 disabled:opacity-50 disabled:pointer-events-none transition-colors"
                >
                  Назад
                </button>
                <div className="px-4 text-sm font-medium text-gray-900">
                  {page} / {totalPages}
                </div>
                <button
                  disabled={page >= totalPages}
                  onClick={() => updateURL({ page: String(page + 1) })}
                  className="px-4 py-2 border border-gray-200 bg-white rounded-xl text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 disabled:opacity-50 disabled:pointer-events-none transition-colors"
                >
                  Вперед
                </button>
              </div>
            )}
          </div>
        )}
      </div>


    </div>
  );
}

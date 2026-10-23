import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock,
  Lock,
  Search,
  ShieldAlert,
  Wallet,
  X,
} from 'lucide-react';
import { SellerContextBanner } from '../components/SellerContextBanner';
import { PermissionGuard } from '../components/PermissionGuard';
import {
  getAdminPayout,
  getAdminPayoutErrorMessage,
  getAdminPayouts,
  getAdminPayoutSummary,
  getPayoutStatusLabel,
  holdAdminPayoutBatch,
  processAdminPayoutBatch,
} from '../api/adminPayouts';
import type {
  AdminPayout,
  AdminPayoutDetail,
  AdminPayoutSummary,
} from '../api/adminPayouts';

const PAGE_SIZE = 20;

export function AdminPayouts() {
  const [searchParams, setSearchParams] = useSearchParams();
  const urlSellerId = searchParams.get('sellerId') || '';

  const [payouts, setPayouts] = useState<AdminPayout[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [summary, setSummary] = useState<AdminPayoutSummary | null>(null);
  const [selectedPayoutId, setSelectedPayoutId] = useState<string | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<AdminPayoutDetail | null>(null);

  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  // Modals for safe confirmation
  const [showConfirmProcessModal, setShowConfirmProcessModal] = useState(false);
  const [showConfirmHoldModal, setShowConfirmHoldModal] = useState(false);

  const [filters, setFilters] = useState({
    q: searchParams.get('q') || '',
    sellerId: urlSellerId,
    status: searchParams.get('status') || '',
    page: 1,
  });
  const [debouncedQuery, setDebouncedQuery] = useState(filters.q);

  // Sync debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(filters.q);
    }, 400);
    return () => clearTimeout(timer);
  }, [filters.q]);

  // Keep sellerId from URL in sync
  useEffect(() => {
    if (urlSellerId !== filters.sellerId) {
      setFilters((prev) => ({ ...prev, sellerId: urlSellerId, page: 1 }));
    }
  }, [urlSellerId]);

  const fetchSummary = async () => {
    try {
      const data = await getAdminPayoutSummary();
      setSummary(data);
    } catch {
      // Non-fatal if summary fails
    }
  };

  const fetchPayouts = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const offset = (filters.page - 1) * PAGE_SIZE;
      const response = await getAdminPayouts({
        q: debouncedQuery || undefined,
        sellerId: filters.sellerId || undefined,
        status: filters.status || undefined,
        limit: PAGE_SIZE,
        offset,
      });
      setPayouts(response.items);
      setTotalCount(response.totalCount);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось загрузить выплаты.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchPayoutDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      setSelectedPayoutId(id);
      const detail = await getAdminPayout(id);
      setSelectedDetail(detail);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось загрузить детали выплаты.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchSummary();
  }, []);

  useEffect(() => {
    fetchPayouts();
  }, [debouncedQuery, filters.sellerId, filters.status, filters.page]);

  const handleProcessConfirm = async () => {
    if (!selectedDetail) return;
    try {
      setIsSubmitting(true);
      setError(null);
      setActionSuccess(null);
      await processAdminPayoutBatch(selectedDetail.payout.id);
      setShowConfirmProcessModal(false);
      setActionSuccess('Выплата успешно подтверждена. Проводка списания зафиксирована в ledger.');
      await fetchSummary();
      await fetchPayouts();
      await fetchPayoutDetail(selectedDetail.payout.id);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось подтвердить выплату.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleHoldConfirm = async () => {
    if (!selectedDetail) return;
    try {
      setIsSubmitting(true);
      setError(null);
      setActionSuccess(null);
      await holdAdminPayoutBatch(selectedDetail.payout.id);
      setShowConfirmHoldModal(false);
      setActionSuccess('Выплата успешно заблокирована (переведена в статус held).');
      await fetchSummary();
      await fetchPayouts();
      await fetchPayoutDetail(selectedDetail.payout.id);
    } catch (err: unknown) {
      setError(getAdminPayoutErrorMessage(err, 'Не удалось заблокировать выплату.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatCents = (cents?: number | null): string => {
    const val = (cents ?? 0) / 100;
    return val.toLocaleString('ru-RU', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  };

  const formatDate = (value?: string | null): string => {
    if (!value) return '—';
    try {
      return new Date(value).toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
      });
    } catch {
      return value;
    }
  };

  const formatDateTime = (value?: string | null): string => {
    if (!value) return '—';
    try {
      return new Date(value).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return value;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'paid':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300">
            <CheckCircle2 className="w-3 h-3 mr-1" />
            Выплачена
          </span>
        );
      case 'processing':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800 dark:bg-indigo-900/40 dark:text-indigo-300">
            <Clock className="w-3 h-3 mr-1" />
            В обработке
          </span>
        );
      case 'scheduled':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300">
            <Clock className="w-3 h-3 mr-1" />
            Запланирована
          </span>
        );
      case 'held':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
            <Lock className="w-3 h-3 mr-1" />
            Заблокирована
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-300">
            <AlertCircle className="w-3 h-3 mr-1" />
            Ошибка
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300">
            {getPayoutStatusLabel(status)}
          </span>
        );
    }
  };

  const totalPages = Math.max(1, Math.ceil(totalCount / PAGE_SIZE));

  return (
    <div className="space-y-6">
      <SellerContextBanner />

      {/* Header */}
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Выплаты продавцам</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Реальные партии выплат (Payout Batches) и финансовые расчеты с продавцами платформы.
          </p>
        </div>
        {filters.sellerId && (
          <button
            onClick={() => {
              setFilters((f) => ({ ...f, sellerId: '', page: 1 }));
              searchParams.delete('sellerId');
              setSearchParams(searchParams);
            }}
            className="inline-flex items-center text-xs text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 font-medium"
          >
            Сбросить фильтр продавца <X className="w-3.5 h-3.5 ml-1" />
          </button>
        )}
      </div>

      {/* Financial Summary KPI Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div className="overflow-hidden rounded-xl bg-white dark:bg-gray-800 px-4 py-5 shadow-sm border border-gray-100 dark:border-gray-700">
          <dt className="truncate text-xs font-medium text-gray-500 dark:text-gray-400">
            Доступно к выплате
          </dt>
          <dd className="mt-1 text-2xl font-bold tracking-tight text-gray-900 dark:text-white">
            {summary ? formatCents(summary.totalAvailableCents) : '0,00'} ₽
          </dd>
          <p className="mt-1 text-xs text-gray-400">Свободный баланс продавцов</p>
        </div>

        <div className="overflow-hidden rounded-xl bg-white dark:bg-gray-800 px-4 py-5 shadow-sm border border-gray-100 dark:border-gray-700">
          <dt className="truncate text-xs font-medium text-gray-500 dark:text-gray-400">
            Ожидает выплаты
          </dt>
          <dd className="mt-1 text-2xl font-bold tracking-tight text-blue-600 dark:text-blue-400">
            {summary ? formatCents(summary.totalPendingCents) : '0,00'} ₽
          </dd>
          <p className="mt-1 text-xs text-gray-400">В активных партиях (batches)</p>
        </div>

        <div className="overflow-hidden rounded-xl bg-amber-50/70 dark:bg-amber-950/20 px-4 py-5 shadow-sm border border-amber-200/60 dark:border-amber-800/40">
          <dt className="truncate text-xs font-medium text-amber-800 dark:text-amber-300">
            Удержано по возвратам
          </dt>
          <dd className="mt-1 text-2xl font-bold tracking-tight text-amber-900 dark:text-amber-200">
            {summary ? formatCents(summary.totalReturnHoldCents) : '0,00'} ₽
          </dd>
          <p className="mt-1 text-xs text-amber-700/80 dark:text-amber-400/80">
            Заблокировано открытыми спорами
          </p>
        </div>

        <div className="overflow-hidden rounded-xl bg-white dark:bg-gray-800 px-4 py-5 shadow-sm border border-gray-100 dark:border-gray-700">
          <dt className="truncate text-xs font-medium text-gray-500 dark:text-gray-400">
            Уже выплачено
          </dt>
          <dd className="mt-1 text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
            {summary ? formatCents(summary.totalPaidCents) : '0,00'} ₽
          </dd>
          <p className="mt-1 text-xs text-gray-400">Фактически подтверждено</p>
        </div>

        <div className="overflow-hidden rounded-xl bg-indigo-50/50 dark:bg-indigo-950/20 px-4 py-5 shadow-sm border border-indigo-100 dark:border-indigo-900/40">
          <dt className="truncate text-xs font-medium text-indigo-700 dark:text-indigo-300">
            Комиссия ZAMK
          </dt>
          <dd className="mt-1 text-2xl font-bold tracking-tight text-indigo-900 dark:text-indigo-200">
            {summary ? formatCents(summary.totalCommissionCents) : '0,00'} ₽
          </dd>
          <p className="mt-1 text-xs text-indigo-600/80 dark:text-indigo-400/80">Доход платформы</p>
        </div>
      </div>

      {/* Manual Processing Notice */}
      <div className="rounded-lg bg-amber-50 dark:bg-amber-950/30 border-l-4 border-amber-400 p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <AlertTriangle className="h-5 w-5 text-amber-500" />
          </div>
          <div className="ml-3">
            <p className="text-xs text-amber-800 dark:text-amber-300 leading-relaxed">
              <strong>Внимание администратора:</strong> Действие «Подтвердить факт выплаты» фиксирует,
              что банковский перевод средств уже фактически выполнен вне платформы ZAMK. Система не
              выполняет автоматических списаний с банковских счетов платформы.
            </p>
          </div>
        </div>
      </div>

      {/* Alerts */}
      {error && (
        <div className="p-4 bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded-lg flex items-center text-sm">
          <AlertCircle className="h-5 w-5 mr-2 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {actionSuccess && (
        <div className="p-4 bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800 text-emerald-700 dark:text-emerald-300 rounded-lg flex items-center text-sm">
          <CheckCircle2 className="h-5 w-5 mr-2 flex-shrink-0" />
          <span>{actionSuccess}</span>
        </div>
      )}

      {/* Filters Bar */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
            <Search className="h-4 w-4 text-gray-400" />
          </div>
          <input
            type="text"
            placeholder="Поиск по ID выплаты или названию продавца..."
            className="w-full pl-9 pr-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            value={filters.q}
            onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value, page: 1 }))}
          />
        </div>

        <select
          className="py-2 px-3 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500 sm:w-56"
          value={filters.status}
          onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value, page: 1 }))}
        >
          <option value="">Все статусы</option>
          <option value="scheduled">Запланирована (scheduled)</option>
          <option value="processing">В обработке (processing)</option>
          <option value="paid">Выплачена (paid)</option>
          <option value="held">Заблокирована (held)</option>
          <option value="failed">Ошибка (failed)</option>
        </select>
      </div>

      {/* Batches Table or Empty State */}
      {isLoading ? (
        <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-3 text-sm text-gray-500 dark:text-gray-400">Загрузка партий выплат...</p>
        </div>
      ) : payouts.length === 0 ? (
        <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700">
          <Wallet className="mx-auto h-12 w-12 text-gray-400 dark:text-gray-500" />
          <h3 className="mt-2 text-base font-semibold text-gray-900 dark:text-white">
            Партий выплат не найдено
          </h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {filters.q || filters.status || filters.sellerId
              ? 'По заданным критериям фильтрации выплат не найдено.'
              : 'В системе пока не сформировано ни одной партии выплат.'}
          </p>
        </div>
      ) : (
        <div className="bg-white dark:bg-gray-800 shadow-sm border border-gray-100 dark:border-gray-700 rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-left text-xs">
              <thead className="bg-gray-50 dark:bg-gray-900/50 text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold">
                <tr>
                  <th scope="col" className="px-4 py-3">
                    ID партии
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Продавец
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Сумма
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Статус
                  </th>
                  <th scope="col" className="px-4 py-3 text-center">
                    Начислений
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Удержано
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Создана
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Плановая дата
                  </th>
                  <th scope="col" className="px-4 py-3">
                    Выплачена
                  </th>
                  <th scope="col" className="px-4 py-3 text-right">
                    Действия
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-700/60 text-gray-700 dark:text-gray-300">
                {payouts.map((batch) => {
                  const isSelected = selectedPayoutId === batch.id;
                  return (
                    <tr
                      key={batch.id}
                      onClick={() => fetchPayoutDetail(batch.id)}
                      className={`cursor-pointer hover:bg-gray-50/80 dark:hover:bg-gray-700/40 transition-colors ${
                        isSelected ? 'bg-indigo-50/40 dark:bg-indigo-950/20' : ''
                      }`}
                    >
                      <td className="px-4 py-3 font-mono text-gray-900 dark:text-gray-200">
                        <span title={batch.id}>{batch.id.substring(0, 8)}…</span>
                      </td>
                      <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">
                        {batch.sellerName || batch.sellerId.substring(0, 8)}
                      </td>
                      <td className="px-4 py-3 font-semibold text-gray-900 dark:text-white">
                        {formatCents(batch.amountCents)} ₽
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap">{getStatusBadge(batch.status)}</td>
                      <td className="px-4 py-3 text-center text-gray-500">
                        {batch.earningsCount ?? '—'}
                      </td>
                      <td className="px-4 py-3 text-amber-700 dark:text-amber-400 font-medium">
                        {(batch.returnHoldCents ?? 0) > 0
                          ? `${formatCents(batch.returnHoldCents)} ₽`
                          : '—'}
                      </td>
                      <td className="px-4 py-3 text-gray-500 whitespace-nowrap">
                        {formatDate(batch.createdAt)}
                      </td>
                      <td className="px-4 py-3 text-gray-500 whitespace-nowrap">
                        {formatDate(batch.scheduledFor)}
                      </td>
                      <td className="px-4 py-3 text-gray-500 whitespace-nowrap">
                        {formatDate(batch.processedAt)}
                      </td>
                      <td className="px-4 py-3 text-right whitespace-nowrap">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            fetchPayoutDetail(batch.id);
                          }}
                          className="inline-flex items-center text-xs font-semibold text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300"
                        >
                          Детали / Управление
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination Controls */}
          {totalPages > 1 && (
            <div className="px-4 py-3 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between text-xs text-gray-500">
              <div>
                Показано {(filters.page - 1) * PAGE_SIZE + 1}–
                {Math.min(filters.page * PAGE_SIZE, totalCount)} из {totalCount} выплат
              </div>
              <div className="flex space-x-2">
                <button
                  disabled={filters.page <= 1}
                  onClick={() => setFilters((f) => ({ ...f, page: f.page - 1 }))}
                  className="px-2.5 py-1.5 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 disabled:opacity-40"
                >
                  <ChevronLeft className="w-4 h-4 inline" />
                </button>
                <span className="px-2 py-1">
                  Стр. {filters.page} из {totalPages}
                </span>
                <button
                  disabled={filters.page >= totalPages}
                  onClick={() => setFilters((f) => ({ ...f, page: f.page + 1 }))}
                  className="px-2.5 py-1.5 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 disabled:opacity-40"
                >
                  <ChevronRight className="w-4 h-4 inline" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Selected Batch Details View */}
      {selectedDetail && (
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-md border border-gray-200 dark:border-gray-700 p-6 space-y-6">
          {/* Details Header */}
          <div className="flex items-start justify-between border-b border-gray-100 dark:border-gray-700 pb-4">
            <div>
              <div className="flex items-center space-x-3">
                <h2 className="text-lg font-bold text-gray-900 dark:text-white">
                  Партия выплаты {selectedDetail.payout.id.substring(0, 8)}…
                </h2>
                {getStatusBadge(selectedDetail.payout.status)}
              </div>
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 font-mono">
                Полный ID: {selectedDetail.payout.id}
              </p>
            </div>
            <button
              onClick={() => {
                setSelectedDetail(null);
                setSelectedPayoutId(null);
              }}
              className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 p-1"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {isDetailLoading && (
            <div className="text-center py-4 text-xs text-gray-500">Обновление данных детализации...</div>
          )}

          {/* KPI Summary of the Batch */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div className="p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg">
              <span className="text-xs text-gray-500 dark:text-gray-400">Продавец</span>
              <p className="text-sm font-bold text-gray-900 dark:text-white mt-0.5">
                {selectedDetail.payout.sellerName || 'Не указан'}
              </p>
              <p className="text-[11px] font-mono text-gray-400 mt-0.5 truncate">
                {selectedDetail.payout.sellerId}
              </p>
            </div>

            <div className="p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg">
              <span className="text-xs text-gray-500 dark:text-gray-400">Сумма к выплате</span>
              <p className="text-sm font-bold text-gray-900 dark:text-white mt-0.5">
                {formatCents(selectedDetail.payout.amountCents)} ₽
              </p>
              <p className="text-[11px] text-gray-400 mt-0.5">Чистая сумма перевода</p>
            </div>

            <div className="p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg">
              <span className="text-xs text-gray-500 dark:text-gray-400">Создана / План</span>
              <p className="text-xs font-semibold text-gray-900 dark:text-white mt-0.5">
                {formatDate(selectedDetail.payout.createdAt)}
              </p>
              <p className="text-[11px] text-gray-400 mt-0.5">
                План: {formatDate(selectedDetail.payout.scheduledFor)}
              </p>
            </div>

            <div className="p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg">
              <span className="text-xs text-gray-500 dark:text-gray-400">Фактическая выплата</span>
              <p className="text-xs font-semibold text-gray-900 dark:text-white mt-0.5">
                {selectedDetail.payout.processedAt
                  ? formatDateTime(selectedDetail.payout.processedAt)
                  : 'Не подтверждена'}
              </p>
              <p className="text-[11px] text-gray-400 mt-0.5">
                {selectedDetail.payout.status === 'paid' ? 'Проведена в ledger' : 'Ожидает решения'}
              </p>
            </div>
          </div>

          {/* Failure reason if any */}
          {selectedDetail.payout.failureReason && (
            <div className="p-3 bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800 rounded-lg text-xs text-rose-800 dark:text-rose-200 flex items-center">
              <AlertCircle className="w-4 h-4 mr-2 flex-shrink-0" />
              <span>Причина сбоя партии: {selectedDetail.payout.failureReason}</span>
            </div>
          )}

          {/* Financial Totals Breakdown */}
          <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50/50 dark:bg-gray-900/20">
            <h3 className="text-xs font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider mb-3">
              Финансовый состав партии (Ledger Totals)
            </h3>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
              <div>
                <span className="text-gray-500 dark:text-gray-400">Начисления за заказы:</span>
                <p className="font-semibold text-emerald-600 dark:text-emerald-400">
                  +{formatCents(selectedDetail.totals.earningsCents)} ₽
                </p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400">Удержано по возвратам:</span>
                <p className="font-semibold text-amber-600 dark:text-amber-400">
                  -{formatCents(selectedDetail.totals.returnHoldCents)} ₽
                </p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400">Корректировки:</span>
                <p className="font-semibold text-gray-700 dark:text-gray-300">
                  {formatCents(selectedDetail.totals.adjustmentsCents)} ₽
                </p>
              </div>
              <div>
                <span className="text-gray-500 dark:text-gray-400">Списание выплаты (ledger):</span>
                <p className="font-semibold text-indigo-600 dark:text-indigo-400">
                  {selectedDetail.totals.payoutCents !== 0
                    ? `${formatCents(selectedDetail.totals.payoutCents)} ₽`
                    : '—'}
                </p>
              </div>
            </div>
          </div>

          {/* Open Return / Dispute Safety Alert */}
          {selectedDetail.uncoveredDisputeCents > 0 && (
            <div className="p-4 bg-red-50 dark:bg-red-950/40 border border-red-300 dark:border-red-800 rounded-xl text-xs text-red-800 dark:text-red-200 flex items-start space-x-3">
              <ShieldAlert className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
              <div>
                <p className="font-bold">
                  Подтверждение выплаты заблокировано из-за открытого возврата!
                </p>
                <p className="mt-1 leading-relaxed">
                  По товарам из этой партии открыт возврат покупателя на сумму{' '}
                  <strong>{formatCents(selectedDetail.uncoveredDisputeCents)} ₽</strong>, которая не
                  была удержана при создании партии. В соответствии с правилами финансовой
                  безопасности выплата не может быть подтверждена до закрытия или отклонения возврата.
                </p>
              </div>
            </div>
          )}

          {/* Status explanation banners */}
          {selectedDetail.payout.status === 'held' && (
            <div className="p-3 bg-amber-50 dark:bg-amber-950/30 border border-amber-300 dark:border-amber-800 rounded-lg text-xs text-amber-800 dark:text-amber-200 flex items-center">
              <Lock className="w-4 h-4 mr-2 flex-shrink-0" />
              <span>
                Выплата заблокирована (на удержании). Автоматическое возобновление в текущей версии
                не предусмотрено — средства остаются замороженными в партии.
              </span>
            </div>
          )}

          {selectedDetail.payout.status === 'paid' && (
            <div className="p-3 bg-emerald-50 dark:bg-emerald-950/30 border border-emerald-300 dark:border-emerald-800 rounded-lg text-xs text-emerald-800 dark:text-emerald-200 flex items-center">
              <CheckCircle2 className="w-4 h-4 mr-2 flex-shrink-0" />
              <span>
                Выплата подтверждена {formatDateTime(selectedDetail.payout.processedAt)}. Списание с
                баланса продавца зафиксировано в ledger.
              </span>
            </div>
          )}

          {/* Safe Actions Controls */}
          <div className="border-t border-gray-100 dark:border-gray-700 pt-4 flex flex-wrap gap-3 items-center justify-between">
            <div className="text-xs text-gray-500">
              {selectedDetail.canConfirmPaid && (
                <span className="text-emerald-700 dark:text-emerald-400 font-medium">
                  ✓ Партия готова к ручному подтверждению выплаты
                </span>
              )}
              {selectedDetail.confirmBlockedReason === 'already_paid' && (
                <span>Партия уже выплачена</span>
              )}
              {selectedDetail.confirmBlockedReason === 'held' && (
                <span className="text-amber-600">Партия заблокирована</span>
              )}
              {selectedDetail.confirmBlockedReason === 'open_return_not_covered' && (
                <span className="text-red-600">Заблокировано открытым возвратом</span>
              )}
            </div>

            <div className="flex gap-3">
              {selectedDetail.canHold && (
                <PermissionGuard permission={['payouts.update', 'payouts.approve', 'payouts.mark_paid']}>
                  <button
                    type="button"
                    disabled={isSubmitting}
                    onClick={() => setShowConfirmHoldModal(true)}
                    className="inline-flex items-center px-4 py-2 border border-amber-300 dark:border-amber-700 text-xs font-semibold rounded-lg text-amber-800 dark:text-amber-200 bg-amber-50 dark:bg-amber-950/50 hover:bg-amber-100 disabled:opacity-50"
                  >
                    <Lock className="w-3.5 h-3.5 mr-1.5" />
                    Заблокировать (Hold)
                  </button>
                </PermissionGuard>
              )}

              {selectedDetail.canConfirmPaid && (
                <PermissionGuard permission="payouts.mark_paid">
                  <button
                    type="button"
                    disabled={isSubmitting}
                    onClick={() => setShowConfirmProcessModal(true)}
                    className="inline-flex items-center px-4 py-2 text-xs font-semibold rounded-lg text-white bg-emerald-600 hover:bg-emerald-700 shadow-sm disabled:opacity-50"
                  >
                    <CheckCircle2 className="w-3.5 h-3.5 mr-1.5" />
                    Подтвердить факт выплаты (перевод выполнен)
                  </button>
                </PermissionGuard>
              )}
            </div>
          </div>

          {/* Ledger Composition Table */}
          <div className="border-t border-gray-100 dark:border-gray-700 pt-5 space-y-3">
            <h3 className="text-xs font-bold text-gray-900 dark:text-white uppercase tracking-wider">
              Состав начислений и удержаний в партии ({selectedDetail.entries.length})
            </h3>

            {selectedDetail.entries.length === 0 ? (
              <p className="text-xs text-gray-500">В партии нет привязанных записей ledger.</p>
            ) : (
              <div className="overflow-x-auto border border-gray-100 dark:border-gray-700 rounded-lg">
                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-left text-xs">
                  <thead className="bg-gray-50 dark:bg-gray-900/40 text-gray-500 uppercase font-semibold">
                    <tr>
                      <th className="px-3 py-2">Тип</th>
                      <th className="px-3 py-2">Заказ</th>
                      <th className="px-3 py-2">Товар</th>
                      <th className="px-3 py-2">Сумма</th>
                      <th className="px-3 py-2">Дата записи</th>
                      <th className="px-3 py-2">Примечание</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100 dark:divide-gray-700/60 text-gray-600 dark:text-gray-300">
                    {selectedDetail.entries.map((entry) => (
                      <tr key={entry.id} className="hover:bg-gray-50/50 dark:hover:bg-gray-700/30">
                        <td className="px-3 py-2 whitespace-nowrap">
                          {entry.type === 'seller_earning' && (
                            <span className="text-emerald-700 dark:text-emerald-400 font-medium">
                              Начисление
                            </span>
                          )}
                          {entry.type === 'adjustment' && (
                            <span className="text-amber-700 dark:text-amber-400 font-medium">
                              Корректировка / Hold
                            </span>
                          )}
                          {entry.type === 'payout' && (
                            <span className="text-indigo-700 dark:text-indigo-400 font-medium">
                              Выплата
                            </span>
                          )}
                          {entry.type !== 'seller_earning' &&
                            entry.type !== 'adjustment' &&
                            entry.type !== 'payout' &&
                            entry.type}
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap font-mono">
                          {entry.orderNumber || (entry.orderId ? entry.orderId.substring(0, 8) : '—')}
                        </td>
                        <td className="px-3 py-2 text-gray-900 dark:text-white max-w-xs truncate">
                          {entry.productTitle || '—'}
                        </td>
                        <td
                          className={`px-3 py-2 whitespace-nowrap font-semibold ${
                            entry.amountCents > 0
                              ? 'text-emerald-600 dark:text-emerald-400'
                              : 'text-gray-900 dark:text-gray-200'
                          }`}
                        >
                          {entry.amountCents > 0 ? '+' : ''}
                          {formatCents(entry.amountCents)} ₽
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap text-gray-400">
                          {formatDateTime(entry.createdAt)}
                        </td>
                        <td className="px-3 py-2 text-gray-500 font-mono text-[11px]">
                          {entry.reason || '—'}
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

      {/* Confirmation Modal for ProcessPayoutBatch */}
      {showConfirmProcessModal && selectedDetail && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4">
          <div className="bg-white dark:bg-gray-800 rounded-2xl max-w-lg w-full p-6 shadow-xl border border-gray-200 dark:border-gray-700 space-y-4">
            <div className="flex items-center space-x-3 text-amber-600 dark:text-amber-400">
              <AlertTriangle className="w-6 h-6 flex-shrink-0" />
              <h3 className="text-base font-bold text-gray-900 dark:text-white">
                Подтверждение факта выплаты продавцу
              </h3>
            </div>

            <div className="text-xs text-gray-600 dark:text-gray-300 space-y-3 leading-relaxed">
              <p>
                Вы подтверждаете, что платёж на сумму{' '}
                <strong className="text-emerald-600 dark:text-emerald-400 text-sm">
                  {formatCents(selectedDetail.payout.amountCents)} ₽
                </strong>{' '}
                продавцу{' '}
                <strong>
                  «{selectedDetail.payout.sellerName || selectedDetail.payout.sellerId}»
                </strong>{' '}
                <strong>УЖЕ БЫЛ ПЕРЕВЕДЁН ВНЕ ПЛАТФОРМЫ</strong> (через клиент-банк или платёжный
                ордер).
              </p>
              <div className="p-3 bg-amber-50 dark:bg-amber-950/40 rounded-lg text-amber-800 dark:text-amber-200 border border-amber-200 dark:border-amber-800">
                Платформа ZAMK не отправляет банковские переводы автоматически. Это действие фиксирует
                выплату в реестре и балансе продавца. Отменить операцию будет невозможно.
              </div>
            </div>

            <div className="flex justify-end space-x-3 pt-3 border-t border-gray-100 dark:border-gray-700">
              <button
                type="button"
                disabled={isSubmitting}
                onClick={() => setShowConfirmProcessModal(false)}
                className="px-4 py-2 text-xs font-semibold text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
              >
                Отмена
              </button>
              <button
                type="button"
                disabled={isSubmitting}
                onClick={handleProcessConfirm}
                className="px-4 py-2 text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-700 rounded-lg shadow-sm disabled:opacity-50"
              >
                {isSubmitting ? 'Сохранение...' : 'Да, подтвердить факт выплаты'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Confirmation Modal for HoldPayoutBatch */}
      {showConfirmHoldModal && selectedDetail && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4">
          <div className="bg-white dark:bg-gray-800 rounded-2xl max-w-md w-full p-6 shadow-xl border border-gray-200 dark:border-gray-700 space-y-4">
            <div className="flex items-center space-x-3 text-amber-600">
              <Lock className="w-6 h-6 flex-shrink-0" />
              <h3 className="text-base font-bold text-gray-900 dark:text-white">
                Блокировка выплаты (Hold)
              </h3>
            </div>

            <p className="text-xs text-gray-600 dark:text-gray-300 leading-relaxed">
              Вы уверены, что хотите заблокировать партию выплаты на сумму{' '}
              <strong>{formatCents(selectedDetail.payout.amountCents)} ₽</strong> для продавца «
              {selectedDetail.payout.sellerName || selectedDetail.payout.sellerId}»? Партия будет
              переведена в статус <strong>held</strong>.
            </p>

            <div className="flex justify-end space-x-3 pt-3 border-t border-gray-100 dark:border-gray-700">
              <button
                type="button"
                disabled={isSubmitting}
                onClick={() => setShowConfirmHoldModal(false)}
                className="px-4 py-2 text-xs font-semibold text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
              >
                Отмена
              </button>
              <button
                type="button"
                disabled={isSubmitting}
                onClick={handleHoldConfirm}
                className="px-4 py-2 text-xs font-semibold text-white bg-amber-600 hover:bg-amber-700 rounded-lg shadow-sm disabled:opacity-50"
              >
                {isSubmitting ? 'Блокировка...' : 'Заблокировать выплату'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

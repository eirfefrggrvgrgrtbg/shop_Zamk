import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertCircle,
  Truck,
  Package,
  Clock,
  ExternalLink,
  RefreshCw,
  CheckCircle2,
  ChevronRight,
  Search,
  Filter,
  X,
} from 'lucide-react';
import {
  deliverAdminShipment,
  getAdminShipmentErrorMessage,
  getAdminShipments,
  getDeliveryErrorMessage,
  getShipmentWorkspaceStatusLabel,
  isShipmentEligibleForDelivery,
} from '../api/adminShipments';
import type { AdminShipmentView } from '../api/adminShipments';
import { getAdminDispatchQueue } from '../api/adminPicking';
import type { DispatchQueueItem } from '../api/adminPicking';
import { formatOrderNumber } from '../utils/orderFormatters';
import { PermissionGuard } from '../components/PermissionGuard';

export type ShipmentLifecycleTab = 'to_dispatch' | 'in_transit' | 'delivered' | 'problems';

export function getDisplaySellerName(sellerName?: string | null): string {
  if (sellerName && sellerName.trim().length > 0) {
    return sellerName.trim();
  }
  return 'Продавец';
}

export function formatPositionsCount(count?: number | null): string {
  const n = Math.max(0, Number(count) || 0);
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) {
    return `${n} позиция`;
  }
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
    return `${n} позиции`;
  }
  return `${n} позиций`;
}

export function formatUnitsCount(count?: number | null): string {
  const n = Math.max(0, Number(count) || 0);
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) {
    return `${n} единица`;
  }
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
    return `${n} единицы`;
  }
  return `${n} единиц`;
}

export function formatTransitDuration(shippedAt?: string | null, deliveredAt?: string | null): string | null {
  if (!shippedAt || !deliveredAt) return null;
  const start = new Date(shippedAt).getTime();
  const end = new Date(deliveredAt).getTime();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return null;

  const diffMs = end - start;
  const totalMinutes = Math.floor(diffMs / (1000 * 60));

  if (totalMinutes < 1) {
    return 'В пути: < 1 мин.';
  }
  if (totalMinutes < 60) {
    return `В пути: ${totalMinutes} мин.`;
  }

  const totalHours = Math.floor(totalMinutes / 60);
  const remainingMinutes = totalMinutes % 60;

  if (totalHours < 24) {
    if (remainingMinutes > 0) {
      return `В пути: ${totalHours} ч. ${remainingMinutes} мин.`;
    }
    return `В пути: ${totalHours} ч.`;
  }

  const days = Math.floor(totalHours / 24);
  const remainingHours = totalHours % 24;

  if (remainingHours > 0) {
    return `В пути: ${days} дн. ${remainingHours} ч.`;
  }
  return `В пути: ${days} дн.`;
}

function isSafeExternalUrl(url?: string | null): url is string {
  if (!url) return false;
  return /^https?:\/\//i.test(url.trim());
}

function toTimestampMs(value?: string | null): number {
  if (!value) return 0;
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function AdminShipments() {
  const [dispatchQueue, setDispatchQueue] = useState<DispatchQueueItem[]>([]);
  const [shipments, setShipments] = useState<AdminShipmentView[]>([]);
  const [activeTab, setActiveTab] = useState<ShipmentLifecycleTab>('to_dispatch');
  const [searchQuery, setSearchQuery] = useState('');
  const [carrierFilter, setCarrierFilter] = useState('');

  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const [deliveryTarget, setDeliveryTarget] = useState<AdminShipmentView | null>(null);
  const [isDelivering, setIsDelivering] = useState(false);
  const [deliveryError, setDeliveryError] = useState<string | null>(null);

  const fetchData = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const [queueData, shipmentsData] = await Promise.all([
        getAdminDispatchQueue(),
        getAdminShipments(),
      ]);
      setDispatchQueue(Array.isArray(queueData) ? queueData : []);
      setShipments(Array.isArray(shipmentsData) ? shipmentsData : []);
    } catch (err: unknown) {
      setError(getAdminShipmentErrorMessage(err, 'Не удалось загрузить данные отправлений.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleConfirmDelivery = async () => {
    if (!deliveryTarget) return;
    try {
      setIsDelivering(true);
      setDeliveryError(null);
      setSuccessMessage(null);
      await deliverAdminShipment(deliveryTarget.id);
      const label = `${formatOrderNumber({ id: deliveryTarget.orderId, orderNumber: deliveryTarget.orderNumber })} · ${getDisplaySellerName(deliveryTarget.sellerName)}`;
      setDeliveryTarget(null);
      setSuccessMessage(`Доставка отправления «${label}» подтверждена.`);
      await fetchData();
    } catch (err: unknown) {
      setDeliveryError(getDeliveryErrorMessage(err, 'Не удалось подтвердить доставку.'));
    } finally {
      setIsDelivering(false);
    }
  };

  const formatDate = (value?: string | null) =>
    value
      ? new Date(value).toLocaleString('ru-RU', {
          day: '2-digit',
          month: '2-digit',
          year: 'numeric',
          hour: '2-digit',
          minute: '2-digit',
        })
      : '—';

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'packed':
        return 'bg-amber-50 text-amber-800 border-amber-200';
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

  // Lifecycle buckets (unfiltered counts + deterministic default sort per tab)
  const toDispatchAll = useMemo(() => {
    return [...dispatchQueue]
      .filter((item) => !item.status || item.status === 'packed')
      .sort((a, b) => {
        const timeA = toTimestampMs(a.packedAt || a.createdAt);
        const timeB = toTimestampMs(b.packedAt || b.createdAt);
        return timeA - timeB;
      });
  }, [dispatchQueue]);

  const inTransitAll = useMemo(() => {
    return shipments
      .filter((s) => s.status === 'shipped')
      .sort((a, b) => toTimestampMs(b.shippedAt || b.createdAt) - toTimestampMs(a.shippedAt || a.createdAt));
  }, [shipments]);

  const deliveredAll = useMemo(() => {
    return shipments
      .filter((s) => s.status === 'delivered')
      .sort((a, b) => toTimestampMs(b.deliveredAt || b.updatedAt) - toTimestampMs(a.deliveredAt || a.updatedAt));
  }, [shipments]);

  const problemsAll = useMemo(() => {
    return shipments
      .filter((s) => s.status === 'failed' || s.status === 'cancelled')
      .sort((a, b) => toTimestampMs(b.updatedAt || b.createdAt) - toTimestampMs(a.updatedAt || a.createdAt));
  }, [shipments]);

  // Distinct non-empty carrier values from loaded shipments
  const availableCarriers = useMemo(() => {
    const set = new Set<string>();
    for (const s of shipments) {
      const c = s.carrier?.trim();
      if (c) {
        set.add(c);
      }
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b, 'ru'));
  }, [shipments]);

  const normalizedQuery = searchQuery.trim().toLowerCase();
  const hasActiveFilters = normalizedQuery.length > 0 || carrierFilter !== '';

  const filteredToDispatch = useMemo(() => {
    return toDispatchAll.filter((item) => {
      if (!normalizedQuery) return true;
      const displayOrder = formatOrderNumber({ id: item.orderId, orderNumber: item.orderNumber }).toLowerCase();
      const rawOrder = (item.orderNumber || '').toLowerCase();
      const seller = getDisplaySellerName(item.sellerName).toLowerCase();
      return (
        displayOrder.includes(normalizedQuery) ||
        rawOrder.includes(normalizedQuery) ||
        seller.includes(normalizedQuery)
      );
    });
  }, [toDispatchAll, normalizedQuery]);

  const filterShipmentsList = (list: AdminShipmentView[]) => {
    return list.filter((s) => {
      if (carrierFilter && (s.carrier || '').trim() !== carrierFilter) {
        return false;
      }
      if (!normalizedQuery) return true;
      const displayOrder = formatOrderNumber({ id: s.orderId, orderNumber: s.orderNumber }).toLowerCase();
      const rawOrder = (s.orderNumber || '').toLowerCase();
      const tracking = (s.trackingNumber || '').toLowerCase();
      const seller = getDisplaySellerName(s.sellerName).toLowerCase();
      return (
        displayOrder.includes(normalizedQuery) ||
        rawOrder.includes(normalizedQuery) ||
        tracking.includes(normalizedQuery) ||
        seller.includes(normalizedQuery)
      );
    });
  };

  const filteredInTransit = useMemo(
    () => filterShipmentsList(inTransitAll),
    [inTransitAll, carrierFilter, normalizedQuery]
  );
  const filteredDelivered = useMemo(
    () => filterShipmentsList(deliveredAll),
    [deliveredAll, carrierFilter, normalizedQuery]
  );
  const filteredProblems = useMemo(
    () => filterShipmentsList(problemsAll),
    [problemsAll, carrierFilter, normalizedQuery]
  );

  const tabs: Array<{
    key: ShipmentLifecycleTab;
    label: string;
    count: number;
  }> = [
    { key: 'to_dispatch', label: 'К отправке', count: toDispatchAll.length },
    { key: 'in_transit', label: 'В пути', count: inTransitAll.length },
    { key: 'delivered', label: 'Доставлены', count: deliveredAll.length },
    { key: 'problems', label: 'Проблемы', count: problemsAll.length },
  ];

  const resetFilters = () => {
    setSearchQuery('');
    setCarrierFilter('');
  };

  const renderEmptyState = (emptyText: string) => {
    if (hasActiveFilters) {
      return (
        <div
          data-testid="shipments-filtered-empty"
          className="p-6 text-center bg-white rounded-2xl border border-gray-200 shadow-sm space-y-2.5"
        >
          <Search className="w-7 h-7 text-gray-400 mx-auto" />
          <h3 className="text-sm font-bold text-gray-900">Ничего не найдено</h3>
          <p className="text-xs text-gray-500 max-w-md mx-auto">
            По вашему запросу или выбранной службе доставки отправления не найдены.
          </p>
          <div>
            <button
              type="button"
              onClick={resetFilters}
              className="inline-flex items-center px-3.5 py-1.5 rounded-xl text-xs font-semibold text-indigo-600 bg-indigo-50 hover:bg-indigo-100 transition-colors"
            >
              Сбросить фильтры
            </button>
          </div>
        </div>
      );
    }

    return (
      <div
        data-testid={`shipments-empty-${activeTab}`}
        className="p-6 text-center bg-white rounded-2xl border border-dashed border-gray-200 shadow-sm space-y-1.5"
      >
        <Package className="w-7 h-7 text-gray-400 mx-auto" />
        <h3 className="text-sm font-bold text-gray-900">{emptyText}</h3>
      </div>
    );
  };

  const searchPlaceholder =
    activeTab === 'to_dispatch' ? 'Заказ или продавец' : 'Заказ или трек-номер';

  return (
    <div data-testid="admin-shipments-page" className="space-y-6 pb-16">
      {/* Page Header */}
      <div className="sm:flex sm:items-center sm:justify-between border-b border-gray-200 pb-5">
        <div>
          <h1 className="text-2xl font-black text-gray-900 tracking-tight">Отправления</h1>
          <p className="mt-1 text-sm text-gray-500">
            Управление передачей заказов в доставку и контроль отправлений
          </p>
        </div>
        <button
          type="button"
          onClick={fetchData}
          disabled={isLoading}
          className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 rounded-xl text-xs font-semibold bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
          Обновить
        </button>
      </div>

      {successMessage && (
        <div className="p-4 bg-emerald-50 border border-emerald-200 text-emerald-800 rounded-xl flex items-center justify-between shadow-sm">
          <div className="flex items-center">
            <CheckCircle2 className="h-5 w-5 mr-2 shrink-0 text-emerald-600" />
            <span className="text-sm font-medium">{successMessage}</span>
          </div>
          <button
            type="button"
            onClick={() => setSuccessMessage(null)}
            className="text-emerald-600 hover:text-emerald-800 p-1 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 text-red-700 rounded-xl flex items-center shadow-sm">
          <AlertCircle className="h-5 w-5 mr-2 shrink-0 text-red-600" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* Lifecycle Tabs */}
      <div
        role="tablist"
        aria-label="Этапы отправлений"
        className="flex flex-wrap gap-2 border-b border-gray-200"
      >
        {tabs.map((tab) => {
          const isActive = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              data-testid={`shipments-tab-${tab.key}`}
              onClick={() => setActiveTab(tab.key)}
              className={`pb-3 px-3 text-sm font-semibold border-b-2 transition-colors inline-flex items-center gap-2 ${
                isActive
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-800'
              }`}
            >
              <span>{tab.label}</span>
              <span
                data-testid={`shipments-tab-count-${tab.key}`}
                className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-bold ${
                  isActive
                    ? 'bg-indigo-50 text-indigo-700 border border-indigo-200'
                    : 'bg-gray-100 text-gray-700'
                }`}
              >
                {tab.count}
              </span>
            </button>
          );
        })}
      </div>

      {/* Search & Carrier Filter */}
      <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex flex-col md:flex-row gap-3 items-stretch md:items-center justify-between">
        <div className="relative flex-1">
          <Search className="absolute left-3.5 top-2.5 h-4 w-4 text-gray-400" />
          <input
            type="text"
            aria-label={searchPlaceholder}
            placeholder={searchPlaceholder}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-4 py-2 text-xs sm:text-sm bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        <div className="flex flex-wrap gap-2 items-center">
          {activeTab !== 'to_dispatch' && (
            <>
              <div className="flex items-center gap-1.5 text-xs text-gray-500 mr-1">
                <Filter className="h-3.5 w-3.5" />
                <span>Служба:</span>
              </div>

              <select
                aria-label="Служба доставки"
                value={carrierFilter}
                onChange={(e) => setCarrierFilter(e.target.value)}
                className="px-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-xl text-gray-700 focus:bg-white focus:outline-none focus:border-indigo-500"
              >
                <option value="">Все службы</option>
                {availableCarriers.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </>
          )}

          {hasActiveFilters && (
            <button
              type="button"
              onClick={resetFilters}
              className="px-2.5 py-1.5 text-xs text-rose-600 hover:text-rose-700 font-semibold transition-colors"
            >
              Сбросить
            </button>
          )}
        </div>
      </div>

      {/* Content Area */}
      {isLoading ? (
        <div className="p-8 text-center bg-white rounded-2xl border border-gray-200 shadow-sm">
          <RefreshCw className="w-6 h-6 animate-spin mx-auto text-indigo-600 mb-2" />
          <p className="text-xs text-gray-500 font-medium">Загрузка отправлений...</p>
        </div>
      ) : activeTab === 'to_dispatch' ? (
        filteredToDispatch.length === 0 ? (
          renderEmptyState('Нет отправлений, ожидающих передачи')
        ) : (
          <div className="bg-white shadow-sm border border-gray-200 rounded-2xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Заказ / продавец
                    </th>
                    <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Статус
                    </th>
                    <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Состав
                    </th>
                    <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Доставка / трек
                    </th>
                    <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Дата
                    </th>
                    <th className="px-5 py-3.5 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Действие
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-100">
                  {filteredToDispatch.map((item) => {
                    const fulfillmentId = item.fulfillmentId || (item as any).id;
                    const displayOrderNumber = formatOrderNumber({
                      id: item.orderId,
                      orderNumber: item.orderNumber,
                    });
                    const displaySellerName = getDisplaySellerName(item.sellerName);
                    const itemsCount = item.itemsCount ?? ((item as any).items?.length || 0);
                    const unitsCount =
                      item.unitsCount ??
                      item.totalQuantity ??
                      ((item as any).items?.reduce((sum: number, it: any) => sum + it.quantity, 0) || 0);

                    return (
                      <tr key={fulfillmentId} className="hover:bg-gray-50/60 transition-colors">
                        <td className="px-5 py-4">
                          <div className="text-sm font-bold text-gray-900">{displayOrderNumber}</div>
                          <div className="text-xs text-gray-500 font-medium mt-0.5">
                            {displaySellerName}
                          </div>
                        </td>
                        <td className="px-5 py-4 whitespace-nowrap">
                          <span
                            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border ${getStatusBadgeClass(
                              'packed'
                            )}`}
                          >
                            {getShipmentWorkspaceStatusLabel('packed')}
                          </span>
                        </td>
                        <td className="px-5 py-4 whitespace-nowrap text-xs">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-gray-900">
                              {formatPositionsCount(itemsCount)}
                            </span>
                            <span className="text-gray-300" aria-hidden="true">
                              ·
                            </span>
                            <span className="text-gray-600">{formatUnitsCount(unitsCount)}</span>
                          </div>
                        </td>
                        <td className="px-5 py-4 text-xs text-gray-700">
                          <div className="font-medium text-gray-900">
                            {item.deliveryMethodName || 'Способ доставки не указан'}
                          </div>
                        </td>
                        <td className="px-5 py-4 whitespace-nowrap text-xs text-gray-600">
                          <div className="flex items-center gap-1.5">
                            <Clock className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                            <span>{formatDate(item.packedAt || item.createdAt)}</span>
                          </div>
                        </td>
                        <td className="px-5 py-4 whitespace-nowrap text-right text-xs">
                          <Link
                            to={`/fulfillment/dispatch/${fulfillmentId}`}
                            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-700 text-white font-bold rounded-xl transition-colors shadow-sm"
                          >
                            <Truck className="w-3.5 h-3.5" />
                            <span>Перейти к отгрузке</span>
                            <ChevronRight className="w-3.5 h-3.5" />
                          </Link>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )
      ) : (
        (() => {
          const activeList =
            activeTab === 'in_transit'
              ? filteredInTransit
              : activeTab === 'delivered'
              ? filteredDelivered
              : filteredProblems;

          const emptyLabel =
            activeTab === 'in_transit'
              ? 'Сейчас нет отправлений в пути'
              : activeTab === 'delivered'
              ? 'Доставленных отправлений пока нет'
              : 'Проблемных отправлений нет';

          const hasActionColumn = activeTab === 'in_transit';

          if (activeList.length === 0) {
            return renderEmptyState(emptyLabel);
          }

          return (
            <div className="bg-white shadow-sm border border-gray-200 rounded-2xl overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                        Заказ / продавец
                      </th>
                      <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                        Статус
                      </th>
                      <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                        Состав
                      </th>
                      <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                        Доставка / трек
                      </th>
                      <th className="px-5 py-3.5 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                        Дата
                      </th>
                      {hasActionColumn && (
                        <th className="px-5 py-3.5 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">
                          Действие
                        </th>
                      )}
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-100">
                    {activeList.map((shipment) => {
                      const displayOrderNumber = formatOrderNumber({
                        id: shipment.orderId,
                        orderNumber: shipment.orderNumber,
                      });
                      const displaySellerName = getDisplaySellerName(shipment.sellerName);
                      const transitDuration =
                        shipment.status === 'delivered'
                          ? formatTransitDuration(shipment.shippedAt, shipment.deliveredAt)
                          : null;

                      const carrierDisplay = shipment.carrier?.trim() || 'Служба не указана';
                      const trackingDisplay = shipment.trackingNumber?.trim() || 'Трек не указан';

                      return (
                        <tr key={shipment.id} className="hover:bg-gray-50/60 transition-colors">
                          <td className="px-5 py-4">
                            <div className="text-sm font-bold text-gray-900">{displayOrderNumber}</div>
                            <div className="text-xs text-gray-500 font-medium mt-0.5">
                              {displaySellerName}
                            </div>
                          </td>
                          <td className="px-5 py-4 whitespace-nowrap">
                            <span
                              className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border ${getStatusBadgeClass(
                                shipment.status
                              )}`}
                            >
                              {getShipmentWorkspaceStatusLabel(shipment.status)}
                            </span>
                          </td>
                          <td className="px-5 py-4 whitespace-nowrap text-xs">
                            <div className="flex items-center gap-1.5">
                              <span className="font-semibold text-gray-900">
                                {formatPositionsCount(shipment.itemsCount)}
                              </span>
                              <span className="text-gray-300" aria-hidden="true">
                                ·
                              </span>
                              <span className="text-gray-600">
                                {formatUnitsCount(shipment.unitsCount)}
                              </span>
                            </div>
                          </td>
                          <td className="px-5 py-4 text-xs text-gray-700">
                            <div className="font-semibold text-gray-900">{carrierDisplay}</div>
                            <div className="text-[11px] text-gray-500 font-mono mt-0.5">
                              {trackingDisplay}
                            </div>
                            {shipment.deliveryMethodName &&
                              shipment.deliveryMethodName !== shipment.carrier && (
                                <div className="text-[11px] text-gray-400 mt-0.5">
                                  {shipment.deliveryMethodName}
                                </div>
                              )}
                          </td>
                          <td className="px-5 py-4 whitespace-nowrap text-xs text-gray-600">
                            {shipment.status === 'delivered' ? (
                              <div className="space-y-0.5">
                                <div className="font-medium text-gray-900">
                                  {formatDate(shipment.deliveredAt)}
                                </div>
                                {shipment.shippedAt && (
                                  <div className="text-[11px] text-gray-500">
                                    Отправлено: {formatDate(shipment.shippedAt)}
                                  </div>
                                )}
                                {transitDuration && (
                                  <div className="text-[11px] font-medium text-emerald-700">
                                    {transitDuration}
                                  </div>
                                )}
                              </div>
                            ) : shipment.status === 'shipped' ? (
                              <div className="space-y-0.5">
                                <div className="font-medium text-gray-900">
                                  {formatDate(shipment.shippedAt)}
                                </div>
                                {shipment.packedAt && (
                                  <div className="text-[11px] text-gray-500">
                                    Упаковано: {formatDate(shipment.packedAt)}
                                  </div>
                                )}
                              </div>
                            ) : (
                              <div className="space-y-0.5">
                                <div className="font-medium text-gray-900">
                                  {formatDate(shipment.updatedAt || shipment.createdAt)}
                                </div>
                                {shipment.shippedAt && (
                                  <div className="text-[11px] text-gray-500">
                                    Отправлено: {formatDate(shipment.shippedAt)}
                                  </div>
                                )}
                              </div>
                            )}
                          </td>
                          {hasActionColumn && (
                            <td className="px-5 py-4 whitespace-nowrap text-right text-xs">
                              {isShipmentEligibleForDelivery(shipment.status) ? (
                                <div className="inline-flex items-center justify-end gap-2">
                                  {isSafeExternalUrl(shipment.trackingUrl) && (
                                    <a
                                      href={shipment.trackingUrl}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl border border-gray-200 text-gray-700 hover:bg-gray-50 font-semibold transition-colors"
                                    >
                                      <span>Отследить</span>
                                      <ExternalLink className="w-3 h-3" />
                                    </a>
                                  )}
                                  <PermissionGuard permission="shipments.update_status">
                                    <button
                                      type="button"
                                      onClick={() => {
                                        setDeliveryTarget(shipment);
                                        setDeliveryError(null);
                                      }}
                                      className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-700 text-white font-bold rounded-xl transition-colors shadow-sm text-xs"
                                    >
                                      <CheckCircle2 className="w-3.5 h-3.5" />
                                      <span>Подтвердить доставку</span>
                                    </button>
                                  </PermissionGuard>
                                </div>
                              ) : null}
                            </td>
                          )}
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          );
        })()
      )}

      {/* Confirmation Modal for Delivery */}
      {deliveryTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 shadow-2xl space-y-5 border border-gray-100">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center shrink-0">
                  <CheckCircle2 className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="text-lg font-bold text-gray-900">Подтвердить доставку?</h3>
                  <p className="text-xs text-gray-500">
                    {formatOrderNumber({
                      id: deliveryTarget.orderId,
                      orderNumber: deliveryTarget.orderNumber,
                    })}{' '}
                    · {getDisplaySellerName(deliveryTarget.sellerName)}
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => {
                  if (!isDelivering) {
                    setDeliveryTarget(null);
                    setDeliveryError(null);
                  }
                }}
                className="text-gray-400 hover:text-gray-600 p-1 rounded-lg transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-3">
              <p className="text-sm text-gray-600">
                Отметить отправление как доставленное покупателю?
              </p>

              <dl className="grid grid-cols-2 gap-2 p-3.5 bg-gray-50 rounded-xl text-xs">
                <div>
                  <dt className="text-gray-400 font-medium">Заказ</dt>
                  <dd className="font-semibold text-gray-900 truncate mt-0.5">
                    {formatOrderNumber({
                      id: deliveryTarget.orderId,
                      orderNumber: deliveryTarget.orderNumber,
                    })}
                  </dd>
                </div>
                <div>
                  <dt className="text-gray-400 font-medium">Продавец</dt>
                  <dd className="font-semibold text-gray-900 truncate mt-0.5">
                    {getDisplaySellerName(deliveryTarget.sellerName)}
                  </dd>
                </div>
                <div>
                  <dt className="text-gray-400 font-medium">Служба доставки</dt>
                  <dd className="font-semibold text-gray-900 mt-0.5">
                    {deliveryTarget.carrier || '—'}
                  </dd>
                </div>
                <div>
                  <dt className="text-gray-400 font-medium">Номер отслеживания</dt>
                  <dd className="font-semibold text-gray-900 truncate mt-0.5">
                    {deliveryTarget.trackingNumber || '—'}
                  </dd>
                </div>
              </dl>
            </div>

            {deliveryError && (
              <div className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 flex items-center gap-2.5 text-xs font-medium">
                <AlertCircle className="w-4 h-4 text-rose-600 shrink-0" />
                <span>{deliveryError}</span>
              </div>
            )}

            <div className="flex items-center justify-end gap-3 pt-2 border-t border-gray-100">
              <button
                type="button"
                onClick={() => {
                  setDeliveryTarget(null);
                  setDeliveryError(null);
                }}
                disabled={isDelivering}
                className="px-4 py-2.5 rounded-xl border border-gray-300 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleConfirmDelivery}
                disabled={isDelivering}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-700 text-xs font-bold text-white transition-colors shadow-sm disabled:opacity-50"
              >
                {isDelivering ? (
                  <>
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                    <span>Подтверждение...</span>
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    <span>Подтвердить доставку</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

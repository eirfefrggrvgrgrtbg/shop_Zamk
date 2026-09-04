import { useState, useEffect, useMemo, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  getReconciliationRoute,
  startInventoryReconciliation,
  getActiveInventoryReconciliation,
  listInventoryReconciliations,
  type ReconciliationSession,
  getAdminInventoryItem,
  getAdminInventoryMovements,
  type AdminInventoryView,
  type AdminInventoryPhysicalUnit,
  type AdminInventoryMovementView,
} from '../../api/adminInventory';
import {
  X,
  Search,
  Copy,
  Check,
  AlertTriangle,
  CheckCircle2,
  Boxes,
  Package,
  ExternalLink,
  History,
  ScanLine,
} from 'lucide-react';
import { orderStatusMap, fulfillmentStatusMap, formatDateTime } from '../../utils/orderFormatters';
import { ZmuTraceabilityDrawer } from './ZmuTraceabilityDrawer';

const formatMovementType = (type: string) => {
  switch (type) {
    case 'receipt':
      return { label: 'Поступление', badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200' };
    case 'sale':
      return { label: 'Продажа', badgeClass: 'bg-indigo-50 text-indigo-700 border-indigo-200' };
    case 'reservation_created':
      return { label: 'Резерв', badgeClass: 'bg-amber-50 text-amber-700 border-amber-200' };
    case 'reservation_released':
      return { label: 'Снятие резерва', badgeClass: 'bg-blue-50 text-blue-700 border-blue-200' };
    case 'return':
      return { label: 'Возврат', badgeClass: 'bg-purple-50 text-purple-700 border-purple-200' };
    case 'write_off':
      return { label: 'Списание', badgeClass: 'bg-rose-50 text-rose-700 border-rose-200' };
    case 'adjustment':
      return { label: 'Корректировка', badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' };
    default:
      return { label: type, badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' };
  }
};

const getMovementRefLink = (refType?: string, refId?: string) => {
  if (!refId) return null;
  switch (refType) {
    case 'order':
      return `/orders/${refId}`;
    case 'supply':
      return `/supply-receiving?id=${refId}`;
    case 'return':
      return `/returns?id=${refId}`;
    default:
      return null;
  }
};

const ISSUE_LABELS: Record<string, string> = {
  stale_active_allocation: 'Физическая единица содержит активное назначение на завершённый заказ.',
  allocated_exceeds_physical: 'Аллокации ZMU превышают количество на складе',
  picked_exceeds_allocated: 'Собранных ZMU больше, чем активных аллокаций',
  physical_exceeds_aggregate: 'Физических ZMU больше, чем коммерческий остаток',
  serialized_allocations_exceed_reserved: 'Аллокаций ZMU больше, чем коммерческий резерв',
  reserved_exceeds_total: 'Резерв превышает общий остаток',
  aggregate_available_negative: 'Отрицательный доступный остаток',
  legacy_projection_negative: 'Отрицательная проекция legacy остатка',
  legacy_reserved_negative: 'Отрицательный legacy резерв',
  legacy_available_negative: 'Отрицательная legacy доступность',
};

const formatRussianOrderStatus = (status?: string | null): string => {
  if (!status) return '—';
  const s = status.toLowerCase();
  if (orderStatusMap[s]) return orderStatusMap[s].label;
  if (fulfillmentStatusMap[s]) return fulfillmentStatusMap[s].label;
  switch (s) {
    case 'delivered':
      return 'Доставлен';
    case 'paid':
      return 'Оплачен';
    case 'assembling':
      return 'Собирается';
    case 'packed':
      return 'Упакован';
    case 'shipped':
      return 'В пути';
    case 'cancelled':
      return 'Отменён';
    case 'returned':
      return 'Возвращён';
    case 'refunded':
      return 'Возврат средств';
    case 'awaiting_payment':
      return 'Ожидает оплаты';
    case 'accepted':
      return 'Принята на хабе';
    case 'completed':
      return 'Завершён';
    case 'completed_with_discrepancies':
      return 'Завершён с расхождениями';
    default:
      return status;
  }
};

const formatDateDot = (dateStr?: string | null): string => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  const day = String(d.getDate()).padStart(2, '0');
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const year = d.getFullYear();
  const hours = String(d.getHours()).padStart(2, '0');
  const minutes = String(d.getMinutes()).padStart(2, '0');
  return `${day}.${month}.${year} · ${hours}:${minutes}`;
};

const formatSessionDate = (dateStr?: string | null): string => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  const now = new Date();
  const isToday = d.toDateString() === now.toDateString();
  if (isToday) return 'Сегодня';
  const day = String(d.getDate()).padStart(2, '0');
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const year = d.getFullYear();
  return `${day}.${month}.${year}`;
};

interface VariantInventoryDrawerProps {
  item: AdminInventoryView | null;
  isOpen: boolean;
  onClose: () => void;
  highlightUnitCode?: string | null;
}

export function VariantInventoryDrawer({
  item,
  isOpen,
  onClose,
  highlightUnitCode,
}: VariantInventoryDrawerProps) {
  const [detailItem, setDetailItem] = useState<AdminInventoryView | null>(item);
  const [isLoadingDetail, setIsLoadingDetail] = useState(true);
  const [unitFilter, setUnitFilter] = useState<string>('all');
  const [unitSearch, setUnitSearch] = useState<string>('');
  const [copiedUnitCode, setCopiedUnitCode] = useState<string | null>(null);
  const [selectedZmuCode, setSelectedZmuCode] = useState<string | null>(highlightUnitCode || null);

  const [activeSession, setActiveSession] = useState<ReconciliationSession | null>(null);
  const [recentSessions, setRecentSessions] = useState<ReconciliationSession[]>([]);
  const [isLoadingSessions, setIsLoadingSessions] = useState(false);
  const [reconcileError, setReconcileError] = useState<string | null>(null);
  const [isStartConfirmOpen, setIsStartConfirmOpen] = useState(false);
  const [isStartingReconciliation, setIsStartingReconciliation] = useState(false);

  const [activeDrawerTab, setActiveDrawerTab] = useState<'units' | 'movements'>('units');
  const [movements, setMovements] = useState<AdminInventoryMovementView[]>([]);
  const [isLoadingMovements, setIsLoadingMovements] = useState(false);
  const [movementsError, setMovementsError] = useState<string | null>(null);

  const navigate = useNavigate();
  const bodyRef = useRef<HTMLDivElement | null>(null);
  const highlightedRowRef = useRef<HTMLTableRowElement | null>(null);

  const handleConfirmStart = async () => {
    if (!variantId) return;
    setIsStartingReconciliation(true);
    setReconcileError(null);
    try {
      const session = await startInventoryReconciliation(variantId);
      setIsStartConfirmOpen(false);
      navigate(getReconciliationRoute(session.id));
    } catch (e: any) {
      setReconcileError(e.response?.data?.message || e.message || 'Ошибка запуска инвентаризации');
    } finally {
      setIsStartingReconciliation(false);
    }
  };

  // Load fresh item details with physical units when drawer opens
  useEffect(() => {
    if (item && isOpen) {
      setDetailItem(item);
      setIsLoadingDetail(true);
      if (highlightUnitCode) {
        setUnitSearch('');
        setUnitFilter('all');
      }
      getAdminInventoryItem(item.id)
        .then((data) => {
          setDetailItem(data);
        })
        .catch(() => {
          setDetailItem(item);
        })
        .finally(() => {
          setIsLoadingDetail(false);
        });
    }
  }, [item?.id, isOpen, highlightUnitCode]);

  // Load movements when movements tab is activated
  useEffect(() => {
    if (item && isOpen && activeDrawerTab === 'movements') {
      setIsLoadingMovements(true);
      setMovementsError(null);
      getAdminInventoryMovements(item.id)
        .then((data) => {
          setMovements(data);
        })
        .catch((err) => {
          setMovementsError(err instanceof Error ? err.message : 'Ошибка загрузки движений');
        })
        .finally(() => {
          setIsLoadingMovements(false);
        });
    }
  }, [item?.id, isOpen, activeDrawerTab]);

  const activeItem = detailItem || item;
  const variantId = activeItem?.productVariantId || activeItem?.id;

  // Load active session and session history
  useEffect(() => {
    if (variantId && isOpen && activeItem?.accountingMode !== 'legacy') {
      getActiveInventoryReconciliation(variantId)
        .then((res) => setActiveSession(res))
        .catch(() => setActiveSession(null));

      setIsLoadingSessions(true);
      listInventoryReconciliations(variantId, 5)
        .then((items) => setRecentSessions(items))
        .catch(() => setRecentSessions([]))
        .finally(() => setIsLoadingSessions(false));
    }
  }, [variantId, isOpen, activeItem?.accountingMode]);

  // Escape key closes drawer
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  }, [isOpen, onClose]);

  // Exact ZMU handoff: scroll to highlighted unit after drawer layout mounts
  useEffect(() => {
    if (highlightUnitCode) {
      setSelectedZmuCode(highlightUnitCode);
      if (highlightedRowRef.current) {
        const timer = setTimeout(() => {
          highlightedRowRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }, 120);
        return () => clearTimeout(timer);
      }
    }
  }, [highlightUnitCode, activeItem?.physicalUnits]);

  // Scroll contract: normal opening always starts at top (scrollTop = 0)
  useEffect(() => {
    if (isOpen && bodyRef.current) {
      bodyRef.current.scrollTop = 0;
    }
  }, [isOpen, item?.id]);

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedUnitCode(text);
    setTimeout(() => setCopiedUnitCode(null), 1800);
  };

  const units: AdminInventoryPhysicalUnit[] = useMemo(() => {
    return activeItem?.physicalUnits || [];
  }, [activeItem?.physicalUnits]);



  // Filter and search units
  const filteredUnits = useMemo(() => {
    return units.filter((unit) => {
      // Local search filter
      if (unitSearch.trim()) {
        const query = unitSearch.trim().toLowerCase();
        const matchesCode = unit.unitCode.toLowerCase().includes(query);
        const matchesOrder =
          unit.liveAllocation?.orderNumber?.toLowerCase().includes(query) ||
          unit.staleAllocation?.orderNumber?.toLowerCase().includes(query);
        const matchesSupply = unit.supplyLineage?.supplyNumber?.toLowerCase().includes(query);
        if (!matchesCode && !matchesOrder && !matchesSupply) {
          return false;
        }
      }

      // Tab filter: "issues" (Расхождения) strictly represents health / invariant issues (e.g. stale active allocation)
      switch (unitFilter) {
        case 'warehouse':
          return unit.status === 'warehouse';
        case 'free':
          return unit.status === 'warehouse' && unit.availability === 'free';
        case 'allocated':
          return unit.status === 'warehouse' && (unit.availability === 'allocated' || unit.availability === 'picked');
        case 'expected':
          return unit.status === 'expected';
        case 'damaged':
          return unit.status === 'damaged';
        case 'shipped':
          return unit.status === 'shipped';
        case 'issues':
          return unit.isStaleAllocation;
        case 'all':
        default:
          return true;
      }
    });
  }, [units, unitFilter, unitSearch]);

  if (!isOpen || !activeItem) return null;

  const agg = activeItem.aggregate;
  const phys = activeItem.physical;
  const leg = activeItem.legacy;
  const isWarning = activeItem.health.status !== 'healthy';
  const issues = activeItem.health.issues;
  const isLegacyPure = activeItem.accountingMode === 'legacy' || (phys.warehouse === 0 && agg.total > 0 && units.length === 0);

  // Find any unit with stale allocation for diagnostic callout
  const staleUnit = units.find((u) => u.isStaleAllocation && u.status === 'warehouse');

  const getAccountingBadge = (mode?: string) => {
    switch (mode) {
      case 'serialized':
        return (
          <span
            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-purple-50 text-purple-700 border border-purple-200 cursor-help"
            title="Каждая физическая единица учитывается по уникальному ZMU."
          >
            Серийный
          </span>
        );
      case 'mixed':
        return (
          <span
            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200 cursor-help"
            title="Часть остатка учитывается по ZMU, часть — количественно."
          >
            Смешанный
          </span>
        );
      case 'legacy':
      default:
        return (
          <span
            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-300 cursor-help"
            title="Физические единицы этого остатка отдельно не отслеживаются."
          >
            Без ZMU
          </span>
        );
    }
  };

  const getPhysicalStatusBadge = (status: string) => {
    switch (status) {
      case 'warehouse':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200">
            На складе
          </span>
        );
      case 'expected':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200">
            Ожидается
          </span>
        );
      case 'damaged':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-rose-50 text-rose-700 border border-rose-200">
            Брак
          </span>
        );
      case 'written_off':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600 border border-slate-200">
            Списана
          </span>
        );
      case 'shipped':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-50 text-indigo-700 border border-indigo-200">
            Отгружена
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600 border border-slate-200">
            {status}
          </span>
        );
    }
  };

  const getAvailabilityContent = (unit: AdminInventoryPhysicalUnit) => {
    if (unit.status === 'warehouse') {
      if (unit.availability === 'free') {
        if (unit.isStaleAllocation) {
          return (
            <div>
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                Свободна
              </span>
              <div className="text-[11px] text-amber-700 flex items-center gap-1 mt-1 font-medium">
                <AlertTriangle className="w-3 h-3 text-amber-600 flex-shrink-0" />
                <span>Старое назначение {unit.staleAllocation?.orderNumber || 'заказа'}</span>
              </div>
            </div>
          );
        }
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            Свободна
          </span>
        );
      }
      if (unit.availability === 'allocated') {
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200">
            Назначена заказу
          </span>
        );
      }
      if (unit.availability === 'picked') {
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-purple-50 text-purple-700 border border-purple-200">
            Собрана
          </span>
        );
      }
    }

    switch (unit.status) {
      case 'expected':
        return <span className="text-xs text-slate-500 italic">Недоступна — ожидается</span>;
      case 'damaged':
        return <span className="text-xs text-rose-600 font-medium">Недоступна — брак</span>;
      case 'written_off':
        return <span className="text-xs text-slate-400 italic">Недоступна</span>;
      case 'shipped':
        return <span className="text-xs text-slate-500 italic">Не на складе</span>;
      default:
        return <span className="text-xs text-slate-400">—</span>;
    }
  };

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs z-40 transition-opacity"
        onClick={onClose}
      />

      {/* Drawer Container (70-75% viewport width, min-w-0, full height 100dvh) */}
      <div
        data-testid="variant-inventory-drawer"
        className="fixed inset-y-0 right-0 h-screen h-[100dvh] w-full lg:w-[75vw] lg:min-w-[768px] lg:max-w-[1280px] bg-white shadow-2xl z-50 flex flex-col border-l border-slate-200 min-w-0"
      >
        {/* Header - Fixed top bar (shrink-0) */}
        <div className="px-6 pt-5 pb-4 border-b border-slate-200 flex items-start justify-between gap-4 bg-white shrink-0">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2.5 flex-wrap">
              <h2 className="text-lg font-bold text-slate-900 truncate">
                {activeItem.productTitle}
              </h2>
              {getAccountingBadge(activeItem.accountingMode)}
              {isWarning && (
                <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
                  <AlertTriangle className="w-3.5 h-3.5 text-rose-600 flex-shrink-0" />
                  <span>Расхождение</span>
                </span>
              )}
            </div>

            <div className="text-sm text-slate-600 font-medium mt-1">
              {activeItem.size || activeItem.color
                ? `${activeItem.size || '-'} · ${activeItem.color || '-'}`
                : activeItem.variant}
            </div>

            <div className="flex items-center gap-4 text-xs text-slate-500 mt-2 flex-wrap">
              {activeItem.barcode && (
                <span className="font-mono bg-slate-100 px-1.5 py-0.5 rounded text-slate-700">
                  ZMK: {activeItem.barcode}
                </span>
              )}
              {activeItem.sku && (
                <span className="font-mono bg-slate-100 px-1.5 py-0.5 rounded text-slate-700">
                  SKU: {activeItem.sku}
                </span>
              )}
              <span className="text-slate-400">·</span>
              <span className="text-slate-700">
                Продавец:{' '}
                <span className="font-medium text-slate-900">
                  {activeItem.source === 'auction_direct_sale' ? 'ZAMK (Свой склад)' : activeItem.sellerName}
                </span>
              </span>
            </div>
          </div>

          <div className="flex items-center gap-3 shrink-0">
            <button
              type="button"
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors cursor-pointer"
              title="Закрыть (Esc)"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Scrollable Drawer Body - flex: 1, min-h: 0, overflow-y: auto */}
        <div ref={bodyRef} className="flex-1 min-h-0 overflow-y-auto p-6 space-y-6">
          {/* Top Balance Section - Compact 4-Card Grid */}
          <div className="bg-slate-50/80 border border-slate-200 rounded-xl p-4 shadow-2xs">
            <div className="text-xs font-bold uppercase tracking-wider text-slate-500 mb-3 flex items-center justify-between">
              <span>Складской баланс</span>
              {isWarning ? (
                <span className="inline-flex items-center gap-1 text-xs text-rose-700 font-semibold bg-rose-100/70 px-2 py-0.5 rounded">
                  <AlertTriangle className="w-3.5 h-3.5 text-rose-600" />
                  {issues.length} {issues.length === 1 ? 'расхождение' : 'расхождения'}
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-xs text-emerald-700 font-semibold bg-emerald-100/70 px-2 py-0.5 rounded">
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                  В норме
                </span>
              )}
            </div>

            <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
              {/* 1. Коммерческий итог */}
              <div className="bg-white p-3.5 rounded-lg border border-slate-200 shadow-2xs">
                <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider whitespace-nowrap">
                  Коммерческий итог
                </div>
                <div className="mt-2.5 space-y-1 text-xs">
                  <div className="flex justify-between">
                    <span className="text-slate-600">Всего:</span>
                    <span className="font-bold text-slate-900 font-mono">{agg.total}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">В резерве:</span>
                    <span className="font-semibold text-amber-700 font-mono">{agg.reserved}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Доступно:</span>
                    <span className="font-bold text-emerald-700 font-mono">{agg.available}</span>
                  </div>
                </div>
              </div>

              {/* 2. Физические ZMU */}
              <div className="bg-white p-3.5 rounded-lg border border-slate-200 shadow-2xs">
                <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider whitespace-nowrap">
                  Физические ZMU
                </div>
                <div className="mt-2.5 space-y-1 text-xs">
                  <div className="flex justify-between">
                    <span className="text-slate-600">На складе:</span>
                    <span className="font-bold text-slate-900 font-mono">{phys.warehouse}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Свободно:</span>
                    <span className="font-semibold text-emerald-700 font-mono">{phys.free}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Назначено:</span>
                    <span className="font-semibold text-amber-700 font-mono">{phys.allocated}</span>
                  </div>
                  <div className="flex justify-between text-slate-400 text-[11px]">
                    <span>(в т.ч. собрано):</span>
                    <span className="font-mono">{phys.picked}</span>
                  </div>
                </div>
              </div>

              {/* 3. БЕЗ ZMU */}
              <div className="bg-white p-3.5 rounded-lg border border-slate-200 shadow-2xs">
                <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider whitespace-nowrap">
                  БЕЗ ZMU
                </div>
                <div className="mt-2.5 space-y-1 text-xs">
                  <div className="flex justify-between">
                    <span className="text-slate-600">На складе:</span>
                    <span className="font-bold text-slate-900 font-mono">{leg.onHand}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">В резерве:</span>
                    <span className="font-semibold text-slate-700 font-mono">{leg.reserved}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Доступно:</span>
                    <span className="font-bold text-slate-900 font-mono">{leg.available}</span>
                  </div>
                </div>
              </div>

              {/* 4. Вне склада */}
              <div className="bg-white p-3.5 rounded-lg border border-slate-200 shadow-2xs">
                <div className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider whitespace-nowrap">
                  Вне склада
                </div>
                <div className="mt-2.5 space-y-1 text-xs">
                  <div className="flex justify-between">
                    <span className="text-slate-600">Ожидается:</span>
                    <span className="font-semibold text-blue-700 font-mono">+{phys.expected}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Брак:</span>
                    <span className="font-semibold text-rose-700 font-mono">{phys.damaged}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Списано:</span>
                    <span className="text-slate-500 font-mono">{phys.writtenOff}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-600">Отгружено:</span>
                    <span className="text-slate-500 font-mono">{phys.shipped}</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Health issues alert box if issues exist */}
            {isWarning && (
              <div className="mt-3 p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-900">
                <div className="font-semibold flex items-center gap-1.5 mb-1 text-rose-800">
                  <AlertTriangle className="w-4 h-4 text-rose-600 flex-shrink-0" />
                  <span>Обнаружены расхождения в учёте:</span>
                </div>
                <ul className="list-disc list-inside space-y-0.5 text-rose-800">
                  {issues.map((code) => (
                    <li key={code}>{ISSUE_LABELS[code] || code}</li>
                  ))}
                </ul>
                {staleUnit && (
                  <div className="mt-2 pt-2 border-t border-rose-200/60 text-[11px] text-rose-700 font-mono">
                    Единица: <span className="font-bold">{staleUnit.unitCode}</span> · Исторический заказ:{' '}
                    <span className="font-bold">{staleUnit.staleAllocation?.orderNumber || 'ORD-?'}</span> ·{' '}
                    <span>{formatRussianOrderStatus(staleUnit.staleAllocation?.orderStatus)}</span>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Compact Session History Section */}
          {activeItem.accountingMode !== 'legacy' && (
            <div className="bg-slate-50/80 border border-slate-200 rounded-xl p-4 shadow-2xs">
              <div className="text-xs font-bold uppercase tracking-wider text-slate-600 mb-2 flex items-center justify-between">
                <span className="flex items-center gap-1.5">
                  <ScanLine className="w-3.5 h-3.5 text-slate-600" />
                  Инвентаризации
                </span>
                <div className="flex items-center gap-2">
                  {isLoadingSessions && <span className="text-[10px] text-slate-400 font-normal">Загрузка...</span>}
                  {!isLoadingSessions && !activeSession && recentSessions.length > 0 && (
                    <button
                      type="button"
                      onClick={() => setIsStartConfirmOpen(true)}
                      className="px-2.5 py-1 text-xs font-semibold text-slate-700 hover:text-slate-900 bg-white hover:bg-slate-100 border border-slate-200 rounded-md transition-colors cursor-pointer"
                    >
                      Начать инвентаризацию
                    </button>
                  )}
                </div>
              </div>
              {recentSessions.length === 0 && !isLoadingSessions ? (
                <div className="flex items-center justify-between py-1 text-xs text-slate-500">
                  <span>Инвентаризации ещё не проводились</span>
                  <button
                    type="button"
                    onClick={() => setIsStartConfirmOpen(true)}
                    className="px-2.5 py-1 text-xs font-semibold text-slate-700 hover:text-slate-900 bg-white hover:bg-slate-100 border border-slate-200 rounded-md transition-colors cursor-pointer"
                  >
                    Начать инвентаризацию
                  </button>
                </div>
              ) : (
                <div className="space-y-1.5 mt-2">
                  {recentSessions.map((s) => {
                    const isAct = s.status === 'in_progress' || s.status === 'review';
                    const isDone = s.status === 'completed';
                    const dateLabel = formatSessionDate(s.startedAt);
                    const problems = s.problemsCount + s.unexpectedCount;

                    return (
                      <div
                        key={s.id}
                        className="bg-white p-2.5 rounded-lg border border-slate-200 flex items-center justify-between text-xs"
                      >
                        <div className="space-y-0.5 min-w-0">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-slate-800">{dateLabel}</span>
                            <span className="text-slate-400">·</span>
                            <span
                              className={
                                isAct
                                  ? 'text-amber-600 font-semibold'
                                  : isDone
                                  ? 'text-emerald-700 font-semibold'
                                  : 'text-slate-500 font-semibold'
                              }
                            >
                              {isAct ? 'В процессе' : isDone ? 'Завершена' : 'Отменена'}
                            </span>
                          </div>
                          <div className="text-slate-600 text-[11px] flex items-center gap-1.5">
                            <span>
                              {s.foundExpectedCount} / {s.expectedCount} найдено
                            </span>
                            {isDone && problems > 0 && (
                              <>
                                <span className="text-slate-400">·</span>
                                <span className="text-rose-600 font-medium">
                                  {problems} {problems === 1 ? 'расхождение' : 'расхождения'}
                                </span>
                              </>
                            )}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => navigate(getReconciliationRoute(s.id))}
                          className={`px-3 py-1 rounded-md text-xs font-semibold cursor-pointer transition-colors ${
                            isAct
                              ? 'bg-indigo-600 hover:bg-indigo-700 text-white'
                              : 'bg-slate-100 hover:bg-slate-200 text-slate-700'
                          }`}
                        >
                          {isAct ? 'Продолжить' : 'Открыть'}
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}

          {/* Tabs: Физические единицы / История */}
          <div className="flex border-b border-slate-200 gap-2">
            <button
              type="button"
              onClick={() => setActiveDrawerTab('units')}
              className={`pb-2.5 px-4 text-xs font-bold border-b-2 transition-colors cursor-pointer flex items-center gap-2 ${
                activeDrawerTab === 'units'
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
              }`}
            >
              <span>Физические единицы</span>
              <span
                className={`px-1.5 py-0.5 rounded text-[11px] ${
                  activeDrawerTab === 'units' ? 'bg-indigo-50 text-indigo-700 font-semibold' : 'bg-slate-100 text-slate-600'
                }`}
              >
                {units.length}
              </span>
            </button>
            <button
              type="button"
              onClick={() => setActiveDrawerTab('movements')}
              className={`pb-2.5 px-4 text-xs font-bold border-b-2 transition-colors cursor-pointer flex items-center gap-2 ${
                activeDrawerTab === 'movements'
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
              }`}
            >
              <History className="w-3.5 h-3.5" />
              <span>История</span>
              {movements.length > 0 && (
                <span
                  className={`px-1.5 py-0.5 rounded text-[11px] ${
                    activeDrawerTab === 'movements' ? 'bg-indigo-50 text-indigo-700 font-semibold' : 'bg-slate-100 text-slate-600'
                  }`}
                >
                  {movements.length}
                </span>
              )}
            </button>
          </div>

          {activeDrawerTab === 'movements' ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-slate-900 uppercase tracking-wider">
                    История движений варианта
                  </h3>
                  <p className="text-xs text-slate-500 mt-0.5">
                    Складские транзакции и агрегированные изменения остатков
                  </p>
                </div>
                <span className="text-xs text-slate-500">
                  Всего движений: <span className="font-semibold text-slate-800">{movements.length}</span>
                </span>
              </div>

              {isLoadingMovements ? (
                <div className="text-center py-12 bg-slate-50 rounded-xl border border-slate-200">
                  <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600 mx-auto"></div>
                  <p className="mt-2 text-xs text-slate-500">Загрузка истории движений...</p>
                </div>
              ) : movementsError ? (
                <div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700">
                  {movementsError}
                </div>
              ) : movements.length === 0 ? (
                <div className="text-center py-12 bg-slate-50 rounded-xl border border-slate-200">
                  <History className="w-8 h-8 text-slate-300 mx-auto mb-1" />
                  <p className="text-xs text-slate-500">
                    Для данного варианта пока нет зафиксированных движений остатков.
                  </p>
                </div>
              ) : (
                <div className="w-full min-w-0 border border-slate-200 rounded-xl overflow-x-auto shadow-2xs">
                  <table className="min-w-full divide-y divide-slate-200 text-xs">
                    <thead className="bg-slate-50">
                      <tr>
                        <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                          Дата и время
                        </th>
                        <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                          Операция
                        </th>
                        <th className="px-3 py-2.5 text-right font-semibold text-slate-600 uppercase tracking-wider">
                          Количество
                        </th>
                        <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                          Основание / Документ
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 bg-white">
                      {movements.map((m) => {
                        const mVisual = formatMovementType(m.type);
                        const refLink = getMovementRefLink(m.referenceType, m.referenceId);
                        const isPositive = m.quantity > 0;

                        return (
                          <tr key={m.id} className="hover:bg-slate-50/70 transition-colors">
                            <td className="px-3 py-2.5 whitespace-nowrap text-slate-600 font-mono">
                              {formatDateTime(m.createdAt)}
                            </td>
                            <td className="px-3 py-2.5 whitespace-nowrap">
                              <span
                                className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold border ${mVisual.badgeClass}`}
                              >
                                {mVisual.label}
                              </span>
                            </td>
                            <td className="px-3 py-2.5 whitespace-nowrap text-right font-mono font-bold">
                              <span className={isPositive ? 'text-emerald-700' : 'text-slate-900'}>
                                {isPositive ? `+${m.quantity}` : m.quantity}
                              </span>
                            </td>
                            <td className="px-3 py-2.5">
                              {m.reason && (
                                <div className="text-slate-800 font-medium text-xs mb-0.5">
                                  {m.reason}
                                </div>
                              )}
                              {m.referenceId && (
                                <div className="text-[11px] text-slate-500 font-mono flex items-center gap-1">
                                  <span>{m.referenceType || 'Документ'}:</span>
                                  {refLink ? (
                                    <Link
                                      to={refLink}
                                      className="text-indigo-600 hover:text-indigo-800 underline inline-flex items-center gap-0.5"
                                    >
                                      <span>{m.referenceId.slice(0, 8)}...</span>
                                      <ExternalLink className="w-2.5 h-2.5" />
                                    </Link>
                                  ) : (
                                    <span className="text-slate-700">{m.referenceId.slice(0, 8)}...</span>
                                  )}
                                </div>
                              )}
                              {!m.reason && !m.referenceId && <span className="text-slate-400">—</span>}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          ) : (
            /* Physical Units Section */
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-bold text-slate-900 uppercase tracking-wider">
                  Физические единицы
                </h3>
                <span className="text-xs text-slate-500">
                  Всего единиц: <span className="font-semibold text-slate-800">{units.length}</span>
                </span>
              </div>

            {isLegacyPure ? (
              /* Pure legacy variant graceful empty state */
              <div className="p-8 text-center bg-slate-50 border border-slate-200 rounded-xl my-4">
                <Boxes className="w-10 h-10 text-slate-400 mx-auto mb-2" />
                <h4 className="text-sm font-semibold text-slate-800">БЕЗ ZMU</h4>
                <p className="text-xs text-slate-500 mt-1 max-w-md mx-auto">
                  Физические единицы ZMU для этого остатка не ведутся. Учёт ведётся агрегированно по количеству на складе (
                  {leg.onHand} шт.).
                </p>
              </div>
            ) : (
              <>
                {/* Search and Filters Bar */}
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
                  {/* Local Search Input */}
                  <div className="relative flex-1 max-w-xs">
                    <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                      <Search className="h-3.5 w-3.5 text-slate-400" />
                    </div>
                    <input
                      type="text"
                      className="block w-full pl-9 pr-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 placeholder:text-slate-400 focus:bg-white focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors shadow-2xs"
                      placeholder="Найти ZMU..."
                      value={unitSearch}
                      onChange={(e) => setUnitSearch(e.target.value)}
                    />
                  </div>

                  {/* Filter Pills - "Расхождения" strictly counts invariant issues */}
                  <div className="flex items-center gap-1 overflow-x-auto pb-1 text-xs">
                    {[
                      { id: 'all', label: 'Все', count: units.length },
                      { id: 'warehouse', label: 'На складе', count: units.filter((u) => u.status === 'warehouse').length },
                      { id: 'free', label: 'Свободные', count: units.filter((u) => u.status === 'warehouse' && u.availability === 'free').length },
                      { id: 'allocated', label: 'Назначенные', count: units.filter((u) => u.status === 'warehouse' && (u.availability === 'allocated' || u.availability === 'picked')).length },
                      { id: 'expected', label: 'Ожидаются', count: units.filter((u) => u.status === 'expected').length },
                      { id: 'damaged', label: 'Брак', count: units.filter((u) => u.status === 'damaged').length },
                      { id: 'shipped', label: 'Отгруженные', count: units.filter((u) => u.status === 'shipped').length },
                      { id: 'issues', label: 'Расхождения', count: units.filter((u) => u.isStaleAllocation).length },
                    ].map((tab) => (
                      <button
                        key={tab.id}
                        type="button"
                        onClick={() => setUnitFilter(tab.id)}
                        className={`px-2.5 py-1 rounded-lg font-medium whitespace-nowrap transition-colors cursor-pointer ${
                          unitFilter === tab.id
                            ? 'bg-slate-900 text-white shadow-2xs'
                            : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                        }`}
                      >
                        {tab.label} ({tab.count})
                      </button>
                    ))}
                  </div>
                </div>

                {/* Units Table - 5 Columns: ZMU, ФИЗИЧЕСКОЕ СОСТОЯНИЕ, ДОСТУПНОСТЬ, ТЕКУЩИЙ КОНТЕКСТ, ПРОИСХОЖДЕНИЕ */}
                {isLoadingDetail ? (
                  <div className="text-center py-12 bg-slate-50 rounded-xl border border-slate-200">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600 mx-auto"></div>
                    <p className="mt-2 text-xs text-slate-500">Загрузка единиц ZMU...</p>
                  </div>
                ) : filteredUnits.length === 0 ? (
                  <div className="text-center py-12 bg-slate-50 rounded-xl border border-slate-200">
                    <Package className="w-8 h-8 text-slate-300 mx-auto mb-1" />
                    <p className="text-xs text-slate-500">
                      {unitSearch ? 'По данному запросу единицы не найдены.' : 'В выбранной категории нет единиц.'}
                    </p>
                  </div>
                ) : (
                  <div className="w-full min-w-0 border border-slate-200 rounded-xl overflow-x-auto shadow-2xs">
                    <table className="min-w-full divide-y divide-slate-200 text-xs">
                      <thead className="bg-slate-50">
                        <tr>
                          <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                            ZMU
                          </th>
                          <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                            Физическое состояние
                          </th>
                          <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                            Доступность
                          </th>
                          <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                            Текущий контекст
                          </th>
                          <th className="px-3 py-2.5 text-left font-semibold text-slate-600 uppercase tracking-wider">
                            Происхождение
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100 bg-white">
                        {filteredUnits.map((u) => {
                          const isHighlighted = highlightUnitCode && u.unitCode === highlightUnitCode;

                          return (
                            <tr
                              key={u.id}
                              ref={
                                isHighlighted
                                  ? (el) => {
                                      highlightedRowRef.current = el;
                                    }
                                  : undefined
                              }
                              className={`transition-colors hover:bg-slate-50/90 cursor-pointer ${
                                isHighlighted ? 'bg-indigo-50/60 ring-2 ring-indigo-500/80' : ''
                              }`}
                              onClick={() => setSelectedZmuCode(u.unitCode)}
                            >
                              {/* 1. ZMU */}
                              <td className="px-3 py-2.5 whitespace-nowrap font-mono">
                                <div className="flex items-center gap-1.5">
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      setSelectedZmuCode(u.unitCode);
                                    }}
                                    className="font-semibold text-indigo-600 hover:text-indigo-800 hover:underline text-left cursor-pointer"
                                    title="Открыть историю единицы"
                                  >
                                    {u.unitCode}
                                  </button>
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handleCopy(u.unitCode);
                                    }}
                                    className="p-1 text-slate-400 hover:text-slate-700 rounded transition-colors cursor-pointer"
                                    title="Скопировать ZMU"
                                  >
                                    {copiedUnitCode === u.unitCode ? (
                                      <Check className="w-3.5 h-3.5 text-emerald-600" />
                                    ) : (
                                      <Copy className="w-3.5 h-3.5" />
                                    )}
                                  </button>
                                </div>
                                {u.createdAt && (
                                  <div className="text-[10px] text-slate-400 font-sans mt-0.5">
                                    {formatDateTime(u.createdAt)}
                                  </div>
                                )}
                              </td>

                              {/* 2. ФИЗИЧЕСКОЕ СОСТОЯНИЕ */}
                              <td className="px-3 py-2.5 whitespace-nowrap">
                                {getPhysicalStatusBadge(u.status)}
                              </td>

                              {/* 3. ДОСТУПНОСТЬ */}
                              <td className="px-3 py-2.5 whitespace-nowrap">
                                {getAvailabilityContent(u)}
                              </td>

                              {/* 4. ТЕКУЩИЙ КОНТЕКСТ (Order + Picking combined) */}
                              <td className="px-3 py-2.5">
                                {u.liveAllocation ? (
                                  <div>
                                    <Link
                                      to={`/orders/${u.liveAllocation.orderId}`}
                                      onClick={(e) => e.stopPropagation()}
                                      className="font-mono font-semibold text-indigo-600 hover:text-indigo-800 hover:underline inline-flex items-center gap-1"
                                      title="Открыть заказ"
                                    >
                                      <span>{u.liveAllocation.orderNumber}</span>
                                      <ExternalLink className="w-3 h-3 text-indigo-400" />
                                    </Link>
                                    <div className="text-[11px] text-slate-600 mt-0.5">
                                      <span>{formatRussianOrderStatus(u.liveAllocation.orderStatus)}</span>
                                      {' · '}
                                      <span className={u.liveAllocation.pickedAt ? 'text-purple-700 font-medium' : 'text-slate-500'}>
                                        {u.liveAllocation.pickedAt ? 'Собрана' : 'Ожидает сборки'}
                                      </span>
                                    </div>
                                  </div>
                                ) : u.staleAllocation ? (
                                  <div>
                                    <div className="text-slate-400 text-xs">Текущий заказ: —</div>
                                    <div className="text-[11px] text-amber-700 mt-0.5">
                                      Историческое назначение:<br />
                                      <Link
                                        to={`/orders/${u.staleAllocation.orderId}`}
                                        onClick={(e) => e.stopPropagation()}
                                        className="font-mono underline hover:text-amber-900 font-semibold"
                                      >
                                        {u.staleAllocation.orderNumber}
                                      </Link>
                                      {' · '}
                                      <span>{formatRussianOrderStatus(u.staleAllocation.orderStatus)}</span>
                                    </div>
                                  </div>
                                ) : (
                                  <span className="text-slate-300">—</span>
                                )}
                              </td>

                              {/* 5. ПРОИСХОЖДЕНИЕ (Supply + Receiving Lineage) */}
                              <td className="px-3 py-2.5">
                                {u.supplyLineage ? (
                                  <div>
                                    <div className="font-mono font-medium text-slate-800 flex items-center gap-1">
                                      <span>{u.supplyLineage.supplyNumber}</span>
                                      <Link
                                        to={`/supply-receiving?id=${u.supplyLineage.supplyId}`}
                                        onClick={(e) => e.stopPropagation()}
                                        className="text-indigo-600 hover:text-indigo-800 inline-flex items-center"
                                        title="Открыть приёмку поставки"
                                      >
                                        <ExternalLink className="w-3 h-3 text-indigo-400" />
                                      </Link>
                                    </div>
                                    <div className="text-[11px] text-slate-500 mt-0.5">
                                      {u.supplyLineage.receivedAt ? (
                                        <>
                                          Принято:<br />
                                          {formatDateDot(u.supplyLineage.receivedAt)}
                                        </>
                                      ) : (
                                        <span className="text-slate-400">Ожидает приёмки</span>
                                      )}
                                    </div>
                                  </div>
                                ) : (
                                  <span className="text-xs text-slate-400 italic">Нет данных</span>
                                )}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </div>
    </div>

    {/* Nested ZMU Traceability Drawer */}
    <ZmuTraceabilityDrawer
      unitCode={selectedZmuCode}
      isOpen={!!selectedZmuCode}
      onClose={() => setSelectedZmuCode(null)}
    />

    {/* Start Reconciliation Confirmation Modal */}
    {isStartConfirmOpen && (
      <div className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-xs">
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="start-reconciliation-title"
          className="bg-white rounded-xl shadow-2xl border border-slate-200 w-full max-w-md p-6 space-y-4"
        >
          <div>
            <h3 id="start-reconciliation-title" className="text-base font-bold text-slate-900">
              Начать инвентаризацию?
            </h3>
            <p className="text-xs text-slate-500 mt-1">
              {activeItem.productTitle} · {activeItem.size || '-'} · {activeItem.color || '-'}
            </p>
          </div>

          <div className="bg-slate-50 border border-slate-200 rounded-lg p-3.5 space-y-2 text-xs">
            <div className="flex justify-between items-center">
              <span className="text-slate-600">Физических ZMU на складе:</span>
              <span className="font-bold text-slate-900 font-mono text-sm">{phys.warehouse}</span>
            </div>

            {activeItem.accountingMode === 'mixed' && (
              <div className="pt-2 border-t border-slate-200/80 text-amber-800 text-[11px] leading-relaxed">
                <p className="font-medium">Проверяются только физические единицы ZMU.</p>
                <p className="text-amber-700 mt-0.5">
                  Остаток без ZMU: <span className="font-bold font-mono">{leg.onHand} шт.</span> в эту проверку не входит.
                </p>
              </div>
            )}
          </div>

          {reconcileError && (
            <div className="p-2.5 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-700 font-medium">
              {reconcileError}
            </div>
          )}

          <div className="flex items-center justify-end gap-2.5 pt-2">
            <button
              type="button"
              onClick={() => {
                setIsStartConfirmOpen(false);
                setReconcileError(null);
              }}
              disabled={isStartingReconciliation}
              className="px-3.5 py-1.5 border border-slate-300 rounded-lg text-xs font-semibold text-slate-700 hover:bg-slate-50 transition-colors cursor-pointer disabled:opacity-50"
            >
              Отмена
            </button>
            <button
              type="button"
              onClick={handleConfirmStart}
              disabled={isStartingReconciliation}
              className="px-4 py-1.5 bg-slate-900 hover:bg-slate-800 text-white rounded-lg text-xs font-semibold transition-colors cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
            >
              {isStartingReconciliation ? (
                <>
                  <div className="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  <span>Создание...</span>
                </>
              ) : (
                <span>Начать инвентаризацию</span>
              )}
            </button>
          </div>
        </div>
      </div>
    )}
  </>
);
}

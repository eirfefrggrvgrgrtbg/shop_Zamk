import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  AlertCircle,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  Filter,
  Info,
  PackageSearch,
  Search,
  X,
} from 'lucide-react';
import { HelpTooltip } from '../components/HelpTooltip';
import { SellerContextBanner } from '../components/SellerContextBanner';
import {
  getAdminInventory,
  getAdminInventoryErrorMessage,
  type PhysicalUnitContext,
} from '../api/adminInventory';
import type { AdminInventoryView } from '../api/adminInventory';
import { VariantInventoryDrawer } from '../components/inventory/VariantInventoryDrawer';

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

export function AdminInventory() {
  const [inventory, setInventory] = useState<AdminInventoryView[]>([]);
  const [unitContext, setUnitContext] = useState<PhysicalUnitContext | null>(null);
  const [issuesCount, setIssuesCount] = useState<number>(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [searchParams] = useSearchParams();
  const urlSellerId = searchParams.get('sellerId');
  const urlQ = searchParams.get('q');

  const [searchQuery, setSearchQuery] = useState(urlQ || '');
  const [sourceFilter, setSourceFilter] = useState('');
  const [sellerFilter, setSellerFilter] = useState(urlSellerId || '');
  const [accountingModeFilter, setAccountingModeFilter] = useState('');
  const [stockStatusFilter, setStockStatusFilter] = useState('');
  const [lowStockFilter, setLowStockFilter] = useState(false);
  const [pagination, setPagination] = useState({ limit: 50, offset: 0, total: 0 });

  useEffect(() => {
    if (urlSellerId) {
      setSellerFilter(urlSellerId);
    }
  }, [urlSellerId]);

  useEffect(() => {
    if (urlQ !== null) {
      setSearchQuery(urlQ);
    }
  }, [urlQ]);

  const [activeIssuePopoverId, setActiveIssuePopoverId] = useState<string | null>(null);
  const [drawerItem, setDrawerItem] = useState<AdminInventoryView | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [highlightUnitCode, setHighlightUnitCode] = useState<string | null>(null);

  const handleOpenDetail = (item: AdminInventoryView, unitCodeToHighlight?: string) => {
    setDrawerItem(item);
    setHighlightUnitCode(unitCodeToHighlight || null);
    setIsDrawerOpen(true);
  };

  useEffect(() => {
    const handleClickOutside = () => setActiveIssuePopoverId(null);
    window.addEventListener('click', handleClickOutside);
    return () => window.removeEventListener('click', handleClickOutside);
  }, []);

  const fetchInventory = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const resp = await getAdminInventory({
        q: searchQuery,
        source: sourceFilter,
        sellerId: sellerFilter,
        accountingMode: accountingModeFilter,
        stockStatus: stockStatusFilter,
        lowStock: lowStockFilter,
        limit: pagination.limit,
        offset: pagination.offset,
      });
      setInventory(resp.items);
      setPagination((p) => ({ ...p, total: resp.totalCount }));
      setIssuesCount(resp.issuesCount || 0);
      setUnitContext(resp.unitContext ?? null);
      if (drawerItem) {
        const refreshed = resp.items.find((item) => item.id === drawerItem.id) ?? null;
        if (refreshed) {
          setDrawerItem(refreshed);
        }
      }
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Не удалось загрузить остатки.'));
      setUnitContext(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchInventory();
  }, [
    searchQuery,
    sourceFilter,
    sellerFilter,
    accountingModeFilter,
    stockStatusFilter,
    lowStockFilter,
    pagination.limit,
    pagination.offset,
  ]);

  const getAccountingBadge = (mode: string) => {
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

  return (
    <div className="space-y-6">
      <SellerContextBanner />

      {/* Header */}
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Остатки / Склад</h1>
          <p className="mt-1 text-sm text-slate-500">Физический и доступный остаток товаров на складе ZAMK</p>
        </div>
        <div className="mt-4 sm:mt-0">
          <Link
            to="/warehouse/free-scan"
            className="inline-flex items-center justify-center px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-lg shadow-sm transition-colors whitespace-nowrap"
          >
            <PackageSearch className="w-4 h-4 mr-2" />
            Свободный сканер ZMU
          </Link>
        </div>
      </div>

      {/* Attention Strip - only rendered when discrepancies exist */}
      {issuesCount > 0 && (
        <div className="p-4 bg-amber-50 border border-amber-200 rounded-xl flex items-center justify-between gap-4 text-amber-900 shadow-sm">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-amber-100 text-amber-700 flex items-center justify-center flex-shrink-0">
              <AlertTriangle className="w-5 h-5" />
            </div>
            <div>
              <span className="font-semibold text-sm">
                Обнаружено {issuesCount} {issuesCount === 1 ? 'вариант' : issuesCount < 5 ? 'варианта' : 'вариантов'} с расхождениями учёта
              </span>
              <p className="text-xs text-amber-700 mt-0.5">
                Физические ZMU или резервы не сходятся с коммерческой проекцией.
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => {
              setStockStatusFilter('has_issue');
              setAccountingModeFilter('');
            }}
            className="px-3.5 py-1.5 bg-amber-600 hover:bg-amber-700 text-white text-xs font-semibold rounded-lg shadow-xs transition-colors whitespace-nowrap"
          >
            Показать
          </button>
        </div>
      )}

      {/* Search & Filter Bar */}
      <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-sm space-y-3">
        {/* Unified Search */}
        <div className="relative rounded-lg shadow-2xs">
          <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none">
            <Search className="h-4 w-4 text-slate-400" />
          </div>
          <input
            type="text"
            className="block w-full pl-10 pr-4 py-2 text-sm border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 transition-colors placeholder:text-slate-400"
            placeholder="Товар, SKU, ZMK или ZMU..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        {/* Filter Bar Controls */}
        <div className="flex flex-wrap items-center gap-2.5 pt-1">
          <div className="flex items-center gap-1.5 text-xs text-slate-500 font-medium mr-0.5">
            <Filter className="h-3.5 w-3.5 text-slate-400" />
            <span>Фильтры:</span>
          </div>

          {/* Accounting Mode Filter */}
          <div className="min-w-[150px] flex-1 sm:flex-none">
            <select
              value={accountingModeFilter}
              onChange={(e) => setAccountingModeFilter(e.target.value)}
              className="w-full px-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 font-medium focus:bg-white focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors shadow-2xs cursor-pointer"
            >
              <option value="">Все режимы учёта</option>
              <option value="serialized">Серийный</option>
              <option value="mixed">Смешанный</option>
              <option value="legacy">Без ZMU</option>
            </select>
          </div>

          {/* Stock Status Filter */}
          <div className="min-w-[170px] flex-1 sm:flex-none">
            <select
              value={stockStatusFilter}
              onChange={(e) => setStockStatusFilter(e.target.value)}
              className="w-full px-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 font-medium focus:bg-white focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors shadow-2xs cursor-pointer"
            >
              <option value="">Все остатки</option>
              <option value="available">Есть доступный остаток</option>
              <option value="out_of_stock">Нет доступного остатка</option>
              <option value="has_reserved">Есть резерв</option>
              <option value="has_inbound">Есть входящий товар</option>
              <option value="has_issue">С расхождениями</option>
            </select>
          </div>

          {/* Source Filter */}
          <div className="min-w-[140px] flex-1 sm:flex-none">
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value)}
              className="w-full px-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 font-medium focus:bg-white focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors shadow-2xs cursor-pointer"
            >
              <option value="">Все источники</option>
              <option value="auction_direct_sale">ZAMK (Свой склад)</option>
              <option value="seller">Продавцы</option>
            </select>
          </div>

          {/* Seller Filter */}
          <div className="min-w-[150px] flex-1 sm:flex-none">
            <input
              type="text"
              className="w-full px-3 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg text-slate-700 font-medium placeholder:text-slate-400 placeholder:font-normal focus:bg-white focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-colors shadow-2xs"
              placeholder="Продавец (ID / Имя)..."
              value={sellerFilter}
              onChange={(e) => setSellerFilter(e.target.value)}
            />
          </div>

          {/* Low Stock Toggle Pill */}
          <div>
            <label
              htmlFor="low-stock"
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition-colors cursor-pointer select-none shadow-2xs ${
                lowStockFilter
                  ? 'bg-amber-50 border-amber-300 text-amber-800'
                  : 'bg-slate-50 border-slate-200 text-slate-600 hover:bg-slate-100 hover:text-slate-800'
              }`}
            >
              <input
                id="low-stock"
                name="low-stock"
                type="checkbox"
                checked={lowStockFilter}
                onChange={(e) => setLowStockFilter(e.target.checked)}
                className="h-3.5 w-3.5 text-indigo-600 focus:ring-indigo-500 border-slate-300 rounded cursor-pointer"
              />
              <span>Меньше 10 шт.</span>
            </label>
          </div>

          {/* Reset Filters Affordance */}
          {(accountingModeFilter || stockStatusFilter || sourceFilter || sellerFilter || lowStockFilter) && (
            <button
              type="button"
              onClick={() => {
                setAccountingModeFilter('');
                setStockStatusFilter('');
                setSourceFilter('');
                setSellerFilter('');
                setLowStockFilter(false);
              }}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50 rounded-lg font-medium transition-colors cursor-pointer"
            >
              <X className="w-3.5 h-3.5" />
              <span>Сбросить</span>
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-xl flex items-center border border-red-200">
          <AlertCircle className="h-5 w-5 mr-2 flex-shrink-0" />
          <span className="text-sm">{error}</span>
        </div>
      )}

      {/* Physical ZMU Context Card */}
      {unitContext && (
        <div
          data-testid="admin-inventory-zmu-context"
          className="p-4 bg-indigo-50/90 border border-indigo-200 rounded-xl flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 shadow-xs"
        >
          <div className="flex items-center space-x-3">
            <div className="p-2 rounded-lg bg-indigo-100 text-indigo-700 flex-shrink-0">
              <Boxes className="w-5 h-5" />
            </div>
            <div>
              <div className="font-semibold text-gray-900 flex items-center space-x-2">
                <span>{unitContext.unitCode}</span>
                <span className="text-gray-300">·</span>
                <span>{unitContext.productTitle}</span>
              </div>
              <div className="text-xs text-gray-500 mt-0.5">
                {unitContext.variant ? `${unitContext.variant} · ` : ''}
                Физическая единица товара
              </div>
            </div>
          </div>
          <div className="flex items-center space-x-3">
            <span className="text-xs text-gray-500 font-medium">Статус:</span>
            <span className="px-2.5 py-1 text-xs font-semibold rounded-full bg-white text-indigo-800 border border-indigo-200 shadow-2xs">
              {unitContext.statusLabel || unitContext.status}
            </span>
            <button
              type="button"
              onClick={() => {
                const target =
                  inventory.find(
                    (i) => i.productVariantId === unitContext.variantId || i.productId === unitContext.productId
                  ) || inventory[0];
                if (target) {
                  handleOpenDetail(target, unitContext.unitCode);
                }
              }}
              className="px-3.5 py-1.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold rounded-lg shadow-xs transition-colors cursor-pointer"
            >
              Открыть единицу
            </button>
          </div>
        </div>
      )}

      {/* Main Table */}
      {isLoading ? (
        <div className="text-center py-16 bg-white rounded-xl border border-slate-200 shadow-sm">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-3 text-sm text-slate-500 font-medium">Загрузка складских остатков...</p>
        </div>
      ) : inventory.length === 0 ? (
        <div className="text-center py-16 bg-white rounded-xl border border-slate-200 shadow-sm">
          <Boxes className="mx-auto h-12 w-12 text-slate-300" />
          <h3 className="mt-3 text-sm font-semibold text-slate-900">
            {unitContext ? 'Нет агрегированных остатков' : 'Товары не найдены'}
          </h3>
          <p className="mt-1 text-xs text-slate-500 max-w-sm mx-auto">
            {unitContext
              ? 'Физическая единица найдена, но агрегированные складские остатки отсутствуют.'
              : 'По заданным фильтрам и поисковому запросу ничего не найдено.'}
          </p>
        </div>
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200">
              <thead className="bg-slate-50">
                <tr>
                  <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-slate-600 tracking-wider">
                    Товар / Вариант
                  </th>
                  <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-slate-600 tracking-wider">
                    Продавец
                  </th>
                  <th scope="col" className="px-3 py-3 text-center text-xs font-semibold text-slate-600 tracking-wider">
                    Учёт
                  </th>
                  <th scope="col" className="px-3 py-3 text-right text-xs font-semibold text-slate-600 tracking-wider">
                    <span className="inline-flex items-center gap-1 justify-end">
                      Всего
                      <HelpTooltip content="Коммерческий остаток на складе. Текущий источник: inventory_items.total_stock." />
                    </span>
                  </th>
                  <th scope="col" className="px-3 py-3 text-right text-xs font-semibold text-slate-600 tracking-wider">
                    <span className="inline-flex items-center gap-1 justify-end">
                      В резерве
                      <HelpTooltip content="Количество товара, удерживаемое активными резервами заказов." />
                    </span>
                  </th>
                  <th scope="col" className="px-3 py-3 text-right text-xs font-semibold text-slate-600 tracking-wider">
                    <span className="inline-flex items-center gap-1 justify-end">
                      Доступно
                      <HelpTooltip content="Доступно для новых заказов: Всего − В резерве." />
                    </span>
                  </th>
                  <th scope="col" className="px-4 py-3 text-left text-xs font-semibold text-slate-600 tracking-wider">
                    <span className="inline-flex items-center gap-1">
                      Физические ZMU
                      <HelpTooltip content="Физические единицы с уникальным ZMU. Reserved/Picked не являются статусами ZMU." />
                    </span>
                  </th>
                  <th scope="col" className="px-3 py-3 text-center text-xs font-semibold text-slate-600 tracking-wider">
                    <span className="inline-flex items-center gap-1 justify-center">
                      В пути
                      <HelpTooltip content="Физические ZMU, ожидаемые по поставкам. Не входят в доступный остаток." />
                    </span>
                  </th>
                  <th scope="col" className="px-3 py-3 text-center text-xs font-semibold text-slate-600 tracking-wider">
                    Состояние
                  </th>
                  <th scope="col" className="relative px-4 py-3">
                    <span className="sr-only">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-slate-100 text-sm">
                {inventory.map((item) => {
                  const isLegacyPure = item.accountingMode === 'legacy' || (item.physical.warehouse === 0 && item.aggregate.total > 0);
                  const isWarning = item.health.status !== 'healthy';

                  return (
                    <tr key={item.id} className="hover:bg-slate-50/70 transition-colors">
                      {/* 1. Товар / Вариант */}
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3">
                          {item.mainImageUrl ? (
                            <img
                              src={item.mainImageUrl}
                              alt={item.productTitle}
                              className="w-10 h-10 rounded-lg object-cover border border-slate-200 flex-shrink-0"
                            />
                          ) : (
                            <div className="w-10 h-10 rounded-lg bg-slate-100 border border-slate-200 flex items-center justify-center flex-shrink-0 text-slate-400">
                              <Boxes className="w-5 h-5" />
                            </div>
                          )}
                          <div className="min-w-0">
                            <div className="font-medium text-slate-900 truncate max-w-xs" title={item.productTitle}>
                              {item.productTitle}
                            </div>
                            <div className="text-xs text-slate-500 mt-0.5">
                              {item.size || item.color ? `${item.size || '-'} · ${item.color || '-'}` : item.variant}
                            </div>
                            <div className="text-[11px] text-slate-400 font-mono mt-0.5 flex items-center gap-2">
                              {item.sku && <span>SKU: {item.sku}</span>}
                              {item.barcode && <span>ZMK: {item.barcode}</span>}
                            </div>
                          </div>
                        </div>
                      </td>

                      {/* 2. Продавец */}
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="flex items-center space-x-1.5">
                          {item.source === 'auction_direct_sale' ? (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">
                              ZAMK
                            </span>
                          ) : (
                            <span className="text-xs font-medium text-slate-700 truncate max-w-[140px]" title={item.sellerName}>
                              {item.sellerName || '-'}
                            </span>
                          )}
                        </div>
                      </td>

                      {/* 3. Учёт */}
                      <td className="px-3 py-3 whitespace-nowrap text-center">
                        {getAccountingBadge(item.accountingMode)}
                      </td>

                      {/* 4. Всего */}
                      <td className="px-3 py-3 whitespace-nowrap text-right font-medium text-slate-900">
                        {item.aggregate.total}
                      </td>

                      {/* 5. В резерве */}
                      <td className="px-3 py-3 whitespace-nowrap text-right text-slate-600">
                        {item.aggregate.reserved > 0 ? (
                          <span className="font-medium text-amber-700 bg-amber-50 px-2 py-0.5 rounded-md border border-amber-200">
                            {item.aggregate.reserved}
                          </span>
                        ) : (
                          '0'
                        )}
                      </td>

                      {/* 6. Доступно */}
                      <td className="px-3 py-3 whitespace-nowrap text-right">
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                            item.aggregate.available > 10
                              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                              : item.aggregate.available > 0
                              ? 'bg-amber-50 text-amber-700 border border-amber-200'
                              : 'bg-slate-100 text-slate-600 border border-slate-200'
                          }`}
                        >
                          {item.aggregate.available}
                        </span>
                      </td>

                      {/* 7. Физические ZMU */}
                      <td className="px-4 py-3 whitespace-nowrap">
                        {isLegacyPure ? (
                          <div>
                            <span className="text-xs text-slate-400 italic">Нет ZMU</span>
                            <div className="text-[11px] text-slate-400">Legacy учёт</div>
                          </div>
                        ) : (
                          <div>
                            <div className="text-xs font-medium text-slate-900">
                              На складе: <span className="font-semibold">{item.physical.warehouse}</span>
                            </div>
                            <div className="text-[11px] text-slate-500 mt-0.5">
                              <span className="text-emerald-600 font-medium">{item.physical.free} свободно</span>
                              <span className="text-slate-300 mx-1">·</span>
                              <span className="text-amber-600 font-medium">{item.physical.allocated} занято</span>
                            </div>
                          </div>
                        )}
                      </td>

                      {/* 8. В пути */}
                      <td className="px-3 py-3 whitespace-nowrap text-center">
                        {item.physical.expected > 0 ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200">
                            +{item.physical.expected}
                          </span>
                        ) : (
                          <span className="text-slate-300">-</span>
                        )}
                      </td>

                      {/* 9. Состояние */}
                      <td className="px-3 py-3 whitespace-nowrap text-center relative">
                        {isWarning ? (
                          <div className="inline-block relative">
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                setActiveIssuePopoverId(activeIssuePopoverId === item.id ? null : item.id);
                              }}
                              className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200 hover:bg-rose-100 hover:border-rose-300 transition-colors cursor-pointer shadow-2xs"
                              title={item.health.issues.map((code) => ISSUE_LABELS[code] || code).join('; ')}
                            >
                              <AlertTriangle className="w-3.5 h-3.5 text-rose-600 flex-shrink-0" />
                              <span>Расхождение</span>
                              <Info className="w-3 h-3 text-rose-400 ml-0.5" />
                            </button>

                            {activeIssuePopoverId === item.id && (
                              <div
                                className="absolute right-0 top-full mt-1.5 w-80 bg-white rounded-xl shadow-xl border border-rose-200 p-3.5 z-30 text-left animate-in fade-in zoom-in-95 duration-100"
                                onClick={(e) => e.stopPropagation()}
                              >
                                <div className="flex items-start justify-between gap-2 pb-2 mb-2 border-b border-rose-100">
                                  <div className="flex items-center gap-1.5 font-semibold text-xs text-rose-900">
                                    <AlertTriangle className="w-4 h-4 text-rose-600 flex-shrink-0" />
                                    <span>Расхождение в учёте</span>
                                  </div>
                                  <button
                                    type="button"
                                    onClick={() => setActiveIssuePopoverId(null)}
                                    className="text-slate-400 hover:text-slate-600 text-xs p-0.5 rounded hover:bg-slate-100"
                                  >
                                    <X className="w-3.5 h-3.5" />
                                  </button>
                                </div>
                                <div className="space-y-1.5">
                                  {item.health.issues.map((code) => (
                                    <div key={code} className="text-xs text-slate-700 leading-snug flex items-start gap-1.5">
                                      <span className="text-rose-500 font-bold">•</span>
                                      <span>{ISSUE_LABELS[code] || code}</span>
                                    </div>
                                  ))}
                                </div>
                                <div className="mt-2.5 pt-2 border-t border-slate-100 text-[11px] text-slate-400 flex items-center gap-1">
                                  <Info className="w-3 h-3 text-slate-400 flex-shrink-0" />
                                  <span>Детализация по единицам и заказам будет доступна в карточке товара (P0.2).</span>
                                </div>
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-xs text-emerald-700 font-medium">
                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                            <span>В норме</span>
                          </span>
                        )}
                      </td>

                      {/* 10. Действия */}
                      <td className="px-4 py-3 whitespace-nowrap text-right">
                        <button
                          type="button"
                          onClick={() => handleOpenDetail(item)}
                          className="px-3 py-1 text-xs font-semibold text-indigo-600 hover:text-indigo-800 hover:bg-indigo-50 border border-indigo-200 rounded-lg transition-colors cursor-pointer shadow-2xs"
                        >
                          Открыть
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination summary */}
          <div className="bg-slate-50 px-4 py-3 border-t border-slate-200 flex items-center justify-between text-xs text-slate-500">
            <div>
              Всего позиций: <span className="font-semibold text-slate-700">{pagination.total}</span>
            </div>
            {pagination.total > pagination.limit && (
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled={pagination.offset === 0}
                  onClick={() => setPagination((p) => ({ ...p, offset: Math.max(0, p.offset - p.limit) }))}
                  className="px-2.5 py-1 rounded bg-white border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:opacity-40"
                >
                  Назад
                </button>
                <button
                  type="button"
                  disabled={pagination.offset + pagination.limit >= pagination.total}
                  onClick={() => setPagination((p) => ({ ...p, offset: p.offset + p.limit }))}
                  className="px-2.5 py-1 rounded bg-white border border-slate-300 text-slate-700 hover:bg-slate-50 disabled:opacity-40"
                >
                  Вперёд
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Variant Inventory Detail Drawer */}
      <VariantInventoryDrawer
        item={drawerItem}
        isOpen={isDrawerOpen}
        onClose={() => {
          setIsDrawerOpen(false);
          setHighlightUnitCode(null);
        }}
        highlightUnitCode={highlightUnitCode}
      />
    </div>
  );
}

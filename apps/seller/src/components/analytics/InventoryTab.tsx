import { useEffect, useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { getAnalyticsInventory, InventoryRow } from '../../api/selleranalytics';
import { PackageX, AlertTriangle, CheckCircle, Package, AlertCircle } from 'lucide-react';

interface InventoryTabProps {
  from: string;
  to: string;
}

export function formatVelocity(vel: number): string {
  if (!vel || isNaN(vel) || !isFinite(vel) || vel <= 0) return '—';
  if (vel < 0.01) return '< 0,01 шт/дн';
  if (vel < 1) {
    return `${vel.toFixed(2).replace('.', ',')} шт/дн`;
  }
  return `${vel.toFixed(1).replace('.', ',')} шт/дн`;
}

export function formatDaysOfStock(row: InventoryRow): string {
  if (row.available === 0) {
    return '—';
  }
  if (row.unitsSold === 0 || row.salesVelocity <= 0 || row.daysOfStock === null || row.daysOfStock === undefined) {
    return '—';
  }
  if (isNaN(row.daysOfStock) || !isFinite(row.daysOfStock)) {
    return '—';
  }
  if (row.daysOfStock < 1 && row.daysOfStock > 0) {
    return '< 1 дн.';
  }
  return `≈ ${Math.round(row.daysOfStock)} дн.`;
}

export function InventoryTab({ from, to }: InventoryTabProps) {
  const [data, setData] = useState<InventoryRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let ignore = false;
    async function fetchInventory() {
      setLoading(true);
      setError(null);
      try {
        const res = await getAnalyticsInventory(from, to);
        if (!ignore) {
          setData(res.items);
        }
      } catch (err: any) {
        if (!ignore) {
          setError(err);
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    }
    fetchInventory();
    return () => {
      ignore = true;
    };
  }, [from, to]);

  const summary = useMemo(() => {
    let outOfStock = 0;
    let lowStock = 0;
    let noSales = 0;
    let totalAvailable = 0;
    let totalInbound = 0;

    data.forEach(item => {
      totalAvailable += item.available;
      totalInbound += item.inbound;
      
      const isOut = item.available === 0;
      const isPhysLow = item.isPhysicalLowStock ?? (item.available > 0 && item.available <= 10);
      const isNoSales = item.hasNoSalesInPeriod ?? (item.unitsSold === 0);

      if (isOut) {
        outOfStock++;
      }
      if (isPhysLow) {
        lowStock++;
      }
      if (item.available > 0 && isNoSales) {
        noSales++;
      }
    });

    return { outOfStock, lowStock, noSales, totalAvailable, totalInbound };
  }, [data]);

  if (loading) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 animate-pulse">
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-8">
          {[1,2,3,4,5].map(i => <div key={i} className="h-24 bg-gray-200 rounded-xl"></div>)}
        </div>
        <div className="h-10 bg-gray-200 rounded mb-4 w-1/3"></div>
        {[1, 2, 3, 4].map(i => <div key={i} className="h-16 bg-gray-100 rounded mb-2"></div>)}
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8 text-center text-red-600 bg-red-50 rounded-xl">
        <p className="font-semibold">Не удалось загрузить данные по остаткам</p>
        <p className="text-sm mt-1">{error.message}</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        <div className="bg-red-50 rounded-xl p-5 border border-red-100" title="Товары с нулевым доступным остатком на складе">
          <h3 className="text-sm font-medium text-red-800 mb-1">Нет в наличии</h3>
          <p className="text-2xl font-bold text-red-900">{summary.outOfStock}</p>
          <p className="text-xs text-red-700 mt-1">Остаток 0 шт.</p>
        </div>
        <div className="bg-amber-50 rounded-xl p-5 border border-amber-100" title="Товары с физическим доступным остатком от 1 до 10 шт. на складе">
          <h3 className="text-sm font-medium text-amber-800 mb-1">Заканчивается</h3>
          <p className="text-2xl font-bold text-amber-900">{summary.lowStock}</p>
          <p className="text-xs text-amber-700 mt-1">Остаток 1–10 шт.</p>
        </div>
        <div className="bg-gray-50 rounded-xl p-5 border border-gray-200" title="Товары с доступным остатком, по которым не было доставленных продаж за период">
          <h3 className="text-sm font-medium text-gray-600 mb-1">Без продаж</h3>
          <p className="text-2xl font-bold text-gray-900">{summary.noSales}</p>
          <p className="text-xs text-gray-500 mt-1">0 продаж за период</p>
        </div>
        <div className="bg-white rounded-xl p-5 border border-gray-200" title="Суммарный доступный для продажи остаток на складе">
          <h3 className="text-sm font-medium text-gray-500 mb-1">Всего доступно</h3>
          <p className="text-2xl font-bold text-gray-900">{summary.totalAvailable}</p>
          <p className="text-xs text-gray-400 mt-1">Готово к заказу</p>
        </div>
        <div className="bg-white rounded-xl p-5 border border-gray-200" title="Ожидаемые поставки в пути и на приёмке">
          <h3 className="text-sm font-medium text-gray-500 mb-1">В поставках</h3>
          <p className="text-2xl font-bold text-blue-600">+{summary.totalInbound}</p>
          <p className="text-xs text-gray-400 mt-1">Ожидается приёмка</p>
        </div>
      </div>

      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        {data.length === 0 ? (
          <div className="p-12 text-center text-gray-500">
            Нет данных об остатках для выбранных товаров.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-gray-50 text-gray-600 font-medium border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3">Товар / Вариант</th>
                  <th className="px-4 py-3 text-right" title="Состояние запаса и продаж за выбранный период">Состояние</th>
                  <th className="px-4 py-3 text-right">Доступно</th>
                  <th className="px-4 py-3 text-right">На складе</th>
                  <th className="px-4 py-3 text-right">В резерве</th>
                  <th className="px-4 py-3 text-right">В поставках</th>
                  <th className="px-4 py-3 text-right">Продано</th>
                  <th className="px-4 py-3 text-right">Скорость</th>
                  <th className="px-4 py-3 text-right" title="Расчетное количество дней до исчерпания остатка при текущей скорости продаж">Запаса на</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {data.map(row => {
                  const isPhysLow = row.isPhysicalLowStock ?? (row.available > 0 && row.available <= 10);
                  const isNoSales = row.hasNoSalesInPeriod ?? (row.unitsSold === 0);
                  const isForecastRisk = row.isForecastLowCoverage ?? (row.daysOfStock !== null && row.daysOfStock !== undefined && row.daysOfStock > 0 && row.daysOfStock <= 7);

                  return (
                    <tr key={row.variantId} className="hover:bg-gray-50 transition-colors">
                      <td className="px-4 py-3">
                        <div className="flex flex-col">
                          <Link to={`/analytics/products/${row.productId}`} className="font-medium text-blue-600 hover:underline line-clamp-1">
                            {row.productTitle || row.displayName || row.sku || 'Без названия'}
                          </Link>
                          <div className="flex items-center gap-2 text-xs text-gray-500 mt-0.5">
                            {row.displayName && <span>{row.displayName}</span>}
                            {row.sku && (
                              <span className="font-mono text-gray-400">
                                {row.sku}
                              </span>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-right">
                        {row.available === 0 ? (
                          <span className="inline-flex items-center text-xs font-medium text-red-700 bg-red-100 px-2 py-0.5 rounded">
                            <PackageX className="w-3 h-3 mr-1" /> Нет в наличии
                          </span>
                        ) : (
                          <div className="flex flex-wrap items-center justify-end gap-1.5">
                            {isPhysLow && (
                              <span className="inline-flex items-center text-xs font-medium text-amber-700 bg-amber-100 px-2 py-0.5 rounded" title="Физический остаток 1–10 шт.">
                                <AlertTriangle className="w-3 h-3 mr-1" /> Мало на складе
                              </span>
                            )}
                            {isNoSales && (
                              <span className="inline-flex items-center text-xs font-medium text-gray-600 bg-gray-100 px-2 py-0.5 rounded" title="Нет доставленных продаж за период">
                                <Package className="w-3 h-3 mr-1" /> Нет продаж за период
                              </span>
                            )}
                            {isForecastRisk && (
                              <span className="inline-flex items-center text-xs font-medium text-rose-700 bg-rose-100 px-2 py-0.5 rounded" title="Прогноз исчерпания остатка ≤ 7 дней">
                                <AlertCircle className="w-3 h-3 mr-1" /> Запас &lt; 7 дн.
                              </span>
                            )}
                            {!isPhysLow && !isNoSales && !isForecastRisk && (
                              <span className="inline-flex items-center text-xs font-medium text-emerald-700 bg-emerald-100 px-2 py-0.5 rounded">
                                <CheckCircle className="w-3 h-3 mr-1" /> Нормальный запас
                              </span>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3 text-right font-semibold text-gray-900">
                        {row.available} шт.
                      </td>
                      <td className="px-4 py-3 text-right text-gray-500">
                        {row.onHand} шт.
                      </td>
                      <td className="px-4 py-3 text-right text-gray-500">
                        {row.reserved > 0 ? `${row.reserved} шт.` : '—'}
                      </td>
                      <td className="px-4 py-3 text-right text-blue-600 font-medium">
                        {row.inbound > 0 ? `+${row.inbound} шт.` : '—'}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-900 font-medium">
                        {row.unitsSold}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-600">
                        {formatVelocity(row.salesVelocity)}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-900 font-medium">
                        {formatDaysOfStock(row)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

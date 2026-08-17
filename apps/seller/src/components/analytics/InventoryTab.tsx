import { useEffect, useState, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { getAnalyticsInventory, InventoryRow } from '../../api/selleranalytics';
import { PackageX, AlertTriangle, CheckCircle, Package } from 'lucide-react';

interface InventoryTabProps {
  from: string;
  to: string;
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
      
      if (item.stockCoverageState === 'out_of_stock') {
        outOfStock++;
      } else if (item.stockCoverageState === 'no_sales') {
        noSales++;
      } else if (item.daysOfStock && item.daysOfStock <= 7) {
        lowStock++;
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
        <div className="bg-red-50 rounded-xl p-5 border border-red-100">
          <h3 className="text-sm font-medium text-red-800 mb-1">Нет в наличии</h3>
          <p className="text-2xl font-bold text-red-900">{summary.outOfStock}</p>
        </div>
        <div className="bg-amber-50 rounded-xl p-5 border border-amber-100">
          <h3 className="text-sm font-medium text-amber-800 mb-1">Заканчивается</h3>
          <p className="text-2xl font-bold text-amber-900">{summary.lowStock}</p>
        </div>
        <div className="bg-gray-50 rounded-xl p-5 border border-gray-200">
          <h3 className="text-sm font-medium text-gray-600 mb-1">Без продаж</h3>
          <p className="text-2xl font-bold text-gray-900">{summary.noSales}</p>
        </div>
        <div className="bg-white rounded-xl p-5 border border-gray-200">
          <h3 className="text-sm font-medium text-gray-500 mb-1">Всего доступно</h3>
          <p className="text-2xl font-bold text-gray-900">{summary.totalAvailable}</p>
        </div>
        <div className="bg-white rounded-xl p-5 border border-gray-200">
          <h3 className="text-sm font-medium text-gray-500 mb-1">В поставках</h3>
          <p className="text-2xl font-bold text-blue-600">+{summary.totalInbound}</p>
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
                  <th className="px-4 py-3 text-right">Состояние</th>
                  <th className="px-4 py-3 text-right">Доступно</th>
                  <th className="px-4 py-3 text-right">На складе</th>
                  <th className="px-4 py-3 text-right">В резерве</th>
                  <th className="px-4 py-3 text-right">В поставках</th>
                  <th className="px-4 py-3 text-right">Продано</th>
                  <th className="px-4 py-3 text-right">Скорость</th>
                  <th className="px-4 py-3 text-right">Запаса на</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {data.map(row => (
                  <tr key={row.variantId} className="hover:bg-gray-50 transition-colors">
                    <td className="px-4 py-3">
                      <Link to={`/analytics/products/${row.productId}`} className="font-medium text-blue-600 hover:underline">
                        {row.sku}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-right">
                      {row.stockCoverageState === 'out_of_stock' && (
                        <span className="inline-flex items-center text-xs font-medium text-red-700 bg-red-100 px-2 py-0.5 rounded">
                          <PackageX className="w-3 h-3 mr-1" /> Нет в наличии
                        </span>
                      )}
                      {row.stockCoverageState === 'no_sales' && (
                        <span className="inline-flex items-center text-xs font-medium text-gray-600 bg-gray-100 px-2 py-0.5 rounded">
                          <Package className="w-3 h-3 mr-1" /> Нет продаж
                        </span>
                      )}
                      {row.stockCoverageState === '' && row.daysOfStock && row.daysOfStock <= 7 && (
                        <span className="inline-flex items-center text-xs font-medium text-amber-700 bg-amber-100 px-2 py-0.5 rounded">
                          <AlertTriangle className="w-3 h-3 mr-1" /> Заканчивается
                        </span>
                      )}
                      {row.stockCoverageState === '' && row.daysOfStock && row.daysOfStock > 7 && (
                        <span className="inline-flex items-center text-xs font-medium text-emerald-700 bg-emerald-100 px-2 py-0.5 rounded">
                          <CheckCircle className="w-3 h-3 mr-1" /> Нормальный запас
                        </span>
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
                      {row.salesVelocity > 0 ? `${row.salesVelocity.toFixed(1)} шт/дн` : '—'}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-900 font-medium">
                      {row.stockCoverageState === 'out_of_stock' || row.stockCoverageState === 'no_sales' 
                        ? '—' 
                        : `≈ ${row.daysOfStock?.toFixed(0)} дн.`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

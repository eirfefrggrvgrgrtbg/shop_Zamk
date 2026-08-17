import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowDownIcon, ArrowUpIcon, MinusIcon } from 'lucide-react';
import { getAnalyticsProducts, ProductRow } from '../../api/selleranalytics';
import { formatCents, cn } from '../../lib/utils';

interface ProductsTabProps {
  from: string;
  to: string;
}

type SortField = 'grossSalesCents' | 'unitsSold' | 'ordersCount' | 'grossSalesChangePercent' | 'returnRatePercent' | 'availableStock';
type SortOrder = 'asc' | 'desc';

export function ProductsTab({ from, to }: ProductsTabProps) {
  const [data, setData] = useState<ProductRow[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [sortField, setSortField] = useState<SortField>('grossSalesCents');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [page, setPage] = useState(0);
  const limit = 50;
  const [retryTrigger, setRetryTrigger] = useState(0);

  useEffect(() => {
    let ignore = false;
    async function fetchProducts() {
      setLoading(true);
      setError(null);
      try {
        const res = await getAnalyticsProducts(from, to, sortField, sortOrder, limit, page * limit);
        if (!ignore) {
          setData(res.items);
          setTotalCount(res.totalCount);
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
    fetchProducts();
    return () => {
      ignore = true;
    };
  }, [from, to, sortField, sortOrder, page, limit, retryTrigger]);

  const sortedData = data;

  const toggleSort = (field: SortField) => {
    if (field === 'grossSalesChangePercent') return; // Cannot sort by this yet via API
    if (sortField === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortOrder('desc'); // default new field to desc
    }
  };

  if (loading) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 animate-pulse">
        <div className="h-10 bg-gray-200 rounded w-full mb-4"></div>
        {[1, 2, 3, 4, 5].map(i => <div key={i} className="h-16 bg-gray-100 rounded w-full mb-2"></div>)}
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 text-red-600 rounded-xl p-8 text-center border border-red-100">
        <p className="font-semibold">Ошибка загрузки товаров</p>
        <p className="text-sm mt-1">{error.message}</p>
        <button onClick={() => setRetryTrigger(r => r + 1)} className="mt-4 px-4 py-2 text-sm font-medium bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors">
          Повторить попытку
        </button>
      </div>
    );
  }

  const SortHeader = ({ field, label, align = 'left' }: { field: SortField, label: string, align?: 'left'|'right'|'center' }) => (
    <th 
      className={cn("px-4 py-3 cursor-pointer hover:bg-gray-100 transition-colors select-none", align === 'right' && 'text-right', align === 'center' && 'text-center')}
      onClick={() => toggleSort(field)}
    >
      <div className={cn("flex items-center gap-1", align === 'right' && 'justify-end', align === 'center' && 'justify-center')}>
        {label}
        {sortField === field && field !== 'grossSalesChangePercent' && (
          sortOrder === 'asc' ? <ArrowUpIcon className="w-4 h-4" /> : <ArrowDownIcon className="w-4 h-4" />
        )}
      </div>
    </th>
  );

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        {data.length === 0 ? (
          <div className="p-12 text-center text-gray-500">
            За выбранный период продаж не было.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-gray-50 text-gray-600 font-medium border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3">Товар</th>
                  <SortHeader field="grossSalesCents" label="Продажи" align="right" />
                  <SortHeader field="ordersCount" label="Заказы" align="right" />
                  <SortHeader field="unitsSold" label="Штук" align="right" />
                  <SortHeader field="grossSalesChangePercent" label="Динамика" align="right" />
                  <SortHeader field="returnRatePercent" label="Возвраты" align="right" />
                  <SortHeader field="availableStock" label="Остаток" align="right" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {sortedData.map(row => (
                  <tr key={row.productId} className="hover:bg-gray-50 transition-colors">
                    <td className="px-4 py-3">
                      <Link to={`/analytics/products/${row.productId}`} className="font-medium text-blue-600 hover:underline">
                        {row.title}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-right font-semibold text-gray-900">
                      {formatCents(row.grossSalesCents)}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {row.ordersCount}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {row.unitsSold} шт.
                    </td>
                    <td className="px-4 py-3 text-right">
                      {row.comparisonState === 'new' ? (
                         <span className="text-xs font-medium text-blue-600 bg-blue-50 px-2 py-0.5 rounded">Новый</span>
                      ) : row.grossSalesChangePercent !== null ? (
                        <span className={cn(
                          "flex items-center justify-end font-medium",
                          row.comparisonState === 'positive' && "text-emerald-600",
                          row.comparisonState === 'negative' && "text-red-600",
                          row.comparisonState === 'unchanged' && "text-gray-500"
                        )}>
                          {row.comparisonState === 'positive' && <ArrowUpIcon className="w-3 h-3 mr-1" />}
                          {row.comparisonState === 'negative' && <ArrowDownIcon className="w-3 h-3 mr-1" />}
                          {row.comparisonState === 'unchanged' && <MinusIcon className="w-3 h-3 mr-1" />}
                          {Math.abs(row.grossSalesChangePercent).toFixed(1)}%
                        </span>
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {row.returnRatePercent.toFixed(1)}%
                    </td>
                    <td className="px-4 py-3 text-right text-gray-600">
                      {row.availableStock} шт.
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {data.length > 0 && (
          <div className="px-4 py-3 border-t border-gray-200 flex items-center justify-between bg-gray-50">
            <span className="text-sm text-gray-700">
              Показаны {page * limit + 1}-{Math.min((page + 1) * limit, totalCount)} из {totalCount}
            </span>
            <div className="flex space-x-2">
              <button
                onClick={() => setPage(p => Math.max(0, p - 1))}
                disabled={page === 0}
                className="px-3 py-1 text-sm bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Назад
              </button>
              <button
                onClick={() => setPage(p => p + 1)}
                disabled={(page + 1) * limit >= totalCount}
                className="px-3 py-1 text-sm bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Вперед
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

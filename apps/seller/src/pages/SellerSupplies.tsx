import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Package, Truck, AlertCircle, Calendar } from 'lucide-react';
import { getSellerSupplies } from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerTableShell } from '../components/SellerSurface';
import { cn } from '../lib/utils';

export function SellerSupplies() {
  const [supplies, setSupplies] = useState<SellerSupply[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState('all');

  useEffect(() => {
    fetchSupplies();
  }, []);

  const fetchSupplies = async () => {
    try {
      setLoading(true);
      const data = await getSellerSupplies();
      setSupplies(data);
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки поставок');
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    const badgeMap: Record<string, { label: string; className: string }> = {
      draft: { label: 'Черновик', className: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10' },
      ready_to_ship: { label: 'Готова к отправке', className: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10' },
      shipped_by_seller: { label: 'В пути', className: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
      arrived_at_zamk: { label: 'Прибыла в ZAMK', className: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
      receiving: { label: 'Приёмка', className: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
      completed: { label: 'Принята', className: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-400 dark:border-emerald-900/40' },
      completed_with_discrepancies: { label: 'Принята с расхождениями', className: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
      cancelled: { label: 'Отменена', className: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40' },
    };

    const item = badgeMap[status] || { label: status, className: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10' };

    return (
      <span className={cn('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium', item.className)}>
        {item.label}
      </span>
    );
  };

  const filteredSupplies = supplies.filter(s => {
    if (filter === 'all') return true;
    if (filter === 'ready') return s.status === 'ready_to_ship';
    if (filter === 'shipped') return s.status === 'shipped_by_seller' || s.status === 'arrived_at_zamk';
    if (filter === 'receiving') return s.status === 'receiving';
    if (filter === 'completed') return s.status === 'completed' || s.status === 'completed_with_discrepancies';
    return true;
  });

  if (loading) {
    return (
      <SellerPageFrame variant="wide">
        <div className="flex justify-center py-20">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="wide">
      <SellerPageHeader
        eyebrow="Ассортимент"
        title="Поставки"
        description="Управление поставками товаров на склад ZAMK и отслеживание статуса приёмки."
        action={
          <Link
            to="/supplies/new"
            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            <Plus className="h-4 w-4" />
            Создать поставку
          </Link>
        }
      />

      <div className="flex space-x-2 mb-6 overflow-x-auto pb-2">
        {[
          { key: 'all', label: 'Все' },
          { key: 'ready', label: 'Готовы к отправке' },
          { key: 'shipped', label: 'В пути' },
          { key: 'receiving', label: 'На приёмке' },
          { key: 'completed', label: 'Завершены' },
        ].map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setFilter(key)}
            className={cn(
              "px-3.5 py-1.5 rounded-full text-xs sm:text-sm font-medium whitespace-nowrap transition-colors",
              filter === key
                ? "bg-gray-900 text-white"
                : "bg-white text-gray-600 border border-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-white/70 dark:border-white/10 dark:hover:bg-white/10"
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {error && (
        <div className="mb-6 bg-red-50 p-4 rounded-lg flex items-center border border-red-200 dark:bg-red-950/20 dark:border-red-900/30">
          <AlertCircle className="h-5 w-5 text-red-500 mr-3" />
          <span className="text-red-700 font-medium text-sm dark:text-red-300">{error}</span>
        </div>
      )}

      {filteredSupplies.length > 0 ? (
        <SellerTableShell>
          <ul className="divide-y divide-gray-100 dark:divide-white/5">
            {filteredSupplies.map((supply) => {
              const skuCount = supply.skuCount ?? (supply.items ? new Set(supply.items.map(i => i.sku || i.variantId)).size : 0);

              return (
                <li key={supply.id}>
                  <Link to={`/supplies/${supply.id}`} className="block hover:bg-gray-50/60 dark:hover:bg-white/[0.02] transition-colors p-5 sm:p-6">
                    <div className="flex flex-col sm:flex-row sm:items-start justify-between">
                      <div className="mb-4 sm:mb-0">
                        <div className="flex items-center space-x-3 mb-2">
                          <h3 className="text-base sm:text-lg font-bold text-gray-900 dark:text-white">{supply.supplyNumber || 'SUP-...'}</h3>
                          {getStatusBadge(supply.status)}
                        </div>
                        <div className="flex flex-wrap items-center text-xs sm:text-sm text-gray-500 dark:text-gray-400 mb-4 gap-x-4 gap-y-1">
                          <span className="flex items-center">
                            <Calendar className="w-4 h-4 mr-1 text-gray-400" />
                            {new Date(supply.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
                          </span>
                          <span className="flex items-center">
                            <Truck className="w-4 h-4 mr-1 text-gray-400" />
                            {supply.carrierName ? `Транспортная компания (${supply.carrierName})` : 'Транспортная компания'}
                          </span>
                          {supply.trackingNumber && (
                            <span className="font-mono text-xs bg-gray-100 dark:bg-white/10 px-2 py-0.5 rounded text-gray-600 dark:text-gray-300">
                              {supply.trackingNumber}
                            </span>
                          )}
                        </div>

                        <div className="flex flex-wrap items-center gap-6">
                          <div>
                            <p className="text-[11px] text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold">SKU</p>
                            <p className="mt-1 text-sm font-medium text-gray-900 dark:text-white">{skuCount}</p>
                          </div>
                          <div>
                            <p className="text-[11px] text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold">Заявлено (шт)</p>
                            <p className="mt-1 text-sm font-medium text-gray-900 dark:text-white">{supply.totalExpectedItems ?? 0}</p>
                          </div>
                          <div>
                            <p className="text-[11px] text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold">Грузомест</p>
                            <p className="mt-1 text-sm font-medium text-gray-900 dark:text-white">{supply.totalExpectedBoxes ?? 1}</p>
                          </div>
                          {(supply.status === 'completed' || supply.status === 'completed_with_discrepancies') && (
                            <div className="pl-6 border-l border-gray-200 dark:border-white/10">
                              <p className="text-[11px] text-gray-500 dark:text-gray-400 uppercase tracking-wider font-semibold">Итог приёмки</p>
                              <p className={`mt-1 text-sm font-bold ${supply.totalExpectedItems === supply.totalAcceptedItems ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}`}>
                                {supply.totalAcceptedItems ?? 0} принято
                              </p>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>
        </SellerTableShell>
      ) : (
        <SellerSurface className="text-center py-16 px-4">
          <Package className="mx-auto h-12 w-12 text-gray-300 dark:text-gray-600 mb-4" />
          <h3 className="text-base font-semibold text-gray-900 dark:text-white">Нет поставок</h3>
          <p className="mt-1.5 text-sm text-gray-500 dark:text-gray-400">
            Пока вы не создали ни одной поставки, подходящей под фильтры.
          </p>
          <div className="mt-6">
            <Link
              to="/supplies/new"
              className="inline-flex items-center px-4 py-2 text-sm font-medium rounded-lg text-white bg-gray-900 hover:bg-gray-800 transition-colors shadow-sm"
            >
              <Plus className="-ml-1 mr-2 h-4 w-4" />
              Создать поставку
            </Link>
          </div>
        </SellerSurface>
      )}
    </SellerPageFrame>
  );
}

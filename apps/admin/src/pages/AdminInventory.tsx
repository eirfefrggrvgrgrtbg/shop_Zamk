import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { AlertCircle, Boxes, PackageSearch, Search } from 'lucide-react';
import { HelpTooltip } from '../components/HelpTooltip';
import { SellerContextBanner } from '../components/SellerContextBanner';
import {
  getAdminInventory,
  getAdminInventoryErrorMessage,
  getAdminInventoryItem,
  getAdminInventoryMovements,
  type PhysicalUnitContext,
} from '../api/adminInventory';
import type { AdminInventoryMovementView, AdminInventoryView } from '../api/adminInventory';

export function AdminInventory() {
  const [inventory, setInventory] = useState<AdminInventoryView[]>([]);
  const [selectedItem, setSelectedItem] = useState<AdminInventoryView | null>(null);
  const [movements, setMovements] = useState<AdminInventoryMovementView[]>([]);
  const [unitContext, setUnitContext] = useState<PhysicalUnitContext | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isMovementsLoading, setIsMovementsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchParams] = useSearchParams();
  const urlSellerId = searchParams.get('sellerId');
  const urlQ = searchParams.get('q');

  const [searchQuery, setSearchQuery] = useState(urlQ || '');
  const [sourceFilter, setSourceFilter] = useState('');
  const [sellerFilter, setSellerFilter] = useState(urlSellerId || '');

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
  const [lowStockFilter, setLowStockFilter] = useState(false);
  const [pagination, setPagination] = useState({ limit: 50, offset: 0, total: 0 });

  const fetchInventory = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const resp = await getAdminInventory({
        q: searchQuery,
        source: sourceFilter,
        sellerId: sellerFilter,
        lowStock: lowStockFilter,
        limit: pagination.limit,
        offset: pagination.offset,
      });
      setInventory(resp.items);
      setPagination(p => ({ ...p, total: resp.totalCount }));
      setUnitContext(resp.unitContext ?? null);
      if (selectedItem) {
        const refreshed = resp.items.find((item) => item.id === selectedItem.id) ?? null;
        setSelectedItem(refreshed);
      }
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Не удалось загрузить остатки.'));
      setUnitContext(null);
    } finally {
      setIsLoading(false);
    }
  };

  const fetchMovements = async (item: AdminInventoryView) => {
    try {
      setIsMovementsLoading(true);
      setError(null);
      const detail = await getAdminInventoryItem(item.id);
      setSelectedItem(detail);
      const data = await getAdminInventoryMovements(item.id);
      setMovements(data);
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Не удалось загрузить движения остатков.'));
    } finally {
      setIsMovementsLoading(false);
    }
  };

  useEffect(() => {
    fetchInventory();
  }, [searchQuery, sourceFilter, sellerFilter, lowStockFilter, pagination.limit, pagination.offset]);


  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <SellerContextBanner />
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Остатки / Склад</h1>
      </div>

      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm hover:border-indigo-200 transition-colors">
        <div className="flex items-center gap-3.5">
          <div className="w-10 h-10 rounded-lg bg-indigo-50 border border-indigo-100 text-indigo-600 flex items-center justify-center flex-shrink-0">
            <PackageSearch className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-slate-900">Свободный сканер ZMU</h2>
            <p className="text-sm text-slate-500">Нашли товар без коробки или номера поставки? Просто отсканируйте ZMU.</p>
          </div>
        </div>
        <Link
          to="/warehouse/free-scan"
          className="inline-flex items-center justify-center px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-lg shadow-sm transition-colors whitespace-nowrap"
        >
          Открыть сканер
        </Link>
      </div>

      <div className="bg-white p-4 rounded-lg shadow space-y-4 sm:space-y-0 sm:flex sm:items-center sm:space-x-4">
        <div className="flex-1 relative rounded-md shadow-sm">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <Search className="h-5 w-5 text-gray-400" />
          </div>
          <input
            type="text"
            className="focus:ring-indigo-500 focus:border-indigo-500 block w-full pl-10 sm:text-sm border-gray-300 rounded-md"
            placeholder="Поиск по названию, SKU или ZMU..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        <div className="w-full sm:w-48">
          <select
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value)}
            className="mt-1 block w-full pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
          >
            <option value="">Все источники</option>
            <option value="auction_direct_sale">ZAMK (Свой склад)</option>
            <option value="seller">Продавцы</option>
          </select>
        </div>
        <div className="w-full sm:w-48">
          <input
            type="text"
            className="focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border-gray-300 rounded-md"
            placeholder="ID продавца..."
            value={sellerFilter}
            onChange={(e) => setSellerFilter(e.target.value)}
          />
        </div>
        <div className="flex items-center">
          <input
            id="low-stock"
            name="low-stock"
            type="checkbox"
            checked={lowStockFilter}
            onChange={(e) => setLowStockFilter(e.target.checked)}
            className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
          />
          <label htmlFor="low-stock" className="ml-2 block text-sm text-gray-900">
            Меньше 10 шт
          </label>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
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
          <div className="flex items-center space-x-2">
            <span className="text-xs text-gray-500 font-medium">Статус единицы:</span>
            <span className="px-2.5 py-1 text-xs font-semibold rounded-full bg-white text-indigo-800 border border-indigo-200 shadow-2xs">
              {unitContext.statusLabel || unitContext.status}
            </span>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Загрузка остатков...</p>
        </div>
      ) : inventory.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Boxes className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">
            {unitContext ? 'Нет агрегированных остатков' : 'Нет данных'}
          </h3>
          <p className="mt-1 text-sm text-gray-500">
            {unitContext
              ? 'Физическая единица найдена, но агрегированные складские остатки отсутствуют.'
              : 'Записи об остатках пока отсутствуют.'}
          </p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Товар / Вариант</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Seller</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      На складе
                      <HelpTooltip content="Количество товара, доступное для продажи (Доступно - Зарезервировано)." />
                    </th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      В резерве
                      <HelpTooltip content="Товар, который находится в активных корзинах или неоплаченных заказах." />
                    </th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Доступно
                      <HelpTooltip content="Физическое количество товара на складе." />
                    </th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {inventory.map((item) => (
                    <tr key={item.id}>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900">{item.productTitle}</div>
                        <div className="text-xs text-gray-500">{item.variant}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center space-x-2">
                          {(item as any).source === 'auction_direct_sale' ? (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                              ZAMK
                            </span>
                          ) : (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
                              Seller
                            </span>
                          )}
                          <span className="text-sm text-gray-500">{item.sellerName || item.sellerId || '-'}</span>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          item.availableStock > 10 ? 'bg-green-100 text-green-800' : 
                          item.availableStock > 0 ? 'bg-yellow-100 text-yellow-800' : 
                          'bg-red-100 text-red-800'
                        }`}>
                          {item.availableStock}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {item.reservedStock}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {item.totalStock}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <button onClick={() => fetchMovements(item)} className="text-indigo-600 hover:text-indigo-900">Открыть</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
      )}

      {selectedItem && (
        <div className="grid gap-6 lg:grid-cols-1">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900">Движения остатков</h2>
            {isMovementsLoading ? (
              <p className="mt-4 text-sm text-gray-500">Загрузка движений...</p>
            ) : movements.length === 0 ? (
              <p className="mt-4 text-sm text-gray-500">Нет данных о движениях для этого товара.</p>
            ) : (
              <div className="mt-4 overflow-hidden border border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Тип</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Кол-во</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Причина</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Создано</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200 bg-white">
                    {movements.map((movement) => (
                      <tr key={movement.id}>
                        <td className="px-4 py-2 text-sm text-gray-900">{movement.type}</td>
                        <td className="px-4 py-2 text-sm text-gray-900">{movement.quantity}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{movement.reason || movement.referenceType || '-'}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{formatDate(movement.createdAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

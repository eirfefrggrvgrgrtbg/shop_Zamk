import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { getSellerInventory } from '@zamk/api-client/src/seller';
import type { SellerInventoryItem } from '@zamk/api-client/src/types';
import { Package, Search, Filter, AlertCircle, TrendingUp, CheckCircle } from 'lucide-react';

type FilterType = 'all' | 'in_stock' | 'low_stock' | 'out_of_stock' | 'inbound';

export function SellerInventory() {
  const navigate = useNavigate();
  const [inventory, setInventory] = useState<SellerInventoryItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  
  const [searchQuery, setSearchQuery] = useState('');
  const [activeFilter, setActiveFilter] = useState<FilterType>('all');

  useEffect(() => {
    async function fetchInventory() {
      try {
        const data = await getSellerInventory();
        setInventory(data.items || []);
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки остатков');
      } finally {
        setIsLoading(false);
      }
    }
    fetchInventory();
  }, []);

  const stats = useMemo(() => {
    let onHand = 0;
    let available = 0;
    let reserved = 0;
    let inbound = 0;
    let outOfStock = 0;

    inventory.forEach(item => {
      onHand += item.onHand;
      available += item.available;
      reserved += item.reserved;
      inbound += item.inbound;
      if (item.available === 0 && item.inbound === 0) {
        outOfStock++;
      }
    });

    return { onHand, available, reserved, inbound, outOfStock };
  }, [inventory]);

  const filteredInventory = useMemo(() => {
    return inventory.filter(item => {
      // Search
      const query = searchQuery.toLowerCase();
      const matchesSearch = !query || 
        item.productTitle.toLowerCase().includes(query) || 
        item.sku.toLowerCase().includes(query);

      if (!matchesSearch) return false;

      // Filter
      switch (activeFilter) {
        case 'in_stock':
          return item.availabilityStatus === 'В наличии' || item.availabilityStatus === 'Заканчивается';
        case 'low_stock':
          return item.availabilityStatus === 'Заканчивается';
        case 'out_of_stock':
          return item.availabilityStatus === 'Нет в наличии';
        case 'inbound':
          return item.inbound > 0;
        case 'all':
        default:
          return true;
      }
    });
  }, [inventory, searchQuery, activeFilter]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex justify-center items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div>
      </div>
    );
  }

  if (error) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 tracking-tight">Склад ZAMK</h1>
          <p className="text-gray-500 mt-2">Единый источник правды об остатках ваших товаров (только для чтения)</p>
        </div>
        <button
          onClick={() => navigate('/supplies/new')}
          className="bg-black text-white px-6 py-2.5 rounded-lg hover:bg-gray-800 transition-colors font-medium flex items-center gap-2"
        >
          <Package className="w-4 h-4" />
          Создать поставку
        </button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-5 gap-4 mb-8">
        <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm flex flex-col">
          <span className="text-sm text-gray-500 font-medium mb-1">На складе</span>
          <span className="text-3xl font-bold text-gray-900">{stats.onHand}</span>
        </div>
        <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm flex flex-col">
          <span className="text-sm text-gray-500 font-medium mb-1 flex items-center gap-1">
            <CheckCircle className="w-4 h-4 text-green-500" />
            Доступно
          </span>
          <span className="text-3xl font-bold text-green-600">{stats.available}</span>
        </div>
        <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm flex flex-col">
          <span className="text-sm text-gray-500 font-medium mb-1">В резерве</span>
          <span className="text-3xl font-bold text-gray-900">{stats.reserved}</span>
        </div>
        <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm flex flex-col">
          <span className="text-sm text-gray-500 font-medium mb-1 flex items-center gap-1">
            <TrendingUp className="w-4 h-4 text-blue-500" />
            В пути
          </span>
          <span className="text-3xl font-bold text-blue-600">{stats.inbound}</span>
        </div>
        <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm flex flex-col">
          <span className="text-sm text-gray-500 font-medium mb-1 flex items-center gap-1">
            <AlertCircle className="w-4 h-4 text-red-500" />
            Без остатка
          </span>
          <span className="text-3xl font-bold text-red-600">{stats.outOfStock}</span>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {/* Controls */}
        <div className="p-4 border-b border-gray-100 flex flex-col sm:flex-row justify-between items-center gap-4 bg-gray-50/50">
          <div className="flex items-center gap-2 overflow-x-auto pb-2 sm:pb-0 w-full sm:w-auto">
            <Filter className="w-4 h-4 text-gray-400 mr-2" />
            {(['all', 'in_stock', 'low_stock', 'out_of_stock', 'inbound'] as const).map(f => {
              const labels: Record<FilterType, string> = {
                all: 'Все',
                in_stock: 'В наличии',
                low_stock: 'Заканчивается',
                out_of_stock: 'Нет в наличии',
                inbound: 'В пути'
              };
              return (
                <button
                  key={f}
                  onClick={() => setActiveFilter(f)}
                  className={`px-3 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors ${
                    activeFilter === f 
                      ? 'bg-black text-white' 
                      : 'bg-white text-gray-600 border border-gray-200 hover:bg-gray-50'
                  }`}
                >
                  {labels[f]}
                </button>
              );
            })}
          </div>
          <div className="relative w-full sm:w-64">
            <input
              type="text"
              placeholder="Поиск по SKU или названию..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-gray-200 rounded-lg focus:ring-2 focus:ring-black focus:border-transparent text-sm"
            />
            <Search className="w-4 h-4 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
          </div>
        </div>

        {/* Table */}
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-white">
              <tr>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Фото</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Товар / Вариант</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">SKU</th>
                <th className="px-6 py-4 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">На складе</th>
                <th className="px-6 py-4 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Резерв</th>
                <th className="px-6 py-4 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Доступно</th>
                <th className="px-6 py-4 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">В пути</th>
                <th className="px-6 py-4 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Статус</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 bg-white">
              {filteredInventory.map((item) => (
                <tr key={item.variantId} data-testid={`inventory-row-${item.variantId}`} className="hover:bg-gray-50/50 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap">
                    {item.image ? (
                      <img src={item.image} alt={item.productTitle} className="h-12 w-12 object-cover rounded border border-gray-200" />
                    ) : (
                      <div className="h-12 w-12 bg-gray-100 rounded border border-gray-200 flex items-center justify-center">
                        <Package className="w-5 h-5 text-gray-300" />
                      </div>
                    )}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-col">
                      <span className="text-sm font-medium text-gray-900 line-clamp-1">{item.productTitle}</span>
                      {item.optionValues && Object.keys(item.optionValues).length > 0 && (
                        <span className="text-xs text-gray-500 mt-0.5">
                          {Object.entries(item.optionValues).map(([k, v]) => `${k}: ${v}`).join(' • ')}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium bg-gray-100 text-gray-800">
                      {item.sku}
                    </span>
                  </td>
                  <td data-testid="onhand-value" className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 text-right">{item.onHand}</td>
                  <td data-testid="reserved-value" className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 text-right">{item.reserved}</td>
                  <td data-testid="available-value" className="px-6 py-4 whitespace-nowrap text-right">
                    <span className={`text-sm font-bold ${item.available > 0 ? 'text-green-600' : 'text-gray-400'}`}>
                      {item.available}
                    </span>
                  </td>
                  <td data-testid="inbound-value" className="px-6 py-4 whitespace-nowrap text-sm text-right">
                    {item.inbound > 0 ? (
                      <span className="text-blue-600 font-medium">+{item.inbound}</span>
                    ) : (
                      <span className="text-gray-300">0</span>
                    )}
                  </td>
                  <td data-testid="status-badge" className="px-6 py-4 whitespace-nowrap">
                    <StatusBadge status={item.availabilityStatus} />
                  </td>
                </tr>
              ))}
              {filteredInventory.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-6 py-12 text-center">
                    <Package className="w-12 h-12 text-gray-300 mx-auto mb-3" />
                    <h3 className="text-sm font-medium text-gray-900">Ничего не найдено</h3>
                    <p className="text-sm text-gray-500 mt-1">Попробуйте изменить параметры поиска или фильтрации</p>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  let bg = 'bg-gray-100';
  let text = 'text-gray-800';
  
  if (status === 'В наличии') {
    bg = 'bg-green-100';
    text = 'text-green-800';
  } else if (status === 'Заканчивается') {
    bg = 'bg-yellow-100';
    text = 'text-yellow-800';
  } else if (status === 'Ожидается поставка') {
    bg = 'bg-blue-100';
    text = 'text-blue-800';
  } else if (status === 'Нет в наличии') {
    bg = 'bg-red-100';
    text = 'text-red-800';
  }

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${bg} ${text}`}>
      {status}
    </span>
  );
}

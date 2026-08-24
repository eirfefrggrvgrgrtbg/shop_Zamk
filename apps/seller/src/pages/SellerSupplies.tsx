import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Package, Truck, AlertCircle, Calendar } from 'lucide-react';
import { getSellerSupplies } from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';

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
    switch (status) {
      case 'draft': return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">Черновик</span>;
      case 'ready_to_ship': return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">Готова к отправке</span>;
      case 'shipped_by_seller': return <span className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-xs font-bold uppercase tracking-wide">В пути</span>;
      case 'arrived_at_zamk': return <span className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-xs font-bold uppercase tracking-wide">Прибыла в ZAMK</span>;
      case 'receiving': return <span className="px-3 py-1 bg-yellow-100 text-yellow-800 rounded-full text-xs font-bold uppercase tracking-wide">Приёмка</span>;
      case 'completed': return <span className="px-3 py-1 bg-green-100 text-green-800 rounded-full text-xs font-bold uppercase tracking-wide">Принята</span>;
      case 'completed_with_discrepancies': return <span className="px-3 py-1 bg-orange-100 text-orange-800 rounded-full text-xs font-bold uppercase tracking-wide">Принята с расхождениями</span>;
      case 'cancelled': return <span className="px-3 py-1 bg-red-100 text-red-800 rounded-full text-xs font-bold uppercase tracking-wide">Отменена</span>;
      default: return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">{status}</span>;
    }
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
    return <div className="p-8 flex justify-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div></div>;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold text-gray-900 tracking-tight">Поставки</h1>
        <Link
          to="/supplies/new"
          className="inline-flex items-center px-6 py-3 border border-transparent rounded-full shadow-sm text-sm font-bold text-white bg-black hover:bg-gray-800 focus:outline-none transition-colors"
        >
          <Plus className="-ml-1 mr-2 h-5 w-5" />
          Создать поставку
        </Link>
      </div>

      <div className="flex space-x-2 mb-6 overflow-x-auto pb-2">
        {['all', 'ready', 'shipped', 'receiving', 'completed'].map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-4 py-2 rounded-full text-sm font-medium whitespace-nowrap transition-colors ${
              filter === f ? 'bg-black text-white' : 'bg-white border border-gray-200 text-gray-700 hover:bg-gray-50'
            }`}
          >
            {f === 'all' && 'Все'}
            {f === 'ready' && 'Готовы к отправке'}
            {f === 'shipped' && 'В пути'}
            {f === 'receiving' && 'На приёмке'}
            {f === 'completed' && 'Завершены'}
          </button>
        ))}
      </div>

      {error && (
        <div className="mb-6 bg-red-50 p-4 rounded-lg flex items-center border border-red-100">
          <AlertCircle className="h-5 w-5 text-red-500 mr-3" />
          <span className="text-red-700 font-medium">{error}</span>
        </div>
      )}

      <div className="bg-white shadow-sm border border-gray-200 overflow-hidden sm:rounded-2xl">
        {filteredSupplies.length > 0 ? (
          <ul className="divide-y divide-gray-100">
            {filteredSupplies.map((supply) => {
              const uniqueSkus = new Set();
              let skuCount = 0;
              if (supply.items) {
                supply.items.forEach(item => uniqueSkus.add(item.sku));
                skuCount = uniqueSkus.size;
              }
              
              return (
                <li key={supply.id}>
                  <Link to={`/supplies/${supply.id}`} className="block hover:bg-gray-50 transition-colors p-6">
                    <div className="flex flex-col sm:flex-row sm:items-start justify-between">
                      <div className="mb-4 sm:mb-0">
                        <div className="flex items-center space-x-4 mb-2">
                          <h3 className="text-lg font-bold text-black">{supply.supplyNumber || 'SUP-...'}</h3>
                          {getStatusBadge(supply.status)}
                        </div>
                        <div className="flex items-center text-sm text-gray-500 mb-4 space-x-4">
                          <span className="flex items-center">
                            <Calendar className="w-4 h-4 mr-1" />
                            {new Date(supply.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
                          </span>
                          {supply.handoffMethod && (
                            <span className="flex items-center">
                              <Truck className="w-4 h-4 mr-1" />
                              Транспортная компания
                            </span>
                          )}
                        </div>
                        
                        <div className="flex items-center space-x-6">
                          <div>
                            <p className="text-xs text-gray-500 uppercase tracking-wide font-bold">SKU</p>
                            <p className="mt-1 font-medium text-gray-900">{skuCount}</p>
                          </div>
                          <div>
                            <p className="text-xs text-gray-500 uppercase tracking-wide font-bold">Заявлено (шт)</p>
                            <p className="mt-1 font-medium text-gray-900">{supply.totalExpectedItems}</p>
                          </div>
                          <div>
                            <p className="text-xs text-gray-500 uppercase tracking-wide font-bold">Грузомест</p>
                            <p className="mt-1 font-medium text-gray-900">{supply.totalExpectedBoxes}</p>
                          </div>
                          {(supply.status === 'completed' || supply.status === 'completed_with_discrepancies') && (
                            <div className="pl-6 border-l border-gray-200">
                              <p className="text-xs text-gray-500 uppercase tracking-wide font-bold">Итог приёмки</p>
                              <p className={`mt-1 font-bold ${supply.totalExpectedItems === supply.totalAcceptedItems ? 'text-green-600' : 'text-orange-600'}`}>
                                {supply.totalAcceptedItems} принято
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
        ) : (
          <div className="text-center py-16 px-4">
            <Package className="mx-auto h-12 w-12 text-gray-300 mb-4" />
            <h3 className="text-lg font-medium text-gray-900">Нет поставок</h3>
            <p className="mt-2 text-sm text-gray-500">
              Пока вы не создали ни одной поставки, подходящей под фильтры.
            </p>
            <div className="mt-6">
              <Link
                to="/supplies/new"
                className="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-black hover:bg-gray-800"
              >
                <Plus className="-ml-1 mr-2 h-5 w-5" />
                Создать поставку
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

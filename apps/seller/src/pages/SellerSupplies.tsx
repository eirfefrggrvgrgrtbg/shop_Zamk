import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Package, Truck, AlertCircle } from 'lucide-react';
import { getSellerSupplies } from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';

export function SellerSupplies() {
  const [supplies, setSupplies] = useState<SellerSupply[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
      case 'ready_to_ship':
      case 'pending_shipment': return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">Готовится к отправке</span>;
      case 'shipped':
      case 'shipped_by_seller': return <span className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-xs font-bold uppercase tracking-wide">Отправлена</span>;
      case 'receiving': return <span className="px-3 py-1 bg-yellow-100 text-yellow-800 rounded-full text-xs font-bold uppercase tracking-wide">На приёмке</span>;
      case 'completed': return <span className="px-3 py-1 bg-green-100 text-green-800 rounded-full text-xs font-bold uppercase tracking-wide">Принята</span>;
      case 'completed_with_discrepancies': return <span className="px-3 py-1 bg-orange-100 text-orange-800 rounded-full text-xs font-bold uppercase tracking-wide">С расхождениями</span>;
      default: return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">{status}</span>;
    }
  };

  const getHandoffMethodText = (method: string) => {
    if (method === 'courier') return 'Курьером / ТК';
    return 'Привоз на ПВЗ';
  };

  if (loading) {
    return <div className="p-8 flex justify-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div></div>;
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 tracking-tight">Поставки</h1>
          <p className="mt-2 text-sm text-gray-500">Управление поставками на склад ZAMK</p>
        </div>
        <Link
          to="/supplies/new"
          className="inline-flex items-center px-6 py-3 border border-transparent rounded-full shadow-sm text-sm font-bold text-white bg-black hover:bg-gray-800 focus:outline-none transition-colors"
        >
          <Plus className="-ml-1 mr-2 h-5 w-5" />
          Создать поставку
        </Link>
      </div>

      {error && (
        <div className="mb-6 bg-red-50 p-4 rounded-lg flex items-center border border-red-100">
          <AlertCircle className="h-5 w-5 text-red-500 mr-3" />
          <span className="text-red-700 font-medium">{error}</span>
        </div>
      )}

      <div className="bg-white shadow-sm border border-gray-200 overflow-hidden sm:rounded-2xl">
        {supplies.length > 0 ? (
          <ul className="divide-y divide-gray-100">
            {supplies.map((supply) => {
              // Calculate unique SKUs
              const uniqueSkus = new Set();
              if (supply.boxes) {
                supply.boxes.forEach(box => box.items?.forEach(item => uniqueSkus.add(item.sku)));
              } else if (supply.items) {
                supply.items.forEach(item => uniqueSkus.add(item.sku));
              }
              const skuCount = uniqueSkus.size || 0;

              return (
                <li key={supply.id}>
                  <Link to={`/supplies/${supply.id}`} className="block hover:bg-gray-50 transition-colors p-6">
                    <div className="flex items-start justify-between">
                      <div>
                        <div className="flex items-center space-x-4 mb-2">
                          <h3 className="text-lg font-bold text-black">Поставка {supply.supplyNumber}</h3>
                          {getStatusBadge(supply.status)}
                        </div>
                        <p className="text-sm text-gray-500 mb-4">
                          {new Date(supply.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
                        </p>
                        
                        <div className="flex items-center space-x-6">
                          <div className="flex items-center text-sm font-medium text-gray-900">
                            <Package className="w-4 h-4 mr-2 text-gray-400" />
                            {skuCount} SKU • {supply.totalExpectedItems || supply.items?.reduce((acc, i) => acc + (i.expectedQuantity || 0), 0) || 0} шт.
                          </div>
                          <div className="flex items-center text-sm text-gray-500">
                            <div className="w-1.5 h-1.5 rounded-full bg-gray-300 mr-2"></div>
                            {supply.totalExpectedBoxes || supply.boxes?.length || 0} короб
                          </div>
                          <div className="flex items-center text-sm text-gray-500">
                            <div className="w-1.5 h-1.5 rounded-full bg-gray-300 mr-2"></div>
                            {getHandoffMethodText(supply.handoffMethod)}
                          </div>
                        </div>
                      </div>
                      <div className="flex flex-col items-end">
                        <Truck className="h-8 w-8 text-gray-200" />
                      </div>
                    </div>
                  </Link>
                </li>
              );
            })}
          </ul>
        ) : (
          <div className="text-center py-16 px-4">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-gray-100 mb-4">
              <Truck className="h-8 w-8 text-gray-400" />
            </div>
            <h3 className="text-lg font-bold text-gray-900 mb-2">Нет поставок</h3>
            <p className="text-sm text-gray-500 max-w-sm mx-auto mb-6">
              Начните работу со складом ZAMK, создав первую поставку.
            </p>
            <Link
              to="/supplies/new"
              className="inline-flex items-center px-6 py-3 border border-transparent shadow-sm text-sm font-bold rounded-full text-white bg-black hover:bg-gray-800 focus:outline-none transition-colors"
            >
              <Plus className="-ml-1 mr-2 h-5 w-5" />
              Создать поставку
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}

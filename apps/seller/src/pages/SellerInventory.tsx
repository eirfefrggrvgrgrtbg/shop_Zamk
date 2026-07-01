import { useEffect, useState } from 'react';
import { getSellerInventory } from '@zamk/api-client/src/seller';
import { adaptInventory } from '../api/sellerOperations';

export function SellerInventory() {
  const [inventory, setInventory] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function fetchInventory() {
      try {
        const data = await getSellerInventory();
        setInventory(adaptInventory(data));
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки остатков');
      } finally {
        setIsLoading(false);
      }
    }
    fetchInventory();
  }, []);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  if (error) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-4">Остатки на складе</h1>
      <p className="text-gray-600 mb-6">Список ваших товаров на складе (только для чтения). Изменением остатков занимается администратор площадки.</p>
      
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID Товара / Варианта</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Всего</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">В резерве</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Доступно</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {inventory.map((item, idx) => (
              <tr key={idx}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{item.productId}</td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{item.totalStock}</td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{item.reservedStock}</td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-green-600 font-semibold">{item.availableStock}</td>
              </tr>
            ))}
            {inventory.length === 0 && (
              <tr>
                <td colSpan={4} className="px-6 py-4 text-center text-sm text-gray-500">Нет данных об остатках</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

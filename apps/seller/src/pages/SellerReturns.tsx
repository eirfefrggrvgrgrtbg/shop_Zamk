import { useEffect, useState } from 'react';
import { getSellerReturns } from '@zamk/api-client/src/seller';
import { adaptReturns } from '../api/sellerOperations';

export function SellerReturns() {
  const [returns, setReturns] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function fetchReturns() {
      try {
        const data = await getSellerReturns();
        setReturns(adaptReturns(data));
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки возвратов');
      } finally {
        setIsLoading(false);
      }
    }
    fetchReturns();
  }, []);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  if (error) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-4">Возвраты</h1>
      <p className="text-gray-600 mb-6">Возвраты покупателей по вашим товарам (только для чтения).</p>
      
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ID Возврата</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {returns.map((ret) => (
              <tr key={ret.id}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{ret.id}</td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-gray-100 text-gray-800">
                    {ret.status}
                  </span>
                </td>
              </tr>
            ))}
            {returns.length === 0 && (
              <tr>
                <td colSpan={2} className="px-6 py-4 text-center text-sm text-gray-500">Нет данных о возвратах</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

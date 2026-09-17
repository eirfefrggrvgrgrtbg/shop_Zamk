import { useEffect, useState } from 'react';
import { getSellerReviews } from '@zamk/api-client/src/seller';
import { adaptReviews } from '../api/sellerOperations';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';

export function SellerReviews() {
  const [reviews, setReviews] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function fetchReviews() {
      try {
        const data = await getSellerReviews();
        setReviews(adaptReviews(data));
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки отзывов');
      } finally {
        setIsLoading(false);
      }
    }
    fetchReviews();
  }, []);

  if (isLoading) {
    return (
      <SellerPageFrame variant="summary">
        <div className="flex h-64 items-center justify-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black" />
        </div>
      </SellerPageFrame>
    );
  }

  if (error) {
    return (
      <SellerPageFrame variant="summary">
        <div className="flex h-64 items-center justify-center text-red-500">{error}</div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Продажи"
        title="Отзывы"
        description="Отзывы покупателей о ваших товарах (только для чтения)."
      />
      
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Оценка</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Текст</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {reviews.map((rev) => (
              <tr key={rev.id}>
                <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                  <div className="flex items-center text-amber-500">
                    {'★'.repeat(rev.rating)}{'☆'.repeat(5 - rev.rating)}
                  </div>
                </td>
                <td className="px-6 py-4 text-sm text-gray-500 break-words">{rev.content}</td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-gray-100 text-gray-800">
                    {rev.status}
                  </span>
                </td>
              </tr>
            ))}
            {reviews.length === 0 && (
              <tr>
                <td colSpan={3} className="px-6 py-4 text-center text-sm text-gray-500">Отзывов пока нет</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </SellerPageFrame>
  );
}

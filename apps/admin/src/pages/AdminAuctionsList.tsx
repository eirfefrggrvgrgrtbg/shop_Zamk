import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Gavel, Plus, Play, Pause, XCircle, CheckCircle } from 'lucide-react';
import { getAdminAuctions, publishAdminAuction, pauseAdminAuction, resumeAdminAuction, cancelAdminAuction, finalizeAdminAuction } from '@zamk/api-client/src/admin';
import type { AdminAuction } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';

const STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  scheduled: 'Запланирован',
  live: 'Идёт',
  paused: 'На паузе',
  ended: 'Завершён',
  cancelled: 'Отменён',
};

export function AdminAuctionsList() {
  const [auctions, setAuctions] = useState<AdminAuction[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { hasPermission } = useAdminAuth();

  const fetchAuctions = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminAuctions();
      setAuctions(data.items || []);
    } catch (err) {
      setError('Не удалось загрузить аукционы.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchAuctions();
  }, []);

  const handleAction = async (id: string, action: 'publish' | 'pause' | 'resume' | 'cancel' | 'finalize') => {
    if (action === 'cancel' && !window.confirm('Отменить аукцион?')) return;
    if (action === 'finalize' && !window.confirm('Завершить аукцион?\nПосле завершения лоты получат итоговые статусы.')) return;

    try {
      if (action === 'publish') await publishAdminAuction(id);
      if (action === 'pause') await pauseAdminAuction(id);
      if (action === 'resume') await resumeAdminAuction(id);
      if (action === 'cancel') await cancelAdminAuction(id);
      if (action === 'finalize') await finalizeAdminAuction(id);
      alert('Действие выполнено.');
      fetchAuctions();
    } catch (err) {
      alert('Не удалось выполнить действие.');
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-gray-500">Загружаем аукционы…</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Аукционы</h1>
          <p className="text-sm text-gray-500 mt-1">Управление аукционами платформы</p>
        </div>
        {hasPermission('auctions.create') && (
          <Link
            to="/auctions/new"
            className="flex items-center px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700"
          >
            <Plus className="h-5 w-5 mr-2" />
            Создать аукцион
          </Link>
        )}
      </div>

      {error && (
        <div className="bg-red-50 text-red-700 p-4 rounded-md">
          {error}
        </div>
      )}

      {auctions.length === 0 && !error ? (
        <div className="text-center py-12 bg-white rounded-lg border border-gray-200">
          <Gavel className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Аукционов пока нет.</h3>
          <p className="mt-1 text-sm text-gray-500">Создайте первый аукцион для покупателей.</p>
        </div>
      ) : (
        <div className="bg-white shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Название</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Даты</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Флаги</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Лоты</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Действия</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {auctions.map((auction) => (
                  <tr key={auction.id}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{auction.title}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                        ${auction.status === 'live' ? 'bg-green-100 text-green-800' : 
                          auction.status === 'draft' ? 'bg-gray-100 text-gray-800' :
                          auction.status === 'scheduled' ? 'bg-blue-100 text-blue-800' :
                          auction.status === 'paused' ? 'bg-yellow-100 text-yellow-800' :
                          'bg-red-100 text-red-800'}`}>
                        {STATUS_LABELS[auction.status] || auction.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <div>Старт: {new Date(auction.startsAt).toLocaleString('ru-RU')}</div>
                      <div>Окончание: {new Date(auction.endsAt).toLocaleString('ru-RU')}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      <div className="flex flex-col space-y-1">
                        <span className={auction.isPublic ? 'text-green-600' : 'text-gray-400'}>
                          Публичный: {auction.isPublic ? 'Да' : 'Нет'}
                        </span>
                        <span className={auction.showOnHomepage ? 'text-green-600' : 'text-gray-400'}>
                          На главной: {auction.showOnHomepage ? 'Да' : 'Нет'}
                        </span>
                        <span className={auction.highlightInNav ? 'text-green-600' : 'text-gray-400'}>
                          В меню: {auction.highlightInNav ? 'Да' : 'Нет'}
                        </span>
                        <span className={auction.biddingEnabled ? 'text-green-600' : 'text-red-500'}>
                          Ставки включены: {auction.biddingEnabled ? 'Да' : 'Нет'}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {auction.lotsCount || 0}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <div className="flex items-center justify-end space-x-2">
                        <Link to={`/auctions/${auction.id}`} className="text-indigo-600 hover:text-indigo-900">
                          Открыть
                        </Link>
                        
                        {auction.status === 'draft' && hasPermission('auctions.publish') && (
                          <button onClick={() => handleAction(auction.id, 'publish')} className="text-blue-600 hover:text-blue-900" title="Опубликовать">
                            <Play className="h-4 w-4 inline" />
                          </button>
                        )}
                        
                        {(auction.status === 'scheduled' || auction.status === 'live') && hasPermission('auctions.pause') && (
                          <button onClick={() => handleAction(auction.id, 'pause')} className="text-yellow-600 hover:text-yellow-900" title="Пауза">
                            <Pause className="h-4 w-4 inline" />
                          </button>
                        )}

                        {auction.status === 'paused' && hasPermission('auctions.pause') && (
                          <button onClick={() => handleAction(auction.id, 'resume')} className="text-green-600 hover:text-green-900" title="Возобновить">
                            <Play className="h-4 w-4 inline" />
                          </button>
                        )}

                        {(auction.status === 'draft' || auction.status === 'scheduled' || auction.status === 'paused') && hasPermission('auctions.cancel') && (
                          <button onClick={() => handleAction(auction.id, 'cancel')} className="text-red-600 hover:text-red-900" title="Отменить">
                            <XCircle className="h-4 w-4 inline" />
                          </button>
                        )}

                        {(auction.status === 'live' || auction.status === 'paused') && hasPermission('auctions.finalize') && (
                          <button onClick={() => handleAction(auction.id, 'finalize')} className="text-purple-600 hover:text-purple-900" title="Завершить">
                            <CheckCircle className="h-4 w-4 inline" />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

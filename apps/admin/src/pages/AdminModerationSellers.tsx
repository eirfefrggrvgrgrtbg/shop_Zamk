import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Store,
  ExternalLink,
  Search,
  AlertCircle,
  Clock,
} from 'lucide-react';
import { getAdminSellers } from '@zamk/api-client/src/admin';

const getStatusBadge = (status: string) => {
  switch (status) {
    case 'pending_setup':
      return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200 dark:border-amber-800/60">Настройка магазина</span>;
    case 'pending_review':
      return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300 border border-blue-200 dark:border-blue-800/60">На проверке</span>;
    case 'pending':
      return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-100 text-indigo-800 dark:bg-indigo-950/60 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800/60">Ожидает решения</span>;
    default:
      return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">{status}</span>;
  }
};

export function AdminModerationSellers() {
  const [sellers, setSellers] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters for actionable states only
  const [statusFilter, setStatusFilter] = useState<'all_pending' | 'pending_review' | 'pending' | 'pending_setup'>('all_pending');
  const [searchQuery, setSearchQuery] = useState('');

  const loadSellers = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const res = await getAdminSellers({ limit: 100 });
      const allItems = res?.items || [];
      // Only keep actionable seller moderation statuses
      const actionable = allItems.filter(
        (s: any) => s.status === 'pending_setup' || s.status === 'pending_review' || s.status === 'pending'
      );
      setSellers(actionable);
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить список продавцов.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadSellers();
  }, []);

  const filteredSellers = sellers.filter((s) => {
    if (statusFilter === 'pending_review' && s.status !== 'pending_review') return false;
    if (statusFilter === 'pending' && s.status !== 'pending') return false;
    if (statusFilter === 'pending_setup' && s.status !== 'pending_setup') return false;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      const matchBrand = s.brandName?.toLowerCase().includes(q);
      const matchOwner = s.ownerName?.toLowerCase().includes(q);
      const matchEmail = s.ownerEmail?.toLowerCase().includes(q);
      if (!matchBrand && !matchOwner && !matchEmail) return false;
    }

    return true;
  });

  return (
    <div className="space-y-6">
      {/* Breadcrumb Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white dark:bg-slate-900 p-6 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs">
        <div>
          <div className="flex items-center gap-2 text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-1">
            <span>Модерация</span>
            <span>/</span>
            <span className="text-indigo-600 dark:text-indigo-400">Продавцы</span>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Продавцы на модерации</h1>
          <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
            Очередь продавцов на проверку документов, досье и активацию в каталоге
          </p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded-xl flex items-center gap-3">
          <AlertCircle className="h-5 w-5 flex-shrink-0" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* Filter Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-white dark:bg-slate-900 p-4 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs">
        <div className="flex items-center gap-1.5 overflow-x-auto scrollbar-hide py-0.5">
          <button
            type="button"
            onClick={() => setStatusFilter('all_pending')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'all_pending'
                ? 'bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-semibold shadow-xs'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Все ожидающие ({sellers.length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('pending_review')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'pending_review'
                ? 'bg-blue-600 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            На проверке ({sellers.filter((s) => s.status === 'pending_review').length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('pending')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'pending'
                ? 'bg-indigo-600 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Ожидают решения ({sellers.filter((s) => s.status === 'pending').length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('pending_setup')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'pending_setup'
                ? 'bg-amber-500 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Настройка магазина ({sellers.filter((s) => s.status === 'pending_setup').length})
          </button>
        </div>

        <div className="relative min-w-[240px]">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Поиск по бренду, владельцу, email..."
            className="w-full pl-9 pr-4 py-1.5 text-xs bg-gray-50 dark:bg-slate-800 border border-gray-200 dark:border-slate-700 rounded-xl text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
      </div>

      {/* Sellers List */}
      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-24 rounded-2xl bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-800 animate-pulse p-4" />
          ))}
        </div>
      ) : filteredSellers.length === 0 ? (
        <div className="bg-white dark:bg-slate-900 rounded-2xl p-12 text-center border border-gray-200 dark:border-slate-800">
          <Store className="w-12 h-12 text-gray-300 dark:text-slate-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-gray-800 dark:text-slate-200">Нет продавцов в этой категории</h3>
          <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
            Все продавцы проверены или нет заявок с выбранными фильтрами.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredSellers.map((seller) => (
            <div
              key={seller.id}
              className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-2xs flex flex-col md:flex-row md:items-center justify-between gap-4"
            >
              <div className="space-y-1.5">
                <div className="flex items-center gap-2">
                  <h3 className="text-base font-bold text-gray-900 dark:text-white">
                    {seller.brandName || seller.ownerName || 'Новый магазин'}
                  </h3>
                  {getStatusBadge(seller.status)}
                </div>

                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-slate-400">
                  {seller.ownerEmail && (
                    <span>Email: <span className="font-medium text-gray-700 dark:text-slate-300">{seller.ownerEmail}</span></span>
                  )}
                  {seller.ownerName && (
                    <span>Владелец: <span className="font-medium text-gray-700 dark:text-slate-300">{seller.ownerName}</span></span>
                  )}
                  {seller.contactPhone && (
                    <span>Телефон: <span className="font-medium text-gray-700 dark:text-slate-300">{seller.contactPhone}</span></span>
                  )}
                  {seller.createdAt && (
                    <span className="flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {new Date(seller.createdAt).toLocaleDateString('ru-RU')}
                    </span>
                  )}
                </div>
              </div>

              {/* Action Button linking to seller dossier */}
              <div className="flex items-center gap-2 shrink-0">
                <Link
                  to={`/sellers/${seller.id}`}
                  className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold rounded-xl transition-colors shadow-xs"
                >
                  <span>Перейти к проверке продавца</span>
                  <ExternalLink className="w-3.5 h-3.5" />
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

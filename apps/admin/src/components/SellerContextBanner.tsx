import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate, useLocation } from 'react-router-dom';
import { getAdminSellerDetail } from '@zamk/api-client/src/admin';
import { Store, X, ExternalLink } from 'lucide-react';

interface SellerContextBannerProps {
  onClearFilter?: () => void;
}

export function SellerContextBanner({ onClearFilter }: SellerContextBannerProps) {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const sellerId = searchParams.get('sellerId');
  const [brandName, setBrandName] = useState<string | null>(null);

  useEffect(() => {
    if (sellerId) {
      getAdminSellerDetail(sellerId)
        .then((s) => setBrandName(s.brandName || s.owner?.name || 'Продавец'))
        .catch(() => setBrandName('Продавец'));
    } else {
      setBrandName(null);
    }
  }, [sellerId]);

  if (!sellerId) return null;

  const handleClear = () => {
    if (onClearFilter) {
      onClearFilter();
    } else {
      const params = new URLSearchParams(searchParams);
      params.delete('sellerId');
      setSearchParams(params);
    }
  };

  return (
    <div className="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 p-3 rounded-2xl flex flex-wrap items-center justify-between gap-2 text-xs text-blue-900 dark:text-blue-200 mb-4 shadow-sm animate-in fade-in">
      <div className="flex items-center space-x-2">
        <Store className="w-4 h-4 text-blue-600 dark:text-blue-400 shrink-0" />
        <span>
          Фильтр по продавцу: <strong className="font-bold text-gray-900 dark:text-white">{brandName || 'Загрузка...'}</strong>
        </span>
      </div>

      <div className="flex items-center space-x-2">
        <button
          onClick={() => navigate(`/sellers/${sellerId}?tab=catalog`, { state: { returnTo: location.pathname + location.search } })}
          className="ml-4 inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-100 hover:bg-indigo-200 text-indigo-700 text-xs font-medium rounded-lg transition-colors"
        >
          <span>Вернуться в досье</span>
          <ExternalLink className="w-3 h-3" />
        </button>

        <button
          onClick={handleClear}
          className="inline-flex items-center space-x-1 px-3 py-1 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-700 font-semibold rounded-xl transition-colors"
        >
          <span>Снять фильтр</span>
          <X className="w-3 h-3" />
        </button>
      </div>
    </div>
  );
}

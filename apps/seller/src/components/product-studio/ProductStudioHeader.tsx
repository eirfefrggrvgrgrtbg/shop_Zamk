import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioViewToggle } from './ProductStudioViewToggle';
import { statusLabels } from '../../lib/seller-products';

export function ProductStudioHeader() {
  const { entryMode, draft } = useProductStudio();

  const isCreate = entryMode === 'create';
  const displayTitle = draft.title?.trim()
    ? draft.title
    : isCreate
    ? 'Новый товар'
    : 'Редактирование товара';

  const statusText = isCreate
    ? 'Черновик'
    : draft.status && (statusLabels as Record<string, string>)[draft.status]
    ? (statusLabels as Record<string, string>)[draft.status]
    : 'Редактирование';

  const subtitle = isCreate
    ? 'Черновик карточки товара'
    : draft.brandName
    ? `Бренд: ${draft.brandName}`
    : 'Карточка товара';

  return (
    <header
      data-testid="product-studio-header"
      className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between py-2 border-b border-gray-200 dark:border-white/10 pb-4"
    >
      <div className="flex items-center gap-3 min-w-0">
        <Link
          to="/products"
          data-testid="studio-back-to-products"
          aria-label="Вернуться к списку товаров"
          className="inline-flex items-center justify-center h-9 w-9 rounded-lg border border-gray-200 dark:border-white/10 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-white/5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
        >
          <ArrowLeft className="w-4 h-4" />
        </Link>

        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h1
              data-testid="studio-product-title"
              className="text-lg sm:text-xl font-bold text-gray-900 dark:text-white truncate"
            >
              {displayTitle}
            </h1>
            <span
              data-testid="studio-entry-badge"
              className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-700 dark:bg-white/10 dark:text-gray-300 shrink-0"
            >
              {statusText}
            </span>
          </div>
          <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5" data-testid="studio-header-subtitle">
            {subtitle}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-3 shrink-0">
        <ProductStudioViewToggle />
      </div>
    </header>
  );
}

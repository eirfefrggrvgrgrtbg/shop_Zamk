import { Link } from 'react-router-dom';
import { ArrowLeft, Save, Eye, Folder } from 'lucide-react';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioViewToggle } from './ProductStudioViewToggle';
import { statusLabels } from '../../lib/seller-products';

export function ProductStudioHeader() {
  const {
    entryMode,
    draft,
    setCategoryModalOpen,
    readiness,
    isFieldAttention,
    showReadinessAttention,
    setShowReadinessAttention,
  } = useProductStudio();
  const isCategoryAttention = isFieldAttention('category');

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
    ? null
    : draft.brandName
    ? `Бренд: ${draft.brandName}`
    : null;

  return (
    <header
      data-testid="product-studio-header"
      className="sticky top-[var(--seller-shell-header-height,56px)] z-20 w-full border-b border-gray-200 dark:border-white/10 bg-white dark:bg-[#121214] shadow-sm"
    >
      <div className="max-w-[1360px] mx-auto px-4 h-[56px] flex items-center justify-between">
        {/* LEFT: Context & Title */}
        <div className="flex items-center gap-4 min-w-0 flex-1">
          <Link
            to="/products"
            data-testid="studio-back-to-products"
            aria-label="Вернуться к списку товаров"
            className="flex items-center gap-2 text-sm font-medium text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
          >
            <ArrowLeft className="w-4 h-4" />
            <span className="hidden sm:inline">Ассортимент</span>
          </Link>

          <div className="h-4 w-px bg-gray-200 dark:bg-white/10 mx-2 hidden sm:block" />

          <div className="flex items-center gap-3 min-w-0">
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <h1
                  data-testid="studio-product-title"
                  className="text-sm font-semibold text-gray-900 dark:text-white truncate max-w-[200px] sm:max-w-md"
                >
                  {displayTitle}
                </h1>
                <span
                  data-testid="studio-entry-badge"
                  className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-gray-100 text-gray-700 dark:bg-white/10 dark:text-gray-300 shrink-0"
                >
                  {statusText}
                </span>

                <button
                  type="button"
                  data-testid="studio-readiness-summary"
                  onClick={() => setShowReadinessAttention(!showReadinessAttention)}
                  title="Нажмите, чтобы подсветить незаполненные обязательные поля"
                  className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-gray-100 text-gray-700 dark:bg-white/10 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/15 transition-colors cursor-pointer shrink-0"
                >
                  {readiness.blockingFields.length > 0
                    ? `Нужно заполнить: ${readiness.blockingFields.length}`
                    : 'Готов к сохранению'}
                </button>

                <button
                  type="button"
                  onClick={() => setCategoryModalOpen(true)}
                  data-testid="studio-header-category-btn"
                  title={draft.categoryName ? 'Изменить категорию' : 'Выбрать категорию товара'}
                  className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium transition-colors cursor-pointer shrink-0 ${
                    draft.categoryId
                      ? 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/15'
                      : isCategoryAttention
                      ? 'bg-amber-50 dark:bg-amber-950/30 text-amber-700 dark:text-amber-400 border border-amber-300 dark:border-amber-700/50 hover:bg-amber-100 dark:hover:bg-amber-900/40'
                      : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-white/10 hover:bg-gray-200 dark:hover:bg-white/15'
                  }`}
                >
                  <Folder className="w-3 h-3" />
                  <span>
                    {draft.categoryName ? `Категория · ${draft.categoryName}` : 'Категория * · Не выбрана'}
                  </span>
                </button>
              </div>
              {subtitle ? (
                <p className="text-[11px] text-gray-500 dark:text-gray-400 truncate hidden lg:block" data-testid="studio-header-subtitle">
                  {subtitle}
                </p>
              ) : null}
            </div>
          </div>
        </div>

        {/* CENTER: Mode Switcher */}
        <div className="flex-shrink-0 flex items-center justify-center">
          <ProductStudioViewToggle />
        </div>

        {/* RIGHT: Actions */}
        <div className="flex items-center justify-end gap-3 flex-1">
          <button
            type="button"
            disabled
            title="Модерация временно недоступна"
            className="hidden sm:inline-flex items-center justify-center gap-2 px-3 py-1.5 text-sm font-medium text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-[#1a1a1c] border border-gray-200 dark:border-white/5 rounded-lg cursor-not-allowed focus-visible:outline-none"
          >
            <Eye className="w-4 h-4" />
            Отправить на модерацию
          </button>
          <button
            type="button"
            disabled
            title="Сохранение временно недоступно"
            className="inline-flex items-center justify-center gap-2 px-4 py-1.5 text-sm font-medium text-white/50 bg-indigo-400 dark:bg-indigo-900 rounded-lg cursor-not-allowed focus-visible:outline-none"
          >
            <Save className="w-4 h-4" />
            <span className="hidden sm:inline">Сохранить</span>
          </button>
        </div>
      </div>
    </header>
  );
}

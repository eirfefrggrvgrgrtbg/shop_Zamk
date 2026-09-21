import { Link } from 'react-router-dom';
import { ArrowLeft, Save, Eye, Folder, Loader2, AlertCircle } from 'lucide-react';
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
    saveStatus,
    saveError,
    canSave,
    saveDraft,
    clearSaveError,
    isSaveInFlight,
    isDirty,
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

  const isSaveAvailable = entryMode === 'edit' && Boolean(saveDraft);

  let saveButtonText = 'Сохранить';
  if (saveStatus === 'staging') {
    saveButtonText = 'Загрузка фото…';
  } else if (saveStatus === 'saving') {
    saveButtonText = 'Сохранение…';
  }

  let saveButtonTitle = 'Сохранить';
  if (!isSaveAvailable) {
    saveButtonTitle = 'Сохранение временно недоступно';
  } else if (saveStatus === 'staging') {
    saveButtonTitle = 'Выполняется загрузка фотографий…';
  } else if (saveStatus === 'saving') {
    saveButtonTitle = 'Выполняется сохранение товара…';
  } else if (saveStatus === 'refresh_error') {
    saveButtonTitle = 'Товар сохранен. Обновите страницу';
  } else if (!isDirty) {
    saveButtonTitle = 'Нет несохраненных изменений';
  } else if (saveStatus === 'error') {
    saveButtonTitle = saveError || 'Ошибка сохранения. Нажмите, чтобы повторить';
  } else {
    saveButtonTitle = 'Сохранить изменения';
  }

  return (
    <header
      data-testid="product-studio-header"
      className="sticky top-[var(--seller-shell-header-height,56px)] z-20 w-full border-b border-gray-200 dark:border-white/10 bg-white dark:bg-[#121214] shadow-sm"
    >
      <div className="max-w-[1360px] mx-auto px-4 min-h-[56px] py-2 flex items-center justify-between gap-4">
        {/* LEFT: Context & Title */}
        <div className="flex items-center gap-4 min-w-0 flex-1">
          <Link
            to="/products"
            data-testid="studio-back-to-products"
            aria-label="Вернуться к списку товаров"
            className="flex items-center gap-2 text-sm font-medium text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 shrink-0"
          >
            <ArrowLeft className="w-4 h-4" />
            <span className="hidden sm:inline">Ассортимент</span>
          </Link>

          <div className="h-6 w-px bg-gray-200 dark:bg-white/10 mx-1 hidden sm:block shrink-0" />

          <div className="flex flex-col gap-1 min-w-0 flex-1">
            {/* Row 1: Product title + Status badges */}
            <div className="flex items-center gap-2.5 min-w-0 flex-wrap">
              <h1
                data-testid="studio-product-title"
                title={displayTitle}
                className="text-sm font-semibold text-gray-900 dark:text-white truncate min-w-0 max-w-[180px] sm:max-w-[240px] md:max-w-[300px] lg:max-w-[360px] xl:max-w-[420px]"
              >
                {displayTitle}
              </h1>

              <div className="flex items-center gap-1.5 shrink-0 flex-wrap">
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
                  title={
                    readiness.blockingFields.length > 0
                      ? 'Нажмите, чтобы подсветить незаполненные обязательные поля'
                      : isDirty
                      ? 'Все обязательные поля заполнены, изменения готовы к сохранению'
                      : 'Все обязательные поля заполнены'
                  }
                  className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-gray-100 text-gray-700 dark:bg-white/10 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/15 transition-colors cursor-pointer shrink-0"
                >
                  {readiness.blockingFields.length > 0
                    ? `Нужно заполнить: ${readiness.blockingFields.length}`
                    : isDirty
                    ? 'Готов к сохранению'
                    : 'Все поля заполнены'}
                </button>
              </div>
            </div>

            {/* Row 2: Category chip */}
            <div className="flex items-center min-w-0">
              <button
                type="button"
                onClick={() => setCategoryModalOpen(true)}
                disabled={isSaveInFlight}
                data-testid="studio-header-category-btn"
                title={draft.categoryName ? 'Изменить категорию' : 'Выбрать категорию товара'}
                className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium transition-colors shrink-0 max-w-full truncate ${
                  isSaveInFlight ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'
                } ${
                  draft.categoryId
                    ? 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-white/15'
                    : isCategoryAttention
                    ? 'bg-amber-50 dark:bg-amber-950/30 text-amber-700 dark:text-amber-400 border border-amber-300 dark:border-amber-700/50 hover:bg-amber-100 dark:hover:bg-amber-900/40'
                    : 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-white/10 hover:bg-gray-200 dark:hover:bg-white/15'
                }`}
              >
                <Folder className="w-3 h-3 shrink-0" />
                <span className="truncate">
                  {draft.categoryName ? `Категория · ${draft.categoryName}` : 'Категория * · Не выбрана'}
                </span>
              </button>
            </div>

            {/* Row 3: Brand line */}
            {subtitle ? (
              <div className="flex items-center min-w-0">
                <p
                  className="text-[11px] text-gray-500 dark:text-gray-400 truncate"
                  data-testid="studio-header-subtitle"
                  title={subtitle}
                >
                  {subtitle}
                </p>
              </div>
            ) : null}
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
            disabled={!canSave}
            onClick={canSave ? saveDraft : undefined}
            data-testid="studio-header-save-btn"
            title={saveButtonTitle}
            className={`inline-flex items-center justify-center gap-2 px-4 py-1.5 text-sm font-medium rounded-lg focus-visible:outline-none transition-colors ${
              canSave
                ? 'text-white bg-indigo-600 hover:bg-indigo-700 cursor-pointer shadow-sm active:bg-indigo-800'
                : 'text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-white/5 border border-gray-200 dark:border-white/10 cursor-not-allowed'
            }`}
          >
            {isSaveInFlight ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Save className="w-4 h-4" />
            )}
            <span className="hidden sm:inline">{saveButtonText}</span>
          </button>
        </div>
      </div>

      {saveError && (
        <div
          data-testid="studio-save-error-toast"
          role="alert"
          className="fixed bottom-5 right-5 z-50 flex items-center gap-3 rounded-xl bg-red-600 px-4 py-3 text-sm font-medium text-white shadow-xl animate-in fade-in slide-in-from-bottom-2"
        >
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{saveError}</span>
          {saveStatus === 'refresh_error' ? (
            <button
              type="button"
              data-testid="studio-refresh-page-btn"
              onClick={() => window.location.reload()}
              className="ml-2 underline font-semibold text-white hover:text-white/80 cursor-pointer text-xs"
            >
              Обновить страницу
            </button>
          ) : (
            <button
              type="button"
              onClick={clearSaveError}
              aria-label="Закрыть уведомление"
              className="ml-2 text-white/80 hover:text-white cursor-pointer"
            >
              ✕
            </button>
          )}
        </div>
      )}
    </header>
  );
}

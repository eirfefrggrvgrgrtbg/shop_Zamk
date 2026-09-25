import { Link } from 'react-router-dom';
import { ArrowLeft, Save, Eye, Folder, Loader2, AlertCircle } from 'lucide-react';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioViewToggle } from './ProductStudioViewToggle';
import { statusLabels } from '../../lib/seller-products';
import { isUuid } from './productStudioPresentationAdapter';

export function ProductStudioHeader() {
  const {
    entryMode,
    draft,
    setCategoryModalOpen,
    categorySchema,
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
    markTouched,
  } = useProductStudio();
  const isCategoryAttention = isFieldAttention('category');
  const hasCategory = Boolean(draft.categoryId);
  const catCandidate = (
    draft.categoryPath && !isUuid(draft.categoryPath.trim())
      ? draft.categoryPath
      : draft.categoryName && !isUuid(draft.categoryName.trim())
      ? draft.categoryName
      : categorySchema?.name && !isUuid(categorySchema.name.trim())
      ? categorySchema.name
      : ''
  ).trim();
  const displayCategory = catCandidate;
  const categoryLabelText = hasCategory
    ? displayCategory || 'Категория выбрана'
    : 'Выберите категорию товара';

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

  const isSaveAvailable = Boolean(saveDraft);

  let saveButtonText = 'Сохранить';
  if (saveStatus === 'staging') {
    saveButtonText = 'Загрузка фото…';
  } else if (saveStatus === 'saving') {
    saveButtonText = 'Сохранение…';
  } else if (saveStatus === 'identity_recovery_required') {
    saveButtonText = 'Повторить попытку';
  }

  let saveButtonTitle = 'Сохранить';
  if (!isSaveAvailable) {
    saveButtonTitle = 'Сохранение временно недоступно';
  } else if (saveStatus === 'staging') {
    saveButtonTitle = 'Выполняется загрузка фотографий…';
  } else if (saveStatus === 'saving') {
    saveButtonTitle = 'Выполняется сохранение товара…';
  } else if (saveStatus === 'identity_recovery_required') {
    saveButtonTitle = saveError || 'Результат создания товара не подтвержден. Нажмите, чтобы повторить попытку';
  } else if (saveStatus === 'refresh_error') {
    saveButtonTitle = 'Товар сохранен. Обновите страницу';
  } else if (saveStatus === 'error') {
    saveButtonTitle = saveError || 'Ошибка сохранения. Нажмите, чтобы повторить';
  } else if (entryMode === 'edit' && !isDirty) {
    saveButtonTitle = 'Нет несохраненных изменений';
  } else if (!canSave) {
    saveButtonTitle = 'Заполните обязательные поля для сохранения';
  } else {
    saveButtonTitle = isCreate ? 'Сохранить товар' : 'Сохранить изменения';
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

            {/* Row 2: Prominent Global Category Control */}
            <div className="flex items-center min-w-0 my-0.5" data-testid="studio-global-category-row">
              <div
                data-testid="studio-global-category-control"
                className={`inline-flex items-center justify-between gap-2.5 px-3 py-1 rounded-lg border text-xs transition-colors max-w-full truncate ${
                  !hasCategory
                    ? isCategoryAttention
                      ? 'border-amber-300 dark:border-amber-700 bg-amber-50 dark:bg-amber-950/30 text-amber-900 dark:text-amber-200 ring-1 ring-amber-300/50'
                      : 'border-amber-200 dark:border-amber-800/60 bg-amber-50/60 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300'
                    : 'border-gray-200 dark:border-white/10 bg-gray-50/80 dark:bg-white/[0.04] text-gray-800 dark:text-gray-200'
                }`}
              >
                <div className="flex items-center gap-1.5 min-w-0 truncate">
                  <Folder
                    className={`w-3.5 h-3.5 shrink-0 ${
                      !hasCategory ? 'text-amber-600 dark:text-amber-400' : 'text-gray-400 dark:text-gray-400'
                    }`}
                  />
                  <span className="text-[11px] font-medium text-gray-500 dark:text-gray-400 shrink-0">
                    Категория товара{!hasCategory ? ' *' : ''}
                  </span>
                  <span className="text-gray-300 dark:text-gray-600 shrink-0">·</span>
                  <span
                    data-testid="studio-header-category-label"
                    title={categoryLabelText}
                    className={`text-xs truncate max-w-[200px] sm:max-w-[280px] md:max-w-[360px] ${
                      !hasCategory
                        ? 'text-amber-700 dark:text-amber-400 font-normal italic'
                        : 'font-semibold text-gray-900 dark:text-white'
                    }`}
                  >
                    {categoryLabelText}
                  </span>
                </div>

                <button
                  type="button"
                  onClick={() => {
                    markTouched('category');
                    setCategoryModalOpen(true);
                  }}
                  disabled={isSaveInFlight}
                  data-testid="studio-header-category-btn"
                  title={hasCategory ? 'Изменить категорию' : 'Выбрать категорию товара'}
                  className={`px-2.5 py-0.5 rounded text-xs font-semibold transition-all shrink-0 ml-1 ${
                    isSaveInFlight
                      ? 'opacity-50 cursor-not-allowed'
                      : !hasCategory
                      ? 'bg-amber-600 hover:bg-amber-700 text-white shadow-xs cursor-pointer'
                      : 'bg-white dark:bg-white/10 text-gray-700 dark:text-gray-200 border border-gray-200 dark:border-white/10 hover:bg-gray-100 dark:hover:bg-white/15 cursor-pointer'
                  }`}
                >
                  {hasCategory ? 'Изменить' : 'Выбрать'}
                </button>
              </div>
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
          ) : saveStatus === 'identity_recovery_required' ? (
            <button
              type="button"
              data-testid="studio-identity-retry-btn"
              onClick={() => saveDraft && saveDraft()}
              className="ml-2 underline font-semibold text-white hover:text-white/80 cursor-pointer text-xs"
            >
              Повторить попытку
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

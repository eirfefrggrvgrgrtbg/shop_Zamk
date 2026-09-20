import { useState, useEffect, useMemo } from 'react';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioSectionNav } from './ProductStudioSectionNav';
import { SellerSurface } from '../SellerSurface';
import { FileText, Image, Sliders, Layers, DollarSign, ShieldCheck, Folder } from 'lucide-react';
import { isColorRequired, isSizeRequired, getSizeChartCompleteness, getOfferedSizes, getCompositionCompleteness } from './productStudioReadinessHelper';
import { MIN_PRODUCT_IMAGES } from './productStudioMediaHelper';
import { getSellerCategorySchema, type SellerCategorySchema } from '@zamk/api-client';
import { ProductStudioCompositionModal } from './ProductStudioCompositionModal';
import { ProductStudioCareModal } from './ProductStudioCareModal';
import { ProductStudioCharacteristicsModal } from './ProductStudioCharacteristicsModal';
import { ProductStudioSizeChartModal } from './ProductStudioSizeChartModal';

export function ProductStudioFormWorkspace() {
  const {
    activeSection,
    draft,
    updateDraft,
    setCategoryModalOpen,
    readiness,
    markTouched,
    isFieldAttention,
    setShowReadinessAttention,
  } = useProductStudio();
  const [categorySchema, setCategorySchema] = useState<SellerCategorySchema | null>(null);
  const [isCompositionModalOpen, setIsCompositionModalOpen] = useState(false);
  const [isCareModalOpen, setIsCareModalOpen] = useState(false);
  const [isCharacteristicsModalOpen, setIsCharacteristicsModalOpen] = useState(false);
  const [isSizeChartModalOpen, setIsSizeChartModalOpen] = useState(false);

  useEffect(() => {
    if (activeSection === 'review') {
      setShowReadinessAttention(true);
    }
  }, [activeSection, setShowReadinessAttention]);

  useEffect(() => {
    if (!draft.categoryId) {
      setCategorySchema(null);
      return;
    }
    let isMounted = true;
    getSellerCategorySchema(draft.categoryId)
      .then((schema) => {
        if (isMounted) setCategorySchema(schema);
      })
      .catch((err) => {
        console.error('Failed to load schema in Form Workspace:', err);
      });
    return () => {
      isMounted = false;
    };
  }, [draft.categoryId]);

  const colorNeeded = isColorRequired(draft, categorySchema);
  const sizeNeeded = isSizeRequired(draft, categorySchema);

  const isCompositionMissing =
    readiness?.blockingFields?.includes('composition') ?? !getCompositionCompleteness(draft).isComplete;
  const offeredSizes = useMemo(() => getOfferedSizes(draft), [draft]);
  const sizeChartCompleteness = useMemo(() => {
    return getSizeChartCompleteness(draft, categorySchema);
  }, [draft, categorySchema]);
  const isSizeChartMissing = readiness?.blockingFields?.includes('sizeChart') ?? !sizeChartCompleteness.isComplete;

  const isTitleAttention = isFieldAttention('title');
  const isCategoryAttention = isFieldAttention('category');
  const isPriceAttention = isFieldAttention('price');
  const isMediaAttention = isFieldAttention('media');
  const isColorAttention = isFieldAttention('color');
  const isSizeAttention = isFieldAttention('size');
  const isDescriptionAttention = isFieldAttention('description');
  const isCompositionAttention = isFieldAttention('composition');
  const isCharacteristicsAttention = isFieldAttention('characteristics');
  const isSizeChartAttention = isFieldAttention('sizeChart');

  const requiredProductAttrs = useMemo(() => {
    return categorySchema?.attributes?.filter((a) => a.scope === 'PRODUCT' && a.required) || [];
  }, [categorySchema]);

  const filledRequiredCharacteristicsCount = useMemo(() => {
    if (requiredProductAttrs.length === 0) return 0;
    return requiredProductAttrs.filter((reqAttr) => {
      const found = draft.attributes?.find(
        (a) =>
          a.attributeDefinitionId === reqAttr.id ||
          (reqAttr.nameRu && a.name === reqAttr.nameRu)
      );
      if (!found) return false;
      if (found.dictionaryValueId) return true;
      if (typeof found.value === 'string') return found.value.trim().length > 0;
      if (typeof found.value === 'number') return !isNaN(found.value);
      if (typeof found.value === 'boolean') return true;
      return false;
    }).length;
  }, [requiredProductAttrs, draft.attributes]);


  return (
    <div
      id="studio-workspace-form"
      role="tabpanel"
      aria-labelledby="studio-tab-form"
      data-testid="studio-form-workspace"
      className="max-w-[1360px] mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6"
    >
      <ProductStudioSectionNav />

      <SellerSurface variant="primary" className="p-6">
        {activeSection === 'basics' && (
          <div
            id="studio-section-panel-basics"
            role="tabpanel"
            aria-labelledby="studio-section-tab-basics"
            data-testid="studio-section-panel-basics"
            className="space-y-6"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <FileText className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Основное
              </h2>
            </div>

            <div className="max-w-2xl space-y-5">
              <div>
                <label
                  htmlFor="form-product-title-input"
                  className={`block text-xs font-medium mb-1 ${
                    isTitleAttention
                      ? 'text-amber-800 dark:text-amber-300'
                      : 'text-gray-700 dark:text-gray-300'
                  }`}
                >
                  Название товара <span className={isTitleAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                </label>
                <input
                  id="form-product-title-input"
                  data-testid="form-product-title-input"
                  type="text"
                  value={draft.title || ''}
                  onChange={(e) => updateDraft({ title: e.target.value })}
                  onBlur={() => markTouched('title')}
                  placeholder="Например: Рубашка оверсайз из плотного хлопка"
                  className={`w-full px-3 py-2 text-sm rounded-lg bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 ${
                    isTitleAttention
                      ? 'border border-amber-400 dark:border-amber-600 focus:ring-amber-500'
                      : 'border border-gray-200 dark:border-white/10 focus:ring-indigo-500'
                  }`}
                />
                {isTitleAttention && (
                  <p data-testid="form-title-required-helper" className="text-[11px] text-amber-700 dark:text-amber-400 mt-1">Укажите название</p>
                )}
              </div>

              <div>
                <label
                  className={`block text-xs font-medium mb-1 ${
                    isCategoryAttention
                      ? 'text-amber-800 dark:text-amber-300'
                      : 'text-gray-700 dark:text-gray-300'
                  }`}
                >
                  Категория <span className={isCategoryAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                </label>
                <div
                  className={`flex items-center justify-between p-3 rounded-lg border ${
                    isCategoryAttention
                      ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/30 dark:bg-amber-950/10'
                      : 'border border-gray-200 dark:border-white/10 bg-gray-50/50 dark:bg-white/[0.02]'
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <Folder
                      className={`w-4 h-4 shrink-0 ${
                        isCategoryAttention ? 'text-amber-600 dark:text-amber-400' : 'text-gray-400'
                      }`}
                    />
                    <span
                      className={`text-sm font-medium truncate ${
                        isCategoryAttention
                          ? 'text-amber-800 dark:text-amber-300'
                          : 'text-gray-900 dark:text-white'
                      }`}
                    >
                      {draft.categoryName || 'Категория не выбрана'}
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      markTouched('category');
                      setCategoryModalOpen(true);
                    }}
                    data-testid="form-select-category-btn"
                    className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors shrink-0 ml-3 ${
                      isCategoryAttention
                        ? 'bg-amber-50 dark:bg-amber-950/30 text-amber-800 dark:text-amber-300 border border-amber-300 dark:border-amber-800 hover:bg-amber-100'
                        : 'bg-white dark:bg-white/10 text-gray-800 dark:text-white border border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20'
                    }`}
                  >
                    {draft.categoryId ? 'Изменить категорию' : 'Выбрать категорию'}
                  </button>
                </div>
                {isCategoryAttention && (
                  <p data-testid="form-category-required-helper" className="text-[11px] text-amber-700 dark:text-amber-400 mt-1">Выберите категорию</p>
                )}
              </div>

              <div>
                <label
                  htmlFor="form-product-description-input"
                  className={`block text-xs font-medium mb-1 ${
                    isDescriptionAttention
                      ? 'text-amber-800 dark:text-amber-300'
                      : 'text-gray-700 dark:text-gray-300'
                  }`}
                >
                  Описание <span className={isDescriptionAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                </label>
                <textarea
                  id="form-product-description-input"
                  data-testid="form-product-description-input"
                  rows={4}
                  value={draft.description || ''}
                  onChange={(e) => updateDraft({ description: e.target.value })}
                  onBlur={() => markTouched('description')}
                  placeholder="Опишите особенности товара, посадку и важные детали"
                  className={`w-full px-3 py-2 text-sm rounded-lg bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 ${
                    isDescriptionAttention
                      ? 'border border-amber-400 dark:border-amber-600 focus:ring-amber-500'
                      : 'border border-gray-200 dark:border-white/10 focus:ring-indigo-500'
                  }`}
                />
                <p
                  data-testid="form-description-helper"
                  className={`text-[11px] mt-1 ${
                    isDescriptionAttention ? 'text-amber-700 dark:text-amber-400 font-medium' : 'text-gray-500 dark:text-gray-400'
                  }`}
                >
                  Опишите особенности товара, посадку и важные детали
                </p>
              </div>
            </div>
          </div>
        )}

        {activeSection === 'media' && (
          <div
            id="studio-section-panel-media"
            role="tabpanel"
            aria-labelledby="studio-section-tab-media"
            data-testid="studio-section-panel-media"
            className="space-y-4"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <Image className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Медиа <span className={isMediaAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
              </h2>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Минимум {MIN_PRODUCT_IMAGES} фото · JPG, PNG, WebP · до 10 МБ. Вертикальное фото · минимум 800×1000 px.
            </p>
            <p className="text-xs text-gray-400 dark:text-gray-500">
              Лучше использовать формат 4:5 — фото лучше заполняет карточку товара.
            </p>
            {isMediaAttention && (
              <div className="p-3 rounded-lg border border-amber-300 dark:border-amber-900/60 bg-amber-50/40 dark:bg-amber-950/20 text-xs text-amber-800 dark:text-amber-300 font-medium">
                Нужно минимум 3 фото (загружено: {(draft.images || []).length})
              </div>
            )}
          </div>
        )}

        {activeSection === 'characteristics' && (
          <div
            id="studio-section-panel-characteristics"
            role="tabpanel"
            aria-labelledby="studio-section-tab-characteristics"
            data-testid="studio-section-panel-characteristics"
            className="space-y-6"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <Sliders className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Характеристики и состав
              </h2>
            </div>

            {/* Composition subsection */}
            <div className={`max-w-2xl p-4 rounded-xl border space-y-3 ${
              isCompositionAttention
                ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                : 'border-gray-200 dark:border-white/10'
            }`}>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className={`text-sm font-semibold ${isCompositionAttention ? 'text-amber-800 dark:text-amber-300' : 'text-gray-900 dark:text-white'}`}>
                    Состав <span className={isCompositionAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                  </h3>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Укажите материалы и их доли (обязательно для публикации).
                  </p>
                </div>
                <button
                  type="button"
                  data-testid="form-edit-composition-btn"
                  onClick={() => setIsCompositionModalOpen(true)}
                  className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-colors ${
                    isCompositionAttention
                      ? 'bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100'
                      : 'bg-white dark:bg-white/10 text-gray-800 dark:text-white border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20'
                  }`}
                >
                  {isCompositionMissing ? 'Добавить состав' : 'Изменить состав'}
                </button>
              </div>

              {isCompositionAttention && (
                <p data-testid="form-composition-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium">
                  Укажите материалы и их доли
                </p>
              )}

              {draft.materialComposition && draft.materialComposition.length > 0 ? (
                <div className="text-xs text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-white/[0.02] p-2.5 rounded-lg space-y-1">
                  <span className="font-medium text-gray-900 dark:text-white">Текущий состав:</span>{' '}
                  {draft.materialComposition.map((mc) => `${mc.materialName || mc.material} — ${mc.percentage}%`).join(', ')}
                </div>
              ) : draft.material ? (
                <div className="text-xs text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-white/[0.02] p-2.5 rounded-lg">
                  <span className="font-medium text-gray-900 dark:text-white">Материал:</span> {draft.material}
                </div>
              ) : null}

              {/* Care instructions */}
              <div className="pt-2 border-t border-gray-100 dark:border-white/5 flex items-center justify-between">
                <div>
                  <h4 className="text-xs font-medium text-gray-700 dark:text-gray-300">
                    Уход за изделием <span className="text-gray-400">(необязательно)</span>
                  </h4>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    {draft.careInstructions?.trim() || 'Рекомендации по стирке и глажке не указаны'}
                  </p>
                </div>
                <button
                  type="button"
                  data-testid="form-edit-care-btn"
                  onClick={() => setIsCareModalOpen(true)}
                  className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline"
                >
                  {draft.careInstructions?.trim() ? 'Изменить' : 'Добавить'}
                </button>
              </div>
            </div>

            {/* Category attributes subsection */}
            <div className={`max-w-2xl p-4 rounded-xl border space-y-3 ${
              isCharacteristicsAttention
                ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                : 'border-gray-200 dark:border-white/10'
            }`}>
              <div className="flex items-center justify-between">
                <div>
                  <h3 className={`text-sm font-semibold ${isCharacteristicsAttention ? 'text-amber-800 dark:text-amber-300' : 'text-gray-900 dark:text-white'}`}>
                    Характеристики категории {requiredProductAttrs.length > 0 ? (
                      <span className={isCharacteristicsAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                    ) : ''}
                  </h3>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    {categorySchema
                      ? `Категория: ${categorySchema.name}. Заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length} обязательных.`
                      : 'Сначала выберите категорию товара.'}
                  </p>
                </div>
                <button
                  type="button"
                  data-testid="form-manage-characteristics-btn"
                  onClick={() => {
                    if (!draft.categoryId) {
                      markTouched('category');
                      setCategoryModalOpen(true);
                    } else {
                      setIsCharacteristicsModalOpen(true);
                    }
                  }}
                  className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-colors ${
                    isCharacteristicsAttention
                      ? 'bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100'
                      : 'bg-white dark:bg-white/10 text-gray-800 dark:text-white border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20'
                  }`}
                >
                  {!draft.categoryId
                    ? 'Выбрать категорию'
                    : isCharacteristicsAttention
                    ? 'Заполнить характеристики'
                    : requiredProductAttrs.length > 0
                    ? 'Изменить характеристики'
                    : 'Настроить характеристики'}
                </button>
              </div>

              {isCharacteristicsAttention && (
                <p data-testid="form-characteristics-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium">
                  Заполните обязательные характеристики
                </p>
              )}

              {draft.attributes && draft.attributes.length > 0 && (
                <div className="grid grid-cols-2 gap-2 pt-2 border-t border-gray-100 dark:border-white/5">
                  {draft.attributes.map((attr, idx) => (
                    <div key={idx} className="text-xs">
                      <span className="text-gray-500 dark:text-gray-400">{attr.name}: </span>
                      <span className="font-medium text-gray-900 dark:text-white">
                        {typeof attr.value === 'boolean' ? (attr.value ? 'Да' : 'Нет') : String(attr.value)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {activeSection === 'variants' && (
          <div
            id="studio-section-panel-variants"
            role="tabpanel"
            aria-labelledby="studio-section-tab-variants"
            data-testid="studio-section-panel-variants"
            className="space-y-4"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <Layers className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Варианты
              </h2>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {colorNeeded && sizeNeeded
                ? 'Для данной категории обязательны цвет * и размер *.'
                : colorNeeded
                ? 'Для данной категории обязателен цвет *.'
                : sizeNeeded
                ? 'Для данной категории обязателен размер *.'
                : 'Товар без разделения по цветам и размерам.'}
            </p>
            {isColorAttention && (
              <p className="text-xs text-amber-700 dark:text-amber-400 font-medium">
                Для данной категории необходимо выбрать хотя бы один цвет.
              </p>
            )}
            {isSizeAttention && (
              <p className="text-xs text-amber-700 dark:text-amber-400 font-medium">
                Для данной категории необходимо выбрать хотя бы один размер.
              </p>
            )}

            {sizeChartCompleteness.isNeeded && (
              <div className={`p-4 rounded-xl border space-y-2 mt-4 ${
                isSizeChartAttention
                  ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                  : 'border-gray-200 dark:border-white/10'
              }`}>
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className={`text-sm font-semibold ${isSizeChartAttention ? 'text-amber-800 dark:text-amber-300' : 'text-gray-900 dark:text-white'}`}>
                      Таблица размеров <span className={isSizeChartAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                    </h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {sizeChartCompleteness.isComplete
                        ? `Заполнено ${sizeChartCompleteness.requiredCellCount} из ${sizeChartCompleteness.requiredCellCount} обязательных мерок`
                        : sizeChartCompleteness.filledRequiredCellCount > 0
                        ? `Заполнено ${sizeChartCompleteness.filledRequiredCellCount} из ${sizeChartCompleteness.requiredCellCount} обязательных мерок`
                        : 'Таблица размеров не заполнена'}
                    </p>
                  </div>
                  <button
                    type="button"
                    data-testid="form-manage-size-chart-btn"
                    onClick={() => setIsSizeChartModalOpen(true)}
                    className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-colors ${
                      isSizeChartAttention
                        ? 'bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100'
                        : 'bg-white dark:bg-white/10 text-gray-800 dark:text-white border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20'
                    }`}
                  >
                    {isSizeChartMissing ? 'Настроить таблицу размеров' : 'Изменить таблицу размеров'}
                  </button>
                </div>
                {isSizeChartAttention && (
                  <p data-testid="form-size-chart-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium">
                    Для товаров с размерами обязательно заполнение таблицы размеров
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {activeSection === 'pricing' && (
          <div
            id="studio-section-panel-pricing"
            role="tabpanel"
            aria-labelledby="studio-section-tab-pricing"
            data-testid="studio-section-panel-pricing"
            className="space-y-6"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <DollarSign className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Цена <span className={isPriceAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
              </h2>
            </div>
            <div className="max-w-xs space-y-4">
              <div>
                <label
                  htmlFor="form-product-price-input"
                  className={`block text-xs font-medium mb-1 ${
                    isPriceAttention
                      ? 'text-amber-800 dark:text-amber-300'
                      : 'text-gray-700 dark:text-gray-300'
                  }`}
                >
                  Базовая цена (рубли) <span className={isPriceAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                </label>
                <input
                  id="form-product-price-input"
                  data-testid="form-product-price-input"
                  type="number"
                  min="0"
                  value={draft.priceCents !== undefined ? draft.priceCents / 100 : ''}
                  onChange={(e) => {
                    const val = e.target.value !== '' ? Math.round(parseFloat(e.target.value) * 100) : undefined;
                    updateDraft({ priceCents: val });
                  }}
                  onBlur={() => markTouched('price')}
                  placeholder="0 ₽"
                  className={`w-full px-3 py-2 text-sm rounded-lg bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 ${
                    isPriceAttention
                      ? 'border border-amber-400 dark:border-amber-600 focus:ring-amber-500'
                      : 'border border-gray-200 dark:border-white/10 focus:ring-indigo-500'
                  }`}
                />
                {isPriceAttention && (
                  <p data-testid="form-price-required-helper" className="text-[11px] text-amber-700 dark:text-amber-400 mt-1">Укажите цену</p>
                )}
              </div>
            </div>
          </div>
        )}

        {activeSection === 'review' && (
          <div
            id="studio-section-panel-review"
            role="tabpanel"
            aria-labelledby="studio-section-tab-review"
            data-testid="studio-section-panel-review"
            className="space-y-4"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <ShieldCheck className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Проверка
              </h2>
            </div>
            <div className="rounded-lg bg-gray-50 dark:bg-white/[0.02] border border-gray-200/80 dark:border-white/10 p-4 space-y-2">
              <p className="text-xs font-semibold text-gray-800 dark:text-gray-200">
                Назначение секции «Проверка»:
              </p>
              <ul className="text-xs text-gray-600 dark:text-gray-400 list-disc list-inside space-y-1">
                <li>Оценка полноты заполнения карточки (readiness);</li>
                <li>Выявление блокирующих ошибок для модерации;</li>
                <li>Предупреждения и подсказки;</li>
                <li>Отправка на модерацию.</li>
              </ul>
              <p className="text-xs text-indigo-600 dark:text-indigo-400 pt-1">
                Проверка готовности карточки.
              </p>
            </div>
          </div>
        )}
      </SellerSurface>

      <ProductStudioCompositionModal
        isOpen={isCompositionModalOpen}
        onClose={() => {
          setIsCompositionModalOpen(false);
          markTouched('composition');
        }}
        materialComposition={draft.materialComposition}
        onSave={({ materialComposition, material }) => {
          updateDraft({ materialComposition, material });
          markTouched('composition');
        }}
      />

      <ProductStudioCareModal
        isOpen={isCareModalOpen}
        onClose={() => setIsCareModalOpen(false)}
        careInstructions={draft.careInstructions}
        onSave={(careInstructions) => updateDraft({ careInstructions })}
      />

      <ProductStudioCharacteristicsModal
        isOpen={isCharacteristicsModalOpen}
        onClose={() => {
          setIsCharacteristicsModalOpen(false);
          markTouched('characteristics');
        }}
        schema={categorySchema}
        attributes={draft.attributes}
        onSave={(attributes) => {
          updateDraft({ attributes });
          markTouched('characteristics');
        }}
      />

      <ProductStudioSizeChartModal
        isOpen={isSizeChartModalOpen}
        onClose={() => {
          setIsSizeChartModalOpen(false);
          markTouched('sizeChart');
        }}
        schema={categorySchema}
        draftSizes={offeredSizes.map((s) => ({ id: s.sizeValueId, label: s.sizeValueName }))}
        sizeChart={draft.sizeChart}
        onSaveSizeChart={(sizeChart) => {
          updateDraft({ sizeChart });
          markTouched('sizeChart');
        }}
      />
    </div>
  );
}

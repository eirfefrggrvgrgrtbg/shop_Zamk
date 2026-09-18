import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioSectionNav } from './ProductStudioSectionNav';
import { SellerSurface } from '../SellerSurface';
import { FileText, Image, Sliders, Layers, DollarSign, ShieldCheck } from 'lucide-react';

export function ProductStudioFormWorkspace() {
  const { activeSection, draft, updateDraft } = useProductStudio();

  return (
    <div
      id="studio-workspace-form"
      role="tabpanel"
      aria-labelledby="studio-tab-form"
      data-testid="studio-form-workspace"
      className="space-y-6"
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

            <div className="max-w-2xl space-y-4">
              <div>
                <label
                  htmlFor="form-product-title-input"
                  className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
                >
                  Название товара
                </label>
                <input
                  id="form-product-title-input"
                  data-testid="form-product-title-input"
                  type="text"
                  value={draft.title || ''}
                  onChange={(e) => updateDraft({ title: e.target.value })}
                  placeholder="Например: Рубашка оверсайз из плотного хлопка"
                  className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-white/10 bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div>
                <label
                  htmlFor="form-product-description-input"
                  className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
                >
                  Описание
                </label>
                <textarea
                  id="form-product-description-input"
                  data-testid="form-product-description-input"
                  rows={4}
                  value={draft.description || ''}
                  onChange={(e) => updateDraft({ description: e.target.value })}
                  placeholder="Подробное описание товара..."
                  className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-white/10 bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
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
                Медиа
              </h2>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Единая упорядоченная галерея медиафайлов товара с опциональной привязкой к цветам.
            </p>
          </div>
        )}

        {activeSection === 'characteristics' && (
          <div
            id="studio-section-panel-characteristics"
            role="tabpanel"
            aria-labelledby="studio-section-tab-characteristics"
            data-testid="studio-section-panel-characteristics"
            className="space-y-4"
          >
            <div className="flex items-center gap-2 pb-2 border-b border-gray-200 dark:border-white/10">
              <Sliders className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                Характеристики
              </h2>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Параметры категории и состав материалов.
            </p>
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
              Размеры, цвета и артикулы товара. Управление складскими остатками осуществляется складом ZAMK (FBO).
            </p>
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
                Цена
              </h2>
            </div>
            <div className="max-w-xs space-y-4">
              <div>
                <label
                  htmlFor="form-product-price-input"
                  className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1"
                >
                  Базовая цена (рубли)
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
                  placeholder="0 ₽"
                  className="w-full px-3 py-2 text-sm rounded-lg border border-gray-200 dark:border-white/10 bg-white dark:bg-white/5 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
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
    </div>
  );
}

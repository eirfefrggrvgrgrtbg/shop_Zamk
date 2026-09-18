import { useProductStudio } from '../../contexts/ProductStudioContext';
import { SellerSurface } from '../SellerSurface';
import { formatPrice } from '../../lib/utils';

export function ProductStudioVisualPlaceholder() {
  const { draft, updateDraft } = useProductStudio();

  return (
    <div
      id="studio-workspace-visual"
      role="tabpanel"
      aria-labelledby="studio-tab-visual"
      data-testid="studio-visual-workspace"
      className="space-y-4"
    >
      <SellerSurface variant="primary" className="p-6">
        <div data-testid="visual-placeholder-notice" className="text-sm text-gray-500 dark:text-gray-400">
          Визуальный режим Product Studio (внутренний плейсхолдер)
        </div>
        <div className="mt-4 space-y-3 max-w-xl">
          <div>
            <span className="text-xs text-gray-500 uppercase tracking-wider">Название</span>
            <div data-testid="visual-draft-title" className="text-lg font-semibold text-gray-900 dark:text-white">
              {draft.title || 'Название не указано'}
            </div>
          </div>

          <div>
            <span className="text-xs text-gray-500 uppercase tracking-wider">Описание</span>
            <div data-testid="visual-draft-description" className="text-sm text-gray-600 dark:text-gray-300">
              {draft.description || 'Описание не указано'}
            </div>
          </div>

          {draft.priceCents !== undefined && (
            <div>
              <span className="text-xs text-gray-500 uppercase tracking-wider">Цена</span>
              <div data-testid="visual-draft-price" className="text-sm font-medium text-gray-900 dark:text-white">
                {formatPrice(draft.priceCents / 100)}
              </div>
            </div>
          )}

          {draft.brandName && (
            <div>
              <span className="text-xs text-gray-500 uppercase tracking-wider">Бренд</span>
              <div data-testid="visual-draft-brand" className="text-sm text-gray-600 dark:text-gray-300">
                {draft.brandName}
              </div>
            </div>
          )}

          <div className="pt-2">
            <button
              type="button"
              data-testid="visual-inline-edit-btn"
              onClick={() => updateDraft({ title: `${draft.title ? draft.title + ' (изменено)' : 'Новое название'}` })}
              className="text-xs text-indigo-600 hover:text-indigo-500 underline"
            >
              Быстрое редактирование названия
            </button>
          </div>
        </div>
      </SellerSurface>
    </div>
  );
}

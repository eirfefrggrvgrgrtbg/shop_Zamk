import { useState, useEffect, useMemo } from 'react';
import { createPortal } from 'react-dom';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioSectionNav } from './ProductStudioSectionNav';
import { SellerSurface } from '../SellerSurface';
import { FileText, Image, Sliders, Layers, DollarSign, ShieldCheck, Folder, Link2, Pencil, Trash2, Plus, GripVertical, CheckCircle, AlertCircle, X } from 'lucide-react';
import {
  isColorRequired,
  isSizeRequired,
  getSizeChartCompleteness,
  getOfferedSizes,
  getCompositionCompleteness,
  getCanonicalProductAttributes,
  getCanonicalRequiredProductAttributes,
} from './productStudioReadinessHelper';
import {
  MIN_PRODUCT_IMAGES,
  MAX_PRODUCT_IMAGES,
  ALLOWED_IMAGE_MIME_TYPES,
  validateImageFile,
  getMediaProgressText,
  createLocalProductStudioImage,
  getProductStudioImageDisplayUrl,
} from './productStudioMediaHelper';
import { getSellerCategorySchema, type SellerCategorySchema } from '@zamk/api-client';
import { cn } from '../../lib/utils';
import { ProductStudioPhotoColorModal, getColorDisplayName } from './ProductStudioPhotoColorModal';
import { ProductStudioCompositionModal } from './ProductStudioCompositionModal';
import { ProductStudioCareModal } from './ProductStudioCareModal';
import { ProductStudioCharacteristicsModal } from './ProductStudioCharacteristicsModal';
import { ProductStudioSizeChartModal } from './ProductStudioSizeChartModal';
import {
  reconcileProductStudioVariantMatrix,
  resolveProductStudioDimensionType,
  canDeactivateVariantTuple,
  toggleProductStudioVariantTuple,
  BLOCKED_LAST_CELL_TOOLTIP,
} from './productStudioMatrixHelper';
import { getSellerColors, getSellerSizeValues, type SellerColor, type SellerSizeValue } from '@zamk/api-client';

const isUuid = (str?: string | null) =>
  Boolean(str && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(str.trim()));

function getCanonicalSizeLabel(
  sizeValueId: string,
  sizesList?: SellerSizeValue[] | null,
  fallbackLabel?: string | null
): string {
  if (sizesList && sizesList.length > 0) {
    const found = sizesList.find((s) => s.id === sizeValueId);
    if (found && found.value && !isUuid(found.value)) {
      return found.value;
    }
  }

  if (fallbackLabel && fallbackLabel.trim() !== '' && !isUuid(fallbackLabel)) {
    return fallbackLabel.trim();
  }

  return 'Размер недоступен';
}

export function ProductStudioFormWorkspace() {
  const {
    activeSection,
    setActiveSection,
    draft,
    updateDraft,
    setCategoryModalOpen,
    readiness,
    markTouched,
    isFieldAttention,
    setShowReadinessAttention,
    categorySchema: contextCategorySchema,
    activeSizeSystemId,
    createMediaUrl,
  } = useProductStudio();
  const [localCategorySchema, setLocalCategorySchema] = useState<SellerCategorySchema | null>(null);
  const categorySchema = contextCategorySchema || localCategorySchema;
  const [isCompositionModalOpen, setIsCompositionModalOpen] = useState(false);
  const [isCareModalOpen, setIsCareModalOpen] = useState(false);
  const [isCharacteristicsModalOpen, setIsCharacteristicsModalOpen] = useState(false);
  const [isSizeChartModalOpen, setIsSizeChartModalOpen] = useState(false);
  const [isPhotoColorModalOpen, setIsPhotoColorModalOpen] = useState(false);
  const [mediaError, setMediaError] = useState<string | null>(null);
  const [isValidatingPhoto, setIsValidatingPhoto] = useState(false);
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  const imagesList = useMemo(() => draft.images || [], [draft.images]);

  const handleAddMedia = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setMediaError(null);
    setIsValidatingPhoto(true);

    try {
      const validation = await validateImageFile(file);
      if (!validation.valid) {
        setMediaError(validation.error || 'Недопустимый файл');
        return;
      }

      const objectUrl = createMediaUrl(file);
      const existingImages = draft.images || [];
      const isFirst = existingImages.length === 0;
      const newImage = createLocalProductStudioImage({
        file,
        previewUrl: objectUrl,
        isMain: isFirst,
        sortOrder: existingImages.length,
        colorId: null,
      });

      updateDraft({
        images: [...existingImages, newImage],
      });
      markTouched('media');
    } finally {
      setIsValidatingPhoto(false);
      e.target.value = '';
    }
  };

  const handleReplaceMedia = async (e: React.ChangeEvent<HTMLInputElement>, index: number) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setMediaError(null);
    setIsValidatingPhoto(true);

    try {
      const validation = await validateImageFile(file);
      if (!validation.valid) {
        setMediaError(validation.error || 'Недопустимый файл');
        return;
      }

      const objectUrl = createMediaUrl(file);
      const existingImages = [...(draft.images || [])];
      const targetOld = existingImages[index];

      const replacedImage = createLocalProductStudioImage({
        file,
        previewUrl: objectUrl,
        isMain: index === 0,
        sortOrder: index,
        colorId: targetOld?.colorId ?? null,
        altText: targetOld?.altText ?? null,
      });

      existingImages[index] = replacedImage;
      updateDraft({ images: existingImages });
      markTouched('media');
    } finally {
      setIsValidatingPhoto(false);
      e.target.value = '';
    }
  };

  const handleDeleteMedia = (index: number) => {
    const existingImages = [...(draft.images || [])];
    existingImages.splice(index, 1);
    const reindexed = existingImages.map((img, idx) => ({
      ...img,
      isMain: idx === 0,
      sortOrder: idx,
    }));
    updateDraft({ images: reindexed });
    markTouched('media');
  };

  const handleSetAsCover = (index: number) => {
    if (index === 0) return;
    const existingImages = [...(draft.images || [])];
    const [moved] = existingImages.splice(index, 1);
    existingImages.unshift(moved);
    const reindexed = existingImages.map((img, idx) => ({
      ...img,
      isMain: idx === 0,
      sortOrder: idx,
    }));
    updateDraft({ images: reindexed });
    markTouched('media');
  };

  const handleReorderMedia = (fromIndex: number, toIndex: number) => {
    if (fromIndex === toIndex || fromIndex < 0 || toIndex < 0) return;
    const existingImages = [...(draft.images || [])];
    if (fromIndex >= existingImages.length || toIndex >= existingImages.length) return;

    const [moved] = existingImages.splice(fromIndex, 1);
    existingImages.splice(toIndex, 0, moved);

    const reindexed = existingImages.map((img, idx) => ({
      ...img,
      isMain: idx === 0,
      sortOrder: idx,
    }));

    updateDraft({ images: reindexed });
    markTouched('media');
  };

  useEffect(() => {
    if (activeSection === 'review') {
      setShowReadinessAttention(true);
    }
  }, [activeSection, setShowReadinessAttention]);

  useEffect(() => {
    if (!draft.categoryId) {
      setLocalCategorySchema(null);
      return;
    }
    if (contextCategorySchema) {
      return;
    }
    let isMounted = true;
    getSellerCategorySchema(draft.categoryId)
      .then((schema) => {
        if (isMounted) setLocalCategorySchema(schema);
      })
      .catch((err) => {
        console.error('Failed to load schema in Form Workspace:', err);
      });
    return () => {
      isMounted = false;
    };
  }, [draft.categoryId, contextCategorySchema]);

  const colorNeeded = isColorRequired(draft, categorySchema);
  const sizeNeeded = isSizeRequired(draft, categorySchema);

  const isCompositionMissing =
    readiness?.blockingFields?.includes('composition') ?? !getCompositionCompleteness(draft).isComplete;
  const offeredSizes = useMemo(() => getOfferedSizes(draft), [draft]);
  const sizeChartCompleteness = useMemo(() => {
    return getSizeChartCompleteness(draft, categorySchema);
  }, [draft, categorySchema]);
  const isSizeChartMissing = readiness?.blockingFields?.includes('sizeChart') ?? !sizeChartCompleteness.isComplete;



  const effectiveDimensionType = resolveProductStudioDimensionType(
    draft.dimensionType,
    categorySchema?.dimensionType
  );

  const showColorEditor = effectiveDimensionType === 'COLOR_ONLY';
  const showSizeEditor = effectiveDimensionType === 'SIZE_ONLY';

  const [isFormColorPopoverOpen, setIsFormColorPopoverOpen] = useState(false);
  const [pendingFormColorIds, setPendingFormColorIds] = useState<Set<string>>(new Set());
  const [isFormSizePopoverOpen, setIsFormSizePopoverOpen] = useState(false);
  const [pendingFormSizeIds, setPendingFormSizeIds] = useState<Set<string>>(new Set());

  const [colorsList, setColorsList] = useState<SellerColor[]>([]);
  const [sizesList, setSizesList] = useState<SellerSizeValue[]>([]);

  useEffect(() => {
    let isMounted = true;
    getSellerColors()
      .then((data) => {
        if (isMounted) setColorsList(data || []);
      })
      .catch((err) => {
        console.error('Failed to load seller colors in Form Workspace:', err);
      });
    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    let isMounted = true;
    if (activeSizeSystemId) {
      getSellerSizeValues(activeSizeSystemId)
        .then((data) => {
          if (isMounted) setSizesList(data || []);
        })
        .catch((err) => {
          console.error('Failed to load size values in Form Workspace:', err);
        });
    } else {
      setSizesList([]);
    }
    return () => {
      isMounted = false;
    };
  }, [activeSizeSystemId]);

  const [sizePopoverAnchor, setSizePopoverAnchor] = useState<DOMRect | null>(null);
  const [colorPopoverAnchor, setColorPopoverAnchor] = useState<DOMRect | null>(null);

  const configuredColors = useMemo(() => {
    const colorMap = new Map<string, { id: string; name: string; hex: string }>();

    for (const c of draft.colors || []) {
      if (c.id && !colorMap.has(c.id)) {
        const found = colorsList.find((item) => item.id === c.id);
        const name = found?.nameRu || c.name || (c as any)?.nameRu || 'Цвет';
        const hex = found?.hex || c.hex || (c as any)?.hexValue || '#cccccc';
        colorMap.set(c.id, { id: c.id, name, hex });
      }
    }

    for (const v of draft.variants || []) {
      if (v.colorId && !colorMap.has(v.colorId)) {
        const found = colorsList.find((item) => item.id === v.colorId);
        const name = found?.nameRu || v.colorName || 'Цвет';
        const hex = found?.hex || v.colorHex || '#cccccc';
        colorMap.set(v.colorId, { id: v.colorId, name, hex });
      }
    }

    return Array.from(colorMap.values());
  }, [draft.colors, draft.variants, colorsList]);

  const configuredSizes = useMemo(() => {
    const sizeIds = new Set<string>();
    // Collect sizeValueIds for sizes that have at least one active variant
    for (const v of draft.variants || []) {
      if (v.isActive !== false && v.sizeValueId) {
        sizeIds.add(v.sizeValueId);
      }
    }

    const ids = Array.from(sizeIds);
    if (sizesList && sizesList.length > 0) {
      ids.sort((a, b) => {
        const idxA = sizesList.findIndex((s) => s.id === a);
        const idxB = sizesList.findIndex((s) => s.id === b);
        if (idxA !== -1 && idxB !== -1) {
          const orderA = sizesList[idxA].sortOrder ?? idxA;
          const orderB = sizesList[idxB].sortOrder ?? idxB;
          return orderA - orderB;
        }
        if (idxA !== -1) return -1;
        if (idxB !== -1) return 1;
        return 0;
      });
    }

    return ids.map((id) => {
      const fallback = (draft.variants || []).find((v) => v.sizeValueId === id);
      const label = getCanonicalSizeLabel(id, sizesList, fallback?.size);
      return { id, label };
    });
  }, [draft.variants, sizesList]);

  const activeVariants = useMemo(() => {
    return (draft.variants || []).filter((v) => v.isActive !== false);
  }, [draft.variants]);

  const totalPossibleVariants = configuredColors.length * configuredSizes.length;
  const activeVariantsInMatrix = useMemo(() => {
    return activeVariants.filter((v) =>
      configuredColors.some((c) => c.id === v.colorId) &&
      configuredSizes.some((s) => s.id === v.sizeValueId)
    ).length;
  }, [activeVariants, configuredColors, configuredSizes]);

  const handleOpenFormColorPopover = (e?: React.MouseEvent<HTMLElement>) => {
    setIsFormSizePopoverOpen(false);
    if (e) {
      setColorPopoverAnchor(e.currentTarget.getBoundingClientRect());
    }
    setPendingFormColorIds(new Set(configuredColors.map((c) => c.id)));
    setIsFormColorPopoverOpen(true);
  };

  const handleOpenFormSizePopover = (e?: React.MouseEvent<HTMLElement>) => {
    setIsFormColorPopoverOpen(false);
    if (e) {
      setSizePopoverAnchor(e.currentTarget.getBoundingClientRect());
    }
    setPendingFormSizeIds(new Set(configuredSizes.map((s) => s.id)));
    setIsFormSizePopoverOpen(true);
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsFormColorPopoverOpen(false);
        setIsFormSizePopoverOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const handleApplyFormColors = (selectedIds: string[]) => {
    if (!effectiveDimensionType) return;
    const nextColors = selectedIds.map((id) => {
      const c = colorsList.find((c) => c.id === id);
      return { id, name: c?.nameRu || (c as any)?.name || id, hex: c?.hex || (c as any)?.hexValue };
    });
    const currentSizes = configuredSizes.map((s) => ({ id: s.id, label: s.label }));

    const updatedVariants = reconcileProductStudioVariantMatrix(
      effectiveDimensionType,
      nextColors,
      currentSizes,
      draft.variants || [],
      configuredColors,
      configuredSizes
    );
    updateDraft({ colors: nextColors, variants: updatedVariants });
    markTouched('variants');
    setIsFormColorPopoverOpen(false);
  };

  const handleApplyFormSizes = (selectedIds: string[]) => {
    if (!effectiveDimensionType) return;
    const nextSizes = selectedIds.map((id) => {
      const fallback = (draft.variants || []).find((v) => v.sizeValueId === id);
      const label = getCanonicalSizeLabel(id, sizesList, fallback?.size);
      return { id, label };
    });

    if (sizesList && sizesList.length > 0) {
      nextSizes.sort((a, b) => {
        const idxA = sizesList.findIndex((s) => s.id === a.id);
        const idxB = sizesList.findIndex((s) => s.id === b.id);
        if (idxA !== -1 && idxB !== -1) {
          const orderA = sizesList[idxA].sortOrder ?? idxA;
          const orderB = sizesList[idxB].sortOrder ?? idxB;
          return orderA - orderB;
        }
        if (idxA !== -1) return -1;
        if (idxB !== -1) return 1;
        return 0;
      });
    }

    const currentSizes = configuredSizes.map((s) => ({ id: s.id, label: s.label }));

    const updatedVariants = reconcileProductStudioVariantMatrix(
      effectiveDimensionType,
      configuredColors,
      nextSizes,
      draft.variants || [],
      configuredColors,
      currentSizes
    );
    updateDraft({ variants: updatedVariants });
    markTouched('variants');
    setIsFormSizePopoverOpen(false);
  };

  const handleRemoveColor = (colorId: string) => {
    if (!effectiveDimensionType) return;
    const nextColors = configuredColors.filter((c) => c.id !== colorId);
    handleApplyFormColors(nextColors.map((c) => c.id));
  };

  const handleRemoveSize = (sizeId: string) => {
    if (!effectiveDimensionType) return;
    const nextSizes = configuredSizes.filter((s) => s.id !== sizeId);
    handleApplyFormSizes(nextSizes.map((s) => s.id));
  };

  const renderColorPopover = () => {
    if (!isFormColorPopoverOpen) return null;
    const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : 1024;
    const popoverWidth = 288;
    let left = colorPopoverAnchor ? colorPopoverAnchor.left : 100;
    if (left < 16) left = 16;
    if (left + popoverWidth > viewportWidth - 16) left = viewportWidth - popoverWidth - 16;
    const top = colorPopoverAnchor ? colorPopoverAnchor.bottom + 8 : 100;

    const content = (
      <div
        data-testid="form-color-popover"
        style={{
          position: 'fixed',
          top: `${top}px`,
          left: `${left}px`,
          width: `${popoverWidth}px`,
          zIndex: 9999,
        }}
        className="bg-white dark:bg-[#1a1a1c] border border-gray-200 dark:border-white/10 rounded-xl shadow-xl p-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-3">
          <h4 className="font-medium text-sm text-gray-900 dark:text-white">
            Выберите цвет
          </h4>
          <button
            type="button"
            onClick={() => setIsFormColorPopoverOpen(false)}
            className="text-xs text-gray-500 hover:text-gray-900 dark:hover:text-white"
          >
            Отмена
          </button>
        </div>
        <div className="max-h-60 overflow-y-auto space-y-1 mb-3">
          {colorsList.map((color) => {
            const hex = color.hex || (color as any).hexValue || '#cccccc';
            const isChecked = pendingFormColorIds.has(color.id);
            return (
              <label
                key={color.id}
                className="w-full flex items-center gap-3 px-2.5 py-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer text-left text-sm"
              >
                <input
                  type="checkbox"
                  data-testid={`form-color-option-${color.id}`}
                  checked={isChecked}
                  onChange={() => {
                    const next = new Set(pendingFormColorIds);
                    if (next.has(color.id)) next.delete(color.id);
                    else next.add(color.id);
                    setPendingFormColorIds(next);
                  }}
                  className="rounded border-gray-300 dark:border-white/20 text-indigo-600 focus:ring-indigo-500"
                />
                <span
                  className="w-5 h-5 rounded-full border border-gray-300 dark:border-white/20 shrink-0 shadow-sm"
                  style={{ backgroundColor: hex }}
                />
                <span className="font-medium text-gray-900 dark:text-white truncate">
                  {color.nameRu || (color as any).name || 'Цвет'}
                </span>
              </label>
            );
          })}
        </div>
        <button
          type="button"
          data-testid="form-color-popover-apply-btn"
          onClick={() => handleApplyFormColors(Array.from(pendingFormColorIds))}
          className="w-full h-9 bg-gray-900 text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
        >
          Готово
        </button>
      </div>
    );

    if (typeof document !== 'undefined') {
      return createPortal(content, document.body);
    }
    return content;
  };

  const renderSizePopover = () => {
    if (!isFormSizePopoverOpen) return null;
    const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : 1024;
    const popoverWidth = 280;
    let left = sizePopoverAnchor ? sizePopoverAnchor.right - popoverWidth : 100;
    if (left < 16) left = 16;
    if (left + popoverWidth > viewportWidth - 16) left = viewportWidth - popoverWidth - 16;
    const top = sizePopoverAnchor ? sizePopoverAnchor.bottom + 8 : 100;

    const content = (
      <div
        data-testid="form-size-popover"
        style={{
          position: 'fixed',
          top: `${top}px`,
          left: `${left}px`,
          width: `${popoverWidth}px`,
          zIndex: 9999,
        }}
        className="bg-white dark:bg-[#1a1a1c] border border-gray-200 dark:border-white/10 rounded-xl shadow-xl p-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-3">
          <h4 className="font-medium text-sm text-gray-900 dark:text-white">
            Выберите размер
          </h4>
          <button
            type="button"
            onClick={() => setIsFormSizePopoverOpen(false)}
            className="text-xs text-gray-500 hover:text-gray-900 dark:hover:text-white"
          >
            Отмена
          </button>
        </div>
        <div className="max-h-60 overflow-y-auto space-y-1 mb-3">
          {sizesList.map((size) => {
            const isChecked = pendingFormSizeIds.has(size.id);
            const displayLabel = size.value || (size as any).name || (size as any).nameRu;
            const safeLabel = displayLabel && !isUuid(displayLabel) ? displayLabel : 'Размер недоступен';
            return (
              <label
                key={size.id}
                className="w-full flex items-center gap-3 px-2.5 py-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer text-left text-sm"
              >
                <input
                  type="checkbox"
                  data-testid={`form-size-option-${size.id}`}
                  checked={isChecked}
                  onChange={() => {
                    const next = new Set(pendingFormSizeIds);
                    if (next.has(size.id)) next.delete(size.id);
                    else next.add(size.id);
                    setPendingFormSizeIds(next);
                  }}
                  className="rounded border-gray-300 dark:border-white/20 text-indigo-600 focus:ring-indigo-500"
                />
                <span className="font-medium text-gray-900 dark:text-white truncate">
                  {safeLabel}
                </span>
              </label>
            );
          })}
        </div>
        <button
          type="button"
          data-testid="form-size-popover-apply-btn"
          onClick={() => handleApplyFormSizes(Array.from(pendingFormSizeIds))}
          className="w-full h-9 bg-gray-900 text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
        >
          Готово
        </button>
      </div>
    );

    if (typeof document !== 'undefined') {
      return createPortal(content, document.body);
    }
    return content;
  };

  const toggleMatrixCell = (colorId: string, sizeValueId: string, currentlyActive: boolean) => {
    const color = configuredColors.find((c) => c.id === colorId);
    const size = configuredSizes.find((s) => s.id === sizeValueId);
    const result = toggleProductStudioVariantTuple(
      draft.variants || [],
      {
        colorId,
        colorName: color?.name,
        colorHex: color?.hex,
        sizeValueId,
        size: size?.label,
      },
      currentlyActive
    );

    if (!result.allowed) {
      return;
    }

    updateDraft({ variants: result.variants });
    markTouched('variants');
  };

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

  const productAttrs = useMemo(() => {
    return getCanonicalProductAttributes(categorySchema);
  }, [categorySchema]);

  const requiredProductAttrs = useMemo(() => {
    return getCanonicalRequiredProductAttributes(categorySchema);
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

  const categoryDisplayName = (
    draft.categoryPath ||
    (!isUuid(draft.categoryName || '') ? draft.categoryName : '') ||
    categorySchema?.name ||
    ''
  ).trim();

  // Structured readiness summaries for Review section
  const reviewSectionSummaries = useMemo(() => {
    const blockers = new Set(readiness?.blockingFields || []);

    // 1. Basics: Title, Category, Description
    const titleOk = Boolean(draft.title?.trim());
    const categoryOk = Boolean(draft.categoryId);
    const descOk = Boolean(draft.description?.trim());
    const basicsSatisfied = titleOk && categoryOk && descOk && !blockers.has('title') && !blockers.has('category') && !blockers.has('description');
    const basicsDetails: string[] = [];
    if (!titleOk) basicsDetails.push('Название не указано');
    if (!categoryOk) basicsDetails.push('Категория не выбрана');
    if (!descOk) basicsDetails.push('Описание не заполнено');
    if (basicsDetails.length === 0) basicsDetails.push('Название, категория и описание заполнены');

    // 2. Media: minimum 3 photos
    const mediaCount = (draft.images || []).length;
    const mediaSatisfied = mediaCount >= MIN_PRODUCT_IMAGES && !blockers.has('media');
    const mediaDetails: string[] = [];
    if (mediaCount < MIN_PRODUCT_IMAGES) {
      mediaDetails.push(`Загружено ${mediaCount} из ${MIN_PRODUCT_IMAGES} фото (нужно минимум 3)`);
    } else {
      mediaDetails.push(`Загружено ${mediaCount} фото`);
    }

    // 3. Characteristics and composition
    const compCompleteness = getCompositionCompleteness(draft);
    const charsSatisfied =
      compCompleteness.isComplete &&
      !blockers.has('composition') &&
      !blockers.has('characteristics');
    const charsDetails: string[] = [];
    if (!compCompleteness.isComplete) {
      if ((draft.materialComposition || []).length === 0) {
        charsDetails.push('Состав не указан');
      } else if (!compCompleteness.allRowsValid) {
        charsDetails.push('Не все строки состава корректно заполнены');
      } else {
        charsDetails.push(`Сумма долей состава ${compCompleteness.totalPercentage}% (должна быть ровно 100%)`);
      }
    } else {
      charsDetails.push('Состав указан (100%)');
    }
    if (requiredProductAttrs.length > 0) {
      if (filledRequiredCharacteristicsCount < requiredProductAttrs.length) {
        charsDetails.push(`Характеристики: заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length} обязательных`);
      } else {
        charsDetails.push(`Характеристики: заполнены все обязательные (${requiredProductAttrs.length})`);
      }
    }

    // 4. Variants: color and size matrices, size chart
    const variantsCount = (draft.variants || []).length;
    const colorSatisfied = !colorNeeded || (draft.colors || []).length > 0;
    const sizeSatisfied = !sizeNeeded || (draft.variants || []).some((v) => Boolean(v.sizeValueId || v.size));
    const sizeChartSatisfied = !sizeChartCompleteness.isNeeded || sizeChartCompleteness.isComplete;
    const variantsSatisfied =
      colorSatisfied &&
      sizeSatisfied &&
      sizeChartSatisfied &&
      !blockers.has('color') &&
      !blockers.has('size') &&
      !blockers.has('sizeChart');
    const variantsDetails: string[] = [];
    if (colorNeeded && (draft.colors || []).length === 0) {
      variantsDetails.push('Не выбран ни один цвет');
    }
    if (sizeNeeded && !sizeSatisfied) {
      variantsDetails.push('Не выбран ни один размер');
    }
    if (sizeChartCompleteness.isNeeded && !sizeChartCompleteness.isComplete) {
      variantsDetails.push(`Таблица размеров: заполнено ${sizeChartCompleteness.filledRequiredCellCount} из ${sizeChartCompleteness.requiredCellCount} мерок`);
    }
    if (variantsDetails.length === 0) {
      variantsDetails.push(`Сформировано вариантов: ${variantsCount}`);
      if (sizeChartCompleteness.isNeeded) {
        variantsDetails.push('Таблица размеров заполнена');
      }
    }

    // 5. Pricing
    const priceSatisfied = (draft.priceCents || 0) > 0 && !blockers.has('price');
    const priceDetails: string[] = [];
    if (!priceSatisfied) {
      priceDetails.push('Цена не указана');
    } else {
      priceDetails.push(`Базовая цена: ${((draft.priceCents || 0) / 100).toLocaleString('ru-RU')} ₽`);
    }

    return [
      {
        id: 'basics',
        title: 'Основное',
        targetSection: 'basics' as const,
        isSatisfied: basicsSatisfied,
        details: basicsDetails,
      },
      {
        id: 'media',
        title: 'Медиа',
        targetSection: 'media' as const,
        isSatisfied: mediaSatisfied,
        details: mediaDetails,
      },
      {
        id: 'characteristics',
        title: 'Характеристики и состав',
        targetSection: 'characteristics' as const,
        isSatisfied: charsSatisfied,
        details: charsDetails,
      },
      {
        id: 'variants',
        title: 'Варианты и размеры',
        targetSection: 'variants' as const,
        isSatisfied: variantsSatisfied,
        details: variantsDetails,
      },
      {
        id: 'pricing',
        title: 'Цена',
        targetSection: 'pricing' as const,
        isSatisfied: priceSatisfied,
        details: priceDetails,
      },
    ];
  }, [
    readiness,
    draft,
    requiredProductAttrs,
    filledRequiredCharacteristicsCount,
    colorNeeded,
    sizeNeeded,
    sizeChartCompleteness,
  ]);

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
            className="space-y-6"
          >
            <div className="flex items-center justify-between pb-2 border-b border-gray-200 dark:border-white/10">
              <div className="flex items-center gap-2">
                <Image className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
                <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                  Медиа <span className={isMediaAttention ? "text-amber-600 dark:text-amber-400" : "text-gray-400 dark:text-gray-500"}>*</span>
                </h2>
              </div>
              <div className="flex items-center gap-3">
                {draft.images && draft.images.length > 0 && (
                  <button
                    type="button"
                    data-testid="form-bind-photos-to-colors-btn"
                    disabled={!draft.colors || draft.colors.length === 0}
                    title={!draft.colors || draft.colors.length === 0 ? "Сначала добавьте цвета" : "Привязать фото к цветам"}
                    onClick={() => setIsPhotoColorModalOpen(true)}
                    className={cn(
                      "inline-flex items-center gap-1.5 text-xs font-medium py-1 transition-colors",
                      !draft.colors || draft.colors.length === 0
                        ? "text-gray-400 cursor-not-allowed"
                        : "text-indigo-600 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-300 cursor-pointer hover:underline"
                    )}
                  >
                    <Link2 className="w-3.5 h-3.5" />
                    <span>Привязать фото к цветам</span>
                    {(!draft.colors || draft.colors.length === 0) && (
                      <span className="text-[11px] text-gray-400">(Сначала добавьте цвета)</span>
                    )}
                  </button>
                )}
                <span className="text-xs font-medium text-gray-500 dark:text-gray-400" data-testid="form-media-progress-badge">
                  {getMediaProgressText(imagesList.length)}
                </span>
              </div>
            </div>

            <div className="space-y-1">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Минимум {MIN_PRODUCT_IMAGES} фото · JPG, PNG, WebP · до 10 МБ. Вертикальное фото · минимум 800×1000 px.
              </p>
              <p className="text-xs text-gray-400 dark:text-gray-500">
                Лучше использовать формат 4:5 — фото лучше заполняет карточку товара. Первая фотография является главной обложкой.
              </p>
            </div>

            {mediaError && (
              <div
                data-testid="form-media-upload-error-banner"
                className="p-3 rounded-lg border border-red-200 dark:border-red-900/50 bg-red-50 dark:bg-red-950/30 text-xs text-red-600 dark:text-red-400 flex items-center justify-between"
              >
                <span>{mediaError}</span>
                <button
                  type="button"
                  onClick={() => setMediaError(null)}
                  className="text-red-500 hover:text-red-700 font-bold ml-2"
                >
                  ✕
                </button>
              </div>
            )}

            {isMediaAttention && (
              <div className="p-3 rounded-lg border border-amber-300 dark:border-amber-900/60 bg-amber-50/40 dark:bg-amber-950/20 text-xs text-amber-800 dark:text-amber-300 font-medium">
                Нужно минимум 3 фото (загружено: {imagesList.length})
              </div>
            )}

            {/* Media thumbnails grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4" data-testid="form-media-grid">
              {imagesList.map((img, index) => {
                const assignedColor = (draft.colors || []).find((c: any) => c.id === img.colorId);
                const assignedColorName = assignedColor ? getColorDisplayName(assignedColor) : '';
                const isCover = index === 0;
                const isDragging = draggedIndex === index;
                const isDragOver = dragOverIndex === index && draggedIndex !== index;

                return (
                  <div
                    key={img.uiKey || index}
                    data-testid={`form-media-card-${index}`}
                    draggable
                    onDragStart={(e) => {
                      e.dataTransfer.setData('text/plain', String(index));
                      e.dataTransfer.effectAllowed = 'move';
                      setDraggedIndex(index);
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                      e.dataTransfer.dropEffect = 'move';
                      if (dragOverIndex !== index) {
                        setDragOverIndex(index);
                      }
                    }}
                    onDragLeave={(e) => {
                      e.preventDefault();
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      if (draggedIndex !== null && draggedIndex !== index) {
                        handleReorderMedia(draggedIndex, index);
                      }
                      setDraggedIndex(null);
                      setDragOverIndex(null);
                    }}
                    onDragEnd={() => {
                      setDraggedIndex(null);
                      setDragOverIndex(null);
                    }}
                    className={cn(
                      "group relative flex flex-col rounded-xl border bg-gray-50 dark:bg-white/[0.02] overflow-hidden transition-all cursor-grab active:cursor-grabbing select-none",
                      isDragging
                        ? "opacity-40 scale-[0.98] border-dashed border-indigo-400 dark:border-indigo-600"
                        : isDragOver
                        ? "ring-2 ring-indigo-500 border-indigo-500 bg-indigo-50/20 dark:bg-indigo-950/30"
                        : "border-gray-200 dark:border-white/10 hover:shadow-md"
                    )}
                  >
                    {/* Image preview with 4:5 ratio */}
                    <div className="relative aspect-[4/5] w-full bg-gray-100 dark:bg-white/5 overflow-hidden">
                      <img
                        src={getProductStudioImageDisplayUrl(img)}
                        alt={`Фото товара ${index + 1}`}
                        className="w-full h-full object-cover pointer-events-none"
                      />
                      {/* Cover badge */}
                      {isCover && (
                        <span
                          data-testid={`form-media-cover-badge-${index}`}
                          className="absolute top-2 left-2 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-gray-900 text-white dark:bg-white dark:text-gray-900 shadow-sm"
                        >
                          Обложка
                        </span>
                      )}
                      {/* Drag handle affordance indicator */}
                      <span
                        data-testid={`form-media-drag-handle-${index}`}
                        className="absolute top-2 right-2 p-1 rounded-md bg-black/40 text-white/80 backdrop-blur-xs opacity-60 group-hover:opacity-100 transition-opacity"
                        title="Перетащите для изменения порядка"
                      >
                        <GripVertical className="w-3.5 h-3.5" />
                      </span>
                      {/* Color indicator badge */}
                      {assignedColor && (
                        <span
                          data-testid={`form-media-color-badge-${index}`}
                          title={`Цвет: ${assignedColorName}`}
                          className="absolute bottom-2 left-2 flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-black/60 text-white backdrop-blur-xs"
                        >
                          <span
                            className="w-2 h-2 rounded-full border border-white"
                            style={{ backgroundColor: assignedColor.hex || '#000000' }}
                          />
                          <span className="truncate max-w-[60px]">{assignedColorName}</span>
                        </span>
                      )}
                    </div>

                    {/* Controls toolbar */}
                    <div
                      className="p-1.5 flex items-center justify-between border-t border-gray-200/80 dark:border-white/10 bg-white dark:bg-[#1a1a1c]"
                      onMouseDown={(e) => e.stopPropagation()}
                    >
                      <label
                        data-testid={`form-media-replace-btn-${index}`}
                        title="Заменить фото"
                        aria-label="Заменить фото"
                        className="p-1.5 rounded-lg text-gray-500 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors cursor-pointer flex items-center justify-center"
                      >
                        <input
                          type="file"
                          accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                          className="hidden"
                          onChange={(e) => handleReplaceMedia(e, index)}
                        />
                        <Pencil className="w-3.5 h-3.5" />
                      </label>
                      <button
                        type="button"
                        data-testid={`form-media-delete-btn-${index}`}
                        onClick={() => handleDeleteMedia(index)}
                        title="Удалить фото"
                        aria-label="Удалить фото"
                        className="p-1.5 rounded-lg text-red-500 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950/30 transition-colors cursor-pointer flex items-center justify-center"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>

                    {!isCover && (
                      <button
                        type="button"
                        data-testid={`form-media-make-cover-${index}`}
                        onMouseDown={(e) => e.stopPropagation()}
                        onClick={() => handleSetAsCover(index)}
                        className="w-full py-1 text-[11px] font-medium text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-950/30 border-t border-gray-100 dark:border-white/5 transition-colors cursor-pointer"
                      >
                        Сделать обложкой
                      </button>
                    )}
                  </div>
                );
              })}

              {/* Add photo card slot */}
              {imagesList.length < MAX_PRODUCT_IMAGES && (
                <label
                  data-testid="form-media-add-card"
                  className={cn(
                    "aspect-[4/5] rounded-xl border-2 border-dashed flex flex-col items-center justify-center p-4 text-center cursor-pointer transition-colors group",
                    isMediaAttention
                      ? "border-amber-300 dark:border-amber-700/50 bg-amber-50/20 dark:bg-amber-950/10 hover:border-amber-400"
                      : "border-gray-300 dark:border-white/20 hover:border-gray-500 dark:hover:border-white/50 bg-gray-50/50 dark:bg-white/[0.02]"
                  )}
                >
                  <input
                    type="file"
                    accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                    className="hidden"
                    onChange={handleAddMedia}
                  />
                  <div className={cn(
                    "w-10 h-10 rounded-full flex items-center justify-center text-xl font-light mb-2 transition-transform group-hover:scale-110",
                    isMediaAttention
                      ? "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300"
                      : "bg-gray-100 dark:bg-white/10 text-gray-600 dark:text-gray-300"
                  )}>
                    <Plus className="w-5 h-5" />
                  </div>
                  <span className="text-xs font-semibold text-gray-900 dark:text-white">
                    Добавить фото
                  </span>
                  <span className="text-[10px] text-gray-400 mt-1">
                    {imagesList.length} из {MAX_PRODUCT_IMAGES}
                  </span>
                </label>
              )}
            </div>

            {isValidatingPhoto && (
              <p className="text-xs text-indigo-600 dark:text-indigo-400 font-medium">
                Проверка файла...
              </p>
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
                  <div className="flex items-center gap-2 flex-wrap">
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {categoryDisplayName
                        ? productAttrs.length === 0
                          ? `Категория: ${categoryDisplayName}. Для данной категории нет дополнительных характеристик.`
                          : `Категория: ${categoryDisplayName}. Заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length} обязательных.`
                        : categorySchema || draft.categoryId
                        ? productAttrs.length === 0
                          ? 'Для данной категории нет дополнительных характеристик.'
                          : `Заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length} обязательных.`
                        : 'Сначала выберите категорию товара.'}
                    </p>
                    {categoryDisplayName && (
                      <button
                        type="button"
                        data-testid="form-characteristics-change-category-btn"
                        onClick={() => {
                          markTouched('category');
                          setCategoryModalOpen(true);
                        }}
                        className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline cursor-pointer font-medium"
                      >
                        [Изменить]
                      </button>
                    )}
                  </div>
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

            {!effectiveDimensionType && (
              <div className="p-4 rounded-xl border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 text-center">
                <p className="text-sm text-gray-500 dark:text-gray-400">Укажите категорию товара для настройки вариантов.</p>
              </div>
            )}

            {/* Structured Colors Editor */}
            {showColorEditor && (
              <div
                data-testid="form-colors-editor-section"
                className={`p-4 rounded-xl border space-y-3 ${
                  isColorAttention
                    ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                    : 'border-gray-200 dark:border-white/10'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div>
                    <h3
                      className={`text-sm font-semibold ${
                        isColorAttention
                          ? 'text-amber-800 dark:text-amber-300'
                          : 'text-gray-900 dark:text-white'
                      }`}
                    >
                      Цвета товара{' '}
                      <span
                        className={
                          isColorAttention
                            ? 'text-amber-600 dark:text-amber-400'
                            : 'text-gray-400 dark:text-gray-500'
                        }
                      >
                        *
                      </span>
                    </h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {configuredColors.length > 0
                        ? `Выбрано цветов: ${configuredColors.length}`
                        : 'Цвета не выбраны'}
                    </p>
                  </div>
                  <div className="relative inline-flex items-center">
                    <button
                      type="button"
                      data-testid="form-add-color-btn"
                      onClick={(e) => {
                        if (isFormColorPopoverOpen) {
                          setIsFormColorPopoverOpen(false);
                        } else {
                          handleOpenFormColorPopover(e);
                        }
                      }}
                      className="px-3 py-1.5 text-xs font-medium rounded-md border transition-colors bg-white dark:bg-white/10 text-gray-800 dark:text-white border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20 flex items-center gap-1.5"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      {configuredColors.length > 0 ? 'Изменить цвета' : 'Добавить цвет'}
                    </button>

                    {renderColorPopover()}
                  </div>
                </div>

                {configuredColors.length > 0 && (
                  <div className="flex flex-wrap gap-2 pt-1">
                    {configuredColors.map((color) => {
                      const hex = color.hex || '#cccccc';
                      return (
                        <div
                          key={color.id}
                          data-testid={`form-color-chip-${color.id}`}
                          className="inline-flex items-center gap-2 pl-2 pr-1.5 py-1 rounded-lg border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 text-xs font-medium text-gray-900 dark:text-white"
                        >
                          <span
                            className="w-3.5 h-3.5 rounded-full border border-black/10 dark:border-white/20 shrink-0"
                            style={{ backgroundColor: hex }}
                          />
                          <span>{color.name}</span>
                          <button
                            type="button"
                            data-testid={`form-remove-color-${color.id}`}
                            aria-label={`Удалить цвет ${color.name}`}
                            onClick={() => handleRemoveColor(color.id)}
                            className="w-4 h-4 rounded hover:bg-gray-200 dark:hover:bg-white/10 text-gray-400 hover:text-gray-700 dark:hover:text-white flex items-center justify-center transition-colors"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}

            {/* Structured Sizes Editor */}
            {showSizeEditor && (
              <div
                data-testid="form-sizes-editor-section"
                className={`p-4 rounded-xl border space-y-3 ${
                  isSizeAttention
                    ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                    : 'border-gray-200 dark:border-white/10'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div>
                    <h3
                      className={`text-sm font-semibold ${
                        isSizeAttention
                          ? 'text-amber-800 dark:text-amber-300'
                          : 'text-gray-900 dark:text-white'
                      }`}
                    >
                      Размеры товара{' '}
                      <span
                        className={
                          isSizeAttention
                            ? 'text-amber-600 dark:text-amber-400'
                            : 'text-gray-400 dark:text-gray-500'
                        }
                      >
                        *
                      </span>
                    </h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {configuredSizes.length > 0
                        ? `Выбрано размеров: ${configuredSizes.length}`
                        : 'Размеры не выбраны'}
                    </p>
                  </div>
                  <div className="relative inline-flex items-center">
                    <button
                      type="button"
                      data-testid="form-add-size-btn"
                      onClick={(e) => {
                        if (isFormSizePopoverOpen) {
                          setIsFormSizePopoverOpen(false);
                        } else {
                          handleOpenFormSizePopover(e);
                        }
                      }}
                      className="px-3 py-1.5 text-xs font-medium rounded-md border transition-colors bg-white dark:bg-white/10 text-gray-800 dark:text-white border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20 flex items-center gap-1.5"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      {configuredSizes.length > 0 ? 'Изменить размеры' : 'Добавить размер'}
                    </button>

                    {renderSizePopover()}
                  </div>
                </div>

                {configuredSizes.length > 0 && (
                  <div className="flex flex-wrap gap-2 pt-1">
                    {configuredSizes.map((size) => (
                      <div
                        key={size.id}
                        data-testid={`form-size-chip-${size.id}`}
                        className="inline-flex items-center gap-2 pl-2.5 pr-1.5 py-1 rounded-lg border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 text-xs font-medium text-gray-900 dark:text-white"
                      >
                        <span>{size.label}</span>
                        <button
                          type="button"
                          data-testid={`form-remove-size-${size.id}`}
                          aria-label={`Удалить размер ${size.label}`}
                          onClick={() => handleRemoveSize(size.id)}
                          className="w-4 h-4 rounded hover:bg-gray-200 dark:hover:bg-white/10 text-gray-400 hover:text-gray-700 dark:hover:text-white flex items-center justify-center transition-colors"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Matrix Editor for COLOR_AND_SIZE */}
            {effectiveDimensionType === 'COLOR_AND_SIZE' ? (
              <div
                data-testid="form-matrix-editor-section"
                className={`w-fit max-w-full p-4 rounded-xl border space-y-3 ${
                  (isColorAttention || isSizeAttention)
                    ? 'border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10'
                    : 'border-gray-200 dark:border-white/10'
                }`}
              >
                <div className="flex items-center justify-between gap-4 mb-2">
                  <div className="flex items-center gap-2.5 flex-wrap">
                    <h3 className={`text-sm font-semibold ${
                      isColorAttention || isSizeAttention
                        ? 'text-amber-800 dark:text-amber-300'
                        : 'text-gray-900 dark:text-white'
                    }`}>
                      Матрица вариантов
                    </h3>
                    {totalPossibleVariants > 0 && (
                      <span
                        data-testid="matrix-active-count-badge"
                        className="text-xs text-gray-500 dark:text-gray-400 font-normal"
                      >
                        ({activeVariantsInMatrix} из {totalPossibleVariants} активны)
                      </span>
                    )}
                  </div>
                </div>

                <div className="overflow-x-auto pb-2 max-w-full">
                  <table className="border-collapse text-left inline-table">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-white/10">
                        <th className="py-2.5 px-3 font-medium text-xs text-gray-500 w-[160px] min-w-[140px] max-w-[180px]">
                          Цвет \ Размер
                        </th>
                        {configuredSizes.map((s) => (
                          <th
                            key={s.id}
                            data-testid={`matrix-size-header-${s.id}`}
                            className="py-2.5 px-2 font-medium text-xs text-gray-900 dark:text-white text-center w-[72px] min-w-[64px] max-w-[80px]"
                          >
                            {s.label}
                          </th>
                        ))}
                        <th className="py-2.5 px-1.5 w-[44px] min-w-[40px] max-w-[48px] text-center">
                          <button
                            type="button"
                            data-testid="matrix-add-size-btn"
                            aria-label="Добавить размер"
                            title="Добавить размер"
                            onClick={(e) => {
                              if (isFormSizePopoverOpen) setIsFormSizePopoverOpen(false);
                              else handleOpenFormSizePopover(e);
                            }}
                            className="w-7 h-7 mx-auto flex items-center justify-center text-gray-500 hover:text-indigo-600 bg-gray-100 dark:bg-white/10 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 rounded-md transition-colors cursor-pointer"
                          >
                            <Plus className="w-4 h-4" />
                          </button>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {configuredColors.map((c) => (
                        <tr
                          key={c.id}
                          className="border-b border-gray-100 dark:border-white/5 hover:bg-gray-50/50 dark:hover:bg-white/[0.02]"
                        >
                          <td className="py-2.5 px-3 text-xs font-medium text-gray-900 dark:text-white w-[160px] min-w-[140px] max-w-[180px]">
                            <div className="flex items-center gap-2">
                              <span
                                className="w-3.5 h-3.5 rounded-full border border-black/10 dark:border-white/20 shrink-0"
                                style={{ backgroundColor: c.hex || '#ccc' }}
                              />
                              <span className="truncate">{c.name}</span>
                            </div>
                          </td>
                          {configuredSizes.map((s) => {
                            const isActive = activeVariants.some(
                              (v) => v.colorId === c.id && v.sizeValueId === s.id
                            );
                            const isCellBlocked =
                              isActive && !canDeactivateVariantTuple(activeVariants, c.id, s.id);
                            return (
                              <td
                                key={s.id}
                                className="py-2.5 px-2 text-center w-[72px] min-w-[64px] max-w-[80px]"
                              >
                                <button
                                  type="button"
                                  data-testid={`form-matrix-cell-${c.id}-${s.id}`}
                                  aria-label={
                                    isCellBlocked
                                      ? BLOCKED_LAST_CELL_TOOLTIP
                                      : isActive
                                      ? `Отключить вариант ${c.name} ${s.label}`
                                      : `Включить вариант ${c.name} ${s.label}`
                                  }
                                  title={isCellBlocked ? BLOCKED_LAST_CELL_TOOLTIP : undefined}
                                  onClick={() => toggleMatrixCell(c.id, s.id, isActive)}
                                  className={cn(
                                    "w-7 h-7 rounded-md inline-flex items-center justify-center transition-colors",
                                    isActive
                                      ? isCellBlocked
                                        ? "bg-indigo-600/80 text-white cursor-not-allowed"
                                        : "bg-indigo-600 text-white hover:bg-indigo-700 cursor-pointer"
                                      : "border border-gray-300 dark:border-white/20 bg-gray-50/50 dark:bg-white/5 text-gray-400 hover:border-gray-400 cursor-pointer"
                                  )}
                                >
                                  {isActive && <CheckCircle className="w-4 h-4" />}
                                </button>
                              </td>
                            );
                          })}
                          <td className="py-2.5 px-1.5 w-[44px] min-w-[40px] max-w-[48px]"></td>
                        </tr>
                      ))}
                      <tr>
                        <td className="py-2.5 px-3 w-[160px] min-w-[140px] max-w-[180px]">
                          <button
                            type="button"
                            data-testid="matrix-add-color-btn"
                            aria-label="Добавить цвет"
                            title="Добавить цвет"
                            onClick={(e) => {
                              if (isFormColorPopoverOpen) setIsFormColorPopoverOpen(false);
                              else handleOpenFormColorPopover(e);
                            }}
                            className="h-7 px-2.5 inline-flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 hover:text-indigo-600 dark:hover:text-indigo-400 bg-gray-100 dark:bg-white/10 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 rounded-md transition-colors cursor-pointer"
                          >
                            <Plus className="w-3.5 h-3.5" />
                            <span>Добавить цвет</span>
                          </button>
                        </td>
                        <td colSpan={configuredSizes.length + 1}></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                {renderSizePopover()}
                {renderColorPopover()}
              </div>
            ) : activeVariants.length > 0 ? (
              <div
                data-testid="form-variants-summary"
                className="p-4 rounded-xl border border-gray-200 dark:border-white/10 space-y-2"
              >
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                    Сформированные варианты ({activeVariants.length})
                  </h4>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 pt-1">
                  {activeVariants.map((v, i) => (
                    <div
                      key={v.id || i}
                      data-testid={`form-variant-row-${v.id || i}`}
                      className="px-2.5 py-1.5 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-100 dark:border-white/5 text-xs flex items-center gap-1.5 truncate"
                    >
                      {v.colorHex && (
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0 border border-black/10 dark:border-white/20"
                          style={{ backgroundColor: v.colorHex }}
                        />
                      )}
                      <span className="font-medium text-gray-900 dark:text-white truncate">
                        {v.colorName || '—'}
                      </span>
                      <span className="text-gray-400 dark:text-gray-500">/</span>
                      <span className="text-gray-600 dark:text-gray-300 truncate">
                        {v.size || '—'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}


            {sizeChartCompleteness.isNeeded && (
              <div className={`p-4 rounded-xl border space-y-2 mt-3 ${
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
            className="space-y-6"
          >
            <div className="flex items-center justify-between pb-2 border-b border-gray-200 dark:border-white/10">
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
                <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                  Проверка готовности к публикации
                </h2>
              </div>
              <div className="flex items-center gap-2">
                {reviewSectionSummaries.some((s) => !s.isSatisfied) ? (
                  <span
                    data-testid="review-readiness-badge"
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-100 dark:bg-amber-900/40 text-amber-800 dark:text-amber-300"
                  >
                    <AlertCircle className="w-3.5 h-3.5" />
                    <span>Требует внимания: {reviewSectionSummaries.filter((s) => !s.isSatisfied).length} секц.</span>
                  </span>
                ) : (
                  <span
                    data-testid="review-readiness-badge"
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/40 text-green-800 dark:text-green-300"
                  >
                    <CheckCircle className="w-3.5 h-3.5" />
                    <span>Карточка полностью заполнена</span>
                  </span>
                )}
              </div>
            </div>

            <p className="text-sm text-gray-500 dark:text-gray-400">
              Оценка полноты заполнения карточки (readiness). Для отправки товара на модерацию все обязательные поля должны быть заполнены.
            </p>

            {/* Structured review cards grid */}
            <div className="space-y-3" data-testid="form-review-sections-list">
              {reviewSectionSummaries.map((sec) => (
                <div
                  key={sec.id}
                  data-testid={`review-section-card-${sec.id}`}
                  className={cn(
                    "p-4 rounded-xl border flex flex-col sm:flex-row sm:items-center justify-between gap-4 transition-colors",
                    sec.isSatisfied
                      ? "border-gray-200 dark:border-white/10 bg-white dark:bg-white/[0.02]"
                      : "border-amber-300 dark:border-amber-900/60 bg-amber-50/20 dark:bg-amber-950/10"
                  )}
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <h3 className={cn(
                        "text-sm font-semibold",
                        sec.isSatisfied ? "text-gray-900 dark:text-white" : "text-amber-900 dark:text-amber-200"
                      )}>
                        {sec.title}
                      </h3>
                      {sec.isSatisfied ? (
                        <span className="inline-flex items-center gap-1 text-xs text-green-600 dark:text-green-400 font-medium">
                          <CheckCircle className="w-3.5 h-3.5" />
                          <span>Готово</span>
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400 font-medium">
                          <AlertCircle className="w-3.5 h-3.5" />
                          <span>Требует заполнения</span>
                        </span>
                      )}
                    </div>
                    <ul className="text-xs space-y-0.5">
                      {sec.details.map((detail, idx) => (
                        <li
                          key={idx}
                          className={cn(
                            sec.isSatisfied
                              ? "text-gray-500 dark:text-gray-400"
                              : "text-amber-800 dark:text-amber-300"
                          )}
                        >
                          • {detail}
                        </li>
                      ))}
                    </ul>
                  </div>

                  <button
                    type="button"
                    data-testid={`review-fix-btn-${sec.id}`}
                    onClick={() => {
                      setActiveSection(sec.targetSection);
                      if (sec.id === 'characteristics' && !draft.categoryId) {
                        setCategoryModalOpen(true);
                      }
                    }}
                    className={cn(
                      "px-3.5 py-1.5 text-xs font-medium rounded-lg border transition-colors shrink-0 self-start sm:self-center",
                      sec.isSatisfied
                        ? "bg-white dark:bg-white/10 text-gray-700 dark:text-gray-200 border-gray-300 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/20"
                        : "bg-amber-50 dark:bg-amber-950/30 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100"
                    )}
                  >
                    {sec.isSatisfied ? 'Изменить' : 'Исправить'}
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </SellerSurface>

      <ProductStudioPhotoColorModal
        isOpen={isPhotoColorModalOpen}
        onClose={() => setIsPhotoColorModalOpen(false)}
        images={draft.images || []}
        colors={draft.colors || []}
        onSave={(updatedImages) => {
          updateDraft({ images: updatedImages });
          markTouched('media');
        }}
      />

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
        categoryName={categoryDisplayName}
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
        categoryName={categoryDisplayName}
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

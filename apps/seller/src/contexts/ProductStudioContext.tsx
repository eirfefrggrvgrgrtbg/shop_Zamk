import React, { createContext, useContext, useReducer, useMemo, useRef, useState, useEffect, useCallback } from 'react';
import {
  getSellerCategorySchema,
  type SellerCategory,
  type SellerCategorySchema,
  type SellerColor,
  type SellerDictionaryValue,
  type StageSellerProductImageResponse,
  type SellerProduct,
} from '@zamk/api-client';
import { ProductStudioCategoryModal } from '../components/product-studio/ProductStudioCategoryModal';
import { createStudioMediaRegistry, type StudioMediaRegistry } from '../components/product-studio/productStudioMediaSession';
import { getProductStudioReadiness, type ProductStudioReadiness } from '../components/product-studio/productStudioReadinessHelper';
import { getProductStudioImagePreviewUrl } from '../components/product-studio/productStudioMediaHelper';
import {
  isProductStudioSaveEligible,
  orchestrateProductStudioEditSave,
} from '../components/product-studio/productStudioSaveProduct';

export type ProductStudioEntryMode = 'create' | 'edit';
export type ProductStudioViewMode = 'visual' | 'form';

export type ProductStudioSection =
  | 'basics'
  | 'media'
  | 'characteristics'
  | 'variants'
  | 'pricing'
  | 'review';

export interface ProductStudioSectionMeta {
  id: ProductStudioSection;
  label: string;
}

export const PRODUCT_STUDIO_SECTIONS: ProductStudioSectionMeta[] = [
  { id: 'basics', label: 'Основное' },
  { id: 'media', label: 'Медиа' },
  { id: 'characteristics', label: 'Характеристики' },
  { id: 'variants', label: 'Варианты' },
  { id: 'pricing', label: 'Цена' },
  { id: 'review', label: 'Проверка' },
];

export type ProductStudioImageSource =
  | {
      kind: 'canonical';
      imageId: string;
      url: string;
    }
  | {
      kind: 'local';
      clientMediaId: string;
      file: File;
      previewUrl: string;
    }
  | {
      kind: 'staged';
      clientMediaId: string;
      stagedId: string;
      stagedUrl: string;
      previewUrl: string;
    };

export interface ProductStudioImage {
  uiKey: string;
  colorId?: string | null;
  altText?: string | null;
  isMain: boolean;
  sortOrder?: number;
  source: ProductStudioImageSource;
}

export interface ProductStudioVariant {
  id?: string;
  colorId?: string;
  colorName?: string;
  colorHex?: string;
  sizeValueId?: string;
  size?: string;
  sellerSku?: string;
  barcode?: string;
  priceCents?: number;
  isActive?: boolean;
}

export interface ProductStudioDraft {
  id?: string;
  title: string;
  description: string;
  categoryId?: string;
  categoryName?: string;
  brandId?: string;
  brandName?: string;
  sellerName?: string;
  status?: string;
  gender?: string;
  color?: string;
  material?: string;
  careInstructions?: string;
  priceCents?: number;
  oldPriceCents?: number;
  currency?: string;
  images?: ProductStudioImage[];
  variants?: ProductStudioVariant[];
  attributes?: Array<{ attributeDefinitionId?: string; name?: string; code?: string; value?: any; dictionaryValueId?: string }>;
  materialComposition?: Array<{ materialId?: string; materialName?: string; percentage?: number; material?: string }>;
  [key: string]: any;
}

export interface ProductStudioState {
  entryMode: ProductStudioEntryMode;
  viewMode: ProductStudioViewMode;
  activeSection: ProductStudioSection;
  draft: ProductStudioDraft;
  initialDraft: ProductStudioDraft;
  isDirty: boolean;
  editingField: string | null;
  isCategoryModalOpen: boolean;
  touchedFields: Record<string, boolean>;
  showReadinessAttention: boolean;
  saveStatus: 'idle' | 'staging' | 'saving' | 'error' | 'refresh_error';
  saveError: string | null;
  selectedPreviewColorId: string | null;
  selectedPreviewSizeValueId: string | null;
}

type ProductStudioAction =
  | { type: 'SET_VIEW_MODE'; payload: ProductStudioViewMode }
  | { type: 'SET_ACTIVE_SECTION'; payload: ProductStudioSection }
  | { type: 'UPDATE_DRAFT'; payload: Partial<ProductStudioDraft> }
  | { type: 'SET_EDITING_FIELD'; payload: string | null }
  | { type: 'SET_CATEGORY_MODAL_OPEN'; payload: boolean }
  | { type: 'MARK_TOUCHED'; payload: string }
  | { type: 'SET_READINESS_ATTENTION'; payload: boolean }
  | { type: 'RESET_DRAFT' }
  | { type: 'SET_SAVE_STATUS'; payload: { status: 'idle' | 'staging' | 'saving' | 'error' | 'refresh_error'; error?: string | null } }
  | { type: 'PERSIST_STAGED_IMAGES'; payload: ProductStudioImage[] }
  | { type: 'COMMIT_SAVED_DRAFT'; payload: ProductStudioDraft }
  | { type: 'CLEAR_SAVE_ERROR' }
  | { type: 'SET_PREVIEW_COLOR'; payload: string | null }
  | { type: 'SET_PREVIEW_SIZE'; payload: string | null };

function computeIsDirty(current: ProductStudioDraft, initial: ProductStudioDraft): boolean {
  return JSON.stringify(current) !== JSON.stringify(initial);
}

function productStudioReducer(
  state: ProductStudioState,
  action: ProductStudioAction
): ProductStudioState {
  switch (action.type) {
    case 'SET_VIEW_MODE':
      return {
        ...state,
        viewMode: action.payload,
      };

    case 'SET_ACTIVE_SECTION':
      return {
        ...state,
        activeSection: action.payload,
      };

    case 'UPDATE_DRAFT': {
      const updatedDraft = {
        ...state.draft,
        ...action.payload,
      };
      return {
        ...state,
        draft: updatedDraft,
        isDirty: computeIsDirty(updatedDraft, state.initialDraft),
      };
    }

    case 'PERSIST_STAGED_IMAGES': {
      const updatedDraft = {
        ...state.draft,
        images: action.payload,
      };
      return {
        ...state,
        draft: updatedDraft,
        isDirty: computeIsDirty(updatedDraft, state.initialDraft),
      };
    }

    case 'COMMIT_SAVED_DRAFT': {
      const nextDraft = JSON.parse(JSON.stringify(action.payload));
      const nextInitial = JSON.parse(JSON.stringify(action.payload));

      let nextColorId = state.selectedPreviewColorId;
      if (nextColorId) {
        const hasColor =
          (nextDraft.colors || []).some((c: any) => c.id === nextColorId) ||
          (nextDraft.variants || []).some((v: any) => v.colorId === nextColorId);
        if (!hasColor) {
          nextColorId = null;
        }
      }

      let nextSizeId = state.selectedPreviewSizeValueId;
      if (nextSizeId) {
        const hasSize = (nextDraft.variants || []).some((v: any) => {
          if (nextColorId && v.colorId && v.colorId !== nextColorId) {
            return false;
          }
          return v.sizeValueId === nextSizeId;
        });
        if (!hasSize) {
          nextSizeId = null;
        }
      }

      return {
        ...state,
        draft: nextDraft,
        initialDraft: nextInitial,
        selectedPreviewColorId: nextColorId,
        selectedPreviewSizeValueId: nextSizeId,
        isDirty: false,
        saveStatus: 'idle',
        saveError: null,
        touchedFields: {},
        showReadinessAttention: false,
      };
    }

    case 'SET_SAVE_STATUS': {
      return {
        ...state,
        saveStatus: action.payload.status,
        saveError: action.payload.error !== undefined ? action.payload.error : state.saveError,
      };
    }

    case 'CLEAR_SAVE_ERROR': {
      return {
        ...state,
        saveError: null,
      };
    }

    case 'SET_PREVIEW_COLOR': {
      return {
        ...state,
        selectedPreviewColorId: action.payload,
      };
    }

    case 'SET_PREVIEW_SIZE': {
      return {
        ...state,
        selectedPreviewSizeValueId: action.payload,
      };
    }

    case 'RESET_DRAFT':
      return {
        ...state,
        draft: JSON.parse(JSON.stringify(state.initialDraft)),
        selectedPreviewColorId: null,
        selectedPreviewSizeValueId: null,
        isDirty: false,
        touchedFields: {},
        showReadinessAttention: false,
      };

    case 'SET_EDITING_FIELD':
      return {
        ...state,
        editingField: action.payload,
      };

    case 'SET_CATEGORY_MODAL_OPEN':
      return {
        ...state,
        isCategoryModalOpen: action.payload,
      };

    case 'MARK_TOUCHED':
      return {
        ...state,
        touchedFields: {
          ...state.touchedFields,
          [action.payload]: true,
        },
      };

    case 'SET_READINESS_ATTENTION':
      return {
        ...state,
        showReadinessAttention: action.payload,
      };

    default:
      return state;
  }
}

export interface ProductStudioContextValue extends ProductStudioState {
  setViewMode: (mode: ProductStudioViewMode) => void;
  setActiveSection: (section: ProductStudioSection) => void;
  updateDraft: (patch: Partial<ProductStudioDraft>) => void;
  setEditingField: (field: string | null) => void;
  setCategoryModalOpen: (open: boolean) => void;
  resetDraft: () => void;
  markTouched: (field: string) => void;
  setShowReadinessAttention: (show: boolean) => void;
  isFieldAttention: (field: string) => boolean;
  readiness: ProductStudioReadiness;
  categorySchema: SellerCategorySchema | null;
  activeSizeSystemId: string | null;
  setActiveSizeSystemId: (id: string | null) => void;
  createMediaUrl: (file: File) => string;
  revokeMediaUrl: (url: string) => void;
  isSaveInFlight: boolean;
  canSave: boolean;
  saveDraft?: () => Promise<void>;
  clearSaveError: () => void;
  selectedPreviewColorId: string | null;
  selectedPreviewSizeValueId: string | null;
  setSelectedPreviewColorId: (colorId: string | null) => void;
  setSelectedPreviewSizeValueId: (sizeValueId: string | null) => void;
}

const ProductStudioContext = createContext<ProductStudioContextValue | undefined>(undefined);

export interface ProductStudioProviderProps {
  entryMode: ProductStudioEntryMode;
  initialDraft?: Partial<ProductStudioDraft>;
  initialCategorySchema?: SellerCategorySchema | null;
  initialSizeSystemId?: string | null;
  canonicalColors?: SellerColor[];
  dictionaryValuesMap?: Record<string, SellerDictionaryValue[]>;
  saveProductFn?: (productId: string, input: any) => Promise<SellerProduct>;
  getProductFn?: (productId: string) => Promise<SellerProduct>;
  stageImageFn?: (productId: string, clientMediaId: string, file: File) => Promise<StageSellerProductImageResponse>;
  children: React.ReactNode;
}

const DEFAULT_DRAFT: ProductStudioDraft = {
  title: '',
  description: '',
  categoryId: '',
};

export function ProductStudioProvider({
  entryMode,
  initialDraft,
  initialCategorySchema,
  initialSizeSystemId,
  canonicalColors,
  dictionaryValuesMap,
  saveProductFn,
  getProductFn,
  stageImageFn,
  children,
}: ProductStudioProviderProps) {
  const normalizedInitial: ProductStudioDraft = useMemo(() => {
    return {
      ...DEFAULT_DRAFT,
      ...initialDraft,
    };
  }, [initialDraft]);

  const [state, dispatch] = useReducer(productStudioReducer, {
    entryMode,
    viewMode: 'visual', // Invariant: defaults to VISUAL
    activeSection: 'basics',
    draft: { ...normalizedInitial },
    initialDraft: { ...normalizedInitial },
    isDirty: false,
    editingField: null,
    isCategoryModalOpen: false,
    touchedFields: {},
    showReadinessAttention: false,
    saveStatus: 'idle',
    saveError: null,
    selectedPreviewColorId: null,
    selectedPreviewSizeValueId: null,
  });

  const isSaveInFlight = state.saveStatus === 'staging' || state.saveStatus === 'saving';
  const isSaveInFlightRef = useRef(isSaveInFlight);
  isSaveInFlightRef.current = isSaveInFlight;

  const isSavingRef = useRef(false);

  const mediaRegistryRef = useRef<StudioMediaRegistry | null>(null);
  if (!mediaRegistryRef.current) {
    mediaRegistryRef.current = createStudioMediaRegistry();
  }
  const mediaRegistry = mediaRegistryRef.current;

  // Cleanup object URLs ONLY when provider unmounts
  useEffect(() => {
    return () => {
      mediaRegistry.revokeAll();
    };
  }, [mediaRegistry]);

  // Load category schema when categoryId changes
  const [categorySchema, setCategorySchema] = useState<SellerCategorySchema | null>(
    initialCategorySchema || null
  );

  useEffect(() => {
    if (initialCategorySchema !== undefined) {
      setCategorySchema(initialCategorySchema);
    }
  }, [initialCategorySchema]);

  useEffect(() => {
    if (!state.draft.categoryId) {
      setCategorySchema(null);
      return;
    }
    let isMounted = true;
    getSellerCategorySchema(state.draft.categoryId)
      .then((schema) => {
        if (isMounted) setCategorySchema(schema);
      })
      .catch(() => {
        if (isMounted) setCategorySchema(null);
      });
    return () => {
      isMounted = false;
    };
  }, [state.draft.categoryId]);

  const [activeSizeSystemId, setActiveSizeSystemId] = useState<string | null>(
    initialSizeSystemId !== undefined ? initialSizeSystemId : null
  );

  useEffect(() => {
    if (initialSizeSystemId !== undefined) {
      setActiveSizeSystemId(initialSizeSystemId);
    }
  }, [initialSizeSystemId]);

  // In Create mode, when category schema loads or changes, initialize canonical default if unset
  useEffect(() => {
    if (entryMode === 'create') {
      if (categorySchema?.allowedSizeSystems && categorySchema.allowedSizeSystems.length > 0) {
        if (!activeSizeSystemId) {
          const defaultSys =
            categorySchema.allowedSizeSystems.find((s) => s.isDefault) ||
            categorySchema.allowedSizeSystems[0];
          if (defaultSys) {
            setActiveSizeSystemId(defaultSys.id);
          }
        }
      } else if (!state.draft.categoryId) {
        setActiveSizeSystemId(null);
      }
    }
  }, [categorySchema, entryMode, state.draft.categoryId, activeSizeSystemId]);

  const readiness = useMemo(() => {
    return getProductStudioReadiness(state.draft, categorySchema);
  }, [state.draft, categorySchema]);

  const markTouched = useCallback((field: string) => {
    if (isSaveInFlightRef.current) return;
    dispatch({ type: 'MARK_TOUCHED', payload: field });
  }, []);

  const setShowReadinessAttention = useCallback((show: boolean) => {
    if (isSaveInFlightRef.current) return;
    dispatch({ type: 'SET_READINESS_ATTENTION', payload: show });
  }, []);

  const isFieldAttention = useCallback(
    (field: string): boolean => {
      const isBlocking = readiness?.blockingFields?.includes(field as any) ?? false;
      if (!isBlocking) return false;
      return Boolean(state.showReadinessAttention || state.touchedFields[field]);
    },
    [readiness?.blockingFields, state.showReadinessAttention, state.touchedFields]
  );

  const updateDraft = useCallback((patch: Partial<ProductStudioDraft>) => {
    if (isSaveInFlightRef.current) return;
    // If draft images are being updated and any registered URL was removed, revoke it
    if (patch.images && state.draft.images) {
      const nextUrls = new Set<string>();
      for (const img of patch.images) {
        const previewUrl = getProductStudioImagePreviewUrl(img);
        if (previewUrl) {
          nextUrls.add(previewUrl);
        }
      }
      for (const prevImg of state.draft.images) {
        const prevUrl = getProductStudioImagePreviewUrl(prevImg);
        if (prevUrl && !nextUrls.has(prevUrl)) {
          mediaRegistry.revokeObjectUrl(prevUrl);
        }
      }
    }
    dispatch({ type: 'UPDATE_DRAFT', payload: patch });
  }, [state.draft.images, mediaRegistry]);

  const createMediaUrl = useCallback((file: File) => {
    return mediaRegistry.createObjectUrl(file);
  }, [mediaRegistry]);

  const revokeMediaUrl = useCallback((url: string) => {
    mediaRegistry.revokeObjectUrl(url);
  }, [mediaRegistry]);

  const clearSaveError = useCallback(() => {
    dispatch({ type: 'CLEAR_SAVE_ERROR' });
  }, []);

  const canSave = useMemo(() => {
    if (entryMode !== 'edit') return false;
    if (isSaveInFlight) return false;
    if (state.saveStatus === 'refresh_error') return false;
    if (!state.isDirty) return false;
    return isProductStudioSaveEligible(state.draft);
  }, [entryMode, isSaveInFlight, state.saveStatus, state.isDirty, state.draft]);

  const saveDraft = useCallback(async () => {
    if (entryMode !== 'edit') return;
    if (isSavingRef.current) return;
    if (!state.isDirty) return;
    if (!isProductStudioSaveEligible(state.draft)) return;
    if (!state.draft.id) return;

    isSavingRef.current = true;
    try {
      await orchestrateProductStudioEditSave({
        productId: state.draft.id,
        draft: state.draft,
        baselineDraft: state.initialDraft,
        categorySchema,
        canonicalColors: canonicalColors || [],
        dictionaryValuesMap: dictionaryValuesMap || {},
        stageImageFn,
        updateProductFn: saveProductFn,
        getProductFn,
        onStagingStart: () => {
          dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'staging', error: null } });
        },
        onStagedImagesPersisted: (images) => {
          dispatch({ type: 'PERSIST_STAGED_IMAGES', payload: images });
        },
        onSavingStart: () => {
          dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'saving', error: null } });
        },
        onSaveSuccess: (hydratedDraft, finalStagedImages) => {
          const urlsToRevoke = new Set<string>();
          for (const img of finalStagedImages || []) {
            const previewUrl = getProductStudioImagePreviewUrl(img);
            if (previewUrl) urlsToRevoke.add(previewUrl);
          }
          if (state.draft.images) {
            for (const img of state.draft.images) {
              const previewUrl = getProductStudioImagePreviewUrl(img);
              if (previewUrl) urlsToRevoke.add(previewUrl);
            }
          }
          for (const url of urlsToRevoke) {
            mediaRegistry.revokeObjectUrl(url);
          }
          dispatch({ type: 'COMMIT_SAVED_DRAFT', payload: hydratedDraft });
        },
        onError: (errMsg) => {
          dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'error', error: errMsg } });
        },
        onRefreshError: (errMsg) => {
          dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'refresh_error', error: errMsg } });
        },
      });
    } finally {
      isSavingRef.current = false;
    }
  }, [
    entryMode,
    state.isDirty,
    state.draft,
    state.initialDraft,
    categorySchema,
    canonicalColors,
    dictionaryValuesMap,
    stageImageFn,
    saveProductFn,
    getProductFn,
    mediaRegistry,
  ]);

  const contextValue = useMemo<ProductStudioContextValue>(() => {
    return {
      ...state,
      isSaveInFlight,
      canSave,
      saveDraft: entryMode === 'edit' ? saveDraft : undefined,
      clearSaveError,
      setViewMode: (mode: ProductStudioViewMode) =>
        dispatch({ type: 'SET_VIEW_MODE', payload: mode }),
      setActiveSection: (section: ProductStudioSection) =>
        dispatch({ type: 'SET_ACTIVE_SECTION', payload: section }),
      updateDraft,
      setEditingField: (field: string | null) => {
        if (isSaveInFlightRef.current) return;
        dispatch({ type: 'SET_EDITING_FIELD', payload: field });
      },
      setCategoryModalOpen: (open: boolean) => {
        if (isSaveInFlightRef.current && open) return;
        dispatch({ type: 'SET_CATEGORY_MODAL_OPEN', payload: open });
      },
      resetDraft: () => {
        if (isSaveInFlightRef.current) return;
        dispatch({ type: 'RESET_DRAFT' });
      },
      markTouched,
      setShowReadinessAttention,
      isFieldAttention,
      readiness,
      categorySchema,
      activeSizeSystemId,
      setActiveSizeSystemId,
      createMediaUrl,
      revokeMediaUrl,
      setSelectedPreviewColorId: (colorId: string | null) =>
        dispatch({ type: 'SET_PREVIEW_COLOR', payload: colorId }),
      setSelectedPreviewSizeValueId: (sizeValueId: string | null) =>
        dispatch({ type: 'SET_PREVIEW_SIZE', payload: sizeValueId }),
    };
  }, [
    state,
    isSaveInFlight,
    canSave,
    saveDraft,
    clearSaveError,
    entryMode,
    updateDraft,
    markTouched,
    setShowReadinessAttention,
    isFieldAttention,
    readiness,
    categorySchema,
    activeSizeSystemId,
    setActiveSizeSystemId,
    createMediaUrl,
    revokeMediaUrl,
  ]);

  const handleSelectCategory = async (cat: SellerCategory) => {
    try {
      const schema = await getSellerCategorySchema(cat.id);
      setCategorySchema(schema);
      dispatch({
        type: 'UPDATE_DRAFT',
        payload: {
          categoryId: cat.id,
          categoryName: cat.name,
          dimensionType: schema.dimensionType || state.draft.dimensionType || 'COLOR_AND_SIZE',
        },
      });
    } catch {
      dispatch({
        type: 'UPDATE_DRAFT',
        payload: {
          categoryId: cat.id,
          categoryName: cat.name,
        },
      });
    }
  };

  return (
    <ProductStudioContext.Provider value={contextValue}>
      {children}
      <ProductStudioCategoryModal
        isOpen={state.isCategoryModalOpen}
        onClose={() => dispatch({ type: 'SET_CATEGORY_MODAL_OPEN', payload: false })}
        onSelectCategory={handleSelectCategory}
        currentCategoryId={state.draft.categoryId}
      />
    </ProductStudioContext.Provider>
  );
}

export function useProductStudio(): ProductStudioContextValue {
  const context = useContext(ProductStudioContext);
  if (!context) {
    throw new Error('useProductStudio must be used within a ProductStudioProvider');
  }
  return context;
}

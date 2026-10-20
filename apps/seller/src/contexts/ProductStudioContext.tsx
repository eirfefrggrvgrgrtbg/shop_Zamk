import React, { createContext, useContext, useReducer, useMemo, useRef, useState, useEffect, useCallback } from 'react';
import {
  getSellerCategorySchema,
  submitSellerProductModeration,
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
import {
  getProductStudioImagePreviewUrl,
  deriveProductStudioMediaMode,
  reconcileMediaOnColorRemoval,
  normalizeProductStudioCovers,
  type ProductStudioMediaMode,
} from '../components/product-studio/productStudioMediaHelper';
import {
  isProductStudioSaveEligible,
  isProductStudioCreateAutosaveEligible,
  orchestrateProductStudioEditSave,
} from '../components/product-studio/productStudioSaveProduct';
import { orchestrateProductStudioCreateSave } from '../components/product-studio/productStudioSaveCreate';
import { resolveDeterministicPreviewColorId } from '../components/product-studio/productStudioPresentationAdapter';

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
      renditionUrl?: string | null;
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
  isUnassigned?: boolean;
  cropX?: number;
  cropY?: number;
  cropWidth?: number;
  cropHeight?: number;
  renditionUrl?: string | null;
  renditionStatus?: 'READY' | 'NEEDS_CROP' | 'PROCESSING' | 'FAILED';
  renditionError?: string | null;
  source: ProductStudioImageSource;
}

export interface ProductStudioVariant {
  id?: string;
  colorId?: string;
  colorName?: string;
  colorHex?: string;
  /** Optional free-text description of the exact shade (e.g. «Графитовый»); NOT used for filtering. */
  shadeName?: string;
  /** Legacy free-text color of a variant that has no dictionary color_id yet (read-only, never auto-assigned). */
  legacyColorText?: string;
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
  categoryPath?: string;
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
  mediaMode?: ProductStudioMediaMode;
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
  mediaMode: ProductStudioMediaMode;
  isDirty: boolean;
  editingField: string | null;
  isCategoryModalOpen: boolean;
  touchedFields: Record<string, boolean>;
  showReadinessAttention: boolean;
  saveStatus: 'idle' | 'staging' | 'saving' | 'error' | 'refresh_error' | 'identity_recovery_required';
  saveError: string | null;
  selectedPreviewColorId: string | null;
  selectedPreviewSizeValueId: string | null;
  selectedMediaColorId?: string | 'UNASSIGNED' | null;
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
  | { type: 'SET_SAVE_STATUS'; payload: { status: 'idle' | 'staging' | 'saving' | 'error' | 'refresh_error' | 'identity_recovery_required'; error?: string | null } }
  | { type: 'PERSIST_STAGED_IMAGES'; payload: ProductStudioImage[] }
  | { type: 'COMMIT_SAVED_DRAFT'; payload: ProductStudioDraft }
  | { type: 'COMMIT_INTERMEDIATE_SAVE'; payload: ProductStudioDraft }
  | { type: 'CLEAR_SAVE_ERROR' }
  | { type: 'SET_PREVIEW_COLOR'; payload: string | null }
  | { type: 'SET_PREVIEW_SIZE'; payload: string | null }
  | { type: 'SET_MEDIA_COLOR'; payload: string | 'UNASSIGNED' | null };

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
      let updatedImages = action.payload.images !== undefined ? action.payload.images : state.draft.images;
      const nextColors = action.payload.colors !== undefined ? action.payload.colors : state.draft.colors;
      const nextVariants = action.payload.variants !== undefined ? action.payload.variants : state.draft.variants;
      let effectiveMode =
        action.payload.mediaMode ||
        state.draft.mediaMode ||
        deriveProductStudioMediaMode(updatedImages, nextColors, nextVariants, state.draft.mediaMode);
      const hasActiveColors = Boolean(nextColors && nextColors.length > 0);

      // If colors are updated and images were not explicitly provided in payload:
      if (action.payload.colors !== undefined && action.payload.images === undefined && effectiveMode === 'COLORWAY') {
        const activeColorIds = new Set<string>((action.payload.colors || []).map((c: any) => String(c.id)));
        updatedImages = reconcileMediaOnColorRemoval(updatedImages, activeColorIds, effectiveMode);
      }

      // If trying to switch to COLORWAY without any colors and without colored images, keep GENERAL:
      if (action.payload.mediaMode === 'COLORWAY' && !hasActiveColors && !(updatedImages || []).some((img) => img.colorId)) {
        effectiveMode = 'GENERAL';
      }

      if (
        (action.payload.images !== undefined || action.payload.mediaMode !== undefined) &&
        updatedImages &&
        updatedImages.length > 0
      ) {
        updatedImages = normalizeProductStudioCovers(
          updatedImages,
          effectiveMode,
          nextColors,
          action.payload.variants !== undefined ? action.payload.variants : state.draft.variants
        );
      }

      const updatedDraft = {
        ...state.draft,
        ...action.payload,
        images: updatedImages,
        mediaMode: effectiveMode,
      };
      const nextColorId = resolveDeterministicPreviewColorId(
        updatedDraft,
        state.selectedPreviewColorId
      );
      let nextSizeId = state.selectedPreviewSizeValueId;
      if (nextSizeId) {
        const hasSize = (updatedDraft.variants || []).some((v: any) => {
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
        draft: updatedDraft,
        mediaMode: effectiveMode,
        selectedPreviewColorId: nextColorId,
        selectedPreviewSizeValueId: nextSizeId,
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

      const nextColorId = resolveDeterministicPreviewColorId(
        nextDraft,
        state.selectedPreviewColorId
      );

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
        entryMode: 'edit',
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

    case 'COMMIT_INTERMEDIATE_SAVE': {
      const nextInitial = JSON.parse(JSON.stringify(action.payload));
      const nextDraft = {
        ...state.draft,
        id: state.draft.id || action.payload.id,
      };
      return {
        ...state,
        entryMode: 'edit',
        draft: nextDraft,
        initialDraft: nextInitial,
        isDirty: computeIsDirty(nextDraft, nextInitial),
        saveStatus: 'idle',
        saveError: null,
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
      const nextColorId = action.payload;
      let nextSizeId = state.selectedPreviewSizeValueId;
      if (nextColorId && nextSizeId) {
        const isCompatible = (state.draft.variants || []).some(
          (v: any) => v.colorId === nextColorId && v.sizeValueId === nextSizeId
        );
        if (!isCompatible) {
          nextSizeId = null;
        }
      }
      return {
        ...state,
        selectedPreviewColorId: nextColorId,
        selectedPreviewSizeValueId: nextSizeId,
      };
    }

    case 'SET_PREVIEW_SIZE': {
      const nextSizeId = action.payload;
      let nextColorId = state.selectedPreviewColorId;
      if (nextSizeId && nextColorId) {
        const isCompatible = (state.draft.variants || []).some(
          (v: any) => v.sizeValueId === nextSizeId && v.colorId === nextColorId
        );
        if (!isCompatible) {
          nextColorId = null;
        }
      }
      return {
        ...state,
        selectedPreviewSizeValueId: nextSizeId,
        selectedPreviewColorId: nextColorId,
      };
    }

    case 'SET_MEDIA_COLOR':
      return {
        ...state,
        selectedMediaColorId: action.payload,
      };

    case 'RESET_DRAFT':
      return {
        ...state,
        draft: JSON.parse(JSON.stringify(state.initialDraft)),
        mediaMode: state.initialDraft.mediaMode || 'GENERAL',
        selectedPreviewColorId: null,
        selectedPreviewSizeValueId: null,
        selectedMediaColorId: null,
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
  selectCategory?: (category: SellerCategory, categoryPath?: string) => Promise<void>;
  activeSizeSystemId: string | null;
  setActiveSizeSystemId: (id: string | null) => void;
  createMediaUrl: (file: File) => string;
  revokeMediaUrl: (url: string) => void;
  isSaveInFlight: boolean;
  canSave: boolean;
  saveDraft?: () => Promise<boolean | void>;
  flushSave: () => Promise<boolean>;
  clearSaveError: () => void;
  isSubmittingModeration: boolean;
  moderationError: string | null;
  canSubmitModeration: boolean;
  submitModeration?: () => Promise<void>;
  clearModerationError: () => void;
  selectedPreviewColorId: string | null;
  selectedPreviewSizeValueId: string | null;
  selectedMediaColorId?: string | 'UNASSIGNED' | null;
  setSelectedPreviewColorId: (colorId: string | null) => void;
  setSelectedPreviewSizeValueId: (sizeValueId: string | null) => void;
  setSelectedMediaColorId: (colorId: string | 'UNASSIGNED' | null) => void;
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
  createProductFn?: (input: any, options?: { idempotencyKey?: string }) => Promise<SellerProduct>;
  getProductFn?: (productId: string) => Promise<SellerProduct>;
  stageImageFn?: (productId: string, clientMediaId: string, file: File) => Promise<StageSellerProductImageResponse>;
  submitModerationFn?: (productId: string, comment?: string) => Promise<void>;
  onNavigate?: (to: string, options?: { replace?: boolean }) => void;
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
  createProductFn,
  getProductFn,
  stageImageFn,
  submitModerationFn,
  onNavigate,
  children,
}: ProductStudioProviderProps) {
  const normalizedInitial: ProductStudioDraft = useMemo(() => {
    const raw: ProductStudioDraft = {
      ...DEFAULT_DRAFT,
      ...initialDraft,
    };
    const effectiveMode =
      initialDraft?.mediaMode ||
      deriveProductStudioMediaMode(raw.images || [], raw.colors, raw.variants, initialDraft?.mediaMode);
    return {
      ...raw,
      mediaMode: effectiveMode,
    };
  }, [initialDraft]);

  const initialMediaMode = normalizedInitial.mediaMode || 'GENERAL';

  const [state, dispatch] = useReducer(productStudioReducer, {
    entryMode,
    viewMode: 'visual', // Invariant: defaults to VISUAL
    activeSection: 'basics',
    draft: { ...normalizedInitial },
    initialDraft: { ...normalizedInitial },
    mediaMode: initialMediaMode,
    isDirty: false,
    editingField: null,
    isCategoryModalOpen: false,
    touchedFields: {},
    showReadinessAttention: false,
    saveStatus: 'idle',
    saveError: null,
    selectedPreviewColorId: null,
    selectedPreviewSizeValueId: null,
    selectedMediaColorId: null,
  });

  const isSaveInFlight = state.saveStatus === 'staging' || state.saveStatus === 'saving';
  const isSaveInFlightRef = useRef(isSaveInFlight);
  isSaveInFlightRef.current = isSaveInFlight;

  const effectiveEntryMode: ProductStudioEntryMode = state.entryMode;
  const effectiveEntryModeRef = useRef(effectiveEntryMode);
  effectiveEntryModeRef.current = effectiveEntryMode;

  const isSavingRef = useRef(false);
  const inFlightSavePromiseRef = useRef<Promise<boolean> | null>(null);
  const queuedSaveRef = useRef(false);
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);
  const generationRef = useRef(0);
  const latestSaveGenerationRef = useRef(0);

  const latestDraftRef = useRef(state.draft);
  latestDraftRef.current = state.draft;
  const latestInitialDraftRef = useRef(state.initialDraft);
  latestInitialDraftRef.current = state.initialDraft;
  const isDirtyRef = useRef(state.isDirty);
  isDirtyRef.current = state.isDirty;

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
    if (
      initialCategorySchema &&
      (initialCategorySchema.id === state.draft.categoryId ||
        (initialCategorySchema as any).categoryId === state.draft.categoryId)
    ) {
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
  }, [state.draft.categoryId, initialCategorySchema]);

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
    if (isSaveInFlightRef.current || state.saveStatus === 'identity_recovery_required') return;
    dispatch({ type: 'MARK_TOUCHED', payload: field });
  }, [state.saveStatus]);

  const setShowReadinessAttention = useCallback((show: boolean) => {
    if (isSaveInFlightRef.current || state.saveStatus === 'identity_recovery_required') return;
    dispatch({ type: 'SET_READINESS_ATTENTION', payload: show });
  }, [state.saveStatus]);

  const isFieldAttention = useCallback(
    (field: string): boolean => {
      const isBlocking = readiness?.blockingFields?.includes(field as any) ?? false;
      if (!isBlocking) return false;
      return Boolean(state.showReadinessAttention || state.touchedFields[field]);
    },
    [readiness?.blockingFields, state.showReadinessAttention, state.touchedFields]
  );

  const updateDraft = useCallback((patch: Partial<ProductStudioDraft>) => {
    if (state.saveStatus === 'identity_recovery_required') return;
    generationRef.current += 1;
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
  }, [state.draft.images, state.saveStatus, mediaRegistry]);

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
    if (isSaveInFlight) return false;
    if (state.saveStatus === 'refresh_error') return false;
    if (state.saveStatus === 'identity_recovery_required') return true;
    if (effectiveEntryMode === 'edit') {
      if (!state.isDirty) return false;
      return isProductStudioSaveEligible(state.draft);
    }
    if (effectiveEntryMode === 'create') {
      return isProductStudioCreateAutosaveEligible(state.draft, { categorySchema });
    }
    return false;
  }, [effectiveEntryMode, isSaveInFlight, state.saveStatus, state.isDirty, state.draft, categorySchema]);

  const executeSave = useCallback(async (reason?: string): Promise<boolean> => {
    if (isSavingRef.current) {
      queuedSaveRef.current = true;
      if (inFlightSavePromiseRef.current) {
        try {
          await inFlightSavePromiseRef.current;
        } catch {
          // ignore rejection, inFlightSavePromise handles errors internally
        }
      }
      if (queuedSaveRef.current) {
        queuedSaveRef.current = false;
        return executeSave(reason || 'queued');
      }
      return state.saveStatus !== 'error';
    }

    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }

    const currentDraft = latestDraftRef.current;
    const currentInitialDraft = latestInitialDraftRef.current;

    if (state.saveStatus === 'refresh_error') return false;

    const currentEntryMode = effectiveEntryModeRef.current;

    if (currentEntryMode === 'edit') {
      if (state.saveStatus !== 'identity_recovery_required' && !isProductStudioSaveEligible(currentDraft)) {
        return false;
      }
      const dirty = computeIsDirty(currentDraft, currentInitialDraft);
      if (!dirty && state.saveStatus !== 'identity_recovery_required') {
        return true;
      }
    } else {
      if (
        state.saveStatus !== 'identity_recovery_required' &&
        !isProductStudioCreateAutosaveEligible(currentDraft, { categorySchema })
      ) {
        return false;
      }
    }

    const saveGen = generationRef.current;
    latestSaveGenerationRef.current = saveGen;
    isSavingRef.current = true;

    const savePromise = (async () => {
      try {
        if (currentEntryMode === 'edit') {
          if (!currentDraft.id) return false;
          let saveSuccess = false;

          await orchestrateProductStudioEditSave({
            productId: currentDraft.id,
            draft: currentDraft,
            baselineDraft: currentInitialDraft,
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
              saveSuccess = true;
              const urlsToRevoke = new Set<string>();
              for (const img of finalStagedImages || []) {
                const previewUrl = getProductStudioImagePreviewUrl(img);
                if (previewUrl) urlsToRevoke.add(previewUrl);
              }
              if (currentDraft.images) {
                for (const img of currentDraft.images) {
                  const previewUrl = getProductStudioImagePreviewUrl(img);
                  if (previewUrl) urlsToRevoke.add(previewUrl);
                }
              }
              for (const url of urlsToRevoke) {
                mediaRegistry.revokeObjectUrl(url);
              }

              if (generationRef.current === saveGen) {
                latestDraftRef.current = hydratedDraft;
                latestInitialDraftRef.current = hydratedDraft;
                isDirtyRef.current = false;
                dispatch({ type: 'COMMIT_SAVED_DRAFT', payload: hydratedDraft });
              } else {
                latestInitialDraftRef.current = hydratedDraft;
                isDirtyRef.current = computeIsDirty(latestDraftRef.current, hydratedDraft);
                dispatch({ type: 'COMMIT_INTERMEDIATE_SAVE', payload: hydratedDraft });
              }
            },
            onError: (errMsg) => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'error', error: errMsg } });
            },
            onRefreshError: (errMsg) => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'refresh_error', error: errMsg } });
            },
          });

          if (generationRef.current !== saveGen) {
            isSavingRef.current = false;
            inFlightSavePromiseRef.current = null;
            return executeSave('queued_after_intermediate');
          }

          return saveSuccess;
        }

        if (currentEntryMode === 'create') {
          let saveSuccess = false;
          await orchestrateProductStudioCreateSave({
            draft: currentDraft,
            categorySchema,
            canonicalColors: canonicalColors || [],
            dictionaryValuesMap: dictionaryValuesMap || {},
            createProductFn,
            stageImageFn,
            updateProductFn: saveProductFn,
            getProductFn,
            onSavingStart: () => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'saving', error: null } });
            },
            onStagingStart: () => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'staging', error: null } });
            },
            onStagedImagesPersisted: (images) => {
              dispatch({ type: 'PERSIST_STAGED_IMAGES', payload: images });
            },
            onSaveSuccess: (newProductId, hydratedDraft, finalStagedImages) => {
              saveSuccess = true;
              const urlsToRevoke = new Set<string>();
              for (const img of finalStagedImages || []) {
                const previewUrl = getProductStudioImagePreviewUrl(img);
                if (previewUrl) urlsToRevoke.add(previewUrl);
              }
              if (currentDraft.images) {
                for (const img of currentDraft.images) {
                  const previewUrl = getProductStudioImagePreviewUrl(img);
                  if (previewUrl) urlsToRevoke.add(previewUrl);
                }
              }
              for (const url of urlsToRevoke) {
                mediaRegistry.revokeObjectUrl(url);
              }

              const canonicalWithId = {
                ...hydratedDraft,
                id: newProductId || hydratedDraft.id,
              };

              if (generationRef.current !== saveGen) {
                latestDraftRef.current = {
                  ...latestDraftRef.current,
                  id: canonicalWithId.id,
                };
                latestInitialDraftRef.current = canonicalWithId;
                isDirtyRef.current = computeIsDirty(latestDraftRef.current, canonicalWithId);
                effectiveEntryModeRef.current = 'edit';
                dispatch({ type: 'COMMIT_INTERMEDIATE_SAVE', payload: canonicalWithId });
              } else {
                latestDraftRef.current = canonicalWithId;
                latestInitialDraftRef.current = canonicalWithId;
                isDirtyRef.current = false;
                effectiveEntryModeRef.current = 'edit';
                dispatch({ type: 'COMMIT_SAVED_DRAFT', payload: canonicalWithId });
              }
            },
            onError: (errMsg) => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'error', error: errMsg } });
            },
            onIdentityRecoveryRequired: (errMsg) => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'identity_recovery_required', error: errMsg } });
            },
            onRefreshError: (errMsg) => {
              dispatch({ type: 'SET_SAVE_STATUS', payload: { status: 'refresh_error', error: errMsg } });
            },
            navigate: onNavigate || ((to) => {
              if (typeof window !== 'undefined' && window.history?.replaceState) {
                window.history.replaceState(null, '', to);
              }
            }),
          });

          if (generationRef.current !== saveGen) {
            isSavingRef.current = false;
            inFlightSavePromiseRef.current = null;
            return executeSave('queued_after_intermediate');
          }

          return saveSuccess;
        }

        return false;
      } finally {
        isSavingRef.current = false;
        inFlightSavePromiseRef.current = null;
      }
    })();

    inFlightSavePromiseRef.current = savePromise;
    return savePromise;
  }, [
    entryMode,
    state.saveStatus,
    categorySchema,
    canonicalColors,
    dictionaryValuesMap,
    saveProductFn,
    createProductFn,
    getProductFn,
    stageImageFn,
    mediaRegistry,
    onNavigate,
  ]);

  const flushSave = useCallback(async (): Promise<boolean> => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }
    return executeSave('flush');
  }, [executeSave]);

  const saveDraft = useCallback(async (): Promise<boolean> => {
    return flushSave();
  }, [flushSave]);

  // Debounced autosave effect (800ms)
  useEffect(() => {
    if (!state.isDirty) return;
    if (state.saveStatus === 'refresh_error' || state.saveStatus === 'identity_recovery_required') return;

    const isEligible =
      effectiveEntryMode === 'edit'
        ? isProductStudioSaveEligible(state.draft)
        : isProductStudioCreateAutosaveEligible(state.draft, { categorySchema });

    if (!isEligible) return;

    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    debounceTimerRef.current = setTimeout(() => {
      debounceTimerRef.current = null;
      executeSave('debounce');
    }, 800);

    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
    };
  }, [state.draft, state.isDirty, state.saveStatus, effectiveEntryMode, categorySchema, executeSave]);

  const shouldFlushOnSwitch = useCallback((): boolean => {
    if (!isDirtyRef.current) return false;
    if (effectiveEntryModeRef.current === 'edit') return true;
    if (
      effectiveEntryModeRef.current === 'create' &&
      isProductStudioCreateAutosaveEligible(latestDraftRef.current, { categorySchema })
    ) {
      return true;
    }
    return false;
  }, [categorySchema]);

  const setViewMode = useCallback((mode: ProductStudioViewMode) => {
    if (shouldFlushOnSwitch()) {
      flushSave();
    }
    dispatch({ type: 'SET_VIEW_MODE', payload: mode });
  }, [shouldFlushOnSwitch, flushSave]);

  const setActiveSection = useCallback((section: ProductStudioSection) => {
    if (shouldFlushOnSwitch()) {
      flushSave();
    }
    dispatch({ type: 'SET_ACTIVE_SECTION', payload: section });
  }, [shouldFlushOnSwitch, flushSave]);

  const [isSubmittingModeration, setIsSubmittingModeration] = useState(false);
  const [moderationError, setModerationError] = useState<string | null>(null);

  const clearModerationError = useCallback(() => {
    setModerationError(null);
  }, []);

  const isStatusEligibleForModeration = useMemo(() => {
    if (!state.draft.id && effectiveEntryMode === 'create') {
      return true;
    }
    return (
      Boolean(state.draft.id) &&
      (!state.draft.status || state.draft.status === 'draft' || state.draft.status === 'rejected')
    );
  }, [state.draft.id, effectiveEntryMode, state.draft.status]);

  const canSubmitModeration = useMemo(() => {
    if (isSubmittingModeration) return false;
    if (!state.draft.id && effectiveEntryMode !== 'create') return false;
    if (!isStatusEligibleForModeration) return false;
    return readiness.blockingFields.length === 0;
  }, [
    isSubmittingModeration,
    state.draft.id,
    effectiveEntryMode,
    isStatusEligibleForModeration,
    readiness.blockingFields.length,
  ]);

  const submitModeration = useCallback(async () => {
    if (!canSubmitModeration) return;
    if (isSubmittingModeration) return;

    setIsSubmittingModeration(true);
    setModerationError(null);
    try {
      if (!latestDraftRef.current.id || isDirtyRef.current || isSaveInFlightRef.current || debounceTimerRef.current) {
        const saveOk = await flushSave();
        if (!saveOk) {
          setIsSubmittingModeration(false);
          return;
        }
      }

      const currentDraft = latestDraftRef.current;
      if (!currentDraft.id) {
        setModerationError('Товар не сохранен на сервере');
        return;
      }

      const fn = submitModerationFn || submitSellerProductModeration;
      await fn(currentDraft.id);
      dispatch({
        type: 'COMMIT_SAVED_DRAFT',
        payload: {
          ...currentDraft,
          status: 'pending_moderation',
        },
      });
    } catch (err: any) {
      const errMsg = err?.message || '';
      const errCode = err?.code || (err as any)?.response?.data?.code || '';
      if (
        errCode === 'product_media_not_ready' ||
        errMsg.includes('4:5') ||
        errMsg.includes('кадрирование') ||
        errMsg.includes('формате 4:5')
      ) {
        const total = (latestDraftRef.current.images || []).length || 3;
        const missing = (latestDraftRef.current.images || []).filter((img) => !img.renditionUrl).length || 1;
        setModerationError(`Для ${missing} из ${total} фото нужно настроить кадрирование 4:5`);
      } else {
        setModerationError(errMsg || 'Не удалось отправить товар на модерацию');
      }
    } finally {
      setIsSubmittingModeration(false);
    }
  }, [canSubmitModeration, isSubmittingModeration, flushSave, submitModerationFn]);

  const handleSelectCategory = useCallback(async (cat: SellerCategory, categoryPath?: string) => {
    const resolvedName = cat.name;
    const resolvedPath = categoryPath || cat.name;
    try {
      const schema = await getSellerCategorySchema(cat.id);
      setCategorySchema(schema);
      dispatch({
        type: 'UPDATE_DRAFT',
        payload: {
          categoryId: cat.id,
          categoryName: resolvedName,
          categoryPath: resolvedPath,
          dimensionType: schema.dimensionType || state.draft.dimensionType || 'COLOR_AND_SIZE',
        },
      });
    } catch {
      dispatch({
        type: 'UPDATE_DRAFT',
        payload: {
          categoryId: cat.id,
          categoryName: resolvedName,
          categoryPath: resolvedPath,
        },
      });
    }
  }, [state.draft.dimensionType]);

  const contextValue = useMemo<ProductStudioContextValue>(() => {
    return {
      ...state,
      entryMode: effectiveEntryMode,
      mediaMode: state.draft.mediaMode || state.mediaMode || 'GENERAL',
      isSaveInFlight,
      canSave,
      saveDraft: (effectiveEntryMode === 'edit' || effectiveEntryMode === 'create') ? saveDraft : undefined,
      flushSave,
      clearSaveError,
      isSubmittingModeration,
      moderationError,
      canSubmitModeration,
      submitModeration,
      clearModerationError,
      setViewMode,
      setActiveSection,
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
      selectCategory: handleSelectCategory,
      activeSizeSystemId,
      setActiveSizeSystemId,
      createMediaUrl,
      revokeMediaUrl,
      setSelectedPreviewColorId: (colorId: string | null) =>
        dispatch({ type: 'SET_PREVIEW_COLOR', payload: colorId }),
      setSelectedPreviewSizeValueId: (sizeValueId: string | null) =>
        dispatch({ type: 'SET_PREVIEW_SIZE', payload: sizeValueId }),
      selectedMediaColorId: state.selectedMediaColorId ?? null,
      setSelectedMediaColorId: (colorId: string | 'UNASSIGNED' | null) =>
        dispatch({ type: 'SET_MEDIA_COLOR', payload: colorId }),
    };
  }, [
    state,
    isSaveInFlight,
    canSave,
    saveDraft,
    flushSave,
    clearSaveError,
    isSubmittingModeration,
    moderationError,
    canSubmitModeration,
    submitModeration,
    clearModerationError,
    effectiveEntryMode,
    setViewMode,
    setActiveSection,
    updateDraft,
    markTouched,
    setShowReadinessAttention,
    isFieldAttention,
    readiness,
    categorySchema,
    handleSelectCategory,
    activeSizeSystemId,
    setActiveSizeSystemId,
    createMediaUrl,
    revokeMediaUrl,
  ]);

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

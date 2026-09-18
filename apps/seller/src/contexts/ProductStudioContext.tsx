import React, { createContext, useContext, useReducer, useMemo } from 'react';

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

export interface ProductStudioImage {
  id?: string;
  url: string;
  isMain?: boolean;
  sortOrder?: number;
  colorId?: string | null;
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
  attributes?: Array<{ attributeDefinitionId?: string; name?: string; code?: string; value?: any }>;
  materialComposition?: Array<{ materialId?: string; materialName?: string; percentage?: number }>;
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
}

type ProductStudioAction =
  | { type: 'SET_VIEW_MODE'; payload: ProductStudioViewMode }
  | { type: 'SET_ACTIVE_SECTION'; payload: ProductStudioSection }
  | { type: 'UPDATE_DRAFT'; payload: Partial<ProductStudioDraft> }
  | { type: 'SET_EDITING_FIELD'; payload: string | null }
  | { type: 'RESET_DRAFT' };

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

    case 'RESET_DRAFT':
      return {
        ...state,
        draft: { ...state.initialDraft },
        isDirty: false,
      };

    case 'SET_EDITING_FIELD':
      return {
        ...state,
        editingField: action.payload,
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
  resetDraft: () => void;
}

const ProductStudioContext = createContext<ProductStudioContextValue | undefined>(undefined);

export interface ProductStudioProviderProps {
  entryMode: ProductStudioEntryMode;
  initialDraft?: Partial<ProductStudioDraft>;
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
  });

  const contextValue = useMemo<ProductStudioContextValue>(() => {
    return {
      ...state,
      setViewMode: (mode: ProductStudioViewMode) =>
        dispatch({ type: 'SET_VIEW_MODE', payload: mode }),
      setActiveSection: (section: ProductStudioSection) =>
        dispatch({ type: 'SET_ACTIVE_SECTION', payload: section }),
      updateDraft: (patch: Partial<ProductStudioDraft>) =>
        dispatch({ type: 'UPDATE_DRAFT', payload: patch }),
      setEditingField: (field: string | null) =>
        dispatch({ type: 'SET_EDITING_FIELD', payload: field }),
      resetDraft: () => dispatch({ type: 'RESET_DRAFT' }),
    };
  }, [state]);

  return (
    <ProductStudioContext.Provider value={contextValue}>
      {children}
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

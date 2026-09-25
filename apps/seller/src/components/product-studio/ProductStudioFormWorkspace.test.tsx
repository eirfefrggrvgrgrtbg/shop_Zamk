/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import React from 'react';
import { render, screen, fireEvent, cleanup, waitFor, within, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() =>
      Promise.resolve([
        { id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' },
        { id: 'col-black', nameRu: 'Чёрный', hexValue: '#000000' },
        { id: 'col-beige', nameRu: 'Бежевый', hexValue: '#f5f5dc' },
      ])
    ),
    getSellerCategories: vi.fn().mockImplementation(() =>
      Promise.resolve([
        { id: 'cat-clothing', name: 'Одежда', type: 'FASHION' },
        { id: 'cat-shoes', name: 'Обувь', type: 'SHOES' },
      ])
    ),
    getSellerCategorySchema: vi.fn().mockImplementation(() =>
      Promise.resolve({
        id: 'sch-clothing',
        categoryId: 'cat-clothing',
        dimensionType: 'COLOR_AND_SIZE',
        name: 'Одежда',
        allowedSizeSystems: [{ id: 'sys-eu', name: 'EU', isDefault: true }],
        attributes: [
          { id: 'attr-color', nameRu: 'Цвет', valueSource: 'VARIANT_COLOR', required: true },
          { id: 'attr-size', nameRu: 'Размер', valueSource: 'VARIANT_SIZE', required: true },
        ],
      })
    ),
    getSellerSizeValues: vi.fn().mockImplementation(() =>
      Promise.resolve([
        { id: 'sz-s', sizeSystemId: 'sys-eu', value: 'S', sortOrder: 1 },
        { id: 'sz-m', sizeSystemId: 'sys-eu', value: 'M', sortOrder: 2 },
        { id: 'sz-l', sizeSystemId: 'sys-eu', value: 'L', sortOrder: 3 },
      ])
    ),
  };
});

// Setup jsdom URL blob mocks
if (!globalThis.URL.createObjectURL) {
  globalThis.URL.createObjectURL = vi.fn((file: any) => `blob:http://localhost/${file?.name || 'test'}`);
} else {
  vi.spyOn(globalThis.URL, 'createObjectURL').mockImplementation(
    (file: any) => `blob:http://localhost/${file?.name || 'test'}`
  );
}

if (!globalThis.URL.revokeObjectURL) {
  globalThis.URL.revokeObjectURL = vi.fn();
} else {
  vi.spyOn(globalThis.URL, 'revokeObjectURL').mockImplementation(() => {});
}

import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { BLOCKED_LAST_CELL_TOOLTIP } from './productStudioMatrixHelper';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';

const baseDraft: Partial<ProductStudioDraft> = {
  id: 'prod-1',
  title: 'Классическая футболка',
  brandName: 'ZAMK Studio',
  description: 'Премиальный хлопок.',
  priceCents: 450000,
  categoryId: 'cat-clothing',
  categoryName: 'Одежда',
  dimensionType: 'COLOR_AND_SIZE',
  colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
  variants: [
    {
      id: 'var-black-m',
      colorId: 'col-black',
      colorName: 'Чёрный',
      sizeValueId: 'sz-m',
      size: 'M',
      priceCents: 450000,
      isActive: true,
    },
  ],
};

function TestWrapper({
  initialDraft = baseDraft,
  initialCategorySchema,
  entryMode = 'create',
  contextCallback,
  children,
}: {
  initialDraft?: Partial<ProductStudioDraft>;
  initialCategorySchema?: any;
  entryMode?: 'create' | 'edit';
  contextCallback?: (ctx: ReturnType<typeof useProductStudio>) => void;
  children?: React.ReactNode;
}) {
  function Inspector() {
    const ctx = useProductStudio();
    if (contextCallback) contextCallback(ctx);
    return null;
  }

  function DefaultRenderer() {
    const { activeSection, setActiveSection } = useProductStudio();
    React.useEffect(() => {
      if (activeSection !== 'variants') {
        setActiveSection('variants');
      }
    }, [activeSection, setActiveSection]);

    return <ProductStudioFormWorkspace />;
  }

  return (
    <MemoryRouter>
      <ProductStudioProvider
        entryMode={entryMode}
        initialDraft={initialDraft}
        initialCategorySchema={initialCategorySchema}
      >
        <Inspector />
        {children || <DefaultRenderer />}
      </ProductStudioProvider>
    </MemoryRouter>
  );
}

function SwitcherWrapper({ startInForm = false }: { startInForm?: boolean }) {
  const { viewMode, setViewMode, activeSection, setActiveSection } = useProductStudio();
  const hasInitializedRef = React.useRef(false);
  React.useEffect(() => {
    if (activeSection !== 'variants') {
      setActiveSection('variants');
    }
    if (startInForm && !hasInitializedRef.current) {
      hasInitializedRef.current = true;
      setViewMode('form');
    }
  }, [activeSection, setActiveSection, setViewMode, startInForm]);

  return (
    <div>
      <button data-testid="switch-to-visual-btn" onClick={() => setViewMode('visual')}>
        Визуально
      </button>
      <button data-testid="switch-to-form-btn" onClick={() => setViewMode('form')}>
        Форма
      </button>
      {viewMode === 'form' ? <ProductStudioFormWorkspace /> : <ProductStudioVisualWorkspace />}
    </div>
  );
}

describe('ProductStudioFormWorkspace - Variant Editing Parity (PS.R4B3.1C4C3B3A)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe('1. Form Colors Editor (COLOR_ONLY)', () => {
    const colorOnlyDraft: Partial<ProductStudioDraft> = {
      ...baseDraft,
      dimensionType: 'COLOR_ONLY',
      variants: [{ id: 'var-black-m', colorId: 'col-black', colorName: 'Чёрный', isActive: true }],
    };

    it('renders color chips from draft', async () => {
      render(<TestWrapper initialDraft={colorOnlyDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-colors-editor-section')).toBeTruthy();
      });
      expect(screen.getByTestId('form-color-chip-col-black')).toBeTruthy();
      // Color name appears in multiple places (chip + variant summary); check by testid not text
      expect(screen.getAllByText('Чёрный').length).toBeGreaterThanOrEqual(1);
    });

    it('adds a new color (White) and updates draft.variants via reconciler into Cartesian matrix', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={colorOnlyDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-add-color-btn')).toBeTruthy();
      });

      // Click Add Color button
      fireEvent.click(screen.getByTestId('form-add-color-btn'));
      expect(screen.getByTestId('form-color-popover')).toBeTruthy();

      // Check White checkbox
      const whiteOption = screen.getByTestId('form-color-option-col-white');
      fireEvent.click(whiteOption);

      // Click Apply
      fireEvent.click(screen.getByTestId('form-color-popover-apply-btn'));

      // Verify colors and variants updated
      await waitFor(() => {
        expect(latestCtx?.draft.colors).toHaveLength(2);
        expect(latestCtx?.draft.colors?.map((c: any) => c.id)).toEqual(['col-black', 'col-white']);
      });

      // Matrix must now be 2 colors = 2 active variants
      const activeVariants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
      expect(activeVariants).toHaveLength(2);

      // Existing variant ID preserved
      const blackM = activeVariants?.find((v: any) => v.colorId === 'col-black');
      expect(blackM?.id).toBe('var-black-m');

      // New variant generated for White
      const whiteM = activeVariants?.find((v: any) => v.colorId === 'col-white');
      expect(whiteM).toBeTruthy();
      expect(whiteM?.colorName).toBe('Белый');

      // Draft must be marked dirty
      expect(latestCtx?.isDirty).toBe(true);
    });

    it('removes a color (Black) and deactivates its variants', async () => {
      const twoColorDraft: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: 'COLOR_ONLY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { id: 'var-black-m', colorId: 'col-black', isActive: true },
          { id: 'var-white-m', colorId: 'col-white', isActive: true },
        ],
      };

      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={twoColorDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-remove-color-col-black')).toBeTruthy();
      });

      // Remove Black
      fireEvent.click(screen.getByTestId('form-remove-color-col-black'));

      await waitFor(() => {
        expect(latestCtx?.draft.colors?.map((c: any) => c.id)).toEqual(['col-white']);
      });

      // Current draft.variants contains ONLY White x M (active variants only)
      expect(latestCtx?.draft.variants).toHaveLength(1);
      expect(latestCtx?.draft.variants?.[0].colorId).toBe('col-white');
      expect(latestCtx?.draft.variants?.[0].isActive).toBe(true);

      // Black x M must be completely absent from draft.variants (no inactive tombstones)
      expect(latestCtx?.draft.variants?.find((v: any) => v.id === 'var-black-m')).toBeUndefined();
      expect(latestCtx?.draft.variants?.some((v: any) => v.isActive === false)).toBe(false);

      expect(latestCtx?.isDirty).toBe(true);
    });

    it('does NOT resurrect inactive variants when adding colors', async () => {
      const draftWithInactive: Partial<ProductStudioDraft> = {
        ...baseDraft,
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [
          { id: 'var-black-m', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M', isActive: true },
          { id: 'var-black-s-dead', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S', isActive: false },
        ],
      };

      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={draftWithInactive}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });

      // Add White
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      fireEvent.click(screen.getByTestId('form-color-option-col-white'));
      fireEvent.click(screen.getByTestId('form-color-popover-apply-btn'));

      await waitFor(() => {
        expect(latestCtx?.draft.colors).toHaveLength(2);
      });

      // S was inactive, so current active sizes is only M!
      // Matrix must NOT include S!
      // Inactive S is NOT resurrected and is NOT in current draft.variants truth
      expect(latestCtx?.draft.variants).toHaveLength(2); // Black x M, White x M
      expect(latestCtx?.draft.variants?.some((v: any) => v.sizeValueId === 'sz-s')).toBe(false);
      expect(latestCtx?.draft.variants?.find((v: any) => v.id === 'var-black-s-dead')).toBeUndefined();
      expect(latestCtx?.draft.variants?.some((v: any) => v.isActive === false)).toBe(false);
    });
  });

  describe('2. Form Sizes Editor (SIZE_ONLY)', () => {
    const sizeOnlyDraft: Partial<ProductStudioDraft> = {
      ...baseDraft,
      dimensionType: 'SIZE_ONLY',
      colors: [],
      variants: [{ id: 'var-black-m', sizeValueId: 'sz-m', size: 'M', isActive: true }],
    };

    it('renders size chips from draft', async () => {
      render(<TestWrapper initialDraft={sizeOnlyDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-sizes-editor-section')).toBeTruthy();
      });
      expect(screen.getByTestId('form-size-chip-sz-m')).toBeTruthy();
      // Size label appears in multiple places (chip + variant summary); verify chip by testid
      expect(screen.getAllByText('M').length).toBeGreaterThanOrEqual(1);
    });

    it('adds a new size (L) and updates draft.variants via reconciler', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={sizeOnlyDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-add-size-btn')).toBeTruthy();
      });

      // Open Size Popover
      fireEvent.click(screen.getByTestId('form-add-size-btn'));
      expect(screen.getByTestId('form-size-popover')).toBeTruthy();

      // Wait for size options to load and select L
      await waitFor(() => {
        expect(screen.getByTestId('form-size-option-sz-l')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('form-size-option-sz-l'));

      // Click Apply
      fireEvent.click(screen.getByTestId('form-size-popover-apply-btn'));

      await waitFor(() => {
        const activeVariants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVariants).toHaveLength(2);
      });

      const activeVariants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
      const sizes = activeVariants?.map((v: any) => v.sizeValueId);
      expect(sizes).toContain('sz-m');
      expect(sizes).toContain('sz-l');

      // Preserves existing variant ID for M
      const mVar = activeVariants?.find((v: any) => v.sizeValueId === 'sz-m');
      expect(mVar?.id).toBe('var-black-m');

      expect(latestCtx?.isDirty).toBe(true);
    });

    it('removes a size (M) and deactivates its variants', async () => {
      const twoSizesDraft: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: 'SIZE_ONLY',
        colors: [],
        variants: [
          { id: 'var-black-m', sizeValueId: 'sz-m', size: 'M', isActive: true },
          { id: 'var-black-l', sizeValueId: 'sz-l', size: 'L', isActive: true },
        ],
      };

      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={twoSizesDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-remove-size-sz-m')).toBeTruthy();
      });

      // Remove M
      fireEvent.click(screen.getByTestId('form-remove-size-sz-m'));

      await waitFor(() => {
        expect(latestCtx?.draft.variants).toHaveLength(1);
        expect(latestCtx?.draft.variants?.[0].sizeValueId).toBe('sz-l');
      });

      // Current draft.variants contains ONLY L (active variants only)
      expect(latestCtx?.draft.variants).toHaveLength(1);
      expect(latestCtx?.draft.variants?.[0].sizeValueId).toBe('sz-l');
      expect(latestCtx?.draft.variants?.[0].isActive).toBe(true);

      // var-black-m must be completely absent from draft.variants (no inactive tombstones)
      expect(latestCtx?.draft.variants?.find((v: any) => v.id === 'var-black-m')).toBeUndefined();
      expect(latestCtx?.draft.variants?.some((v: any) => v.isActive === false)).toBe(false);

      expect(latestCtx?.isDirty).toBe(true);
    });
  });

  describe('3. Cross-Tab Synchronization (Form <-> Visual)', () => {
    function SwitcherWrapper({ startInForm = false }: { startInForm?: boolean }) {
      const { viewMode, setViewMode, activeSection, setActiveSection } = useProductStudio();
      const hasInitializedRef = React.useRef(false);
      React.useEffect(() => {
        if (activeSection !== 'variants') {
          setActiveSection('variants');
        }
        if (startInForm && !hasInitializedRef.current) {
          hasInitializedRef.current = true;
          setViewMode('form');
        }
      }, [activeSection, setActiveSection, setViewMode, startInForm]);

      return (
        <div>
          <button data-testid="switch-to-visual-btn" onClick={() => setViewMode('visual')}>
            Визуально
          </button>
          <button data-testid="switch-to-form-btn" onClick={() => setViewMode('form')}>
            Форма
          </button>
          {viewMode === 'form' ? <ProductStudioFormWorkspace /> : <ProductStudioVisualWorkspace />}
        </div>
      );
    }

    it('adding color in Form is immediately reflected when switching to Visual', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        >
          <SwitcherWrapper startInForm={true} />
        </TestWrapper>
      );

      // Start in Form
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });

      // Add White
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      fireEvent.click(screen.getByTestId('form-color-option-col-white'));
      fireEvent.click(screen.getByTestId('form-color-popover-apply-btn'));

      await waitFor(() => {
        expect(latestCtx?.draft.colors).toHaveLength(2);
      });

      // Switch to Visual
      fireEvent.click(screen.getByTestId('switch-to-visual-btn'));

      // Both colors must be visible in Visual workspace
      await waitFor(() => {
        expect(screen.getByRole('radio', { name: /Белый/i })).toBeTruthy();
        expect(screen.getByRole('radio', { name: /Чёрный/i })).toBeTruthy();
      });
    });

    it('adding size in Visual is immediately reflected when switching to Form', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        >
          <SwitcherWrapper startInForm={true} />
        </TestWrapper>
      );

      // Switch to Visual first
      fireEvent.click(screen.getByTestId('switch-to-visual-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('size-manage-btn')).toBeTruthy();
      });

      // Open Visual size popover
      fireEvent.click(screen.getByTestId('size-manage-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('size-popover')).toBeTruthy();
      });

      // Select L in Visual popover
      const lButton = screen.getByRole('button', { name: /^L$/i });
      fireEvent.click(lButton);
      fireEvent.click(screen.getByTestId('size-popover-apply-btn'));

      await waitFor(() => {
        const activeVariants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVariants).toHaveLength(2);
      });

      // Switch back to Form
      fireEvent.click(screen.getByTestId('switch-to-form-btn'));

      // Form must show both M and L in matrix table
      await waitFor(() => {
        const matrix = screen.getByTestId('form-matrix-editor-section');
        expect(within(matrix).getByText('M')).toBeTruthy();
        expect(within(matrix).getByText('L')).toBeTruthy();
      });
    });
  });

  describe('4. Dimension Type Support (PS.R4B3.1C4C3B3A2)', () => {
    it('1. COLOR_AND_SIZE: shows matrix editor, hides separate colors and sizes cards', async () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, dimensionType: 'COLOR_AND_SIZE' }} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
        expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
        expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
      });
    });

    it('2. COLOR_ONLY: shows color editor only, hides size editor', async () => {
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            dimensionType: 'COLOR_ONLY',
            variants: [{ id: 'var-1', colorId: 'col-black', colorName: 'Чёрный', isActive: true }],
          }}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('form-colors-editor-section')).toBeTruthy();
        expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
      });
    });

    it('3. SIZE_ONLY: shows size editor only, hides color editor', async () => {
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            dimensionType: 'SIZE_ONLY',
            colors: [],
            variants: [{ id: 'var-1', sizeValueId: 'sz-m', size: 'M', isActive: true }],
          }}
        />
      );
      await waitFor(() => {
        expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
        expect(screen.getByTestId('form-sizes-editor-section')).toBeTruthy();
      });
    });

    it('4. SINGLE_VARIANT: hides both color and size editors', async () => {
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            dimensionType: 'SINGLE_VARIANT',
            colors: [],
            variants: [{ id: 'var-1', isActive: true }],
          }}
        />
      );
      await waitFor(() => {
        expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
        expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
      });
    });

    it('5. draft.dimensionType absent, categorySchema = SIZE_ONLY: shows size editor only', async () => {
      const draftWithoutDim: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: undefined,
        categoryId: 'cat-size-only',
      };
      const schemaSizeOnly = {
        id: 'sch-size-only',
        categoryId: 'cat-size-only',
        dimensionType: 'SIZE_ONLY',
        name: 'Обувь',
        allowedSizeSystems: [{ id: 'sys-eu', name: 'EU', isDefault: true }],
        attributes: [],
      };

      render(
        <TestWrapper
          initialDraft={draftWithoutDim}
          initialCategorySchema={schemaSizeOnly}
        />
      );

      await waitFor(() => {
        expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
        expect(screen.getByTestId('form-sizes-editor-section')).toBeTruthy();
      });
    });

    it('6. draft.dimensionType absent, categorySchema absent: neither editor rendered, no Cartesian generation', async () => {
      const draftNoDimNoCat: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: undefined,
        categoryId: undefined,
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [{ id: 'var-1', isActive: true }],
      };

      render(
        <TestWrapper
          initialDraft={draftNoDimNoCat}
          initialCategorySchema={null}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
      });

      expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
      expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
      expect(screen.getByText('Укажите категорию товара для настройки вариантов.')).toBeTruthy();
    });

    it('7. draft.dimensionType = UNKNOWN: fails closed (neither editor rendered, no Cartesian generation)', async () => {
      const draftUnknownDim: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: 'UNKNOWN_DIM' as any,
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [{ id: 'var-1', isActive: true }],
      };

      render(
        <TestWrapper
          initialDraft={draftUnknownDim}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
      });

      expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
      expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
    });

    it('8. categorySchema.dimensionType = UNKNOWN with no draft dimension: fails closed (neither editor rendered, no Cartesian generation)', async () => {
      const draftNoDim: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: undefined,
        categoryId: 'cat-custom',
      };
      const schemaUnknownDim = {
        id: 'sch-custom',
        categoryId: 'cat-custom',
        dimensionType: 'UNSUPPORTED_DIM_TYPE',
        name: 'Кастом',
        attributes: [],
      };

      render(
        <TestWrapper
          initialDraft={draftNoDim}
          initialCategorySchema={schemaUnknownDim}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
      });

      expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
      expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
    });

    it('9. unknown dimension must never call reconcile as COLOR_AND_SIZE', async () => {
      const draftUnknown: Partial<ProductStudioDraft> = {
        ...baseDraft,
        dimensionType: 'INVALID' as any,
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [{ id: 'var-1', colorId: 'col-black', isActive: true }],
      };

      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={draftUnknown}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
      });

      // Editors are hidden
      expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
      expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();

      // Variants remain untouched in context (not expanded to Cartesian matrix)
      expect(latestCtx?.draft.variants).toHaveLength(1);
      expect(latestCtx?.draft.variants[0].id).toBe('var-1');
    });
  });

  describe('5. Save Payload Invariants', () => {
    it('generates canonical variant payload containing all active Cartesian combinations configured in Form', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={baseDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });

      // Add White in Form
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      fireEvent.click(screen.getByTestId('form-color-option-col-white'));
      fireEvent.click(screen.getByTestId('form-color-popover-apply-btn'));

      // Add L in Form
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-size-btn')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('matrix-add-size-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-size-option-sz-l')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('form-size-option-sz-l'));
      fireEvent.click(screen.getByTestId('form-size-popover-apply-btn'));

      // Reconciler should produce 2 colors x 2 sizes = 4 active variants
      await waitFor(() => {
        const activeVariants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVariants).toHaveLength(4);
      });

      const variants = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false) || [];

      // Check all combinations exist
      const combinations = variants.map((v: any) => `${v.colorId}x${v.sizeValueId}`);
      expect(combinations).toContain('col-blackxsz-m');
      expect(combinations).toContain('col-blackxsz-l');
      expect(combinations).toContain('col-whitexsz-m');
      expect(combinations).toContain('col-whitexsz-l');

      // Every variant has color, size, and price
      for (const v of variants) {
        expect(v.colorId).toBeDefined();
        expect(v.sizeValueId).toBeDefined();
        expect(v.isActive).toBe(true);
      }
    });
  });

  describe('5. Sparse Matrix Table Component & Cross-Workspace State (PS.R4B3.1C4C3B3B-R2)', () => {
    const twoByTwoDraft: Partial<ProductStudioDraft> = {
      dimensionType: 'COLOR_AND_SIZE',
      categoryId: 'cat-hoodies',
      colors: [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ],
      variants: [
        { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
        { id: 'v-b-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
        { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
        { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
      ],
    };

    it('1. matrix renders all configured rows and columns', async () => {
      render(<TestWrapper initialDraft={twoByTwoDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });

      // Headers for columns M and L
      const matrix = screen.getByTestId('form-matrix-editor-section');
      expect(within(matrix).getByText('M')).toBeTruthy();
      expect(within(matrix).getByText('L')).toBeTruthy();

      // Cells for all 4 combinations
      expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-black-sz-l')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-white-sz-l')).toBeTruthy();
    });

    it('2 & 3 & 4. ordinary active cell can toggle OFF, remains rendered, and can toggle ON', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={twoByTwoDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      });

      // 2. Toggle White/M OFF
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-m'));

      await waitFor(() => {
        const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVars).toHaveLength(3);
        expect(activeVars?.some((v: any) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m')).toBe(false);
      });

      // 3. OFF cell remains rendered in DOM
      const offCell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
      expect(offCell).toBeTruthy();

      // 4. Toggle White/M back ON
      fireEvent.click(offCell);

      await waitFor(() => {
        const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVars).toHaveLength(4);
        expect(activeVars?.some((v: any) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m')).toBe(true);
      });
    });

    it('5. last cell in row cannot toggle OFF (mutation blocked, tooltip present)', async () => {
      let latestCtx: any = null;
      // Draft where Black has only M (1 variant), but White has M and L (2 variants)
      const blackSingleRowDraft: Partial<ProductStudioDraft> = {
        dimensionType: 'COLOR_AND_SIZE',
        categoryId: 'cat-hoodies',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
          { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
          { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
        ],
      };

      render(
        <TestWrapper
          initialDraft={blackSingleRowDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
      });

      const cell = screen.getByTestId('form-matrix-cell-col-black-sz-m');
      expect(cell.getAttribute('title')).toBe(BLOCKED_LAST_CELL_TOOLTIP);

      // Attempt to toggle OFF
      fireEvent.click(cell);

      // Mutation is blocked: draft still has 3 variants and Black/M remains active
      const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
      expect(activeVars).toHaveLength(3);
      expect(activeVars?.some((v: any) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toBe(true);
    });

    it('6. last cell in column cannot toggle OFF (mutation blocked, tooltip present)', async () => {
      let latestCtx: any = null;
      // Draft where L has only Black (1 variant), but M has Black and White (2 variants)
      const lSingleColDraft: Partial<ProductStudioDraft> = {
        dimensionType: 'COLOR_AND_SIZE',
        categoryId: 'cat-hoodies',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
          { id: 'v-b-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
          { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
        ],
      };

      render(
        <TestWrapper
          initialDraft={lSingleColDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-black-sz-l')).toBeTruthy();
      });

      const cell = screen.getByTestId('form-matrix-cell-col-black-sz-l');
      expect(cell.getAttribute('title')).toBe(BLOCKED_LAST_CELL_TOOLTIP);

      // Attempt to toggle OFF
      fireEvent.click(cell);

      // Mutation is blocked: draft still has 3 variants and Black/L remains active
      const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
      expect(activeVars).toHaveLength(3);
      expect(activeVars?.some((v: any) => v.colorId === 'col-black' && v.sizeValueId === 'sz-l')).toBe(true);
    });

    it('7. removing whole color via color selector removes all variants for that color', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={twoByTwoDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });

      // Open color selector from matrix color +
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-color-popover')).toBeTruthy();
      });

      // Uncheck White
      fireEvent.click(screen.getByTestId('form-color-option-col-white'));
      fireEvent.click(screen.getByTestId('form-color-popover-apply-btn'));

      await waitFor(() => {
        const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVars).toHaveLength(2);
        expect(activeVars?.every((v: any) => v.colorId === 'col-black')).toBe(true);
      });
    });

    it('8. removing whole size via size selector removes all variants for that size', async () => {
      let latestCtx: any = null;
      render(
        <TestWrapper
          initialDraft={twoByTwoDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-size-btn')).toBeTruthy();
      });

      // Open size selector from matrix size +
      fireEvent.click(screen.getByTestId('matrix-add-size-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-size-popover')).toBeTruthy();
      });

      // Uncheck L
      fireEvent.click(screen.getByTestId('form-size-option-sz-l'));
      fireEvent.click(screen.getByTestId('form-size-popover-apply-btn'));

      await waitFor(() => {
        const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVars).toHaveLength(2);
        expect(activeVars?.every((v: any) => v.sizeValueId === 'sz-m')).toBe(true);
      });
    });

    it('Section 14: Form Grey/M OFF disables Grey in Visual for size M, and re-enabling in Form enables immediately in Visual', async () => {
      let latestCtx: any = null;
      const initialSparseDraft: Partial<ProductStudioDraft> = {
        dimensionType: 'COLOR_AND_SIZE',
        categoryId: 'cat-hoodies',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', priceCents: 450000, isActive: true },
          { id: 'v-b-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
          { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', priceCents: 450000, isActive: true },
          // White/M is OFF (absent)
        ],
      };

      render(
        <TestWrapper
          initialDraft={initialSparseDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        >
          <SwitcherWrapper startInForm={true} />
        </TestWrapper>
      );

      // Start in Form, switch to Visual
      await waitFor(() => {
        expect(screen.getByTestId('switch-to-visual-btn')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('switch-to-visual-btn'));

      // In Visual, select size M
      await waitFor(() => {
        expect(screen.getByTestId('size-button-M')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('size-button-M'));

      // Black is enabled, White is disabled/incompatible because White/M is OFF
      const blackBtn = screen.getByTestId('color-swatch-col-black') as HTMLButtonElement;
      const whiteBtn = screen.getByTestId('color-swatch-col-white') as HTMLButtonElement;
      expect(blackBtn.disabled).toBe(false);
      expect(whiteBtn.disabled).toBe(true);

      // Switch back to Form
      fireEvent.click(screen.getByTestId('switch-to-form-btn'));

      // In Form, toggle White/M ON
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-m'));

      await waitFor(() => {
        const activeVars = latestCtx?.draft.variants?.filter((v: any) => v.isActive !== false);
        expect(activeVars).toHaveLength(4);
      });

      // Switch back to Visual
      fireEvent.click(screen.getByTestId('switch-to-visual-btn'));

      // Both Black and White are now enabled for size M immediately without save/reload
      await waitFor(() => {
        const updatedWhiteBtn = screen.getByTestId('color-swatch-col-white') as HTMLButtonElement;
        expect(updatedWhiteBtn.disabled).toBe(false);
      });
    });
  });

  describe('6. PS.R4B3.1C4C3B3B-R3 — Final Variant Matrix UX (Form)', () => {
    const r3Draft: Partial<ProductStudioDraft> = {
      dimensionType: 'COLOR_AND_SIZE',
      categoryId: 'cat-clothing',
      colors: [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ],
      variants: [
        { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-b-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', isActive: true },
        { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', isActive: true },
      ],
    };

    it('1. COLOR_AND_SIZE: matrix visible', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });
    });

    it('2. separate large "Цвета товара" card absent', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });
      expect(screen.queryByTestId('form-colors-editor-section')).toBeNull();
    });

    it('3. separate large "Размеры товара" card absent', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });
      expect(screen.queryByTestId('form-sizes-editor-section')).toBeNull();
    });

    it('4. size-axis + visible to right of headers', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-size-btn')).toBeTruthy();
      });
      const sizeBtn = screen.getByTestId('matrix-add-size-btn');
      expect(sizeBtn.closest('th')).toBeTruthy();
    });

    it('5. color-axis + visible below rows', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });
      const colorBtn = screen.getByTestId('matrix-add-color-btn');
      expect(colorBtn.closest('td')).toBeTruthy();
    });

    it('6. size + opens existing size editor', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-size-btn')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('matrix-add-size-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-size-popover')).toBeTruthy();
        expect(screen.getByTestId('form-size-popover-apply-btn')).toBeTruthy();
      });
    });

    it('7. color + opens existing color editor', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-color-popover')).toBeTruthy();
        expect(screen.getByTestId('form-color-popover-apply-btn')).toBeTruthy();
      });
    });

    it('8. active matrix cell uses compact active visual', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
      });
      const cell = screen.getByTestId('form-matrix-cell-col-black-sz-m');
      // Compact control: has check icon and compact button classes
      expect(cell.querySelector('svg')).toBeTruthy();
      expect(cell.className).toContain('w-7');
      expect(cell.className).toContain('h-7');
    });

    it('9. inactive matrix cell remains visible', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      });
      const cell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
      // Toggle to inactive
      fireEvent.click(cell);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      });
      // Inactive cell remains visible in DOM with compact control
      const inactiveCell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
      expect(inactiveCell).toBeTruthy();
      expect(inactiveCell.querySelector('svg')).toBeNull();
      expect(inactiveCell.className).toContain('w-7');
      expect(inactiveCell.className).toContain('h-7');
    });

    it('10. no "ВКЛ" / "ВЫКЛ" text used as primary cell UI', async () => {
      render(<TestWrapper initialDraft={r3Draft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
      });
      const cell = screen.getByTestId('form-matrix-cell-col-black-sz-m');
      expect(cell.textContent).not.toContain('ВКЛ');
      expect(cell.textContent).not.toContain('ВЫКЛ');

      // Also after toggling off
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-m'));
      await waitFor(() => {
        const offCell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
        expect(offCell.textContent).not.toContain('ВКЛ');
        expect(offCell.textContent).not.toContain('ВЫКЛ');
      });
    });

    it('11. blocked last cell remains blocked', async () => {
      let latestCtx: any = null;
      const singleCellDraft: Partial<ProductStudioDraft> = {
        dimensionType: 'COLOR_AND_SIZE',
        categoryId: 'cat-clothing',
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [
          { id: 'v-b-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', isActive: true },
        ],
      };
      render(
        <TestWrapper
          initialDraft={singleCellDraft}
          contextCallback={(ctx) => {
            latestCtx = ctx;
          }}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
      });
      const cell = screen.getByTestId('form-matrix-cell-col-black-sz-m');
      expect(cell.getAttribute('title')).toBe(BLOCKED_LAST_CELL_TOOLTIP);

      // Attempt to toggle
      fireEvent.click(cell);

      // Remains active and blocked
      expect(latestCtx?.draft.variants?.[0].isActive).toBe(true);
      expect(screen.getByTestId('form-matrix-cell-col-black-sz-m')).toBeTruthy();
    });
  });

  describe('7. PS.R4B3.1C4C3B3B-R3A — Stable Matrix Geometry, Canonical Labels & Popover Exclusivity', () => {
    const multiRowColDraft: Partial<ProductStudioDraft> = {
      dimensionType: 'COLOR_AND_SIZE',
      categoryId: 'cat-clothing',
      colors: [
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        { id: 'col-beige', name: 'Бежевый', hex: '#f5f5dc' },
      ],
      variants: [
        { id: 'v-w-s', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-s', size: 'S', isActive: true },
        { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', isActive: true },
        { id: 'v-b-s', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-s', size: 'S', isActive: true },
        { id: 'v-b-m', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-b-l', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-l', size: 'L', isActive: true },
      ],
    };

    it('1. toggling cell OFF and ON preserves column headers and order (does NOT reorder sizes)', async () => {
      render(<TestWrapper initialDraft={multiRowColDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-size-header-sz-s')).toBeTruthy();
        expect(screen.getByTestId('matrix-size-header-sz-m')).toBeTruthy();
        expect(screen.getByTestId('matrix-size-header-sz-l')).toBeTruthy();
      });

      // Initial header order: S, M, L
      const getHeaderTexts = () => [
        screen.getByTestId('matrix-size-header-sz-s').textContent?.trim(),
        screen.getByTestId('matrix-size-header-sz-m').textContent?.trim(),
        screen.getByTestId('matrix-size-header-sz-l').textContent?.trim(),
      ];
      expect(getHeaderTexts()).toEqual(['S', 'M', 'L']);

      // Toggle White / M OFF
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-m'));

      await waitFor(() => {
        // Headers remain S, M, L in exact order
        expect(getHeaderTexts()).toEqual(['S', 'M', 'L']);
      });

      // Both White and Beige rows remain rendered
      expect(screen.getByTestId('form-matrix-cell-col-white-sz-s')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-white-sz-l')).toBeTruthy();
      expect(screen.getByTestId('form-matrix-cell-col-beige-sz-m')).toBeTruthy();

      // Toggle White / M back ON
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-m'));

      await waitFor(() => {
        expect(getHeaderTexts()).toEqual(['S', 'M', 'L']);
      });

      // Toggle White / S OFF then Beige / L OFF: headers remain stable
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-white-sz-s'));
      fireEvent.click(screen.getByTestId('form-matrix-cell-col-beige-sz-l'));

      await waitFor(() => {
        expect(getHeaderTexts()).toEqual(['S', 'M', 'L']);
      });
    });

    it('2. size headers and size popover display human-readable labels and never raw UUIDs', async () => {
      const uuidDraft: Partial<ProductStudioDraft> = {
        dimensionType: 'COLOR_AND_SIZE',
        categoryId: 'cat-clothing',
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
        variants: [
          // size contains raw UUID, but sizeValueId matches dictionary sz-m (which has value 'M')
          { id: 'v-b-m', colorId: 'col-black', sizeValueId: 'sz-m', size: '1ed6ddd3-bf7b-4029-9e8a-028fec3a4b95', isActive: true },
        ],
      };

      render(<TestWrapper initialDraft={uuidDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-size-header-sz-m').textContent?.trim()).toBe('M');
      });

      const header = screen.getByTestId('matrix-size-header-sz-m');
      expect(header.textContent).not.toContain('1ed6ddd3');

      // Open Size Popover
      fireEvent.click(screen.getByTestId('matrix-add-size-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-size-popover')).toBeTruthy();
      });

      // Check size options in popover
      const optionS = screen.getByTestId('form-size-option-sz-s');
      const optionM = screen.getByTestId('form-size-option-sz-m');
      const optionL = screen.getByTestId('form-size-option-sz-l');
      expect(optionS.closest('label')?.textContent?.trim()).toBe('S');
      expect(optionM.closest('label')?.textContent?.trim()).toBe('M');
      expect(optionL.closest('label')?.textContent?.trim()).toBe('L');
      expect(screen.getByTestId('form-size-popover').textContent).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}/i);
    });

    it('3. active and inactive matrix cells share identical 28x28px geometry', async () => {
      render(<TestWrapper initialDraft={multiRowColDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-cell-col-white-sz-m')).toBeTruthy();
      });

      const activeCell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
      expect(activeCell.className).toContain('w-7');
      expect(activeCell.className).toContain('h-7');
      expect(activeCell.className).toContain('rounded-md');

      // Toggle OFF
      fireEvent.click(activeCell);

      await waitFor(() => {
        const inactiveCell = screen.getByTestId('form-matrix-cell-col-white-sz-m');
        expect(inactiveCell.className).toContain('w-7');
        expect(inactiveCell.className).toContain('h-7');
        expect(inactiveCell.className).toContain('rounded-md');
      });
    });

    it('4. popovers are mutually exclusive and close on Escape key', async () => {
      render(<TestWrapper initialDraft={multiRowColDraft} />);
      await waitFor(() => {
        expect(screen.getByTestId('matrix-add-color-btn')).toBeTruthy();
        expect(screen.getByTestId('matrix-add-size-btn')).toBeTruthy();
      });

      // Open Color popover
      fireEvent.click(screen.getByTestId('matrix-add-color-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-color-popover')).toBeTruthy();
      });
      expect(screen.queryByTestId('form-size-popover')).toBeNull();

      // Open Size popover -> Color popover must close
      fireEvent.click(screen.getByTestId('matrix-add-size-btn'));
      await waitFor(() => {
        expect(screen.getByTestId('form-size-popover')).toBeTruthy();
      });
      expect(screen.queryByTestId('form-color-popover')).toBeNull();

      // Press Escape -> Size popover must close
      fireEvent.keyDown(window, { key: 'Escape' });
      await waitFor(() => {
        expect(screen.queryByTestId('form-size-popover')).toBeNull();
        expect(screen.queryByTestId('form-color-popover')).toBeNull();
      });
    });
  });

  describe('8. PS.R4B3.1C4C3B3C — Category Prominence + Matrix Layout Polish (Sections 20, 21, 22)', () => {
    const draft3x3: Partial<ProductStudioDraft> = {
      dimensionType: 'COLOR_AND_SIZE',
      categoryId: 'cat-clothing',
      categoryName: 'Одежда',
      colors: [
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-beige', name: 'Бежевый', hex: '#f5f5dc' },
      ],
      variants: [
        { id: 'v-w-s', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-s', size: 'S', isActive: true },
        { id: 'v-w-m', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-w-l', colorId: 'col-white', colorName: 'Белый', sizeValueId: 'sz-l', size: 'L', isActive: true },
        { id: 'v-k-s', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-s', size: 'S', isActive: true },
        { id: 'v-k-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-k-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', isActive: true },
        { id: 'v-b-s', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-s', size: 'S', isActive: true },
        { id: 'v-b-m', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-m', size: 'M', isActive: true },
        { id: 'v-b-l', colorId: 'col-beige', colorName: 'Бежевый', sizeValueId: 'sz-l', size: 'L', isActive: true },
      ],
    };

    it('20. Characteristics category does not act as separate state and has [Изменить] invoking canonical selector', async () => {
      let currentCtx: any;
      function TestContent() {
        currentCtx = useProductStudio();
        return <ProductStudioFormWorkspace />;
      }

      render(
        <TestWrapper initialDraft={draft3x3}>
          <TestContent />
        </TestWrapper>
      );

      // Switch to characteristics tab
      act(() => {
        currentCtx.setActiveSection('characteristics');
      });

      await waitFor(() => {
        expect(screen.getByTestId('form-characteristics-change-category-btn')).toBeTruthy();
      });

      const changeBtn = screen.getByTestId('form-characteristics-change-category-btn');
      expect(changeBtn.textContent).toBe('[Изменить]');

      // Click [Изменить] in characteristics
      act(() => {
        fireEvent.click(changeBtn);
      });

      // Verify canonical modal was opened in context
      expect(currentCtx.isCategoryModalOpen).toBe(true);
    });

    it('21. Matrix compact geometry: w-fit max-w-full, compact columns, "Добавить цвет" label, and active count badge', async () => {
      render(<TestWrapper initialDraft={draft3x3} />);

      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });

      // 1. Matrix container is compact (w-fit max-w-full), not 100% full-width empty card
      const section = screen.getByTestId('form-matrix-editor-section');
      expect(section.className).toContain('w-fit');
      expect(section.className).toContain('max-w-full');

      // 2. Size column headers have compact 64-80px geometry
      const sizeHeaderS = screen.getByTestId('matrix-size-header-sz-s');
      expect(sizeHeaderS.className).toContain('w-[72px]');
      expect(sizeHeaderS.className).toContain('min-w-[64px]');
      expect(sizeHeaderS.className).toContain('max-w-[80px]');

      // 3. Add-size button is inside th directly following last size column
      const addSizeBtn = screen.getByTestId('matrix-add-size-btn');
      const addSizeTh = addSizeBtn.closest('th');
      expect(addSizeTh).toBeTruthy();
      expect(addSizeTh?.className).toContain('w-[44px]');
      expect(addSizeTh?.className).toContain('min-w-[40px]');
      expect(addSizeTh?.className).toContain('max-w-[48px]');

      // 4. Add color button says "Добавить цвет"
      const addColorBtn = screen.getByTestId('matrix-add-color-btn');
      expect(addColorBtn.textContent).toContain('Добавить цвет');

      // 5. Active count badge shows "9 из 9 активны" for 3x3 matrix
      const countBadge = screen.getByTestId('matrix-active-count-badge');
      expect(countBadge.textContent).toContain('9 из 9 активны');

      // 6. Toggling one cell OFF updates active count to "8 из 9 активны"
      const cell = screen.getByTestId('form-matrix-cell-col-white-sz-s');
      fireEvent.click(cell);

      await waitFor(() => {
        expect(screen.getByTestId('matrix-active-count-badge').textContent).toContain('8 из 9 активны');
      });
    });

    it('22. Many sizes overflow: table wrapper has overflow-x-auto and first column preserves readable min-width', async () => {
      // 10 sizes draft
      const manySizesVariants = [];
      const manySizes = [
        'XXS', 'XS', 'S', 'M', 'L', 'XL', '2XL', '3XL', '4XL', '5XL'
      ].map((val, idx) => ({
        id: `sz-${idx}`,
        label: val,
      }));

      for (const s of manySizes) {
        manySizesVariants.push({
          id: `var-white-${s.id}`,
          colorId: 'col-white',
          colorName: 'Белый',
          sizeValueId: s.id,
          size: s.label,
          priceCents: 450000,
          isActive: true,
        });
      }

      const manySizesDraft: Partial<ProductStudioDraft> = {
        ...draft3x3,
        colors: [{ id: 'col-white', name: 'Белый', hex: '#ffffff' }],
        variants: manySizesVariants,
      };

      render(<TestWrapper initialDraft={manySizesDraft} />);

      await waitFor(() => {
        expect(screen.getByTestId('form-matrix-editor-section')).toBeTruthy();
      });

      // Verify wrapper has overflow-x-auto
      const table = screen.getByRole('table');
      const scrollWrapper = table.parentElement;
      expect(scrollWrapper?.className).toContain('overflow-x-auto');
      expect(scrollWrapper?.className).toContain('max-w-full');

      // Verify first column retains stable min-width for readability
      const firstColHeader = screen.getByText('Цвет \\ Размер');
      expect(firstColHeader.className).toContain('min-w-[140px]');
      expect(firstColHeader.className).toContain('w-[160px]');
    });
  });
});

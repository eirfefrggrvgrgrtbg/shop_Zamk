/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import fs from 'fs';
import path from 'path';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
  type ProductStudioVariant,
  type ProductStudioImage,
} from '../../contexts/ProductStudioContext';
import { ProductStudio } from './ProductStudio';

interface ContextRef {
  current: ReturnType<typeof useProductStudio> | null;
}

// Helper component to inspect context in tests
function ContextInspector({ contextRef }: { contextRef: ContextRef }) {
  const ctx = useProductStudio();
  contextRef.current = ctx;
  return (
    <div data-testid="context-inspector">
      <span data-testid="ctx-dirty">{ctx.isDirty ? 'dirty' : 'clean'}</span>
      <span data-testid="ctx-view-mode">{ctx.viewMode}</span>
      <span data-testid="ctx-section">{ctx.activeSection}</span>
      <button
        data-testid="ctx-reset-btn"
        onClick={() => ctx.resetDraft()}
      >
        Сброс
      </button>
    </div>
  );
}

describe('SHOP PS.R1 — Product Studio Internal Foundation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Requirement 1: default view is VISUAL
  it('1. default view is VISUAL', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create">
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Visual workspace is mounted by default
    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
    expect(screen.queryByTestId('studio-form-workspace')).toBeNull();

    // Toggle button reflects VISUAL active
    const visualBtn = screen.getByTestId('studio-view-toggle-visual');
    expect(visualBtn.getAttribute('aria-selected')).toBe('true');

    const formBtn = screen.getByTestId('studio-view-toggle-form');
    expect(formBtn.getAttribute('aria-selected')).toBe('false');
  });

  // Requirement 2: switching Visual -> Form -> Visual preserves the SAME draft state
  it('2. switching Visual -> Form -> Visual preserves the SAME draft state', () => {
    const initialDraft: Partial<ProductStudioDraft> = {
      title: 'Классический кардиган',
      description: 'Кардиган крупной вязки из 100% шерсти мериноса.',
      priceCents: 1200000, // 12 000 ₽
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={initialDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Verify initial values in Visual placeholder
    expect(screen.getByTestId('visual-draft-title').textContent).toBe('Классический кардиган');
    expect(screen.getByTestId('visual-draft-description').textContent).toBe('Кардиган крупной вязки из 100% шерсти мериноса.');
    expect(screen.getByTestId('visual-draft-price').textContent).toMatch(/12[\s\u00a0]000/);

    // Switch to Form mode
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    expect(screen.getByTestId('studio-form-workspace')).toBeTruthy();
    expect(screen.queryByTestId('studio-visual-workspace')).toBeNull();

    // Modify title and description in Form
    const titleInput = screen.getByTestId('form-product-title-input') as HTMLInputElement;
    const descInput = screen.getByTestId('form-product-description-input') as HTMLTextAreaElement;

    expect(titleInput.value).toBe('Классический кардиган');
    expect(descInput.value).toBe('Кардиган крупной вязки из 100% шерсти мериноса.');

    fireEvent.change(titleInput, { target: { value: 'Обновленный кардиган оверсайз' } });
    fireEvent.change(descInput, { target: { value: 'Новое детальное описание товара.' } });

    // Switch to Pricing section and update price
    fireEvent.click(screen.getByTestId('studio-section-btn-pricing'));
    const priceInput = screen.getByTestId('form-product-price-input') as HTMLInputElement;
    expect(priceInput.value).toBe('12000');

    fireEvent.change(priceInput, { target: { value: '15500' } });

    // Switch back to Visual mode
    fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));
    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
    expect(screen.queryByTestId('studio-form-workspace')).toBeNull();

    // Verify Visual displays updated draft values seamlessly
    expect(screen.getByTestId('visual-draft-title').textContent).toBe('Обновленный кардиган оверсайз');
    expect(screen.getByTestId('visual-draft-description').textContent).toBe('Новое детальное описание товара.');
    expect(screen.getByTestId('visual-draft-price').textContent).toMatch(/15[\s\u00a0]500/);

    // Switch back to Form mode and verify inputs retain changes
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    fireEvent.click(screen.getByTestId('studio-section-btn-basics'));
    expect((screen.getByTestId('form-product-title-input') as HTMLInputElement).value).toBe('Обновленный кардиган оверсайз');
    expect((screen.getByTestId('form-product-description-input') as HTMLTextAreaElement).value).toBe('Новое детальное описание товара.');
  });

  // Requirement 3: inline/form draft update marks draft dirty
  it('3. inline/form draft update marks draft dirty', () => {
    const contextRef: ContextRef = { current: null };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={{ title: 'Исходное название' }}>
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Initially clean
    expect(screen.getByTestId('ctx-dirty').textContent).toBe('clean');
    expect(contextRef.current?.isDirty).toBe(false);

    // Switch to Form and edit title
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    const titleInput = screen.getByTestId('form-product-title-input');
    fireEvent.change(titleInput, { target: { value: 'Модифицированное название' } });

    // Draft is now marked dirty
    expect(screen.getByTestId('ctx-dirty').textContent).toBe('dirty');
    expect(contextRef.current?.isDirty).toBe(true);

    // Resetting draft restores initial values and clean state
    fireEvent.click(screen.getByTestId('ctx-reset-btn'));
    expect(screen.getByTestId('ctx-dirty').textContent).toBe('clean');
    expect(contextRef.current?.isDirty).toBe(false);
  });

  // Requirement 4: Form uses horizontal sections: Основное, Медиа, Характеристики, Варианты, Цена, Проверка
  it('4. Form uses horizontal sections: Основное, Медиа, Характеристики, Варианты, Цена, Проверка', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create">
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));

    const nav = screen.getByTestId('product-studio-section-nav');
    expect(nav).toBeTruthy();

    // Verify horizontal flex layout classes (no vertical rail)
    expect(nav.className).toContain('flex');
    expect(nav.className).toContain('overflow-x-auto');

    // Verify all 6 exact section labels
    const expectedSections = [
      { id: 'basics', label: 'Основное' },
      { id: 'media', label: 'Медиа' },
      { id: 'characteristics', label: 'Характеристики' },
      { id: 'variants', label: 'Варианты' },
      { id: 'pricing', label: 'Цена' },
      { id: 'review', label: 'Проверка' },
    ];

    for (const section of expectedSections) {
      const btn = screen.getByTestId(`studio-section-btn-${section.id}`);
      expect(btn).toBeTruthy();
      expect(btn.textContent).toBe(section.label);
    }
  });

  // Requirement 5: clicking Form sections changes activeSection
  it('5. clicking Form sections changes activeSection', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create">
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));

    // Default section is basics
    expect(screen.getByTestId('studio-section-panel-basics')).toBeTruthy();

    // Media
    fireEvent.click(screen.getByTestId('studio-section-btn-media'));
    expect(screen.getByTestId('studio-section-panel-media')).toBeTruthy();
    expect(screen.queryByTestId('studio-section-panel-basics')).toBeNull();

    // Characteristics
    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));
    expect(screen.getByTestId('studio-section-panel-characteristics')).toBeTruthy();

    // Variants
    fireEvent.click(screen.getByTestId('studio-section-btn-variants'));
    expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();

    // Pricing
    fireEvent.click(screen.getByTestId('studio-section-btn-pricing'));
    expect(screen.getByTestId('studio-section-panel-pricing')).toBeTruthy();

    // Review
    fireEvent.click(screen.getByTestId('studio-section-btn-review'));
    expect(screen.getByTestId('studio-section-panel-review')).toBeTruthy();
    expect(screen.getByText(/Оценка полноты заполнения карточки/i)).toBeTruthy();
  });

  // Requirement 6: draft media supports optional colorId
  it('6. draft media supports optional colorId', () => {
    const contextRef: ContextRef = { current: null };

    const initialMedia: ProductStudioImage[] = [
      { id: 'm1', url: 'https://example.com/general-1.jpg', isMain: true, colorId: null },
      { id: 'm2', url: 'https://example.com/general-2.jpg', colorId: undefined },
      { id: 'm3', url: 'https://example.com/black-1.jpg', colorId: 'color-uuid-black' },
      { id: 'm4', url: 'https://example.com/white-1.jpg', colorId: 'color-uuid-white' },
    ];

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={{ images: initialMedia }}>
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(contextRef.current?.draft.images).toHaveLength(4);
    expect(contextRef.current?.draft.images?.[0].colorId).toBeNull();
    expect(contextRef.current?.draft.images?.[1].colorId).toBeUndefined();
    expect(contextRef.current?.draft.images?.[2].colorId).toBe('color-uuid-black');
    expect(contextRef.current?.draft.images?.[3].colorId).toBe('color-uuid-white');

    // Updating media with color-tagged item
    act(() => {
      contextRef.current?.updateDraft({
        images: [
          ...initialMedia,
          { id: 'm5', url: 'https://example.com/black-2.jpg', colorId: 'color-uuid-black' },
        ],
      });
    });

    expect(contextRef.current?.draft.images).toHaveLength(5);
    expect(contextRef.current?.draft.images?.[4].colorId).toBe('color-uuid-black');
  });

  // Requirement 7: media remains one ordered collection
  it('7. media remains one ordered collection', () => {
    const contextRef: ContextRef = { current: null };

    const orderedMedia: ProductStudioImage[] = [
      { id: 'm1', url: 'https://example.com/1.jpg', sortOrder: 1, colorId: null },
      { id: 'm2', url: 'https://example.com/2.jpg', sortOrder: 2, colorId: 'color-black' },
      { id: 'm3', url: 'https://example.com/3.jpg', sortOrder: 3, colorId: null },
      { id: 'm4', url: 'https://example.com/4.jpg', sortOrder: 4, colorId: 'color-white' },
    ];

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={{ images: orderedMedia }}>
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Ordered single array check
    const images = contextRef.current?.draft.images;
    expect(Array.isArray(images)).toBe(true);
    expect(images?.map((i: ProductStudioImage) => i.id)).toEqual(['m1', 'm2', 'm3', 'm4']);

    // Check no separate galleries per color exist in draft
    expect((contextRef.current?.draft as any).galleriesByColor).toBeUndefined();
    expect((contextRef.current?.draft as any).colorGalleries).toBeUndefined();
  });

  // Requirement 8: variant draft contains NO editable stock / InitialStock contract
  it('8. variant draft contains NO editable stock / InitialStock contract', () => {
    const contextRef: ContextRef = { current: null };

    const validVariants: ProductStudioVariant[] = [
      {
        id: 'v1',
        colorId: 'color-1',
        colorName: 'Черный',
        colorHex: '#000000',
        sizeValueId: 'size-s',
        size: 'S',
        sellerSku: 'SKU-BLK-S',
        barcode: '2000000000018',
        priceCents: 990000,
        isActive: true,
      },
      {
        id: 'v2',
        colorId: 'color-1',
        colorName: 'Черный',
        colorHex: '#000000',
        sizeValueId: 'size-m',
        size: 'M',
        sellerSku: 'SKU-BLK-M',
        barcode: '2000000000025',
        priceCents: 990000,
        isActive: true,
      },
    ];

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={{ variants: validVariants }}>
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const variants = contextRef.current?.draft.variants;
    expect(variants).toHaveLength(2);

    for (const v of variants || []) {
      // Forbidden FBO stock fields must NOT exist in variant draft
      expect((v as any).stock).toBeUndefined();
      expect((v as any).initialStock).toBeUndefined();
      expect((v as any).availableStock).toBeUndefined();
      expect((v as any).warehouseStock).toBeUndefined();
    }

    // Forbidden draft sizes[].stock must NOT exist
    expect((contextRef.current?.draft as any).sizes).toBeUndefined();

    // Verify UI in variants section has no stock inputs
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    fireEvent.click(screen.getByTestId('studio-section-btn-variants'));
    expect(screen.queryByLabelText(/остаток/i)).toBeNull();
    expect(screen.queryByLabelText(/склад/i)).toBeNull();
    expect(screen.queryByPlaceholderText(/остаток/i)).toBeNull();
  });

  // Requirement 9: brandId can be hydrated/preserved without brand selector UI
  it('9. brandId can be hydrated/preserved without brand selector UI', () => {
    const contextRef: ContextRef = { current: null };

    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={{
            id: 'p1',
            title: 'Шелковая блуза',
            brandId: 'brand-uuid-12345',
            brandName: 'Atelier Monochrome',
          }}
        >
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // brandId is preserved in draft context
    expect(contextRef.current?.draft.brandId).toBe('brand-uuid-12345');
    expect(contextRef.current?.draft.brandName).toBe('Atelier Monochrome');

    // Displayed in header subtitle
    expect(screen.getByTestId('studio-header-subtitle').textContent).toBe('Бренд: Atelier Monochrome');

    // No brand selector UI exists in Studio (single-brand seller rule)
    expect(screen.queryByTestId('brand-select')).toBeNull();
    expect(screen.queryByLabelText(/выберите бренд/i)).toBeNull();
    expect(screen.queryByRole('combobox', { name: /бренд/i })).toBeNull();
  });

  // Requirement 10: no autosave/persistence request occurs from local edits
  it('10. no autosave/persistence request occurs from local edits', () => {
    const fetchSpy = vi.fn();
    global.fetch = fetchSpy;

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create">
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Perform form edits
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    const titleInput = screen.getByTestId('form-product-title-input');
    fireEvent.change(titleInput, { target: { value: 'Тестовое название' } });

    fireEvent.click(screen.getByTestId('studio-section-btn-pricing'));
    const priceInput = screen.getByTestId('form-product-price-input');
    fireEvent.change(priceInput, { target: { value: '5000' } });

    // Toggle view mode
    fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));

    // No network/persistence request occurred
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  // Requirement 11: no Shop component is imported
  it('11. no Shop component is imported in Product Studio', () => {
    const studioDir = path.resolve(__dirname);
    const contextDir = path.resolve(__dirname, '../../contexts');

    const checkFiles = [
      path.join(contextDir, 'ProductStudioContext.tsx'),
      path.join(studioDir, 'ProductStudio.tsx'),
      path.join(studioDir, 'ProductStudioHeader.tsx'),
      path.join(studioDir, 'ProductStudioViewToggle.tsx'),
      path.join(studioDir, 'ProductStudioSectionNav.tsx'),
      path.join(studioDir, 'ProductStudioVisualPlaceholder.tsx'),
      path.join(studioDir, 'ProductStudioFormWorkspace.tsx'),
    ];

    for (const filePath of checkFiles) {
      expect(fs.existsSync(filePath)).toBe(true);
      const content = fs.readFileSync(filePath, 'utf-8');

      // Assert no import from apps/shop or shop components
      expect(content).not.toMatch(/from\s+['"][^'"]*apps\/shop/);
      expect(content).not.toMatch(/from\s+['"][^'"]*shop\//);
      expect(content).not.toMatch(/from\s+['"][^'"]*product-presentation/);
    }
  });

  // Requirement 12: no production route is changed
  it('12. no production route is changed in App.tsx', () => {
    const appPath = path.resolve(__dirname, '../../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    // Production routes must point to original components
    expect(appContent).toContain('path="/products" element={<SellerProtectedRoute><SellerLayout><SellerProducts /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/new" element={<SellerProtectedRoute><SellerLayout><SellerProductNew /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/:id/edit" element={<SellerProtectedRoute><SellerLayout><SellerProductEdit /></SellerLayout></SellerProtectedRoute>}');

    // ProductStudio must NOT be mounted in App.tsx routes yet
    expect(appContent).not.toContain('ProductStudio');
    expect(appContent).not.toContain('/products/studio');
  });
});

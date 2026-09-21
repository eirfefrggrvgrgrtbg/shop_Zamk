/** @vitest-environment jsdom */
vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockResolvedValue([{ id: 'col-black', name: 'Чёрный', hexValue: '#000000' }]),
    getSellerSizeValues: vi.fn().mockResolvedValue([]),
  };
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act, cleanup, within } from '@testing-library/react';
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


afterEach(() => {
  cleanup();
});

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SHOP PS.R1 — Product Studio Internal Foundation', () => {

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

    // Verify initial values in Visual workspace
    expect(within(screen.getByTestId('studio-visual-workspace')).getByRole('heading', { level: 1 }).textContent).toBe('Классический кардиган');
    expect(screen.getByText('Кардиган крупной вязки из 100% шерсти мериноса.')).toBeTruthy();
    expect(screen.getByText(/12[\s\u00a0]000/)).toBeTruthy();

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
    expect(within(screen.getByTestId('studio-visual-workspace')).getByRole('heading', { level: 1 }).textContent).toBe('Обновленный кардиган оверсайз');
    expect(screen.getByText('Новое детальное описание товара.')).toBeTruthy();
    expect(screen.getByText(/15[\s\u00a0]500/)).toBeTruthy();

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
      {
        uiKey: 'm1',
        isMain: true,
        colorId: null,
        source: { kind: 'canonical', imageId: 'm1', url: 'https://example.com/general-1.jpg' },
      },
      {
        uiKey: 'm2',
        isMain: false,
        colorId: undefined,
        source: { kind: 'canonical', imageId: 'm2', url: 'https://example.com/general-2.jpg' },
      },
      {
        uiKey: 'm3',
        isMain: false,
        colorId: 'color-uuid-black',
        source: { kind: 'canonical', imageId: 'm3', url: 'https://example.com/black-1.jpg' },
      },
      {
        uiKey: 'm4',
        isMain: false,
        colorId: 'color-uuid-white',
        source: { kind: 'canonical', imageId: 'm4', url: 'https://example.com/white-1.jpg' },
      },
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
          {
            uiKey: 'm5',
            isMain: false,
            colorId: 'color-uuid-black',
            source: { kind: 'canonical', imageId: 'm5', url: 'https://example.com/black-2.jpg' },
          },
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
      {
        uiKey: 'm1',
        sortOrder: 1,
        isMain: true,
        colorId: null,
        source: { kind: 'canonical', imageId: 'm1', url: 'https://example.com/1.jpg' },
      },
      {
        uiKey: 'm2',
        sortOrder: 2,
        isMain: false,
        colorId: 'color-black',
        source: { kind: 'canonical', imageId: 'm2', url: 'https://example.com/2.jpg' },
      },
      {
        uiKey: 'm3',
        sortOrder: 3,
        isMain: false,
        colorId: null,
        source: { kind: 'canonical', imageId: 'm3', url: 'https://example.com/3.jpg' },
      },
      {
        uiKey: 'm4',
        sortOrder: 4,
        isMain: false,
        colorId: 'color-white',
        source: { kind: 'canonical', imageId: 'm4', url: 'https://example.com/4.jpg' },
      },
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
    expect(images?.map((i: ProductStudioImage) => i.uiKey)).toEqual(['m1', 'm2', 'm3', 'm4']);

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
      path.join(studioDir, 'ProductStudioVisualWorkspace.tsx'),
      path.join(studioDir, 'productStudioPresentationAdapter.ts'),
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

  // Requirement 12: create and edit route points to Product Studio in App.tsx
  it('12. /products/new and /products/:id/edit route to Product Studio in App.tsx', () => {
    const appPath = path.resolve(__dirname, '../../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    expect(appContent).toContain('path="/products" element={<SellerProtectedRoute><SellerLayout><SellerProducts /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/new" element={<SellerProtectedRoute><SellerLayout><SellerProductStudioNew /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/:id/edit" element={<SellerProtectedRoute><SellerLayout><SellerProductStudioEdit /></SellerLayout></SellerProtectedRoute>}');
  });
});


describe('SHOP PS.R3A — Connect Shared Product Presentation to Seller Studio Visual Mode', () => {
  const sampleColorAndSizeDraft: Partial<ProductStudioDraft> = {
    title: 'Шелковая блуза',
    brandName: 'Acne Studios',
    brandId: 'b-acne',
    categoryName: 'Блузы',
    description: 'Премиальная блуза из натурального шелка.',
    priceCents: 1800000, // 18 000 ₽
    images: [
      {
        uiKey: 'main',
        isMain: true,
        sortOrder: 1,
        source: { kind: 'canonical', imageId: 'img-main', url: 'https://example.com/main.jpg' },
      },
      {
        uiKey: 'black',
        isMain: false,
        sortOrder: 2,
        colorId: 'col-black',
        source: { kind: 'canonical', imageId: 'img-black', url: 'https://example.com/black.jpg' },
      },
      {
        uiKey: 'white',
        isMain: false,
        sortOrder: 3,
        colorId: 'col-white',
        source: { kind: 'canonical', imageId: 'img-white', url: 'https://example.com/white.jpg' },
      },
    ],
    variants: [
      { id: 'v1', colorId: 'col-black', colorName: 'Черный', colorHex: '#000000', sizeValueId: 'sz-s', size: 'S', sellerSku: 'SKU-BLK-S' },
      { id: 'v2', colorId: 'col-black', colorName: 'Черный', colorHex: '#000000', sizeValueId: 'sz-m', size: 'M', sellerSku: 'SKU-BLK-M' },
      { id: 'v3', colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', sizeValueId: 'sz-s', size: 'S', sellerSku: 'SKU-WHT-S' },
    ],
  };

  // 1. Visual remains default view
  it('1. Visual remains default view', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
    expect(screen.getByTestId('studio-view-toggle-visual').getAttribute('aria-selected')).toBe('true');
  });

  // 2. Visual uses shared ProductPresentationCore
  it('2. Visual uses shared ProductPresentationCore', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByTestId('add-to-cart-button')).toBeTruthy();
    expect(screen.getByTestId('main-product-image')).toBeTruthy();
    expect(screen.getByRole('radiogroup', { name: 'Выбор цвета' })).toBeTruthy();
  });

  // 3. Draft title renders in actual presentation
  it('3. Draft title renders in actual presentation', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={{ title: 'Трендовый тренч оверсайз' }}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(within(screen.getByTestId('studio-visual-workspace')).getByRole('heading', { level: 1 }).textContent).toBe('Трендовый тренч оверсайз');
  });

  // 4. Draft brandName renders read-only
  it('4. Draft brandName renders read-only', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByText('Acne Studios')).toBeTruthy();
  });

  // 5. Draft price maps cents -> display correctly
  it('5. Draft price maps cents -> display correctly', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByText(/18[\s\u00a0]000/)).toBeTruthy();
  });

  // 6. ordered draft media renders in expected order
  it('6. ordered draft media renders in expected order', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const mainImg = screen.getByTestId('main-product-image');
    expect(mainImg.getAttribute('src')).toBe('https://example.com/main.jpg');
  });

  // 7. media.colorId mapping preserved
  it('7. media.colorId mapping preserved', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Initial canonical media
    expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/main.jpg');

    // Clicking black swatch focuses black media
    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/black.jpg');
  });

  // 8. clean Visual preview has no implicit selected color
  it('8. clean Visual preview has no implicit selected color', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.getByText('Цвет:').parentElement?.textContent).toContain('Не выбран');
    expect(screen.getByTestId('color-swatch-col-black').getAttribute('aria-checked')).toBe('false');
    expect(screen.getByTestId('color-swatch-col-white').getAttribute('aria-checked')).toBe('false');
  });

  // 9. explicit color click selects color locally
  it('9. explicit color click selects color locally', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    expect(screen.getByTestId('color-swatch-col-black').getAttribute('aria-checked')).toBe('true');
    expect(screen.getByText('Цвет:').parentElement?.textContent).toContain('Черный');
  });

  // 10. explicit color click focuses matching media
  it('10. explicit color click focuses matching media', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-white'));
    expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/white.jpg');
  });

  // 11. manual gallery browsing remains continuous
  it('11. manual gallery browsing remains continuous', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Click thumbnail 0
    fireEvent.click(screen.getByTestId('pdp-thumbnail-0'));
    expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/main.jpg');

    // Click thumbnail 1
    fireEvent.click(screen.getByTestId('pdp-thumbnail-1'));
    expect(screen.getByTestId('main-product-image').getAttribute('src')).toBe('https://example.com/black.jpg');
  });

  // 12. sizes disabled before color for COLOR_AND_SIZE
  it('12. sizes disabled before color for COLOR_AND_SIZE', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect((screen.getByTestId('size-button-S') as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByTestId('size-button-M') as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByTestId('size-selection-notice').textContent).toBe('Сначала выберите цвет');
  });

  // 13. offered size becomes AVAILABLE after color
  it('13. offered size becomes AVAILABLE after color', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    expect((screen.getByTestId('size-button-S') as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByTestId('size-button-S').getAttribute('data-state')).toBe('AVAILABLE');
    expect((screen.getByTestId('size-button-M') as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByTestId('size-button-M').getAttribute('data-state')).toBe('AVAILABLE');
  });

  // 14. absent combination renders NOT_OFFERED
  it('14. absent combination renders NOT_OFFERED', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // For white color, only S is offered; M is absent
    fireEvent.click(screen.getByTestId('color-swatch-col-white'));
    expect((screen.getByTestId('size-button-S') as HTMLButtonElement).disabled).toBe(false);
    expect((screen.getByTestId('size-button-M') as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByTestId('size-button-M').getAttribute('data-state')).toBe('NOT_OFFERED');
  });

  // 15. no SOLD_OUT is fabricated from Seller draft
  it('15. no SOLD_OUT is fabricated from Seller draft', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-white'));
    expect(screen.getByTestId('size-button-S').getAttribute('data-state')).not.toBe('SOLD_OUT');
    expect(screen.getByTestId('size-button-M').getAttribute('data-state')).not.toBe('SOLD_OUT');
  });

  // 16. size click updates local preview selection only
  it('16. size click updates local preview selection only', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    fireEvent.click(screen.getByTestId('size-button-S'));

    expect(screen.getByTestId('size-button-S').getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByText('Размер:').parentElement?.textContent).toContain('S');
    expect(screen.getByTestId('add-to-cart-button').textContent).toContain('Добавить в корзину');
  });

  // 17. Visual interactions do NOT mutate ProductStudioDraft merely by selecting preview color/size/gallery
  it('17. Visual interactions do NOT mutate ProductStudioDraft merely by selecting preview color/size/gallery', () => {
    const contextRef: ContextRef = { current: null };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
          <ContextInspector contextRef={contextRef} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Perform preview interactions
    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    fireEvent.click(screen.getByTestId('size-button-S'));
    fireEvent.click(screen.getByTestId('pdp-thumbnail-1'));

    // Draft remains clean and unmodified
    expect(contextRef.current?.isDirty).toBe(false);
  });

  // 18. Form -> Visual still shows latest edited draft values
  it('18. Form -> Visual still shows latest edited draft values', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Switch to Form and edit title
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    const titleInput = screen.getByTestId('form-product-title-input');
    fireEvent.change(titleInput, { target: { value: 'Шелковая блуза новая редакция' } });

    // Switch back to Visual
    fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));
    expect(within(screen.getByTestId('studio-visual-workspace')).getByRole('heading', { level: 1 }).textContent).toBe('Шелковая блуза новая редакция');
  });

  // 19. CTA causes NO cart/network side effect
  it('19. CTA causes NO cart/network side effect', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('color-swatch-col-black'));
    fireEvent.click(screen.getByTestId('size-button-S'));

    // Click CTA
    fireEvent.click(screen.getByTestId('add-to-cart-button'));

    // No network requests or commerce side effects
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  // 20. favorite/navigation preview actions cause NO customer side effects
  it('20. favorite/navigation preview actions cause NO customer side effects', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Click favorite button (should be disabled or harmless no-op)
    const favoriteBtns = screen.getAllByLabelText(/избранн/i);
    expect(favoriteBtns.length).toBeGreaterThan(0);
    fireEvent.click(favoriteBtns[0]);

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  // 21. no Shop imports exist in Seller Product Studio
  it('21. no Shop imports exist in Seller Product Studio', () => {
    const studioDir = path.resolve(__dirname);
    const files = fs.readdirSync(studioDir).filter((f) => f.endsWith('.tsx') || f.endsWith('.ts'));

    for (const file of files) {
      const content = fs.readFileSync(path.join(studioDir, file), 'utf-8');
      expect(content).not.toMatch(/from\s+['"][^'"]*apps\/shop/);
      expect(content).not.toMatch(/from\s+['"][^'"]*shop\//);
    }
  });

  // 22. production route cutover in App.tsx mounts Studio components
  it('22. production route cutover in App.tsx mounts SellerProductStudio components', () => {
    const appPath = path.resolve(__dirname, '../../App.tsx');
    const appContent = fs.readFileSync(appPath, 'utf-8');

    expect(appContent).toContain('path="/products" element={<SellerProtectedRoute><SellerLayout><SellerProducts /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/new" element={<SellerProtectedRoute><SellerLayout><SellerProductStudioNew /></SellerLayout></SellerProtectedRoute>}');
    expect(appContent).toContain('path="/products/:id/edit" element={<SellerProtectedRoute><SellerLayout><SellerProductStudioEdit /></SellerLayout></SellerProtectedRoute>}');
  });

  // 23. empty draft: Save disabled, Moderation disabled
  it('23. empty draft: Save disabled, Moderation disabled', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create">
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const saveBtn = screen.getByText('Сохранить').closest('button');
    const modBtn = screen.getByText('Отправить на модерацию').closest('button');

    expect(saveBtn?.disabled).toBe(true);
    expect(modBtn?.disabled).toBe(true);
  });

  // 24. fully populated draft: Save STILL disabled, Moderation STILL disabled
  it('24. fully populated draft: Save STILL disabled, Moderation STILL disabled', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={sampleColorAndSizeDraft}>
          <ProductStudio />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const saveBtn = screen.getByText('Сохранить').closest('button');
    const modBtn = screen.getByText('Отправить на модерацию').closest('button');

    expect(saveBtn?.disabled).toBe(true);
    expect(modBtn?.disabled).toBe(true);
  });
});

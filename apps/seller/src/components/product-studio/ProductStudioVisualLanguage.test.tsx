/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' },
      { id: 'col-black', nameRu: 'Чёрный', hexValue: '#000000' },
    ])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'cat-clothing', name: 'Одежда', type: 'FASHION' },
      { id: 'cat-shoes', name: 'Обувь', type: 'SHOES' },
    ])),
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => Promise.resolve({
      id: 'sch-clothing',
      categoryId: id,
      dimensionType: 'COLOR_AND_SIZE',
      name: 'Одежда',
      sizeChartRequired: true,
      allowedSizeSystems: [{ id: 'sys-eu', name: 'EU', isDefault: true }],
      attributes: [
        { id: 'attr-color', nameRu: 'Цвет', valueSource: 'VARIANT_COLOR', scope: 'VARIANT', required: true },
        { id: 'attr-size', nameRu: 'Размер', valueSource: 'VARIANT_SIZE', scope: 'VARIANT', required: true },
        { id: 'attr-season', code: 'SEASON', nameRu: 'Сезон', valueSource: 'DICTIONARY', scope: 'PRODUCT', required: true },
      ],
      sizeChartFields: [
        { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
      ],
    })),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'sz-s', sizeSystemId: 'sys-eu', nameRu: 'S', value: 'S', sortOrder: 1 },
      { id: 'sz-m', sizeSystemId: 'sys-eu', nameRu: 'M', value: 'M', sortOrder: 2 },
    ])),
    getSellerMaterials: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'mat-cotton', code: 'COTTON', nameRu: 'Хлопок' },
      { id: 'mat-poly', code: 'POLYESTER', nameRu: 'Полиэстер' },
    ])),
  };
});

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' },
      { id: 'col-black', nameRu: 'Чёрный', hexValue: '#000000' },
    ])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'cat-clothing', name: 'Одежда', type: 'FASHION' },
      { id: 'cat-shoes', name: 'Обувь', type: 'SHOES' },
    ])),
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => Promise.resolve({
      id: 'sch-clothing',
      categoryId: id,
      dimensionType: 'COLOR_AND_SIZE',
      name: 'Одежда',
      sizeChartRequired: true,
      allowedSizeSystems: [{ id: 'sys-eu', name: 'EU', isDefault: true }],
      attributes: [
        { id: 'attr-color', nameRu: 'Цвет', valueSource: 'VARIANT_COLOR', scope: 'VARIANT', required: true },
        { id: 'attr-size', nameRu: 'Размер', valueSource: 'VARIANT_SIZE', scope: 'VARIANT', required: true },
        { id: 'attr-season', code: 'SEASON', nameRu: 'Сезон', valueSource: 'DICTIONARY', scope: 'PRODUCT', required: true },
      ],
      sizeChartFields: [
        { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
      ],
    })),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'sz-s', sizeSystemId: 'sys-eu', nameRu: 'S', value: 'S', sortOrder: 1 },
      { id: 'sz-m', sizeSystemId: 'sys-eu', nameRu: 'M', value: 'M', sortOrder: 2 },
    ])),
    getSellerMaterials: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'mat-cotton', code: 'COTTON', nameRu: 'Хлопок' },
      { id: 'mat-poly', code: 'POLYESTER', nameRu: 'Полиэстер' },
    ])),
  };
});

if (!globalThis.URL.createObjectURL) {
  globalThis.URL.createObjectURL = vi.fn((file: any) => `blob:http://localhost/${file?.name || 'test'}`);
}
if (!globalThis.URL.revokeObjectURL) {
  globalThis.URL.revokeObjectURL = vi.fn();
}

import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioHeader } from './ProductStudioHeader';
import { ProductStudioProvider, useProductStudio, type ProductStudioDraft } from '../../contexts/ProductStudioContext';

function TestWrapper({
  initialDraft,
  children,
  contextCallback,
}: {
  initialDraft?: Partial<ProductStudioDraft>;
  children?: React.ReactNode;
  contextCallback?: (ctx: ReturnType<typeof useProductStudio>) => void;
}) {
  function Inspector() {
    const ctx = useProductStudio();
    if (contextCallback) contextCallback(ctx);
    return null;
  }
  return (
    <MemoryRouter>
      <ProductStudioProvider entryMode="create" initialDraft={initialDraft}>
        <Inspector />
        {children}
      </ProductStudioProvider>
    </MemoryRouter>
  );
}

describe('PS.R4A.6B — Validation & Required-State Visual Language', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    cleanup();
  });

  const cleanDraft: Partial<ProductStudioDraft> = {
    title: '',
    description: '',
    categoryId: '',
    priceCents: 0,
    images: [],
    materialComposition: [],
    material: '',
    attributes: [],
  };

  it('A. Clean draft initial render: shows required asterisks but calm neutral styling without red or amber', async () => {
    let capturedCtx: any = null;
    render(
      <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => { capturedCtx = ctx; }}>
        <ProductStudioHeader />
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    // Business readiness knows the draft is incomplete
    expect(capturedCtx.readiness.isReadyForModeration).toBe(false);
    expect(capturedCtx.readiness.blockingFields).toContain('title');
    expect(capturedCtx.readiness.blockingFields).toContain('price');
    expect(capturedCtx.readiness.blockingFields).toContain('category');
    expect(capturedCtx.readiness.blockingFields).toContain('media');
    expect(capturedCtx.readiness.blockingFields).toContain('description');
    expect(capturedCtx.readiness.blockingFields).toContain('composition');

    // UI state: touchedFields are empty, showReadinessAttention is false
    expect(capturedCtx.showReadinessAttention).toBe(false);
    expect(capturedCtx.isFieldAttention('title')).toBe(false);
    expect(capturedCtx.isFieldAttention('price')).toBe(false);
    expect(capturedCtx.isFieldAttention('media')).toBe(false);

    // Title: rendered with semantic asterisk, but neutral text/borders, NO red or amber
    const titleHeading = screen.getByRole('heading', { name: 'Название товара *' });
    expect(titleHeading.textContent).toBe('Название товара *');
    expect(titleHeading.className).not.toMatch(/border-red|text-red|bg-red/);
    expect(titleHeading.className).not.toMatch(/border-amber|text-amber/);
    expect(screen.queryByTestId('title-required-helper')).toBeNull();

    // Price: rendered with placeholder, neutral, NO red or amber
    const displayPrice = screen.getByTestId('visual-display-price');
    expect(displayPrice.textContent).toBe('Цена, ₽ *');
    expect(displayPrice.className).not.toMatch(/border-red|text-red|bg-red/);
    expect(displayPrice.className).not.toMatch(/border-amber|text-amber/);
    expect(screen.queryByTestId('price-required-helper')).toBeNull();

    // Category in Header: neutral, NO red or amber
    const categoryBtn = screen.getByTestId('studio-header-category-btn');
    expect(categoryBtn.className).not.toMatch(/border-red|text-red|bg-red/);
    expect(categoryBtn.className).not.toMatch(/border-amber|text-amber/);

    // Media slot: neutral border-ash/30, NO red or amber
    const emptyMedia = screen.getByTestId('presentation-empty-media-slot');
    expect(emptyMedia.className).toMatch(/border-ash\/30/);
    expect(emptyMedia.className).not.toMatch(/border-red|text-red/);
    expect(emptyMedia.className).not.toMatch(/border-amber|text-amber/);
    expect(screen.queryByTestId('media-required-helper')).toBeNull();

    // Composition: neutral border-border-soft, NO red or amber
    const compContainer = screen.getByTestId('composition-required-container');
    expect(compContainer.className).toMatch(/border-border-soft/);
    expect(compContainer.className).not.toMatch(/border-red|border-amber/);
    const compHelper = screen.getByTestId('composition-required-helper');
    expect(compHelper.className).toMatch(/text-ash/);
    expect(compHelper.className).not.toMatch(/text-red|text-amber/);
  });

  it('B. Touch title: blur without text transitions title to attention, other fields remain neutral', async () => {
    let capturedCtx: any = null;
    render(
      <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => { capturedCtx = ctx; }}>
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    // Click title heading to activate inline editing
    const titleHeading = screen.getByRole('heading', { level: 1 });
    fireEvent.click(titleHeading);

    // Find input and blur without typing
    const titleInput = screen.getByTestId('visual-inline-input');
    fireEvent.blur(titleInput);

    // Title attention now active
    expect(capturedCtx.touchedFields['title']).toBe(true);
    expect(capturedCtx.isFieldAttention('title')).toBe(true);

    const updatedHeading = screen.getByRole('heading', { level: 1 });
    expect(updatedHeading.className).toMatch(/border-amber-400|text-amber-700/);
    const titleHelper = screen.getByTestId('title-required-helper');
    expect(titleHelper).toBeDefined();
    expect(titleHelper.textContent).toBe('Укажите название');

    // Other fields remain neutral
    expect(capturedCtx.isFieldAttention('price')).toBe(false);
    expect(screen.queryByTestId('price-required-helper')).toBeNull();
    const displayPrice = screen.getByTestId('visual-display-price');
    expect(displayPrice.className).not.toMatch(/border-amber|text-amber/);
  });

  it('C. Invalid media upload: displays local media error in RED, other fields remain neutral', async () => {
    render(
      <TestWrapper initialDraft={cleanDraft}>
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    const fileInput = screen.getByTestId('empty-stage-photo-input') as HTMLInputElement;

    // Upload an unsupported format (e.g. text/plain or invalid extension)
    const invalidFile = new File(['dummy content'], 'document.pdf', { type: 'application/pdf' });
    fireEvent.change(fileInput, { target: { files: [invalidFile] } });

    // Error is shown in RED
    const errorEl = await screen.findByTestId('media-upload-error');
    expect(errorEl).toBeDefined();
    expect(errorEl.className).toMatch(/text-red-500/);
    expect(errorEl.textContent).toMatch(/Поддерживаются JPG, PNG и WebP/);

    // Title and price remain untouched and neutral
    const titleHeading = screen.getByRole('heading', { level: 1 });
    expect(titleHeading.className).not.toMatch(/border-red|text-red|border-amber|text-amber/);
    expect(screen.queryByTestId('title-required-helper')).toBeNull();
  });

  it('D. Category schema: Color and Size gain required markers, but remain neutral until interaction', async () => {
    render(
      <TestWrapper
        initialDraft={{
          ...cleanDraft,
          categoryId: 'cat-clothing',
          categoryName: 'Одежда',
          dimensionType: 'COLOR_AND_SIZE',
        }}
      >
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    await waitFor(() => {
      expect(screen.getByTestId('color-manage-btn')).toBeDefined();
    });

    // Color and size slots are present
    const colorBtn = screen.getByTestId('color-manage-btn');
    const sizeBtn = screen.getByTestId('size-manage-btn');

    // They are neutral (dashed border-ash), NOT amber or red
    expect(colorBtn.className).toMatch(/border-ash/);
    expect(colorBtn.className).not.toMatch(/border-amber|border-red/);
    expect(sizeBtn.className).toMatch(/border-ash/);
    expect(sizeBtn.className).not.toMatch(/border-amber|border-red/);

    // Helpers are not shown
    expect(screen.queryByTestId('color-required-helper')).toBeNull();
    expect(screen.queryByTestId('size-required-helper')).toBeNull();
  });

  it('E. Touch color manager: applying empty selection transitions color to attention, size remains neutral', async () => {
    render(
      <TestWrapper
        initialDraft={{
          ...cleanDraft,
          categoryId: 'cat-clothing',
          categoryName: 'Одежда',
          dimensionType: 'COLOR_AND_SIZE',
        }}
      >
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    await waitFor(() => {
      expect(screen.getByTestId('color-manage-btn')).toBeDefined();
    });

    // Open color popover
    fireEvent.click(screen.getByTestId('color-manage-btn'));
    expect(screen.getByTestId('color-popover')).toBeDefined();

    // Click "Готово" with 0 colors selected
    fireEvent.click(screen.getByTestId('color-popover-apply-btn'));

    // Color button is now in attention state
    const colorBtn = screen.getByTestId('color-manage-btn');
    expect(colorBtn.className).toMatch(/border-amber-400|text-amber-700/);
    const colorHelper = screen.getByTestId('color-required-helper');
    expect(colorHelper).toBeDefined();
    expect(colorHelper.textContent).toBe('Выберите цвет');

    // Size button remains untouched & neutral
    const sizeBtn = screen.getByTestId('size-manage-btn');
    expect(sizeBtn.className).toMatch(/border-ash/);
    expect(sizeBtn.className).not.toMatch(/border-amber|border-red/);
    expect(screen.queryByTestId('size-required-helper')).toBeNull();
  });

  it('F. Complete field clears attention state immediately', async () => {
    let capturedCtx: any = null;
    render(
      <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => { capturedCtx = ctx; }}>
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    // Focus and blur empty title to trigger attention
    fireEvent.click(screen.getByRole('heading', { level: 1 }));
    const titleInput = screen.getByTestId('visual-inline-input');
    fireEvent.blur(titleInput);
    expect(capturedCtx.isFieldAttention('title')).toBe(true);
    expect(screen.getByTestId('title-required-helper')).toBeDefined();

    // Now edit and enter valid title
    fireEvent.click(screen.getByRole('heading', { level: 1 }));
    const activeInput = screen.getByTestId('visual-inline-input');
    fireEvent.change(activeInput, { target: { value: 'Шёлковая рубашка' } });
    fireEvent.blur(activeInput);

    // Attention clears because blocking field is satisfied
    expect(capturedCtx.isFieldAttention('title')).toBe(false);
    expect(screen.queryByTestId('title-required-helper')).toBeNull();
    const updatedHeading = screen.getByRole('heading', { level: 1 });
    expect(updatedHeading.textContent).toBe('Шёлковая рубашка');
    expect(updatedHeading.className).not.toMatch(/border-amber|text-amber/);
  });

  it('G. Readiness summary button shows canonical blocking count and toggles attention across all blockers', async () => {
    let capturedCtx: any = null;
    render(
      <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => { capturedCtx = ctx; }}>
        <ProductStudioHeader />
        <ProductStudioVisualWorkspace />
      </TestWrapper>
    );

    const summaryBtn = screen.getByTestId('studio-readiness-summary');
    const expectedCount = capturedCtx.readiness.blockingFields.length;
    expect(summaryBtn.textContent).toBe(`Нужно заполнить: ${expectedCount}`);

    // Click summary button to reveal attention across all remaining blockers
    fireEvent.click(summaryBtn);
    expect(capturedCtx.showReadinessAttention).toBe(true);

    // Now all blockers show attention
    expect(screen.getByTestId('title-required-helper')).toBeDefined();
    expect(screen.getByTestId('price-required-helper')).toBeDefined();
    expect(screen.getByTestId('media-required-helper')).toBeDefined();
    expect(screen.getByTestId('composition-required-container').className).toMatch(/border-amber-500/);

    // Optional care instructions are NEVER counted as a blocker
    expect(capturedCtx.readiness.blockingFields).not.toContain('care');
  });
});

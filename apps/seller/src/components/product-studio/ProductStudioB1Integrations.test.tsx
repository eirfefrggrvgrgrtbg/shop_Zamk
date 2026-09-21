/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'col-black', nameRu: 'Черный', hexValue: '#000000' },
      { id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' },
    ])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'cat-hoodie', name: 'Худи', type: 'FASHION' },
      { id: 'cat-bag', name: 'Сумки', type: 'ACCESSORIES' },
    ])),
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => {
      if (id === 'cat-bag') {
        return Promise.resolve({
          id: 'sch-bag',
          categoryId: 'cat-bag',
          name: 'Сумки',
          dimensionType: 'COLOR_ONLY',
          sizeChartRequired: false,
          allowedSizeSystems: [],
          attributes: [
            {
              id: 'attr-brand-style',
              nameRu: 'Стиль',
              valueType: 'TEXT',
              valueSource: 'CUSTOM',
              scope: 'PRODUCT',
              required: false,
            },
          ],
        });
      }
      return Promise.resolve({
        id: 'sch-hoodie',
        categoryId: 'cat-hoodie',
        name: 'Худи',
        dimensionType: 'COLOR_AND_SIZE',
        sizeChartRequired: true,
        allowedSizeSystems: [{ id: 'sys-ru', name: 'RU', isDefault: true }],
        attributes: [
          {
            id: 'attr-color',
            nameRu: 'Цвет',
            valueSource: 'VARIANT_COLOR',
            scope: 'VARIANT',
            required: true,
          },
          {
            id: 'attr-size',
            nameRu: 'Размер',
            valueSource: 'VARIANT_SIZE',
            scope: 'VARIANT',
            required: true,
          },
          {
            id: 'attr-season',
            code: 'SEASON',
            nameRu: 'Сезон',
            valueType: 'DICTIONARY',
            valueSource: 'DICTIONARY',
            dictionaryId: 'dict-season',
            scope: 'PRODUCT',
            required: true,
          },
          {
            id: 'attr-fit',
            nameRu: 'Посадка',
            valueType: 'TEXT',
            valueSource: 'CUSTOM',
            scope: 'PRODUCT',
            required: false,
          },
        ],
        sizeChartFields: [
          { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
        ],
      });
    }),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'sz-m', sizeSystemId: 'sys-ru', nameRu: 'M', value: 'M', sortOrder: 1 },
      { id: 'sz-l', sizeSystemId: 'sys-ru', nameRu: 'L', value: 'L', sortOrder: 2 },
    ])),
    getSellerMaterials: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'mat-cotton', code: 'COTTON', nameRu: 'Хлопок' },
      { id: 'mat-poly', code: 'POLY', nameRu: 'Полиэстер' },
    ])),
    getSellerDictionaryValues: vi.fn().mockImplementation((dictId: string) => {
      if (dictId === 'dict-season') {
        return Promise.resolve([
          { id: 'val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
          { id: 'val-summer', dictionaryId: 'dict-season', code: 'SUMMER', nameRu: 'Лето' },
        ]);
      }
      return Promise.resolve([]);
    }),
  };
});

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'col-black', nameRu: 'Черный', hexValue: '#000000' },
      { id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' },
    ])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'cat-hoodie', name: 'Худи', type: 'FASHION' },
      { id: 'cat-bag', name: 'Сумки', type: 'ACCESSORIES' },
    ])),
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => {
      if (id === 'cat-bag') {
        return Promise.resolve({
          id: 'sch-bag',
          categoryId: 'cat-bag',
          name: 'Сумки',
          dimensionType: 'COLOR_ONLY',
          sizeChartRequired: false,
          allowedSizeSystems: [],
          attributes: [
            {
              id: 'attr-brand-style',
              nameRu: 'Стиль',
              valueType: 'TEXT',
              valueSource: 'CUSTOM',
              scope: 'PRODUCT',
              required: false,
            },
          ],
        });
      }
      return Promise.resolve({
        id: 'sch-hoodie',
        categoryId: 'cat-hoodie',
        name: 'Худи',
        dimensionType: 'COLOR_AND_SIZE',
        sizeChartRequired: true,
        allowedSizeSystems: [{ id: 'sys-ru', name: 'RU', isDefault: true }],
        attributes: [
          {
            id: 'attr-color',
            nameRu: 'Цвет',
            valueSource: 'VARIANT_COLOR',
            scope: 'VARIANT',
            required: true,
          },
          {
            id: 'attr-size',
            nameRu: 'Размер',
            valueSource: 'VARIANT_SIZE',
            scope: 'VARIANT',
            required: true,
          },
          {
            id: 'attr-season',
            code: 'SEASON',
            nameRu: 'Сезон',
            valueType: 'DICTIONARY',
            valueSource: 'DICTIONARY',
            dictionaryId: 'dict-season',
            scope: 'PRODUCT',
            required: true,
          },
          {
            id: 'attr-fit',
            nameRu: 'Посадка',
            valueType: 'TEXT',
            valueSource: 'CUSTOM',
            scope: 'PRODUCT',
            required: false,
          },
        ],
        sizeChartFields: [
          { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
        ],
      });
    }),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'sz-m', sizeSystemId: 'sys-ru', nameRu: 'M', value: 'M', sortOrder: 1 },
      { id: 'sz-l', sizeSystemId: 'sys-ru', nameRu: 'L', value: 'L', sortOrder: 2 },
    ])),
    getSellerMaterials: vi.fn().mockImplementation(() => Promise.resolve([
      { id: 'mat-cotton', code: 'COTTON', nameRu: 'Хлопок' },
      { id: 'mat-poly', code: 'POLY', nameRu: 'Полиэстер' },
    ])),
    getSellerDictionaryValues: vi.fn().mockImplementation((dictId: string) => {
      if (dictId === 'dict-season') {
        return Promise.resolve([
          { id: 'val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
          { id: 'val-summer', dictionaryId: 'dict-season', code: 'SUMMER', nameRu: 'Лето' },
        ]);
      }
      return Promise.resolve([]);
    }),
  };
});

if (!globalThis.URL.createObjectURL) {
  globalThis.URL.createObjectURL = vi.fn((file: any) => `blob:http://localhost/${file?.name || 'test'}`);
}
if (!globalThis.URL.revokeObjectURL) {
  globalThis.URL.revokeObjectURL = vi.fn();
}

import { render, screen, fireEvent, cleanup, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import { ProductStudioProvider, useProductStudio, type ProductStudioDraft } from '../../contexts/ProductStudioContext';

function TestWrapper({
  initialDraft,
  children,
  contextCallback,
}: {
  initialDraft: Partial<ProductStudioDraft>;
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

describe('PS.R4A.5B1 — Required Product Content + Canonical Characteristics UX', () => {
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

  const filledDraft: Partial<ProductStudioDraft> = {
    title: 'Худи тёплое',
    description: 'Плотный футер 3-х нитка с начесом',
    categoryId: 'cat-hoodie',
    categoryName: 'Худи',
    priceCents: 490000,
    images: [
      {
        uiKey: '1',
        isMain: true,
        source: { kind: 'canonical', imageId: '1', url: 'https://images/1.jpg' },
      },
      {
        uiKey: '2',
        isMain: false,
        source: { kind: 'canonical', imageId: '2', url: 'https://images/2.jpg' },
      },
      {
        uiKey: '3',
        isMain: false,
        source: { kind: 'canonical', imageId: '3', url: 'https://images/3.jpg' },
      },
    ],
    colors: [{ id: 'col-black', name: 'Черный' }],
    variants: [{ id: 'v-1', sizeValueId: 'sz-m', size: 'M', colorId: 'col-black', priceCents: 490000 }],
    materialComposition: [{ materialName: 'Хлопок', percentage: 100 }],
    material: 'Хлопок 100%',
    careInstructions: 'Стирка при 30 градусах',
    attributes: [
      { attributeDefinitionId: 'attr-season', name: 'Сезон', value: 'Зима', dictionaryValueId: 'val-winter' },
    ],
    sizeChart: {
      fields: [{ key: 'chest', label: 'Обхват груди', unit: 'см' }],
      rows: [{ size: 'M', measurements: { chest: 104 } }],
    },
  };

  describe('1. Description is REQUIRED', () => {
    it('clean draft in Visual surfaces red required state and helper for description', () => {
      render(
        <TestWrapper initialDraft={cleanDraft}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      expect(screen.getByText('Добавить описание *')).toBeTruthy();
      expect(screen.getByTestId('description-required-helper')).toBeTruthy();
      expect(screen.getByText('Добавьте описание товара')).toBeTruthy();
    });

    it('typing description in Visual in-place editor updates draft and clears required helper', async () => {
      let currentCtx: any;
      render(
        <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => (currentCtx = ctx)}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      const descTrigger = screen.getByTestId('description-container');
      fireEvent.click(descTrigger);

      const textarea = screen.getByTestId('visual-inline-textarea') as HTMLTextAreaElement;
      expect(textarea.placeholder).toBe('Расскажите о посадке, особенностях модели и важных деталях товара');

      fireEvent.change(textarea, { target: { value: 'Новое красивое описание' } });
      fireEvent.blur(textarea);

      await waitFor(() => {
        expect(currentCtx.draft.description).toBe('Новое красивое описание');
        expect(screen.queryByTestId('description-required-helper')).toBeNull();
        expect(screen.getByText('Новое красивое описание')).toBeTruthy();
      });
    });

    it('Form workspace renders Description * with canonical helper and reflects draft', () => {
      render(
        <TestWrapper initialDraft={cleanDraft}>
          <ProductStudioFormWorkspace />
        </TestWrapper>
      );
      expect(screen.getByText(/Описание/i)).toBeTruthy();
      expect(screen.getByTestId('form-description-helper')).toBeTruthy();
      expect(screen.getByText('Опишите особенности товара, посадку и важные детали')).toBeTruthy();
    });
  });

  describe('2. Composition is REQUIRED', () => {
    it('clean draft in Visual surfaces red required state and helper for composition', () => {
      render(
        <TestWrapper initialDraft={cleanDraft}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      expect(screen.getByText('Состав *')).toBeTruthy();
      expect(screen.getByTestId('composition-required-helper')).toBeTruthy();
      expect(screen.getByText('Укажите состав товара')).toBeTruthy();
      expect(screen.getByTestId('add-composition-btn')).toBeTruthy();
    });

    it('clicking Add Composition opens Composition Modal, saving rows updates draft atomically', async () => {
      let currentCtx: any;
      render(
        <TestWrapper initialDraft={cleanDraft} contextCallback={(ctx) => (currentCtx = ctx)}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      const addCompBtn = screen.getByTestId('add-composition-btn');
      fireEvent.click(addCompBtn);

      await waitFor(() => {
        expect(screen.getByTestId('composition-modal')).toBeTruthy();
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      const trigger = screen.getByTestId('composition-material-trigger-0');
      fireEvent.click(trigger);
      const option = screen.getByTestId('material-option-mat-cotton');
      fireEvent.click(option);

      const pctInput = screen.getByTestId('composition-percentage-input-0');
      fireEvent.change(pctInput, { target: { value: '100' } });

      // Apply
      const applyBtn = screen.getByTestId('composition-modal-apply');
      fireEvent.click(applyBtn);

      await waitFor(() => {
        expect(screen.queryByTestId('composition-modal')).toBeNull();
        expect(currentCtx.draft.materialComposition).toHaveLength(1);
        expect(currentCtx.draft.materialComposition[0].materialName).toBe('Хлопок');
        expect(screen.queryByTestId('composition-required-helper')).toBeNull();
      });
    });

    it('filled composition renders Shop specs in Visual and Form', () => {
      render(
        <TestWrapper initialDraft={filledDraft}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      expect(screen.getAllByText(/Хлопок 100%/i).length).toBeGreaterThan(0);
      expect(screen.getByTestId('edit-composition-btn')).toBeTruthy();
    });
  });

  describe('3. Care is SEPARATE and OPTIONAL', () => {
    it('Care section renders optional affordance in Visual and is not marked red', () => {
      render(
        <TestWrapper initialDraft={cleanDraft}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      expect(screen.getByText('Уход')).toBeTruthy();
      const addCareBtn = screen.getByTestId('add-care-btn');
      expect(addCareBtn).toBeTruthy();
      expect(addCareBtn.textContent).toContain('Добавить уход');
      // Must not be red
      expect(addCareBtn.className).not.toContain('text-red');
    });

    it('displays saved care instructions when present', () => {
      render(
        <TestWrapper initialDraft={filledDraft}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      expect(screen.getByText(/Стирка при 30 градусах/i)).toBeTruthy();
      expect(screen.getByTestId('edit-care-btn')).toBeTruthy();
    });
  });

  describe('4. Canonical Category-Driven Characteristics UX', () => {
    it('characteristics are driven by schema attributes and do not show arbitrary + Добавить характеристику', async () => {
      render(
        <TestWrapper initialDraft={{ ...cleanDraft, categoryId: 'cat-hoodie', categoryName: 'Худи' }}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      const accordion = screen.getByText('Характеристики');
      fireEvent.click(accordion);

      // Wait for schema loading
      await waitFor(() => {
        expect(screen.getByTestId('manage-characteristics-btn')).toBeTruthy();
      });

      expect(screen.queryByText('+ Добавить характеристику')).toBeNull();
      // Required attribute from schema is 'attr-season'
      expect(screen.getByTestId('characteristics-progress-badge')).toBeTruthy();
      expect(screen.getByText(/Характеристики \* · заполнено 0 из 1/i)).toBeTruthy();
      expect(screen.getByTestId('characteristics-required-helper')).toBeTruthy();
    });

    it('modal opens canonical controls and atomic save updates draft.attributes and accordion', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{ ...cleanDraft, categoryId: 'cat-hoodie', categoryName: 'Худи' }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        >
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      const accordion = screen.getByText('Характеристики');
      fireEvent.click(accordion);

      await waitFor(() => {
        expect(screen.getByTestId('manage-characteristics-btn')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('manage-characteristics-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('characteristics-modal')).toBeTruthy();
        expect(screen.getByTestId('characteristic-select-SEASON')).toBeTruthy();
      });

      // Select Winter
      const select = screen.getByTestId('characteristic-select-SEASON');
      fireEvent.change(select, { target: { value: 'val-winter' } });

      // Apply
      fireEvent.click(screen.getByTestId('characteristics-modal-apply'));

      await waitFor(() => {
        expect(screen.queryByTestId('characteristics-modal')).toBeNull();
        expect(currentCtx.draft.attributes).toHaveLength(1);
        expect(currentCtx.draft.attributes[0].value).toBe('Зима');
        expect(screen.getByText(/Характеристики · заполнено 1 из 1/i)).toBeTruthy();
      });
    });
  });

  describe('5. Size Chart is CONDITIONALLY REQUIRED', () => {
    it('category without sizes (COLOR_ONLY) does not require size chart', async () => {
      render(
        <TestWrapper initialDraft={{ ...cleanDraft, categoryId: 'cat-bag', categoryName: 'Сумки' }}>
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );
      await waitFor(() => {
        expect(screen.queryByTestId('size-chart-required-btn')).toBeNull();
      });
    });

    it('category with sizes when offered sizes exist shows Таблица размеров *', async () => {
      render(
        <TestWrapper
          initialDraft={{
            ...cleanDraft,
            categoryId: 'cat-hoodie',
            categoryName: 'Худи',
            variants: [{ id: 'v1', sizeValueId: 'sz-m', size: 'M', priceCents: 490000 }],
          }}
        >
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      await waitFor(() => {
        const reqBtn = screen.getByTestId('size-chart-required-btn');
        expect(reqBtn).toBeTruthy();
        expect(reqBtn.textContent).toContain('Таблица размеров *');
      });
    });

    it('clicking size chart required button opens real measurement editor, entering values and saving updates draft', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{
            ...cleanDraft,
            categoryId: 'cat-hoodie',
            categoryName: 'Худи',
            variants: [{ id: 'v1', sizeValueId: 'sz-m', size: 'M', priceCents: 490000 }],
          }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        >
          <ProductStudioVisualWorkspace />
        </TestWrapper>
      );

      await waitFor(() => {
        expect(screen.getByTestId('size-chart-required-btn')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('size-chart-required-btn'));

      await waitFor(() => {
        const modal = screen.getByTestId('size-chart-modal');
        expect(modal).toBeTruthy();
        expect(within(modal).getByRole('heading', { name: /Таблица размеров и мерки/i })).toBeTruthy();
        expect(screen.getByTestId('measurement-input-sz-m-chest')).toBeTruthy();
      });

      // Type measurement into required cell
      const chestInput = screen.getByTestId('measurement-input-sz-m-chest');
      fireEvent.change(chestInput, { target: { value: '104' } });

      const saveBtn = screen.getByTestId('size-chart-modal-save');
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(screen.queryByTestId('size-chart-modal')).toBeNull();
        expect(currentCtx.draft.sizeChart.rows.length).toBe(1);
        expect(currentCtx.draft.sizeChart.rows[0].measurements.chest).toBe(104);
        expect(screen.queryByTestId('size-chart-required-btn')).toBeNull();
        expect(screen.getByTestId('size-chart-btn')).toBeTruthy();
        expect(screen.getByTestId('size-chart-btn').textContent).toContain('Таблица размеров · заполнено 1 из 1');
      });
    });
  });
});

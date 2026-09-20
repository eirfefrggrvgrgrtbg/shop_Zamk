/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' }, { id: 'col-black', nameRu: 'Чёрный', hexValue: '#000000' }])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'cat-clothing', name: 'Одежда', type: 'FASHION' }, { id: 'cat-shoes', name: 'Обувь', type: 'SHOES' }])),
    getSellerCategorySchema: vi.fn().mockImplementation(() => Promise.resolve({ id: 'sch-clothing', categoryId: 'cat-clothing', dimensionType: 'COLOR_AND_SIZE', name: 'Одежда', allowedSizeSystems: [{ id: 'sys-eu', name: 'EU' }], attributes: [{ id: 'attr-color', nameRu: 'Цвет', valueSource: 'VARIANT_COLOR' }, { id: 'attr-size', nameRu: 'Размер', valueSource: 'VARIANT_SIZE' }, { id: 'attr-season', nameRu: 'Сезон', valueSource: 'DICTIONARY' }] })),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'sz-s', sizeSystemId: 'sys-eu', nameRu: 'S', value: 'S', sortOrder: 1 }, { id: 'sz-m', sizeSystemId: 'sys-eu', nameRu: 'M', value: 'M', sortOrder: 2 }, { id: 'sz-l', sizeSystemId: 'sys-eu', nameRu: 'L', value: 'L', sortOrder: 3 }])),
  };
});

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'col-white', nameRu: 'Белый', hexValue: '#ffffff' }, { id: 'col-black', nameRu: 'Чёрный', hexValue: '#000000' }])),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'cat-clothing', name: 'Одежда', type: 'FASHION' }, { id: 'cat-shoes', name: 'Обувь', type: 'SHOES' }])),
    getSellerCategorySchema: vi.fn().mockImplementation(() => Promise.resolve({ id: 'sch-clothing', categoryId: 'cat-clothing', dimensionType: 'COLOR_AND_SIZE', name: 'Одежда', allowedSizeSystems: [{ id: 'sys-eu', name: 'EU' }], attributes: [{ id: 'attr-color', nameRu: 'Цвет', valueSource: 'VARIANT_COLOR' }, { id: 'attr-size', nameRu: 'Размер', valueSource: 'VARIANT_SIZE' }, { id: 'attr-season', nameRu: 'Сезон', valueSource: 'DICTIONARY' }] })),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([{ id: 'sz-s', sizeSystemId: 'sys-eu', nameRu: 'S', value: 'S', sortOrder: 1 }, { id: 'sz-m', sizeSystemId: 'sys-eu', nameRu: 'M', value: 'M', sortOrder: 2 }, { id: 'sz-l', sizeSystemId: 'sys-eu', nameRu: 'L', value: 'L', sortOrder: 3 }])),
  };
});

// Setup jsdom URL blob mocks
if (!globalThis.URL.createObjectURL) {
  globalThis.URL.createObjectURL = vi.fn((file: any) => `blob:http://localhost/${file?.name || 'test'}`);
} else {
  vi.spyOn(globalThis.URL, 'createObjectURL').mockImplementation((file: any) => `blob:http://localhost/${file?.name || 'test'}`);
}

if (!globalThis.URL.revokeObjectURL) {
  globalThis.URL.revokeObjectURL = vi.fn();
} else {
  vi.spyOn(globalThis.URL, 'revokeObjectURL').mockImplementation(() => {});
}

import { render, screen, fireEvent, cleanup, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';

const baseDraft: Partial<ProductStudioDraft> = {
  id: 'd-1',
  title: 'Классическая футболка',
  brandName: 'ZAMK Studio',
  description: 'Премиальный хлопок.',
  material: '100% органический хлопок',
  priceCents: 450000,
  oldPriceCents: 600000,
  categoryId: 'cat-clothing',
  categoryName: 'Одежда',
  colors: [
    { id: 'col-black', name: 'Чёрный', hex: '#000000' },
  ],
  variants: [
    { id: 'var-1', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', priceCents: 450000 },
  ],
  images: [
    { id: 'img-1', url: 'https://images/1.jpg', colorId: 'col-black', sortOrder: 0 },
  ],
  materialComposition: [
    { materialName: 'Хлопок', percentage: 100 },
  ],
  attributes: [
    { attributeDefinitionId: 'attr-1', name: 'Сезон', value: 'Лето' },
  ],
};

function TestWrapper({
  initialDraft = baseDraft,
  contextCallback,
  children,
}: {
  initialDraft?: Partial<ProductStudioDraft>;
  contextCallback?: (ctx: ReturnType<typeof useProductStudio>) => void;
  children?: React.ReactNode;
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
        {children || <ProductStudioVisualWorkspace />}
      </ProductStudioProvider>
    </MemoryRouter>
  );
}

describe('ProductStudioVisualWorkspace - In-Canvas Constructor', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe('A. Initial Render and Data Display', () => {
    it('1. Renders title correctly', () => {
      render(<TestWrapper />);
      expect(screen.getByText('Классическая футболка')).toBeTruthy();
    });

    it('2. Renders brand name', () => {
      render(<TestWrapper />);
      expect(screen.getByText('ZAMK Studio')).toBeTruthy();
    });

    it('3. Renders price', () => {
      render(<TestWrapper />);
      expect(screen.getAllByText(/4\s*500/)[0]).toBeTruthy();
    });

    it('4. Renders old price', () => {
      render(<TestWrapper />);
      expect(screen.getAllByText(/6\s*000/)[0]).toBeTruthy();
    });

    it('5. Renders description', () => {
      render(<TestWrapper />);
      expect(screen.getByText('Премиальный хлопок.')).toBeTruthy();
    });

    it('6. Renders material in composition', () => {
      render(<TestWrapper />);
      expect(screen.getByText('Хлопок 100%')).toBeTruthy();
    });

    it('7. Renders empty description placeholder if description empty', () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, description: '' }} />);
      expect(screen.getAllByText(/Добавить описание/i).length).toBeGreaterThan(0);
    });

    it('8. Renders color swatch', () => {
      render(<TestWrapper />);
      expect(screen.getByTestId('color-swatch-col-black')).toBeTruthy();
    });

    it('9. Renders size button', () => {
      render(<TestWrapper />);
      expect(screen.getByTestId('size-button-M')).toBeTruthy();
    });

    it('10. Renders characteristics accordion title', () => {
      render(<TestWrapper />);
      expect(screen.getAllByText('Характеристики').length).toBeGreaterThan(0);
    });
  });

  describe('B. Direct-Add Affordances and Controls in Real PDP', () => {
    it('11. Photo uses native file picker, no URL textbox, creates local blob preview', async () => {
      let currentCtx: any;
      const fetchSpy = vi.spyOn(globalThis, 'fetch');

      render(
        <TestWrapper
          initialDraft={{ ...baseDraft, images: [] }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      const photoLabel = screen.getByText(/Добавить фото/i).closest('label');
      expect(photoLabel).toBeTruthy();

      const fileInput = photoLabel!.querySelector('input[type="file"]') as HTMLInputElement;
      expect(fileInput).toBeTruthy();
      expect(screen.queryByPlaceholderText(/URL/i)).toBeNull();

      // Fire change with actual File
      const file = new File(['test-image'], 'preview.png', { type: 'image/png' });
      fireEvent.change(fileInput, { target: { files: [file] } });

      // Draft gains local media entry with blob URL
      await waitFor(() => {
        expect(currentCtx.draft.images).toHaveLength(1);
        expect(currentCtx.draft.images[0].url).toContain('blob:');
      });

      // No network upload request occurred
      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('12. Renders + color button and opens canonical color popover', async () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, colors: [], variants: [] }} />);
      const addColorBtn = screen.getByLabelText('Добавить цвет');
      expect(addColorBtn).toBeTruthy();

      fireEvent.click(addColorBtn);
      await waitFor(() => {
        expect(screen.getAllByText('Выберите цвет')[0]).toBeTruthy();
        expect(screen.getByText('Чёрный')).toBeTruthy();
        expect(screen.getByText('Белый')).toBeTruthy();
      });
    });

    it('13. Renders + size button and opens canonical size popover', async () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, variants: [] }} />);
      const addSizeBtn = screen.getByLabelText('Добавить размер');
      expect(addSizeBtn).toBeTruthy();

      fireEvent.click(addSizeBtn);
      await waitFor(() => {
        expect(screen.getAllByText('Выберите размер')[0]).toBeTruthy();
        expect(screen.getAllByText('S')[0]).toBeTruthy();
        expect(screen.getAllByText('M')[0]).toBeTruthy();
        expect(screen.getByText('L')).toBeTruthy();
      });
    });

    it('14. Renders composition edit button', () => {
      render(<TestWrapper />);
      const btn = screen.getByTestId('edit-composition-btn');
      expect(btn).toBeTruthy();
      expect(btn.textContent).toBe('Изменить');
    });

    it('15. Characteristics shows "Сначала выберите категорию товара" and "Выбрать категорию" when category missing', async () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, categoryId: '', categoryName: '' }} />);
      const accordion = screen.getByText('Характеристики');
      fireEvent.click(accordion);

      expect(screen.getByText(/Сначала выберите категорию товара/i)).toBeTruthy();
      const chooseCatBtn = screen.getByTestId('choose-category-characteristics-btn');
      expect(chooseCatBtn).toBeTruthy();
      expect(chooseCatBtn.textContent).toContain('Выбрать категорию');
      expect(screen.queryByText(/Добавить характеристику/i)).toBeNull();
      expect(document.body.innerHTML).not.toContain('TODO');
      expect(document.body.innerHTML).not.toContain('Hydration Gap');
    });
  });

  describe('C. Inline Editors Actually Edit Draft and Sync with Form', () => {
    it('16. Clicking title mounts visual inline editor, updates draft, reflects in Form', () => {
      let currentCtx: any;
      const { rerender } = render(
        <TestWrapper
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      // Click title
      fireEvent.click(screen.getByRole('heading', { level: 1, name: 'Классическая футболка' }));
      const input = screen.getByTestId('visual-inline-input') as HTMLInputElement;
      expect(input).toBeTruthy();
      expect(input.value).toBe('Классическая футболка');

      // Edit and blur to commit
      fireEvent.change(input, { target: { value: 'Оверсайз футболка ZAMK' } });
      fireEvent.blur(input);

      expect(currentCtx.draft.title).toBe('Оверсайз футболка ZAMK');

      // Form sees the updated value
      rerender(
        <TestWrapper initialDraft={currentCtx.draft}>
          <ProductStudioFormWorkspace />
        </TestWrapper>
      );
      expect((screen.getByTestId('form-product-title-input') as HTMLInputElement).value).toBe('Оверсайз футболка ZAMK');
    });

    it('17. Clicking price mounts visual inline editor, 8900 => 890000 cents, reflects in Form', () => {
      let currentCtx: any;
      const { rerender } = render(
        <TestWrapper
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(document.querySelector('[data-slot="price"]')!);
      const input = screen.getByTestId('visual-inline-input') as HTMLInputElement;
      expect(input).toBeTruthy();

      fireEvent.change(input, { target: { value: '8900' } });
      fireEvent.blur(input);

      expect(currentCtx.draft.priceCents).toBe(890000);

      // Form sees 8900
      rerender(
        <TestWrapper initialDraft={currentCtx.draft}>
          <ProductStudioFormWorkspace />
        </TestWrapper>
      );
      fireEvent.click(screen.getByTestId('studio-section-btn-pricing'));
      expect((screen.getByTestId('form-product-price-input') as HTMLInputElement).value).toBe('8900');
    });

    it('18. Clicking description mounts visual inline textarea, updates draft, reflects in Form', () => {
      let currentCtx: any;
      const { rerender } = render(
        <TestWrapper
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(document.querySelector('[data-slot="description"]')!);
      const textarea = screen.getByTestId('visual-inline-textarea') as HTMLTextAreaElement;
      expect(textarea).toBeTruthy();
      expect(textarea.value).toBe('Премиальный хлопок.');

      fireEvent.change(textarea, { target: { value: 'Новое многострочное описание\nСтрока 2' } });
      fireEvent.blur(textarea);

      expect(currentCtx.draft.description).toBe('Новое многострочное описание\nСтрока 2');

      // Form sees updated description
      rerender(
        <TestWrapper initialDraft={currentCtx.draft}>
          <ProductStudioFormWorkspace />
        </TestWrapper>
      );
      expect((screen.getByTestId('form-product-description-input') as HTMLTextAreaElement).value).toBe(
        'Новое многострочное описание\nСтрока 2'
      );
    });
  });

  describe('D. Canonical Color and Size Flow', () => {
    it('19. Selecting canonical color sets exact colorId from dictionary and renders swatch', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{ ...baseDraft, colors: [], variants: [] }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      screen.debug(undefined, 300000);

      fireEvent.click(screen.getByLabelText('Добавить цвет'));
      await waitFor(() => {
        expect(screen.getByText('Чёрный')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('Чёрный'));
      fireEvent.click(screen.getByRole('button', { name: 'Готово' }));

      // Draft colors contains exact canonical col-black
      expect(currentCtx.draft.colors).toHaveLength(1);
      expect(currentCtx.draft.colors[0].id).toBe('col-black');
      expect(currentCtx.draft.colors[0].id).not.toContain('Date.now');

      // Swatch appears in real PDP
      await waitFor(() => {
        expect(screen.getByTestId('color-swatch-col-black')).toBeTruthy();
      });
    });

    it('20. Selecting canonical size sets exact sizeValueId from dictionary, no sizeChart required', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{ ...baseDraft, variants: [], sizeChart: undefined }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByLabelText('Добавить размер'));
      await waitFor(() => {
        expect(screen.getByText('S')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('S'));
      fireEvent.click(screen.getByRole('button', { name: 'Готово' }));

      // Draft variant uses canonical sz-s
      expect(currentCtx.draft.variants).toHaveLength(1);
      expect(currentCtx.draft.variants[0].sizeValueId).toBe('sz-s');
      expect(currentCtx.draft.variants[0].size).toBe('S');
      expect(currentCtx.draft.sizeChart).toBeUndefined();

      // Size button appears in real PDP
      await waitFor(() => {
        expect(screen.getByTestId('size-button-S')).toBeTruthy();
      });
    });
  });

  describe('E. Cartesian Combinations and Removal', () => {
    it('21. Adding color expands Cartesian combinations across existing sizes', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
            variants: [
              { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
              { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' },
            ],
          }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByLabelText('Управление цветами'));
      await waitFor(() => {
        expect(screen.getByText('Белый')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('Белый'));
      fireEvent.click(screen.getByRole('button', { name: 'Готово' }));

      // Result has 4 variants: Black/S, Black/M, White/S, White/M
      expect(currentCtx.draft.variants).toHaveLength(4);
      expect(currentCtx.draft.variants.find((v: any) => v.colorId === 'col-white' && v.sizeValueId === 'sz-s')).toBeTruthy();
      expect(currentCtx.draft.variants.find((v: any) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m')).toBeTruthy();
    });

    it('22. Removing color prunes matching variants only and cleans draft.colors', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            colors: [
              { id: 'col-black', name: 'Чёрный', hex: '#000000' },
              { id: 'col-white', name: 'Белый', hex: '#ffffff' },
            ],
            variants: [
              { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
              { id: 'v2', colorId: 'col-white', sizeValueId: 'sz-s', size: 'S' },
            ],
          }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      // No permanent cross button on swatches
      expect(screen.queryByLabelText('Удалить цвет Чёрный')).toBeNull();

      // Open color manager popover
      fireEvent.click(screen.getByLabelText('Управление цветами'));
      await waitFor(() => {
        expect(screen.getByTestId('color-popover')).toBeTruthy();
      });

      const colorPopover = screen.getByTestId('color-popover');
      expect(within(colorPopover).getByText('Чёрный')).toBeTruthy();

      // Uncheck Black
      fireEvent.click(within(colorPopover).getByText('Чёрный'));
      fireEvent.click(within(colorPopover).getByRole('button', { name: 'Готово' }));

      expect(currentCtx.draft.colors).toHaveLength(1);
      expect(currentCtx.draft.colors[0].id).toBe('col-white');
      expect(currentCtx.draft.variants).toHaveLength(1);
      expect(currentCtx.draft.variants[0].colorId).toBe('col-white');
    });

    it('23. Removing size prunes matching variants only', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            variants: [
              { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
              { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' },
            ],
          }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      // No permanent cross button on size buttons
      expect(screen.queryByLabelText('Удалить размер M')).toBeNull();

      // Open size manager popover
      fireEvent.click(screen.getByLabelText('Управление размерами'));
      await waitFor(() => {
        expect(screen.getByTestId('size-popover')).toBeTruthy();
      });

      const sizePopover = screen.getByTestId('size-popover');
      await waitFor(() => {
        expect(within(sizePopover).getByRole('button', { name: 'M' })).toBeTruthy();
      });

      // Uncheck M
      fireEvent.click(within(sizePopover).getByRole('button', { name: 'M' }));
      fireEvent.click(within(sizePopover).getByRole('button', { name: 'Готово' }));

      expect(currentCtx.draft.variants).toHaveLength(1);
      expect(currentCtx.draft.variants[0].sizeValueId).toBe('sz-s');
    });
  });

  describe('F. Legacy Components Removed and Safety Invariants', () => {
    it('24. No generic variants editor shell exists', () => {
      render(<TestWrapper />);
      expect(screen.queryByTestId('variants-editor-shell')).toBeNull();
    });

    it('25. No generic media editor shell exists', () => {
      render(<TestWrapper />);
      expect(screen.queryByTestId('media-editor-shell')).toBeNull();
    });

    it('26. No generic characteristics editor shell exists', () => {
      render(<TestWrapper />);
      expect(screen.queryByTestId('characteristics-editor-shell')).toBeNull();
    });

    it('27. Does not crash with completely empty draft', () => {
      const emptyDraft: Partial<ProductStudioDraft> = {
        id: 'empty',
        colors: [],
        variants: [],
        images: [],
        attributes: [],
        sizeChart: { rows: [] },
      };
      render(<TestWrapper initialDraft={emptyDraft} />);
      expect(screen.getAllByText(/Добавить фото/i).length).toBeGreaterThan(0);
    });

    it('28. DimensionType ONLY_SIZE hides color swatch section', () => {
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            dimensionType: 'ONLY_SIZE',
            colors: [],
            variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S' }],
          }}
        />
      );
      expect(screen.queryByText('Цвет:')).toBeNull();
    });

    it('29. VisualWorkspace renders without generic wrappers and preserves FBO safety', () => {
      let currentCtx: any;
      render(<TestWrapper contextCallback={(ctx) => (currentCtx = ctx)} />);
      expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();

      // No stock on variants
      currentCtx.draft.variants.forEach((v: any) => {
        expect(v.stock).toBeUndefined();
        expect(v.initialStock).toBeUndefined();
      });
    });

    it('30. Photo color assignment popover assigns color to media item', async () => {
      let currentCtx: any;
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            images: [
              { id: 'img-1', url: 'https://images/1.jpg', colorId: null, sortOrder: 0 },
            ],
            colors: [
              { id: 'col-black', name: 'Чёрный', hex: '#000000' },
              { id: 'col-white', name: 'Белый', hex: '#ffffff' },
            ],
          }}
          contextCallback={(ctx) => (currentCtx = ctx)}
        />
      );

      // Verify no per-thumbnail popovers or buttons
      expect(screen.queryByTestId('media-color-assign-btn-0')).toBeNull();

      const openModalBtn = screen.getByTestId('bind-photos-to-colors-btn');
      expect(openModalBtn).toBeTruthy();
      fireEvent.click(openModalBtn);

      await waitFor(() => {
        expect(screen.getByTestId('photo-color-binding-modal')).toBeTruthy();
        expect(screen.getByText('Привязка фотографий к цветам')).toBeTruthy();
      });

      const modal = screen.getByTestId('photo-color-binding-modal');
      const select0 = within(modal).getByTestId('photo-color-select-0') as HTMLSelectElement;
      expect(select0.value).toBe('');

      // Change selection to col-white
      fireEvent.change(select0, { target: { value: 'col-white' } });

      // Save modal
      const saveBtn = within(modal).getByTestId('photo-color-modal-submit');
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(currentCtx.draft.images[0].colorId).toBe('col-white');
        expect(screen.getAllByTestId('thumbnail-color-dot-0')[0]).toBeTruthy();
      });
    });

    it('31. Simplified empty media copy and format recommendation tooltip', async () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, images: [] }} />);

      const emptySlot = screen.getByTestId('presentation-empty-media-slot');
      expect(emptySlot.textContent).toContain('Добавить фото *');
      expect(emptySlot.textContent).toContain('Первая — обложка');
      expect(emptySlot.textContent).toContain('Вертикальные · от 800×1000 · JPG/PNG/WebP · до 10 МБ');

      const infoBtn = screen.getByTitle('Рекомендация по формату');
      fireEvent.click(infoBtn);

      await waitFor(() => {
        expect(screen.getByText(/Лучше использовать формат 4:5/)).toBeTruthy();
      });
    });

    it('32. Care Discoverability: renders neutral Уход block and interactive control without red required styling', () => {
      render(<TestWrapper initialDraft={{ ...baseDraft, careInstructions: undefined }} />);

      const careSection = screen.getByTestId('visual-care-section');
      expect(careSection).toBeTruthy();
      expect(within(careSection).getByText('Уход')).toBeTruthy();
      expect(within(careSection).getByText('Необязательно')).toBeTruthy();

      const addCareBtn = screen.getByTestId('add-care-btn');
      expect(addCareBtn).toBeTruthy();
      expect(addCareBtn.textContent).toContain('Добавить уход');
      expect(addCareBtn.className).not.toContain('text-red');
      expect(addCareBtn.className).not.toContain('border-red');
      expect(careSection.className).not.toContain('border-red');
    });

    it('33. Composition entry button clickability and sibling modal actions wiring regression', async () => {
      // Draft with undefined materialComposition (like a new product at /products/new)
      render(
        <TestWrapper
          initialDraft={{
            ...baseDraft,
            materialComposition: undefined,
            material: '',
            careInstructions: undefined,
            categoryId: 'cat-clothing',
            categoryName: 'Одежда',
            variants: [{ size: 'S', colorName: 'Черный', colorHex: '#000000', priceCents: 1000 }],
          }}
        />
      );

      // A. "Добавить состав" opens ProductStudioCompositionModal
      const addCompBtn = screen.getByTestId('add-composition-btn');
      expect(addCompBtn).toBeTruthy();
      fireEvent.click(addCompBtn);

      await waitFor(() => {
        expect(screen.getByTestId('composition-modal')).toBeTruthy();
      });

      // Close composition modal
      const compModal = screen.getByTestId('composition-modal');
      fireEvent.click(within(compModal).getByLabelText('Закрыть'));
      await waitFor(() => {
        expect(screen.queryByTestId('composition-modal')).toBeNull();
      });

      // B. "Добавить уход" still opens Care modal
      const addCareBtn = screen.getByTestId('add-care-btn');
      expect(addCareBtn).toBeTruthy();
      fireEvent.click(addCareBtn);

      await waitFor(() => {
        expect(screen.getByTestId('care-modal')).toBeTruthy();
      });

      // Close care modal
      const careModal = screen.getByTestId('care-modal');
      fireEvent.click(within(careModal).getByLabelText('Закрыть'));
      await waitFor(() => {
        expect(screen.queryByTestId('care-modal')).toBeNull();
      });

      // C. "Заполнить характеристики" / "Настроить характеристики" opens Characteristics modal
      fireEvent.click(screen.getByText('Характеристики'));
      await waitFor(() => {
        expect(screen.getByTestId('manage-characteristics-btn')).toBeTruthy();
      });
      const charBtn = screen.getByTestId('manage-characteristics-btn');
      fireEvent.click(charBtn);

      await waitFor(() => {
        expect(screen.getByTestId('characteristics-modal')).toBeTruthy();
      });

      // Close characteristics modal
      const charModal = screen.getByTestId('characteristics-modal');
      fireEvent.click(within(charModal).getByLabelText('Закрыть'));
      await waitFor(() => {
        expect(screen.queryByTestId('characteristics-modal')).toBeNull();
      });

      // D. "Таблица размеров" opens Size Chart modal
      const sizeChartBtn = screen.getByTestId('size-chart-btn');
      expect(sizeChartBtn).toBeTruthy();
      fireEvent.click(sizeChartBtn);

      await waitFor(() => {
        expect(screen.getByTestId('size-chart-modal')).toBeTruthy();
      });
    });
  });
});

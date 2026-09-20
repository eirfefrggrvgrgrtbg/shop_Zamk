/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudio } from './ProductStudio';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';

const mockCategories = [
  { id: 'cat-root-clothing', name: 'Одежда', parentId: null, sortOrder: 1 },
  { id: 'cat-outerwear', name: 'Верхняя одежда', parentId: 'cat-root-clothing', sortOrder: 1 },
  { id: 'cat-jackets', name: 'Куртки', parentId: 'cat-outerwear', sortOrder: 1 },
  { id: 'cat-tshirts', name: 'Футболки', parentId: 'cat-root-clothing', sortOrder: 2 },
  { id: 'cat-accessories', name: 'Аксессуары (без размеров)', parentId: null, sortOrder: 2 },
];

const mockCategorySchemas: Record<string, any> = {
  'cat-jackets': {
    id: 'sch-jackets',
    categoryId: 'cat-jackets',
    dimensionType: 'COLOR_AND_SIZE',
    allowedSizeSystems: [
      { id: 'sys-int', name: 'INT', isDefault: true },
      { id: 'sys-ru', name: 'RU', isDefault: false },
    ],
  },
  'cat-tshirts': {
    id: 'sch-tshirts',
    categoryId: 'cat-tshirts',
    dimensionType: 'COLOR_AND_SIZE',
    allowedSizeSystems: [
      { id: 'sys-int', name: 'INT', isDefault: true },
    ],
  },
  'cat-accessories': {
    id: 'sch-acc',
    categoryId: 'cat-accessories',
    dimensionType: 'COLOR_AND_SIZE',
    allowedSizeSystems: [], // No size systems configured
  },
};

const mockSizeValues: Record<string, any[]> = {
  'sys-int': [
    { id: 'sz-s', sizeSystemId: 'sys-int', value: 'S' },
    { id: 'sz-m', sizeSystemId: 'sys-int', value: 'M' },
  ],
  'sys-ru': [
    { id: 'sz-48', sizeSystemId: 'sys-ru', value: '48' },
    { id: 'sz-50', sizeSystemId: 'sys-ru', value: '50' },
  ],
};

const mockColors = [
  { id: 'col-beige', nameRu: 'Бежевый', hex: '#f5f5dc' },
  { id: 'col-white', nameRu: 'Белый', hex: '#ffffff' },
];

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve(mockColors)),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve(mockCategories)),
    getSellerCategorySchema: vi.fn().mockImplementation((catId: string) =>
      Promise.resolve(mockCategorySchemas[catId] || { id: 'sch-def', categoryId: catId, allowedSizeSystems: [] })
    ),
    getSellerSizeValues: vi.fn().mockImplementation((sysId: string) =>
      Promise.resolve(mockSizeValues[sysId] || [])
    ),
  };
});

function StudioTestHarness({
  initialDraft = {},
  onContext,
}: {
  initialDraft?: Partial<ProductStudioDraft>;
  onContext?: (ctx: ReturnType<typeof useProductStudio>) => void;
}) {
  function Inspector() {
    const ctx = useProductStudio();
    if (onContext) onContext(ctx);
    return null;
  }

  return (
    <MemoryRouter>
      <ProductStudioProvider entryMode="create" initialDraft={initialDraft}>
        <Inspector />
        <ProductStudio />
      </ProductStudioProvider>
    </MemoryRouter>
  );
}

describe('PS.R4A.5A — Hero Builder Usability Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe('1. Studio Toolbar Category Indicator & Modal Flow', () => {
    it('renders "Категория * · Не выбрана" when category is not set', () => {
      render(<StudioTestHarness initialDraft={{ title: 'Тестовый товар' }} />);
      const catBtn = screen.getByTestId('studio-header-category-btn');
      expect(catBtn.textContent).toContain('Категория * · Не выбрана');
    });

    it('renders "Категория · [Name]" when category is set', () => {
      render(
        <StudioTestHarness
          initialDraft={{ categoryId: 'cat-jackets', categoryName: 'Куртки' }}
        />
      );
      const catBtn = screen.getByTestId('studio-header-category-btn');
      expect(catBtn.textContent).toContain('Категория · Куртки');
    });

    it('opens category modal upon clicking category button', async () => {
      render(<StudioTestHarness initialDraft={{}} />);
      fireEvent.click(screen.getByTestId('studio-header-category-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('category-modal')).toBeTruthy();
        expect(screen.getByText('Выберите категорию товара')).toBeTruthy();
      });
    });

    it('drills down parent categories and allows only leaf selection', async () => {
      let currentCtx: any;
      render(
        <StudioTestHarness
          initialDraft={{}}
          onContext={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByTestId('studio-header-category-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('category-parent-cat-root-clothing')).toBeTruthy();
      });

      // Click root clothing parent -> drills down
      fireEvent.click(screen.getByTestId('category-parent-cat-root-clothing'));

      await waitFor(() => {
        expect(screen.getByText('Верхняя одежда')).toBeTruthy();
        expect(screen.getByTestId('category-leaf-cat-tshirts')).toBeTruthy();
      });

      // Click leaf "Футболки"
      fireEvent.click(screen.getByTestId('category-leaf-cat-tshirts'));

      // Modal closes, draft updated
      await waitFor(() => {
        expect(screen.queryByTestId('category-modal')).toBeNull();
        expect(currentCtx.draft.categoryId).toBe('cat-tshirts');
        expect(currentCtx.draft.categoryName).toBe('Футболки');
      });
    });

    it('filters leaf categories via search query', async () => {
      let currentCtx: any;
      render(
        <StudioTestHarness
          initialDraft={{}}
          onContext={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByTestId('studio-header-category-btn'));

      await waitFor(() => {
        expect(screen.getByTestId('category-search-input')).toBeTruthy();
      });

      fireEvent.change(screen.getByTestId('category-search-input'), {
        target: { value: 'Курт' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('category-leaf-cat-jackets')).toBeTruthy();
        expect(screen.getByText(/Одежда › Верхняя одежда › Куртки/)).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('category-leaf-cat-jackets'));

      await waitFor(() => {
        expect(screen.queryByTestId('category-modal')).toBeNull();
        expect(currentCtx.draft.categoryId).toBe('cat-jackets');
      });
    });
  });

  describe('2. Size Canonical Flow After Category', () => {
    it('prompts to select category when size + clicked without category', async () => {
      render(<StudioTestHarness initialDraft={{ categoryId: undefined }} />);

      fireEvent.click(screen.getByLabelText('Добавить размер'));

      await waitFor(() => {
        expect(screen.getByText('Сначала выберите категорию товара')).toBeTruthy();
      });

      // Direct button opens category modal
      fireEvent.click(screen.getByRole('button', { name: 'Выбрать категорию' }));

      await waitFor(() => {
        expect(screen.getByTestId('category-modal')).toBeTruthy();
      });
    });

    it('displays unconfigured size notice with change category button when category has 0 systems', async () => {
      render(
        <StudioTestHarness
          initialDraft={{ categoryId: 'cat-accessories', categoryName: 'Аксессуары' }}
        />
      );

      fireEvent.click(screen.getByLabelText('Добавить размер'));

      await waitFor(() => {
        expect(
          screen.getByText('Для этой категории пока не настроены размеры')
        ).toBeTruthy();
        expect(screen.getByRole('button', { name: 'Выбрать другую категорию' })).toBeTruthy();
      });

      fireEvent.click(screen.getByRole('button', { name: 'Выбрать другую категорию' }));

      await waitFor(() => {
        expect(screen.getByTestId('category-modal')).toBeTruthy();
      });
    });

    it('supports multiple size systems and switching between systems', async () => {
      let currentCtx: any;
      render(
        <StudioTestHarness
          initialDraft={{
            categoryId: 'cat-jackets',
            categoryName: 'Куртки',
            variants: [],
          }}
          onContext={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByLabelText('Добавить размер'));

      // Both INT and RU tabs are visible
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'INT' })).toBeTruthy();
        expect(screen.getByRole('button', { name: 'RU' })).toBeTruthy();
        expect(screen.getByText('S')).toBeTruthy();
      });

      // Switch to RU
      fireEvent.click(screen.getByRole('button', { name: 'RU' }));

      await waitFor(() => {
        expect(screen.getByText('48')).toBeTruthy();
        expect(screen.getByText('50')).toBeTruthy();
      });

      // Pick size 48
      fireEvent.click(screen.getByText('48'));
      fireEvent.click(screen.getByRole('button', { name: 'Готово' }));

      await waitFor(() => {
        expect(currentCtx.draft.variants).toHaveLength(1);
        expect(currentCtx.draft.variants[0].sizeValueId).toBe('sz-48');
        expect(currentCtx.draft.variants[0].size).toBe('48');
      });
    });
  });

  describe('3. Color Canonical Rendering', () => {
    it('handles color dictionary returning hex and renders swatch with exact color', async () => {
      let currentCtx: any;
      render(
        <StudioTestHarness
          initialDraft={{ colors: [], variants: [] }}
          onContext={(ctx) => (currentCtx = ctx)}
        />
      );

      fireEvent.click(screen.getByLabelText('Добавить цвет'));

      await waitFor(() => {
        expect(screen.getByText('Бежевый')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('Бежевый'));
      fireEvent.click(screen.getByRole('button', { name: 'Готово' }));

      await waitFor(() => {
        const swatch = screen.getByTestId('color-swatch-col-beige');
        expect(swatch).toBeTruthy();
        // DOM serializes hex #f5f5dc as rgb(245, 245, 220) in inline style
        expect(swatch.innerHTML).toContain('rgb(245, 245, 220)');
        expect(currentCtx.draft.colors[0].hex).toBe('#f5f5dc');
      });
    });
  });

  describe('4. Photo Create Stage and Guidance', () => {
    it('renders 4:5 empty media slot with canonical guidance when images are empty', () => {
      render(<StudioTestHarness initialDraft={{ images: [] }} />);

      const emptySlot = screen.getByTestId('presentation-empty-media-slot');
      expect(emptySlot).toBeTruthy();
      expect(emptySlot.textContent).toContain('Добавить фото *');
      expect(emptySlot.textContent).toContain('Первая — обложка');
      expect(emptySlot.textContent).toContain('0 из 3');
      expect(emptySlot.textContent).toContain('Вертикальные · от 800×1000 · JPG/PNG/WebP · до 10 МБ');

      // format recommendation popover
      const infoBtn = screen.getByTitle('Рекомендация по формату');
      expect(infoBtn).toBeTruthy();
      fireEvent.click(infoBtn);
      expect(screen.getByText('Лучше использовать формат 4:5 — фото лучше заполняет карточку товара')).toBeTruthy();
    });

    it('shows validation error when selecting an invalid file type', async () => {
      render(<StudioTestHarness initialDraft={{ images: [] }} />);

      const fileInput = screen.getByTestId('empty-stage-photo-input') as HTMLInputElement;
      const invalidFile = new File(['dummy'], 'test.pdf', { type: 'application/pdf' });

      fireEvent.change(fileInput, { target: { files: [invalidFile] } });

      await waitFor(() => {
        const err = screen.getByTestId('media-upload-error');
        expect(err.textContent).toContain('Поддерживаются JPG, PNG и WebP');
      });
    });
  });
});

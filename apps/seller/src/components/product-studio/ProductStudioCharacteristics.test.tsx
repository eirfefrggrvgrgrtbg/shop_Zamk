/* @vitest-environment jsdom */
import { vi, describe, it, expect, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import { ProductStudioCharacteristicsModal } from './ProductStudioCharacteristicsModal';
import { ProductStudioPhotoColorModal, getColorDisplayName } from './ProductStudioPhotoColorModal';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';
import type { SellerCategorySchema } from '@zamk/api-client';

afterEach(() => {
  cleanup();
});

const mockSchemaWithoutName: SellerCategorySchema = {
  id: 'cat-hoodies',
  slug: 'hoodies',
  name: undefined as any,
  dimensionType: 'COLOR_AND_SIZE',
  sizeChartRequired: true,
  allowedSizeSystems: [{ id: 'sys-eu', code: 'EU', name: 'EU', isDefault: true }],
  attributes: [
    {
      id: 'attr-season',
      code: 'SEASON',
      nameRu: 'Сезон',
      valueType: 'ENUM',
      valueSource: 'DICTIONARY',
      scope: 'PRODUCT',
      required: true,
      filterable: true,
      variantAxis: false,
      sortOrder: 1,
      dictionaryId: 'dict-season',
    },
  ],
  sizeChartFields: [],
};

const mockSchemaWithName: SellerCategorySchema = {
  ...mockSchemaWithoutName,
  name: 'Платья',
};

vi.mock('@zamk/api-client', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerCategorySchema: vi.fn().mockImplementation((id: string) => {
      if (id === 'cat-with-name') return Promise.resolve(mockSchemaWithName);
      return Promise.resolve(mockSchemaWithoutName);
    }),
    getSellerDictionaryValues: vi.fn().mockResolvedValue([
      { id: 'val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
    ]),
  };
});

function FormTestWrapper({
  initialDraft,
  initialCategorySchema,
}: {
  initialDraft: Partial<ProductStudioDraft>;
  initialCategorySchema?: SellerCategorySchema | null;
}) {
  return (
    <MemoryRouter>
      <ProductStudioProvider
        entryMode="edit"
        initialDraft={initialDraft}
        initialCategorySchema={initialCategorySchema}
      >
        <ProductStudioFormWorkspace />
      </ProductStudioProvider>
    </MemoryRouter>
  );
}

describe('ProductStudio Form Characteristics Category Display', () => {
  it('renders resolved category name from draft.categoryName when schema has no name', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-1',
          categoryId: 'cat-hoodies',
          categoryName: 'Худи',
          attributes: [],
        }}
        initialCategorySchema={mockSchemaWithoutName}
      />
    );

    // Switch to characteristics section
    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    await waitFor(() => {
      expect(
        screen.getByText('Категория: Худи. Заполнено 0 из 1 обязательных.')
      ).toBeDefined();
    });

    // Ensure "undefined" is NEVER present
    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('renders resolved category name from categorySchema.name as fallback when draft.categoryName is missing', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-2',
          categoryId: 'cat-with-name',
          categoryName: undefined,
          attributes: [],
        }}
        initialCategorySchema={mockSchemaWithName}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    await waitFor(() => {
      expect(
        screen.getByText('Категория: Платья. Заполнено 0 из 1 обязательных.')
      ).toBeDefined();
    });

    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('renders generic completeness count and NEVER renders "undefined" when neither draft nor schema has name', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-3',
          categoryId: 'cat-hoodies',
          categoryName: '',
          attributes: [],
        }}
        initialCategorySchema={mockSchemaWithoutName}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    await waitFor(() => {
      expect(
        screen.getByText('Заполнено 0 из 1 обязательных.')
      ).toBeDefined();
    });

    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('renders "Сначала выберите категорию товара." when no category is selected', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          categoryId: '',
          categoryName: '',
          attributes: [],
        }}
        initialCategorySchema={null}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    expect(screen.getByText('Сначала выберите категорию товара.')).toBeDefined();
    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('regression: computes "1 из 1 обязательных" when required characteristic is filled', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-4',
          categoryId: 'cat-hoodies',
          categoryName: 'Худи',
          attributes: [
            {
              attributeDefinitionId: 'attr-season',
              name: 'Сезон',
              dictionaryValueId: 'val-winter',
              value: 'Зима',
            },
          ],
        }}
        initialCategorySchema={mockSchemaWithoutName}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    await waitFor(() => {
      expect(
        screen.getByText('Категория: Худи. Заполнено 1 из 1 обязательных.')
      ).toBeDefined();
    });

    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('ProductStudioCharacteristicsModal renders categoryName and never "undefined"', () => {
    render(
      <ProductStudioCharacteristicsModal
        isOpen={true}
        onClose={() => {}}
        schema={mockSchemaWithoutName}
        categoryName="Худи"
        attributes={[]}
        onSave={() => {}}
      />
    );

    expect(
      screen.getByText('Категория: Худи. Заполните канонические свойства модели.')
    ).toBeDefined();
    expect(screen.queryByText(/undefined/)).toBeNull();
  });

  it('PS.R4B2.3: Form Media renders persisted photos, allows reorder, cover selection, and deletion', async () => {
    const initialDraft: Partial<ProductStudioDraft> = {
      id: 'prod-media-test',
      title: 'Худи оверсайз',
      images: [
        {
          uiKey: 'img-1',
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://example.com/photo1.jpg' },
          isMain: true,
          sortOrder: 0,
          colorId: null,
        },
        {
          uiKey: 'img-2',
          source: { kind: 'canonical', imageId: 'img-2', url: 'https://example.com/photo2.jpg' },
          isMain: false,
          sortOrder: 1,
          colorId: null,
        },
        {
          uiKey: 'img-3',
          source: { kind: 'canonical', imageId: 'img-3', url: 'https://example.com/photo3.jpg' },
          isMain: false,
          sortOrder: 2,
          colorId: null,
        },
      ],
    };

    render(
      <FormTestWrapper
        initialDraft={initialDraft}
        initialCategorySchema={mockSchemaWithoutName}
      />
    );

    // Switch to Media section
    fireEvent.click(screen.getByTestId('studio-section-btn-media'));

    // Check that 3 cards are rendered
    expect(screen.getByTestId('form-media-card-0')).toBeTruthy();
    expect(screen.getByTestId('form-media-card-1')).toBeTruthy();
    expect(screen.getByTestId('form-media-card-2')).toBeTruthy();

    // Check that first card has cover badge
    expect(screen.getByTestId('form-media-cover-badge-0')).toBeTruthy();
    expect(screen.queryByTestId('form-media-cover-badge-1')).toBeNull();

    // Second card has "Сделать обложкой"
    const makeCoverBtn = screen.getByTestId('form-media-make-cover-1');
    fireEvent.click(makeCoverBtn);

    // Now img-2 is first
    const imgs = screen.getAllByRole('img');
    expect(imgs[0].getAttribute('src')).toBe('https://example.com/photo2.jpg');

    // Delete first image
    const deleteBtn = screen.getByTestId('form-media-delete-btn-0');
    fireEvent.click(deleteBtn);

    // Now only 2 images remain
    expect(screen.queryByTestId('form-media-card-2')).toBeNull();
  });

  it('PS.R4B2.3: Form Characteristics when category has only MATERIAL_COMPOSITION shows consistent empty message', async () => {
    const schemaWithMaterialOnly: SellerCategorySchema = {
      ...mockSchemaWithoutName,
      attributes: [
        {
          id: 'attr-mat',
          code: 'MATERIAL_COMPOSITION',
          nameRu: 'Состав',
          valueType: 'COMPOSITION',
          valueSource: 'MATERIAL_COMPOSITION',
          scope: 'PRODUCT',
          required: true,
          filterable: false,
          variantAxis: false,
          sortOrder: 1,
        },
      ],
    };

    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-hoodie-mat',
          categoryId: 'cat-hoodies',
          categoryName: 'Худи',
          attributes: [],
        }}
        initialCategorySchema={schemaWithMaterialOnly}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-characteristics'));

    // Summary reflects 0 additional characteristics
    expect(
      screen.getByText('Категория: Худи. Для данной категории нет дополнительных характеристик.')
    ).toBeTruthy();

    // Modal also reflects 0 additional characteristics
    fireEvent.click(screen.getByTestId('form-manage-characteristics-btn'));
    expect(
      screen.getByText('Для данной категории нет дополнительных характеристик.')
    ).toBeTruthy();
  });

  it('PS.R4B2.3: Form Review renders structured readiness cards with navigation', async () => {
    render(
      <FormTestWrapper
        initialDraft={{
          id: 'prod-review-test',
          title: 'Худи оверсайз',
          description: 'Тестовое описание',
          categoryId: 'cat-hoodies',
          categoryName: 'Худи',
          priceCents: 100000,
          images: [
            {
              uiKey: 'img-1',
              source: { kind: 'canonical', imageId: 'img-1', url: 'https://example.com/photo1.jpg' },
              isMain: true,
              sortOrder: 0,
            },
          ], // Incomplete media
          materialComposition: [], // Incomplete composition
        }}
        initialCategorySchema={mockSchemaWithoutName}
      />
    );

    fireEvent.click(screen.getByTestId('studio-section-btn-review'));

    expect(screen.getByTestId('review-readiness-badge')).toBeTruthy();
    expect(screen.getByTestId('review-section-card-basics')).toBeTruthy();
    expect(screen.getByTestId('review-section-card-media')).toBeTruthy();
    expect(screen.getByTestId('review-section-card-characteristics')).toBeTruthy();

    const fixMediaBtn = screen.getByTestId('review-fix-btn-media');
    expect(fixMediaBtn.textContent).toBe('Исправить');
    fireEvent.click(fixMediaBtn);

    expect(screen.getByTestId('studio-section-panel-media')).toBeTruthy();
  });

  describe('PS.R4B2.4 — Redesigned Photo-to-Color Binding UX and Form Media Grid', () => {
    const mockImages: import('../../contexts/ProductStudioContext').ProductStudioImage[] = [
      {
        uiKey: 'img-1',
        source: { kind: 'canonical', imageId: 'img-1', url: 'https://example.com/p1.jpg' },
        isMain: true,
        sortOrder: 0,
        colorId: null,
      },
      {
        uiKey: 'img-2',
        source: { kind: 'canonical', imageId: 'img-2', url: 'https://example.com/p2.jpg' },
        isMain: false,
        sortOrder: 1,
        colorId: null,
      },
    ];
    const mockColors = [
      { id: 'col-black', name: 'Чёрный', hex: '#000000' },
      { id: 'col-red', name: 'Красный', hex: '#ff0000' },
    ];

    it('renders two-column layout with left photos list and right color target buttons', () => {
      render(
        <ProductStudioPhotoColorModal
          isOpen={true}
          onClose={() => {}}
          images={mockImages}
          colors={mockColors}
          onSave={() => {}}
        />
      );

      // Modal container and summary bar
      expect(screen.getByTestId('photo-color-binding-modal')).toBeTruthy();
      expect(screen.getByTestId('photo-color-summary-bar')).toBeTruthy();
      expect(screen.getByText('Общие:')).toBeTruthy();
      expect(screen.getByText('Чёрный:')).toBeTruthy();
      expect(screen.getByText('Красный:')).toBeTruthy();

      // Left column photo cards
      expect(screen.getByTestId('photo-card-0')).toBeTruthy();
      expect(screen.getByTestId('photo-card-1')).toBeTruthy();

      // Right column target buttons
      expect(screen.getByTestId('photo-bind-target-general')).toBeTruthy();
      expect(screen.getByTestId('photo-bind-target-col-black')).toBeTruthy();
      expect(screen.getByTestId('photo-bind-target-col-red')).toBeTruthy();

      // Helper callout
      expect(screen.getByText(/Если фото подходит для всех вариантов/)).toBeTruthy();
    });

    it('binds selected photo to color and updates assignments, badges, and counts', () => {
      const onSaveMock = vi.fn();
      render(
        <ProductStudioPhotoColorModal
          isOpen={true}
          onClose={() => {}}
          images={mockImages}
          colors={mockColors}
          onSave={onSaveMock}
        />
      );

      // Photo 0 is selected by default; assign to black
      fireEvent.click(screen.getByTestId('photo-bind-target-col-black'));
      expect(screen.getByTestId('photo-assigned-badge-0').textContent).toContain('Чёрный');

      // Select photo 1 on the left and assign to red
      fireEvent.click(screen.getByTestId('photo-card-1'));
      fireEvent.click(screen.getByTestId('photo-bind-target-col-red'));
      expect(screen.getByTestId('photo-assigned-badge-1').textContent).toContain('Красный');

      // Re-assign photo 1 back to General
      fireEvent.click(screen.getByTestId('photo-bind-target-general'));
      expect(screen.getByTestId('photo-assigned-badge-1').textContent).toContain('Общее');

      // Submit modal
      fireEvent.click(screen.getByTestId('photo-color-modal-submit'));
      expect(onSaveMock).toHaveBeenCalledWith([
        expect.objectContaining({ uiKey: 'img-1', colorId: 'col-black' }),
        expect.objectContaining({ uiKey: 'img-2', colorId: null }),
      ]);
    });

    it('Form Workspace media toolbar does NOT render arrow controls and integrates with binding modal', async () => {
      const initialDraft: Partial<ProductStudioDraft> = {
        id: 'prod-media-grid-test',
        title: 'Тестовый товар',
        images: mockImages,
        colors: mockColors,
      };

      render(
        <FormTestWrapper
          initialDraft={initialDraft}
          initialCategorySchema={mockSchemaWithoutName}
        />
      );

      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // Verify NO arrow reorder buttons exist on media cards
      expect(screen.queryByTestId('form-media-move-up-0')).toBeNull();
      expect(screen.queryByTestId('form-media-move-down-0')).toBeNull();
      expect(screen.queryByTestId('form-media-move-up-1')).toBeNull();
      expect(screen.queryByTestId('form-media-move-down-1')).toBeNull();

      // Verify replace and delete controls exist
      expect(screen.getByTestId('form-media-replace-btn-0')).toBeTruthy();
      expect(screen.getByTestId('form-media-delete-btn-0')).toBeTruthy();
      expect(screen.getByTestId('form-media-make-cover-1')).toBeTruthy();

      // Open binding modal from Form header
      const openModalBtn = screen.getByTestId('form-bind-photos-to-colors-btn');
      fireEvent.click(openModalBtn);

      expect(screen.getByTestId('photo-color-binding-modal')).toBeTruthy();

      // Assign photo 0 to red
      fireEvent.click(screen.getByTestId('photo-bind-target-col-red'));
      fireEvent.click(screen.getByTestId('photo-color-modal-submit'));

      // Modal closed, color badge appears on media card in Form
      await waitFor(() => {
        expect(screen.queryByTestId('photo-color-binding-modal')).toBeNull();
      });
      expect(screen.getByTestId('form-media-color-badge-0').textContent).toContain('Красный');
    });
  });

  describe('PS.R4B2.4.1 — Restore media ordering via drag-and-drop without arrow buttons', () => {
    it('supports drag-and-drop reorder: preserves persisted IDs, colorId, normalizes sortOrder (A B C D -> A D B C)', async () => {
      let currentDraft: Partial<ProductStudioDraft> | null = null;
      function ContextWatcher() {
        const { draft } = useProductStudio();
        currentDraft = draft;
        return null;
      }

      const initialImages = [
        {
          uiKey: 'img-A',
          source: { kind: 'canonical' as const, imageId: 'img-A', url: 'https://example.com/A.jpg' },
          isMain: true,
          sortOrder: 0,
          colorId: null,
        },
        {
          uiKey: 'img-B',
          source: { kind: 'canonical' as const, imageId: 'img-B', url: 'https://example.com/B.jpg' },
          isMain: false,
          sortOrder: 1,
          colorId: 'col-black',
        },
        {
          uiKey: 'img-C',
          source: { kind: 'canonical' as const, imageId: 'img-C', url: 'https://example.com/C.jpg' },
          isMain: false,
          sortOrder: 2,
          colorId: 'col-red',
        },
        {
          uiKey: 'img-D',
          source: { kind: 'canonical' as const, imageId: 'img-D', url: 'https://example.com/D.jpg' },
          isMain: false,
          sortOrder: 3,
          colorId: null,
        },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="edit"
            initialDraft={{
              id: 'prod-reorder-test',
              images: initialImages,
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000' },
                { id: 'col-red', name: 'Красный', hex: '#f00' },
              ],
            }}
          >
            <ContextWatcher />
            <ProductStudioFormWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // 1. Verify drag handles exist and arrows do NOT exist
      expect(screen.getByTestId('form-media-drag-handle-0')).toBeTruthy();
      expect(screen.getByTestId('form-media-drag-handle-3')).toBeTruthy();
      expect(screen.queryByTestId('form-media-move-up-0')).toBeNull();
      expect(screen.queryByTestId('form-media-move-down-0')).toBeNull();

      // 2. Perform drag-and-drop: move image at index 3 (D) to index 1 (between A and B) -> A D B C
      const card3 = screen.getByTestId('form-media-card-3');
      const card1 = screen.getByTestId('form-media-card-1');

      fireEvent.dragStart(card3, { dataTransfer: { setData: () => {}, effectAllowed: 'move' } });
      fireEvent.dragOver(card1, { dataTransfer: { dropEffect: 'move' } });
      fireEvent.drop(card1);

      // 3. Verify reordered draft
      expect(currentDraft!.images).toHaveLength(4);
      const reordered = currentDraft!.images!;

      // Order: A, D, B, C
      expect(reordered[0].uiKey).toBe('img-A');
      expect(reordered[1].uiKey).toBe('img-D');
      expect(reordered[2].uiKey).toBe('img-B');
      expect(reordered[3].uiKey).toBe('img-C');

      // 4. Verify sortOrder normalization (0, 1, 2, 3) and isMain (only first is true)
      expect(reordered.map((img) => img.sortOrder)).toEqual([0, 1, 2, 3]);
      expect(reordered.map((img) => img.isMain)).toEqual([true, false, false, false]);

      // 5. Verify color bindings survive reorder
      expect(reordered[1].colorId).toBeNull(); // img-D
      expect(reordered[2].colorId).toBe('col-black'); // img-B
      expect(reordered[3].colorId).toBe('col-red'); // img-C

      // 6. Verify "Сделать обложкой" still works: make B (index 2) cover -> B A D C
      const makeCoverBtn = screen.getByTestId('form-media-make-cover-2');
      fireEvent.click(makeCoverBtn);

      const afterCover = currentDraft!.images!;
      expect(afterCover[0].uiKey).toBe('img-B');
      expect(afterCover[0].isMain).toBe(true);
      expect(afterCover[0].colorId).toBe('col-black'); // color binding intact
      expect(afterCover.map((img) => img.sortOrder)).toEqual([0, 1, 2, 3]);
    });
  });

  describe('PS.R4B2.4.2 — Restore color names in photo ↔ color binding modal', () => {
    it('renders display names for canonical colors with nameRu or name, photo counts and summary bar', () => {
      const canonicalColors = [
        { id: 'col-red-id', nameRu: 'Красный', hex: '#ff0000' },
        { id: 'col-white-id', name: 'Белый', hex: '#ffffff' },
      ];
      const testImages: import('../../contexts/ProductStudioContext').ProductStudioImage[] = [
        {
          uiKey: 'img-1',
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://example.com/1.jpg' },
          isMain: true,
          sortOrder: 0,
          colorId: 'col-red-id',
        },
        {
          uiKey: 'img-2',
          source: { kind: 'canonical', imageId: 'img-2', url: 'https://example.com/2.jpg' },
          isMain: false,
          sortOrder: 1,
          colorId: null,
        },
      ];

      const onSaveMock = vi.fn();

      render(
        <ProductStudioPhotoColorModal
          isOpen={true}
          onClose={() => {}}
          images={testImages}
          colors={canonicalColors as any}
          onSave={onSaveMock}
        />
      );

      // Top summary bar renders color display names
      const summaryBar = screen.getByTestId('photo-color-summary-bar');
      expect(summaryBar.textContent).toContain('Общие: 1');
      expect(summaryBar.textContent).toContain('Красный: 1');
      expect(summaryBar.textContent).toContain('Белый: 0');

      // Right column target buttons render display names and photo counts
      const redTarget = screen.getByTestId('photo-bind-target-col-red-id');
      const whiteTarget = screen.getByTestId('photo-bind-target-col-white-id');
      const generalTarget = screen.getByTestId('photo-bind-target-general');

      expect(redTarget.textContent).toContain('Красный');
      expect(redTarget.textContent).toContain('1 фото');

      expect(whiteTarget.textContent).toContain('Белый');
      expect(whiteTarget.textContent).toContain('0 фото');

      expect(generalTarget.textContent).toContain('Общее (все цвета)');
      expect(generalTarget.textContent).toContain('1 фото');

      // Left column photo card 0 badge shows "Красный"
      expect(screen.getByTestId('photo-assigned-badge-0').textContent).toContain('Красный');

      // Clicking white target for photo 0 updates binding strictly by colorId 'col-white-id'
      fireEvent.click(whiteTarget);
      fireEvent.click(screen.getByTestId('photo-color-modal-submit'));

      expect(onSaveMock).toHaveBeenCalledWith([
        expect.objectContaining({ uiKey: 'img-1', colorId: 'col-white-id' }),
        expect.objectContaining({ uiKey: 'img-2', colorId: null }),
      ]);
    });

    it('PS.R4B2.4.2A: getColorDisplayName strictly uses human-readable names and rejects technical code or id fallbacks', () => {
      // 1. nameRu renders
      expect(getColorDisplayName({ id: 'uuid-1', code: 'COLOR_RED', nameRu: 'Красный' })).toBe('Красный');
      // 2. name renders
      expect(getColorDisplayName({ id: 'uuid-2', code: 'COLOR_BLUE', name: 'Синий' })).toBe('Синий');
      // 3. colorName renders
      expect(getColorDisplayName({ id: 'uuid-3', code: 'COLOR_GREEN', colorName: 'Зелёный' })).toBe('Зелёный');
      // 4. code alone does NOT become visible name -> renders safe fallback "Цвет без названия"
      expect(getColorDisplayName({ id: 'uuid-4', code: 'COLOR_YELLOW' })).toBe('Цвет без названия');
      // 5. id alone does NOT become visible name -> renders safe fallback "Цвет без названия"
      expect(getColorDisplayName({ id: '550e8400-e29b-41d4-a716-446655440000' })).toBe('Цвет без названия');
      // 6. null or undefined -> renders safe fallback "Цвет без названия"
      expect(getColorDisplayName(null)).toBe('Цвет без названия');
    });
  });
});

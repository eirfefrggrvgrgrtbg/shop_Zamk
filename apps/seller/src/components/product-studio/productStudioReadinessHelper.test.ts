import { describe, it, expect } from 'vitest';
import {
  getProductStudioReadiness,
  getSizeChartCompleteness,
} from './productStudioReadinessHelper';
import type { ProductStudioDraft, ProductStudioImage } from '../../contexts/ProductStudioContext';
import type { SellerCategorySchema } from '@zamk/api-client/src/seller';
import { createCanonicalProductStudioImage } from './productStudioMediaHelper';

const mockImages: ProductStudioImage[] = [
  createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://images.unsplash.com/1.jpg', isMain: true }),
  createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://images.unsplash.com/2.jpg' }),
  createCanonicalProductStudioImage({ imageId: 'img-3', url: 'https://images.unsplash.com/3.jpg' }),
];

describe('productStudioReadinessHelper', () => {
  const emptyDraft: ProductStudioDraft = {
    title: '',
    description: '',
    categoryId: '',
  };

  it('clean draft returns exact canonical blockers', () => {
    const readiness = getProductStudioReadiness(emptyDraft);
    expect(readiness.blockingFields).toContain('title');
    expect(readiness.blockingFields).toContain('category');
    expect(readiness.blockingFields).toContain('price');
    expect(readiness.blockingFields).toContain('media');
    expect(readiness.blockingFields).toContain('description');
    expect(readiness.blockingFields).toContain('composition');
    // Dynamic fields not blockers on clean draft before category/schema
    expect(readiness.blockingFields).not.toContain('color');
    expect(readiness.blockingFields).not.toContain('size');
    expect(readiness.blockingFields).not.toContain('characteristics');
    expect(readiness.blockingFields).not.toContain('sizeChart');
    expect(readiness.isReadyForSave).toBe(false);
    expect(readiness.isReadyForModeration).toBe(false);
  });

  it('description blocker enforces required non-empty text', () => {
    const draft: ProductStudioDraft = {
      title: 'Футболка',
      description: '   ',
      categoryId: 'cat-1',
      priceCents: 1000,
      material: 'Хлопок',
      images: mockImages,
    };
    const r0 = getProductStudioReadiness(draft);
    expect(r0.blockingFields).toContain('description');
    expect(r0.fieldStatus.description.required).toBe(true);
    expect(r0.fieldStatus.description.isSatisfied).toBe(false);

    const r1 = getProductStudioReadiness({ ...draft, description: 'Отличная модель' });
    expect(r1.blockingFields).not.toContain('description');
    expect(r1.fieldStatus.description.isSatisfied).toBe(true);
  });

  it('composition blocker requires structured composition summing to 100% for create mode', () => {
    const draft: ProductStudioDraft = {
      title: 'Футболка',
      description: 'Отличная футболка',
      categoryId: 'cat-1',
      priceCents: 1000,
      images: mockImages,
    };
    const r0 = getProductStudioReadiness(draft);
    expect(r0.blockingFields).toContain('composition');
    expect(r0.fieldStatus.composition.required).toBe(true);
    expect(r0.fieldStatus.composition.isSatisfied).toBe(false);

    // Free text alone in create mode does NOT satisfy composition blocker
    const rLegacyCreate = getProductStudioReadiness({
      ...draft,
      material: '100% хлопок',
    });
    expect(rLegacyCreate.blockingFields).toContain('composition');
    expect(rLegacyCreate.fieldStatus.composition.isSatisfied).toBe(false);

    // Partial percentage (< 100) does NOT satisfy
    const rPartial = getProductStudioReadiness({
      ...draft,
      materialComposition: [{ materialName: 'Хлопок', percentage: 80 }],
    });
    expect(rPartial.blockingFields).toContain('composition');
    expect(rPartial.fieldStatus.composition.isSatisfied).toBe(false);

    // Overflow percentage (> 100) does NOT satisfy
    const rOverflow = getProductStudioReadiness({
      ...draft,
      materialComposition: [
        { materialName: 'Хлопок', percentage: 80 },
        { materialName: 'Полиэстер', percentage: 30 },
      ],
    });
    expect(rOverflow.blockingFields).toContain('composition');
    expect(rOverflow.fieldStatus.composition.isSatisfied).toBe(false);

    // Satisfied via structured materialComposition summing to 100%
    const r1 = getProductStudioReadiness({
      ...draft,
      materialComposition: [
        { materialName: 'Хлопок', percentage: 80 },
        { materialName: 'Полиэстер', percentage: 20 },
      ],
    });
    expect(r1.blockingFields).not.toContain('composition');
    expect(r1.fieldStatus.composition.isSatisfied).toBe(true);

    // Legacy edit mode with draft.id preserves backwards compatibility
    const rLegacyEdit = getProductStudioReadiness({
      ...draft,
      id: 'prod-legacy-123',
      material: '100% хлопок',
    });
    expect(rLegacyEdit.blockingFields).not.toContain('composition');
    expect(rLegacyEdit.fieldStatus.composition.isSatisfied).toBe(true);
  });

  it('characteristics blocker enforces required category product attributes', () => {
    const schemaWithReqAttr: SellerCategorySchema = {
      id: 'sch-dress',
      name: 'Платья',
      slug: 'dresses',
      dimensionType: 'SINGLE_VARIANT',
      sizeChartRequired: false,
      attributes: [
        {
          id: 'attr-season',
          code: 'SEASON',
          nameRu: 'Сезон',
          valueType: 'DICTIONARY',
          valueSource: 'DICTIONARY',
          scope: 'PRODUCT',
          required: true,
          filterable: true,
          variantAxis: false,
          sortOrder: 1,
        },
      ],
      allowedSizeSystems: [],
      sizeChartFields: [],
    };

    const draft: ProductStudioDraft = {
      title: 'Платье вечернее',
      description: 'Красивое платье',
      categoryId: 'cat-dress',
      priceCents: 500000,
      material: 'Шелк',
      images: mockImages,
      attributes: [],
    };

    const r0 = getProductStudioReadiness(draft, schemaWithReqAttr);
    expect(r0.blockingFields).toContain('characteristics');
    expect(r0.fieldStatus.characteristics.required).toBe(true);
    expect(r0.fieldStatus.characteristics.isSatisfied).toBe(false);

    const r1 = getProductStudioReadiness(
      {
        ...draft,
        attributes: [
          {
            attributeDefinitionId: 'attr-season',
            name: 'Сезон',
            value: 'Лето',
            dictionaryValueId: 'val-summer',
          },
        ],
      },
      schemaWithReqAttr
    );
    expect(r1.blockingFields).not.toContain('characteristics');
    expect(r1.fieldStatus.characteristics.isSatisfied).toBe(true);
  });

  const sampleSizeChartSchema: SellerCategorySchema = {
    id: 'sch-hoodie',
    name: 'Худи',
    slug: 'hoodies',
    dimensionType: 'COLOR_AND_SIZE',
    sizeChartRequired: true,
    attributes: [],
    allowedSizeSystems: [],
    sizeChartFields: [
      {
        code: 'chest',
        name: 'Обхват груди',
        unit: 'см',
        isRequired: true,
        sortOrder: 1,
      },
      {
        code: 'length',
        name: 'Длина изделия',
        unit: 'см',
        isRequired: true,
        sortOrder: 2,
      },
      {
        code: 'sleeve',
        name: 'Длина рукава',
        unit: 'см',
        isRequired: false,
        sortOrder: 3,
      },
    ],
  };

  it('sizeChart blocker activates only when category uses sizes, has sizeChartFields, AND sizes are offered', () => {
    const draft: ProductStudioDraft = {
      title: 'Худи оверсайз',
      description: 'Удобное худи',
      categoryId: 'cat-hoodie',
      priceCents: 350000,
      material: 'Хлопок',
      images: mockImages,
      colors: [{ id: 'col-1', name: 'Черный' }],
      variants: [],
    };

    // No sizes offered yet -> sizeChart not yet required
    const r0 = getProductStudioReadiness(draft, sampleSizeChartSchema);
    expect(r0.blockingFields).not.toContain('sizeChart');
    expect(r0.fieldStatus.sizeChart.required).toBe(false);

    // Sizes offered -> sizeChart is now required and blocking
    const draftWithSizes: ProductStudioDraft = {
      ...draft,
      variants: [{ id: 'v1', sizeValueId: 'sz-m', priceCents: 350000 }],
    };
    const r1 = getProductStudioReadiness(draftWithSizes, sampleSizeChartSchema);
    expect(r1.blockingFields).toContain('sizeChart');
    expect(r1.fieldStatus.sizeChart.required).toBe(true);
    expect(r1.fieldStatus.sizeChart.isSatisfied).toBe(false);

    // Size chart configured with valid positive measurements -> blocker clears
    const draftWithSizeChart: ProductStudioDraft = {
      ...draftWithSizes,
      sizeChart: {
        fields: sampleSizeChartSchema.sizeChartFields,
        rows: [{ sizeValueId: 'sz-m', size: 'M', measurements: { chest: 100, length: 70 } }],
      },
    };
    const r2 = getProductStudioReadiness(draftWithSizeChart, sampleSizeChartSchema);
    expect(r2.blockingFields).not.toContain('sizeChart');
    expect(r2.fieldStatus.sizeChart.isSatisfied).toBe(true);
  });

  const createTestDraft = (overrides?: Partial<ProductStudioDraft>): ProductStudioDraft => ({
    title: 'Тест',
    description: 'Описание',
    categoryId: 'cat-hoodie',
    ...overrides,
  });

  it('getSizeChartCompleteness: empty chart is NOT complete', () => {
    const draft = createTestDraft({
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: { rows: [] },
    });
    const c = getSizeChartCompleteness(draft, sampleSizeChartSchema);
    expect(c.isNeeded).toBe(true);
    expect(c.isComplete).toBe(false);
    expect(c.requiredCellCount).toBe(2); // chest & length for sz-s
    expect(c.filledRequiredCellCount).toBe(0);
    expect(c.missingCells).toHaveLength(2);
  });

  it('getSizeChartCompleteness: partial measurements are NOT complete', () => {
    const draft = createTestDraft({
      variants: [
        { id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 },
        { id: 'v2', sizeValueId: 'sz-m', size: 'M', priceCents: 1000 },
      ],
      sizeChart: {
        rows: [
          { sizeValueId: 'sz-s', measurements: { chest: 95, length: 68 } },
          { sizeValueId: 'sz-m', measurements: { chest: 100 } }, // missing length
        ],
      },
    });
    const c = getSizeChartCompleteness(draft, sampleSizeChartSchema);
    expect(c.isComplete).toBe(false);
    expect(c.requiredCellCount).toBe(4); // 2 sizes * 2 required fields
    expect(c.filledRequiredCellCount).toBe(3);
    expect(c.missingCells).toHaveLength(1);
    expect(c.missingCells[0].sizeValueId).toBe('sz-m');
    expect(c.missingCells[0].fieldCode).toBe('length');
  });

  it('getSizeChartCompleteness: zero or negative values or whitespace strings are NOT valid measurements', () => {
    const draft = createTestDraft({
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: {
        rows: [
          {
            sizeValueId: 'sz-s',
            measurements: {
              chest: 0, // 0 is invalid
              length: -15, // negative is invalid
            },
          },
        ],
      },
    });
    const c1 = getSizeChartCompleteness(draft, sampleSizeChartSchema);
    expect(c1.isComplete).toBe(false);
    expect(c1.filledRequiredCellCount).toBe(0);

    const draftWithStrings = createTestDraft({
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: {
        rows: [
          {
            sizeValueId: 'sz-s',
            measurements: {
              chest: '  ' as any, // whitespace is invalid
              length: '0' as any, // string 0 is invalid
            },
          },
        ],
      },
    });
    const c2 = getSizeChartCompleteness(draftWithStrings, sampleSizeChartSchema);
    expect(c2.isComplete).toBe(false);
    expect(c2.filledRequiredCellCount).toBe(0);
  });

  it('getSizeChartCompleteness: chart complete only when all offered sizes have all required measurements filled with positive numbers', () => {
    const draft = createTestDraft({
      variants: [
        { id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 },
        { id: 'v2', sizeValueId: 'sz-m', size: 'M', priceCents: 1000 },
      ],
      sizeChart: {
        rows: [
          { sizeValueId: 'sz-s', measurements: { chest: 95, length: '68.5' as any } },
          { sizeValueId: 'sz-m', measurements: { chest: 102, length: 72 } },
        ],
      },
    });
    const c = getSizeChartCompleteness(draft, sampleSizeChartSchema);
    expect(c.isComplete).toBe(true);
    expect(c.requiredCellCount).toBe(4);
    expect(c.filledRequiredCellCount).toBe(4);
    expect(c.missingCells).toHaveLength(0);
  });

  it('getSizeChartCompleteness: optional measurements do not block completeness', () => {
    const draft = createTestDraft({
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: {
        rows: [
          {
            sizeValueId: 'sz-s',
            measurements: {
              chest: 96,
              length: 70,
              // sleeve is optional (isRequired: false) and omitted here
            },
          },
        ],
      },
    });
    const c = getSizeChartCompleteness(draft, sampleSizeChartSchema);
    expect(c.isComplete).toBe(true);
    expect(c.requiredCellCount).toBe(2);
    expect(c.filledRequiredCellCount).toBe(2);
  });

  it('getSizeChartCompleteness: adding a size after completion makes chart incomplete again', () => {
    const baseDraft = createTestDraft({
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: {
        rows: [{ sizeValueId: 'sz-s', measurements: { chest: 95, length: 68 } }],
      },
    });
    const c1 = getSizeChartCompleteness(baseDraft, sampleSizeChartSchema);
    expect(c1.isComplete).toBe(true);

    // Add a new size 'L'
    const expandedDraft: ProductStudioDraft = {
      ...baseDraft,
      variants: [
        ...baseDraft.variants!,
        { id: 'v2', sizeValueId: 'sz-l', size: 'L', priceCents: 1000 },
      ],
    };
    const c2 = getSizeChartCompleteness(expandedDraft, sampleSizeChartSchema);
    expect(c2.isComplete).toBe(false);
    expect(c2.requiredCellCount).toBe(4);
    expect(c2.filledRequiredCellCount).toBe(2);
    expect(c2.missingCells.map((m) => m.sizeValueId)).toEqual(['sz-l', 'sz-l']);
  });

  it('getSizeChartCompleteness: removing a size ignores/removes orphan requirements', () => {
    const draftWithOrphanRow = createTestDraft({
      // Only size S is offered in variants
      variants: [{ id: 'v1', sizeValueId: 'sz-s', size: 'S', priceCents: 1000 }],
      sizeChart: {
        rows: [
          { sizeValueId: 'sz-s', measurements: { chest: 95, length: 68 } },
          // Orphan row for XL which is not offered in variants
          { sizeValueId: 'sz-xl', measurements: { chest: 0, length: 0 } },
        ],
      },
    });
    const c = getSizeChartCompleteness(draftWithOrphanRow, sampleSizeChartSchema);
    expect(c.isComplete).toBe(true);
    expect(c.requiredCellCount).toBe(2);
    expect(c.filledRequiredCellCount).toBe(2);
  });

  it('getSizeChartCompleteness: incompatible size identities do not satisfy new sizes', () => {
    const draftWithOldSizes = createTestDraft({
      // Seller switched to Russian size system (sizes are sz-ru-44, sz-ru-46)
      variants: [
        { id: 'v1', sizeValueId: 'sz-ru-44', size: '44', priceCents: 1000 },
        { id: 'v2', sizeValueId: 'sz-ru-46', size: '46', priceCents: 1000 },
      ],
      sizeChart: {
        // Rows still contain previous International sizes sz-int-s, sz-int-m
        rows: [
          { sizeValueId: 'sz-int-s', size: 'S', measurements: { chest: 95, length: 68 } },
          { sizeValueId: 'sz-int-m', size: 'M', measurements: { chest: 100, length: 70 } },
        ],
      },
    });
    const c = getSizeChartCompleteness(draftWithOldSizes, sampleSizeChartSchema);
    expect(c.isComplete).toBe(false);
    expect(c.requiredCellCount).toBe(4);
    expect(c.filledRequiredCellCount).toBe(0);
    expect(c.missingCells).toHaveLength(4);
  });

  it('dynamic schema COLOR_AND_SIZE requires both color and size as blocker', () => {
    const schema: SellerCategorySchema = {
      id: 'sch-jacket',
      name: 'Куртки',
      slug: 'jackets',
      dimensionType: 'COLOR_AND_SIZE',
      sizeChartRequired: false,
      attributes: [],
      allowedSizeSystems: [],
      sizeChartFields: [],
    };
    const draft: ProductStudioDraft = {
      title: 'Куртка зимняя',
      description: 'Теплая',
      categoryId: 'cat-jacket',
      priceCents: 1000000,
      material: 'Полиэстер',
      materialComposition: [{ materialName: 'Полиэстер', percentage: 100 }],
      images: mockImages,
      colors: [],
      variants: [],
    };

    const r0 = getProductStudioReadiness(draft, schema);
    expect(r0.blockingFields).toContain('color');
    expect(r0.blockingFields).toContain('size');

    // Add color -> color clears, size remains
    const r1 = getProductStudioReadiness({
      ...draft,
      colors: [{ id: 'col-black', name: 'Черный' }],
    }, schema);
    expect(r1.blockingFields).not.toContain('color');
    expect(r1.blockingFields).toContain('size');

    // Add size variant and sizeChart -> both clear
    const r2 = getProductStudioReadiness({
      ...draft,
      colors: [{ id: 'col-black', name: 'Черный' }],
      variants: [{ id: 'v1', sizeValueId: 'sz-m', priceCents: 1000000 }],
      sizeChart: { rows: [{ size: 'M', measurements: {} }] },
    }, schema);
    expect(r2.blockingFields).not.toContain('color');
    expect(r2.blockingFields).not.toContain('size');
    expect(r2.blockingFields).not.toContain('sizeChart');
    expect(r2.blockingFields).toHaveLength(0);
  });

  it('media blocker enforces MIN_PRODUCT_IMAGES=3 progression', () => {
    const draft: ProductStudioDraft = {
      title: 'Футболка оверсайз',
      description: 'Отличная футболка',
      categoryId: 'cat-1',
      priceCents: 150000,
      material: 'Хлопок',
      images: [],
      colors: [{ id: 'col-black', name: 'Черный', hex: '#000000' }],
      variants: [{ id: 'var-1', sizeValueId: 'sz-m', priceCents: 150000 }],
    };

    // 0 images -> media blocker
    const r0 = getProductStudioReadiness(draft);
    expect(r0.blockingFields).toContain('media');
    expect(r0.fieldStatus.media.isSatisfied).toBe(false);

    // 1 image -> still blocker
    const r1 = getProductStudioReadiness({
      ...draft,
      images: [mockImages[0]],
    });
    expect(r1.blockingFields).toContain('media');
    expect(r1.fieldStatus.media.isSatisfied).toBe(false);

    // 2 images -> still blocker
    const r2 = getProductStudioReadiness({
      ...draft,
      images: [mockImages[0], mockImages[1]],
    });
    expect(r2.blockingFields).toContain('media');
    expect(r2.fieldStatus.media.isSatisfied).toBe(false);

    // 3 images -> media blocker clears
    const r3 = getProductStudioReadiness({
      ...draft,
      images: mockImages,
    });
    expect(r3.blockingFields).not.toContain('media');
    expect(r3.fieldStatus.media.isSatisfied).toBe(true);
  });

  it('blockers clear as all fields are satisfied', () => {
    const draft: ProductStudioDraft = {
      title: 'Футболка оверсайз',
      description: 'Отличная футболка',
      categoryId: 'cat-1',
      priceCents: 150000,
      material: 'Хлопок',
      materialComposition: [{ materialName: 'Хлопок', percentage: 100 }],
      images: mockImages,
      colors: [{ id: 'col-black', name: 'Черный', hex: '#000000' }],
      variants: [{ id: 'var-1', sizeValueId: 'sz-m', priceCents: 150000 }],
    };

    const readiness = getProductStudioReadiness(draft);
    expect(readiness.blockingFields).toHaveLength(0);
    expect(readiness.fieldStatus.title.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.category.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.price.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.media.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.color.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.size.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.description.isSatisfied).toBe(true);
    expect(readiness.fieldStatus.composition.isSatisfied).toBe(true);
    expect(readiness.isReadyForSave).toBe(false);
    expect(readiness.isReadyForModeration).toBe(false);
  });

  it('dynamic schema COLOR_ONLY does not require size as blocker', () => {
    const schema: SellerCategorySchema = {
      id: 'sch-scarf',
      name: 'Шарфы',
      slug: 'scarves',
      dimensionType: 'COLOR_ONLY',
      sizeChartRequired: false,
      attributes: [],
      allowedSizeSystems: [],
      sizeChartFields: [],
    };

    const draft: ProductStudioDraft = {
      title: 'Шарф шерстяной',
      description: 'Теплый шарф',
      categoryId: 'cat-scarf',
      priceCents: 200000,
      material: 'Шерсть',
      materialComposition: [{ materialName: 'Шерсть', percentage: 100 }],
      images: mockImages,
      colors: [{ id: 'col-red', name: 'Красный', hex: '#ff0000' }],
      variants: [],
    };

    const readiness = getProductStudioReadiness(draft, schema);
    expect(readiness.blockingFields).not.toContain('size');
    expect(readiness.fieldStatus.size.required).toBe(false);
    expect(readiness.blockingFields).toHaveLength(0);
  });

  it('dynamic schema ONLY_SIZE does not require color as blocker', () => {
    const schema: SellerCategorySchema = {
      id: 'sch-ring',
      name: 'Кольца',
      slug: 'rings',
      dimensionType: 'ONLY_SIZE',
      sizeChartRequired: false,
      attributes: [],
      allowedSizeSystems: [{ id: 'sys-ru', code: 'RU', nameRu: 'RU' }],
      sizeChartFields: [],
    };

    const draft: ProductStudioDraft = {
      title: 'Кольцо серебряное',
      description: 'Серебро 925 пробы',
      categoryId: 'cat-ring',
      priceCents: 500000,
      material: 'Серебро',
      materialComposition: [{ materialName: 'Серебро', percentage: 100 }],
      images: mockImages,
      colors: [],
      variants: [{ id: 'var-1', sizeValueId: 'sz-17', priceCents: 500000 }],
      sizeChart: { rows: [{ size: '17', measurements: {} }] },
    };

    const readiness = getProductStudioReadiness(draft, schema);
    expect(readiness.blockingFields).not.toContain('color');
    expect(readiness.fieldStatus.color.required).toBe(false);
    expect(readiness.blockingFields).toHaveLength(0);
  });

  it('dynamic schema SINGLE_VARIANT requires neither color nor size as blocker', () => {
    const schema: SellerCategorySchema = {
      id: 'sch-umbrella',
      name: 'Зонты',
      slug: 'umbrellas',
      dimensionType: 'SINGLE_VARIANT',
      sizeChartRequired: false,
      attributes: [],
      allowedSizeSystems: [],
      sizeChartFields: [],
    };

    const draft: ProductStudioDraft = {
      title: 'Зонт складной',
      description: 'Компактный зонт',
      categoryId: 'cat-umbrella',
      priceCents: 300000,
      material: 'Полиэстер',
      materialComposition: [{ materialName: 'Полиэстер', percentage: 100 }],
      images: mockImages,
      colors: [],
      variants: [],
    };

    const readiness = getProductStudioReadiness(draft, schema);
    expect(readiness.blockingFields).not.toContain('color');
    expect(readiness.blockingFields).not.toContain('size');
    expect(readiness.fieldStatus.color.required).toBe(false);
    expect(readiness.fieldStatus.size.required).toBe(false);
    expect(readiness.blockingFields).toHaveLength(0);
  });
});

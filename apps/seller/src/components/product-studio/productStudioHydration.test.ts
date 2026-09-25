import { describe, it, expect } from 'vitest';
import {
  hydrateProductStudioDraft,
  resolveSizeSystemForProduct,
} from './productStudioHydration';
import type {
  SellerProduct,
  SellerCategorySchema,
  SellerColor,
  SellerSizeValue,
} from '@zamk/api-client';

describe('productStudioHydration mapper tests', () => {
  const mockColors: SellerColor[] = [
    { id: 'col-red', code: 'RED', nameRu: 'Красный', hex: '#FF0000' },
    { id: 'col-white', code: 'WHITE', nameRu: 'Белый', hex: '#FFFFFF' },
    { id: 'col-black', code: 'BLACK', nameRu: 'Черный', hex: '#000000' },
  ];

  const mockSchema: SellerCategorySchema = {
    id: 'cat-hoodies',
    name: 'Худи',
    slug: 'hoodies',
    dimensionType: 'COLOR_AND_SIZE',
    sizeChartRequired: true,
    allowedSizeSystems: [
      { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
      { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
    ],
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
      {
        id: 'attr-fit',
        code: 'FIT',
        nameRu: 'Покрой',
        valueType: 'TEXT',
        valueSource: 'FREE_TEXT',
        scope: 'PRODUCT',
        required: false,
        filterable: false,
        variantAxis: false,
        sortOrder: 2,
      },
    ],
    sizeChartFields: [
      { code: 'CHEST', name: 'Обхват груди', unit: 'cm', isRequired: true, sortOrder: 1 },
      { code: 'LENGTH', name: 'Длина', unit: 'cm', isRequired: true, sortOrder: 2 },
    ],
  };

  const sampleProduct: SellerProduct = {
    id: 'prod-123',
    title: 'худи оверсайз',
    description: 'Теплое худи из плотного футера.',
    categoryId: 'cat-hoodies',
    categoryName: 'Худи',
    brandId: 'brand-456',
    brandName: 'Dev Brand',
    sellerId: 'seller-789',
    sellerSlug: 'dev-seller',
    sellerName: 'ZAMK Dev Seller', // Must NOT overwrite brandName!
    createdAt: '2026-08-25T17:00:00Z',
    status: 'pending_moderation',
    priceCents: 122200,
    oldPriceCents: 150000,
    currency: 'RUB',
    slug: 'hoodie-oversize',
    careInstructions: 'Стирка при 30 градусах.',
    gender: 'unisex',
    images: [
      {
        id: 'img-1',
        imageUrl: 'https://cdn.example.com/img1.jpg',
        sortOrder: 0,
        colorId: 'col-red',
        isMain: true,
      },
      {
        id: 'img-2',
        imageUrl: 'https://cdn.example.com/img2.jpg',
        sortOrder: 1,
        colorId: 'col-white',
        isMain: false,
      },
      {
        id: 'img-3',
        imageUrl: 'https://cdn.example.com/img3.jpg',
        sortOrder: 1, // Same sort order as img-2: test deterministic tie-breaking
        colorId: null,
        isMain: false,
      },
    ],
    variants: [
      {
        id: 'var-red-l',
        productId: 'prod-123',
        colorId: 'col-red',
        colorName: 'Красный',
        colorHex: '#FF0000',
        sizeValueId: 'sz-l',
        size: 'L',
        sellerSku: 'SKU-RED-L',
        barcode: 'ZMK-BAR-001',
        priceCents: 122200,
        isActive: true,
      },
      {
        id: 'var-white-l',
        productId: 'prod-123',
        colorId: 'col-white',
        colorName: 'Белый',
        colorHex: '#FFFFFF',
        sizeValueId: 'sz-l',
        size: 'L',
        sellerSku: 'SKU-WHT-L',
        barcode: 'ZMK-BAR-002',
        priceCents: 122200,
        isActive: true,
      },
    ],
    materialComposition: [
      {
        materialId: 'mat-cotton',
        materialName: 'Хлопок',
        percentage: 100,
      },
    ],
    sizeChart: {
      id: 'sc-1',
      productId: 'prod-123',
      categoryId: 'cat-hoodies',
      rows: [
        {
          sizeChartId: 'sc-1',
          sizeValueId: 'sz-l',
          sizeValueName: 'L',
          measurements: { CHEST: 120, LENGTH: 75 },
        },
      ],
    },
    attributes: [
      {
        id: 'attr-val-1',
        productId: 'prod-123',
        attributeDefinitionId: 'attr-season',
        enumValueId: 'dict-val-winter',
      },
      {
        id: 'attr-val-2',
        productId: 'prod-123',
        attributeDefinitionId: 'attr-fit',
        textValue: 'Oversize',
      },
    ],
  };

  const mockDictMap = {
    'dict-season': [
      { id: 'dict-val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
      { id: 'dict-val-summer', dictionaryId: 'dict-season', code: 'SUMMER', nameRu: 'Лето' },
    ],
  };

  it('hydrates basic fields, identity, and strictly preserves product brand over sellerName', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(draft.id).toBe('prod-123');
    expect(draft.title).toBe('худи оверсайз');
    expect(draft.description).toBe('Теплое худи из плотного футера.');
    expect(draft.categoryId).toBe('cat-hoodies');
    expect(draft.categoryName).toBe('Худи');

    // HARD RULE: brandName must be Dev Brand, NOT sellerName (ZAMK Dev Seller)
    expect(draft.brandId).toBe('brand-456');
    expect(draft.brandName).toBe('Dev Brand');

    expect(draft.status).toBe('pending_moderation');
    expect(draft.priceCents).toBe(122200);
    expect(draft.oldPriceCents).toBe(150000);
    expect(draft.careInstructions).toBe('Стирка при 30 градусах.');
    expect(draft.gender).toBe('unisex');
    expect(draft.dimensionType).toBe('COLOR_AND_SIZE');
  });

  it('hydrates media preserving sort order and original index tie-breaker', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(draft.images).toHaveLength(3);
    expect(draft.images![0]).toEqual({
      uiKey: 'img-1',
      sortOrder: 0,
      colorId: 'col-red',
      isMain: true,
      altText: null,
      source: {
        kind: 'canonical',
        imageId: 'img-1',
        url: 'https://cdn.example.com/img1.jpg',
      },
    });
    expect(draft.images![1]).toEqual({
      uiKey: 'img-2',
      sortOrder: 1,
      colorId: 'col-white',
      isMain: false,
      altText: null,
      source: {
        kind: 'canonical',
        imageId: 'img-2',
        url: 'https://cdn.example.com/img2.jpg',
      },
    });
    expect(draft.images![2]).toEqual({
      uiKey: 'img-3',
      sortOrder: 1,
      colorId: null,
      isMain: false,
      altText: null,
      source: {
        kind: 'canonical',
        imageId: 'img-3',
        url: 'https://cdn.example.com/img3.jpg',
      },
    });
  });

  it('hydrates colors using canonical IDs only and throws on unknown colorId', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    // Contains col-red and col-white
    expect(draft.colors).toHaveLength(2);
    expect(draft.colors!.find((c: any) => c.id === 'col-red')).toEqual({
      id: 'col-red',
      nameRu: 'Красный',
      hex: '#FF0000',
      code: 'RED',
    });
    expect(draft.colors!.find((c: any) => c.id === 'col-white')).toEqual({
      id: 'col-white',
      nameRu: 'Белый',
      hex: '#FFFFFF',
      code: 'WHITE',
    });

    // Unknown colorId throws data integrity error
    const brokenProduct: SellerProduct = {
      ...sampleProduct,
      variants: [
        {
          ...sampleProduct.variants![0],
          colorId: 'unknown-color-id',
        },
      ],
    };

    expect(() =>
      hydrateProductStudioDraft({
        product: brokenProduct,
        categorySchema: mockSchema,
        canonicalColors: mockColors,
        dictionaryValuesMap: mockDictMap,
      })
    ).toThrowError(/references unknown colorId "unknown-color-id"/);
  });

  it('strictly preserves exact variant IDs without regenerating them', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(draft.variants).toHaveLength(2);
    expect(draft.variants![0].id).toBe('var-red-l');
    expect(draft.variants![0].sellerSku).toBe('SKU-RED-L');
    expect(draft.variants![0].barcode).toBe('ZMK-BAR-001');
    expect(draft.variants![0].sizeValueId).toBe('sz-l');

    expect(draft.variants![1].id).toBe('var-white-l');
    expect(draft.variants![1].sellerSku).toBe('SKU-WHT-L');
    expect(draft.variants![1].barcode).toBe('ZMK-BAR-002');
    expect(draft.variants![1].sizeValueId).toBe('sz-l');
  });

  it('hydrates active variants only: ignores inactive variants and does not resurrect their colors', () => {
    const testColors: SellerColor[] = [
      { id: 'col-black', code: 'BLACK', nameRu: 'Черный', hex: '#000000' },
      { id: 'col-white', code: 'WHITE', nameRu: 'Белый', hex: '#FFFFFF' },
      { id: 'col-beige', code: 'BEIGE', nameRu: 'Бежевый', hex: '#F5F5DC' },
    ];

    const productWithHistory: SellerProduct = {
      ...sampleProduct,
      images: [],
      variants: [
        {
          id: 'var-black-m',
          productId: 'prod-123',
          colorId: 'col-black',
          sizeValueId: 'sz-m',
          size: 'M',
          isActive: true,
          priceCents: 122200,
        },
        {
          id: 'var-white-m-inactive',
          productId: 'prod-123',
          colorId: 'col-white',
          sizeValueId: 'sz-m',
          size: 'M',
          isActive: false,
          priceCents: 99900,
        },
        {
          id: 'var-beige-m-inactive',
          productId: 'prod-123',
          colorId: 'col-beige',
          sizeValueId: 'sz-m',
          size: 'M',
          isActive: false,
          priceCents: 88800,
        },
      ],
    };

    const draft = hydrateProductStudioDraft({
      product: productWithHistory,
      categorySchema: mockSchema,
      canonicalColors: testColors,
      dictionaryValuesMap: mockDictMap,
    });

    // Expected hydrated ProductStudioDraft:
    // colors: Black only (No White, No Beige)
    expect(draft.colors).toHaveLength(1);
    expect(draft.colors?.[0]?.id).toBe('col-black');
    expect(draft.colors?.some((c: any) => c.id === 'col-white')).toBe(false);
    expect(draft.colors?.some((c: any) => c.id === 'col-beige')).toBe(false);

    // variants: Black/M only (active only)
    expect(draft.variants).toHaveLength(1);
    expect(draft.variants?.[0]?.id).toBe('var-black-m');
    expect(draft.variants?.[0]?.colorId).toBe('col-black');
    expect(draft.variants?.[0]?.sizeValueId).toBe('sz-m');
    expect(draft.variants?.[0]?.isActive).toBe(true);

    // Inactive variants do not contribute to variants or colors
    expect(draft.variants?.some((v) => v.id === 'var-white-m-inactive')).toBe(false);
    expect(draft.variants?.some((v) => v.id === 'var-beige-m-inactive')).toBe(false);
  });

  it('hydrates structured material composition and preserves legacy fallback if empty', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(draft.materialComposition).toEqual([
      {
        materialId: 'mat-cotton',
        materialName: 'Хлопок',
        percentage: 100,
      },
    ]);

    // Legacy product fallback
    const legacyProduct: SellerProduct = {
      ...sampleProduct,
      materialComposition: [],
      material: '100% Лен',
    };

    const legacyDraft = hydrateProductStudioDraft({
      product: legacyProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(legacyDraft.materialComposition).toEqual([]);
    expect(legacyDraft.material).toBe('100% Лен');
  });

  it('hydrates size chart with exact rows and measurements', () => {
    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: mockDictMap,
    });

    expect(draft.sizeChart).toEqual({
      id: 'sc-1',
      categoryId: 'cat-hoodies',
      rows: [
        {
          sizeChartId: 'sc-1',
          sizeValueId: 'sz-l',
          sizeValueName: 'L',
          measurements: { CHEST: 120, LENGTH: 75 },
        },
      ],
    });
  });

  it('hydrates attributes with dictionaryValueId preserved and enriched from dictionaries map', () => {
    const dictMap = {
      'dict-season': [
        { id: 'dict-val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
      ],
    };

    const draft = hydrateProductStudioDraft({
      product: sampleProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
      dictionaryValuesMap: dictMap,
    });

    expect(draft.attributes).toHaveLength(2);
    const seasonAttr = draft.attributes!.find((a) => a.attributeDefinitionId === 'attr-season');
    expect(seasonAttr).toEqual({
      attributeDefinitionId: 'attr-season',
      code: 'SEASON',
      name: 'Сезон',
      dictionaryValueId: 'dict-val-winter',
      value: 'Зима',
    });

    const fitAttr = draft.attributes!.find((a) => a.attributeDefinitionId === 'attr-fit');
    expect(fitAttr).toEqual({
      attributeDefinitionId: 'attr-fit',
      code: 'FIT',
      name: 'Покрой',
      dictionaryValueId: undefined,
      value: 'Oversize',
    });
  });

  it('G. throws data integrity error when dictionary data is missing for enum attribute', () => {
    const productWithEnum: SellerProduct = {
      ...sampleProduct,
      attributes: [
        {
          id: 'attr-1',
          productId: 'prod-hoodie-1',
          attributeDefinitionId: 'attr-season',
          enumValueId: 'dict-val-winter',
        },
      ],
    };

    expect(() =>
      hydrateProductStudioDraft({
        product: productWithEnum,
        categorySchema: mockSchema,
        canonicalColors: mockColors,
        dictionaryValuesMap: {}, // missing dict-season
      })
    ).toThrowError(/Data integrity error: missing canonical dictionary data/);
  });

  it('H. throws data integrity error when enumValueId is not found in canonical dictionary', () => {
    const productWithUnknownEnum: SellerProduct = {
      ...sampleProduct,
      attributes: [
        {
          id: 'attr-1',
          productId: 'prod-hoodie-1',
          attributeDefinitionId: 'attr-season',
          enumValueId: 'dict-val-alien',
        },
      ],
    };

    expect(() =>
      hydrateProductStudioDraft({
        product: productWithUnknownEnum,
        categorySchema: mockSchema,
        canonicalColors: mockColors,
        dictionaryValuesMap: {
          'dict-season': [
            { id: 'dict-val-winter', dictionaryId: 'dict-season', code: 'WINTER', nameRu: 'Зима' },
          ],
        },
      })
    ).toThrowError(/Data integrity error: product references unknown enumValueId "dict-val-alien"/);
  });

  it('I. throws data integrity error when attributeDefinitionId is not found in category schema', () => {
    const productWithUnknownDef: SellerProduct = {
      ...sampleProduct,
      attributes: [
        {
          id: 'attr-unknown',
          productId: 'prod-hoodie-1',
          attributeDefinitionId: 'attr-nonexistent-999',
          textValue: 'Some Value',
        },
      ],
    };

    expect(() =>
      hydrateProductStudioDraft({
        product: productWithUnknownDef,
        categorySchema: mockSchema,
        canonicalColors: mockColors,
      })
    ).toThrowError(/Data integrity error: product references attributeDefinitionId "attr-nonexistent-999" not found in category schema/);
  });

  it('PS.R4B3.1C4C3B3B-R2: Save + hydration sparse round trip preserves sparse matrix and never resurrects inactive/absent variants', () => {
    const rawProduct: SellerProduct = {
      id: 'prod-sparse-1',
      title: 'Худи Оверсайз',
      categoryId: 'cat-hoodies',
      priceCents: 450000,
      currency: 'RUB',
      status: 'draft',
      createdAt: '2026-09-20T10:00:00Z',
      updatedAt: '2026-09-20T10:00:00Z',
      variants: [
        {
          id: 'v-black-m',
          productId: 'prod-sparse-1',
          colorId: 'col-black',
          colorName: 'Черный',
          sizeValueId: 'sz-m',
          size: 'M',
          isActive: true,
        },
        {
          id: 'v-black-l',
          productId: 'prod-sparse-1',
          colorId: 'col-black',
          colorName: 'Черный',
          sizeValueId: 'sz-l',
          size: 'L',
          isActive: true,
        },
        {
          id: 'v-white-l',
          productId: 'prod-sparse-1',
          colorId: 'col-white',
          colorName: 'Белый',
          sizeValueId: 'sz-l',
          size: 'L',
          isActive: true,
        },
        // Inactive historical variant: White/M was deactivated and marked inactive: false
        {
          id: 'v-white-m-dead',
          productId: 'prod-sparse-1',
          colorId: 'col-white',
          colorName: 'Белый',
          sizeValueId: 'sz-m',
          size: 'M',
          isActive: false,
        },
      ],
    } as any;

    const draft = hydrateProductStudioDraft({
      product: rawProduct,
      categorySchema: mockSchema,
      canonicalColors: mockColors,
    });

    // Active variants must strictly be Black/M, Black/L, White/L (3 total)
    expect(draft.variants).toBeDefined();
    expect(draft.variants!).toHaveLength(3);
    expect(draft.variants!.every((v) => v.isActive !== false)).toBe(true);

    const activeKeys = draft.variants!.map((v) => `${v.colorId}:${v.sizeValueId}`);
    expect(activeKeys).toContain('col-black:sz-m');
    expect(activeKeys).toContain('col-black:sz-l');
    expect(activeKeys).toContain('col-white:sz-l');

    // Dead / inactive White/M is NOT resurrected
    expect(activeKeys).not.toContain('col-white:sz-m');
  });
});

describe('resolveSizeSystemForProduct', () => {
  const schemaWithSystems: SellerCategorySchema = {
    id: 'cat-1',
    name: 'Одежда',
    slug: 'clothing',
    sizeChartRequired: true,
    allowedSizeSystems: [
      { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
      { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
    ],
    attributes: [],
    sizeChartFields: [],
  };

  const mockFetchSizeValues = async (systemId: string): Promise<SellerSizeValue[]> => {
    if (systemId === 'sys-int') {
      return [
        { id: 'sz-s', sizeSystemId: 'sys-int', value: 'S', sortOrder: 1 },
        { id: 'sz-m', sizeSystemId: 'sys-int', value: 'M', sortOrder: 2 },
        { id: 'sz-l', sizeSystemId: 'sys-int', value: 'L', sortOrder: 3 },
      ];
    }
    if (systemId === 'sys-ru') {
      return [
        { id: 'sz-44', sizeSystemId: 'sys-ru', value: '44', sortOrder: 1 },
        { id: 'sz-46', sizeSystemId: 'sys-ru', value: '46', sortOrder: 2 },
        { id: 'sz-48', sizeSystemId: 'sys-ru', value: '48', sortOrder: 3 },
      ];
    }
    return [];
  };

  it('A. resolves RU size system when values match RU even if INT is schema default', async () => {
    const res = await resolveSizeSystemForProduct(
      ['sz-44', 'sz-48'],
      schemaWithSystems,
      mockFetchSizeValues
    );
    expect(res.systemId).toBe('sys-ru');
    expect(res.error).toBeUndefined();
  });

  it('resolves INT size system when values match INT', async () => {
    const res = await resolveSizeSystemForProduct(
      ['sz-m', 'sz-l'],
      schemaWithSystems,
      mockFetchSizeValues
    );
    expect(res.systemId).toBe('sys-int');
    expect(res.error).toBeUndefined();
  });

  it('B. returns explicit ambiguity error when multiple systems contain all size values', async () => {
    const ambiguousFetch = async (sysId: string): Promise<SellerSizeValue[]> => {
      // Both sys-int and sys-ru contain sz-universal
      return [
        { id: 'sz-universal', sizeSystemId: sysId, value: 'ONE-SIZE', sortOrder: 1 },
      ];
    };

    const res = await resolveSizeSystemForProduct(
      ['sz-universal'],
      schemaWithSystems,
      ambiguousFetch
    );
    expect(res.systemId).toBeNull();
    expect(res.error).toBe('Размерная система товара определяется неоднозначно');
  });

  it('C. returns hydration error when any allowed-system fetch fails', async () => {
    const failingFetch = async (sysId: string): Promise<SellerSizeValue[]> => {
      if (sysId === 'sys-ru') {
        throw new Error('Network timeout for RU sizes');
      }
      return [
        { id: 'sz-m', sizeSystemId: 'sys-int', value: 'M', sortOrder: 2 },
      ];
    };

    const res = await resolveSizeSystemForProduct(
      ['sz-m'],
      schemaWithSystems,
      failingFetch
    );
    expect(res.systemId).toBeNull();
    expect(res.error).toMatch(/Data integrity error: failed to fetch canonical size values for allowed system/);
  });

  it('returns data integrity error when size values belong to no allowed system', async () => {
    const res = await resolveSizeSystemForProduct(
      ['sz-unknown-99'],
      schemaWithSystems,
      mockFetchSizeValues
    );
    expect(res.systemId).toBeNull();
    expect(res.error).toMatch(/Data integrity error: product size values/);
  });

  it('D. returns default schema system when product has 0 size values and exactly one default exists', async () => {
    const res = await resolveSizeSystemForProduct(
      [],
      schemaWithSystems,
      mockFetchSizeValues
    );
    expect(res.systemId).toBe('sys-int');
    expect(res.error).toBeUndefined();
  });

  it('E. returns null systemId when product has 0 size values and no default system exists', async () => {
    const schemaNoDefault: SellerCategorySchema = {
      ...schemaWithSystems,
      allowedSizeSystems: [
        { id: 'sys-int', code: 'INT', name: 'International', isDefault: false },
        { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: false },
      ],
    };

    const res = await resolveSizeSystemForProduct(
      [],
      schemaNoDefault,
      mockFetchSizeValues
    );
    expect(res.systemId).toBeNull();
    expect(res.error).toBeUndefined();
  });

  it('returns null systemId when product has 0 size values and multiple default systems exist', async () => {
    const schemaMultiDefault: SellerCategorySchema = {
      ...schemaWithSystems,
      allowedSizeSystems: [
        { id: 'sys-int', code: 'INT', name: 'International', isDefault: true },
        { id: 'sys-ru', code: 'RU', name: 'Russian', isDefault: true },
      ],
    };

    const res = await resolveSizeSystemForProduct(
      [],
      schemaMultiDefault,
      mockFetchSizeValues
    );
    expect(res.systemId).toBeNull();
    expect(res.error).toBeUndefined();
  });
});

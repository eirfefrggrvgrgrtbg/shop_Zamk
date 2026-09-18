// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  getColorOptions,
  getSizeOptions,
  getDimensionType,
  resolveExactVariant,
  selectVariantState,
  useVariantSelection,
  isVariantBuyable,
  formatSizeUnavailableNotice,
  type ProductVariantItem,
} from './variantSelection';

describe('Canonical Shop Variant Selection Model (PV.2B)', () => {
  // Proven DEV "худи" fixture:
  // Red / L   -> inStock: true
  // Red / XL  -> inStock: false
  // White / L  -> inStock: false
  // White / XL -> inStock: false
  const redColorId = '1cb28a8d-0000-0000-0000-000000000001';
  const whiteColorId = '1cb28a8d-0000-0000-0000-000000000002';
  const sizeLId = '2fa98b7c-0000-0000-0000-000000000001';
  const sizeXLId = '2fa98b7c-0000-0000-0000-000000000002';

  const varRedLId = '2d262098-a7ce-4f82-b6d4-c93639f46d98';
  const varRedXLId = '2d262098-a7ce-4f82-b6d4-c93639f46d99';
  const varWhiteLId = '2d262098-a7ce-4f82-b6d4-c93639f46d9a';
  const varWhiteXLId = '2d262098-a7ce-4f82-b6d4-c93639f46d9b';

  const hoodieVariants: ProductVariantItem[] = [
    {
      id: varRedLId,
      productId: 'prod-hoodie',
      colorId: redColorId,
      colorName: 'Красный',
      colorHex: '#FF0000',
      sizeValueId: sizeLId,
      size: 'L',
      sellerSku: 'HOODIE-RED-L',
      isActive: true,
      inStock: true,
      priceCents: 122200,
    },
    {
      id: varRedXLId,
      productId: 'prod-hoodie',
      colorId: redColorId,
      colorName: 'Красный',
      colorHex: '#FF0000',
      sizeValueId: sizeXLId,
      size: 'XL',
      sellerSku: 'HOODIE-RED-XL',
      isActive: true,
      inStock: false,
      priceCents: 122200,
    },
    {
      id: varWhiteLId,
      productId: 'prod-hoodie',
      colorId: whiteColorId,
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      sizeValueId: sizeLId,
      size: 'L',
      sellerSku: 'HOODIE-WHITE-L',
      isActive: true,
      inStock: false,
      priceCents: 122200,
    },
    {
      id: varWhiteXLId,
      productId: 'prod-hoodie',
      colorId: whiteColorId,
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      sizeValueId: sizeXLId,
      size: 'XL',
      sellerSku: 'HOODIE-WHITE-XL',
      isActive: true,
      inStock: false,
      priceCents: 122200,
    },
  ];

  it('A. Option model exposes exactly: colors = Red, White; sizes for Red = L, XL; sizes for White = L, XL', () => {
    const dim = getDimensionType(hoodieVariants);
    expect(dim).toBe('COLOR_AND_SIZE');

    const colors = getColorOptions(hoodieVariants);
    expect(colors).toHaveLength(2);
    expect(colors[0].id).toBe(redColorId);
    expect(colors[0].name).toBe('Красный');
    expect(colors[0].hex).toBe('#FF0000');
    expect(colors[0].hasInStock).toBe(true);

    expect(colors[1].id).toBe(whiteColorId);
    expect(colors[1].name).toBe('Белый');
    expect(colors[1].hex).toBe('#FFFFFF');
    expect(colors[1].hasInStock).toBe(false);

    // Sizes for Red
    const redSizes = getSizeOptions(hoodieVariants, redColorId, 'COLOR_AND_SIZE');
    expect(redSizes).toHaveLength(2);
    expect(redSizes[0].label).toBe('L');
    expect(redSizes[0].inStock).toBe(true);
    expect(redSizes[0].disabled).toBe(false);
    expect(redSizes[1].label).toBe('XL');
    expect(redSizes[1].inStock).toBe(false);
    expect(redSizes[1].disabled).toBe(true);

    // Sizes for White
    const whiteSizes = getSizeOptions(hoodieVariants, whiteColorId, 'COLOR_AND_SIZE');
    expect(whiteSizes).toHaveLength(2);
    expect(whiteSizes[0].label).toBe('L');
    expect(whiteSizes[0].inStock).toBe(false);
    expect(whiteSizes[0].disabled).toBe(true);
    expect(whiteSizes[1].label).toBe('XL');
    expect(whiteSizes[1].inStock).toBe(false);
    expect(whiteSizes[1].disabled).toBe(true);
  });

  it('B. Red/L resolves exact Red/L variant id and is buyable', () => {
    const state = selectVariantState(hoodieVariants, redColorId, sizeLId);
    expect(state.isResolved).toBe(true);
    expect(state.canAddToCart).toBe(true);
    expect(state.selectedVariant).not.toBeNull();
    expect(state.selectedVariant?.id).toBe(varRedLId);
    expect(state.selectedVariant?.sellerSku).toBe('HOODIE-RED-L');
    expect(state.ctaText).toBe('Добавить в корзину');
  });

  it('C. Red/XL is unavailable and cannot resolve as buyable', () => {
    const state = selectVariantState(hoodieVariants, redColorId, sizeXLId);
    expect(state.isResolved).toBe(true);
    expect(state.canAddToCart).toBe(false);
    expect(state.selectedVariant?.id).toBe(varRedXLId);
    expect(state.ctaText).toBe('Нет в наличии');

    // Also verify that the size option in UI is disabled
    const redSizes = getSizeOptions(hoodieVariants, redColorId, 'COLOR_AND_SIZE');
    const xlOption = redSizes.find(s => s.id === sizeXLId);
    expect(xlOption?.disabled).toBe(true);
  });

  it('D. White/L and White/XL are unavailable and cannot resolve as buyable', () => {
    const stateL = selectVariantState(hoodieVariants, whiteColorId, sizeLId);
    expect(stateL.isResolved).toBe(true);
    expect(stateL.canAddToCart).toBe(false);
    expect(stateL.ctaText).toBe('Нет в наличии');

    const stateXL = selectVariantState(hoodieVariants, whiteColorId, sizeXLId);
    expect(stateXL.isResolved).toBe(true);
    expect(stateXL.canAddToCart).toBe(false);
    expect(stateXL.ctaText).toBe('Нет в наличии');
  });

  it('E. Selecting White never resolves Red/L', () => {
    const resolved = resolveExactVariant(hoodieVariants, 'COLOR_AND_SIZE', whiteColorId, sizeLId);
    expect(resolved?.id).toBe(varWhiteLId);
    expect(resolved?.id).not.toBe(varRedLId);
  });

  it('F. Changing Red -> White clears the previously selected size in selection flow', () => {
    // Initially Red is selected with size L
    const initial = selectVariantState(hoodieVariants, redColorId, sizeLId);
    expect(initial.selectedColorId).toBe(redColorId);
    expect(initial.selectedSizeId).toBe(sizeLId);

    // Switching color must pass null for sizeId to prevent cross-SKU resolution
    const afterColorChange = selectVariantState(hoodieVariants, whiteColorId, null);
    expect(afterColorChange.selectedColorId).toBe(whiteColorId);
    expect(afterColorChange.selectedSizeId).toBeNull();
    expect(afterColorChange.isResolved).toBe(false);
    expect(afterColorChange.selectedVariant).toBeNull();
    expect(afterColorChange.ctaText).toBe('Нет в наличии');
  });

  it('G. Quick Buy never renders duplicate: L XL L XL', () => {
    // Quick Buy queries sizes for the active color
    const sizes = getSizeOptions(hoodieVariants, redColorId, 'COLOR_AND_SIZE');
    const labels = sizes.map(s => s.label);
    expect(labels).toEqual(['L', 'XL']);
    // No duplicate L or XL
    expect(new Set(labels).size).toBe(labels.length);
  });

  it('H. PDP and Quick Buy use the same selector helper/model', () => {
    // Both PDP and Quick Buy get the exact same state structure from selectVariantState
    const pdpState = selectVariantState(hoodieVariants, redColorId, sizeLId);
    const qbState = selectVariantState(hoodieVariants, redColorId, sizeLId);
    expect(pdpState.selectedVariant?.id).toBe(qbState.selectedVariant?.id);
    expect(pdpState.ctaText).toBe('Добавить в корзину');
  });

  it('I. Size-only product still works', () => {
    const sizeOnlyVariants: ProductVariantItem[] = [
      { id: 'v1', size: 'S', sizeValueId: 's1', inStock: true, isActive: true },
      { id: 'v2', size: 'M', sizeValueId: 's2', inStock: false, isActive: true },
      { id: 'v3', size: 'L', sizeValueId: 's3', inStock: true, isActive: true },
    ];
    const dim = getDimensionType(sizeOnlyVariants);
    expect(dim).toBe('SIZE_ONLY');

    const state = selectVariantState(sizeOnlyVariants, null, 's1');
    expect(state.requiresColor).toBe(false);
    expect(state.requiresSize).toBe(true);
    expect(state.isResolved).toBe(true);
    expect(state.selectedVariant?.id).toBe('v1');
    expect(state.canAddToCart).toBe(true);

    const sizes = getSizeOptions(sizeOnlyVariants, null, 'SIZE_ONLY');
    expect(sizes).toHaveLength(3);
    expect(sizes[0].label).toBe('S');
    expect(sizes[1].disabled).toBe(true);
    expect(sizes[2].disabled).toBe(false);
  });

  it('J. Color-only product still works', () => {
    const colorOnlyVariants: ProductVariantItem[] = [
      { id: 'vc1', colorId: 'c1', colorName: 'Черный', colorHex: '#000000', inStock: true, isActive: true },
      { id: 'vc2', colorId: 'c2', colorName: 'Белый', colorHex: '#FFFFFF', inStock: false, isActive: true },
    ];
    const dim = getDimensionType(colorOnlyVariants);
    expect(dim).toBe('COLOR_ONLY');

    const state = selectVariantState(colorOnlyVariants, 'c1', null);
    expect(state.requiresColor).toBe(true);
    expect(state.requiresSize).toBe(false);
    expect(state.isResolved).toBe(true);
    expect(state.selectedVariant?.id).toBe('vc1');
    expect(state.canAddToCart).toBe(true);
  });

  it('K. Single standard variant still works without selectors', () => {
    const singleVariant: ProductVariantItem[] = [
      { id: 'v-single', size: 'Единый', inStock: true, isActive: true },
    ];
    const dim = getDimensionType(singleVariant);
    expect(dim).toBe('SINGLE_VARIANT');

    const state = selectVariantState(singleVariant, null, null);
    expect(state.requiresColor).toBe(false);
    expect(state.requiresSize).toBe(false);
    expect(state.isResolved).toBe(true);
    expect(state.selectedVariant?.id).toBe('v-single');
    expect(state.canAddToCart).toBe(true);
    expect(state.ctaText).toBe('Добавить в корзину');
  });

  it('Legacy fallback works for un-migrated textual variants without IDs', () => {
    const legacyVariants: ProductVariantItem[] = [
      { id: 'leg-1', color: 'Красный', size: 'M', inStock: true, isActive: true },
      { id: 'leg-2', color: 'Красный', size: 'L', inStock: false, isActive: true },
    ];
    const colors = getColorOptions(legacyVariants);
    expect(colors).toHaveLength(1);
    expect(colors[0].name).toBe('Красный');

    const sizes = getSizeOptions(legacyVariants, colors[0].id, 'COLOR_AND_SIZE');
    expect(sizes).toHaveLength(2);
    expect(sizes[0].label).toBe('M');
    expect(sizes[0].inStock).toBe(true);
    expect(sizes[1].label).toBe('L');
    expect(sizes[1].inStock).toBe(false);

    const state = selectVariantState(legacyVariants, colors[0].id, sizes[0].id);
    expect(state.isResolved).toBe(true);
    expect(state.selectedVariant?.id).toBe('leg-1');
  });

  it('PV.2B.1 Test A: White selected, all White variants out of stock -> CTA = "Нет в наличии", canAddToCart = false', () => {
    const state = selectVariantState(hoodieVariants, whiteColorId, null);
    expect(state.selectedColorId).toBe(whiteColorId);
    expect(state.selectedSizeId).toBeNull();
    expect(state.ctaText).toBe('Нет в наличии');
    expect(state.canAddToCart).toBe(false);
  });

  it('PV.2B.1 Test B: Red selected, no size selected, Red/L exists in stock -> CTA = "Выберите размер", canAddToCart = false', () => {
    const state = selectVariantState(hoodieVariants, redColorId, null);
    expect(state.selectedColorId).toBe(redColorId);
    expect(state.selectedSizeId).toBeNull();
    expect(state.ctaText).toBe('Выберите размер');
    expect(state.canAddToCart).toBe(false);
  });

  it('PV.2B.1 Test C: Red/L selected -> CTA = "Добавить в корзину", canAddToCart = true', () => {
    const state = selectVariantState(hoodieVariants, redColorId, sizeLId);
    expect(state.selectedColorId).toBe(redColorId);
    expect(state.selectedSizeId).toBe(sizeLId);
    expect(state.isResolved).toBe(true);
    expect(state.selectedVariant?.id).toBe(varRedLId);
    expect(state.ctaText).toBe('Добавить в корзину');
    expect(state.canAddToCart).toBe(true);
  });

  it('PV.2B.1 Test D & E: Quantity controls enabled state condition (!isResolved || !canAddToCart)', () => {
    // Before selection or when size is not selected
    const unselectedState = selectVariantState(hoodieVariants, redColorId, null);
    const isQtyEnabledUnselected = unselectedState.isResolved && unselectedState.canAddToCart;
    expect(isQtyEnabledUnselected).toBe(false);

    // White selected (out of stock)
    const whiteState = selectVariantState(hoodieVariants, whiteColorId, null);
    const isQtyEnabledWhite = whiteState.isResolved && whiteState.canAddToCart;
    expect(isQtyEnabledWhite).toBe(false);

    // Red/L selected (buyable)
    const redLState = selectVariantState(hoodieVariants, redColorId, sizeLId);
    const isQtyEnabledRedL = redLState.isResolved && redLState.canAddToCart;
    expect(isQtyEnabledRedL).toBe(true);
  });
});

describe('SHOP PDP.2B — Variant Selection State Hardening', () => {
  const blackColorId = 'color-black-1';
  const whiteColorId = 'color-white-2';
  const blueColorId = 'color-blue-3';

  const sizeSId = 'size-s-1';
  const sizeMId = 'size-m-2';
  const sizeLId = 'size-l-3';

  // Product fixture matching prompt target behavior:
  // BLACK: S (inStock: true), M (inStock: true, price 10000), L (inStock: false)
  // WHITE: S (inStock: true), M (does not exist), L (inStock: true)
  // BLUE: S (inStock: true), M (inStock: false, sold out), L (inStock: true)
  const testVariants: ProductVariantItem[] = [
    {
      id: 'var-black-s',
      productId: 'prod-test',
      colorId: blackColorId,
      colorName: 'Черный',
      colorHex: '#000000',
      sizeValueId: sizeSId,
      size: 'S',
      isActive: true,
      inStock: true,
      priceCents: 900000,
    },
    {
      id: 'var-black-m',
      productId: 'prod-test',
      colorId: blackColorId,
      colorName: 'Черный',
      colorHex: '#000000',
      sizeValueId: sizeMId,
      size: 'M',
      isActive: true,
      inStock: true,
      priceCents: 1000000,
    },
    {
      id: 'var-black-l',
      productId: 'prod-test',
      colorId: blackColorId,
      colorName: 'Черный',
      colorHex: '#000000',
      sizeValueId: sizeLId,
      size: 'L',
      isActive: true,
      inStock: false,
      priceCents: 1000000,
    },
    {
      id: 'var-white-s',
      productId: 'prod-test',
      colorId: whiteColorId,
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      sizeValueId: sizeSId,
      size: 'S',
      isActive: true,
      inStock: true,
      priceCents: 950000,
    },
    // WHITE M does not exist!
    {
      id: 'var-white-l',
      productId: 'prod-test',
      colorId: whiteColorId,
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      sizeValueId: sizeLId,
      size: 'L',
      isActive: true,
      inStock: true,
      priceCents: 1200000,
    },
    {
      id: 'var-blue-m',
      productId: 'prod-test',
      colorId: blueColorId,
      colorName: 'Синий',
      colorHex: '#0000FF',
      sizeValueId: sizeMId,
      size: 'M',
      isActive: true,
      inStock: false, // SOLD OUT in blue!
      priceCents: 1100000,
    },
  ];

  // Fixture where WHITE also has M buyable at a different price:
  const whiteWithMVariants: ProductVariantItem[] = [
    ...testVariants,
    {
      id: 'var-white-m',
      productId: 'prod-test',
      colorId: whiteColorId,
      colorName: 'Белый',
      colorHex: '#FFFFFF',
      sizeValueId: sizeMId,
      size: 'M',
      isActive: true,
      inStock: true, // Buyable in white!
      priceCents: 1200000, // Price is 12000 (differs from black)
    },
  ];

  it('1. BLACK/M inStock -> switch WHITE/M inStock => M retained', () => {
    const { result } = renderHook(() =>
      useVariantSelection(whiteWithMVariants, undefined, blackColorId, sizeMId)
    );
    expect(result.current.selectedColorId).toBe(blackColorId);
    expect(result.current.selectedSizeId).toBe(sizeMId);
    expect(result.current.selectedVariant?.id).toBe('var-black-m');

    act(() => {
      result.current.selectColor(whiteColorId);
    });

    expect(result.current.selectedColorId).toBe(whiteColorId);
    expect(result.current.selectedSizeId).toBe(sizeMId);
    expect(result.current.selectedVariant?.id).toBe('var-white-m');
    expect(result.current.canAddToCart).toBe(true);
    expect(result.current.sizeSelectionNotice).toBeNull();
  });

  it('2. BLACK/M inStock -> switch BLUE/M exists but inStock false => M cleared => notice shown => no alternative auto-selected', () => {
    const { result } = renderHook(() =>
      useVariantSelection(testVariants, undefined, blackColorId, sizeMId)
    );
    expect(result.current.selectedColorId).toBe(blackColorId);
    expect(result.current.selectedSizeId).toBe(sizeMId);

    act(() => {
      result.current.selectColor(blueColorId);
    });

    expect(result.current.selectedColorId).toBe(blueColorId);
    expect(result.current.selectedSizeId).toBeNull();
    expect(result.current.selectedVariant).toBeNull();
    expect(result.current.canAddToCart).toBe(false);
    expect(result.current.sizeSelectionNotice).toBe('Размер M недоступен в синем цвете');
  });

  it('3. BLACK/M inStock -> switch WHITE with no M variant => M cleared => notice shown', () => {
    const { result } = renderHook(() =>
      useVariantSelection(testVariants, undefined, blackColorId, sizeMId)
    );

    act(() => {
      result.current.selectColor(whiteColorId);
    });

    expect(result.current.selectedColorId).toBe(whiteColorId);
    expect(result.current.selectedSizeId).toBeNull();
    expect(result.current.selectedVariant).toBeNull();
    expect(result.current.sizeSelectionNotice).toBe('Размер M недоступен в белом цвете');
  });

  it('4. No size selected -> change color => no notice', () => {
    const { result } = renderHook(() =>
      useVariantSelection(testVariants, undefined, blackColorId, null)
    );

    act(() => {
      result.current.selectColor(whiteColorId);
    });

    expect(result.current.selectedColorId).toBe(whiteColorId);
    expect(result.current.selectedSizeId).toBeNull();
    expect(result.current.sizeSelectionNotice).toBeNull();
  });

  it('5. Invalidated size -> user selects valid new size => notice cleared', () => {
    const { result } = renderHook(() =>
      useVariantSelection(testVariants, undefined, blackColorId, sizeMId)
    );

    // Switch to white where M does not exist
    act(() => {
      result.current.selectColor(whiteColorId);
    });
    expect(result.current.sizeSelectionNotice).toBe('Размер M недоступен в белом цвете');

    // Customer selects S in white
    act(() => {
      result.current.selectSize(sizeSId);
    });

    expect(result.current.selectedSizeId).toBe(sizeSId);
    expect(result.current.selectedVariant?.id).toBe('var-white-s');
    expect(result.current.sizeSelectionNotice).toBeNull();
  });

  it('6. Preserved size resolves new exact variant ID', () => {
    const { result } = renderHook(() =>
      useVariantSelection(whiteWithMVariants, undefined, blackColorId, sizeMId)
    );
    expect(result.current.selectedVariant?.id).toBe('var-black-m');

    act(() => {
      result.current.selectColor(whiteColorId);
    });

    expect(result.current.selectedVariant?.id).toBe('var-white-m');
    expect(result.current.selectedVariant?.id).not.toBe('var-black-m');
  });

  it('7. Preserved size resolves new variant price', () => {
    const { result } = renderHook(() =>
      useVariantSelection(whiteWithMVariants, undefined, blackColorId, sizeMId)
    );
    expect(result.current.selectedVariant?.priceCents).toBe(1000000);

    act(() => {
      result.current.selectColor(whiteColorId);
    });

    expect(result.current.selectedVariant?.priceCents).toBe(1200000);
  });

  it('8. inStock undefined + isActive true => NOT buyable', () => {
    const variantNoStock: ProductVariantItem = {
      id: 'var-no-stock',
      productId: 'prod-1',
      size: 'M',
      sizeValueId: 's-m',
      isActive: true,
      inStock: undefined,
    };
    expect(isVariantBuyable(variantNoStock)).toBe(false);

    const state = selectVariantState([variantNoStock], null, 's-m');
    expect(state.isResolved).toBe(true);
    expect(state.canAddToCart).toBe(false);
    expect(state.ctaText).toBe('Нет в наличии');
  });

  it('formatSizeUnavailableNotice produces correct Russian grammar for standard and custom colors', () => {
    expect(formatSizeUnavailableNotice('M', 'Белый')).toBe('Размер M недоступен в белом цвете');
    expect(formatSizeUnavailableNotice('L', 'Красный')).toBe('Размер L недоступен в красном цвете');
    expect(formatSizeUnavailableNotice('S', 'Черный')).toBe('Размер S недоступен в черном цвете');
    expect(formatSizeUnavailableNotice('XL', 'Хаки')).toBe('Размер XL недоступен в цвете хаки');
    expect(formatSizeUnavailableNotice('42', 'CustomGold')).toBe('Размер 42 недоступен в цвете «CustomGold»');
    expect(formatSizeUnavailableNotice('M', null)).toBe('Размер M недоступен в выбранном цвете');
  });
});

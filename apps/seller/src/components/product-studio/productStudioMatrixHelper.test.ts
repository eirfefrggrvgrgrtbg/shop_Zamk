import { describe, it, expect } from 'vitest';
import {
  addColorToMatrix,
  addSizeToMatrix,
  removeColorFromMatrix,
  removeSizeFromMatrix,
} from './productStudioMatrixHelper';
import type { ProductStudioVariant } from '../../contexts/ProductStudioContext';

describe('productStudioMatrixHelper', () => {
  it('Example A: existing Black/S, Black/M + White => Black/S, Black/M, White/S, White/M', () => {
    const existing: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', colorName: 'Черный', sizeValueId: 'sz-s', size: 'S', isActive: true },
      { id: 'v2', colorId: 'col-black', colorName: 'Черный', sizeValueId: 'sz-m', size: 'M', isActive: true },
    ];

    const result = addColorToMatrix(existing, { id: 'col-white', name: 'Белый', hex: '#ffffff' });

    expect(result).toHaveLength(4);
    // Preserves existing v1 and v2
    expect(result.find((v) => v.id === 'v1')).toEqual(existing[0]);
    expect(result.find((v) => v.id === 'v2')).toEqual(existing[1]);

    // Added combinations
    const whiteS = result.find((v) => v.colorId === 'col-white' && v.sizeValueId === 'sz-s');
    const whiteM = result.find((v) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m');
    expect(whiteS).toBeTruthy();
    expect(whiteS?.colorName).toBe('Белый');
    expect(whiteS?.size).toBe('S');
    expect(whiteM).toBeTruthy();
    expect(whiteM?.colorName).toBe('Белый');
    expect(whiteM?.size).toBe('M');

    // No stock
    result.forEach((v) => {
      expect((v as any).stock).toBeUndefined();
      expect((v as any).initialStock).toBeUndefined();
    });
  });

  it('Example B: existing Black/S, White/S + M => Black/S, White/S, Black/M, White/M', () => {
    const existing: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', colorName: 'Черный', colorHex: '#000000', sizeValueId: 'sz-s', size: 'S', isActive: true },
      { id: 'v2', colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', sizeValueId: 'sz-s', size: 'S', isActive: true },
    ];

    const result = addSizeToMatrix(existing, { id: 'sz-m', label: 'M' });

    expect(result).toHaveLength(4);
    expect(result.find((v) => v.id === 'v1')).toEqual(existing[0]);
    expect(result.find((v) => v.id === 'v2')).toEqual(existing[1]);

    const blackM = result.find((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m');
    const whiteM = result.find((v) => v.colorId === 'col-white' && v.sizeValueId === 'sz-m');
    expect(blackM).toBeTruthy();
    expect(blackM?.colorName).toBe('Черный');
    expect(blackM?.size).toBe('M');
    expect(whiteM).toBeTruthy();
    expect(whiteM?.colorName).toBe('Белый');
    expect(whiteM?.size).toBe('M');
  });

  it('prevents duplicate (colorId, sizeValueId) additions', () => {
    const existing: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S', isActive: true },
    ];

    const afterColor = addColorToMatrix(existing, { id: 'col-black', name: 'Черный' });
    expect(afterColor).toHaveLength(1);

    const afterSize = addSizeToMatrix(existing, { id: 'sz-s', label: 'S' });
    expect(afterSize).toHaveLength(1);
  });

  it('removes matching variants when color is removed', () => {
    const existing: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' },
      { id: 'v3', colorId: 'col-white', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v4', colorId: 'col-white', sizeValueId: 'sz-m', size: 'M' },
    ];

    const result = removeColorFromMatrix(existing, 'col-black');
    expect(result).toHaveLength(2);
    expect(result.every((v) => v.colorId === 'col-white')).toBe(true);
  });

  it('removes matching variants when size is removed', () => {
    const existing: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' },
      { id: 'v3', colorId: 'col-white', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v4', colorId: 'col-white', sizeValueId: 'sz-m', size: 'M' },
    ];

    const result = removeSizeFromMatrix(existing, 'sz-s');
    expect(result).toHaveLength(2);
    expect(result.every((v) => v.sizeValueId === 'sz-m')).toBe(true);
  });

  it('Case A: COLOR_ONLY -> COLOR_AND_SIZE', () => {
    // start empty
    let variants: ProductStudioVariant[] = [];
    
    // add Black
    variants = addColorToMatrix(variants, { id: 'col-black', name: 'Black' });
    expect(variants).toHaveLength(1);
    expect(variants[0].colorId).toBe('col-black');
    expect(variants[0].sizeValueId).toBeUndefined(); // NO ONE_SIZE sentinel

    // then add M
    variants = addSizeToMatrix(variants, { id: 'sz-m', label: 'M' });
    expect(variants).toHaveLength(1);
    expect(variants[0].colorId).toBe('col-black');
    expect(variants[0].sizeValueId).toBe('sz-m');
  });

  it('Case B: SIZE_ONLY -> COLOR_AND_SIZE', () => {
    // start empty
    let variants: ProductStudioVariant[] = [];
    
    // add M
    variants = addSizeToMatrix(variants, { id: 'sz-m', label: 'M' });
    expect(variants).toHaveLength(1);
    expect(variants[0].sizeValueId).toBe('sz-m');
    expect(variants[0].colorId).toBeUndefined(); // NO default color sentinel

    // then add Black
    variants = addColorToMatrix(variants, { id: 'col-black', name: 'Black' });
    expect(variants).toHaveLength(1);
    expect(variants[0].colorId).toBe('col-black');
    expect(variants[0].sizeValueId).toBe('sz-m');
  });

  it('Case C: FULL CARTESIAN exactly', () => {
    let variants: ProductStudioVariant[] = [];
    variants = addColorToMatrix(variants, { id: 'col-black', name: 'Black' });
    variants = addColorToMatrix(variants, { id: 'col-white', name: 'White' });
    variants = addSizeToMatrix(variants, { id: 'sz-s', label: 'S' });
    variants = addSizeToMatrix(variants, { id: 'sz-m', label: 'M' });
    
    expect(variants).toHaveLength(4);
    const hasBlackS = variants.some(v => v.colorId === 'col-black' && v.sizeValueId === 'sz-s');
    const hasBlackM = variants.some(v => v.colorId === 'col-black' && v.sizeValueId === 'sz-m');
    const hasWhiteS = variants.some(v => v.colorId === 'col-white' && v.sizeValueId === 'sz-s');
    const hasWhiteM = variants.some(v => v.colorId === 'col-white' && v.sizeValueId === 'sz-m');
    
    expect(hasBlackS).toBe(true);
    expect(hasBlackM).toBe(true);
    expect(hasWhiteS).toBe(true);
    expect(hasWhiteM).toBe(true);
  });

  it('removeColorFromMatrix keeps SIZE_ONLY variants if they exist independently', () => {
    let variants: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v2', colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' }
    ];
    
    // Remove the only color 'Black'
    variants = removeColorFromMatrix(variants, 'col-black');
    expect(variants).toHaveLength(2);
    expect(variants[0].colorId).toBeUndefined();
    expect(variants[0].sizeValueId).toBe('sz-s');
    expect(variants[1].colorId).toBeUndefined();
    expect(variants[1].sizeValueId).toBe('sz-m');
  });

  it('removeSizeFromMatrix keeps COLOR_ONLY variants if they exist independently', () => {
    let variants: ProductStudioVariant[] = [
      { id: 'v1', colorId: 'col-black', sizeValueId: 'sz-s', size: 'S' },
      { id: 'v2', colorId: 'col-white', sizeValueId: 'sz-s', size: 'S' }
    ];
    
    // Remove the only size 'S'
    variants = removeSizeFromMatrix(variants, 'sz-s');
    expect(variants).toHaveLength(2);
    expect(variants[0].sizeValueId).toBeUndefined();
    expect(variants[0].colorId).toBe('col-black');
    expect(variants[1].sizeValueId).toBeUndefined();
    expect(variants[1].colorId).toBe('col-white');
  });
});

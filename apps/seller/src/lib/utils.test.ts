import { describe, it, expect } from 'vitest';
import { parseRubToCentsExact } from './utils';

describe('parseRubToCentsExact', () => {
  it('converts exact integer and decimal ruble amounts to cents without float inaccuracy', () => {
    expect(parseRubToCentsExact('1')).toBe(100);
    expect(parseRubToCentsExact('1.01')).toBe(101);
    expect(parseRubToCentsExact('10.10')).toBe(1010);
    expect(parseRubToCentsExact('999.99')).toBe(99999);
    expect(parseRubToCentsExact('1000')).toBe(100000);
  });

  it('handles comma as decimal separator and single fractional digit', () => {
    expect(parseRubToCentsExact('1,01')).toBe(101);
    expect(parseRubToCentsExact('10,10')).toBe(1010);
    expect(parseRubToCentsExact('10.1')).toBe(1010);
    expect(parseRubToCentsExact('10,1')).toBe(1010);
    expect(parseRubToCentsExact('  1000  ')).toBe(100000);
    expect(parseRubToCentsExact('1000.00')).toBe(100000);
  });

  it('rejects invalid precision with more than 2 decimal digits', () => {
    expect(() => parseRubToCentsExact('1.001')).toThrow(
      'Максимальная скидка не может содержать более 2 знаков после запятой.'
    );
    expect(() => parseRubToCentsExact('1,001')).toThrow(
      'Максимальная скидка не может содержать более 2 знаков после запятой.'
    );
    expect(() => parseRubToCentsExact('10.12345')).toThrow(
      'Максимальная скидка не может содержать более 2 знаков после запятой.'
    );
  });

  it('rejects zero, negative, empty or non-numeric inputs', () => {
    expect(() => parseRubToCentsExact('')).toThrow('Укажите сумму в рублях.');
    expect(() => parseRubToCentsExact('   ')).toThrow('Укажите сумму в рублях.');
    expect(() => parseRubToCentsExact('0')).toThrow(
      'Максимальная скидка должна быть положительным числом.'
    );
    expect(() => parseRubToCentsExact('0.00')).toThrow(
      'Максимальная скидка должна быть положительным числом.'
    );
    expect(() => parseRubToCentsExact('-5')).toThrow(
      'Максимальная скидка должна быть положительным числом.'
    );
    expect(() => parseRubToCentsExact('abc')).toThrow(
      'Максимальная скидка должна быть положительным числом.'
    );
  });
});

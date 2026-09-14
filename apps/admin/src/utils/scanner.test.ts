import { describe, it, expect } from 'vitest';
import { normalizeScannerCode } from './scanner';

describe('normalizeScannerCode (SCN.1C)', () => {
  // Prompt Mandatory Examples:
  it('A. leaves canonical Latin ZMU unchanged', () => {
    expect(normalizeScannerCode('ZMU-BR8XJV54XCMX48ZZ')).toBe('ZMU-BR8XJV54XCMX48ZZ');
  });

  it('B. normalizes Russian-layout physical-key equivalent of ZMU-BR8XJV54XCMX48ZZ to canonical ZMU', () => {
    // Z->Я, M->Ь, U->Г, - -> -, B->И, R->К, 8->8, X->Ч, J->О, V->М, 5->5, 4->4, X->Ч, C->С, M->Ь, X->Ч, 4->4, 8->8, Z->Я, Z->Я
    const ruLayoutInput = 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ';
    expect(normalizeScannerCode(ruLayoutInput)).toBe('ZMU-BR8XJV54XCMX48ZZ');
  });

  it('C. lowercase RU mapping works and preserves lowercase', () => {
    const ruLowerInput = 'яьг-ик8чом54чсьч48яя';
    expect(normalizeScannerCode(ruLowerInput)).toBe('zmu-br8xjv54xcmx48zz');
  });

  it('D. uppercase RU mapping works and preserves uppercase', () => {
    const ruUpperInput = 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ';
    expect(normalizeScannerCode(ruUpperInput)).toBe('ZMU-BR8XJV54XCMX48ZZ');
  });

  it('E. mixed Latin + Cyrillic works character-by-character', () => {
    expect(normalizeScannerCode('ZMU-ИК8Ч')).toBe('ZMU-BR8X');
    expect(normalizeScannerCode('ЯЬГ-BR8X')).toBe('ZMU-BR8X');
  });

  it('F. digits remain unchanged', () => {
    expect(normalizeScannerCode('0123456789')).toBe('0123456789');
    expect(normalizeScannerCode('4601234567890')).toBe('4601234567890');
  });

  it('G. hyphen remains unchanged', () => {
    expect(normalizeScannerCode('-')).toBe('-');
    expect(normalizeScannerCode('---')).toBe('---');
  });

  it('reverses complete standard RU JCUKEN mapping for all 33 letters (lowercase and uppercase)', () => {
    const ruLower = 'йцукенгшщзхъфывапролджэячсмитьбюё';
    const enLower = "qwertyuiop[]asdfghjkl;'zxcvbnm,.`";
    expect(normalizeScannerCode(ruLower)).toBe(enLower);

    const ruUpper = 'ЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮЁ';
    const enUpper = 'QWERTYUIOP{}ASDFGHJKL:"ZXCVBNM<>~';
    expect(normalizeScannerCode(ruUpper)).toBe(enUpper);
  });

  it('trims scanner transport whitespace and newlines', () => {
    expect(normalizeScannerCode('  \r\n\t ZMU-BR8XJV54XCMX48ZZ \n  ')).toBe('ZMU-BR8XJV54XCMX48ZZ');
    expect(normalizeScannerCode('  \n ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ \r\n ')).toBe('ZMU-BR8XJV54XCMX48ZZ');
  });

  it('handles empty, null, or undefined inputs gracefully', () => {
    expect(normalizeScannerCode('')).toBe('');
    expect(normalizeScannerCode('   ')).toBe('');
    expect(normalizeScannerCode(null)).toBe('');
    expect(normalizeScannerCode(undefined)).toBe('');
  });

  it('normalizes Russian-layout ZMK barcode, SKU, and supply/fulfillment codes', () => {
    // ZMK-b4f777d2-097 in RU -> ЯЬЛ-и4а777в2-097
    expect(normalizeScannerCode('ЯЬЛ-и4а777в2-097')).toBe('ZMK-b4f777d2-097');

    // SKU-4444 in RU -> ЫЛГ-4444
    expect(normalizeScannerCode('ЫЛГ-4444')).toBe('SKU-4444');

    // SUP-2026-00001 in RU -> ЫГЗ-2026-00001
    expect(normalizeScannerCode('ЫГЗ-2026-00001')).toBe('SUP-2026-00001');

    // FUL-2026-00001 in RU -> АГД-2026-00001
    expect(normalizeScannerCode('АГД-2026-00001')).toBe('FUL-2026-00001');
  });
});

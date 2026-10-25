import { describe, it, expect } from 'vitest';
import { formatDeliveryAddress, toAddressPayload, validateAddress } from './addressValidation';

const ok = { recipientName: 'Анна Иванова', phone: '+7 999 000-11-22', city: 'Москва', street: 'Тверская', house: '1' };

describe('validateAddress', () => {
  it('accepts a complete address with optional fields empty', () => {
    expect(validateAddress(ok)).toEqual({});
    expect(validateAddress({ ...ok, apartment: '12', postalCode: '101000', label: 'Дом' })).toEqual({});
  });

  it('requires recipient, phone, city, street and house (whitespace counts as empty)', () => {
    const errors = validateAddress({ recipientName: ' ', phone: '', city: '  ', street: '', house: '' });
    expect(Object.keys(errors).sort()).toEqual(['city', 'house', 'phone', 'recipientName', 'street']);
    expect(errors.city).toBe('Заполните поле «Город»');
  });

  it('limits lengths like the API', () => {
    expect(validateAddress({ ...ok, city: 'я'.repeat(121) }).city).toMatch(/слишком длинное/);
    expect(validateAddress({ ...ok, city: 'я'.repeat(120) }).city).toBeUndefined();
    expect(validateAddress({ ...ok, comment: 'x'.repeat(501) }).comment).toMatch(/500/);
  });

  it('checks the phone: characters and 7–15 digits', () => {
    expect(validateAddress({ ...ok, phone: 'abc' }).phone).toMatch(/только цифры/);
    expect(validateAddress({ ...ok, phone: '12345' }).phone).toMatch(/корректный телефон/);
    expect(validateAddress({ ...ok, phone: '1'.repeat(16) }).phone).toMatch(/корректный телефон/);
    expect(validateAddress({ ...ok, phone: '8 (999) 000-11-22' }).phone).toBeUndefined();
  });

  it('checks the postal code characters', () => {
    expect(validateAddress({ ...ok, postalCode: '10<1>' }).postalCode).toMatch(/Индекс/);
    expect(validateAddress({ ...ok, postalCode: 'SW1A 1AA' }).postalCode).toBeUndefined();
  });
});

describe('toAddressPayload / formatDeliveryAddress', () => {
  it('sends only the fields the API accepts, trimmed', () => {
    const payload = toAddressPayload({ ...ok, recipientName: '  Анна  ', id: 'x', userId: 'u', createdAt: 'c' } as never);
    expect(Object.keys(payload).sort()).toEqual(['apartment', 'city', 'comment', 'house', 'label', 'phone', 'postalCode', 'recipientName', 'street']);
    expect(payload.recipientName).toBe('Анна');
  });

  it('writes the one-line order address', () => {
    expect(formatDeliveryAddress({ city: 'Москва', street: 'Тверская', house: '1', apartment: '12' })).toBe('г. Москва, ул. Тверская, д. 1, кв. 12');
    expect(formatDeliveryAddress({ city: 'Москва', street: 'Тверская', house: '1' })).toBe('г. Москва, ул. Тверская, д. 1');
  });
});

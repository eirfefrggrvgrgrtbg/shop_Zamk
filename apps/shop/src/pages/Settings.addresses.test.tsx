/**
 * @vitest-environment jsdom
 */
// SHOP.ADDRESSES.1 — the address book in «Настройки»: empty state, inline validation, payload, server errors, delete.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Settings } from './Settings';

const getAddresses = vi.fn();
const createAddress = vi.fn();
const updateAddress = vi.fn();
const deleteAddress = vi.fn();
const setDefaultAddress = vi.fn();
const showToast = vi.fn();

vi.mock('@zamk/api-client/src/customer', () => ({
  getAddresses: (...a: unknown[]) => getAddresses(...a),
  createAddress: (...a: unknown[]) => createAddress(...a),
  updateAddress: (...a: unknown[]) => updateAddress(...a),
  deleteAddress: (...a: unknown[]) => deleteAddress(...a),
  setDefaultAddress: (...a: unknown[]) => setDefaultAddress(...a),
}));
vi.mock('../contexts/ThemeContext', () => ({ useTheme: () => ({ theme: 'light', setTheme: vi.fn() }) }));
vi.mock('../contexts/ToastContext', () => ({ useToast: () => ({ showToast }) }));
vi.mock('../components/account/AccountLayout', () => ({ AccountLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));

const home = { id: 'a1', label: 'Дом', recipientName: 'Анна Иванова', phone: '+7 999 000-11-22', city: 'Москва', street: 'Тверская', house: '1', apartment: '12', isDefault: true };
const office = { id: 'a2', label: 'Офис', recipientName: 'Анна Иванова', phone: '+7 999 000-11-22', city: 'Казань', street: 'Баумана', house: '5', isDefault: false };

const open = () => render(<MemoryRouter><Settings /></MemoryRouter>);
async function fillNew(over: Record<string, string> = {}) {
  fireEvent.click(await screen.findByRole('button', { name: /Добавить адрес/ }));
  const values: Record<string, string> = { recipientName: 'Анна Иванова', phone: '+7 999 000-11-22', city: 'Москва', street: 'Тверская', house: '1', ...over };
  for (const [field, value] of Object.entries(values)) fireEvent.change(await screen.findByTestId(`addr-${field}`), { target: { value } });
}

describe('Settings — address book', () => {
  beforeEach(() => {
    for (const f of [getAddresses, createAddress, updateAddress, deleteAddress, setDefaultAddress, showToast]) f.mockReset();
    getAddresses.mockResolvedValue([]);
    createAddress.mockResolvedValue({});
    deleteAddress.mockResolvedValue({ status: 'ok' });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });
  afterEach(() => cleanup());

  it('empty state, then the form opens', async () => {
    open();
    expect(await screen.findByText('У вас пока нет сохранённых адресов.')).toBeDefined();
    expect(screen.queryAllByTestId('address-card')).toHaveLength(0);
  });

  it('lists addresses, marks the main one', async () => {
    getAddresses.mockResolvedValue([home, office]);
    open();
    const cards = await screen.findAllByTestId('address-card');
    expect(cards).toHaveLength(2);
    expect(cards[0].getAttribute('data-default')).toBe('true');
    expect(cards[0].textContent).toContain('Основной');
    expect(cards[1].textContent).toContain('Сделать основным');
    expect(cards[1].getAttribute('data-default')).toBe('false');
  });

  it('empty required fields: inline errors next to the fields, nothing is sent', async () => {
    open();
    fireEvent.click(await screen.findByRole('button', { name: /Добавить адрес/ }));
    fireEvent.click(await screen.findByTestId('addr-save'));
    for (const f of ['recipientName', 'phone', 'city', 'street', 'house']) {
      expect((await screen.findByTestId(`addr-error-${f}`)).textContent).toMatch(/Заполните поле/);
    }
    expect(createAddress).not.toHaveBeenCalled();
  });

  it('an error disappears when the field is corrected; a bad phone is explained', async () => {
    open();
    await fillNew({ phone: '123' });
    fireEvent.click(screen.getByTestId('addr-save'));
    expect((await screen.findByTestId('addr-error-phone')).textContent).toMatch(/корректный телефон/);
    fireEvent.change(screen.getByTestId('addr-phone'), { target: { value: '+7 999 000-11-22' } });
    await waitFor(() => expect(screen.queryByTestId('addr-error-phone')).toBeNull());
  });

  it('saves a trimmed payload with only the supported fields', async () => {
    open();
    await fillNew({ recipientName: '  Анна Иванова  ' });
    fireEvent.click(screen.getByTestId('addr-save'));
    await waitFor(() => expect(createAddress).toHaveBeenCalledTimes(1));
    const payload = createAddress.mock.calls[0][0];
    expect(payload.recipientName).toBe('Анна Иванова');
    expect(Object.keys(payload).sort()).toEqual(['apartment', 'city', 'comment', 'house', 'label', 'phone', 'postalCode', 'recipientName', 'street']);
    await waitFor(() => expect(getAddresses).toHaveBeenCalledTimes(2)); // the list is reloaded
  });

  it('a server validation error is shown at its field', async () => {
    createAddress.mockRejectedValue(Object.assign(new Error('generic'), { rawMessage: 'Поле «Улица» содержит недопустимые символы', data: { error: { code: 'validation_error', field: 'street' } } }));
    open();
    await fillNew();
    fireEvent.click(screen.getByTestId('addr-save'));
    expect((await screen.findByTestId('addr-error-street')).textContent).toContain('недопустимые символы');
  });

  it('editing sends the changes for that address id; the main address can be deleted after confirmation', async () => {
    getAddresses.mockResolvedValue([home]);
    updateAddress.mockResolvedValue({});
    open();
    fireEvent.click(await screen.findByRole('button', { name: 'Изм.' }));
    fireEvent.change(await screen.findByTestId('addr-city'), { target: { value: 'Санкт-Петербург' } });
    fireEvent.click(screen.getByTestId('addr-save'));
    await waitFor(() => expect(updateAddress).toHaveBeenCalled());
    expect(updateAddress.mock.calls[0][0]).toBe('a1');
    expect(updateAddress.mock.calls[0][1].city).toBe('Санкт-Петербург');

    fireEvent.click(await screen.findByTestId('address-delete'));
    await waitFor(() => expect(deleteAddress).toHaveBeenCalledWith('a1'));
    expect(window.confirm).toHaveBeenCalled();
  });
});

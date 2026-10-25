// SHOP.ADDRESSES.1: the address form checks what the API checks (same limits), so mistakes are shown next to the
// field before anything is sent. The API stays the authority; its message is shown when it refuses.

export interface AddressForm {
  label?: string;
  recipientName?: string;
  phone?: string;
  city?: string;
  street?: string;
  house?: string;
  apartment?: string;
  postalCode?: string;
  comment?: string;
}

export const ADDRESS_MAX: Record<string, number> = {
  label: 60, recipientName: 120, phone: 32, city: 120, street: 200, house: 30, apartment: 30, postalCode: 12, comment: 500,
};

const REQUIRED: Array<[keyof AddressForm, string]> = [
  ['recipientName', 'ФИО получателя'], ['phone', 'Телефон'], ['city', 'Город'], ['street', 'Улица'], ['house', 'Дом'],
];

const LABELS: Record<string, string> = {
  label: 'Название', recipientName: 'ФИО получателя', phone: 'Телефон', city: 'Город', street: 'Улица', house: 'Дом',
  apartment: 'Квартира', postalCode: 'Индекс', comment: 'Комментарий',
};

export function validateAddress(form: AddressForm): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const [field, label] of REQUIRED) {
    if (!(form[field] ?? '').trim()) errors[field] = `Заполните поле «${label}»`;
  }
  for (const [field, max] of Object.entries(ADDRESS_MAX)) {
    const value = (form[field as keyof AddressForm] ?? '').trim();
    if (!errors[field] && [...value].length > max) errors[field] = `Поле «${LABELS[field]}» слишком длинное (не более ${max} символов)`;
  }
  const phone = (form.phone ?? '').trim();
  if (phone && !errors.phone) {
    const digits = phone.replace(/\D/g, '').length;
    if (!/^[0-9+\-() .]+$/.test(phone)) errors.phone = 'Телефон может содержать только цифры, пробелы и символы + - ( )';
    else if (digits < 7 || digits > 15) errors.phone = 'Укажите корректный телефон (от 7 до 15 цифр)';
  }
  const postal = (form.postalCode ?? '').trim();
  if (postal && !errors.postalCode && !/^[\p{L}\p{N} -]+$/u.test(postal)) {
    errors.postalCode = 'Индекс может содержать только буквы, цифры, пробел и дефис';
  }
  return errors;
}

/** Only the fields the API accepts, trimmed (no ids, timestamps or owner). */
export function toAddressPayload(form: AddressForm): Required<AddressForm> {
  const t = (v?: string) => (v ?? '').trim();
  return {
    label: t(form.label), recipientName: t(form.recipientName), phone: t(form.phone), city: t(form.city), street: t(form.street),
    house: t(form.house), apartment: t(form.apartment), postalCode: t(form.postalCode), comment: t(form.comment),
  };
}

/** The one-line delivery address an order stores (a snapshot taken when the order is placed). */
export function formatDeliveryAddress(a: { city: string; street: string; house: string; apartment?: string | null }): string {
  return `г. ${a.city}, ул. ${a.street}, д. ${a.house}${a.apartment ? `, кв. ${a.apartment}` : ''}`;
}

/** The optional extras of a checkout (postal code and a note for the courier), checked like the API checks them. */
export function validateDeliveryExtras(postalCode: string, comment: string): Record<'postalCode' | 'comment', string | undefined> {
  const postal = postalCode.trim();
  const note = comment.trim();
  const out: Record<'postalCode' | 'comment', string | undefined> = { postalCode: undefined, comment: undefined };
  if ([...postal].length > ADDRESS_MAX.postalCode) out.postalCode = 'Индекс слишком длинный (не более 12 символов)';
  else if (postal && !/^[\p{L}\p{N} -]+$/u.test(postal)) out.postalCode = 'Индекс может содержать только буквы, цифры, пробел и дефис';
  if ([...note].length > ADDRESS_MAX.comment) out.comment = 'Комментарий для курьера слишком длинный (не более 500 символов)';
  else if (/\p{Cc}/u.test(note)) out.comment = 'Комментарий для курьера содержит недопустимые символы';
  return out;
}

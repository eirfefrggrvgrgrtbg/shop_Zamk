export function formatRussianPhone(value: string): string {
  // Extract all digits
  const digits = value.replace(/\D/g, '');
  
  if (!digits) return '';
  
  // Handle case where user starts typing with 8 or 7
  let normalizedDigits = digits;
  if (digits[0] === '8' || digits[0] === '7') {
    normalizedDigits = digits.substring(1);
  }

  // Cap at 10 digits after the country code
  normalizedDigits = normalizedDigits.substring(0, 10);

  if (normalizedDigits.length === 0) {
    return '+7';
  }

  let formatted = '+7';
  if (normalizedDigits.length > 0) {
    formatted += ' (' + normalizedDigits.substring(0, 3);
  }
  if (normalizedDigits.length >= 4) {
    formatted += ') ' + normalizedDigits.substring(3, 6);
  }
  if (normalizedDigits.length >= 7) {
    formatted += '-' + normalizedDigits.substring(6, 8);
  }
  if (normalizedDigits.length >= 9) {
    formatted += '-' + normalizedDigits.substring(8, 10);
  }

  return formatted;
}

export function normalizeRussianPhone(value: string): string {
  const digits = value.replace(/\D/g, '');
  if (!digits) return '';
  
  let normalizedDigits = digits;
  if (digits[0] === '8' || digits[0] === '7') {
    normalizedDigits = digits.substring(1);
  }
  
  if (normalizedDigits.length === 0) return '';
  
  return '+7' + normalizedDigits.substring(0, 10);
}

export function validateRussianPhone(value: string): boolean {
  const normalized = normalizeRussianPhone(value);
  // +7 and exactly 10 digits
  return normalized.length === 12 && normalized.startsWith('+7');
}

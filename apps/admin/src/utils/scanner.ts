/**
 * Maps standard Russian JCUKEN physical keyboard layout characters
 * back to US QWERTY physical keys.
 *
 * Used for hardware HID barcode scanners that emit keystrokes
 * under an active Russian keyboard layout on macOS/Windows.
 */
const RU_TO_EN_KEY_MAP: Record<string, string> = {
  // Lowercase
  'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y',
  'г': 'u', 'ш': 'i', 'щ': 'o', 'з': 'p', 'х': '[', 'ъ': ']',
  'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h',
  'о': 'j', 'л': 'k', 'д': 'l', 'ж': ';', 'э': "'",
  'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n',
  'ь': 'm', 'б': ',', 'ю': '.', 'ё': '`',

  // Uppercase
  'Й': 'Q', 'Ц': 'W', 'У': 'E', 'К': 'R', 'Е': 'T', 'Н': 'Y',
  'Г': 'U', 'Ш': 'I', 'Щ': 'O', 'З': 'P', 'Х': '{', 'Ъ': '}',
  'Ф': 'A', 'Ы': 'S', 'В': 'D', 'А': 'F', 'П': 'G', 'Р': 'H',
  'О': 'J', 'Л': 'K', 'Д': 'L', 'Ж': ':', 'Э': '"',
  'Я': 'Z', 'Ч': 'X', 'С': 'C', 'М': 'V', 'И': 'B', 'Т': 'N',
  'Ь': 'M', 'Б': '<', 'Ю': '>', 'Ё': '~',
};

/**
 * Normalizes scanned machine codes (ZMU, ZMK, Barcode, SKU, QR, etc.) by mapping
 * Russian keyboard layout keystrokes back to standard English QWERTY characters,
 * while trimming transport whitespace.
 *
 * Preserves unchanged:
 * - existing Latin characters
 * - digits
 * - hyphen
 * - unmapped punctuation / symbols
 * - casing (lowercase remains lowercase, uppercase remains uppercase)
 */
export function normalizeScannerCode(input?: string | null): string {
  if (!input) return '';
  const trimmed = input.trim();
  let result = '';
  for (let i = 0; i < trimmed.length; i++) {
    const char = trimmed[i];
    result += RU_TO_EN_KEY_MAP[char] ?? char;
  }
  return result;
}

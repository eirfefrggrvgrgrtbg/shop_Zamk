export interface VariantDisplaySource {
  colorName?: string | null;
  color?: string | null;
  size?: string | null;
}

export function formatVariantLabel(variant: VariantDisplaySource): string {
  const color = (variant.colorName?.trim() || variant.color?.trim()) || '';
  const size = variant.size?.trim() || '';

  if (color && size) {
    return `${color} · ${size}`;
  }
  if (color) {
    return color;
  }
  if (size) {
    return size;
  }
  return 'Единый вариант';
}

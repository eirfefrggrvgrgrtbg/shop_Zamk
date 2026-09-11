export function formatDaysRussian(days: number): string {
  if (days < 0) days = 0;
  const mod100 = days % 100;
  const mod10 = days % 10;
  if (mod100 >= 11 && mod100 <= 19) {
    return `${days} дней`;
  }
  switch (mod10) {
    case 1:
      return `${days} день`;
    case 2:
    case 3:
    case 4:
      return `${days} дня`;
    default:
      return `${days} дней`;
  }
}

export function formatApproximateDaysOfCover(daysOfCover: number): string {
  const rounded = Math.round(daysOfCover);
  return `≈ ${formatDaysRussian(rounded)} запаса`;
}

export function formatVariantDisplayLabel(color?: string | null, size?: string | null): string {
  const c = (color || '').trim();
  const s = (size || '').trim();
  if (c && s) return `${c} · ${s}`;
  if (c) return c;
  if (s) return s;
  return '';
}

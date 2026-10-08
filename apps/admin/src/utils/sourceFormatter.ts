export function formatSource(sourceKind: string, sourceKey: string | null): string {
  if (sourceKind === 'direct') return 'Прямой заход';
  if (sourceKind === 'unattributed') return 'Неизвестный источник';
  return sourceKey || 'Неизвестный источник';
}

export function mapSavedQueryError(err: unknown): string {
  if (err && typeof err === 'object') {
    const e = err as { status?: number; code?: string; message?: string; error?: string };
    if (
      e.status === 409 ||
      e.code === 'duplicate_name' ||
      e.error === 'duplicate_name' ||
      e.message?.includes('уже существует')
    ) {
      return 'Запрос с таким названием уже существует.';
    }
    if (
      e.status === 404 ||
      e.code === 'not_found' ||
      e.message?.includes('не найден') ||
      e.message?.includes('больше не существует')
    ) {
      return 'Сохранённый запрос больше не существует.';
    }
    if (
      e.status === 400 ||
      e.code === 'bad_request' ||
      e.message?.includes('некоррект') ||
      e.message?.includes('параметр')
    ) {
      return 'Не удалось сохранить запрос. Проверьте параметры.';
    }
  }
  return 'Не удалось выполнить действие. Попробуйте ещё раз.';
}

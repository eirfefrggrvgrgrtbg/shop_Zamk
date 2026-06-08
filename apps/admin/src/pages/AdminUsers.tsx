export function AdminUsers() {
  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Пользователи</h1>
      </div>
      
      <div className="bg-gray-50 border-l-4 border-gray-300 p-4">
        <p className="text-sm text-gray-700">
          Управление пользователями пока не подключено к API.
        </p>
      </div>

      <div className="rounded-lg border border-dashed border-gray-300 bg-white p-8 text-center">
        <p className="text-lg font-semibold text-gray-900">Нет данных</p>
        <p className="mt-2 text-sm text-gray-500">Раздел будет доступен позже.</p>
      </div>
    </div>
  );
}

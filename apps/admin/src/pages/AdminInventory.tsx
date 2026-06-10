import { useEffect, useState } from 'react';
import { AlertCircle, Boxes } from 'lucide-react';
import {
  adjustInventoryStock,
  getAdminInventory,
  getAdminInventoryErrorMessage,
  getAdminInventoryItem,
  getAdminInventoryMovements,
  receiveInventoryStock,
  writeOffInventoryStock,
} from '../api/adminInventory';
import type { AdminInventoryMovementView, AdminInventoryView } from '../api/adminInventory';
import { PermissionGuard } from '../components/PermissionGuard';

type InventoryAction = 'receipt' | 'adjustment' | 'write_off';

const actionLabels: Record<InventoryAction, string> = {
  receipt: 'Приёмка',
  adjustment: 'Корректировка',
  write_off: 'Списание',
};

export function AdminInventory() {
  const [inventory, setInventory] = useState<AdminInventoryView[]>([]);
  const [selectedItem, setSelectedItem] = useState<AdminInventoryView | null>(null);
  const [movements, setMovements] = useState<AdminInventoryMovementView[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isMovementsLoading, setIsMovementsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [action, setAction] = useState<InventoryAction>('receipt');
  const [quantity, setQuantity] = useState('');
  const [reason, setReason] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const fetchInventory = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminInventory();
      setInventory(data);
      if (selectedItem) {
        const refreshed = data.find((item) => item.id === selectedItem.id) ?? null;
        setSelectedItem(refreshed);
      }
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Не удалось загрузить остатки.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchMovements = async (item: AdminInventoryView) => {
    try {
      setIsMovementsLoading(true);
      setError(null);
      const detail = await getAdminInventoryItem(item.id);
      setSelectedItem(detail);
      const data = await getAdminInventoryMovements(item.id);
      setMovements(data);
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Не удалось загрузить движения остатков.'));
    } finally {
      setIsMovementsLoading(false);
    }
  };

  useEffect(() => {
    fetchInventory();
  }, []);

  const resetForm = () => {
    setQuantity('');
    setReason('');
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!selectedItem) return;

    const parsedQuantity = Number(quantity);
    if (!Number.isFinite(parsedQuantity) || parsedQuantity === 0) {
      setError('Количество должно быть ненулевым числом.');
      return;
    }
    if ((action === 'receipt' || action === 'write_off') && parsedQuantity <= 0) {
      setError('Для приёмки и списания количество должно быть больше нуля.');
      return;
    }
    if (action === 'write_off' && !window.confirm('Вы уверены, что хотите списать этот товар?')) {
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      const input = {
        productVariantId: selectedItem.productVariantId,
        quantity: parsedQuantity,
        reason,
      };
      if (action === 'receipt') await receiveInventoryStock(input);
      if (action === 'adjustment') await adjustInventoryStock(input);
      if (action === 'write_off') await writeOffInventoryStock(input);
      resetForm();
      await fetchInventory();
      await fetchMovements(selectedItem);
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, `Не удалось выполнить действие: ${actionLabels[action].toLowerCase()}.`));
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Остатки / Склад</h1>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Загрузка остатков...</p>
        </div>
      ) : inventory.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Boxes className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Нет данных</h3>
          <p className="mt-1 text-sm text-gray-500">Записи об остатках пока отсутствуют.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Товар / Вариант</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Продавец</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Доступно</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Зарезервировано</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Всего</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {inventory.map((item) => (
                    <tr key={item.id}>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="text-sm font-medium text-gray-900">{item.productTitle}</div>
                        <div className="text-xs text-gray-500">{item.variant}</div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {item.sellerName || item.sellerId || '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          item.availableStock > 10 ? 'bg-green-100 text-green-800' : 
                          item.availableStock > 0 ? 'bg-yellow-100 text-yellow-800' : 
                          'bg-red-100 text-red-800'
                        }`}>
                          {item.availableStock}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {item.reservedStock}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                        {item.totalStock}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <button onClick={() => fetchMovements(item)} className="text-indigo-600 hover:text-indigo-900">Просмотр / Изменить</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
      )}

      {selectedItem && (
        <div className="grid gap-6 lg:grid-cols-2">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900">Операция с остатками</h2>
            <p className="mt-1 text-sm text-gray-500">Вариант: {selectedItem.variant}</p>
            <PermissionGuard
              permission={['inventory.receipt', 'inventory.adjust', 'inventory.write_off']}
              fallback={<p className="mt-4 text-sm text-gray-500">У вас нет прав для выполнения операций со складом.</p>}
            >
              <form onSubmit={handleSubmit} className="mt-4 space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">Действие</label>
                  <select value={action} onChange={(event) => setAction(event.target.value as InventoryAction)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                    <PermissionGuard permission="inventory.receipt"><option value="receipt">Приёмка</option></PermissionGuard>
                    <PermissionGuard permission="inventory.adjust"><option value="adjustment">Корректировка</option></PermissionGuard>
                    <PermissionGuard permission="inventory.write_off"><option value="write_off">Списание</option></PermissionGuard>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Количество</label>
                  <input required type="number" value={quantity} onChange={(event) => setQuantity(event.target.value)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
                  {action === 'adjustment' && <p className="mt-1 text-xs text-gray-500">Укажите положительную или отрицательную дельту. Бэкенд проверяет лимиты.</p>}
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Причина</label>
                  <textarea required={action !== 'receipt'} rows={3} value={reason} onChange={(event) => setReason(event.target.value)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
                </div>
                <button type="submit" disabled={isSubmitting} className="inline-flex justify-center rounded-md border border-transparent bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-50">
                  {isSubmitting ? 'Отправка...' : actionLabels[action]}
                </button>
              </form>
            </PermissionGuard>
          </div>

          <div className="bg-white shadow sm:rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900">Движения остатков</h2>
            {isMovementsLoading ? (
              <p className="mt-4 text-sm text-gray-500">Загрузка движений...</p>
            ) : movements.length === 0 ? (
              <p className="mt-4 text-sm text-gray-500">Нет данных о движениях для этого товара.</p>
            ) : (
              <div className="mt-4 overflow-hidden border border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Тип</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Кол-во</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Причина</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Создано</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200 bg-white">
                    {movements.map((movement) => (
                      <tr key={movement.id}>
                        <td className="px-4 py-2 text-sm text-gray-900">{movement.type}</td>
                        <td className="px-4 py-2 text-sm text-gray-900">{movement.quantity}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{movement.reason || movement.referenceType || '-'}</td>
                        <td className="px-4 py-2 text-sm text-gray-500">{formatDate(movement.createdAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

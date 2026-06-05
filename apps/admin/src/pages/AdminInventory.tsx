import { useEffect, useState } from 'react';
import { AlertCircle, Boxes } from 'lucide-react';
import {
  adjustInventoryStock,
  getAdminInventory,
  getAdminInventoryErrorMessage,
  getAdminInventoryMovements,
  receiveInventoryStock,
  writeOffInventoryStock,
} from '../api/adminInventory';
import type { AdminInventoryMovementView, AdminInventoryView } from '../api/adminInventory';

type InventoryAction = 'receipt' | 'adjustment' | 'write_off';

const actionLabels: Record<InventoryAction, string> = {
  receipt: 'Receive stock',
  adjustment: 'Adjust stock',
  write_off: 'Write off stock',
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
      setError(getAdminInventoryErrorMessage(err, 'Failed to load inventory.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchMovements = async (item: AdminInventoryView) => {
    try {
      setIsMovementsLoading(true);
      setError(null);
      setSelectedItem(item);
      const data = await getAdminInventoryMovements(item.id);
      setMovements(data);
    } catch (err: unknown) {
      setError(getAdminInventoryErrorMessage(err, 'Failed to load inventory movements.'));
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
      setError('Quantity must be a non-zero number.');
      return;
    }
    if ((action === 'receipt' || action === 'write_off') && parsedQuantity <= 0) {
      setError('Receipt and write-off quantity must be greater than zero.');
      return;
    }
    if (action === 'write_off' && !window.confirm('Are you sure you want to write off this stock?')) {
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
      setError(getAdminInventoryErrorMessage(err, `Failed to ${actionLabels[action].toLowerCase()}.`));
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Inventory</h1>
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
          <p className="mt-2 text-sm text-gray-500">Loading inventory...</p>
        </div>
      ) : inventory.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Boxes className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No inventory</h3>
          <p className="mt-1 text-sm text-gray-500">No stock records are available yet.</p>
        </div>
      ) : (
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Product / Variant</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Seller</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Available</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Reserved</th>
                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Total Stock</th>
                    <th scope="col" className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
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
                        <button onClick={() => fetchMovements(item)} className="text-indigo-600 hover:text-indigo-900">View / Adjust</button>
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
            <h2 className="text-lg font-medium text-gray-900">Inventory action</h2>
            <p className="mt-1 text-sm text-gray-500">Variant: {selectedItem.variant}</p>
            <form onSubmit={handleSubmit} className="mt-4 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Action</label>
                <select value={action} onChange={(event) => setAction(event.target.value as InventoryAction)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
                  <option value="receipt">Receive stock</option>
                  <option value="adjustment">Adjust stock</option>
                  <option value="write_off">Write off stock</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Quantity</label>
                <input required type="number" value={quantity} onChange={(event) => setQuantity(event.target.value)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
                {action === 'adjustment' && <p className="mt-1 text-xs text-gray-500">Use a positive or negative delta. Backend validates stock limits.</p>}
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Reason</label>
                <textarea required={action !== 'receipt'} rows={3} value={reason} onChange={(event) => setReason(event.target.value)} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              </div>
              <button type="submit" disabled={isSubmitting} className="inline-flex justify-center rounded-md border border-transparent bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-50">
                {isSubmitting ? 'Submitting...' : actionLabels[action]}
              </button>
            </form>
          </div>

          <div className="bg-white shadow sm:rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900">Movements</h2>
            {isMovementsLoading ? (
              <p className="mt-4 text-sm text-gray-500">Loading movements...</p>
            ) : movements.length === 0 ? (
              <p className="mt-4 text-sm text-gray-500">No movements for this inventory item.</p>
            ) : (
              <div className="mt-4 overflow-hidden border border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Type</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Qty</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Reason</th>
                      <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Created</th>
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

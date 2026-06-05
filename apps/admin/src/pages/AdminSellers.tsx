import { useState, useEffect } from 'react';
import { getAdminSellers, createAdminSeller, updateAdminSellerStatus } from '../api/adminOperations';
import type { AdminSeller } from '@zamk/api-client/src/types';
import { AlertCircle, Plus, CheckCircle2, Store } from 'lucide-react';

export function AdminSellers() {
  const [sellers, setSellers] = useState<AdminSeller[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [tempPassword, setTempPassword] = useState<string | null>(null);

  // Form state
  const [brandName, setBrandName] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [ownerName, setOwnerName] = useState('');
  const [ownerEmail, setOwnerEmail] = useState('');
  const [password, setPassword] = useState('');

  const fetchSellers = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminSellers();
      // Ensure data is an array (API might return items wrapped in { items: [] } depending on backend list response format, 
      // but according to our adapter and types it expects an array directly, wait let's check DTO: ListSellersResponse has Items)
      // Actually backend DTO says `ListSellersResponse { Items []Seller }` so we might need to handle both just in case.
      if (data && (data as any).items) {
        setSellers((data as any).items);
      } else {
        setSellers(data || []);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load sellers');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchSellers();
  }, []);

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    setCreateError(null);
    try {
      const data = {
        brandName,
        contactEmail,
        ownerName,
        ownerEmail,
        temporaryPassword: password
      };
      const res = await createAdminSeller(data);
      if (res.temporaryPassword) {
        setTempPassword(res.temporaryPassword);
      } else {
        setIsCreateModalOpen(false);
      }
      // Reset form
      setBrandName('');
      setContactEmail('');
      setOwnerName('');
      setOwnerEmail('');
      setPassword('');
      fetchSellers();
    } catch (err: any) {
      setCreateError(err.message || 'Failed to create seller');
    } finally {
      setIsCreating(false);
    }
  };

  const closeTempPasswordModal = () => {
    setTempPassword(null);
    setIsCreateModalOpen(false);
  };

  const handleStatusChange = async (id: string, newStatus: string) => {
    const confirmMsg = newStatus === 'blocked' || newStatus === 'archived' 
      ? `Are you sure you want to change seller status to ${newStatus}?` 
      : null;
      
    if (confirmMsg && !window.confirm(confirmMsg)) return;

    try {
      await updateAdminSellerStatus(id, newStatus);
      // Optimistic update
      setSellers(sellers.map(s => s.id === id ? { ...s, status: newStatus } : s));
    } catch (err: any) {
      alert(err.message || 'Failed to update status');
      fetchSellers();
    }
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Sellers</h1>
        <button 
          onClick={() => setIsCreateModalOpen(true)}
          className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
        >
          <Plus className="-ml-1 mr-2 h-5 w-5" />
          Create Seller
        </button>
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
          <p className="mt-2 text-sm text-gray-500">Loading sellers...</p>
        </div>
      ) : sellers.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Store className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No sellers</h3>
          <p className="mt-1 text-sm text-gray-500">Get started by creating a new seller.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name / ID</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                      <th scope="col" className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {sellers.map((seller) => (
                      <tr key={seller.id}>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm font-medium text-gray-900">{seller.name}</div>
                          <div className="text-xs text-gray-500">ID: {seller.id}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            seller.status === 'active' ? 'bg-green-100 text-green-800' : 
                            seller.status === 'blocked' ? 'bg-red-100 text-red-800' : 
                            'bg-yellow-100 text-yellow-800'
                          }`}>
                            {seller.status}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          {seller.status !== 'active' && (
                            <button onClick={() => handleStatusChange(seller.id, 'active')} className="text-green-600 hover:text-green-900 mr-4">Approve/Activate</button>
                          )}
                          {seller.status !== 'blocked' && (
                            <button onClick={() => handleStatusChange(seller.id, 'blocked')} className="text-red-600 hover:text-red-900">Block</button>
                          )}
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

      {/* Create Seller Modal */}
      {isCreateModalOpen && !tempPassword && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">Create New Seller</h2>
            
            {createError && (
              <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                <span>{createError}</span>
              </div>
            )}
            
            <form onSubmit={handleCreateSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Brand Name</label>
                <input required type="text" value={brandName} onChange={e => setBrandName(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Contact Email</label>
                <input required type="email" value={contactEmail} onChange={e => setContactEmail(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Owner Name</label>
                <input required type="text" value={ownerName} onChange={e => setOwnerName(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Owner Email</label>
                <input required type="email" value={ownerEmail} onChange={e => setOwnerEmail(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Temporary Password</label>
                <input required type="text" minLength={8} value={password} onChange={e => setPassword(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div className="mt-5 flex justify-end space-x-3">
                <button type="button" onClick={() => setIsCreateModalOpen(false)} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={isCreating} className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                  {isCreating ? 'Creating...' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Temporary Password Secure Alert Modal */}
      {tempPassword && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <div className="flex items-center text-green-600 mb-4">
              <CheckCircle2 className="h-8 w-8 mr-2" />
              <h2 className="text-xl font-bold">Seller Created</h2>
            </div>
            <p className="text-sm text-gray-600 mb-4">
              Please provide the following temporary credentials to the seller securely. This password will not be shown again.
            </p>
            <div className="bg-gray-100 p-4 rounded text-center mb-6 border border-gray-200">
              <code className="text-lg font-mono font-bold text-gray-900">{tempPassword}</code>
            </div>
            <button onClick={closeTempPasswordModal} className="w-full px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700">
              I have copied the password securely
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

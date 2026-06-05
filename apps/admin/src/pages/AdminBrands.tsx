import { useState, useEffect } from 'react';
import { getAdminBrands, createAdminBrand, uploadAdminBrandLogo } from '../api/adminOperations';
import type { Brand } from '@zamk/api-client/src/types';
import { AlertCircle, Plus, Tag, Upload } from 'lucide-react';

export function AdminBrands() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');

  const [uploadingBrandId, setUploadingBrandId] = useState<string | null>(null);

  const fetchBrands = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminBrands();
      if (data && (data as any).items) {
        setBrands((data as any).items);
      } else {
        setBrands(data || []);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load brands');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchBrands();
  }, []);

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    setCreateError(null);
    try {
      await createAdminBrand({ name, slug });
      setIsCreateModalOpen(false);
      setName('');
      setSlug('');
      fetchBrands();
    } catch (err: any) {
      setCreateError(err.message || 'Failed to create brand');
    } finally {
      setIsCreating(false);
    }
  };

  const handleLogoUpload = async (brandId: string, e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadingBrandId(brandId);
    try {
      await uploadAdminBrandLogo(brandId, file);
      fetchBrands();
    } catch (err: any) {
      alert(err.message || 'Failed to upload logo');
    } finally {
      setUploadingBrandId(null);
      e.target.value = '';
    }
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Brands</h1>
        <button 
          onClick={() => setIsCreateModalOpen(true)}
          className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
        >
          <Plus className="-ml-1 mr-2 h-5 w-5" />
          Create Brand
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
          <p className="mt-2 text-sm text-gray-500">Loading brands...</p>
        </div>
      ) : brands.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Tag className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No brands</h3>
          <p className="mt-1 text-sm text-gray-500">Get started by creating a new brand.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Brand</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Logo</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Slug</th>
                      <th scope="col" className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {brands.map((brand) => (
                      <tr key={brand.id}>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm font-medium text-gray-900">{brand.name}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          {brand.logoUrl ? (
                            <img src={brand.logoUrl} alt={brand.name} className="h-10 w-10 object-contain rounded" />
                          ) : (
                            <span className="text-sm text-gray-500">No logo</span>
                          )}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {brand.slug}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <div className="relative inline-block text-left">
                            <label className="cursor-pointer text-indigo-600 hover:text-indigo-900 flex items-center justify-end">
                              <Upload className="h-4 w-4 mr-1" />
                              {uploadingBrandId === brand.id ? 'Uploading...' : 'Upload Logo'}
                              <input 
                                type="file" 
                                className="hidden" 
                                accept="image/jpeg, image/png, image/webp" 
                                disabled={uploadingBrandId === brand.id}
                                onChange={(e) => handleLogoUpload(brand.id, e)} 
                              />
                            </label>
                          </div>
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

      {/* Create Brand Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">Create New Brand</h2>
            
            {createError && (
              <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                <span>{createError}</span>
              </div>
            )}
            
            <form onSubmit={handleCreateSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Brand Name</label>
                <input required type="text" value={name} onChange={e => { setName(e.target.value); setSlug(e.target.value.toLowerCase().replace(/\s+/g, '-')); }} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Slug</label>
                <input required type="text" value={slug} onChange={e => setSlug(e.target.value)} className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
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
    </div>
  );
}

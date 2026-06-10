import { useState, useEffect } from 'react';
import { getAdminCategories, createAdminCategory, getAdminBrands, createAdminBrand, uploadAdminBrandLogo } from '../api/adminOperations';
import type { Category, Brand } from '@zamk/api-client/src/types';
import { AlertCircle, Plus, LayoutGrid, Tag, Upload } from 'lucide-react';

type Tab = 'categories' | 'brands';

export function AdminCatalog() {
  const [activeTab, setActiveTab] = useState<Tab>('categories');

  // --- Categories state ---
  const [categories, setCategories] = useState<Category[]>([]);
  const [catLoading, setCatLoading] = useState(true);
  const [catError, setCatError] = useState<string | null>(null);
  const [catCreateOpen, setCatCreateOpen] = useState(false);
  const [catCreating, setCatCreating] = useState(false);
  const [catCreateError, setCatCreateError] = useState<string | null>(null);
  const [catName, setCatName] = useState('');
  const [catSlug, setCatSlug] = useState('');

  // --- Brands state ---
  const [brands, setBrands] = useState<Brand[]>([]);
  const [brandLoading, setBrandLoading] = useState(true);
  const [brandError, setBrandError] = useState<string | null>(null);
  const [brandCreateOpen, setBrandCreateOpen] = useState(false);
  const [brandCreating, setBrandCreating] = useState(false);
  const [brandCreateError, setBrandCreateError] = useState<string | null>(null);
  const [brandName, setBrandName] = useState('');
  const [brandSlug, setBrandSlug] = useState('');
  const [uploadingBrandId, setUploadingBrandId] = useState<string | null>(null);

  const fetchCategories = async () => {
    try {
      setCatLoading(true);
      setCatError(null);
      const data = await getAdminCategories();
      if (data && (data as any).items) {
        setCategories((data as any).items);
      } else {
        setCategories(data || []);
      }
    } catch (err: any) {
      setCatError(err.message || 'Не удалось загрузить категории');
    } finally {
      setCatLoading(false);
    }
  };

  const fetchBrands = async () => {
    try {
      setBrandLoading(true);
      setBrandError(null);
      const data = await getAdminBrands();
      if (data && (data as any).items) {
        setBrands((data as any).items);
      } else {
        setBrands(data || []);
      }
    } catch (err: any) {
      setBrandError(err.message || 'Не удалось загрузить бренды');
    } finally {
      setBrandLoading(false);
    }
  };

  useEffect(() => {
    fetchCategories();
    fetchBrands();
  }, []);

  const handleCreateCategory = async (e: React.FormEvent) => {
    e.preventDefault();
    setCatCreating(true);
    setCatCreateError(null);
    try {
      await createAdminCategory({ name: catName, slug: catSlug });
      setCatCreateOpen(false);
      setCatName('');
      setCatSlug('');
      fetchCategories();
    } catch (err: any) {
      setCatCreateError(err.message || 'Не удалось создать категорию');
    } finally {
      setCatCreating(false);
    }
  };

  const handleCreateBrand = async (e: React.FormEvent) => {
    e.preventDefault();
    setBrandCreating(true);
    setBrandCreateError(null);
    try {
      await createAdminBrand({ name: brandName, slug: brandSlug });
      setBrandCreateOpen(false);
      setBrandName('');
      setBrandSlug('');
      fetchBrands();
    } catch (err: any) {
      setBrandCreateError(err.message || 'Не удалось создать бренд');
    } finally {
      setBrandCreating(false);
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
      alert(err.message || 'Не удалось загрузить логотип');
    } finally {
      setUploadingBrandId(null);
      e.target.value = '';
    }
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Категории и бренды</h1>
      </div>

      {/* Tab switcher */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          <button
            onClick={() => setActiveTab('categories')}
            className={`py-4 px-1 border-b-2 text-sm font-medium ${
              activeTab === 'categories'
                ? 'border-indigo-500 text-indigo-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            <LayoutGrid className="inline h-4 w-4 mr-2" />
            Категории
          </button>
          <button
            onClick={() => setActiveTab('brands')}
            className={`py-4 px-1 border-b-2 text-sm font-medium ${
              activeTab === 'brands'
                ? 'border-indigo-500 text-indigo-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            <Tag className="inline h-4 w-4 mr-2" />
            Бренды
          </button>
        </nav>
      </div>

      {/* CATEGORIES TAB */}
      {activeTab === 'categories' && (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button
              onClick={() => setCatCreateOpen(true)}
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
            >
              <Plus className="-ml-1 mr-2 h-5 w-5" />
              Создать категорию
            </button>
          </div>

          {catError && (
            <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
              <AlertCircle className="h-5 w-5 mr-2" />
              {catError}
            </div>
          )}

          {catLoading ? (
            <div className="text-center py-10">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
              <p className="mt-2 text-sm text-gray-500">Загрузка категорий...</p>
            </div>
          ) : categories.length === 0 ? (
            <div className="text-center py-10 bg-white rounded-lg shadow">
              <LayoutGrid className="mx-auto h-12 w-12 text-gray-400" />
              <h3 className="mt-2 text-sm font-medium text-gray-900">Категорий нет</h3>
              <p className="mt-1 text-sm text-gray-500">Создайте первую категорию.</p>
            </div>
          ) : (
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Название</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Slug</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {categories.map((cat) => (
                    <tr key={cat.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{cat.name}</td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{cat.slug}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {catCreateOpen && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4">Создать категорию</h2>
                {catCreateError && (
                  <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                    <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                    <span>{catCreateError}</span>
                  </div>
                )}
                <form onSubmit={handleCreateCategory} className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Название категории</label>
                    <input
                      required
                      type="text"
                      value={catName}
                      onChange={e => { setCatName(e.target.value); setCatSlug(e.target.value.toLowerCase().replace(/\s+/g, '-')); }}
                      className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Slug</label>
                    <input
                      required
                      type="text"
                      value={catSlug}
                      onChange={e => setCatSlug(e.target.value)}
                      className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500"
                    />
                  </div>
                  <div className="mt-5 flex justify-end space-x-3">
                    <button type="button" onClick={() => setCatCreateOpen(false)} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Отмена</button>
                    <button type="submit" disabled={catCreating} className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                      {catCreating ? 'Создание...' : 'Создать'}
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      )}

      {/* BRANDS TAB */}
      {activeTab === 'brands' && (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button
              onClick={() => setBrandCreateOpen(true)}
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
            >
              <Plus className="-ml-1 mr-2 h-5 w-5" />
              Создать бренд
            </button>
          </div>

          {brandError && (
            <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
              <AlertCircle className="h-5 w-5 mr-2" />
              {brandError}
            </div>
          )}

          {brandLoading ? (
            <div className="text-center py-10">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
              <p className="mt-2 text-sm text-gray-500">Загрузка брендов...</p>
            </div>
          ) : brands.length === 0 ? (
            <div className="text-center py-10 bg-white rounded-lg shadow">
              <Tag className="mx-auto h-12 w-12 text-gray-400" />
              <h3 className="mt-2 text-sm font-medium text-gray-900">Брендов нет</h3>
              <p className="mt-1 text-sm text-gray-500">Создайте первый бренд.</p>
            </div>
          ) : (
            <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Бренд</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Логотип</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Slug</th>
                    <th className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {brands.map((brand) => (
                    <tr key={brand.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{brand.name}</td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {brand.logoUrl ? (
                          <img src={brand.logoUrl} alt={brand.name} className="h-10 w-10 object-contain rounded" />
                        ) : (
                          <span className="text-sm text-gray-400">Нет логотипа</span>
                        )}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{brand.slug}</td>
                      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                        <label className="cursor-pointer text-indigo-600 hover:text-indigo-900 flex items-center justify-end">
                          <Upload className="h-4 w-4 mr-1" />
                          {uploadingBrandId === brand.id ? 'Загрузка...' : 'Загрузить логотип'}
                          <input
                            type="file"
                            className="hidden"
                            accept="image/jpeg, image/png, image/webp"
                            disabled={uploadingBrandId === brand.id}
                            onChange={(e) => handleLogoUpload(brand.id, e)}
                          />
                        </label>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {brandCreateOpen && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4">Создать бренд</h2>
                {brandCreateError && (
                  <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                    <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                    <span>{brandCreateError}</span>
                  </div>
                )}
                <form onSubmit={handleCreateBrand} className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Название бренда</label>
                    <input
                      required
                      type="text"
                      value={brandName}
                      onChange={e => { setBrandName(e.target.value); setBrandSlug(e.target.value.toLowerCase().replace(/\s+/g, '-')); }}
                      className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Slug</label>
                    <input
                      required
                      type="text"
                      value={brandSlug}
                      onChange={e => setBrandSlug(e.target.value)}
                      className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500"
                    />
                  </div>
                  <div className="mt-5 flex justify-end space-x-3">
                    <button type="button" onClick={() => setBrandCreateOpen(false)} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Отмена</button>
                    <button type="submit" disabled={brandCreating} className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                      {brandCreating ? 'Создание...' : 'Создать'}
                    </button>
                  </div>
                </form>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

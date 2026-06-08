import { useState, useEffect } from 'react';
import { Package, AlertCircle } from 'lucide-react';
import {
  approveProduct,
  blockProduct,
  getAdminProductErrorMessage,
  getAdminProducts,
  hideProduct,
  publishProduct,
  rejectProduct,
  uploadAdminProductImage,
} from '../api/adminProducts';
import type { AdminProductView } from '../api/adminProducts';

export function AdminProducts() {
  const [products, setProducts] = useState<AdminProductView[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Reject/Block Modal State
  const [actionModal, setActionModal] = useState<{ isOpen: boolean, type: 'reject' | 'block' | null, productId: string | null }>({ isOpen: false, type: null, productId: null });
  const [actionComment, setActionComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [actionProductId, setActionProductId] = useState<string | null>(null);
  const [uploadingProductId, setUploadingProductId] = useState<string | null>(null);

  const fetchProducts = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminProducts();
      setProducts(data);
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось загрузить товары.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchProducts();
  }, []);

  const handleAction = async (id: string, action: 'publish' | 'hide' | 'approve') => {
    if (action === 'hide' && !window.confirm('Are you sure you want to hide this product?')) return;

    try {
      setError(null);
      setActionProductId(id);
      if (action === 'publish') await publishProduct(id);
      if (action === 'hide') await hideProduct(id);
      if (action === 'approve') await approveProduct(id);
      await fetchProducts();
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, `Не удалось выполнить действие с товаром: ${action}.`));
    } finally {
      setActionProductId(null);
    }
  };

  const submitModalAction = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!actionModal.productId || !actionModal.type) return;
    if (actionModal.type === 'block' && !window.confirm('Are you sure you want to block this product?')) return;

    setIsSubmitting(true);
    try {
      setError(null);
      if (actionModal.type === 'reject') {
        await rejectProduct(actionModal.productId, actionComment);
      } else if (actionModal.type === 'block') {
        await blockProduct(actionModal.productId, actionComment);
      }
      setActionModal({ isOpen: false, type: null, productId: null });
      setActionComment('');
      await fetchProducts();
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, `Не удалось выполнить действие с товаром: ${actionModal.type}.`));
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleImageUpload = async (productId: string, event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.currentTarget.files?.[0];
    if (!file) return;

    try {
      setError(null);
      setUploadingProductId(productId);
      await uploadAdminProductImage(productId, file);
      await fetchProducts();
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось загрузить изображение товара.'));
    } finally {
      setUploadingProductId(null);
      event.currentTarget.value = '';
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'published': return 'bg-green-100 text-green-800';
      case 'approved': return 'bg-blue-100 text-blue-800';
      case 'pending_moderation': return 'bg-yellow-100 text-yellow-800';
      case 'rejected': return 'bg-red-100 text-red-800';
      case 'blocked': return 'bg-red-200 text-red-900';
      case 'hidden': return 'bg-gray-100 text-gray-800';
      case 'draft': return 'bg-gray-200 text-gray-600';
      case 'out_of_stock': return 'bg-orange-100 text-orange-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  const formatPrice = (product: AdminProductView) => {
    return `${product.price.toFixed(2)} ${product.currency}`;
  };

  const formatDate = (value?: string) => {
    return value ? new Date(value).toLocaleDateString('ru-RU') : '-';
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">All Products</h1>
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
          <p className="mt-2 text-sm text-gray-500">Loading products...</p>
        </div>
      ) : products.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Package className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No products</h3>
          <p className="mt-1 text-sm text-gray-500">There are no products in the catalog yet.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Товар</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Продавец</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Category / Brand</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Price</th>
                      <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {products.map((product) => (
                      <tr key={product.id}>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            {product.image && (
                              <div className="flex-shrink-0 h-10 w-10 mr-4">
                                <img className="h-10 w-10 rounded-md object-cover" src={product.image} alt="" />
                              </div>
                            )}
                            <div>
                              <div className="text-sm font-medium text-gray-900">{product.title}</div>
                              <div className="text-xs text-gray-500">ID: {product.id}</div>
                              <div className="text-xs text-gray-500">Updated: {formatDate(product.updatedAt)}</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                          <div>{product.sellerName || product.sellerId || '-'}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                          <div>{product.category || '-'}</div>
                          <div className="text-xs text-gray-500">{product.brand || '-'}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(product.status)}`}>
                            {product.statusLabel}
                          </span>
                          {product.moderationComment && (
                            <div className="mt-1 max-w-xs truncate text-xs text-gray-500" title={product.moderationComment}>
                              {product.moderationComment}
                            </div>
                          )}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                          <div>{formatPrice(product)}</div>
                          {product.oldPrice !== undefined && (
                            <div className="text-xs text-gray-500 line-through">
                              {product.oldPrice.toFixed(2)} {product.currency}
                            </div>
                          )}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          {product.status === 'pending_moderation' && (
                            <>
                              <button disabled={actionProductId === product.id} onClick={() => handleAction(product.id, 'approve')} className="text-blue-600 hover:text-blue-900 disabled:opacity-50 mr-4">Approve</button>
                              <button onClick={() => setActionModal({ isOpen: true, type: 'reject', productId: product.id })} className="text-red-600 hover:text-red-900 mr-4">Reject</button>
                            </>
                          )}
                          {(product.status === 'approved' || product.status === 'hidden') && (
                            <button disabled={actionProductId === product.id} onClick={() => handleAction(product.id, 'publish')} className="text-green-600 hover:text-green-900 disabled:opacity-50 mr-4">Publish</button>
                          )}
                          {product.status === 'published' && (
                            <button disabled={actionProductId === product.id} onClick={() => handleAction(product.id, 'hide')} className="text-gray-600 hover:text-gray-900 disabled:opacity-50 mr-4">Hide</button>
                          )}
                          {product.status !== 'blocked' && product.status !== 'rejected' && (
                            <button onClick={() => setActionModal({ isOpen: true, type: 'block', productId: product.id })} className="text-red-600 hover:text-red-900">Block</button>
                          )}
                          <label className="ml-4 inline-flex cursor-pointer text-indigo-600 hover:text-indigo-900">
                            {uploadingProductId === product.id ? 'Uploading...' : 'Upload image'}
                            <input
                              type="file"
                              accept="image/*"
                              className="sr-only"
                              disabled={uploadingProductId === product.id}
                              onChange={(event) => handleImageUpload(product.id, event)}
                            />
                          </label>
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

      {/* Action Modal (Reject / Block) */}
      {actionModal.isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4 capitalize">{actionModal.type} Product</h2>
            <form onSubmit={submitModalAction} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">
                  Reason / Comment {actionModal.type === 'reject' && <span className="text-red-500">*</span>}
                </label>
                <textarea 
                  required={actionModal.type === 'reject'}
                  value={actionComment} 
                  onChange={e => setActionComment(e.target.value)} 
                  rows={3}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" 
                />
              </div>
              <div className="mt-5 flex justify-end space-x-3">
                <button type="button" onClick={() => setActionModal({ isOpen: false, type: null, productId: null })} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={isSubmitting} className="px-4 py-2 bg-red-600 text-white rounded-md text-sm font-medium hover:bg-red-700 disabled:opacity-50 capitalize">
                  {isSubmitting ? 'Submitting...' : actionModal.type}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

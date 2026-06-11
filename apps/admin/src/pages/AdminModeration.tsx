import { useState, useEffect } from 'react';
import { AlertCircle, ShieldAlert } from 'lucide-react';
import {
  approveProduct,
  getAdminProductErrorMessage,
  getModerationProducts,
  rejectProduct,
} from '../api/adminProducts';
import type { AdminProductView } from '../api/adminProducts';

export function AdminModeration() {
  const [products, setProducts] = useState<AdminProductView[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Reject Modal State
  const [rejectModal, setRejectModal] = useState<{ isOpen: boolean, productId: string | null }>({ isOpen: false, productId: null });
  const [rejectComment, setRejectComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const fetchModerationProducts = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getModerationProducts();
      setProducts(data);
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось загрузить товары на модерации.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchModerationProducts();
  }, []);

  const handleApprove = async (id: string) => {
    try {
      setError(null);
      await approveProduct(id);
      await fetchModerationProducts();
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось одобрить товар.'));
    }
  };

  const handleRejectSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rejectModal.productId) return;

    setIsSubmitting(true);
    try {
      setError(null);
      await rejectProduct(rejectModal.productId, rejectComment);
      setRejectModal({ isOpen: false, productId: null });
      setRejectComment('');
      await fetchModerationProducts();
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось отклонить товар.'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'pending_moderation': return 'bg-yellow-100 text-yellow-800';
      case 'approved': return 'bg-blue-100 text-blue-800';
      case 'published': return 'bg-green-100 text-green-800';
      case 'rejected':
      case 'blocked': return 'bg-red-100 text-red-800';
      case 'hidden': return 'bg-gray-100 text-gray-800';
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
        <h1 className="text-2xl font-bold text-gray-900">Очередь модерации</h1>
      </div>
      
      <div className="bg-blue-50 border-l-4 border-blue-400 p-4">
        <p className="text-sm text-blue-700">
          Backend moderation rules are the source of truth. The queue refreshes after each action.
        </p>
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
          <p className="mt-2 text-sm text-gray-500">Loading moderation queue...</p>
        </div>
      ) : products.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ShieldAlert className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">All clear!</h3>
          <p className="mt-1 text-sm text-gray-500">No products are currently pending moderation.</p>
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
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Price</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Submitted</th>
                      <th scope="col" className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {products.map((product) => (
                      <tr key={product.id}>
                        <td className="px-6 py-4">
                          <div className="flex items-start">
                            {product.image && (
                              <div className="flex-shrink-0 h-12 w-12 mr-4">
                                <img className="h-12 w-12 rounded-md object-cover" src={product.image} alt="" />
                              </div>
                            )}
                            <div>
                              <div className="text-sm font-medium text-gray-900">{product.title}</div>
                              <div className="text-xs text-gray-500">ID: {product.id}</div>
                              <div className="text-xs text-gray-500">Seller: {product.sellerName || product.sellerId || '-'}</div>
                              <div className="text-xs text-gray-500">
                                Category: {product.category || '-'} / Brand: {product.brand || '-'}
                              </div>
                              {product.description && (
                                <p className="mt-2 max-w-xl text-sm text-gray-600">{product.description}</p>
                              )}
                              {product.gallery.length > 0 && (
                                <div className="mt-2 flex flex-wrap gap-2">
                                  {product.gallery.slice(0, 4).map((image) => (
                                    <img key={image.id || image.url} className="h-10 w-10 rounded object-cover" src={image.url} alt={image.altText || product.title} />
                                  ))}
                                </div>
                              )}
                              {product.variants.length > 0 && (
                                <div className="mt-2 text-xs text-gray-500">
                                  Variants: {product.variants.map((variant) => variant.label).join(', ')}
                                </div>
                              )}
                              {product.moderationComment && (
                                <div className="mt-2 p-2 bg-red-50 border-l-2 border-red-500 text-sm text-red-700">
                                  <strong>Предыдущая причина отклонения:</strong> {product.moderationComment}
                                </div>
                              )}
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(product.status)}`}>
                            {product.statusLabel}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                          {formatPrice(product)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {formatDate(product.submittedAt || product.createdAt)}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button onClick={() => handleApprove(product.id)} className="text-blue-600 hover:text-blue-900 mr-4">Одобрить</button>
                          <button onClick={() => setRejectModal({ isOpen: true, productId: product.id })} className="text-red-600 hover:text-red-900">Отклонить</button>
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

      {/* Reject Modal */}
      {rejectModal.isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">Reject Product</h2>
            <form onSubmit={handleRejectSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">
                  Rejection Reason <span className="text-red-500">*</span>
                </label>
                <textarea 
                  required
                  value={rejectComment} 
                  onChange={e => setRejectComment(e.target.value)} 
                  rows={3}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" 
                  placeholder="Объясните, что продавцу нужно исправить..."
                />
              </div>
              <div className="mt-5 flex justify-end space-x-3">
                <button type="button" onClick={() => setRejectModal({ isOpen: false, productId: null })} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={isSubmitting} className="px-4 py-2 bg-red-600 text-white rounded-md text-sm font-medium hover:bg-red-700 disabled:opacity-50">
                  {isSubmitting ? 'Rejecting...' : 'Reject'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

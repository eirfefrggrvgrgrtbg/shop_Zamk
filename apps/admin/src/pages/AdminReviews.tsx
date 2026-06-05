import { useEffect, useState } from 'react';
import { AlertCircle, Star } from 'lucide-react';
import {
  getAdminReview,
  getAdminReviewErrorMessage,
  getAdminReviews,
  moderateAdminReview,
} from '../api/adminReviews';
import type { AdminReviewView, ReviewAction } from '../api/adminReviews';

export function AdminReviews() {
  const [reviews, setReviews] = useState<AdminReviewView[]>([]);
  const [selectedReview, setSelectedReview] = useState<AdminReviewView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [modal, setModal] = useState<{ action: ReviewAction; reviewId: string } | null>(null);
  const [comment, setComment] = useState('');

  const fetchReviews = async () => {
    try {
      setIsLoading(true);
      setError(null);
      setReviews(await getAdminReviews());
    } catch (err: unknown) {
      setError(getAdminReviewErrorMessage(err, 'Failed to load reviews.'));
    } finally {
      setIsLoading(false);
    }
  };

  const fetchReviewDetail = async (id: string) => {
    try {
      setIsDetailLoading(true);
      setError(null);
      setSelectedReview(await getAdminReview(id));
    } catch (err: unknown) {
      setError(getAdminReviewErrorMessage(err, 'Failed to load review details.'));
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchReviews();
  }, []);

  const submitAction = async (action: ReviewAction, reviewId: string, actionComment?: string) => {
    if ((action === 'hide' || action === 'block') && !window.confirm(`Are you sure you want to ${action} this review?`)) return;
    try {
      setIsSubmitting(true);
      setError(null);
      await moderateAdminReview(reviewId, action, actionComment);
      await fetchReviews();
      if (selectedReview?.id === reviewId) await fetchReviewDetail(reviewId);
      setModal(null);
      setComment('');
    } catch (err: unknown) {
      setError(getAdminReviewErrorMessage(err, `Failed to ${action} review.`));
    } finally {
      setIsSubmitting(false);
    }
  };

  const getStatusBadgeClass = (status: string) => {
    switch (status) {
      case 'published':
        return 'bg-green-100 text-green-800';
      case 'rejected':
      case 'blocked':
        return 'bg-red-100 text-red-800';
      case 'hidden':
        return 'bg-gray-100 text-gray-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString('ru-RU') : '-';

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Reviews</h1>
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
          <p className="mt-2 text-sm text-gray-500">Loading reviews...</p>
        </div>
      ) : reviews.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Star className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">No reviews</h3>
          <p className="mt-1 text-sm text-gray-500">No reviews are available yet.</p>
        </div>
      ) : (
        <div className="flex flex-col">
          <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
            <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
              <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Review</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Product</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Rating</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                      <th className="relative px-6 py-3"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {reviews.map((review) => (
                      <tr key={review.id}>
                        <td className="px-6 py-4">
                          <div className="text-sm font-medium text-gray-900">{review.title || review.id}</div>
                          <div className="max-w-xl truncate text-sm text-gray-500">{review.comment || '-'}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{review.productTitle || review.productId}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{review.rating}/5</td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(review.status)}`}>
                            {review.statusLabel}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                          <button onClick={() => fetchReviewDetail(review.id)} className="text-indigo-600 hover:text-indigo-900 mr-4">View</button>
                          {review.status === 'pending_moderation' && <button onClick={() => submitAction('approve', review.id)} className="text-green-600 hover:text-green-900 mr-4">Approve</button>}
                          {review.status === 'pending_moderation' && <button onClick={() => setModal({ action: 'reject', reviewId: review.id })} className="text-red-600 hover:text-red-900 mr-4">Reject</button>}
                          {review.status === 'published' && <button onClick={() => submitAction('hide', review.id)} className="text-gray-600 hover:text-gray-900 mr-4">Hide</button>}
                          {review.status !== 'blocked' && <button onClick={() => setModal({ action: 'block', reviewId: review.id })} className="text-red-600 hover:text-red-900">Block</button>}
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

      {selectedReview && (
        <div className="bg-white shadow sm:rounded-lg p-6">
          <div className="sm:flex sm:items-start sm:justify-between">
            <div>
              <h2 className="text-lg font-medium text-gray-900">Review details</h2>
              <p className="mt-1 text-sm text-gray-500">{selectedReview.id}</p>
            </div>
            {isDetailLoading && <span className="text-sm text-gray-500">Loading...</span>}
          </div>
          <dl className="mt-4 grid gap-4 md:grid-cols-4">
            <div><dt className="text-sm font-medium text-gray-500">Product</dt><dd className="mt-1 text-sm text-gray-900">{selectedReview.productTitle || selectedReview.productId}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Seller</dt><dd className="mt-1 text-sm text-gray-900">{selectedReview.sellerName || selectedReview.sellerId || '-'}</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Rating</dt><dd className="mt-1 text-sm text-gray-900">{selectedReview.rating}/5</dd></div>
            <div><dt className="text-sm font-medium text-gray-500">Created</dt><dd className="mt-1 text-sm text-gray-900">{formatDate(selectedReview.createdAt)}</dd></div>
            <div className="md:col-span-2"><dt className="text-sm font-medium text-gray-500">Comment</dt><dd className="mt-1 text-sm text-gray-900">{selectedReview.comment || '-'}</dd></div>
            <div className="md:col-span-2"><dt className="text-sm font-medium text-gray-500">Moderation comment</dt><dd className="mt-1 text-sm text-gray-900">{selectedReview.moderationComment || '-'}</dd></div>
          </dl>
        </div>
      )}

      {modal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4 capitalize">{modal.action} Review</h2>
            <form onSubmit={(event) => { event.preventDefault(); submitAction(modal.action, modal.reviewId, comment); }} className="space-y-4">
              <textarea required={modal.action === 'reject' || modal.action === 'block'} value={comment} onChange={(event) => setComment(event.target.value)} rows={3} placeholder="Reason / comment" className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
              <div className="flex justify-end space-x-3">
                <button type="button" onClick={() => setModal(null)} className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
                <button type="submit" disabled={isSubmitting} className="px-4 py-2 bg-red-600 text-white rounded-md text-sm font-medium hover:bg-red-700 disabled:opacity-50 capitalize">
                  {isSubmitting ? 'Submitting...' : modal.action}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

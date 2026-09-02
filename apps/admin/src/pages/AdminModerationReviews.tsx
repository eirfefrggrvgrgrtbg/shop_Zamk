import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ReviewDetailOverlay } from './ReviewDetailOverlay';

import {
  Star,
  CheckCircle2,
  XCircle,
  EyeOff,
  ShieldAlert,
  Search,
  AlertCircle,
  MessageSquare,
  Clock,
  MoreVertical,
} from 'lucide-react';
import {
  getAdminReviews,
  getAdminReview,
  moderateAdminReview,
  getAdminReviewErrorMessage,
} from '../api/adminReviews';
import type { AdminReviewView, ReviewAction } from '../api/adminReviews';

const REJECTION_PRESETS = [
  'Не относится к товару',
  'Оскорбления / недопустимый текст',
  'Спам / реклама',
  'Персональные данные',
  'Другое',
];

export function AdminModerationReviews() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [reviews, setReviews] = useState<AdminReviewView[]>([]);
  const [selectedReviewId, setSelectedReviewId] = useState<string | null>(searchParams.get('selected'));
  const [selectedReview, setSelectedReview] = useState<AdminReviewView | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isOverlayOpen, setIsOverlayOpen] = useState(searchParams.get('view') === 'detail');

  useEffect(() => {
    setIsOverlayOpen(searchParams.get('view') === 'detail');
  }, [searchParams]);

  const openOverlay = (id: string) => {
    handleSelectReview(id);
    const next = new URLSearchParams(searchParams);
    next.set('selected', id);
    next.set('view', 'detail');
    setSearchParams(next, { replace: true });
  };

  const closeOverlay = () => {
    const next = new URLSearchParams(searchParams);
    next.delete('view');
    setSearchParams(next, { replace: true });
    setIsOverlayOpen(false);
  };


  // Filters
  const [statusFilter, setStatusFilter] = useState<'pending' | 'all' | 'published' | 'rejected' | 'hidden_blocked'>('pending');
  const [searchQuery, setSearchQuery] = useState('');

  // Rejection / Action Modal
  const [modal, setModal] = useState<{ action: ReviewAction; reviewId: string; title: string } | null>(null);
  const [comment, setComment] = useState('');
  const [showMoreActions, setShowMoreActions] = useState(false);

  const fetchReviews = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminReviews();
      setReviews(data);

      const currentSelected = searchParams.get('selected') || selectedReviewId;
      if (currentSelected) {
        const found = data.find((r) => r.id === currentSelected);
        if (found) {
          setSelectedReview(found);
          setSelectedReviewId(found.id);
        } else if (data.length > 0) {
          setSelectedReview(data[0]);
          setSelectedReviewId(data[0].id);
        }
      } else if (data.length > 0) {
        const firstPending = data.find((r) => r.status === 'pending_moderation') || data[0];
        setSelectedReview(firstPending);
        setSelectedReviewId(firstPending.id);
      }
    } catch (err: unknown) {
      setError(getAdminReviewErrorMessage(err, 'Не удалось загрузить отзывы.'));
    } finally {
      setIsLoading(false);
    }
  };

  const handleSelectReview = async (id: string) => {
    setSelectedReviewId(id);
    const next = new URLSearchParams(searchParams);
    next.set('selected', id);
    setSearchParams(next, { replace: true });

    const cached = reviews.find((r) => r.id === id);
    if (cached) setSelectedReview(cached);

    try {
      setIsDetailLoading(true);
      const detailed = await getAdminReview(id);
      setSelectedReview(detailed);
      setReviews((prev) => prev.map((r) => (r.id === id ? detailed : r)));
    } catch {
      // Keep cached if network detail fetch fails
    } finally {
      setIsDetailLoading(false);
    }
  };

  useEffect(() => {
    fetchReviews();
  }, []);

    const handleAction = async (action: ReviewAction, reviewId: string, actionComment?: string) => {
    try {
      setIsSubmitting(true);
      setError(null);
      await moderateAdminReview(reviewId, action, actionComment);
      await fetchReviews();
      setModal(null);
      setComment('');
      setShowMoreActions(false);
      
      // If we are in 'pending' queue and approved/rejected, close the overlay to move on
      if (statusFilter === 'pending' && (action === 'approve' || action === 'reject')) {
        closeOverlay();
      }
    } catch (err: unknown) {
      setError(getAdminReviewErrorMessage(err, `Не удалось выполнить действие с отзывом: ${action}.`));
    } finally {
      setIsSubmitting(false);
    }
  };

  // Filter reviews
  const filteredReviews = reviews.filter((r) => {
    if (statusFilter === 'pending' && r.status !== 'pending_moderation') return false;
    if (statusFilter === 'published' && r.status !== 'published') return false;
    if (statusFilter === 'rejected' && r.status !== 'rejected') return false;
    if (statusFilter === 'hidden_blocked' && r.status !== 'hidden' && r.status !== 'blocked') return false;

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      const matchProduct = r.productTitle?.toLowerCase().includes(q);
      const matchTitle = r.title?.toLowerCase().includes(q);
      const matchComment = r.comment?.toLowerCase().includes(q);
      const matchSeller = r.sellerName?.toLowerCase().includes(q);
      if (!matchProduct && !matchTitle && !matchComment && !matchSeller) return false;
    }

    return true;
  });

  const getStatusBadge = (status: string, label: string) => {
    switch (status) {
      case 'pending_moderation':
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200/60 dark:border-amber-800/40">На проверке</span>;
      case 'published':
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300 border border-emerald-200/60 dark:border-emerald-800/40">Опубликован</span>;
      case 'rejected':
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-100 text-rose-800 dark:bg-rose-950/60 dark:text-rose-300 border border-rose-200/60 dark:border-rose-800/40">Отклонён</span>;
      case 'hidden':
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-gray-100 text-gray-800 dark:bg-slate-800 dark:text-slate-300 border border-gray-200 dark:border-slate-700">Скрыт</span>;
      case 'blocked':
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300 border border-red-200 dark:border-red-800">Заблокирован</span>;
      default:
        return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800">{label || status}</span>;
    }
  };

  const formatDate = (val?: string) => {
    if (!val) return '—';
    return new Date(val).toLocaleDateString('ru-RU', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div className="space-y-6">
      {/* Breadcrumb Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white dark:bg-slate-900 p-6 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs">
        <div>
          <div className="flex items-center gap-2 text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-1">
            <span>Модерация</span>
            <span>/</span>
            <span className="text-indigo-600 dark:text-indigo-400">Отзывы</span>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Модерация отзывов</h1>
          <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
            Проверка отзывов покупателей перед публикацией в каталоге
          </p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded-xl flex items-center gap-3">
          <AlertCircle className="h-5 w-5 flex-shrink-0" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* Filter Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-white dark:bg-slate-900 p-4 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs">
        {/* Status Filters */}
        <div className="flex items-center gap-1.5 overflow-x-auto scrollbar-hide py-0.5">
          <button
            type="button"
            onClick={() => setStatusFilter('pending')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'pending'
                ? 'bg-amber-500 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            На проверке ({reviews.filter((r) => r.status === 'pending_moderation').length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('all')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'all'
                ? 'bg-indigo-600 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Все ({reviews.length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('published')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'published'
                ? 'bg-emerald-600 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Опубликованные ({reviews.filter((r) => r.status === 'published').length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('rejected')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'rejected'
                ? 'bg-rose-600 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Отклонённые ({reviews.filter((r) => r.status === 'rejected').length})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter('hidden_blocked')}
            className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
              statusFilter === 'hidden_blocked'
                ? 'bg-slate-700 text-white shadow-xs font-semibold'
                : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200 dark:hover:bg-slate-700'
            }`}
          >
            Скрытые / Блок ({reviews.filter((r) => r.status === 'hidden' || r.status === 'blocked').length})
          </button>
        </div>

        {/* Search */}
        <div className="relative min-w-[240px]">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Поиск по товару, тексту..."
            className="w-full pl-9 pr-4 py-1.5 text-xs bg-gray-50 dark:bg-slate-800 border border-gray-200 dark:border-slate-700 rounded-xl text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
        </div>
      </div>

      {/* Master-Detail Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Left: Queue List */}
        <div className="lg:col-span-5 space-y-3">
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-28 rounded-2xl bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-800 animate-pulse p-4" />
              ))}
            </div>
          ) : filteredReviews.length === 0 ? (
            <div className="bg-white dark:bg-slate-900 rounded-2xl p-10 text-center border border-gray-200 dark:border-slate-800">
              <MessageSquare className="w-10 h-10 text-gray-300 dark:text-slate-600 mx-auto mb-3" />
              <h3 className="text-sm font-semibold text-gray-800 dark:text-slate-200">Нет отзывов</h3>
              <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                {statusFilter === 'pending' ? 'Все отзывы проверены модератором.' : 'Отзывы с выбранными фильтрами не найдены.'}
              </p>
            </div>
          ) : (
            <div className="space-y-2.5">
              {filteredReviews.map((review) => {
                const isSelected = selectedReview?.id === review.id;
                return (
                  <div
                    key={review.id}
                    onClick={() => handleSelectReview(review.id)}
                    className={`p-4 rounded-2xl border transition-all cursor-pointer text-left ${
                      isSelected
                        ? 'bg-indigo-50/70 dark:bg-indigo-950/40 border-indigo-500 dark:border-indigo-400 shadow-sm ring-1 ring-indigo-500 dark:ring-indigo-400'
                        : 'bg-white dark:bg-slate-900 hover:bg-gray-50 dark:hover:bg-slate-800/80 border-gray-200 dark:border-slate-800 shadow-2xs'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2 mb-1.5">
                      <div className="flex items-center gap-1 text-amber-500">
                        {Array.from({ length: 5 }).map((_, i) => (
                          <Star
                            key={i}
                            className={`w-3.5 h-3.5 ${i < review.rating ? 'fill-current' : 'text-gray-200 dark:text-slate-700'}`}
                          />
                        ))}
                        <span className="text-xs font-bold ml-1 text-gray-700 dark:text-slate-300">{review.rating}/5</span>
                      </div>
                      {getStatusBadge(review.status, review.statusLabel)}
                    </div>

                    <h4 className="text-sm font-semibold text-gray-900 dark:text-white truncate">
                      {review.productTitle || 'Товар ZAMK'}
                    </h4>

                    {review.title && (
                      <p className="text-xs font-medium text-gray-800 dark:text-slate-200 mt-0.5 line-clamp-1">
                        {review.title}
                      </p>
                    )}

                    {review.comment && (
                      <p className="text-xs text-gray-500 dark:text-slate-400 mt-1 line-clamp-2 leading-relaxed">
                        {review.comment}
                      </p>
                    )}

                    <div className="flex items-center justify-between text-[11px] text-gray-400 dark:text-slate-500 mt-2.5 pt-2 border-t border-gray-100 dark:border-slate-800/80">
                      <span>{review.sellerName || 'ZAMK Store'}</span>
                      <span>{review.createdAt ? new Date(review.createdAt).toLocaleDateString('ru-RU') : ''}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Right: Detail & Action Pane */}
        <div className="lg:col-span-7 sticky top-6">
          {selectedReview ? (
            <div className="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-6 shadow-sm space-y-6">
              {/* Detail Header */}
              <div className="flex items-start justify-between gap-4 pb-5 border-b border-gray-100 dark:border-slate-800">
                <div>
                  <span className="text-xs uppercase tracking-wider font-semibold text-indigo-600 dark:text-indigo-400">
                    Отзыв покупателя
                  </span>
                  <h3 className="text-lg font-bold text-gray-900 dark:text-white mt-1">
                    {selectedReview.productTitle || 'Товар ZAMK'}
                  </h3>
                  {selectedReview.sellerName && (
                    <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">
                      Продавец: <span className="font-medium text-gray-700 dark:text-slate-300">{selectedReview.sellerName}</span>
                    </p>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {isDetailLoading && <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-indigo-600" />}
                  {getStatusBadge(selectedReview.status, selectedReview.statusLabel)}
                  <button 
                    onClick={() => openOverlay(selectedReview.id)}
                    className="ml-2 inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-50 hover:bg-indigo-100 text-indigo-700 dark:bg-indigo-500/10 dark:hover:bg-indigo-500/20 dark:text-indigo-400 text-xs font-semibold rounded-lg transition-colors"
                  >
                    Открыть подробно
                  </button>
                </div>
              </div>

              {/* Review Content */}
              <div className="space-y-4 bg-gray-50/70 dark:bg-slate-800/40 p-5 rounded-xl border border-gray-100 dark:border-slate-800">
                {/* Rating */}
                <div className="flex items-center gap-2">
                  <div className="flex items-center text-amber-500">
                    {Array.from({ length: 5 }).map((_, i) => (
                      <Star
                        key={i}
                        className={`w-5 h-5 ${i < selectedReview.rating ? 'fill-current' : 'text-gray-300 dark:text-slate-700'}`}
                      />
                    ))}
                  </div>
                  <span className="text-sm font-bold text-gray-900 dark:text-white">
                    {selectedReview.rating} из 5
                  </span>
                </div>

                {/* Title */}
                {selectedReview.title && (
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-500">Заголовок</p>
                    <p className="text-base font-semibold text-gray-900 dark:text-white mt-0.5">
                      {selectedReview.title}
                    </p>
                  </div>
                )}

                {/* Text */}
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-500">Текст отзыва</p>
                  <p className="text-sm text-gray-800 dark:text-slate-200 mt-1 leading-relaxed whitespace-pre-wrap">
                    {selectedReview.comment || 'Текст отзыва отсутствует'}
                  </p>
                </div>

                {/* Date */}
                <div className="flex items-center gap-1.5 text-xs text-gray-500 dark:text-slate-400 pt-2">
                  <Clock className="w-3.5 h-3.5" />
                  <span>Отправлен: {formatDate(selectedReview.createdAt)}</span>
                </div>
              </div>

              {/* Rejection comment history if rejected */}
              {selectedReview.status === 'rejected' && selectedReview.moderationComment && (
                <div className="p-4 bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-800/60 rounded-xl">
                  <p className="text-xs font-semibold text-rose-800 dark:text-rose-300">Причина отклонения:</p>
                  <p className="text-sm text-rose-900 dark:text-rose-200 mt-1">{selectedReview.moderationComment}</p>
                </div>
              )}

              {/* Action Area */}
              <div className="pt-2 border-t border-gray-100 dark:border-slate-800">
                <p className="text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-3">
                  Решение модератора
                </p>

                <div className="flex flex-wrap items-center gap-3">
                  {selectedReview.status === 'pending_moderation' && (
                    <>
                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => handleAction('approve', selectedReview.id)}
                        className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                      >
                        <CheckCircle2 className="w-4 h-4" />
                        <span>Опубликовать</span>
                      </button>

                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => setModal({ action: 'reject', reviewId: selectedReview.id, title: 'Отклонить отзыв' })}
                        className="inline-flex items-center gap-2 px-5 py-2.5 bg-rose-600 hover:bg-rose-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                      >
                        <XCircle className="w-4 h-4" />
                        <span>Отклонить</span>
                      </button>

                      <div className="relative ml-auto">
                        <button
                          type="button"
                          onClick={() => setShowMoreActions((prev) => !prev)}
                          className="p-2.5 text-gray-500 hover:text-gray-800 dark:text-slate-400 dark:hover:text-white rounded-xl hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                          title="Дополнительные действия"
                        >
                          <MoreVertical className="w-5 h-5" />
                        </button>
                        {showMoreActions && (
                          <div className="absolute right-0 bottom-full mb-2 w-48 bg-white dark:bg-slate-800 border border-gray-200 dark:border-slate-700 rounded-xl shadow-lg py-1 z-10">
                            <button
                              type="button"
                              onClick={() => setModal({ action: 'block', reviewId: selectedReview.id, title: 'Заблокировать отзыв' })}
                              className="w-full text-left px-4 py-2 text-xs font-medium text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/40 flex items-center gap-2"
                            >
                              <ShieldAlert className="w-4 h-4" />
                              <span>Заблокировать отзыв</span>
                            </button>
                          </div>
                        )}
                      </div>
                    </>
                  )}

                  {selectedReview.status === 'published' && (
                    <>
                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => handleAction('hide', selectedReview.id)}
                        className="inline-flex items-center gap-2 px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-xl transition-colors disabled:opacity-50"
                      >
                        <EyeOff className="w-4 h-4" />
                        <span>Скрыть отзыв</span>
                      </button>

                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => setModal({ action: 'block', reviewId: selectedReview.id, title: 'Заблокировать отзыв' })}
                        className="inline-flex items-center gap-2 px-4 py-2 border border-rose-200 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-sm font-medium rounded-xl transition-colors disabled:opacity-50"
                      >
                        <ShieldAlert className="w-4 h-4" />
                        <span>Заблокировать отзыв</span>
                      </button>
                    </>
                  )}

                  {(selectedReview.status === 'rejected' || selectedReview.status === 'hidden') && (
                    <>
                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => handleAction('approve', selectedReview.id)}
                        className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                      >
                        <CheckCircle2 className="w-4 h-4" />
                        <span>Опубликовать отзыв</span>
                      </button>

                      <button
                        type="button"
                        disabled={isSubmitting}
                        onClick={() => setModal({ action: 'block', reviewId: selectedReview.id, title: 'Заблокировать отзыв' })}
                        className="inline-flex items-center gap-2 px-4 py-2 border border-rose-200 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-sm font-medium rounded-xl transition-colors disabled:opacity-50 ml-auto"
                      >
                        <ShieldAlert className="w-4 h-4" />
                        <span>Заблокировать отзыв</span>
                      </button>
                    </>
                  )}

                  {selectedReview.status === 'blocked' && (
                    <div className="text-xs text-gray-500 dark:text-slate-400">
                      Отзыв заблокирован в системе.
                    </div>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-12 text-center text-gray-400 dark:text-slate-500">
              <MessageSquare className="w-12 h-12 mx-auto mb-3 text-gray-300 dark:text-slate-600" />
              <p className="text-sm font-medium text-gray-600 dark:text-slate-400">
                Выберите отзыв из списка слева для просмотра деталей и принятия решения
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Rejection / Block Modal */}
      {modal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl p-6 w-full max-w-lg shadow-2xl border border-gray-200 dark:border-slate-800 space-y-4">
            <div>
              <h3 className="text-lg font-bold text-gray-900 dark:text-white">{modal.title}</h3>
              <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">
                {modal.action === 'reject'
                  ? 'Укажите причину отклонения. Комментарий будет сохранён и доступен покупателю.'
                  : 'Укажите причину блокировки отзыва.'}
              </p>
            </div>

            {/* Presets */}
            {modal.action === 'reject' && (
              <div>
                <p className="text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-2">Шаблоны причин:</p>
                <div className="flex flex-wrap gap-1.5">
                  {REJECTION_PRESETS.map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      onClick={() => setComment(preset)}
                      className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                        comment === preset
                          ? 'bg-indigo-50 border-indigo-400 text-indigo-700 dark:bg-indigo-950/60 dark:border-indigo-500 dark:text-indigo-300 font-semibold'
                          : 'bg-gray-50 dark:bg-slate-800 border-gray-200 dark:border-slate-700 text-gray-600 dark:text-slate-300 hover:bg-gray-100'
                      }`}
                    >
                      {preset}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Comment Textarea */}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!comment.trim()) return;
                handleAction(modal.action, modal.reviewId, comment.trim());
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs font-semibold text-gray-700 dark:text-slate-300 mb-1">
                  Комментарий модератора <span className="text-red-500">*</span>
                </label>
                <textarea
                  required
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  rows={3}
                  placeholder="Опишите причину отклонения..."
                  className="w-full p-3 text-sm rounded-xl border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setModal(null);
                    setComment('');
                  }}
                  className="px-4 py-2 border border-gray-300 dark:border-slate-700 rounded-xl text-sm font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting || !comment.trim()}
                  className="px-5 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-sm font-semibold transition-colors disabled:opacity-50 shadow-xs"
                >
                  {isSubmitting ? 'Сохранение...' : (modal.action === 'reject' ? 'Отклонить отзыв' : 'Заблокировать')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <ReviewDetailOverlay 
        isOpen={isOverlayOpen}
        onClose={closeOverlay}
        review={selectedReview}
        onAction={handleAction}
        isSubmitting={isSubmitting}
        getStatusBadge={getStatusBadge}
      />
    </div>
  );
}

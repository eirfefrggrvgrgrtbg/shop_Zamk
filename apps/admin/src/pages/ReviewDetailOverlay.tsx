import React, { useEffect, useState } from 'react';
import { X, Star, Clock, AlertCircle, ShoppingBag, Image as ImageIcon, ExternalLink, CheckCircle2, XCircle, ShieldAlert, EyeOff } from 'lucide-react';
import type { AdminReviewView, ReviewAction } from '../api/adminReviews';
import { getAdminProduct } from '../api/adminProducts';
import type { AdminProductView } from '../api/adminProducts';

interface ReviewDetailOverlayProps {
  isOpen: boolean;
  onClose: () => void;
  review: AdminReviewView | null;
  onAction: (action: ReviewAction, reviewId: string, comment?: string) => Promise<void>;
  isSubmitting: boolean;
  getStatusBadge: (status: string, label: string) => React.ReactNode;
}

const REJECTION_PRESETS = [
  'Не относится к товару',
  'Оскорбления / недопустимый текст',
  'Спам / реклама',
  'Персональные данные',
  'Другое',
];

export function ReviewDetailOverlay({ isOpen, onClose, review, onAction, isSubmitting, getStatusBadge }: ReviewDetailOverlayProps) {
  const [product, setProduct] = useState<AdminProductView | null>(null);
  const [isLoadingProduct, setIsLoadingProduct] = useState(false);
  const [rejectMode, setRejectMode] = useState<ReviewAction | null>(null);
  const [comment, setComment] = useState('');

  useEffect(() => {
    if (isOpen && review?.productId) {
      setRejectMode(null);
      setComment('');
      setIsLoadingProduct(true);
      getAdminProduct(review.productId)
        .then(setProduct)
        .catch(() => setProduct(null))
        .finally(() => setIsLoadingProduct(false));
    }
  }, [isOpen, review]);

  if (!isOpen || !review) return null;

  const handleAction = async (action: ReviewAction) => {
    if ((action === 'reject' || action === 'block') && !comment.trim()) {
      return; // Require comment
    }
    await onAction(action, review.id, comment);
    // Reset state after action
    setRejectMode(null);
    setComment('');
  };

  const formatDate = (dateString?: string) => {
    if (!dateString) return '';
    return new Date(dateString).toLocaleString('ru-RU', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div className="fixed inset-0 z-[100] flex justify-end" data-testid="review-detail-overlay">
      {/* Backdrop */}
      <div 
        className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity"
        onClick={onClose}
        data-testid="overlay-backdrop"
      />
      
      {/* Drawer Panel */}
      <div className="relative w-full max-w-4xl bg-white dark:bg-slate-900 h-full shadow-2xl flex flex-col animate-in slide-in-from-right duration-300 border-l border-gray-200 dark:border-slate-800">
        
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100 dark:border-slate-800 bg-white dark:bg-slate-900 z-10">
          <div className="flex items-center gap-3">
            <h2 className="text-lg font-bold text-gray-900 dark:text-white">Модерация отзыва</h2>
            {getStatusBadge(review.status, review.statusLabel)}
          </div>
          <button 
            onClick={onClose}
            aria-label="Закрыть подробный просмотр"
            className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-full hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content - Scrollable */}
        <div className="flex-1 overflow-y-auto p-6 bg-gray-50/50 dark:bg-slate-900">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
            
            {/* Left Col: Review */}
            <div className="space-y-6">
              <div className="bg-white dark:bg-slate-800 rounded-2xl p-6 border border-gray-200 dark:border-slate-700 shadow-sm">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-1 text-amber-500">
                    {Array.from({ length: 5 }).map((_, i) => (
                      <Star
                        key={i}
                        className={`w-5 h-5 ${i < review.rating ? 'fill-current' : 'text-gray-200 dark:text-slate-600'}`}
                      />
                    ))}
                    <span className="text-sm font-bold text-gray-900 dark:text-white ml-2">{review.rating} / 5</span>
                  </div>
                  <div className="flex items-center gap-1.5 text-xs text-gray-500">
                    <Clock className="w-3.5 h-3.5" />
                    {formatDate(review.createdAt)}
                  </div>
                </div>

                <div className="space-y-4">
                  {review.title && (
                    <div>
                      <p className="text-xs uppercase tracking-wider font-semibold text-gray-400 mb-1">Заголовок</p>
                      <h3 className="text-lg font-bold text-gray-900 dark:text-white">{review.title}</h3>
                    </div>
                  )}
                  
                  <div>
                    <p className="text-xs uppercase tracking-wider font-semibold text-gray-400 mb-1">Текст отзыва</p>
                    <div className="bg-gray-50 dark:bg-slate-900/50 p-4 rounded-xl text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap leading-relaxed border border-gray-100 dark:border-slate-800">
                      {review.comment || <span className="text-gray-400 italic">Без текста</span>}
                    </div>
                  </div>
                </div>

                {review.status === 'rejected' && review.moderationComment && (
                  <div className="mt-6 p-4 bg-rose-50 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-900/50 rounded-xl flex items-start gap-3">
                    <AlertCircle className="w-5 h-5 text-rose-600 dark:text-rose-400 shrink-0 mt-0.5" />
                    <div>
                      <p className="text-sm font-semibold text-rose-800 dark:text-rose-300">Причина отклонения</p>
                      <p className="text-sm text-rose-700 dark:text-rose-200 mt-1">{review.moderationComment}</p>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Right Col: Product Context & Actions */}
            <div className="space-y-6">
              <div className="bg-white dark:bg-slate-800 rounded-2xl p-6 border border-gray-200 dark:border-slate-700 shadow-sm">
                <div className="flex items-center gap-2 mb-4">
                  <ShoppingBag className="w-5 h-5 text-indigo-500" />
                  <h3 className="text-base font-bold text-gray-900 dark:text-white">Оригинальный товар</h3>
                </div>

                {isLoadingProduct ? (
                  <div className="animate-pulse flex space-x-4">
                    <div className="rounded-xl bg-gray-200 dark:bg-slate-700 h-24 w-24"></div>
                    <div className="flex-1 space-y-3 py-1">
                      <div className="h-4 bg-gray-200 dark:bg-slate-700 rounded w-3/4"></div>
                      <div className="h-4 bg-gray-200 dark:bg-slate-700 rounded w-1/2"></div>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-col sm:flex-row gap-5">
                    {/* Image */}
                    <div className="shrink-0 w-32 h-32 bg-gray-100 dark:bg-slate-900 rounded-xl overflow-hidden border border-gray-200 dark:border-slate-700 flex items-center justify-center">
                      {(product?.mainImageUrl || product?.image) ? (
                        <img 
                          src={product.mainImageUrl || product.image} 
                          alt={product.title}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <ImageIcon className="w-8 h-8 text-gray-300 dark:text-slate-600" />
                      )}
                    </div>
                    
                    {/* Info */}
                    <div className="flex flex-col justify-center space-y-2 flex-1">
                      <h4 className="font-semibold text-gray-900 dark:text-white leading-tight">
                        {product?.title || review.productTitle || 'Неизвестный товар'}
                      </h4>
                      
                      <div className="text-sm text-gray-600 dark:text-gray-400">
                        <span className="font-medium text-gray-900 dark:text-gray-300">Продавец:</span> {product?.sellerName || review.sellerName || '—'}
                      </div>
                      
                      {product?.priceCents !== undefined && (
                        <div className="text-sm text-gray-600 dark:text-gray-400">
                          <span className="font-medium text-gray-900 dark:text-gray-300">Цена:</span> {(product.priceCents / 100).toLocaleString('ru-RU')} ₽
                        </div>
                      )}

                      <div className="pt-2">
                        <a 
                          href={`/products/${review.productId}`} 
                          target="_blank" 
                          rel="noreferrer"
                          className="inline-flex items-center gap-1.5 text-xs font-semibold text-indigo-600 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-300"
                        >
                          <ExternalLink className="w-3.5 h-3.5" />
                          Смотреть товар
                        </a>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Moderation Actions Inside Right Col (Bottom) */}
              <div className="bg-white dark:bg-slate-800 rounded-2xl p-6 border border-gray-200 dark:border-slate-700 shadow-sm">
                <h3 className="text-sm uppercase tracking-wider font-semibold text-gray-400 mb-4">Решение модератора</h3>
                
                {rejectMode ? (
                  <div className="space-y-4 animate-in fade-in slide-in-from-top-2">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                        {rejectMode === 'reject' ? 'Причина отклонения' : 'Причина блокировки отзыва'} <span className="text-red-500">*</span>
                      </label>
                      <div className="flex flex-wrap gap-2 mb-3">
                        {REJECTION_PRESETS.map((preset) => (
                          <button
                            key={preset}
                            type="button"
                            onClick={() => setComment(preset)}
                            className="px-2.5 py-1 text-xs bg-gray-100 hover:bg-gray-200 dark:bg-slate-700 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-300 rounded-lg transition-colors border border-gray-200 dark:border-slate-600"
                          >
                            {preset}
                          </button>
                        ))}
                      </div>
                      <textarea
                        value={comment}
                        onChange={(e) => setComment(e.target.value)}
                        placeholder="Опишите причину..."
                        className="w-full h-24 px-3 py-2 text-sm bg-white dark:bg-slate-900 border border-gray-300 dark:border-slate-700 rounded-xl focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 dark:text-white"
                        required
                      />
                    </div>
                    <div className="flex gap-3">
                      <button
                        onClick={() => handleAction(rejectMode)}
                        disabled={isSubmitting || !comment.trim()}
                        className="flex-1 py-2.5 bg-rose-600 hover:bg-rose-700 text-white text-sm font-semibold rounded-xl transition-colors disabled:opacity-50"
                      >
                        {isSubmitting ? 'Сохранение...' : 'Подтвердить'}
                      </button>
                      <button
                        onClick={() => {
                          setRejectMode(null);
                          setComment('');
                        }}
                        disabled={isSubmitting}
                        className="flex-1 py-2.5 bg-gray-100 hover:bg-gray-200 dark:bg-slate-700 dark:hover:bg-slate-600 text-gray-700 dark:text-white text-sm font-semibold rounded-xl transition-colors"
                      >
                        Отмена
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-col gap-3">
                    {/* pending_moderation: Publish, Reject, Block */}
                    {review.status === 'pending_moderation' && (
                      <div className="flex flex-wrap items-center gap-3">
                        <button
                          type="button"
                          onClick={() => handleAction('approve')}
                          disabled={isSubmitting}
                          className="flex-1 inline-flex items-center justify-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                        >
                          <CheckCircle2 className="w-4 h-4" />
                          <span>Опубликовать</span>
                        </button>

                        <button
                          type="button"
                          onClick={() => setRejectMode('reject')}
                          disabled={isSubmitting}
                          className="flex-1 inline-flex items-center justify-center gap-2 px-5 py-2.5 bg-rose-600 hover:bg-rose-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                        >
                          <XCircle className="w-4 h-4" />
                          <span>Отклонить</span>
                        </button>

                        <button
                          type="button"
                          onClick={() => setRejectMode('block')}
                          disabled={isSubmitting}
                          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 border border-rose-200 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-sm font-medium rounded-xl transition-colors disabled:opacity-50"
                        >
                          <ShieldAlert className="w-4 h-4" />
                          <span>Заблокировать отзыв</span>
                        </button>
                      </div>
                    )}

                    {/* published: Hide, Block */}
                    {review.status === 'published' && (
                      <div className="flex flex-wrap items-center gap-3">
                        <button
                          type="button"
                          onClick={() => handleAction('hide')}
                          disabled={isSubmitting}
                          className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2.5 bg-gray-600 hover:bg-gray-700 text-white text-sm font-medium rounded-xl transition-colors disabled:opacity-50"
                        >
                          <EyeOff className="w-4 h-4" />
                          <span>Скрыть</span>
                        </button>

                        <button
                          type="button"
                          onClick={() => setRejectMode('block')}
                          disabled={isSubmitting}
                          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 border border-rose-200 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-sm font-medium rounded-xl transition-colors disabled:opacity-50"
                        >
                          <ShieldAlert className="w-4 h-4" />
                          <span>Заблокировать отзыв</span>
                        </button>
                      </div>
                    )}

                    {/* rejected & hidden: Publish, Block */}
                    {(review.status === 'rejected' || review.status === 'hidden') && (
                      <div className="flex flex-wrap items-center gap-3">
                        <button
                          type="button"
                          onClick={() => handleAction('approve')}
                          disabled={isSubmitting}
                          className="flex-1 inline-flex items-center justify-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                        >
                          <CheckCircle2 className="w-4 h-4" />
                          <span>Опубликовать</span>
                        </button>

                        <button
                          type="button"
                          onClick={() => setRejectMode('block')}
                          disabled={isSubmitting}
                          className="inline-flex items-center justify-center gap-2 px-4 py-2.5 border border-rose-200 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 text-sm font-medium rounded-xl transition-colors disabled:opacity-50 ml-auto"
                        >
                          <ShieldAlert className="w-4 h-4" />
                          <span>Заблокировать отзыв</span>
                        </button>
                      </div>
                    )}

                    {/* blocked: No actions */}
                    {review.status === 'blocked' && (
                      <div className="p-4 bg-gray-50 dark:bg-slate-900 rounded-xl border border-gray-200 dark:border-slate-800 text-center text-sm text-gray-500 dark:text-slate-400">
                        Отзыв заблокирован в системе.
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>

          </div>
        </div>
      </div>
    </div>
  );
}

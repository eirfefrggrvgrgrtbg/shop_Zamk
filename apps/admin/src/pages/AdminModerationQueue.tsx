import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Inbox,
  Star,
  CheckCircle2,
  XCircle,
  ExternalLink,
  AlertCircle,
  ArrowRight,
  ShieldCheck,
} from 'lucide-react';
import { getAdminReviews, moderateAdminReview } from '../api/adminReviews';
import { getModerationProducts } from '../api/adminProducts';
import { getAdminSellers } from '@zamk/api-client/src/admin';
import type { AdminReviewView } from '../api/adminReviews';

interface QueueItem {
  id: string;
  type: 'review' | 'product' | 'seller';
  typeLabel: string;
  title: string;
  subtitle: string;
  createdAt?: string;
  raw: any;
}

const REJECTION_PRESETS = [
  'Не относится к товару',
  'Оскорбления / недопустимый текст',
  'Спам / реклама',
  'Персональные данные',
  'Другое',
];

const getSellerStatusLabel = (status: string) => {
  switch (status) {
    case 'pending_setup':
      return 'Настройка магазина';
    case 'pending_review':
      return 'На проверке';
    case 'pending':
      return 'Ожидает решения';
    default:
      return status;
  }
};

export function AdminModerationQueue() {
  const [items, setItems] = useState<QueueItem[]>([]);
  const [selectedItem, setSelectedItem] = useState<QueueItem | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState<'all' | 'review' | 'product' | 'seller'>('all');

  // Rejection modal
  const [rejectionModal, setRejectionModal] = useState<{ open: boolean; item: QueueItem | null }>({ open: false, item: null });
  const [rejectionComment, setRejectionComment] = useState('');

  const loadQueue = async () => {
    try {
      setIsLoading(true);
      setError(null);

      const [reviewsRes, productsRes, sellersRes] = await Promise.allSettled([
        getAdminReviews(),
        getModerationProducts({ status: 'pending_moderation', limit: 50 }),
        getAdminSellers({ limit: 100 }),
      ]);

      const queue: QueueItem[] = [];

      // 1. Pending Reviews
      if (reviewsRes.status === 'fulfilled') {
        const rawReviews = Array.isArray(reviewsRes.value) ? reviewsRes.value : (reviewsRes.value as any)?.items || [];
        rawReviews
          .filter((r: any) => r.status === 'pending_moderation')
          .forEach((r: AdminReviewView) => {
            queue.push({
              id: `review-${r.id}`,
              type: 'review',
              typeLabel: 'Отзыв',
              title: r.productTitle ? `Отзыв на «${r.productTitle}»` : 'Отзыв на товар',
              subtitle: `${r.rating}/5 ★ · ${r.title || r.comment || 'Без заголовка'}`,
              createdAt: r.createdAt,
              raw: r,
            });
          });
      }

      // 2. Pending Products
      if (productsRes.status === 'fulfilled') {
        const rawProducts = productsRes.value?.items || [];
        rawProducts.forEach((p: any) => {
          queue.push({
            id: `product-${p.id}`,
            type: 'product',
            typeLabel: 'Товар',
            title: p.title || 'Новый товар',
            subtitle: `Продавец: ${p.sellerName || 'ZAMK Store'} · ${p.variantsCount || 1} вар.`,
            createdAt: p.submittedAt || p.createdAt,
            raw: p,
          });
        });
      }

      // 3. Pending Sellers (pending_setup, pending_review, pending)
      if (sellersRes.status === 'fulfilled') {
        const rawSellers = sellersRes.value?.items || [];
        rawSellers
          .filter((s: any) => s.status === 'pending' || s.status === 'pending_setup' || s.status === 'pending_review')
          .forEach((s: any) => {
            queue.push({
              id: `seller-${s.id}`,
              type: 'seller',
              typeLabel: 'Продавец',
              title: s.brandName || s.ownerName || 'Новый продавец',
              subtitle: `Владелец: ${s.ownerEmail || '—'} · ${getSellerStatusLabel(s.status)}`,
              createdAt: s.createdAt,
              raw: s,
            });
          });
      }

      // Sort by creation date descending
      queue.sort((a, b) => {
        const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
        const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
        return timeB - timeA;
      });

      setItems(queue);
      if (queue.length > 0) {
        setSelectedItem((prev) => (prev ? queue.find((q) => q.id === prev.id) || queue[0] : queue[0]));
      } else {
        setSelectedItem(null);
      }
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить общую очередь модерации.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadQueue();
  }, []);

  const handleApproveReview = async (reviewId: string) => {
    try {
      setIsSubmitting(true);
      await moderateAdminReview(reviewId, 'approve');
      await loadQueue();
    } catch (err: any) {
      setError(err?.message || 'Ошибка при публикации отзыва');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRejectReview = async (reviewId: string, commentText: string) => {
    try {
      setIsSubmitting(true);
      await moderateAdminReview(reviewId, 'reject', commentText);
      setRejectionModal({ open: false, item: null });
      setRejectionComment('');
      await loadQueue();
    } catch (err: any) {
      setError(err?.message || 'Ошибка при отклонении отзыва');
    } finally {
      setIsSubmitting(false);
    }
  };

  const filteredItems = items.filter((item) => {
    if (typeFilter === 'all') return true;
    return item.type === typeFilter;
  });

  const getTypeBadge = (type: string) => {
    switch (type) {
      case 'review':
        return <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold bg-amber-100 dark:bg-amber-950/60 text-amber-800 dark:text-amber-300 border border-amber-200 dark:border-amber-800/60">Отзыв</span>;
      case 'product':
        return <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold bg-indigo-100 dark:bg-indigo-950/60 text-indigo-800 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800/60">Товар</span>;
      case 'seller':
        return <span className="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold bg-emerald-100 dark:bg-emerald-950/60 text-emerald-800 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/60">Продавец</span>;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Breadcrumb Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white dark:bg-slate-900 p-6 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs">
        <div>
          <div className="flex items-center gap-2 text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-1">
            <span>Модерация</span>
            <span>/</span>
            <span className="text-indigo-600 dark:text-indigo-400">Очередь</span>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Очередь модерации</h1>
          <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
            Единый список задач модерации, требующих решения
          </p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 rounded-xl flex items-center gap-3">
          <AlertCircle className="h-5 w-5 flex-shrink-0" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* Queue Filters */}
      <div className="flex items-center gap-2 bg-white dark:bg-slate-900 p-3.5 rounded-2xl border border-gray-200 dark:border-slate-800 shadow-xs overflow-x-auto scrollbar-hide">
        <button
          type="button"
          onClick={() => setTypeFilter('all')}
          className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
            typeFilter === 'all'
              ? 'bg-slate-900 dark:bg-white text-white dark:text-slate-900 font-semibold shadow-xs'
              : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200'
          }`}
        >
          Вся очередь ({items.length})
        </button>
        <button
          type="button"
          onClick={() => setTypeFilter('review')}
          className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
            typeFilter === 'review'
              ? 'bg-amber-500 text-white font-semibold shadow-xs'
              : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200'
          }`}
        >
          Отзывы ({items.filter((i) => i.type === 'review').length})
        </button>
        <button
          type="button"
          onClick={() => setTypeFilter('product')}
          className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
            typeFilter === 'product'
              ? 'bg-indigo-600 text-white font-semibold shadow-xs'
              : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200'
          }`}
        >
          Товары ({items.filter((i) => i.type === 'product').length})
        </button>
        <button
          type="button"
          onClick={() => setTypeFilter('seller')}
          className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-colors whitespace-nowrap ${
            typeFilter === 'seller'
              ? 'bg-emerald-600 text-white font-semibold shadow-xs'
              : 'bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-400 hover:bg-gray-200'
          }`}
        >
          Продавцы ({items.filter((i) => i.type === 'seller').length})
        </button>
      </div>

      {/* Master-Detail Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Left: Queue List */}
        <div className="lg:col-span-5 space-y-3">
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-24 rounded-2xl bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-800 animate-pulse p-4" />
              ))}
            </div>
          ) : filteredItems.length === 0 ? (
            <div className="bg-white dark:bg-slate-900 rounded-2xl p-12 text-center border border-gray-200 dark:border-slate-800">
              <ShieldCheck className="w-12 h-12 text-emerald-500 mx-auto mb-3" />
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Очередь модерации пуста</h3>
              <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                Все заявки проверены. Новые объекты появятся здесь автоматически.
              </p>
            </div>
          ) : (
            <div className="space-y-2.5">
              {filteredItems.map((item) => {
                const isSelected = selectedItem?.id === item.id;
                return (
                  <div
                    key={item.id}
                    onClick={() => setSelectedItem(item)}
                    className={`p-4 rounded-2xl border transition-all cursor-pointer text-left ${
                      isSelected
                        ? 'bg-indigo-50/70 dark:bg-indigo-950/40 border-indigo-500 dark:border-indigo-400 shadow-sm ring-1 ring-indigo-500 dark:ring-indigo-400'
                        : 'bg-white dark:bg-slate-900 hover:bg-gray-50 dark:hover:bg-slate-800/80 border-gray-200 dark:border-slate-800 shadow-2xs'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2 mb-1.5">
                      {getTypeBadge(item.type)}
                      <span className="text-[11px] text-gray-400 dark:text-slate-500">
                        {item.createdAt ? new Date(item.createdAt).toLocaleDateString('ru-RU') : ''}
                      </span>
                    </div>

                    <h4 className="text-sm font-semibold text-gray-900 dark:text-white truncate">
                      {item.title}
                    </h4>

                    <p className="text-xs text-gray-500 dark:text-slate-400 mt-1 line-clamp-2 leading-relaxed">
                      {item.subtitle}
                    </p>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Right: Inspector Pane */}
        <div className="lg:col-span-7 sticky top-6">
          {selectedItem ? (
            <div className="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-6 shadow-sm space-y-6">
              {/* Header */}
              <div className="flex items-start justify-between gap-4 pb-5 border-b border-gray-100 dark:border-slate-800">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    {getTypeBadge(selectedItem.type)}
                    <span className="text-xs font-semibold text-amber-600 dark:text-amber-400">Требует решения</span>
                  </div>
                  <h3 className="text-lg font-bold text-gray-900 dark:text-white">
                    {selectedItem.title}
                  </h3>
                </div>
              </div>

              {/* Review Inspector */}
              {selectedItem.type === 'review' && (
                <div className="space-y-5">
                  <div className="bg-gray-50/70 dark:bg-slate-800/40 p-5 rounded-xl border border-gray-100 dark:border-slate-800 space-y-3">
                    <div className="flex items-center gap-2">
                      <div className="flex items-center text-amber-500">
                        {Array.from({ length: 5 }).map((_, i) => (
                          <Star
                            key={i}
                            className={`w-4 h-4 ${i < (selectedItem.raw.rating || 5) ? 'fill-current' : 'text-gray-300 dark:text-slate-700'}`}
                          />
                        ))}
                      </div>
                      <span className="text-sm font-bold text-gray-900 dark:text-white">
                        {selectedItem.raw.rating} из 5
                      </span>
                    </div>

                    {selectedItem.raw.title && (
                      <p className="text-sm font-semibold text-gray-900 dark:text-white">
                        {selectedItem.raw.title}
                      </p>
                    )}

                    <p className="text-sm text-gray-800 dark:text-slate-200 leading-relaxed whitespace-pre-wrap">
                      {selectedItem.raw.comment || selectedItem.raw.text || 'Текст отзыва отсутствует'}
                    </p>
                  </div>

                  <div className="flex items-center gap-3 pt-2">
                    <button
                      type="button"
                      disabled={isSubmitting}
                      onClick={() => handleApproveReview(selectedItem.raw.id)}
                      className="inline-flex items-center gap-2 px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                    >
                      <CheckCircle2 className="w-4 h-4" />
                      <span>Опубликовать</span>
                    </button>

                    <button
                      type="button"
                      disabled={isSubmitting}
                      onClick={() => setRejectionModal({ open: true, item: selectedItem })}
                      className="inline-flex items-center gap-2 px-5 py-2.5 bg-rose-600 hover:bg-rose-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs disabled:opacity-50"
                    >
                      <XCircle className="w-4 h-4" />
                      <span>Отклонить</span>
                    </button>

                    <Link
                      to={`/moderation/reviews?selected=${selectedItem.raw.id}`}
                      className="inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white ml-auto"
                    >
                      <span>Все отзывы</span>
                      <ArrowRight className="w-4 h-4" />
                    </Link>
                  </div>
                </div>
              )}

              {/* Product Inspector */}
              {selectedItem.type === 'product' && (
                <div className="space-y-5">
                  <div className="bg-gray-50/70 dark:bg-slate-800/40 p-5 rounded-xl border border-gray-100 dark:border-slate-800 space-y-2">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">
                      Товар отправлен продавцом на модерацию.
                    </p>
                    <p className="text-xs text-gray-500 dark:text-slate-400">
                      Проверьте описание, характеристики, фотографии 4:5 и варианты перед публикацией в каталоге.
                    </p>
                  </div>

                  <div className="flex items-center gap-3 pt-2">
                    <Link
                      to={`/moderation/products/${selectedItem.raw.id}`}
                      className="inline-flex items-center gap-2 px-5 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs"
                    >
                      <span>Перейти к проверке товара</span>
                      <ExternalLink className="w-4 h-4" />
                    </Link>

                    <Link
                      to="/moderation/products"
                      className="inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium text-gray-600 dark:text-slate-300 hover:text-gray-900 ml-auto"
                    >
                      <span>Все товары на модерации</span>
                      <ArrowRight className="w-4 h-4" />
                    </Link>
                  </div>
                </div>
              )}

              {/* Seller Inspector */}
              {selectedItem.type === 'seller' && (
                <div className="space-y-5">
                  <div className="bg-gray-50/70 dark:bg-slate-800/40 p-5 rounded-xl border border-gray-100 dark:border-slate-800 space-y-2">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">
                      Новый продавец ожидает проверки документов и активации в досье.
                    </p>
                    <p className="text-xs text-gray-500 dark:text-slate-400">
                      Текущий этап: <span className="font-semibold text-gray-800 dark:text-slate-200">{getSellerStatusLabel(selectedItem.raw.status)}</span>
                    </p>
                  </div>

                  <div className="flex items-center gap-3 pt-2">
                    <Link
                      to={`/sellers/${selectedItem.raw.id}`}
                      className="inline-flex items-center gap-2 px-5 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold rounded-xl transition-colors shadow-xs"
                    >
                      <span>Перейти к проверке продавца</span>
                      <ExternalLink className="w-4 h-4" />
                    </Link>

                    <Link
                      to="/moderation/sellers"
                      className="inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium text-gray-600 dark:text-slate-300 hover:text-gray-900 ml-auto"
                    >
                      <span>Все продавцы на модерации</span>
                      <ArrowRight className="w-4 h-4" />
                    </Link>
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="bg-white dark:bg-slate-900 rounded-2xl border border-gray-200 dark:border-slate-800 p-12 text-center text-gray-400 dark:text-slate-500">
              <Inbox className="w-12 h-12 mx-auto mb-3 text-gray-300 dark:text-slate-600" />
              <p className="text-sm font-medium text-gray-600 dark:text-slate-400">
                Выберите объект из очереди для просмотра деталей
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Review Rejection Modal */}
      {rejectionModal.open && rejectionModal.item && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl p-6 w-full max-w-lg shadow-2xl border border-gray-200 dark:border-slate-800 space-y-4">
            <div>
              <h3 className="text-lg font-bold text-gray-900 dark:text-white">Отклонить отзыв</h3>
              <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">
                Укажите причину отклонения. Комментарий будет виден покупателю.
              </p>
            </div>

            <div>
              <p className="text-xs font-semibold text-gray-400 dark:text-slate-500 uppercase tracking-wider mb-2">Шаблоны причин:</p>
              <div className="flex flex-wrap gap-1.5">
                {REJECTION_PRESETS.map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    onClick={() => setRejectionComment(preset)}
                    className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                      rejectionComment === preset
                        ? 'bg-indigo-50 border-indigo-400 text-indigo-700 dark:bg-indigo-950/60 dark:border-indigo-500 dark:text-indigo-300 font-semibold'
                        : 'bg-gray-50 dark:bg-slate-800 border-gray-200 dark:border-slate-700 text-gray-600 dark:text-slate-300 hover:bg-gray-100'
                    }`}
                  >
                    {preset}
                  </button>
                ))}
              </div>
            </div>

            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!rejectionComment.trim()) return;
                handleRejectReview(rejectionModal.item!.raw.id, rejectionComment.trim());
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-xs font-semibold text-gray-700 dark:text-slate-300 mb-1">
                  Комментарий модератора <span className="text-red-500">*</span>
                </label>
                <textarea
                  required
                  value={rejectionComment}
                  onChange={(e) => setRejectionComment(e.target.value)}
                  rows={3}
                  placeholder="Опишите причину отклонения..."
                  className="w-full p-3 text-sm rounded-xl border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setRejectionModal({ open: false, item: null });
                    setRejectionComment('');
                  }}
                  className="px-4 py-2 border border-gray-300 dark:border-slate-700 rounded-xl text-sm font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting || !rejectionComment.trim()}
                  className="px-5 py-2 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-sm font-semibold transition-colors disabled:opacity-50"
                >
                  {isSubmitting ? 'Сохранение...' : 'Отклонить отзыв'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

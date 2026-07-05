import { useState, useEffect, useCallback, useRef } from 'react';
import { HelpTooltip } from '../components/HelpTooltip';
import { Package, Search, AlertCircle, X, ChevronRight, CheckCircle, XCircle, EyeOff, Lock, Eye } from 'lucide-react';
import {
  approveProduct,
  blockProduct,
  getAdminProduct,
  getAdminProductErrorMessage,
  getAdminProductModerationHistory,
  getAdminProducts,
  hideProduct,
  publishProduct,
  rejectProduct,
} from '../api/adminProducts';
import type { AdminProductView } from '../api/adminProducts';
import type { ModerationHistoryResponse } from '@zamk/api-client/src/types';

const STATUS_BADGE: Record<string, string> = {
  published:          'bg-green-100 text-green-800',
  approved:           'bg-blue-100 text-blue-800',
  pending_moderation: 'bg-yellow-100 text-yellow-800',
  rejected:           'bg-red-100 text-red-800',
  blocked:            'bg-red-200 text-red-900',
  hidden:             'bg-gray-100 text-gray-800',
  draft:              'bg-gray-200 text-gray-600',
  out_of_stock:       'bg-orange-100 text-orange-800',
};

const STATUS_LABEL: Record<string, string> = {
  published:          'Опубликован',
  approved:           'Одобрен',
  pending_moderation: 'На модерации',
  rejected:           'Отклонён',
  blocked:            'Заблокирован',
  hidden:             'Скрыт',
  draft:              'Черновик',
  out_of_stock:       'Нет в наличии',
};

const statusBadge = (status: string) =>
  STATUS_BADGE[status] ?? 'bg-gray-100 text-gray-800';

const statusLabel = (status: string) =>
  STATUS_LABEL[status] ?? status;

const formatDate = (value?: string | null) =>
  value ? new Date(value).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' }) : '—';

const formatPrice = (cents: number, currency = 'RUB') =>
  `${(cents / 100).toFixed(2)} ${currency}`;

// ── Reject modal ───────────────────────────────────────────────────────────────

interface RejectModalProps {
  isOpen: boolean;
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => void;
}

function RejectModal({ isOpen, isSubmitting, onClose, onSubmit }: RejectModalProps) {
  const [reason, setReason] = useState('');

  useEffect(() => { if (isOpen) setReason(''); }, [isOpen]);

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!reason.trim()) return;
    onSubmit(reason.trim());
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-xl shadow-2xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Укажите причину отклонения</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <textarea
            autoFocus
            required
            rows={4}
            value={reason}
            onChange={e => setReason(e.target.value)}
            placeholder="Причина отклонения товара..."
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
          />
          <p className="text-xs text-gray-500">Поле обязательно. Продавец увидит эту причину.</p>
          <div className="flex justify-end gap-3">
            <button type="button" onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-300 rounded-lg hover:bg-gray-50">
              Отмена
            </button>
            <button type="submit" disabled={isSubmitting || !reason.trim()}
              className="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 disabled:opacity-50">
              {isSubmitting ? 'Отклоняем…' : 'Отклонить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Product detail drawer ──────────────────────────────────────────────────────

interface ProductDrawerProps {
  productId: string | null;
  onClose: () => void;
  onActionDone: () => void;
}

function ProductDrawer({ productId, onClose, onActionDone }: ProductDrawerProps) {
  const [product, setProduct] = useState<AdminProductView | null>(null);
  const [logs, setLogs] = useState<ModerationHistoryResponse['items']>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isLogsLoading, setIsLogsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [isActing, setIsActing] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [isRejectSubmitting, setIsRejectSubmitting] = useState(false);
  const drawerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!productId) { setProduct(null); setLogs([]); setError(null); setActionError(null); setActionSuccess(null); return; }
    let cancelled = false;
    setIsLoading(true); setError(null); setActionError(null); setActionSuccess(null);
    getAdminProduct(productId)
      .then(p => { if (!cancelled) setProduct(p); })
      .catch(() => { if (!cancelled) setError('Не удалось загрузить товар.'); })
      .finally(() => { if (!cancelled) setIsLoading(false); });
    setIsLogsLoading(true);
    getAdminProductModerationHistory(productId)
      .then(r => { if (!cancelled) setLogs(r.items ?? []); })
      .catch(() => { if (!cancelled) setLogs([]); })
      .finally(() => { if (!cancelled) setIsLogsLoading(false); });
    return () => { cancelled = true; };
  }, [productId]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (drawerRef.current && !drawerRef.current.contains(e.target as Node)) onClose();
  };

  if (!productId) return null;

  const doAction = async (label: string, action: () => Promise<void>) => {
    setIsActing(true); setActionError(null); setActionSuccess(null);
    try {
      await action();
      setActionSuccess(`Действие выполнено: ${label}`);
      const [p, r] = await Promise.all([getAdminProduct(productId), getAdminProductModerationHistory(productId)]);
      setProduct(p); setLogs(r.items ?? []);
      onActionDone();
    } catch (err) {
      setActionError(getAdminProductErrorMessage(err, `Не удалось выполнить: ${label}`));
    } finally { setIsActing(false); }
  };

  const handleRejectSubmit = async (reason: string) => {
    setIsRejectSubmitting(true);
    try {
      await rejectProduct(productId, reason);
      setRejectOpen(false); setActionSuccess('Товар отклонён.');
      const [p, r] = await Promise.all([getAdminProduct(productId), getAdminProductModerationHistory(productId)]);
      setProduct(p); setLogs(r.items ?? []);
      onActionDone();
    } catch (err) {
      setActionError(getAdminProductErrorMessage(err, 'Не удалось отклонить товар.'));
    } finally { setIsRejectSubmitting(false); }
  };

  const isPlatform = product?.source === 'auction_direct_sale';
  const status = product?.status ?? '';

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40" onClick={handleBackdropClick}>
        <div ref={drawerRef} className="absolute right-0 top-0 h-full w-full max-w-2xl bg-white shadow-2xl flex flex-col overflow-hidden" onClick={e => e.stopPropagation()}>

          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 shrink-0">
            <h2 className="text-lg font-semibold text-gray-900 truncate">
              {isLoading ? 'Загружаем товар…' : (product?.title ?? 'Товар')}
            </h2>
            <button onClick={onClose} className="p-1 rounded-md text-gray-500 hover:bg-gray-100" aria-label="Закрыть">
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Body */}
          <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">

            {isLoading && (
              <div className="flex flex-col items-center justify-center py-16 gap-3 text-gray-500">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
                <p className="text-sm">Загружаем товар…</p>
              </div>
            )}

            {error && !isLoading && (
              <div className="flex items-center gap-2 p-4 bg-red-50 text-red-700 rounded-lg text-sm">
                <AlertCircle className="w-5 h-5 shrink-0" />{error}
              </div>
            )}
            {actionError && (
              <div className="flex items-center gap-2 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
                <AlertCircle className="w-4 h-4 shrink-0" />{actionError}
              </div>
            )}
            {actionSuccess && (
              <div className="flex items-center gap-2 p-3 bg-green-50 text-green-700 rounded-lg text-sm">
                <CheckCircle className="w-4 h-4 shrink-0" />{actionSuccess}
              </div>
            )}

            {product && !isLoading && (
              <>
                {/* Image + basic */}
                <div className="flex gap-5">
                  {product.image ? (
                    <img src={product.image} alt={product.title} className="w-24 h-24 rounded-xl object-cover shrink-0 border border-gray-200" />
                  ) : (
                    <div className="w-24 h-24 rounded-xl bg-gray-100 flex items-center justify-center shrink-0">
                      <Package className="w-8 h-8 text-gray-400" />
                    </div>
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap gap-2 items-center mb-2">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusBadge(status)}`}>{statusLabel(status)}</span>
                      {isPlatform && (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">Платформенный товар (ZAMK)</span>
                      )}
                    </div>
                    <h3 className="font-semibold text-gray-900 text-sm">{product.title}</h3>
                    <p className="text-xs text-gray-500 mt-0.5">ID: {product.id}</p>
                  </div>
                </div>

                {/* Details grid */}
                <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                  {[
                    ['Продавец', product.sellerName || product.sellerId || '—'],
                    ['Источник', isPlatform ? 'ZAMK (auction_direct_sale)' : (product.source ?? 'seller')],
                    ['Категория', product.category ?? '—'],
                    ['Бренд', product.brand ?? '—'],
                    ['Создан', formatDate(product.createdAt)],
                    ['Обновлён', formatDate(product.updatedAt)],
                    ['На модерации', formatDate(product.submittedAt)],
                    ['Цена', formatPrice(product.price * 100, product.currency)],
                  ].map(([label, value]) => (
                    <div key={label}>
                      <span className="block text-xs font-medium text-gray-500 uppercase tracking-wide mb-0.5">{label}</span>
                      <span className="text-gray-900">{value}</span>
                    </div>
                  ))}
                </div>

                {product.description && (
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">Описание</p>
                    <p className="text-sm text-gray-700 whitespace-pre-line">{product.description}</p>
                  </div>
                )}

                {product.moderationComment && (
                  <div className="p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
                    <p className="text-xs font-medium text-yellow-800 mb-1">Комментарий модератора</p>
                    <p className="text-sm text-yellow-900">{product.moderationComment}</p>
                  </div>
                )}

                {/* Variants */}
                {product.variants.length > 0 && (
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-2">Варианты ({product.variants.length})</p>
                    <div className="space-y-1">
                      {product.variants.map(v => (
                        <div key={v.id} className="flex items-center justify-between text-sm px-3 py-2 bg-gray-50 rounded-lg">
                          <span className="text-gray-800">{v.label}</span>
                          <div className="flex items-center gap-3 text-xs text-gray-500">
                            {v.sku && <span>SKU: {v.sku}</span>}
                            {v.price !== undefined && <span>{v.price.toFixed(2)}</span>}
                            <span className={v.isActive ? 'text-green-600' : 'text-gray-400'}>{v.isActive ? 'Активен' : 'Неактивен'}</span>
                            {v.inStock !== undefined && <span className={v.inStock ? 'text-green-600' : 'text-orange-500'}>{v.inStock ? 'В наличии' : 'Нет'}</span>}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Gallery */}
                {product.gallery.length > 1 && (
                  <div>
                    <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-2">Галерея</p>
                    <div className="flex gap-2 flex-wrap">
                      {product.gallery.map((img, i) => (
                        <img key={i} src={img.url} alt={img.altText ?? `Фото ${i + 1}`} className="w-16 h-16 rounded-lg object-cover border border-gray-200" />
                      ))}
                    </div>
                  </div>
                )}

                {/* Actions */}
                <div className="border-t border-gray-200 pt-4">
                  <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">Действия модератора</p>
                  <div className="flex flex-wrap gap-2">
                    {status === 'pending_moderation' && (
                      <>
                        <button disabled={isActing} onClick={() => doAction('Одобрить', () => approveProduct(product.id))}
                          className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50">
                          <CheckCircle className="w-4 h-4" /> Одобрить
                        </button>
                        <button disabled={isActing} onClick={() => setRejectOpen(true)}
                          className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 disabled:opacity-50">
                          <XCircle className="w-4 h-4" /> Отклонить
                        </button>
                      </>
                    )}
                    {(status === 'approved' || status === 'hidden') && (
                      <button disabled={isActing} onClick={() => doAction('Опубликовать', () => publishProduct(product.id))}
                        className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-white bg-green-600 rounded-lg hover:bg-green-700 disabled:opacity-50">
                        <Eye className="w-4 h-4" /> Опубликовать
                      </button>
                    )}
                    {status === 'published' && (
                      <button disabled={isActing} onClick={() => doAction('Скрыть', () => hideProduct(product.id))}
                        className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50">
                        <EyeOff className="w-4 h-4" /> Скрыть
                      </button>
                    )}
                    {status !== 'blocked' && status !== 'rejected' && (
                      <button disabled={isActing}
                        onClick={() => { if (window.confirm('Заблокировать товар? Это снимет его с публикации.')) doAction('Заблокировать', () => blockProduct(product.id)); }}
                        className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-red-700 bg-red-50 rounded-lg hover:bg-red-100 disabled:opacity-50">
                        <Lock className="w-4 h-4" /> Заблокировать
                      </button>
                    )}
                  </div>
                  {isActing && <p className="mt-2 text-xs text-gray-500">Выполняем действие…</p>}
                </div>

                {/* Moderation logs */}
                <div className="border-t border-gray-200 pt-4">
                  <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">История модерации</p>
                  {isLogsLoading ? (
                    <p className="text-sm text-gray-500">Загружаем историю…</p>
                  ) : logs.length === 0 ? (
                    <p className="text-sm text-gray-400 italic">История модерации пока пустая.</p>
                  ) : (
                    <ol className="relative border-l border-gray-200 ml-2 space-y-4">
                      {logs.map(log => (
                        <li key={log.id} className="ml-4">
                          <div className="absolute -left-1.5 mt-1.5 w-3 h-3 rounded-full bg-indigo-400 border-2 border-white" />
                          <div className="text-xs text-gray-400 mb-0.5">{formatDate(log.createdAt)}</div>
                          <div className="text-sm text-gray-800">
                            {log.fromStatus && (
                              <><span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${statusBadge(log.fromStatus)}`}>{statusLabel(log.fromStatus)}</span>{' → '}</>
                            )}
                            <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${statusBadge(log.toStatus)}`}>{statusLabel(log.toStatus)}</span>
                          </div>
                          {log.comment && <p className="mt-1 text-sm text-gray-600 italic">{log.comment}</p>}
                        </li>
                      ))}
                    </ol>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <RejectModal isOpen={rejectOpen} isSubmitting={isRejectSubmitting} onClose={() => setRejectOpen(false)} onSubmit={handleRejectSubmit} />
    </>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────

export function AdminProducts() {
  const [products, setProducts] = useState<AdminProductView[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [drawerProductId, setDrawerProductId] = useState<string | null>(null);

  const fetchProducts = useCallback(async () => {
    try {
      setIsLoading(true); setError(null);
      const data = await getAdminProducts(page, 20, { q: searchQuery, status: statusFilter, source: sourceFilter });
      setProducts(data.items);
      setTotalCount(data.totalCount);
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось загрузить товары.'));
    } finally { setIsLoading(false); }
  }, [page, searchQuery, statusFilter, sourceFilter]);

  useEffect(() => {
    const timer = setTimeout(fetchProducts, 300);
    return () => clearTimeout(timer);
  }, [fetchProducts]);

  const totalPages = Math.max(1, Math.ceil(totalCount / 20));

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">
          Каталог товаров <HelpTooltip content="Управление всеми товарами продавцов и платформенными товарами (ZAMK)." />
        </h1>
        <span className="mt-1 text-sm text-gray-500">{totalCount} товаров</span>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <Search className="h-4 w-4 text-gray-400" />
          </div>
          <input
            id="products-search" type="text"
            className="block w-full pl-9 pr-3 py-2 border border-gray-300 rounded-lg text-sm bg-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500"
            placeholder="Поиск по названию или ID…"
            value={searchQuery} onChange={e => { setSearchQuery(e.target.value); setPage(1); }}
          />
        </div>
        <select id="products-status-filter" value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setPage(1); }}
          className="block w-full sm:w-48 pl-3 pr-8 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500">
          <option value="">Все статусы</option>
          <option value="published">Опубликован</option>
          <option value="approved">Одобрен</option>
          <option value="pending_moderation">На модерации</option>
          <option value="rejected">Отклонён</option>
          <option value="hidden">Скрыт</option>
          <option value="blocked">Заблокирован</option>
          <option value="draft">Черновик</option>
        </select>
        <select id="products-source-filter" value={sourceFilter} onChange={e => { setSourceFilter(e.target.value); setPage(1); }}
          className="block w-full sm:w-48 pl-3 pr-8 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500">
          <option value="">Все источники</option>
          <option value="auction_direct_sale">ZAMK</option>
          <option value="seller">Продавцы</option>
        </select>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-4 bg-red-50 text-red-700 rounded-lg text-sm">
          <AlertCircle className="h-5 w-5 shrink-0" />{error}
        </div>
      )}

      {isLoading ? (
        <div className="flex flex-col items-center justify-center py-16 gap-3 text-gray-500">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
          <p className="text-sm">Загружаем товары…</p>
        </div>
      ) : products.length === 0 ? (
        <div className="text-center py-16 bg-white rounded-xl shadow-sm border border-gray-100">
          <Package className="mx-auto h-12 w-12 text-gray-300" />
          <h3 className="mt-3 text-sm font-semibold text-gray-700">Товары не найдены</h3>
          <p className="mt-1 text-sm text-gray-400">Попробуйте изменить фильтры или поисковый запрос.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl shadow-sm border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 bg-white">
            <thead className="bg-gray-50">
              <tr>
                {['Товар', 'Продавец', 'Категория / Бренд', 'Статус', 'Цена', ''].map(h => (
                  <th key={h} className={`px-5 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider ${h === '' ? 'text-right' : 'text-left'}`}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {products.map(product => (
                <tr key={product.id} className="hover:bg-indigo-50/40 transition-colors cursor-pointer" onClick={() => setDrawerProductId(product.id)}>
                  <td className="px-5 py-4">
                    <div className="flex items-center gap-3">
                      {product.image
                        ? <img src={product.image} alt="" className="h-10 w-10 rounded-lg object-cover border border-gray-200 shrink-0" />
                        : <div className="h-10 w-10 rounded-lg bg-gray-100 flex items-center justify-center shrink-0"><Package className="w-5 h-5 text-gray-400" /></div>
                      }
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-gray-900 truncate max-w-[200px]">
                          {product.title}
                          {product.source === 'auction_direct_sale' && (
                            <span className="ml-2 inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">ZAMK</span>
                          )}
                        </div>
                        <div className="text-xs text-gray-400 truncate">{product.id}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-5 py-4 text-sm text-gray-700 whitespace-nowrap">{product.sellerName || product.sellerId || '—'}</td>
                  <td className="px-5 py-4 text-sm">
                    <div className="text-gray-800">{product.category || '—'}</div>
                    <div className="text-xs text-gray-400">{product.brand || '—'}</div>
                  </td>
                  <td className="px-5 py-4 whitespace-nowrap">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusBadge(product.status)}`}>{statusLabel(product.status)}</span>
                    {product.moderationComment && (
                      <div className="mt-1 max-w-[180px] truncate text-xs text-gray-400" title={product.moderationComment}>{product.moderationComment}</div>
                    )}
                  </td>
                  <td className="px-5 py-4 text-sm text-gray-800 whitespace-nowrap">
                    <div>{formatPrice(product.price * 100, product.currency)}</div>
                    {product.oldPrice !== undefined && <div className="text-xs text-gray-400 line-through">{formatPrice(product.oldPrice * 100, product.currency)}</div>}
                  </td>
                  <td className="px-5 py-4 text-right"><ChevronRight className="w-4 h-4 text-gray-400 ml-auto" /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination */}
      {totalCount > 0 && (
        <div className="flex items-center justify-center gap-2">
          <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
            className="px-3 py-1.5 text-sm border border-gray-300 rounded-lg disabled:opacity-40 hover:bg-gray-50">← Назад</button>
          <span className="text-sm text-gray-600">Страница {page} из {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages}
            className="px-3 py-1.5 text-sm border border-gray-300 rounded-lg disabled:opacity-40 hover:bg-gray-50">Вперёд →</button>
        </div>
      )}

      <ProductDrawer productId={drawerProductId} onClose={() => setDrawerProductId(null)} onActionDone={fetchProducts} />
    </div>
  );
}

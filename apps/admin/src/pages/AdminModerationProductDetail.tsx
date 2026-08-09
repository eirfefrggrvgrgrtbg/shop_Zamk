import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  ShieldAlert,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  Eye,
  Check,
  X,
  RotateCcw,
  Package,
  Layers,
  DollarSign,
  History,
} from 'lucide-react';
import {
  getAdminProduct,
  startProductReview,
  approveProduct,
  rejectProduct,
  blockProduct,
  createProductPreviewLink,
  getAdminProductModerationHistory,
  getModerationProducts,
  getAdminProductErrorMessage,
  type AdminProductView,
} from '../api/adminProducts';

const statusLocalizationMap: Record<string, string> = {
  draft: 'Черновик',
  pending_moderation: 'На модерации',
  in_review: 'На проверке',
  approved: 'Одобрен',
  rejected: 'Отклонен',
  sent_to_revision: 'На доработке',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
  published: 'Опубликован',
};

const localizeStatus = (st?: string | null): string => {
  if (!st) return '—';
  return statusLocalizationMap[st] || st;
};

const cleanComment = (c?: string | null): string | null => {
  if (!c) return null;
  return c.replace(/^(Статус\s*:\s*)+/i, '').trim();
};

const REVISE_OPTIONS = [
  'Отсутствует или некачественное фото',
  'Недостаточное или некорректное описание (<10 симв.)',
  'Не указана категория товара',
  'Не указан бренд товара',
  'Отсутствуют активные варианты товара',
  'Некорректная цена товара (≤ 0 ₽)',
  'Обнаружены дублирующие SKU вариантов',
  'Другое замечание к контенту'
];

export function AdminModerationProductDetail() {
  const { productId } = useParams<{ productId: string }>();
  const navigate = useNavigate();

  const [product, setProduct] = useState<AdminProductView | null>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);

  const [nextProductId, setNextProductId] = useState<string | null>(null);

  // Moderation action modals
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [modalAction, setModalAction] = useState<'approve' | 'reject' | 'revise' | null>(null);
  const [actionComment, setActionComment] = useState('');
  const [selectedReviseReasons, setSelectedReviseReasons] = useState<string[]>([]);
  const [selectedRejectReason, setSelectedRejectReason] = useState<string>('Запрещённый законодательством товар');

  // Preview link
  const [previewLink, setPreviewLink] = useState<{ pageUrl: string; catalogCardUrl: string; expiresAt: string } | null>(null);
  const [isGeneratingPreview, setIsGeneratingPreview] = useState(false);

  useEffect(() => {
    if (!productId) return;
    loadProductData(productId);
  }, [productId]);

  const loadProductData = async (id: string) => {
    try {
      setIsLoading(true);
      setError(null);
      const [prodData, histData, pendingQueue] = await Promise.all([
        getAdminProduct(id),
        getAdminProductModerationHistory(id).catch(() => ({ items: [] })),
        getModerationProducts({ status: 'pending_moderation', limit: 10 }).catch(() => ({ items: [] })),
      ]);

      setProduct(prodData);
      setHistory(histData.items || []);

      // Find next product in pending queue
      const otherPending = (pendingQueue.items || []).filter((p) => p.id !== id);
      if (otherPending.length > 0) {
        setNextProductId(otherPending[0].id);
      } else {
        setNextProductId(null);
      }
    } catch (err: any) {
      console.error('Failed to load moderation product:', err);
      setError(getAdminProductErrorMessage(err, 'Ошибка загрузки данных товара для модерации'));
    } finally {
      setIsLoading(false);
    }
  };

  // Start Review action
  const handleStartReview = async () => {
    if (!product) return;
    try {
      setIsSubmitting(true);
      setError(null);
      await startProductReview(product.id, product.updatedAt);
      loadProductData(product.id);
      setActionSuccess('Проверка товара успешно начата.');
    } catch (err: any) {
      setError(getAdminProductErrorMessage(err, 'Не удалось начать проверку товара'));
    } finally {
      setIsSubmitting(false);
    }
  };

  // Generate Shop Preview
  const handleGeneratePreview = async () => {
    if (!product) return;
    try {
      setIsGeneratingPreview(true);
      const res = await createProductPreviewLink(product.id);
      setPreviewLink(res);
      window.open(res.pageUrl, '_blank');
    } catch (err: any) {
      setError(getAdminProductErrorMessage(err, 'Ошибка генерации ссылки предпросмотра'));
    } finally {
      setIsGeneratingPreview(false);
    }
  };

  const openReviseModal = (failedLabels: string[]) => {
    const mapped = failedLabels.map((l) => {
      if (l === 'Главное изображение') return 'Отсутствует или некачественное фото';
      if (l === 'Описание товара') return 'Недостаточное или некорректное описание (<10 симв.)';
      if (l === 'Категория привязана') return 'Не указана категория товара';
      if (l === 'Бренд привязан') return 'Не указан бренд товара';
      if (l === 'Активные варианты') return 'Отсутствуют активные варианты товара';
      if (l === 'Корректная цена') return 'Некорректная цена товара (≤ 0 ₽)';
      if (l === 'Уникальность SKU') return 'Обнаружены дублирующие SKU вариантов';
      return l;
    });
    setSelectedReviseReasons(mapped);
    setActionComment('');
    setModalAction('revise');
  };

  const openRejectModal = () => {
    setSelectedRejectReason('Запрещённый законодательством товар');
    setActionComment('');
    setModalAction('reject');
  };

  // Submit decision (Approve / Reject / Return for Revision)
  const handleDecisionSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!product || !modalAction) return;

    try {
      setIsSubmitting(true);
      setError(null);

      if (modalAction === 'approve') {
        await approveProduct(product.id, actionComment);
        setActionSuccess('Товар успешно одобрен.');
      } else if (modalAction === 'reject') {
        if (!actionComment.trim()) {
          setError('Причина и подробный комментарий отклонения обязательны.');
          setIsSubmitting(false);
          return;
        }
        await blockProduct(product.id, `Отклонено (${selectedRejectReason}): ${actionComment.trim()}`);
        setActionSuccess('Товар отклонён с указанием критического нарушения.');
      } else if (modalAction === 'revise') {
        if (!actionComment.trim() || actionComment.trim().length < 10) {
          setError('Комментарий для возврата на исправление должен содержать минимум 10 символов.');
          setIsSubmitting(false);
          return;
        }
        const reasonsPart = selectedReviseReasons.length > 0 ? `Причины: ${selectedReviseReasons.join('; ')}. ` : '';
        await rejectProduct(product.id, `На исправление: ${reasonsPart}${actionComment.trim()}`);
        setActionSuccess('Товар отправлен продавцу на исправление.');
      }

      setModalAction(null);
      setActionComment('');
      loadProductData(product.id);
    } catch (err: any) {
      setError(getAdminProductErrorMessage(err, 'Ошибка при сохранении решения модерации'));
    } finally {
      setIsSubmitting(false);
    }
  };

  // Format date helper
  const formatDate = (d?: string) => (d ? new Date(d).toLocaleString('ru-RU') : '—');
  const formatPrice = (rub: number) =>
    new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(rub);

  if (isLoading) {
    return (
      <div className="p-12 text-center">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-indigo-600 border-t-transparent mx-auto"></div>
        <p className="mt-3 text-xs text-slate-500">Загрузка рабочего места модератора...</p>
      </div>
    );
  }

  if (error && !product) {
    return (
      <div className="p-8 max-w-xl mx-auto text-center space-y-4">
        <AlertTriangle className="w-12 h-12 text-rose-500 mx-auto" />
        <h2 className="text-base font-bold text-slate-900 dark:text-white">Ошибка доступа к товару</h2>
        <p className="text-xs text-slate-500">{error}</p>
        <button
          onClick={() => navigate('/moderation')}
          className="px-4 py-2 text-xs font-semibold text-white bg-indigo-600 rounded-xl hover:bg-indigo-700"
        >
          Вернуться в очередь модерации
        </button>
      </div>
    );
  }

  if (!product) return null;

  // System Checks Rules
  const checks = [
    { label: 'Главное изображение', pass: !!product.image, resultText: !!product.image ? 'Изображение загружено' : 'Изображение отсутствует', hint: 'Главное фото товара' },
    { label: 'Описание товара', pass: !!product.description && product.description.trim().length > 10, resultText: (!!product.description && product.description.trim().length > 10) ? 'Описание заполнено корректно' : 'Описание отсутствует или слишком короткое (<10 симв.)', hint: 'Минимум 10 символов' },
    { label: 'Категория привязана', pass: !!product.category, resultText: !!product.category ? 'Категория привязана' : 'Категория не указана', hint: 'Категория маркетплейса' },
    { label: 'Бренд привязан', pass: !!product.brand, resultText: !!product.brand ? 'Бренд привязан' : 'Бренд не указан', hint: 'Указан бренд' },
    { label: 'Активные варианты', pass: (product.variants?.filter((v) => v.isActive).length ?? 0) > 0, resultText: (product.variants?.filter((v) => v.isActive).length ?? 0) > 0 ? 'Обнаружены активные варианты' : 'Активные варианты отсутствуют', hint: 'Хотя бы 1 активный вариант' },
    { label: 'Корректная цена', pass: product.price > 0, resultText: product.price > 0 ? 'Цена указана корректно (>0 ₽)' : 'Цена некорректна (≤ 0 ₽)', hint: 'Больше 0 ₽' },
    { label: 'Уникальность SKU', pass: new Set(product.variants.map((v) => v.sku).filter(Boolean)).size === product.variants.map((v) => v.sku).filter(Boolean).length, resultText: (new Set(product.variants.map((v) => v.sku).filter(Boolean)).size === product.variants.map((v) => v.sku).filter(Boolean).length) ? 'Дубли SKU отсутствуют' : 'Обнаружены дублирующие SKU', hint: 'Уникальные артикулы' },
  ];
  const passedChecksCount = checks.filter((c) => c.pass).length;
  const failedChecks = checks.filter((c) => !c.pass);
  const failedChecksCount = failedChecks.length;
  const allChecksPassed = passedChecksCount === checks.length;

  return (
    <div className="space-y-6">
      {/* Top Header Navigation */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 bg-white dark:bg-slate-900 p-4 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/moderation')}
            className="p-2 text-slate-500 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-slate-800 rounded-xl transition-colors"
            title="Назад к очереди модерации"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <div className="flex items-center gap-2">
              <span className="px-2.5 py-0.5 rounded-full text-[11px] font-bold bg-amber-100 text-amber-800 border border-amber-300">
                Рабочее место модератора
              </span>
              <span className="text-xs text-slate-400 font-mono">ID: {product.id.slice(0, 8)}...</span>
            </div>
            <h1 className="text-lg font-bold text-slate-900 dark:text-white mt-0.5 line-clamp-1">{product.title}</h1>
          </div>
        </div>

        {/* Action Controls Header */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Shop Preview Button */}
          <button
            onClick={handleGeneratePreview}
            disabled={isGeneratingPreview}
            className="px-3 py-1.5 text-xs font-semibold text-indigo-700 bg-indigo-50 hover:bg-indigo-100 border border-indigo-200 rounded-xl transition-colors flex items-center gap-1.5"
          >
            <Eye className="w-3.5 h-3.5" />
            <span>{isGeneratingPreview ? 'Создание...' : 'Shop Preview'}</span>
          </button>

          {/* Action buttons strictly depending on current status */}
          {product.status === 'pending_moderation' && (
            <button
              data-testid="btn-start-review"
              onClick={handleStartReview}
              disabled={isSubmitting}
              className="px-3.5 py-1.5 text-xs font-bold text-white bg-amber-600 hover:bg-amber-500 rounded-xl shadow transition-colors flex items-center gap-1.5"
            >
              <Clock className="w-3.5 h-3.5" />
              <span>Начать проверку</span>
            </button>
          )}

          {product.status === 'in_review' && (
            <>
              <button
                onClick={() => {
                  setModalAction('approve');
                  setActionComment('');
                }}
                disabled={!allChecksPassed}
                title={!allChecksPassed ? `Одобрение недоступно: есть ошибки (${failedChecks.map(c => c.label).join(', ')})` : ''}
                className="px-3.5 py-1.5 text-xs font-bold text-white bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-300 dark:disabled:bg-slate-800 disabled:text-slate-500 disabled:cursor-not-allowed rounded-xl shadow transition-colors flex items-center gap-1.5"
              >
                <Check className="w-3.5 h-3.5" />
                <span>Одобрить</span>
              </button>

              <button
                data-testid="btn-revise"
                onClick={() => openReviseModal(failedChecks.map(c => c.label))}
                className="px-3.5 py-1.5 text-xs font-bold text-amber-800 bg-amber-100 hover:bg-amber-200 border border-amber-300 rounded-xl transition-colors flex items-center gap-1.5"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Вернуть на исправление</span>
              </button>

              <button
                data-testid="btn-reject"
                onClick={openRejectModal}
                className="px-3.5 py-1.5 text-xs font-bold text-white bg-rose-600 hover:bg-rose-500 rounded-xl shadow transition-colors flex items-center gap-1.5"
              >
                <X className="w-3.5 h-3.5" />
                <span>Отклонить</span>
              </button>
            </>
          )}

          {product.status === 'approved' && (
            <div className="flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold text-emerald-800 bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300 border border-emerald-300 rounded-xl">
              <CheckCircle className="w-4 h-4 text-emerald-600" />
              <span>Товар прошёл модерацию и одобрен</span>
            </div>
          )}

          {product.status === 'blocked' && (
            <div className="flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold text-rose-800 bg-rose-100 dark:bg-rose-950/60 dark:text-rose-300 border border-rose-300 rounded-xl">
              <XCircle className="w-4 h-4 text-rose-600" />
              <span>Товар заблокирован{product.moderationComment ? `: ${cleanComment(product.moderationComment)}` : ''}</span>
            </div>
          )}

          {/* Other statuses badge */}
          {!['pending_moderation', 'in_review', 'approved', 'blocked'].includes(product.status) && (
            <div className="flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 rounded-xl border">
              <span>Статус: {localizeStatus(product.status)}</span>
            </div>
          )}

          {/* Transition to next pending product */}
          {nextProductId && (
            <button
              onClick={() => navigate(`/moderation/products/${nextProductId}`)}
              className="px-3 py-1.5 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-xl transition-colors border border-slate-300 ml-2"
            >
              К следующему →
            </button>
          )}
        </div>
      </div>

      {/* Warning banner when in_review but automated checks failed */}
      {product.status === 'in_review' && !allChecksPassed && (
        <div className="p-4 bg-amber-50 dark:bg-amber-950/60 text-amber-900 dark:text-amber-200 rounded-2xl border border-amber-300 dark:border-amber-800 text-xs flex items-center justify-between shadow-sm">
          <div className="flex items-center gap-3">
            <AlertTriangle className="w-5 h-5 text-amber-600 flex-shrink-0" />
            <div>
              <h4 className="text-sm font-bold">Одобрение товара недоступно</h4>
              <p className="mt-0.5 text-slate-700 dark:text-slate-300">
                Обязательные автоматические проверки не пройдены: <strong className="font-bold text-amber-800 dark:text-amber-300">{failedChecks.map(c => c.label).join(', ')}</strong>. Вы можете вернуть товар на исправление продавцу с указанием причин.
              </p>
            </div>
          </div>
          <button
            onClick={() => openReviseModal(failedChecks.map(c => c.label))}
            className="px-3 py-1.5 bg-amber-600 hover:bg-amber-500 text-white font-bold rounded-xl whitespace-nowrap transition-colors"
          >
            Вернуть на исправление
          </button>
        </div>
      )}

      {/* Notifications */}
      {actionSuccess && (
        <div className="p-3 bg-emerald-50 text-emerald-800 rounded-xl border border-emerald-200 text-xs flex items-center justify-between">
          <span>✓ {actionSuccess}</span>
          <button onClick={() => setActionSuccess(null)} className="text-emerald-600 hover:underline">
            Закрыть
          </button>
        </div>
      )}

      {error && (
        <div className="p-3 bg-rose-50 text-rose-800 rounded-xl border border-rose-200 text-xs flex items-center justify-between">
          <span>⚠️ {error}</span>
          <button onClick={() => setError(null)} className="text-rose-600 hover:underline">
            Закрыть
          </button>
        </div>
      )}

      {/* Preview Link Result Banner */}
      {previewLink && (
        <div className="p-3 bg-indigo-50 rounded-xl border border-indigo-200 text-xs space-y-1 text-indigo-900">
          <div className="flex items-center justify-between">
            <span className="font-bold">Ссылка предпросмотра Shop (действительна 15 мин):</span>
            <button onClick={() => setPreviewLink(null)} className="text-indigo-400 hover:text-indigo-700">
              <X className="w-4 h-4" />
            </button>
          </div>
          <a href={previewLink.pageUrl} target="_blank" rel="noreferrer" className="underline break-all block font-mono text-indigo-700">
            {previewLink.pageUrl}
          </a>
        </div>
      )}

      {/* Main Grid: 2 Columns */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left 2 Columns: Product Content & Checks */}
        <div className="lg:col-span-2 space-y-6">
          {/* Automated System Checks Box */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3">
            <div className="flex items-center justify-between border-b pb-2">
              <h2 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <ShieldAlert className="w-4 h-4 text-indigo-600" />
                <span>Автоматические проверки модерации</span>
              </h2>
              <span className={`px-2.5 py-0.5 rounded-full text-xs font-bold ${allChecksPassed ? 'bg-emerald-100 text-emerald-800' : 'bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300 border border-rose-200 dark:border-rose-800'}`}>
                {allChecksPassed ? `${passedChecksCount}/${checks.length} успешно` : `${failedChecksCount} ${failedChecksCount === 1 ? 'ошибка' : (failedChecksCount >= 2 && failedChecksCount <= 4) ? 'ошибки' : 'ошибок'} (${passedChecksCount}/${checks.length} успешно)`}
              </span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
              {checks.map((check, i) => (
                <div
                  key={i}
                  className={`p-2.5 rounded-xl border flex items-start gap-2.5 ${
                    check.pass ? 'bg-emerald-50/50 border-emerald-200 text-emerald-900 dark:bg-emerald-950/20 dark:border-emerald-900 dark:text-emerald-300' : 'bg-rose-50/50 border-rose-200 text-rose-900 dark:bg-rose-950/20 dark:border-rose-900 dark:text-rose-300'
                  }`}
                >
                  {check.pass ? <CheckCircle className="w-4 h-4 text-emerald-600 flex-shrink-0 mt-0.5" /> : <XCircle className="w-4 h-4 text-rose-500 flex-shrink-0 mt-0.5" />}
                  <div>
                    <span className="font-semibold block">{check.label}</span>
                    <span className={`text-[11px] block font-medium mt-0.5 ${check.pass ? 'text-emerald-700 dark:text-emerald-400' : 'text-rose-700 dark:text-rose-400'}`}>{check.resultText}</span>
                    <span className="text-[10px] opacity-70 block mt-0.5">{check.hint}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Product Overview Card */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4">
            <h2 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2 border-b pb-2">
              <Package className="w-4 h-4 text-indigo-600" />
              <span>Данные карточки товара</span>
            </h2>

            <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-xs">
              <div>
                <span className="text-slate-500 block">Категория</span>
                <span className="font-semibold text-slate-900 dark:text-white">{product.category || '—'}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Бренд</span>
                <span className="font-semibold text-slate-900 dark:text-white">{product.brand || '—'}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Базовая цена</span>
                <span className="font-bold text-slate-900 dark:text-white">{formatPrice(product.price)}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Продавец</span>
                <button
                  onClick={() => product.sellerId && navigate(`/sellers/${product.sellerId}`)}
                  className="font-semibold text-indigo-600 hover:underline text-left block"
                >
                  {product.sellerName || 'Продавец'}
                </button>
              </div>
              <div>
                <span className="text-slate-500 block">Статус продавца</span>
                <span className={`font-semibold ${product.sellerIsActive ? 'text-emerald-600' : 'text-rose-600'}`}>
                  {product.sellerStatus || (product.sellerIsActive ? 'Активен' : 'Неактивен')}
                </span>
              </div>
              <div>
                <span className="text-slate-500 block">Дата подачи</span>
                <span className="font-medium text-slate-700">{formatDate(product.submittedAt || product.createdAt)}</span>
              </div>
            </div>

            <div>
              <span className="text-slate-500 text-xs block mb-1">Описание</span>
              <p className="text-xs text-slate-800 dark:text-slate-200 bg-slate-50 dark:bg-slate-800 p-3 rounded-xl whitespace-pre-wrap leading-relaxed">
                {product.description || 'Описание отсутствует.'}
              </p>
            </div>
          </div>

          {/* Photo Gallery */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3">
            <h2 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2 border-b pb-2">
              <Layers className="w-4 h-4 text-indigo-600" />
              <span>Фотографии товара ({product.gallery.length})</span>
            </h2>

            {product.gallery.length === 0 ? (
              <p className="text-xs text-rose-500 bg-rose-50 p-3 rounded-xl">⚠️ Изображения не загружены.</p>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                {product.gallery.map((img, idx) => (
                  <div key={img.id || idx} className="group relative rounded-xl overflow-hidden border border-slate-200 bg-slate-100 aspect-square">
                    <img src={img.url} alt={img.altText || product.title} className="w-full h-full object-cover group-hover:scale-105 transition-transform" />
                    {idx === 0 && <span className="absolute top-1.5 left-1.5 px-2 py-0.5 bg-black/70 text-white text-[10px] rounded-md font-bold">Главное</span>}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Variants Table */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3">
            <h2 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2 border-b pb-2">
              <DollarSign className="w-4 h-4 text-indigo-600" />
              <span>Варианты и SKU ({product.variants.length})</span>
            </h2>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b bg-slate-50 dark:bg-slate-800 text-slate-500 uppercase text-[10px]">
                    <th className="py-2.5 px-3">Размер / Цвет</th>
                    <th className="py-2.5 px-3">SKU</th>
                    <th className="py-2.5 px-3 text-right">Цена</th>
                    <th className="py-2.5 px-3 text-center">Статус варианта</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                  {product.variants.map((v, i) => (
                    <tr key={v.id || i}>
                      <td className="py-2.5 px-3 font-medium text-slate-900 dark:text-white">
                        {[v.size, v.color].filter(Boolean).join(' / ') || '—'}
                      </td>
                      <td className="py-2.5 px-3 font-mono text-slate-600">{v.sku || '—'}</td>
                      <td className="py-2.5 px-3 text-right font-bold text-slate-900 dark:text-white">
                        {v.price != null ? formatPrice(v.price) : '—'}
                      </td>
                      <td className="py-2.5 px-3 text-center">
                        <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold ${v.isActive ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-500'}`}>
                          {v.isActive ? 'Активен' : 'Неактивен'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>

        {/* Right Sidebar Column: Moderation History & SLA */}
        <div className="space-y-6">
          {/* Moderation Status Card */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider">Статус проверки</h3>

            <div className="p-3 bg-slate-50 dark:bg-slate-800 rounded-xl space-y-2 text-xs">
              <div className="flex justify-between">
                <span className="text-slate-500">Текущий статус:</span>
                <span className="font-bold text-indigo-600">{localizeStatus(product.status)}</span>
              </div>
              {product.moderationComment && (
                <div className="pt-2 border-t text-rose-700 dark:text-rose-400">
                  <span className="font-semibold block mb-0.5">Предыдущий комментарий:</span>
                  <p className="bg-rose-50 dark:bg-rose-950/40 p-2 rounded-lg text-[11px]">{cleanComment(product.moderationComment)}</p>
                </div>
              )}
            </div>
          </div>

          {/* Moderation History Timeline */}
          <div className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3">
            <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
              <History className="w-3.5 h-3.5" />
              <span>История решений модератора</span>
            </h3>

            {history.length === 0 ? (
              <p className="text-xs text-slate-400 italic">Записи истории отсутствуют.</p>
            ) : (
              <div className="space-y-3 border-l-2 border-slate-200 dark:border-slate-800 pl-3 text-xs">
                {history.map((log) => (
                  <div key={log.id} className="space-y-0.5">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-bold text-slate-900 dark:text-white">{localizeStatus(log.toStatus)}</span>
                      <span className="text-[10px] text-slate-400">{formatDate(log.createdAt)}</span>
                    </div>
                    {log.adminName && <span className="text-[11px] text-indigo-600 block">{log.adminName}</span>}
                    {log.comment && <p className="text-[11px] text-slate-600 dark:text-slate-400 bg-slate-50 dark:bg-slate-800 p-2 rounded-lg mt-1">{cleanComment(log.comment)}</p>}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Decision Modal (Approve / Reject / Revise) */}
      {modalAction && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl shadow-2xl max-w-lg w-full p-6 border border-slate-200 dark:border-slate-800 space-y-4">
            <h3 className="text-base font-bold text-slate-900 dark:text-white">
              {modalAction === 'approve' && 'Одобрить товар'}
              {modalAction === 'revise' && 'Вернуть товар на исправление'}
              {modalAction === 'reject' && 'Отклонить товар (критическое нарушение)'}
            </h3>

            <form onSubmit={handleDecisionSubmit} className="space-y-4">
              {modalAction === 'revise' && (
                <div>
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300 block mb-2">
                    Выберите причины отправки на доработку (автоматически отмечены ошибки проверок):
                  </label>
                  <div className="space-y-1.5 max-h-52 overflow-y-auto p-2.5 bg-slate-50 dark:bg-slate-800 rounded-xl border">
                    {REVISE_OPTIONS.map((opt) => {
                      const isChecked = selectedReviseReasons.includes(opt);
                      return (
                        <label key={opt} className="flex items-center gap-2.5 text-xs text-slate-700 dark:text-slate-300 cursor-pointer p-1 hover:bg-slate-100 dark:hover:bg-slate-700/50 rounded-lg">
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={(e) => {
                              if (e.target.checked) setSelectedReviseReasons(prev => [...prev, opt]);
                              else setSelectedReviseReasons(prev => prev.filter(x => x !== opt));
                            }}
                            className="rounded border-slate-300 text-amber-600 focus:ring-amber-500 h-4 w-4"
                          />
                          <span>{opt}</span>
                        </label>
                      );
                    })}
                  </div>
                </div>
              )}

              {modalAction === 'reject' && (
                <div>
                  <label className="text-xs font-semibold text-slate-700 dark:text-slate-300 block mb-2">
                    Выберите критическое нарушение (блокировка/отказ без доработки):
                  </label>
                  <div className="space-y-2 p-2.5 bg-slate-50 dark:bg-slate-800 rounded-xl border">
                    {[
                      'Запрещённый законодательством товар',
                      'Мошенничество или введение покупателя в заблуждение',
                      'Нарушение авторских прав и чужая торговая марка (контрафакт)',
                      'Критическое нарушение правил маркетплейса',
                    ].map((opt) => (
                      <label key={opt} className="flex items-center gap-2.5 text-xs text-slate-700 dark:text-slate-300 cursor-pointer p-1 hover:bg-slate-100 dark:hover:bg-slate-700/50 rounded-lg">
                        <input
                          type="radio"
                          name="reject-reason"
                          checked={selectedRejectReason === opt}
                          onChange={() => setSelectedRejectReason(opt)}
                          className="text-rose-600 focus:ring-rose-500 h-4 w-4"
                        />
                        <span className="font-medium">{opt}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="text-xs font-medium text-slate-700 dark:text-slate-300 block mb-1">
                  Комментарий для продавца {modalAction !== 'approve' && <span className="text-rose-500">* (мин. 10 символов для доработки)</span>}
                </label>
                <textarea
                  rows={3}
                  value={actionComment}
                  onChange={(e) => setActionComment(e.target.value)}
                  required={modalAction !== 'approve'}
                  placeholder={modalAction === 'approve' ? 'Опциональный комментарий...' : modalAction === 'revise' ? 'Опишите, что именно нужно поправить (мин. 10 символов)...' : 'Подробный комментарий о причине отклонения...'}
                  className="w-full p-2.5 text-xs bg-slate-50 dark:bg-slate-800 border rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
                {modalAction === 'revise' && (
                  <p className={`text-[11px] mt-1 ${actionComment.trim().length < 10 ? 'text-amber-600 dark:text-amber-400 font-medium' : 'text-emerald-600 dark:text-emerald-400'}`}>
                    {actionComment.trim().length < 10 ? `Минимум 10 символов (сейчас: ${actionComment.trim().length})` : '✓ Длина комментария корректна.'}
                  </p>
                )}
              </div>

              <div className="flex items-center justify-end gap-3 pt-2 border-t">
                <button
                  type="button"
                  data-testid="modal-cancel-btn"
                  onClick={() => setModalAction(null)}
                  className="px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-100 rounded-xl"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  data-testid="modal-submit-btn"
                  disabled={isSubmitting || (modalAction === 'revise' && actionComment.trim().length < 10) || (modalAction === 'reject' && !actionComment.trim())}
                  className={`px-4 py-2 text-xs font-bold text-white rounded-xl shadow transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
                    modalAction === 'approve' ? 'bg-emerald-600 hover:bg-emerald-500' : modalAction === 'revise' ? 'bg-amber-600 hover:bg-amber-500' : 'bg-rose-600 hover:bg-rose-500'
                  }`}
                >
                  {isSubmitting ? 'Сохранение...' : 'Подтвердить решение'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

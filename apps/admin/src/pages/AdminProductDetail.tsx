import { useState, useEffect } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import {
  ArrowLeft,
  AlertCircle,
  AlertTriangle,
  Clock,
  ExternalLink,
  Store,
  Tag,
  X,
  Eye,
  DollarSign,
  ShieldAlert,
  Edit3,
  Package,
  Layers,
  Star,
  History,
  Lock,
  EyeOff,
  TrendingUp,
  Boxes,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { createProductPreviewLink } from '@zamk/api-client/src/admin';
import {
  getAdminProduct,
  blockProduct,
  publishProduct,
  hideProduct,
  getAdminProductModerationHistory,
  updateAdminProduct,
  getAdminProductErrorMessage,
} from '../api/adminProducts';
import {
  getAdminCategories,
  getAdminBrands,
} from '../api/adminOperations';
import type { AdminProductView } from '../api/adminProducts';
import { getProductStatusConfig } from '../utils/productStatusMapper';
import { formatMoneyRubles } from '../utils/money';
import { computeStockInfo } from '../utils/stock';
import { computeActualVisibility } from '../utils/productVisibility';

interface ModerationLogItem {
  id: string;
  adminName?: string;
  fromStatus?: string;
  toStatus: string;
  comment?: string;
  createdAt: string;
}

const SHOP_PUBLIC_URL = import.meta.env.VITE_SHOP_PUBLIC_URL || 'http://localhost:3000';

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

type TabId = 'overview' | 'content' | 'variants' | 'stock' | 'moderation' | 'sales' | 'reviews' | 'history';
const VALID_TABS: TabId[] = ['overview', 'content', 'variants', 'stock', 'moderation', 'sales', 'reviews', 'history'];

export function AdminProductDetail() {
  const { productId } = useParams<{ productId: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // Active dossier tab — persisted in URL ?tab=
  const tabParam = searchParams.get('tab') as TabId | null;
  const [activeTab, setActiveTab] = useState<TabId>(
    tabParam && VALID_TABS.includes(tabParam) ? tabParam : 'overview'
  );

  const handleTabChange = (tab: TabId) => {
    setActiveTab(tab);
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    setSearchParams(next, { replace: true });
  };

  // State
  const [product, setProduct] = useState<AdminProductView | null>(null);
  const [logs, setLogs] = useState<ModerationLogItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  // Backend returns { token }, we build URLs from it
  const [previewToken, setPreviewToken] = useState<string | null>(null);
  const [previewExpiresAt, setPreviewExpiresAt] = useState<string | null>(null);
  const [isGeneratingPreview, setIsGeneratingPreview] = useState(false);

  // Edit Modal State
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [editCategory, setEditCategory] = useState('');
  const [editBrand, setEditBrand] = useState('');
  const [editPrice, setEditPrice] = useState<number>(0);
  const [categoriesList, setCategoriesList] = useState<Array<{ id: string; name: string }>>([]);
  const [brandsList, setBrandsList] = useState<Array<{ id: string; name: string }>>([]);
  const [isEditSubmitting, setIsEditSubmitting] = useState(false);

  // Decision Modals State
  const [reasonModal, setReasonModal] = useState<{ type: 'hide' | 'block' | 'publish'; label: string } | null>(null);
  const [actionReason, setActionReason] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Load Categories & Brands for editing
  useEffect(() => {
    getAdminCategories()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setCategoriesList(items.map((c: any) => ({ id: c.id, name: c.name })));
      })
      .catch(() => {});

    getAdminBrands()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setBrandsList(items.map((b: any) => ({ id: b.id, name: b.name })));
      })
      .catch(() => {});
  }, []);

  const handleGeneratePreview = async () => {
    if (!product) return;
    try {
      setIsGeneratingPreview(true);
      const linkData = await createProductPreviewLink(product.id);
      setPreviewToken(linkData.pageUrl);
      if (linkData.expiresAt) {
        setPreviewExpiresAt(new Date(linkData.expiresAt).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }));
      }

      const newWin = window.open(linkData.pageUrl, '_blank');
      if (!newWin) {
        setActionError(null);
      }
    } catch (err: any) {
      console.error('Failed to create preview link:', err);
      setActionError(err.message || 'Ошибка генерации ссылки предпросмотра');
    } finally {
      setIsGeneratingPreview(false);
    }
  };

  // Fetch product detail & history logs
  const loadProductData = async () => {
    if (!productId) return;
    try {
      setIsLoading(true);
      setLoadError(null);

      const [pData, logsData] = await Promise.all([
        getAdminProduct(productId),
        getAdminProductModerationHistory(productId).catch(() => ({ items: [] })),
      ]);

      setProduct(pData);
      setLogs((logsData.items || []) as unknown as ModerationLogItem[]);

      // Populate edit fields (editPrice in Rubles)
      setEditTitle(pData.title);
      setEditDescription(pData.description || '');
      setEditPrice(pData.price);
      setEditCategory(pData.categoryId || '');
      setEditBrand(pData.brandId || '');
    } catch (err: unknown) {
      console.error('[AdminProductDetail] Failed to load product:', { productId, error: err });
      setLoadError(getAdminProductErrorMessage(err, 'Не удалось загрузить карточку товара.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadProductData();
  }, [productId]);

  if (isLoading) {
    return (
      <div className="bg-white dark:bg-slate-900 rounded-xl p-16 text-center border border-slate-200 dark:border-slate-800">
        <div className="animate-spin rounded-full h-10 w-10 border-2 border-indigo-600 border-t-transparent mx-auto"></div>
        <p className="mt-4 text-sm font-medium text-slate-600 dark:text-slate-400">Загрузка досье товара...</p>
      </div>
    );
  }

  if (loadError || !product) {
    return (
      <div className="space-y-4">
        <button
          type="button"
          onClick={() => navigate('/products')}
          className="inline-flex items-center gap-2 text-sm font-medium text-slate-600 dark:text-slate-300 hover:text-slate-900"
        >
          <ArrowLeft className="h-4 w-4" />
          Вернуться в каталог
        </button>
        <div className="p-6 bg-rose-50 dark:bg-rose-950/50 rounded-xl border border-rose-200 dark:border-rose-800 text-rose-700 dark:text-rose-300 flex items-center justify-between gap-4">
          <div>
            <AlertCircle className="h-6 w-6 mb-2" />
            <h3 className="font-semibold text-base">Ошибка загрузки товара</h3>
            <p className="text-sm mt-1">{loadError || 'Товар не найден или был удалён.'}</p>
          </div>
          <button
            type="button"
            onClick={() => loadProductData()}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium text-xs rounded-lg transition-all shadow-sm flex-shrink-0"
          >
            Повторить загрузку
          </button>
        </div>
      </div>
    );
  }

  const statusCfg = getProductStatusConfig(product.status);
  const stockInfo = computeStockInfo(product.stock, product.reservedStock, product.variants?.length ?? 1, true);
  const visibilityInfo = computeActualVisibility({
    status: product.status,
    sellerStatus: product.sellerStatus,
    stock: product.stock,
    variantsCount: product.variants?.length,
  });

  // Perform Edit Submit
  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!product) return;
    try {
      setIsEditSubmitting(true);
      setActionError(null);

      await updateAdminProduct(product.id, {
        title: editTitle.trim(),
        description: editDescription.trim(),
        categoryId: editCategory || undefined,
        brandId: editBrand || undefined,
        priceCents: editPrice > 0 ? Math.round(editPrice * 100) : undefined,
      });

      setIsEditModalOpen(false);
      await loadProductData();
    } catch (err: any) {
      setActionError(getAdminProductErrorMessage(err, 'Не удалось сохранить изменения товара.'));
    } finally {
      setIsEditSubmitting(false);
    }
  };

  // Perform Status Action (Hide / Block / Publish)
  const handleStatusActionSubmit = async () => {
    if (!reasonModal || !product) return;
    try {
      setIsSubmitting(true);
      setActionError(null);

      if (reasonModal.type === 'hide') {
        await hideProduct(product.id, actionReason || 'Скрыт администратором');
      } else if (reasonModal.type === 'block') {
        await blockProduct(product.id, actionReason || 'Заблокирован администратором');
      } else if (reasonModal.type === 'publish') {
        await publishProduct(product.id, actionReason || 'Опубликован администратором');
      }

      setReasonModal(null);
      setActionReason('');
      await loadProductData();
    } catch (err: any) {
      const reasons = err?.reasons || err?.data?.reasons || (err?.data && Array.isArray(err?.data) ? err.data : null);
      if (reasons && Array.isArray(reasons)) {
        const reasonLabels: Record<string, string> = {
          seller_inactive: 'Продавец неактивен',
          product_hidden: 'Товар скрыт',
          product_blocked: 'Товар заблокирован',
          moderation_required: 'Требуется прохождение модерации',
          no_active_variants: 'Нет активных вариантов',
          invalid_price: 'Некорректная цена',
          no_inventory: 'Нет складской записи',
          out_of_stock: 'Нет доступного остатка',
        };
        const localized = reasons.map((r: string) => reasonLabels[r] || r).join('. ');
        setActionError(`Не удалось вернуть товар на витрину. Причины: ${localized}`);
      } else {
        setActionError(getAdminProductErrorMessage(err, 'Ошибка выполнения действия.'));
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="space-y-6 pb-24">
      {actionError && (
        <div className="bg-rose-50 dark:bg-rose-950/60 border border-rose-300 dark:border-rose-800 rounded-2xl p-4 flex items-center justify-between shadow-sm">
          <div className="flex items-center gap-3">
            <AlertTriangle className="h-5 w-5 text-rose-600 dark:text-rose-400 flex-shrink-0" />
            <div>
              <h4 className="text-sm font-bold text-rose-900 dark:text-rose-200">Ошибка выполнения действия</h4>
              <p className="text-xs text-rose-700 dark:text-rose-300 mt-0.5 font-medium">{actionError}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setActionError(null)}
            className="text-xs px-3 py-1 bg-rose-100 dark:bg-rose-900/50 text-rose-800 dark:text-rose-200 rounded-lg hover:bg-rose-200 transition-colors font-semibold"
          >
            Закрыть
          </button>
        </div>
      )}
      {/* Top Navigation & Action Header */}
      <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={() => navigate('/products')}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 rounded-xl transition-all"
            >
              <ArrowLeft className="h-4 w-4" />
              К каталогу товаров
            </button>

            <h1 className="text-xl font-bold text-slate-900 dark:text-white truncate max-w-md" title={product.title}>
              {product.title}
            </h1>

            <span data-testid="badge-product-status" className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border ${statusCfg.badgeClass}`}>
              <span className={`h-1.5 w-1.5 rounded-full ${statusCfg.dotClass}`} />
              <span>{statusCfg.label}</span>
            </span>

            {/* Actual Visibility Badge */}
            <span className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium border ${visibilityInfo.badgeClass}`}>
              {visibilityInfo.isVisible ? <CheckCircle2 className="w-3 h-3 text-emerald-600" /> : <XCircle className="w-3 h-3 text-amber-600" />}
              <span>{visibilityInfo.reasonLabel}</span>
            </span>
          </div>

          {/* Action Buttons Toolbar */}
          <div className="flex flex-wrap items-center gap-2">
            {/* Open on Shop only if actually visible */}
            {product.actualVisibility && (product.storefrontUrl || SHOP_PUBLIC_URL) ? (
              <a
                data-testid="btn-open-storefront"
                href={product.storefrontUrl || `${SHOP_PUBLIC_URL}/product/${product.slug || product.id}`}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold text-emerald-700 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-300 dark:border-emerald-800 rounded-xl hover:bg-emerald-100 transition-all"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                <span>Открыть на витрине</span>
              </a>
            ) : null}

            {!product.actualVisibility && (
              <button
                type="button"
                data-testid="btn-preview"
                onClick={() => handleGeneratePreview()}
                disabled={isGeneratingPreview}
                className="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold text-indigo-700 dark:text-indigo-300 bg-indigo-50 dark:bg-indigo-950/60 border border-indigo-300 dark:border-indigo-800 rounded-xl hover:bg-indigo-100 transition-all disabled:opacity-50"
              >
                <Eye className="h-3.5 w-3.5" />
                <span>{isGeneratingPreview ? 'Создание ссылки...' : 'Открыть предпросмотр'}</span>
              </button>
            )}

            {/* Restore to storefront for approved or hidden products that are currently hidden/not visible */}
            {(product.status === 'hidden' || (product.status === 'approved' && !product.actualVisibility)) && (
              <button
                type="button"
                data-testid="btn-restore"
                onClick={() => setReasonModal({ type: 'publish', label: 'Вернуть товар на витрину' })}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold text-emerald-700 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-300 rounded-xl hover:bg-emerald-100 transition-all"
              >
                <Eye className="h-3.5 w-3.5" />
                <span>Вернуть на витрину</span>
              </button>
            )}

            {/* Moderation shortcut if pending/in_review — opens the specific moderation dossier directly */}
            {(product.status === 'pending_moderation' || product.status === 'in_review') && (
              <button
                type="button"
                onClick={() => navigate(`/moderation/products/${product.id}`)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-950/60 border border-amber-300 rounded-xl hover:bg-amber-100 transition-all"
              >
                <ShieldAlert className="h-3.5 w-3.5" />
                <span>Открыть в модерации</span>
              </button>
            )}

            {/* Seller Dossier */}
            {product.sellerId && (
              <button
                type="button"
                onClick={() => navigate(`/sellers/${product.sellerId}?tab=catalog`)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 rounded-xl transition-all"
              >
                <Store className="h-3.5 w-3.5 text-indigo-600" />
                <span>Досье продавца</span>
              </button>
            )}

            {/* Edit Button */}
            <button
              type="button"
              onClick={() => setIsEditModalOpen(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-800 dark:text-slate-100 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 border border-slate-200 dark:border-slate-700 rounded-xl transition-all"
            >
              <Edit3 className="h-3.5 w-3.5" />
              <span>Редактировать</span>
            </button>

            {/* Hide / Block Buttons */}
            {(product.status === 'published' || (product.status === 'approved' && product.actualVisibility)) && (
              <button
                type="button"
                onClick={() => setReasonModal({ type: 'hide', label: 'Скрыть товар с витрины' })}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-rose-700 dark:text-rose-300 bg-rose-50 dark:bg-rose-950/60 border border-rose-200 rounded-xl hover:bg-rose-100 transition-all"
              >
                <EyeOff className="h-3.5 w-3.5" />
                <span>Скрыть</span>
              </button>
            )}

            {product.status !== 'blocked' && (
              <button
                type="button"
                onClick={() => setReasonModal({ type: 'block', label: 'Заблокировать товар' })}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-950/60 border border-red-300 rounded-xl hover:bg-red-200 transition-all"
              >
                <Lock className="h-3.5 w-3.5" />
                <span>Заблокировать</span>
              </button>
            )}
          </div>
        </div>

        {previewToken && (
          <div className="p-3 bg-indigo-50 dark:bg-indigo-950/40 rounded-xl border border-indigo-200 dark:border-indigo-800 text-xs space-y-1 text-indigo-900 dark:text-indigo-200">
            <div className="flex items-center justify-between gap-2">
              <span className="font-semibold">Ссылка предпросмотра{previewExpiresAt ? ` (до ${previewExpiresAt})` : ' (15 мин)'}:</span>
              <button
                type="button"
                onClick={() => { setPreviewToken(null); setPreviewExpiresAt(null); }}
                className="text-indigo-400 hover:text-indigo-700 flex-shrink-0"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
            <a
              href={previewToken}
              target="_blank"
              rel="noreferrer"
              className="underline text-indigo-700 dark:text-indigo-300 break-all hover:text-indigo-900 block"
            >
              Открыть предпросмотр вручную →
            </a>
            <button
              type="button"
              onClick={() => handleGeneratePreview()}
              disabled={isGeneratingPreview}
              className="text-[11px] text-indigo-500 hover:text-indigo-700 underline"
            >
              Создать новую ссылку
            </button>
          </div>
        )}

        {/* Store & Timestamps info */}
        <div className="flex flex-wrap items-center gap-6 text-xs text-slate-500 dark:text-slate-400 pt-3 border-t border-slate-100 dark:border-slate-800">
          <div className="flex items-center gap-1.5">
            <Store className="h-4 w-4 text-slate-400" />
            <span>Магазин: </span>
            <strong className="text-slate-900 dark:text-white font-medium">{product.sellerName || 'ZAMK Seller'}</strong>
          </div>
          <div className="flex items-center gap-1.5">
            <Clock className="h-4 w-4 text-slate-400" />
            <span>Создан: </span>
            <strong className="text-slate-900 dark:text-white font-medium">
              {product.createdAt ? new Date(product.createdAt).toLocaleDateString('ru-RU') : '—'}
            </strong>
          </div>
          <div className="flex items-center gap-1.5">
            <Tag className="h-4 w-4 text-slate-400" />
            <span>ID товара: </span>
            <span className="font-mono text-slate-700 dark:text-slate-300">{product.id}</span>
          </div>
        </div>
      </div>

      {/* 8 Dossier Tabs Navigation (Clean labels, smooth horizontal scroll, persisted in URL ?tab=) */}
      <div className="border-b border-slate-200 dark:border-slate-800 overflow-x-auto scrollbar-thin">
        <nav className="flex space-x-2 pb-px min-w-max">
          {([
            { id: 'overview', label: 'Обзор', icon: Package },
            { id: 'content', label: 'Контент и фото', icon: Layers },
            { id: 'variants', label: 'Варианты и цены', icon: DollarSign },
            { id: 'stock', label: 'Склад', icon: Boxes },
            { id: 'moderation', label: 'Публикация и модерация', icon: ShieldAlert },
            { id: 'sales', label: 'Продажи', icon: TrendingUp },
            { id: 'reviews', label: 'Качество и отзывы', icon: Star },
            { id: 'history', label: 'История', icon: History },
          ] as { id: TabId; label: string; icon: any }[]).map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => handleTabChange(tab.id)}
                className={`py-2.5 px-4 text-xs font-medium rounded-t-xl transition-all flex items-center gap-2 whitespace-nowrap ${
                  isActive
                    ? 'bg-white dark:bg-slate-900 text-indigo-600 dark:text-indigo-400 border-t-2 border-x border-indigo-600 dark:border-indigo-400 shadow-sm font-semibold'
                    : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100/50'
                }`}
              >
                <Icon className="w-3.5 h-3.5" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
      </div>

      {/* TAB CONTENT PANELS */}

      {/* TAB: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="space-y-6 animate-in fade-in">
          {/* Key Metrics Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white dark:bg-slate-900 p-4 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm">
              <span className="text-xs text-slate-500 block">Базовая цена</span>
              <span className="text-xl font-bold text-slate-900 dark:text-white mt-1 block">
                {formatMoneyRubles(product.price)}
              </span>
            </div>
            <div className="bg-white dark:bg-slate-900 p-4 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm">
              <span className="text-xs text-slate-500 block">Доступный остаток</span>
              <span className={`text-xl font-bold mt-1 block ${stockInfo.availableStock > 0 ? 'text-emerald-600' : 'text-orange-600'}`}>
                {stockInfo.label}
              </span>
            </div>
            <div className="bg-white dark:bg-slate-900 p-4 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm">
              <span className="text-xs text-slate-500 block">Рейтинг</span>
              <span className="text-xl font-bold text-amber-600 mt-1 block">
                {product.rating && product.rating > 0 ? `★ ${product.rating.toFixed(1)}` : 'Нет отзывов'}
              </span>
            </div>
            <div className="bg-white dark:bg-slate-900 p-4 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm">
              <span className="text-xs text-slate-500 block">Варианты</span>
              <span className="text-xl font-bold text-slate-900 dark:text-white mt-1 block">
                {product.variants ? product.variants.length : 0} шт.
              </span>
            </div>
          </div>

          {/* Details Overview Grid */}
          <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white border-b pb-2">Основные характеристики</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
              <div>
                <span className="text-slate-500 block">Категория:</span>
                <span className="font-medium text-slate-900 dark:text-white">{product.category || 'Не указана'}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Бренд:</span>
                <span className="font-medium text-slate-900 dark:text-white">{product.brand || 'Без бренда'}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Продавец:</span>
                <span className="font-medium text-indigo-600">{product.sellerName || '—'}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Источник:</span>
                <span className="font-medium text-slate-900 dark:text-white">Продавец</span>
              </div>
              <div>
                <span className="text-slate-500 block">Дата создания:</span>
                <span className="font-medium text-slate-900 dark:text-white">
                  {product.createdAt ? new Date(product.createdAt).toLocaleString('ru-RU') : '—'}
                </span>
              </div>
              <div>
                <span className="text-slate-500 block">Дата обновления:</span>
                <span className="font-medium text-slate-900 dark:text-white">
                  {product.updatedAt ? new Date(product.updatedAt).toLocaleString('ru-RU') : '—'}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB: CONTENT & PHOTOS */}
      {activeTab === 'content' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-6 animate-in fade-in">
          <div>
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white mb-3">Главное изображение и галерея</h3>
            <div className="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-6 gap-3">
              {product.image && (
                <div className="relative rounded-xl overflow-hidden border-2 border-indigo-600 bg-slate-100 aspect-[4/5]">
                  <img src={product.image} alt="Главное" className="w-full h-full object-cover" />
                  <span className="absolute bottom-1 left-1 bg-indigo-600 text-white text-[9px] px-1.5 py-0.5 rounded font-bold">Главное</span>
                </div>
              )}
              {product.gallery?.map((img: any, idx: number) => (
                <div key={idx} className="rounded-xl overflow-hidden border border-slate-200 dark:border-slate-700 bg-slate-100 aspect-[4/5]">
                  <img src={typeof img === 'string' ? img : img.url} alt={`Галерея ${idx}`} className="w-full h-full object-cover" />
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Описание товара</h3>
            <p className="text-xs text-slate-700 dark:text-slate-300 leading-relaxed bg-slate-50 dark:bg-slate-800 p-4 rounded-xl">
              {product.description || 'Описание не заполнено продавцом.'}
            </p>
          </div>
        </div>
      )}

      {/* TAB: VARIANTS & PRICES */}
      {activeTab === 'variants' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Варианты товара ({product.variants?.length || 0})</h3>
            <p className="text-[11px] text-slate-400">«Наличие» определяется складской записью (вкладка Склад)</p>
          </div>
          {!product.variants || product.variants.length === 0 ? (
            <div className="p-6 text-center text-xs text-rose-500 bg-rose-50 dark:bg-rose-950/40 rounded-xl">
              ⚠️ У товара отсутствуют варианты! Продавец должен добавить хотя бы один вариант.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="border-b bg-slate-50 dark:bg-slate-800 text-slate-500 uppercase text-[10px]">
                    <th className="py-2.5 px-3">Цвет / Размер</th>
                    <th className="py-2.5 px-3">Артикул продавца (SKU)</th>
                    <th className="py-2.5 px-3">Штрихкод</th>
                    <th className="py-2.5 px-3 text-right">Цена</th>
                    <th className="py-2.5 px-3 text-center">Статус варианта</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                  {product.variants.map((v, i) => (
                    <tr key={v.id || i} className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
                      <td className="py-2.5 px-3 font-medium text-slate-900 dark:text-white">
                        {[v.color ? (v.shadeName ? `${v.color} (${v.shadeName})` : v.color) : null, v.size].filter(Boolean).join(' / ') || '—'}
                      </td>
                      <td className="py-2.5 px-3 font-mono text-slate-600 dark:text-slate-400">{v.sellerSku || v.sku || '—'}</td>
                      <td className="py-2.5 px-3 font-mono text-slate-500 text-[11px]">{v.barcode || '—'}</td>
                      <td className="py-2.5 px-3 text-right font-semibold text-slate-900 dark:text-white">
                        {v.price != null ? formatMoneyRubles(v.price) : '—'}
                      </td>
                      <td className="py-2.5 px-3 text-center">
                        <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium ${
                          v.isActive
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : 'bg-slate-100 text-slate-500 border border-slate-200'
                        }`}>
                          {v.isActive ? '✓ Активен' : '✕ Неактивен'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {/* Stock info note */}
          <div className="text-[11px] text-slate-400 bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl">
            ℹ️ Складской остаток и резервирование отображаются на вкладке <button onClick={() => handleTabChange('stock')} className="underline text-indigo-500 hover:text-indigo-700">Склад</button>.
            Статус варианта «Активен» означает только что вариант доступен для заказа — не гарантирует наличие на складе.
          </div>
        </div>
      )}

      {/* TAB: STOCK */}
      {activeTab === 'stock' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Складской остаток</h3>
            <button
              onClick={() => navigate(`/inventory?productId=${product.id}`)}
              className="text-xs font-semibold text-indigo-600 hover:underline flex items-center gap-1"
            >
              <span>Открыть в Складе Admin</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </button>
          </div>

          {product.stock === undefined || product.stock === null ? (
            <div className="p-6 text-center bg-amber-50 dark:bg-amber-950/40 rounded-xl border border-amber-200">
              <Boxes className="w-8 h-8 text-amber-400 mx-auto mb-2" />
              <p className="text-xs font-semibold text-amber-700">Нет складской записи</p>
              <p className="text-[11px] text-amber-600 mt-1">
                Backend не вернул данные об остатке для этого товара.
                Перейдите в раздел <button onClick={() => navigate('/inventory')} className="underline">Склад</button> для управления остатком.
              </p>
            </div>
          ) : (
            <div className="p-4 bg-slate-50 dark:bg-slate-800 rounded-xl text-xs space-y-2">
              <div className="flex justify-between">
                <span className="text-slate-500">Общий складской остаток:</span>
                <span className="font-bold text-slate-900 dark:text-white">{product.stock} шт.</span>
              </div>
              <div className="flex justify-between">
                <span className="text-slate-500">Зарезервировано:</span>
                <span className="font-bold text-slate-700 dark:text-slate-300">{product.reservedStock ?? 0} шт.</span>
              </div>
              <div className="flex justify-between border-t pt-2">
                <span className="text-slate-500 font-semibold">Доступно к заказу:</span>
                <span className={`font-bold ${stockInfo.availableStock > 0 ? 'text-emerald-600' : 'text-orange-600'}`}>
                  {stockInfo.label}
                </span>
              </div>
            </div>
          )}
        </div>
      )}

      {/* TAB: MODERATION & LOGS */}
      {activeTab === 'moderation' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <h3 className="text-sm font-semibold text-slate-900 dark:text-white">История решений модерации</h3>
          {logs.length === 0 ? (
            <p className="text-xs text-slate-400">История решений модерации пуста.</p>
          ) : (
            <div className="space-y-3">
              {logs.map((log) => (
                <div key={log.id} className="p-3 bg-slate-50 dark:bg-slate-800 rounded-xl border text-xs space-y-1">
                  <div className="flex justify-between font-semibold">
                    <span>Переход: {localizeStatus(log.fromStatus)} → {localizeStatus(log.toStatus)}</span>
                    <span className="text-slate-400">{new Date(log.createdAt).toLocaleString('ru-RU')}</span>
                  </div>
                  {log.comment && <p className="text-slate-600 dark:text-slate-300">{cleanComment(log.comment)}</p>}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* TAB: SALES */}
      {activeTab === 'sales' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Показатели продаж</h3>
            <button
              onClick={() => navigate(`/orders?productId=${product.id}`)}
              className="text-xs font-semibold text-indigo-600 hover:underline flex items-center gap-1"
            >
              <span>Заказы с товаром</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </button>
          </div>
          <div className="p-6 text-center text-xs text-slate-400 bg-slate-50 dark:bg-slate-800 rounded-xl">
            По данному товару пока нет оформленных заказов.
          </div>
        </div>
      )}

      {/* TAB: REVIEWS */}
      {activeTab === 'reviews' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Отзывы покупателей</h3>
            <button
              onClick={() => navigate(`/reviews?productId=${product.id}`)}
              className="text-xs font-semibold text-indigo-600 hover:underline flex items-center gap-1"
            >
              <span>Все отзывы в Admin</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </button>
          </div>
          <div className="p-6 text-center text-xs text-slate-400 bg-slate-50 dark:bg-slate-800 rounded-xl">
            Отзывов покупателей по этому товару еще нет.
          </div>
        </div>
      )}

      {/* TAB: HISTORY (Real audit logs) */}
      {activeTab === 'history' && (
        <div className="bg-white dark:bg-slate-900 p-6 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4 animate-in fade-in">
          <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Аудит изменений товара</h3>
          {logs.length === 0 ? (
            <div className="p-8 text-center text-xs text-slate-400 bg-slate-50 dark:bg-slate-800/50 rounded-xl">
              История изменений пока отсутствует.
            </div>
          ) : (
            <div className="space-y-3 text-xs">
              {logs.map((item) => (
                <div key={item.id} className="p-3 bg-slate-50 dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 flex justify-between items-start gap-4">
                  <div>
                    <span className="font-semibold text-slate-900 dark:text-white block">
                      Изменение статуса: {localizeStatus(item.fromStatus)} → {localizeStatus(item.toStatus)}
                    </span>
                    {item.comment && <p className="text-slate-600 dark:text-slate-300 mt-1">{cleanComment(item.comment)}</p>}
                  </div>
                  <span className="text-slate-400 whitespace-nowrap">{new Date(item.createdAt).toLocaleString('ru-RU')}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ADMIN EDIT MODAL */}
      {isEditModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl shadow-2xl max-w-lg w-full p-6 border border-slate-200 dark:border-slate-800 space-y-4">
            <div className="flex items-center justify-between border-b pb-3">
              <h3 className="text-base font-bold text-slate-900 dark:text-white">Редактирование товара (Admin)</h3>
              <button onClick={() => setIsEditModalOpen(false)} className="text-slate-400 hover:text-slate-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleEditSubmit} className="space-y-3 text-xs">
              <div>
                <label className="font-medium text-slate-700 dark:text-slate-300 block mb-1">Название товара</label>
                <input
                  type="text"
                  required
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-50 dark:bg-slate-800 border rounded-xl"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-medium text-slate-700 dark:text-slate-300 block mb-1">Категория</label>
                  <select
                    value={editCategory}
                    onChange={(e) => setEditCategory(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-50 dark:bg-slate-800 border rounded-xl"
                  >
                    <option value="">-- Не выбрана --</option>
                    {categoriesList.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="font-medium text-slate-700 dark:text-slate-300 block mb-1">Бренд</label>
                  <select
                    value={editBrand}
                    onChange={(e) => setEditBrand(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-50 dark:bg-slate-800 border rounded-xl"
                  >
                    <option value="">-- Без бренда --</option>
                    {brandsList.map((b) => (
                      <option key={b.id} value={b.id}>
                        {b.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div>
                <label className="font-medium text-slate-700 dark:text-slate-300 block mb-1">Цена (₽)</label>
                <input
                  type="number"
                  min="0"
                  step="1"
                  value={editPrice}
                  onChange={(e) => setEditPrice(parseFloat(e.target.value) || 0)}
                  className="w-full px-3 py-2 bg-slate-50 dark:bg-slate-800 border rounded-xl font-semibold"
                />
              </div>

              <div>
                <label className="font-medium text-slate-700 dark:text-slate-300 block mb-1">Описание товара</label>
                <textarea
                  rows={4}
                  value={editDescription}
                  onChange={(e) => setEditDescription(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-50 dark:bg-slate-800 border rounded-xl"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-3 border-t">
                <button
                  type="button"
                  onClick={() => setIsEditModalOpen(false)}
                  className="px-4 py-2 font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 rounded-xl"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isEditSubmitting || !editTitle.trim()}
                  className="px-4 py-2 font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-xl shadow"
                >
                  {isEditSubmitting ? 'Сохранение...' : 'Сохранить'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* REASON MODAL FOR HIDE / BLOCK / PUBLISH */}
      {reasonModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl shadow-2xl max-w-md w-full p-6 border border-slate-200 dark:border-slate-800 space-y-4">
            <h3 className="text-base font-bold text-slate-900 dark:text-white">{reasonModal.label}</h3>
            <div>
              <label className="text-xs font-medium text-slate-700 dark:text-slate-300 block mb-1">
                Укажите причину действия (обязательно)
              </label>
              <textarea
                value={actionReason}
                onChange={(e) => setActionReason(e.target.value)}
                placeholder="Причина действия..."
                rows={3}
                className="w-full p-2.5 text-xs bg-slate-50 dark:bg-slate-800 border rounded-xl focus:outline-none"
              />
            </div>
            <div className="flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setReasonModal(null)}
                className="px-4 py-2 text-xs font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 rounded-xl"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleStatusActionSubmit}
                disabled={isSubmitting || !actionReason.trim()}
                className="px-4 py-2 text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 rounded-xl shadow"
              >
                {isSubmitting ? 'Выполнение...' : 'Подтвердить'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import {
  AlertTriangle,
  ArrowLeft,
  BarChart3,
  Eye,
  Edit2,
  PackagePlus,
  Search,
  ShoppingBag,
  Sparkles,
  type LucideIcon,
} from 'lucide-react';
import {
  statusLabels,
  type SellerProduct,
  type SellerProductStatus,
} from '../lib/seller-products';
import { getSellerProducts, getSellerMe } from '@zamk/api-client/src/seller';
import { adaptProductList } from '../api/adapter';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const numberFormatter = new Intl.NumberFormat('ru-RU');
const formatCurrency = (value: number) => currencyFormatter.format(value);
const formatNumber = (value: number) => numberFormatter.format(value);

const statusFilterOptions: Array<{ value: SellerProductStatus | 'all'; label: string }> = [
  { value: 'all', label: 'Все статусы' },
  { value: 'published', label: statusLabels.published },
  { value: 'draft', label: statusLabels.draft },
  { value: 'pending_moderation', label: statusLabels.pending_moderation },
  { value: 'in_review', label: statusLabels.in_review },
  { value: 'approved', label: statusLabels.approved },
  { value: 'rejected', label: statusLabels.rejected },
  { value: 'hidden', label: statusLabels.hidden },
  { value: 'blocked', label: statusLabels.blocked },
  { value: 'out_of_stock', label: statusLabels.out_of_stock },
];

function ProductBadge({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'neutral' | 'good' | 'warning' | 'danger' | 'info' }) {
  const styles = {
    neutral: 'bg-ice text-graphite dark:bg-white/10 dark:text-white/76',
    good: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300',
    warning: 'bg-amber-50 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300',
    danger: 'bg-red-50 text-red-700 dark:bg-red-400/10 dark:text-red-300',
    info: 'bg-sky-50 text-sky-700 dark:bg-sky-400/10 dark:text-sky-300',
  };

  return <span className={cn('inline-flex rounded-full px-3 py-1 text-xs font-semibold', styles[tone])}>{children}</span>;
}

function getStatusTone(status: SellerProductStatus) {
  const tones: Record<SellerProductStatus, 'neutral' | 'good' | 'warning' | 'danger' | 'info'> = {
    draft: 'neutral',
    pending_moderation: 'info',
    in_review: 'info',
    approved: 'good',
    published: 'good',
    rejected: 'danger',
    hidden: 'warning',
    blocked: 'danger',
    out_of_stock: 'warning'
  };

  return tones[status] || 'neutral';
}

function SummaryCard({ label, value, hint, icon: Icon }: { label: string; value: string; hint: string; icon: LucideIcon }) {
  return (
    <article className="glass-panel p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">{label}</p>
          <p className="mt-3 text-3xl font-semibold text-graphite dark:text-white">{value}</p>
          <p className="mt-2 text-sm text-graphite-light dark:text-white/68">{hint}</p>
        </div>
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-white/75 text-graphite dark:bg-white/10 dark:text-white">
          <Icon className="h-5 w-5" />
        </span>
      </div>
    </article>
  );
}

function ProductAvatar({ product }: { product: SellerProduct }) {
  if (product.mainPhoto && product.mainPhoto.startsWith('http')) {
    return (
      <img src={product.mainPhoto} alt={product.title} className="h-14 w-14 shrink-0 rounded-2xl object-cover shadow-sm" />
    );
  }
  
  const initials = product.title
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'ПР';

  return (
    <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-graphite to-accent text-sm font-bold text-white shadow-sm dark:from-white dark:to-accent dark:text-black">
      {initials}
    </div>
  );
}

function ProductDetailPanel({ product, sellerStatus }: { product: SellerProduct; sellerStatus: string }) {
  const isApprovedAndNoStock = product.status === 'approved' && (!product.sizes.length || product.sizes.every(s => s.stock === 0));

  return (
    <aside className="glass-panel-strong p-6 md:p-8">
      <div className="flex items-start gap-4">
        <ProductAvatar product={product} />
        <div>
          <p className="studio-label">{product.sku}</p>
          <h2 className="mt-2 text-3xl font-serif leading-tight text-graphite dark:text-white">{product.title}</h2>
          <div className="mt-3 flex flex-wrap gap-2">
            <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
            {isApprovedAndNoStock && (
              <ProductBadge tone="warning">Требуется поставка</ProductBadge>
            )}
            {(product.status === 'published' && (product.sizes?.reduce((sum, s) => sum + (s.stock || 0), 0) || 0) > 0) && (
              <ProductBadge tone="good">В наличии</ProductBadge>
            )}
          </div>
          {product.status === 'rejected' && product.rejectionReason && (
            <div className="mt-4 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200">
              <span className="mb-1 block font-semibold">Причина отклонения:</span>
              {product.rejectionReason}
            </div>
          )}
        </div>
      </div>

      <div className="mt-7 grid gap-3 sm:grid-cols-2">
        <div className="rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Цена</p>
          <p className="mt-2 text-lg font-semibold text-graphite dark:text-white">{formatCurrency(product.price)}</p>
        </div>
        <div className="rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Склад ZAMK</p>
          <p className="mt-2 text-sm font-medium text-graphite dark:text-white">
            Ожидается поставка
          </p>
        </div>
      </div>

      <div className="mt-7 rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Варианты (SKU)</p>
        <div className="mt-3 flex flex-col gap-2">
          {product.sizes.map((item) => (
            <div key={item.size} className="flex justify-between items-center rounded-xl border border-border-lighter px-3 py-2 text-sm text-graphite dark:border-white/16 dark:text-white/78">
              <span>{item.size}</span>
              <span className="text-graphite-light">ZAMK: 0 шт.</span>
            </div>
          ))}
        </div>
      </div>

      {isApprovedAndNoStock && (
        <div className="mt-5 rounded-xl bg-blue-50 p-4 text-sm text-blue-800">
          Товар одобрен. Для старта продаж необходимо оформить поставку на склад ZAMK.
        </div>
      )}

      <p className="mt-5 text-sm leading-relaxed text-graphite-light dark:text-white/68 line-clamp-3">{product.description}</p>
      
      <div className="mt-6 flex flex-col">
        {sellerStatus === 'blocked' || sellerStatus === 'archived' ? (
          <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
            Действия недоступны из-за статуса магазина.
          </div>
        ) : (
          <Link 
            to={`/products/${product.id}/edit`}
            className="inline-flex h-12 items-center justify-center gap-2 rounded-full border border-border-lighter bg-white/75 px-6 text-sm font-semibold text-graphite transition-colors hover:bg-white dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
          >
            {['pending_moderation', 'in_review', 'approved', 'published', 'hidden', 'blocked'].includes(product.status) ? (
              <>
                <Eye className="h-4 w-4" />
                Просмотр товара
              </>
            ) : product.status === 'rejected' ? (
              <>
                <AlertTriangle className="h-4 w-4 text-red-500" />
                Исправить карточку
              </>
            ) : (
              <>
                <Edit2 className="h-4 w-4" />
                Продолжить заполнение
              </>
            )}
          </Link>
        )}
      </div>
    </aside>
  );
}

export function SellerProducts() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<SellerProductStatus | 'all'>('all');
  const [selectedId, setSelectedId] = useState('');
  const [sellerStatus, setSellerStatus] = useState<string>('active');

  const loadData = useCallback(async (silent = false) => {
    try {
      if (!silent) {
        setIsLoading(true);
        setError('');
      }
      const [me, rawProducts] = await Promise.all([getSellerMe(), getSellerProducts()]);
      setSellerStatus(me.seller.status);
      const adapted = adaptProductList(rawProducts);
      setProducts(adapted);
      setSelectedId((prev) => {
        if (prev && adapted.some((p) => p.id === prev)) return prev;
        return adapted.length > 0 ? adapted[0].id : '';
      });
    } catch (err: any) {
      if (!silent) {
        setError(err.message || 'Ошибка загрузки товаров');
      }
    } finally {
      if (!silent) {
        setIsLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    loadData(false);
  }, [loadData]);

  useVisibilityPolling(useCallback(() => loadData(true), [loadData]), 4000);

  const filteredProducts = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();

    return products.filter((product) => {
      const matchesQuery = !normalizedQuery || [product.title, product.sku, product.category].some((item) => item.toLowerCase().includes(normalizedQuery));
      const matchesStatus = status === 'all' || product.status === status;
      return matchesQuery && matchesStatus;
    });
  }, [products, query, status]);

  const selectedProduct = products.find((product) => product.id === selectedId) || filteredProducts[0] || products[0];
  const moderationCount = products.filter((product) => product.status === 'pending_moderation' || product.status === 'in_review').length;
  const approvedCount = products.filter((product) => product.status === 'approved' || product.status === 'published').length;
  const revenue = products.reduce((sum, product) => sum + product.revenue, 0);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 md:pt-28 md:pb-20 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  if (error) {
    return <div className="min-h-screen pt-24 pb-24 md:pt-28 md:pb-20 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="relative z-10 min-h-screen pt-24 pb-24 md:pt-28 md:pb-20">
      <div className="container mx-auto max-w-[1400px] px-4 sm:px-6">
        <Link to="/dashboard" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white">
          <ArrowLeft className="h-4 w-4" />
          Кабинет продавца
        </Link>

        <section className="mt-6 glass-panel-strong p-7 md:p-10">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="studio-label">Ассортимент</p>
              <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-6xl">Мои товары</h1>
              <p className="studio-subtitle mt-4 max-w-3xl">
                Управляйте карточками товаров. После модерации необходимо оформить поставку на склад ZAMK для старта продаж.
              </p>
            </div>
            <Link
              to="/products/new"
              className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-graphite px-6 text-sm font-semibold text-white transition-colors hover:bg-graphite-light dark:bg-white dark:text-black dark:hover:bg-white/86"
            >
              <PackagePlus className="h-4 w-4" />
              Добавить товар
            </Link>
          </div>
        </section>

        <section className="mt-6 grid gap-4 md:grid-cols-3 xl:grid-cols-4">
          <SummaryCard label="Всего карточек" value={formatNumber(products.length)} hint="создано в системе" icon={ShoppingBag} />
          <SummaryCard label="Одобрено" value={formatNumber(approvedCount)} hint="готово к поставке" icon={Sparkles} />
          <SummaryCard label="На проверке" value={formatNumber(moderationCount)} hint="ожидают решения" icon={AlertTriangle} />
          <div className="hidden xl:block">
             <SummaryCard label="Текущая выручка" value={formatCurrency(revenue)} hint="данные отсутствуют" icon={BarChart3} />
          </div>
        </section>

        <section className="mt-6 glass-panel-strong p-5 md:p-6">
          <div className="grid gap-3 lg:grid-cols-[1fr_220px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-ash" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Поиск по названию или категории"
                className="seller-setting-input h-12 w-full rounded-2xl border border-border-lighter bg-white/78 pl-11 pr-4 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
              />
            </label>
            <select
              value={status}
              onChange={(event) => setStatus(event.target.value as SellerProductStatus | 'all')}
              className="seller-setting-input h-12 rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none dark:border-white/16 dark:bg-black/24 dark:text-white"
            >
              {statusFilterOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </section>

          {products.length === 0 ? (
            <div className="flex flex-col items-center justify-center p-12 text-center">
              <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-ice text-graphite dark:bg-white/10 dark:text-white">
                <PackagePlus className="h-8 w-8" />
              </div>
              <h2 className="mb-2 text-2xl font-serif text-graphite dark:text-white">У вас пока нет товаров</h2>
              <p className="mb-8 max-w-md text-graphite-light dark:text-white/60">
                Добавьте первый товар, чтобы отправить его на модерацию.
              </p>
              <Link to="/products/new" className="button-dark">
                Добавить товар
              </Link>
            </div>
          ) : (
            <div className="mt-6 grid gap-6 xl:grid-cols-[1.35fr_0.65fr]">
              <section className="glass-panel-strong p-5 md:p-6 overflow-hidden">
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm whitespace-nowrap">
                    <thead>
                      <tr className="border-b border-border-lighter text-[11px] uppercase tracking-[0.14em] text-ash dark:border-white/10">
                        <th className="py-3 pr-4 font-semibold w-16">Фото</th>
                        <th className="py-3 pr-4 font-semibold min-w-[200px]">Товар</th>
                        <th className="py-3 pr-4 font-semibold">Варианты</th>
                        <th className="py-3 pr-4 font-semibold">Цена</th>
                        <th className="py-3 pr-4 font-semibold">Статус</th>
                        <th className="py-3 pr-4 font-semibold">Склад ZAMK</th>
                        <th className="py-3 font-semibold text-right">Наличие</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredProducts.map((product) => {
                        const isSelected = product.id === selectedProduct?.id;

                        return (
                          <tr
                            key={product.id}
                            className={cn('border-b border-border-lighter/70 last:border-b-0 dark:border-white/8 cursor-pointer', isSelected && 'bg-ice/50 dark:bg-white/5')}
                            onClick={() => setSelectedId(product.id)}
                            onDoubleClick={() => navigate(`/products/${product.id}/edit`)}
                          >
                            <td className="py-4 pr-4">
                                <ProductAvatar product={product} />
                            </td>
                            <td className="py-4 pr-4">
                               <span className="block font-medium text-graphite dark:text-white max-w-[200px] truncate">{product.title}</span>
                               <span className="mt-1 block text-xs text-graphite-light dark:text-white/58">{product.category}</span>
                            </td>
                            <td className="py-4 pr-4 text-graphite dark:text-white">{product.sizes.length} SKU</td>
                            <td className="py-4 pr-4 text-graphite dark:text-white">{formatCurrency(product.price)}</td>
                            <td className="py-4 pr-4">
                              <div className="flex flex-col gap-1 items-start">
                                  <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
                                  {(product.status === 'published' && (!product.sizes?.length || product.sizes.every(s => s.stock === 0))) && (
                                    <ProductBadge tone="warning">Требуется поставка</ProductBadge>
                                  )}
                                  {(product.status === 'published' && (product.sizes?.reduce((sum, s) => sum + (s.stock || 0), 0) || 0) > 0) && (
                                    <ProductBadge tone="good">В наличии</ProductBadge>
                                  )}
                                </div>
                            </td>
                            <td className="py-4 pr-4 text-graphite-light dark:text-white/68">Нет товара</td>
                            <td className="py-4 text-right">
                                {product.status === 'published' ? (
                                    <span className="text-emerald-600 dark:text-emerald-400 font-medium">Доступен</span>
                                ) : (
                                    <span className="text-ash dark:text-white/40">Недоступен</span>
                                )}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </section>

              {selectedProduct && <ProductDetailPanel product={selectedProduct} sellerStatus={sellerStatus} />}
            </div>
          )}
      </div>
    </div>
  );
}

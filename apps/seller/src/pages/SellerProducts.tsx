import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import {
  AlertTriangle,
  BarChart3,
  Eye,
  Edit2,
  PackagePlus,
  Search,
  ShoppingBag,
  Sparkles,
} from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import {
  SellerSurface,
  SellerKpiCard,
  SellerTableShell,
  SellerDrawer,
} from '../components/SellerSurface';
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
    neutral: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10',
    good: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-400 dark:border-emerald-900/40',
    warning: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40',
    danger: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40',
    info: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40',
  };

  return <span className={cn('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium', styles[tone])}>{children}</span>;
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
  const totalStock = product.sizes.reduce((sum, item) => sum + (item.stock || 0), 0);
  const isApprovedAndNoStock = product.status === 'approved' && totalStock === 0;

  return (
    <div className="flex flex-col justify-between h-full space-y-6">
      <div>
        <div className="flex items-start gap-4">
          <ProductAvatar product={product} />
          <div className="min-w-0 flex-1">
            <p className="text-xs font-mono uppercase tracking-wider text-gray-500 dark:text-gray-400">{product.sku}</p>
            <h2 className="mt-1 text-xl font-semibold tracking-tight text-gray-900 dark:text-white line-clamp-2">{product.title}</h2>
            <div className="mt-2.5 flex flex-wrap gap-2">
              <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
              {isApprovedAndNoStock && (
                <ProductBadge tone="warning">Требуется поставка</ProductBadge>
              )}
              {totalStock > 0 && (
                <ProductBadge tone="good">В наличии ({totalStock} шт.)</ProductBadge>
              )}
            </div>
            {product.status === 'rejected' && product.rejectionReason && (
              <div className="mt-4 rounded-lg border border-red-200 bg-red-50 p-3.5 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200">
                <span className="mb-1 block font-medium">Причина отклонения:</span>
                {product.rejectionReason}
              </div>
            )}
          </div>
        </div>

        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Цена</p>
            <p className="mt-1.5 text-lg font-bold text-gray-900 dark:text-white">{formatCurrency(product.price)}</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Склад ZAMK</p>
            <p className="mt-1.5 text-sm font-medium text-gray-900 dark:text-white">
              {totalStock > 0 ? `${totalStock} шт. на складе` : 'Ожидается поставка'}
            </p>
          </div>
        </div>

        <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
          <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Варианты (SKU)</p>
          <div className="mt-2.5 flex flex-col gap-1.5">
            {product.sizes.map((item) => (
              <div key={item.size} className="flex justify-between items-center rounded-md border border-gray-200/70 bg-white dark:bg-white/[0.03] px-3 py-2 text-sm text-gray-800 dark:border-white/10 dark:text-white/80">
                <span>{item.size}</span>
                <span className="text-gray-500 font-medium">ZAMK: {item.stock} шт.</span>
              </div>
            ))}
          </div>
        </div>

        {isApprovedAndNoStock && (
          <div className="mt-4 rounded-lg border border-blue-200 bg-blue-50 p-3.5 text-sm text-blue-800 dark:border-blue-900/30 dark:bg-blue-950/20 dark:text-blue-200">
            Товар одобрен. Для старта продаж необходимо оформить поставку на склад ZAMK.
          </div>
        )}

        <p className="mt-4 text-sm leading-relaxed text-gray-600 dark:text-gray-400 line-clamp-3">{product.description}</p>
      </div>
      
      <div className="pt-4 border-t border-gray-100 dark:border-white/10">
        {sellerStatus === 'blocked' || sellerStatus === 'archived' ? (
          <div className="rounded-lg border border-red-200 bg-red-50 p-3.5 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200">
            Действия недоступны из-за статуса магазина.
          </div>
        ) : (
          <Link 
            to={`/products/${product.id}/edit`}
            className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-900 shadow-sm transition-colors hover:bg-gray-50 hover:border-gray-300 dark:border-white/10 dark:bg-white/5 dark:text-white dark:hover:bg-white/10"
          >
            {['pending_moderation', 'in_review', 'approved', 'published', 'hidden', 'blocked'].includes(product.status) ? (
              <>
                <Eye className="h-4 w-4 text-gray-500" />
                Просмотр товара
              </>
            ) : product.status === 'rejected' ? (
              <>
                <AlertTriangle className="h-4 w-4 text-red-500" />
                Исправить карточку
              </>
            ) : (
              <>
                <Edit2 className="h-4 w-4 text-gray-500" />
                Продолжить заполнение
              </>
            )}
          </Link>
        )}
      </div>
    </div>
  );
}

export function SellerProducts() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<SellerProductStatus | 'all'>('all');
  const [selectedId, setSelectedId] = useState<string | null>(null);
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
        return null;
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

  const selectedProduct = selectedId ? (products.find((product) => product.id === selectedId) || null) : null;
  const moderationCount = products.filter((product) => product.status === 'pending_moderation' || product.status === 'in_review').length;
  const approvedCount = products.filter((product) => product.status === 'approved' || product.status === 'published').length;
  const revenue = products.reduce((sum, product) => sum + product.revenue, 0);

  if (isLoading) {
    return (
      <SellerPageFrame variant="wide">
        <div className="flex justify-center py-20">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div>
        </div>
      </SellerPageFrame>
    );
  }

  if (error) {
    return (
      <SellerPageFrame variant="wide">
        <div className="flex justify-center py-20 text-red-500">
          {error}
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="wide">
      <SellerPageHeader
        eyebrow="Ассортимент"
        title="Мои товары"
        description="Управляйте карточками товаров. После модерации необходимо оформить поставку на склад ZAMK для старта продаж."
        action={
          <Link
            to="/products/new"
            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-black px-4 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            <PackagePlus className="h-4 w-4" />
            Добавить товар
          </Link>
        }
      />

        <section className="mt-6 grid grid-cols-2 lg:grid-cols-4 gap-4">
          <SellerKpiCard
            label="Всего карточек"
            value={formatNumber(products.length)}
            supportText="создано в системе"
            icon={ShoppingBag}
          />
          <SellerKpiCard
            label="Одобрено"
            value={formatNumber(approvedCount)}
            supportText="готово к поставке"
            accent={approvedCount > 0 ? "positive" : undefined}
            icon={Sparkles}
          />
          <SellerKpiCard
            label="На проверке"
            value={formatNumber(moderationCount)}
            supportText="ожидают решения"
            accent={moderationCount > 0 ? "warning" : undefined}
            icon={AlertTriangle}
          />
          <SellerKpiCard
            label="Текущая выручка"
            value={formatCurrency(revenue)}
            supportText="данные по продажам"
            icon={BarChart3}
          />
        </section>

        <SellerSurface className="mt-6 p-4">
          <div className="grid gap-3 sm:grid-cols-[1fr_220px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Поиск по названию или категории..."
                className="h-10 w-full rounded-lg border border-gray-200 bg-white pl-10 pr-3 text-sm text-gray-900 outline-none placeholder:text-gray-400 focus:border-gray-900 focus:ring-1 focus:ring-gray-900 dark:border-white/10 dark:bg-white/5 dark:text-white"
              />
            </label>
            <select
              value={status}
              onChange={(event) => setStatus(event.target.value as SellerProductStatus | 'all')}
              className="h-10 rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 outline-none focus:border-gray-900 focus:ring-1 focus:ring-gray-900 dark:border-white/10 dark:bg-white/5 dark:text-white"
            >
              {statusFilterOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </SellerSurface>

          {products.length === 0 ? (
            <SellerSurface className="mt-6 flex flex-col items-center justify-center p-12 text-center">
              <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-gray-50 border border-gray-200 text-gray-400 dark:bg-white/5 dark:border-white/10 dark:text-white/60">
                <PackagePlus className="h-6 w-6" />
              </div>
              <h2 className="mb-2 text-xl font-semibold text-gray-900 dark:text-white">У вас пока нет товаров</h2>
              <p className="mb-6 max-w-md text-sm text-gray-500 dark:text-gray-400">
                Добавьте первый товар, чтобы отправить его на модерацию.
              </p>
              <Link
                to="/products/new"
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-800"
              >
                <PackagePlus className="h-4 w-4" />
                Добавить товар
              </Link>
            </SellerSurface>
          ) : (
            <>
              <SellerTableShell className="mt-6">
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm whitespace-nowrap">
                    <thead>
                      <tr className="bg-gray-50/60 dark:bg-white/[0.02] border-b border-gray-200 dark:border-white/10 text-xs font-semibold uppercase tracking-wider text-gray-500">
                        <th className="px-4 py-3.5 w-16">Фото</th>
                        <th className="px-4 py-3.5 min-w-[200px]">Товар</th>
                        <th className="px-4 py-3.5">Варианты</th>
                        <th className="px-4 py-3.5">Цена</th>
                        <th className="px-4 py-3.5">Статус</th>
                        <th className="px-4 py-3.5">Склад ZAMK</th>
                        <th className="px-4 py-3.5 text-right">Наличие</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                      {filteredProducts.map((product) => {
                        const isSelected = product.id === selectedId;

                        return (
                          <tr
                            key={product.id}
                            className={cn('hover:bg-gray-50/60 dark:hover:bg-white/[0.02] cursor-pointer transition-colors', isSelected && 'bg-gray-50 dark:bg-white/5')}
                            onClick={() => setSelectedId(product.id)}
                            onDoubleClick={() => navigate(`/products/${product.id}/edit`)}
                          >
                            <td className="px-4 py-3.5">
                              <ProductAvatar product={product} />
                            </td>
                            <td className="px-4 py-3.5">
                              <span className="block font-medium text-gray-900 dark:text-white max-w-[280px] truncate">{product.title}</span>
                              <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">{product.category}</span>
                            </td>
                            <td className="px-4 py-3.5 text-gray-700 dark:text-gray-300">{product.sizes.length} SKU</td>
                            <td className="px-4 py-3.5 font-medium text-gray-900 dark:text-white">{formatCurrency(product.price)}</td>
                            <td className="px-4 py-3.5">
                              <div className="flex flex-col gap-1 items-start">
                                <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
                                {(product.status === 'published' || product.status === 'approved') && (!product.sizes?.length || product.sizes.every(s => (s.stock || 0) === 0)) && (
                                  <ProductBadge tone="warning">Требуется поставка</ProductBadge>
                                )}
                                {(product.sizes?.reduce((sum, s) => sum + (s.stock || 0), 0) || 0) > 0 && (
                                  <ProductBadge tone="good">В наличии ({product.sizes.reduce((sum, s) => sum + (s.stock || 0), 0)} шт.)</ProductBadge>
                                )}
                              </div>
                            </td>
                            <td className="px-4 py-3.5 text-gray-500 dark:text-gray-400">
                              {(() => {
                                const total = product.sizes?.reduce((sum, s) => sum + (s.stock || 0), 0) || 0;
                                return total > 0 ? (
                                  <span className="font-semibold text-emerald-600 dark:text-emerald-400">{total} шт.</span>
                                ) : (
                                  <span>Нет на складе</span>
                                );
                              })()}
                            </td>
                            <td className="px-4 py-3.5 text-right">
                              {product.status === 'published' ? (
                                <span className="text-emerald-600 dark:text-emerald-400 font-medium">Доступен</span>
                              ) : (
                                <span className="text-gray-400 dark:text-gray-500">Недоступен</span>
                              )}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </SellerTableShell>

              <SellerDrawer
                isOpen={Boolean(selectedProduct)}
                onClose={() => setSelectedId(null)}
                title="Карточка товара"
                data-testid="seller-product-drawer"
              >
                {selectedProduct && (
                  <ProductDetailPanel product={selectedProduct} sellerStatus={sellerStatus} />
                )}
              </SellerDrawer>
            </>
          )}
    </SellerPageFrame>
  );
}

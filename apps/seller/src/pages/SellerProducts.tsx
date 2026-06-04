import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertTriangle,
  ArrowLeft,
  BarChart3,
  Copy,
  Eye,
  PackagePlus,
  PauseCircle,
  Search,
  ShoppingBag,
  Sparkles,
  type LucideIcon,
} from 'lucide-react';
import {
  issueLabels,
  loadSellerProducts,
  saveSellerProducts,
  statusLabels,
  type SellerProduct,
  type SellerProductIssue,
  type SellerProductStatus,
} from '../lib/seller-products';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const numberFormatter = new Intl.NumberFormat('ru-RU');
const formatCurrency = (value: number) => currencyFormatter.format(value);
const formatNumber = (value: number) => numberFormatter.format(value);
const formatPercent = (value: number) => `${value.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`;

const statusFilterOptions: Array<{ value: SellerProductStatus | 'all'; label: string }> = [
  { value: 'all', label: 'Все статусы' },
  { value: 'published', label: statusLabels.published },
  { value: 'draft', label: statusLabels.draft },
  { value: 'moderation', label: statusLabels.moderation },
  { value: 'needs_changes', label: statusLabels.needs_changes },
  { value: 'low_stock', label: statusLabels.low_stock },
  { value: 'paused', label: statusLabels.paused },
];

const issueFilterOptions: Array<{ value: SellerProductIssue | 'all'; label: string }> = [
  { value: 'all', label: 'Все сигналы' },
  { value: 'low_stock', label: issueLabels.low_stock },
  { value: 'weak_card', label: issueLabels.weak_card },
  { value: 'ads_waste', label: issueLabels.ads_waste },
  { value: 'no_issue', label: issueLabels.no_issue },
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
    published: 'good',
    draft: 'neutral',
    moderation: 'info',
    needs_changes: 'warning',
    low_stock: 'warning',
    paused: 'danger',
  };

  return tones[status];
}

function getIssueTone(issue: SellerProductIssue) {
  const tones: Record<SellerProductIssue, 'neutral' | 'good' | 'warning' | 'danger' | 'info'> = {
    no_issue: 'good',
    low_stock: 'warning',
    weak_card: 'warning',
    ads_waste: 'danger',
  };

  return tones[issue];
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

function QualityMeter({ value }: { value: number }) {
  const tone = value >= 80 ? 'bg-emerald-500 dark:bg-emerald-300' : value >= 60 ? 'bg-amber-500 dark:bg-amber-300' : 'bg-red-500 dark:bg-red-300';

  return (
    <div>
      <div className="flex items-center justify-between text-xs">
        <span className="font-semibold uppercase tracking-[0.12em] text-ash dark:text-white/62">Качество</span>
        <span className="font-semibold text-graphite dark:text-white">{value}%</span>
      </div>
      <div className="mt-2 h-2 overflow-hidden rounded-full bg-ice dark:bg-white/10">
        <div className={cn('h-full rounded-full', tone)} style={{ width: `${value}%` }} />
      </div>
    </div>
  );
}

function ProductAvatar({ product }: { product: SellerProduct }) {
  return (
    <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-graphite to-accent text-sm font-bold text-white shadow-sm dark:from-white dark:to-accent dark:text-black">
      {product.mainPhoto}
    </div>
  );
}

function ProductDetailPanel({ product }: { product: SellerProduct }) {
  const totalStock = product.sizes.reduce((sum, item) => sum + item.stock, 0);
  const margin = product.price - product.cost;
  const adShare = product.revenue > 0 ? (product.adsSpend / product.revenue) * 100 : 0;

  return (
    <aside className="glass-panel-strong p-6 md:p-8">
      <div className="flex items-start gap-4">
        <ProductAvatar product={product} />
        <div>
          <p className="studio-label">{product.sku}</p>
          <h2 className="mt-2 text-3xl font-serif leading-tight text-graphite dark:text-white">{product.title}</h2>
          <div className="mt-3 flex flex-wrap gap-2">
            <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
            <ProductBadge tone={getIssueTone(product.issue)}>{issueLabels[product.issue]}</ProductBadge>
          </div>
        </div>
      </div>

      <div className="mt-7">
        <QualityMeter value={product.quality} />
      </div>

      <div className="mt-7 grid gap-3 sm:grid-cols-2">
        {[
          ['Цена', formatCurrency(product.price)],
          ['Маржа на штуку', formatCurrency(margin)],
          ['Остаток', `${formatNumber(totalStock)} ед.`],
          ['Доля рекламы', formatPercent(adShare)],
          ['Просмотры', formatNumber(product.views)],
          ['Заказы', formatNumber(product.orders)],
        ].map(([label, value]) => (
          <div key={label} className="rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">{label}</p>
            <p className="mt-2 text-lg font-semibold text-graphite dark:text-white">{value}</p>
          </div>
        ))}
      </div>

      <div className="mt-7 rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Размеры и остатки</p>
        <div className="mt-3 flex flex-wrap gap-2">
          {product.sizes.map((item) => (
            <span key={item.size} className="rounded-full border border-border-lighter px-3 py-1 text-xs text-graphite dark:border-white/16 dark:text-white/78">
              {item.size}: {item.stock}
            </span>
          ))}
        </div>
      </div>

      <p className="mt-5 text-sm leading-relaxed text-graphite-light dark:text-white/68">{product.description}</p>
    </aside>
  );
}

export function SellerProducts() {
  const [products, setProducts] = useState<SellerProduct[]>(() => loadSellerProducts());
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<SellerProductStatus | 'all'>('all');
  const [issue, setIssue] = useState<SellerProductIssue | 'all'>('all');
  const [selectedId, setSelectedId] = useState(products[0]?.id || '');

  useEffect(() => {
    saveSellerProducts(products);
  }, [products]);

  const filteredProducts = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();

    return products.filter((product) => {
      const matchesQuery = !normalizedQuery || [product.title, product.sku, product.category].some((item) => item.toLowerCase().includes(normalizedQuery));
      const matchesStatus = status === 'all' || product.status === status;
      const matchesIssue = issue === 'all' || product.issue === issue;
      return matchesQuery && matchesStatus && matchesIssue;
    });
  }, [issue, products, query, status]);

  const selectedProduct = products.find((product) => product.id === selectedId) || filteredProducts[0] || products[0];
  const totalStock = products.reduce((sum, product) => sum + product.sizes.reduce((sizeSum, item) => sizeSum + item.stock, 0), 0);
  const problemCount = products.filter((product) => product.issue !== 'no_issue').length;
  const moderationCount = products.filter((product) => product.status === 'moderation').length;
  const revenue = products.reduce((sum, product) => sum + product.revenue, 0);

  const updateProductStatus = (id: string, nextStatus: SellerProductStatus) => {
    setProducts((current) => current.map((product) => (product.id === id ? { ...product, status: nextStatus, updatedAt: 'Только что' } : product)));
  };

  const duplicateProduct = (product: SellerProduct) => {
    const copy: SellerProduct = {
      ...product,
      id: `seller-product-${Date.now()}`,
      title: `${product.title} копия`,
      sku: `${product.sku}-COPY`,
      status: 'draft',
      issue: 'no_issue',
      views: 0,
      orders: 0,
      revenue: 0,
      adsSpend: 0,
      updatedAt: 'Только что',
    };
    setProducts((current) => [copy, ...current]);
    setSelectedId(copy.id);
  };

  return (
    <div className="relative z-10 min-h-screen pt-24 pb-24 md:pt-28 md:pb-20">
      <div className="container mx-auto max-w-[1400px] px-4 sm:px-6">
        <Link to="/seller-dashboard" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white">
          <ArrowLeft className="h-4 w-4" />
          Кабинет продавца
        </Link>

        <section className="mt-6 glass-panel-strong p-7 md:p-10">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="studio-label">Ассортимент</p>
              <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-6xl">Мои товары</h1>
              <p className="studio-subtitle mt-4 max-w-3xl">
                Управляйте опубликованными товарами, черновиками, остатками и сигналами качества карточек в одном месте.
              </p>
            </div>
            <Link
              to="/seller-products/new"
              className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-graphite px-6 text-sm font-semibold text-white transition-colors hover:bg-graphite-light dark:bg-white dark:text-black dark:hover:bg-white/86"
            >
              <PackagePlus className="h-4 w-4" />
              Добавить товар
            </Link>
          </div>
        </section>

        <section className="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <SummaryCard label="Всего товаров" value={formatNumber(products.length)} hint={`${formatNumber(totalStock)} единиц на остатках`} icon={ShoppingBag} />
          <SummaryCard label="Выручка ассортимента" value={formatCurrency(revenue)} hint="по текущим моковым данным" icon={BarChart3} />
          <SummaryCard label="Проблемные карточки" value={formatNumber(problemCount)} hint="требуют действия сегодня" icon={AlertTriangle} />
          <SummaryCard label="На модерации" value={formatNumber(moderationCount)} hint="ожидают проверки площадки" icon={Sparkles} />
        </section>

        <section className="mt-6 glass-panel-strong p-5 md:p-6">
          <div className="grid gap-3 lg:grid-cols-[1fr_220px_220px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-ash" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Поиск по названию, артикулу или категории"
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
            <select
              value={issue}
              onChange={(event) => setIssue(event.target.value as SellerProductIssue | 'all')}
              className="seller-setting-input h-12 rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none dark:border-white/16 dark:bg-black/24 dark:text-white"
            >
              {issueFilterOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </section>

        <div className="mt-6 grid gap-6 xl:grid-cols-[1.35fr_0.65fr]">
          <section className="glass-panel-strong p-5 md:p-6">
            <div className="overflow-x-auto">
              <table className="min-w-[1120px] w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-border-lighter text-[11px] uppercase tracking-[0.14em] text-ash dark:border-white/10">
                    <th className="py-3 pr-4 font-semibold">Товар</th>
                    <th className="py-3 pr-4 font-semibold">Статус</th>
                    <th className="py-3 pr-4 font-semibold">Цена</th>
                    <th className="py-3 pr-4 font-semibold">Остаток</th>
                    <th className="py-3 pr-4 font-semibold">Просмотры</th>
                    <th className="py-3 pr-4 font-semibold">Заказы</th>
                    <th className="py-3 pr-4 font-semibold">Качество</th>
                    <th className="py-3 pr-4 font-semibold">Сигнал</th>
                    <th className="py-3 font-semibold">Действия</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredProducts.map((product) => {
                    const stock = product.sizes.reduce((sum, item) => sum + item.stock, 0);
                    const isSelected = product.id === selectedProduct?.id;

                    return (
                      <tr
                        key={product.id}
                        className={cn('border-b border-border-lighter/70 last:border-b-0 dark:border-white/8', isSelected && 'bg-ice/50 dark:bg-white/5')}
                      >
                        <td className="py-4 pr-4">
                          <button type="button" onClick={() => setSelectedId(product.id)} className="flex items-center gap-3 text-left">
                            <ProductAvatar product={product} />
                            <span>
                              <span className="block font-medium text-graphite dark:text-white">{product.title}</span>
                              <span className="mt-1 block text-xs text-graphite-light dark:text-white/58">{product.sku} · {product.category}</span>
                            </span>
                          </button>
                        </td>
                        <td className="py-4 pr-4"><ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge></td>
                        <td className="py-4 pr-4 text-graphite dark:text-white">{formatCurrency(product.price)}</td>
                        <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(stock)}</td>
                        <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(product.views)}</td>
                        <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(product.orders)}</td>
                        <td className="py-4 pr-4 min-w-[130px]"><QualityMeter value={product.quality} /></td>
                        <td className="py-4 pr-4"><ProductBadge tone={getIssueTone(product.issue)}>{issueLabels[product.issue]}</ProductBadge></td>
                        <td className="py-4">
                          <div className="flex gap-2">
                            <button
                              type="button"
                              onClick={() => updateProductStatus(product.id, product.status === 'paused' ? 'published' : 'paused')}
                              className="rounded-full border border-border-lighter p-2 text-graphite-light transition-colors hover:text-graphite dark:border-white/16 dark:text-white/62 dark:hover:text-white"
                              aria-label="Переключить продажу"
                            >
                              <PauseCircle className="h-4 w-4" />
                            </button>
                            <button
                              type="button"
                              onClick={() => duplicateProduct(product)}
                              className="rounded-full border border-border-lighter p-2 text-graphite-light transition-colors hover:text-graphite dark:border-white/16 dark:text-white/62 dark:hover:text-white"
                              aria-label="Дублировать товар"
                            >
                              <Copy className="h-4 w-4" />
                            </button>
                            <button
                              type="button"
                              onClick={() => setSelectedId(product.id)}
                              className="rounded-full border border-border-lighter p-2 text-graphite-light transition-colors hover:text-graphite dark:border-white/16 dark:text-white/62 dark:hover:text-white"
                              aria-label="Открыть детали"
                            >
                              <Eye className="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>

          {selectedProduct && <ProductDetailPanel product={selectedProduct} />}
        </div>
      </div>
    </div>
  );
}

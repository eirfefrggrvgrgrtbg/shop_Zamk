import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Archive, BarChart2, Package, RotateCcw, ShoppingCart, Wallet, AlertTriangle } from 'lucide-react';
import {
  getSellerBalance,
  getSellerInventory,
  getSellerOrders,
  getSellerProducts,
  getSellerReturns,
  getSellerMe,
  getSellerWarnings,
  getSellerViolations,
} from '@zamk/api-client/src/seller';
import type { InventoryItem, SellerBalance, SellerOrder, SellerProduct, SellerReturn, SellerMe, SellerWarning, SellerViolation } from '@zamk/api-client/src/types';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const formatMoney = (amountCents?: number) => currencyFormatter.format((amountCents ?? 0) / 100);

type DashboardState = {
  products: SellerProduct[];
  orders: SellerOrder[];
  returns: SellerReturn[];
  inventory: InventoryItem[];
  balance: SellerBalance | null;
  sellerMe: SellerMe | null;
  warnings: SellerWarning[];
  violations: SellerViolation[];
};

const initialState: DashboardState = {
  products: [],
  orders: [],
  returns: [],
  inventory: [],
  balance: null,
  sellerMe: null,
  warnings: [],
  violations: [],
};

const unwrapItems = <T,>(response: T[] | { items?: T[] } | null): T[] => {
  if (!response) return [];
  return Array.isArray(response) ? response : response.items ?? [];
};

function MetricCard({
  title,
  value,
  description,
  icon: Icon,
}: {
  title: string;
  value: string;
  description: string;
  icon: typeof Package;
}) {
  return (
    <article className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-gray-500">{title}</p>
          <p className="mt-2 text-3xl font-semibold text-gray-900">{value}</p>
        </div>
        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-gray-100 text-gray-600">
          <Icon className="h-5 w-5" />
        </span>
      </div>
      <p className="mt-3 text-sm text-gray-500">{description}</p>
    </article>
  );
}

function EmptyMetric({ title }: { title: string }) {
  return (
    <article className="rounded-2xl border border-dashed border-gray-300 bg-white p-5">
      <p className="text-sm font-medium text-gray-500">{title}</p>
      <p className="mt-2 text-lg font-semibold text-gray-900">Метрика пока не подключена</p>
      <p className="mt-2 text-sm text-gray-500">Данные появятся после подключения аналитики backend.</p>
    </article>
  );
}

export function SellerDashboard() {
  const [data, setData] = useState<DashboardState>(initialState);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    async function loadDashboard() {
      setIsLoading(true);
      setError('');

      try {
        const [products, orders, returns, inventory, balance, sellerMe, warnings, violations] = await Promise.all([
          getSellerProducts(),
          getSellerOrders(),
          getSellerReturns(),
          getSellerInventory(),
          getSellerBalance().catch(() => null),
          getSellerMe().catch(() => null),
          getSellerWarnings().catch(() => []),
          getSellerViolations().catch(() => []),
        ]);

        if (!cancelled) {
          setData({
            products: unwrapItems(products),
            orders: unwrapItems(orders),
            returns: unwrapItems(returns),
            inventory: unwrapItems(inventory),
            balance,
            sellerMe,
            warnings,
            violations,
          });
        }
      } catch (err: any) {
        if (!cancelled) {
          if (err?.status === 401 || err?.code === 'unauthorized') {
            setError('Сессия истекла. Обновите страницу и войдите снова.');
          } else {
            setError('Не удалось загрузить данные. Проверьте, запущен ли backend и выполнен ли вход.');
          }
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadDashboard();

    return () => {
      cancelled = true;
    };
  }, []);

  const activeOrders = data.orders.filter((order) => !['delivered', 'cancelled'].includes(order.status)).length;
  const totalStock = data.inventory.reduce((sum, item) => sum + (item.quantityAvailable ?? 0), 0);

  const checklist = [
    { id: 'brandName', label: 'Название магазина', done: !!data.sellerMe?.seller.brandName },
    { id: 'slug', label: 'Адрес магазина / slug', done: !!data.sellerMe?.seller.slug },
    { id: 'description', label: 'Описание магазина', done: !!data.sellerMe?.seller.description },
    { id: 'contact', label: 'Контактная почта или телефон', done: !!(data.sellerMe?.seller.contactEmail || data.sellerMe?.seller.contactPhone) },
    { id: 'logo', label: 'Логотип магазина', done: !!data.sellerMe?.seller.logoUrl },
    { id: 'first_product', label: 'Первый товар', done: data.products.length > 0 },
  ];
  
  const completedCount = checklist.filter(c => c.done).length;
  const progressPercent = Math.round((completedCount / checklist.length) * 100);
  const isProfileComplete = completedCount === checklist.length;
  const activeWarnings = data.warnings.filter(w => w.status === 'active').length;
  const activeViolations = data.violations.filter(v => v.status === 'active').length;

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="mx-auto max-w-7xl space-y-8">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-gray-500">Панель продавца</p>
          <h1 className="mt-2 text-3xl font-bold text-gray-900">Обзор магазина</h1>
          <p className="mt-2 text-gray-600">
            Здесь отображаются только данные из API. Неподключённые метрики не заполняются фейковыми значениями.
          </p>
        </div>

        {error && (
          <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {isLoading ? (
          <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center text-gray-500">
            Загрузка данных...
          </div>
        ) : (
          <>
            {!isProfileComplete && data.sellerMe && (
              <section className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
                <div className="flex items-center justify-between">
                  <h2 className="text-lg font-semibold text-gray-900">Заполненность профиля</h2>
                  <span className="text-sm font-medium text-gray-600">Профиль заполнен на {progressPercent}%</span>
                </div>
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  {checklist.map(item => (
                    <div key={item.id} className="flex items-center gap-3">
                      <div className={`flex h-6 w-6 items-center justify-center rounded-full border ${item.done ? 'border-green-500 bg-green-500 text-white' : 'border-gray-300'}`}>
                        {item.done && (
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                        )}
                      </div>
                      <span className={item.done ? 'text-gray-900' : 'text-gray-500'}>{item.label}</span>
                    </div>
                  ))}
                </div>
                <div className="mt-6">
                  <Link to="/settings" className="inline-flex rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800">
                    Заполнить профиль магазина
                  </Link>
                </div>
              </section>
            )}

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
              <MetricCard title="Товары" value={String(data.products.length)} description="Реальное количество товаров продавца" icon={Package} />
              <MetricCard title="Активные заказы" value={String(activeOrders)} description="Заказы кроме доставленных и отменённых" icon={ShoppingCart} />
              <MetricCard title="Возвраты" value={String(data.returns.length)} description="Реальные возвраты продавца" icon={RotateCcw} />
              <MetricCard title="Остатки" value={String(totalStock)} description="Сумма доступных остатков" icon={Archive} />
              <MetricCard title="Доступно к выплате" value={formatMoney(data.balance?.availableBalanceCents)} description="Баланс из API выплат" icon={Wallet} />
              <MetricCard title="Предупреждения / нарушения" value={`${activeWarnings} / ${activeViolations}`} description="Активные предупреждения и нарушения" icon={AlertTriangle} />
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <EmptyMetric title="Выручка" />
              <EmptyMetric title="Просмотры" />
              <EmptyMetric title="Конверсия" />
              <EmptyMetric title="Расходы на продвижение" />
            </section>

            <section className="rounded-2xl border border-gray-200 bg-white p-6">
              <div className="flex items-start gap-3">
                <BarChart2 className="mt-1 h-5 w-5 text-gray-400" />
                <div>
                  <h2 className="text-lg font-semibold text-gray-900">Аналитика</h2>
                  <p className="mt-2 text-sm text-gray-600">
                    Расширенная аналитика будет доступна позже. До подключения backend-агрегаций страница не показывает
                    фейковую выручку, просмотры, конверсию или рекламные расходы.
                  </p>
                </div>
              </div>
            </section>

            <section className="rounded-2xl border border-gray-200 bg-white p-6">
              <h2 className="text-lg font-semibold text-gray-900">Быстрые действия</h2>
              <div className="mt-4 flex flex-wrap gap-3">
                <Link to="/products" className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800">
                  Открыть товары
                </Link>
                <Link to="/orders" className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Открыть заказы
                </Link>
                <Link to="/payouts" className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Открыть выплаты
                </Link>
              </div>
            </section>
          </>
        )}
      </div>
    </div>
  );
}

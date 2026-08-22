import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Archive, Package, RotateCcw, ShoppingCart, AlertTriangle, CheckCircle2, ChevronRight, PlusCircle } from 'lucide-react';
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
import type { SellerInventoryItem, SellerBalance, SellerOrder, SellerProduct, SellerReturn, SellerMe, SellerWarning, SellerViolation } from '@zamk/api-client/src/types';

type DashboardState = {
  products: SellerProduct[];
  orders: SellerOrder[];
  returns: SellerReturn[];
  inventory: SellerInventoryItem[];
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

function pluralize(count: number, words: [string, string, string]) {
  const cases = [2, 0, 1, 1, 1, 2];
  return count + ' ' + words[(count % 100 > 4 && count % 100 < 20) ? 2 : cases[(count % 10 < 5) ? count % 10 : 5]];
}

function MetricCard({
  title,
  value,
  description,
  icon: Icon,
  to,
  alert
}: {
  title: string;
  value: string;
  description: string;
  icon: typeof Package;
  to: string;
  alert?: boolean;
}) {
  return (
    <Link 
      to={to}
      className={`group rounded-2xl border bg-white p-5 shadow-sm transition-all hover:shadow-md ${alert ? 'border-red-200 hover:border-red-300' : 'border-gray-200 hover:border-gray-300'}`}
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-gray-500">{title}</p>
          <p className={`mt-2 text-3xl font-semibold ${alert ? 'text-red-600' : 'text-gray-900'}`}>{value}</p>
        </div>
        <span className={`flex h-11 w-11 items-center justify-center rounded-xl transition-colors ${alert ? 'bg-red-50 text-red-600 group-hover:bg-red-100' : 'bg-gray-50 text-gray-600 group-hover:bg-gray-100'}`}>
          <Icon className="h-5 w-5" />
        </span>
      </div>
      <p className="mt-3 text-sm text-gray-500 flex items-center justify-between">
        <span>{description}</span>
        <ChevronRight className="h-4 w-4 text-gray-400 opacity-0 transition-all group-hover:opacity-100 group-hover:translate-x-1" />
      </p>
    </Link>
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
            setError('Не удалось загрузить данные.');
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

  const activeOrders = data.orders.filter((order) => !['delivered', 'cancelled', 'returned', 'refunded', 'fully_returned'].includes(order.commercialStatus)).length;
  const totalStock = data.inventory.reduce((sum, item) => sum + (item.available ?? 0), 0);

  const checklist = [
    { id: 'brandName', label: 'Название магазина', done: !!data.sellerMe?.seller.brandName },
    { id: 'slug', label: 'Адрес магазина / slug', done: !!data.sellerMe?.seller.slug },
    { id: 'description', label: 'Описание магазина', done: !!data.sellerMe?.seller.description },
    { id: 'contact', label: 'Контактные данные', done: !!(data.sellerMe?.seller.contactEmail || data.sellerMe?.seller.contactPhone) },
    { id: 'logo', label: 'Логотип', done: !!data.sellerMe?.seller.logoUrl },
    { id: 'first_product', label: 'Первый товар', done: data.products.length > 0 },
  ];
  
  const completedCount = checklist.filter(c => c.done).length;
  const progressPercent = Math.round((completedCount / checklist.length) * 100);
  const isProfileComplete = completedCount === checklist.length;
  const activeWarnings = data.warnings.filter(w => w.status === 'active').length;
  const activeViolations = data.violations.filter(v => v.status === 'active').length;
  const totalIssues = activeWarnings + activeViolations;
  
  const isNewSeller = data.products.length === 0;

  const attentionItems = [];
  if (!isProfileComplete) attentionItems.push({ label: 'Заполните профиль магазина для активации всех функций', to: '/settings' });
  if (isNewSeller) attentionItems.push({ label: 'Добавьте свой первый товар в каталог', to: '/products/new' });
  if (totalIssues > 0) attentionItems.push({ label: `У вас ${pluralize(totalIssues, ['новое предупреждение', 'новых предупреждения', 'новых предупреждений'])}`, to: '/warnings', alert: true });
  if (data.returns.length > 0) attentionItems.push({ label: `Обработайте ${pluralize(data.returns.length, ['новый возврат', 'новых возврата', 'новых возвратов'])}`, to: '/returns' });

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="mx-auto max-w-7xl space-y-8">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-gray-500">Панель продавца</p>
          <h1 className="mt-2 text-3xl font-bold text-gray-900">Обзор магазина</h1>
          <p className="mt-2 text-gray-600">
            Ключевые показатели и задачи, требующие вашего внимания.
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
            {/* Attention Block */}
            <section className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
              <h2 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
                <AlertTriangle className={`w-5 h-5 ${attentionItems.some(i => i.alert) ? 'text-orange-500' : 'text-gray-400'}`} />
                Требует внимания
              </h2>
              {attentionItems.length > 0 ? (
                <div className="space-y-3">
                  {attentionItems.map((item, idx) => (
                    <Link key={idx} to={item.to} className={`flex items-center justify-between p-3 rounded-xl border transition-colors ${item.alert ? 'bg-orange-50/50 border-orange-100 hover:bg-orange-50' : 'bg-gray-50/50 border-gray-100 hover:bg-gray-50'}`}>
                      <span className={`text-sm font-medium ${item.alert ? 'text-orange-900' : 'text-gray-900'}`}>{item.label}</span>
                      <ChevronRight className={`w-4 h-4 ${item.alert ? 'text-orange-400' : 'text-gray-400'}`} />
                    </Link>
                  ))}
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-6 text-center">
                  <div className="w-12 h-12 rounded-full bg-green-50 flex items-center justify-center mb-3">
                    <CheckCircle2 className="w-6 h-6 text-green-500" />
                  </div>
                  <p className="text-gray-900 font-medium">Отлично! Все задачи выполнены.</p>
                  <p className="text-sm text-gray-500 mt-1">Проблем не найдено, магазин работает в штатном режиме.</p>
                </div>
              )}
            </section>

            {/* Compact Onboarding if incomplete */}
            {!isProfileComplete && data.sellerMe && (
              <section className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div>
                    <h2 className="text-lg font-semibold text-gray-900">Заполненность профиля</h2>
                    <div className="mt-2 flex items-center gap-2">
                      <div className="h-2 w-48 bg-gray-100 rounded-full overflow-hidden">
                        <div className="h-full bg-green-500 rounded-full" style={{ width: `${progressPercent}%` }}></div>
                      </div>
                      <span className="text-sm font-medium text-gray-600">{progressPercent}%</span>
                    </div>
                  </div>
                  <Link to="/settings" className="inline-flex rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 shrink-0">
                    Настроить профиль
                  </Link>
                </div>
              </section>
            )}

            {/* Action for New Sellers */}
            {isNewSeller && isProfileComplete && (
              <section className="rounded-2xl border border-blue-200 bg-blue-50 p-8 text-center">
                <div className="w-16 h-16 rounded-2xl bg-blue-100 flex items-center justify-center mx-auto mb-4">
                  <Package className="w-8 h-8 text-blue-600" />
                </div>
                <h2 className="text-xl font-bold text-gray-900 mb-2">Начните продажи на ZAMK</h2>
                <p className="text-gray-600 mb-6 max-w-md mx-auto">Ваш профиль готов к работе. Добавьте свой первый товар, чтобы покупатели могли его найти.</p>
                <Link to="/products/new" className="inline-flex items-center gap-2 rounded-xl bg-blue-600 px-6 py-3 text-sm font-medium text-white hover:bg-blue-700 transition-colors shadow-sm hover:shadow">
                  <PlusCircle className="w-4 h-4" />
                  Создать товар
                </Link>
              </section>
            )}

            {/* Operating Summary */}
            <section className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              <MetricCard 
                title="Товары" 
                value={String(data.products.length)} 
                description={data.products.length === 0 ? 'Нет товаров' : pluralize(data.products.length, ['товар в каталоге', 'товара в каталоге', 'товаров в каталоге'])} 
                icon={Package} 
                to="/products"
              />
              <MetricCard 
                title="Активные заказы" 
                value={String(activeOrders)} 
                description={activeOrders === 0 ? 'Нет активных заказов' : pluralize(activeOrders, ['заказ в работе', 'заказа в работе', 'заказов в работе'])} 
                icon={ShoppingCart} 
                to="/orders"
              />
              <MetricCard 
                title="Возвраты" 
                value={String(data.returns.length)} 
                description={data.returns.length === 0 ? 'Нет активных возвратов' : pluralize(data.returns.length, ['возврат ожидает', 'возврата ожидают', 'возвратов ожидают'])} 
                icon={RotateCcw} 
                to="/returns"
              />
              <MetricCard 
                title="Остатки" 
                value={String(totalStock)} 
                description={totalStock === 0 ? 'Нет товаров на складе' : pluralize(totalStock, ['единица доступна', 'единицы доступно', 'единиц доступно'])} 
                icon={Archive} 
                to="/inventory"
              />
            </section>
            
            {/* Warnings Summary if any */}
            {totalIssues > 0 && (
              <section className="grid gap-4 md:grid-cols-1 lg:grid-cols-2">
                <MetricCard 
                  title="Нарушения и предупреждения" 
                  value={String(totalIssues)} 
                  description={pluralize(totalIssues, ['активное предупреждение', 'активных предупреждения', 'активных предупреждений'])} 
                  icon={AlertTriangle} 
                  to="/warnings"
                  alert={true}
                />
              </section>
            )}

          </>
        )}
      </div>
    </div>
  );
}

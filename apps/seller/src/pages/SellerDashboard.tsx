import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Archive, Package, RotateCcw, ShoppingCart, AlertTriangle, CheckCircle2, ChevronRight, PlusCircle } from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerKpiCard } from '../components/SellerSurface';
import { cn } from '../lib/utils';
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
import { prepareAddProductNavigation } from '../components/product-studio/productStudioCreateSession';

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
  if (!isProfileComplete) {
    const isActive = data.sellerMe?.seller.status === 'active';
    attentionItems.push({ 
      label: isActive ? 'Завершите оформление профиля магазина (логотип, контакты)' : 'Заполните профиль магазина для активации всех функций', 
      to: '/settings' 
    });
  }
  if (isNewSeller) attentionItems.push({ label: 'Добавьте свой первый товар в каталог', to: '/products/new' });
  if (totalIssues > 0) attentionItems.push({ label: `У вас ${pluralize(totalIssues, ['новое предупреждение', 'новых предупреждения', 'новых предупреждений'])}`, to: '/warnings', alert: true });
  if (data.returns.length > 0) attentionItems.push({ label: `Обработайте ${pluralize(data.returns.length, ['новый возврат', 'новых возврата', 'новых возвратов'])}`, to: '/returns' });

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Панель продавца"
        title="Обзор магазина"
        description="Ключевые показатели и задачи, требующие вашего внимания."
      />

        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
        )}

        {isLoading ? (
          <SellerSurface className="p-8 text-center text-gray-500">
            Загрузка данных...
          </SellerSurface>
        ) : (
          <>
            {/* Attention Block */}
            <SellerSurface as="section" className="p-6">
              <h2 className="text-base font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
                <AlertTriangle className={cn("w-5 h-5", attentionItems.some(i => i.alert) ? "text-amber-500" : "text-gray-400")} />
                Требует внимания
              </h2>
              {attentionItems.length > 0 ? (
                <div className="space-y-2.5">
                  {attentionItems.map((item, idx) => (
                    <Link
                      key={idx}
                      to={item.to}
                      className={cn(
                        "flex items-center justify-between p-3 rounded-lg border transition-colors",
                        item.alert
                          ? "bg-amber-50/60 border-amber-200 text-amber-900 hover:bg-amber-50"
                          : "bg-gray-50/70 border-gray-200 text-gray-900 hover:bg-gray-100/70"
                      )}
                    >
                      <span className="text-sm font-medium">{item.label}</span>
                      <ChevronRight className={cn("w-4 h-4", item.alert ? "text-amber-500" : "text-gray-400")} />
                    </Link>
                  ))}
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-6 text-center">
                  <div className="w-10 h-10 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center mb-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-500" />
                  </div>
                  <p className="text-gray-900 dark:text-white font-medium">Отлично! Все задачи выполнены.</p>
                  <p className="text-sm text-gray-500 mt-1">Проблем не найдено, магазин работает в штатном режиме.</p>
                </div>
              )}
            </SellerSurface>

            {/* Compact Onboarding if incomplete */}
            {!isProfileComplete && data.sellerMe && (
              <SellerSurface as="section" className="p-6">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div>
                    <h2 className="text-base font-semibold text-gray-900 dark:text-white">Заполненность профиля</h2>
                    <div className="mt-2 flex items-center gap-2">
                      <div className="h-2 w-48 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
                        <div className="h-full bg-emerald-500 rounded-full" style={{ width: `${progressPercent}%` }}></div>
                      </div>
                      <span className="text-sm font-medium text-gray-600 dark:text-gray-400">{progressPercent}%</span>
                    </div>
                  </div>
                  <Link to="/settings" className="inline-flex rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 shrink-0">
                    Настроить профиль
                  </Link>
                </div>
              </SellerSurface>
            )}

            {/* Action for New Sellers */}
            {isNewSeller && isProfileComplete && (
              <SellerSurface as="section" className="border-blue-200 bg-blue-50/50 dark:bg-blue-950/20 dark:border-blue-900/50 p-8 text-center">
                <div className="w-12 h-12 rounded-xl bg-blue-100 dark:bg-blue-900/40 flex items-center justify-center mx-auto mb-3 text-blue-600 dark:text-blue-400">
                  <Package className="w-6 h-6" />
                </div>
                <h2 className="text-lg font-bold text-gray-900 dark:text-white mb-2">Начните продажи на ZAMK</h2>
                <p className="text-gray-600 dark:text-gray-400 mb-6 max-w-md mx-auto text-sm">Ваш профиль готов к работе. Добавьте свой первый товар, чтобы покупатели могли его найти.</p>
                <Link
                  to="/products/new"
                  onClick={() => prepareAddProductNavigation()}
                  className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
                >
                  <PlusCircle className="w-4 h-4" />
                  Создать товар
                </Link>
              </SellerSurface>
            )}

            {/* Operating Summary */}
            <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <SellerKpiCard
                label="Товары"
                value={String(data.products.length)}
                supportText={data.products.length === 0 ? 'Нет товаров' : pluralize(data.products.length, ['товар в каталоге', 'товара в каталоге', 'товаров в каталоге'])}
                icon={Package}
                to="/products"
              />
              <SellerKpiCard
                label="Активные заказы"
                value={String(activeOrders)}
                supportText={activeOrders === 0 ? 'Нет активных заказов' : pluralize(activeOrders, ['заказ в работе', 'заказа в работе', 'заказов в работе'])}
                icon={ShoppingCart}
                to="/orders"
              />
              <SellerKpiCard
                label="Возвраты"
                value={String(data.returns.length)}
                supportText={data.returns.length === 0 ? 'Нет активных возвратов' : pluralize(data.returns.length, ['возврат ожидает', 'возврата ожидают', 'возвратов ожидают'])}
                icon={RotateCcw}
                to="/returns"
              />
              <SellerKpiCard
                label="Остатки"
                value={String(totalStock)}
                supportText={totalStock === 0 ? 'Нет товаров на складе' : pluralize(totalStock, ['единица доступна', 'единицы доступно', 'единиц доступно'])}
                icon={Archive}
                to="/inventory"
              />
            </section>

            {/* Warnings Summary if any */}
            {totalIssues > 0 && (
              <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <SellerKpiCard
                  label="Нарушения и предупреждения"
                  value={String(totalIssues)}
                  supportText={pluralize(totalIssues, ['активное предупреждение', 'активных предупреждения', 'активных предупреждений'])}
                  icon={AlertTriangle}
                  accent="danger"
                  to="/warnings"
                />
              </section>
            )}

          </>
        )}
    </SellerPageFrame>
  );
}

import { useEffect, useState } from 'react';
import { Package, RotateCcw, Store, Wallet, AlertTriangle, AlertCircle, Info, RefreshCw, Box, ClipboardList, Gavel } from 'lucide-react';
import { getDashboardSummary } from '@zamk/api-client';
import type { AdminDashboardSummary } from '@zamk/api-client';
import { HelpTooltip } from '../components/HelpTooltip';
import { Link } from 'react-router-dom';

const formatCents = (cents: number) => {
  return (cents / 100).toLocaleString('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 0, maximumFractionDigits: 0 });
};

export function AdminDashboard() {
  const [summary, setSummary] = useState<AdminDashboardSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const loadStats = async () => {
    setIsLoading(true);
    setError('');

    try {
      const data = await getDashboardSummary();
      setSummary(data);
      setLastUpdated(new Date());
    } catch (err: any) {
      console.error(err);
      setError('Не удалось загрузить сводку. Проверьте, запущен ли backend.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadStats();
  }, []);

  const renderAttentionBlock = () => {
    if (!summary?.attention || summary.attention.length === 0) return null;

    return (
      <div className="mb-8">
        <h2 className="text-lg font-medium text-gray-900 mb-4 flex items-center">
          <AlertTriangle className="h-5 w-5 text-red-500 mr-2" />
          Требует внимания
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {summary.attention.map((item, idx) => {
            const isDanger = item.severity === 'danger';
            const isWarning = item.severity === 'warning';
            
            let colorClasses = 'bg-blue-50 border-blue-200 text-blue-800';
            let icon = <Info className="h-5 w-5 text-blue-500" />;
            
            if (isDanger) {
              colorClasses = 'bg-red-50 border-red-200 text-red-800';
              icon = <AlertCircle className="h-5 w-5 text-red-500" />;
            } else if (isWarning) {
              colorClasses = 'bg-yellow-50 border-yellow-200 text-yellow-800';
              icon = <AlertTriangle className="h-5 w-5 text-yellow-500" />;
            }

            const content = (
              <div className={`border rounded-lg p-4 flex items-center justify-between ${colorClasses}`}>
                <div className="flex items-center">
                  {icon}
                  <span className="ml-3 font-medium">{item.title}</span>
                </div>
                <span className="text-xl font-bold">{item.count}</span>
              </div>
            );

            if (item.link) {
              return <Link to={item.link} key={idx} className="block transition-transform hover:scale-105">{content}</Link>;
            }
            return <div key={idx}>{content}</div>;
          })}
        </div>
      </div>
    );
  };

  const renderStatCard = (title: string, value: string | number, tooltip?: string) => (
    <div className="bg-white overflow-hidden shadow rounded-lg p-5">
      <div className="flex items-center mb-1">
        <p className="text-sm font-medium text-gray-500 truncate">{title}</p>
        {tooltip && <HelpTooltip content={tooltip} />}
      </div>
      <p className="text-2xl font-semibold text-gray-900">{isLoading ? '...' : value}</p>
    </div>
  );

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Сводка</h1>
          {lastUpdated && (
            <p className="text-sm text-gray-500 mt-1">
              Обновлено: {lastUpdated.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
            </p>
          )}
        </div>
        <button
          onClick={loadStats}
          disabled={isLoading}
          className="flex items-center px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          Обновить
        </button>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 p-4">
          <div className="flex">
            <div className="flex-shrink-0">
              <AlertCircle className="h-5 w-5 text-red-400" aria-hidden="true" />
            </div>
            <div className="ml-3">
              <h3 className="text-sm font-medium text-red-800">{error}</h3>
            </div>
          </div>
        </div>
      )}

      {isLoading && !summary && (
        <div className="text-center py-12">
          <RefreshCw className="h-8 w-8 text-indigo-500 animate-spin mx-auto mb-4" />
          <p className="text-gray-500">Загружаем сводку...</p>
        </div>
      )}

      {!isLoading && !summary && !error && (
        <div className="text-center py-12 bg-white rounded-lg shadow">
          <p className="text-gray-500">Данных пока нет.</p>
        </div>
      )}

      {summary && (
        <>
          {renderAttentionBlock()}

          {/* 1. Общая сводка */}
          <div>
            <h2 className="text-lg font-medium text-gray-900 mb-4 flex items-center">
              <Store className="h-5 w-5 text-indigo-500 mr-2" />
              Общая сводка
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              {renderStatCard('Всего заказов', summary.overview.totalOrders)}
              {renderStatCard('Заказы сегодня', summary.overview.ordersToday)}
              {renderStatCard('Выручка сегодня', formatCents(summary.overview.revenueTodayCents), 'Сумма оплаченных заказов за выбранный период.')}
              {renderStatCard('Выручка (7 дней)', formatCents(summary.overview.revenue7dCents), 'Сумма оплаченных заказов за выбранный период.')}
              {renderStatCard('Товары на модерации', summary.overview.pendingModeration, 'Товары или заявки, которые нужно проверить перед публикацией.')}
              {renderStatCard('Активные продавцы', summary.overview.activeSellers)}
              {renderStatCard('Активные товары', summary.overview.activeProducts)}
              {renderStatCard('Низкий остаток (кол-во)', summary.overview.lowStockCount, 'Товары, по которым осталось мало доступных единиц.')}
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mt-8">
            {/* 2. Заказы */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <ClipboardList className="h-4 w-4 text-blue-500 mr-2" />
                Заказы
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Новые / В обработке <HelpTooltip content="Заказы, которые ещё не завершены и требуют действий." />
                    </span>
                    <span className="font-semibold">{summary.orders.newOrPending}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Оплачены</span>
                    <span className="font-semibold">{summary.orders.paid}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">В сборке</span>
                    <span className="font-semibold">{summary.orders.inFulfillment}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Отправлены / Доставлены</span>
                    <span className="font-semibold">{summary.orders.shippedOrDelivered}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Отменены / Возвращены</span>
                    <span className="font-semibold">{summary.orders.cancelledOrRefunded}</span>
                  </li>
                </ul>
              </div>
            </div>

            {/* 3. Продавцы */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <Store className="h-4 w-4 text-purple-500 mr-2" />
                Продавцы
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Активные</span>
                    <span className="font-semibold">{summary.sellers.active}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Ожидают модерации <HelpTooltip content="Продавцы, отправившие заявку на регистрацию." />
                    </span>
                    <span className="font-semibold text-yellow-600">{summary.sellers.waitingModeration}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Заблокированные</span>
                    <span className="font-semibold text-red-600">{summary.sellers.blocked}</span>
                  </li>
                </ul>
              </div>
            </div>

            {/* 4. Товары */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <Package className="h-4 w-4 text-green-500 mr-2" />
                Товары
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Опубликованы</span>
                    <span className="font-semibold">{summary.products.published}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Ожидают модерации <HelpTooltip content="Товары или заявки, которые нужно проверить перед публикацией." />
                    </span>
                    <span className="font-semibold text-yellow-600">{summary.products.pendingModeration}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Отклонены / Заблокированы</span>
                    <span className="font-semibold text-red-600">{summary.products.rejectedOrBlocked}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Нет в наличии</span>
                    <span className="font-semibold">{summary.products.outOfStock}</span>
                  </li>
                </ul>
              </div>
            </div>

            {/* 5. Аукционы */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <Gavel className="h-4 w-4 text-indigo-500 mr-2" />
                Аукционы
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Активные</span>
                    <span className="font-semibold">{summary.auctions.active}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Лоты ожидают оплаты</span>
                    <span className="font-semibold">{summary.auctions.awaitingPayment}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Лоты без оплаты <HelpTooltip content="Выигранные лоты, по которым покупатель не оплатил заказ в срок." />
                    </span>
                    <span className="font-semibold text-red-600">{summary.auctions.unpaidManualReview}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">В прямой продаже (Вещи ZAMK)</span>
                    <span className="font-semibold">{summary.auctions.directSaleItems}</span>
                  </li>
                </ul>
              </div>
            </div>

            {/* 6. Склад / остатки */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <Box className="h-4 w-4 text-orange-500 mr-2" />
                Склад / остатки
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Низкий остаток <HelpTooltip content="Товары, по которым осталось мало доступных единиц." />
                    </span>
                    <span className="font-semibold text-yellow-600">{summary.inventory.lowStockVariants}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Резерв <HelpTooltip content="Количество товара, которое уже занято заказами, но ещё не списано окончательно." />
                    </span>
                    <span className="font-semibold">{summary.inventory.reservedStock}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Отсутствуют на складе</span>
                    <span className="font-semibold text-red-600">{summary.inventory.outOfStockCount}</span>
                  </li>
                </ul>
              </div>
            </div>

            {/* 7. Платежи / выплаты */}
            <div>
              <h3 className="text-md font-medium text-gray-900 mb-3 flex items-center">
                <Wallet className="h-4 w-4 text-green-600 mr-2" />
                Платежи / выплаты
              </h3>
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <ul className="divide-y divide-gray-200">
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Оплаченные заказы (сумма)</span>
                    <span className="font-semibold">{formatCents(summary.payments.paidOrdersSumCents)}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600 flex items-center">
                      Выплаты <HelpTooltip content="Деньги, которые должны быть перечислены продавцам." />
                    </span>
                    <span className="font-semibold">{formatCents(summary.payments.pendingPayoutsCents)}</span>
                  </li>
                  <li className="px-4 py-3 flex justify-between">
                    <span className="text-sm text-gray-600">Ошибки платежей</span>
                    <span className="font-semibold text-red-600">{summary.payments.failedPaymentsCount}</span>
                  </li>
                </ul>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

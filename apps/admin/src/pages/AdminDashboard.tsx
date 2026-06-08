import { useEffect, useState } from 'react';
import { Package, RotateCcw, Store, Wallet } from 'lucide-react';
import { getAdminSellers } from '../api/adminOperations';
import { getAdminProducts, getModerationProducts } from '../api/adminProducts';
import { getAdminOrders } from '../api/adminOrders';
import { getAdminReturns } from '../api/adminReturns';
import { getAdminPayouts } from '../api/adminPayouts';

const unwrapItems = <T,>(response: T[] | { items?: T[] } | null): T[] => {
  if (!response) return [];
  return Array.isArray(response) ? response : response.items ?? [];
};

export function AdminDashboard() {
  const [stats, setStats] = useState({
    sellers: 0,
    products: 0,
    pendingModeration: 0,
    activeOrders: 0,
    pendingReturns: 0,
    pendingPayouts: 0,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    async function loadStats() {
      setIsLoading(true);
      setError('');

      try {
        const [sellersResponse, productsResponse, moderationResponse, ordersResponse, returnsResponse, payoutsResponse] = await Promise.all([
          getAdminSellers(),
          getAdminProducts(),
          getModerationProducts(),
          getAdminOrders(),
          getAdminReturns(),
          getAdminPayouts(),
        ]);

        if (!cancelled) {
          const sellers = unwrapItems(sellersResponse);
          const products = unwrapItems(productsResponse);
          const moderationProducts = unwrapItems(moderationResponse);
          const orders = unwrapItems(ordersResponse);
          const returns = unwrapItems(returnsResponse);
          const payouts = unwrapItems(payoutsResponse);

          setStats({
            sellers: sellers.length,
            products: products.length,
            pendingModeration: moderationProducts.length,
            activeOrders: orders.filter((order) => !['delivered', 'cancelled'].includes(order.status)).length,
            pendingReturns: returns.filter((item) => ['requested', 'approved'].includes(item.status)).length,
            pendingPayouts: payouts.filter((item) => ['requested', 'approved'].includes(item.status)).length,
          });
        }
      } catch {
        if (!cancelled) {
          setError('Не удалось загрузить метрики панели. Проверьте, запущен ли backend.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadStats();

    return () => {
      cancelled = true;
    };
  }, []);

  const statCards = [
    { name: 'Активные продавцы', value: stats.sellers, icon: Store, color: 'bg-indigo-500' },
    { name: 'Товары', value: stats.products, icon: Package, color: 'bg-blue-500' },
    { name: 'На модерации', value: stats.pendingModeration, icon: Package, color: 'bg-yellow-500' },
    { name: 'Активные заказы', value: stats.activeOrders, icon: Package, color: 'bg-green-500' },
    { name: 'Ожидают возврата', value: stats.pendingReturns, icon: RotateCcw, color: 'bg-red-500' },
    { name: 'Ожидают выплаты', value: stats.pendingPayouts, icon: Wallet, color: 'bg-purple-500' },
  ];

  return (
    <div className="space-y-6">
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {statCards.map((stat) => (
          <div key={stat.name} className="bg-white overflow-hidden shadow rounded-lg flex items-center p-5">
            <div className={`p-3 rounded-md ${stat.color} text-white mr-4`}>
              <stat.icon className="h-6 w-6" aria-hidden="true" />
            </div>
            <div>
              <p className="text-sm font-medium text-gray-500 truncate">{stat.name}</p>
              <p className="mt-1 text-2xl font-semibold text-gray-900">{isLoading ? '...' : stat.value}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Модерация</h2>
          <div className="rounded-lg border border-dashed border-gray-300 p-4 text-sm text-gray-500">
            {isLoading ? 'Загрузка...' : stats.pendingModeration > 0 ? `Товаров на модерации: ${stats.pendingModeration}` : 'Нет данных'}
          </div>
        </div>

        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Журнал действий</h2>
          <div className="rounded-lg border border-dashed border-gray-300 p-4 text-sm text-gray-500">
            Журнал действий пока не подключён.
          </div>
        </div>
      </div>
      
      <div className="bg-gray-50 border-l-4 border-gray-300 p-4 mt-6">
        <p className="text-sm text-gray-700">
          Неподключённые агрегированные метрики не заполняются демо-данными. Данные появятся после подключения backend-агрегаций.
        </p>
      </div>
    </div>
  );
}

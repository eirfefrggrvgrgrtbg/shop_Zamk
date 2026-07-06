import { useState, useEffect } from 'react';
import { getAdminReportsSummary } from '@zamk/api-client/src/admin';
import { FileText, AlertCircle, TrendingUp, Users, ShoppingCart, Package } from 'lucide-react';

export function AdminReports() {
  const [report, setReport] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadReport();
  }, []);

  const loadReport = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await getAdminReportsSummary();
      setReport(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить отчеты');
    } finally {
      setIsLoading(false);
    }
  };

  const formatCurrency = (cents: number, currency: string) => {
    return new Intl.NumberFormat('ru-RU', { style: 'currency', currency }).format(cents / 100);
  };

  if (isLoading) {
    return (
      <div className="text-center py-10">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
        <p className="mt-2 text-sm text-gray-500">Загрузка отчетов...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
        <AlertCircle className="h-5 w-5 mr-2 shrink-0" />
        {error}
      </div>
    );
  }

  if (!report) return null;

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Сводные отчеты</h1>
          <p className="mt-1 text-sm text-gray-500">
            Общая статистика и показатели платформы
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <ShoppingCart className="h-6 w-6 text-indigo-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Всего заказов</dt>
                  <dd>
                    <div className="text-lg font-medium text-gray-900">{report.totalOrders}</div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <TrendingUp className="h-6 w-6 text-green-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Оборот</dt>
                  <dd>
                    <div className="text-lg font-medium text-gray-900">
                      {formatCurrency(report.totalRevenueCents, report.currency)}
                    </div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Users className="h-6 w-6 text-blue-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Продавцы (всего / актив)</dt>
                  <dd>
                    <div className="text-lg font-medium text-gray-900">{report.totalSellers} / {report.activeSellers}</div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="p-5">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Package className="h-6 w-6 text-orange-400" />
              </div>
              <div className="ml-5 w-0 flex-1">
                <dl>
                  <dt className="text-sm font-medium text-gray-500 truncate">Товары (всего / опубл.)</dt>
                  <dd>
                    <div className="text-lg font-medium text-gray-900">{report.totalProducts} / {report.publishedProducts}</div>
                  </dd>
                </dl>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white shadow rounded-lg p-6">
        <h3 className="text-lg leading-6 font-medium text-gray-900 flex items-center mb-4">
          <FileText className="h-5 w-5 mr-2 text-gray-400" />
          Детализация
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <ul className="divide-y divide-gray-200">
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Ожидают модерации (товары)</span>
              <span className="text-sm font-medium text-gray-900">{report.pendingProducts}</span>
            </li>
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Отклонены / Заблокированы</span>
              <span className="text-sm font-medium text-gray-900">{report.rejectedProducts}</span>
            </li>
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Остаток заканчивается (SKU)</span>
              <span className="text-sm font-medium text-gray-900">{report.lowStockItems}</span>
            </li>
          </ul>
          <ul className="divide-y divide-gray-200">
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Ожидающие выплаты</span>
              <span className="text-sm font-medium text-gray-900">{formatCurrency(report.pendingPayouts, report.currency)}</span>
            </li>
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Оплачено заказов на сумму</span>
              <span className="text-sm font-medium text-gray-900">{formatCurrency(report.paidPayouts, report.currency)}</span>
            </li>
            <li className="py-3 flex justify-between">
              <span className="text-sm text-gray-500">Открытые жалобы</span>
              <span className="text-sm font-medium text-gray-900">{report.openComplaints}</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
}

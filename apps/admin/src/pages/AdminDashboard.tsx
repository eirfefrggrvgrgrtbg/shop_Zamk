import { Users, Store, Package, ShoppingCart, RotateCcw, Wallet } from 'lucide-react';
import { mockUsers, mockOrders, mockReturns, mockPayouts, mockAuditLogs } from '../lib/mock-admin-data';

export function AdminDashboard() {
  const stats = [
    { name: 'Total Users', value: mockUsers.length, icon: Users, color: 'bg-blue-500' },
    { name: 'Active Sellers', value: 0, icon: Store, color: 'bg-indigo-500' },
    { name: 'Pending Moderation', value: 0, icon: Package, color: 'bg-yellow-500' },
    { name: 'Active Orders', value: mockOrders.filter(o => o.status !== 'delivered' && o.status !== 'cancelled').length, icon: ShoppingCart, color: 'bg-green-500' },
    { name: 'Pending Returns', value: mockReturns.filter(r => r.status === 'pending').length, icon: RotateCcw, color: 'bg-red-500' },
    { name: 'Pending Payouts', value: mockPayouts.filter(p => p.status === 'pending').length, icon: Wallet, color: 'bg-purple-500' },
  ];

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {stats.map((stat) => (
          <div key={stat.name} className="bg-white overflow-hidden shadow rounded-lg flex items-center p-5">
            <div className={`p-3 rounded-md ${stat.color} text-white mr-4`}>
              <stat.icon className="h-6 w-6" aria-hidden="true" />
            </div>
            <div>
              <p className="text-sm font-medium text-gray-500 truncate">{stat.name}</p>
              <p className="mt-1 text-2xl font-semibold text-gray-900">{stat.value}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Pending Moderation</h2>
          <div className="flow-root">
            <ul className="-my-5 divide-y divide-gray-200">
              <div className="p-4 text-center text-sm text-gray-500 border-b">
                Data will be loaded from real API in later phases.
              </div>
            </ul>
          </div>
        </div>

        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-lg font-medium text-gray-900 mb-4">Recent Audit Logs</h2>
          <div className="flow-root">
            <ul className="-my-5 divide-y divide-gray-200">
              {mockAuditLogs.map(log => (
                <li key={log.id} className="py-4">
                  <div className="flex items-center space-x-4">
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 truncate">{log.action}</p>
                      <p className="text-sm text-gray-500 truncate">by {log.actor} • {new Date(log.createdAt).toLocaleString('ru-RU')}</p>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
      
      <div className="bg-indigo-50 border-l-4 border-indigo-400 p-4 mt-6">
        <div className="flex">
          <div className="ml-3">
            <p className="text-sm text-indigo-700">
              <strong>Security Note:</strong> Admin dashboard metrics currently use mock data. Real backend aggregation and role verification will be implemented later.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

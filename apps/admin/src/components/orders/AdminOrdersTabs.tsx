import { NavLink } from 'react-router-dom';
import { ShoppingBag, PackageCheck, QrCode, AlertTriangle } from 'lucide-react';

export function AdminOrdersTabs() {
  const tabs = [
    { path: '/orders', label: 'Заказы', icon: ShoppingBag, exact: true },
    { path: '/fulfillment/picking', label: 'Сборка заказов', icon: PackageCheck },
    { path: '/orders/fulfillments', label: 'Сборки продавцов', icon: PackageCheck },
    { path: '/orders/receiving', label: 'Приёмка', icon: QrCode },
    { path: '/orders/problems', label: 'Проблемы', icon: AlertTriangle },
  ];

  return (
    <div className="border-b border-slate-700 bg-slate-900/60 mb-6 px-4 sm:px-6">
      <nav className="-mb-px flex space-x-6 overflow-x-auto" aria-label="Orders Navigation">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <NavLink
              key={tab.path}
              to={tab.path}
              end={tab.exact}
              className={({ isActive }) =>
                `group inline-flex items-center py-4 px-1 border-b-2 font-medium text-sm transition-colors whitespace-nowrap ${
                  isActive
                    ? 'border-indigo-500 text-indigo-400 font-semibold'
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-500'
                }`
              }
            >
              <Icon className="mr-2 h-4 w-4" />
              <span>{tab.label}</span>
            </NavLink>
          );
        })}
      </nav>
    </div>
  );
}

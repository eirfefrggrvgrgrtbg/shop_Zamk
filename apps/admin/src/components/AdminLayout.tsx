import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Store,
  Package,
  ShieldAlert,
  ShoppingCart,
  Boxes,
  RotateCcw,
  Wallet,
  LogOut,
  Bell,
  BookOpen,
  CreditCard,
  Truck,
  ReceiptText,
  Star,
  Users,
  Shield,
  ClipboardList,
} from 'lucide-react';

import { useAdminAuth } from '../contexts/AdminAuthContext';

interface NavItem {
  name: string;
  path: string;
  icon: React.ElementType;
  permission?: string | string[];
}

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout, user, staff, hasPermission, hasAnyPermission } = useAdminAuth();

  const isPermissionVisible = (permission?: string | string[]) => {
    if (!permission) return true;
    // If staff is null (legacy admin with no staff row), hide all permission-gated items
    if (staff === null) return false;
    if (Array.isArray(permission)) return hasAnyPermission(permission);
    return hasPermission(permission);
  };

  const baseNavItems: NavItem[] = [
    { name: 'Главная', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Продавцы', path: '/sellers', icon: Store, permission: 'sellers.read' },
    { name: 'Товары', path: '/products', icon: Package, permission: 'products.read' },
    { name: 'Модерация', path: '/moderation', icon: ShieldAlert, permission: 'products.moderate' },
    { name: 'Категории и бренды', path: '/catalog', icon: BookOpen, permission: ['categories.read', 'brands.read'] },
    { name: 'Заказы', path: '/orders', icon: ShoppingCart, permission: 'orders.read' },
    { name: 'Доставка / Отгрузки', path: '/shipments', icon: Truck, permission: 'shipments.read' },
    { name: 'Остатки / Склад', path: '/inventory', icon: Boxes, permission: 'inventory.read' },
    { name: 'Платежи покупателей', path: '/payments', icon: CreditCard, permission: 'payments.read' },
    { name: 'Возвраты', path: '/returns', icon: RotateCcw, permission: 'returns.read' },
    { name: 'Возмещения', path: '/refunds', icon: ReceiptText, permission: 'refunds.read' },
    { name: 'Выплаты продавцам', path: '/payouts', icon: Wallet, permission: 'payouts.read' },
    { name: 'Отзывы', path: '/reviews', icon: Star, permission: 'reviews.read' },
  ];

  const staffNavItems: NavItem[] = [
    { name: 'Доступы и роли', path: '/roles', icon: Shield, permission: 'roles.read' },
    { name: 'Сотрудники', path: '/staff', icon: Users, permission: 'staff.read' },
    { name: 'Журнал действий', path: '/audit', icon: ClipboardList, permission: 'audit.read' },
  ];

  const visibleBaseItems = baseNavItems.filter(item => isPermissionVisible(item.permission));
  const visibleStaffItems = staffNavItems.filter(item => isPermissionVisible(item.permission));

  const allNavItems = [...visibleBaseItems, ...visibleStaffItems];

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 text-white flex flex-col hidden md:flex shrink-0">
        <div className="p-4 border-b border-slate-800">
          <Link to="/dashboard" className="text-xl font-bold tracking-wider">ZAMK Admin</Link>
        </div>
        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {visibleBaseItems.map((item) => {
            const isActive = location.pathname === item.path || (location.pathname.startsWith(item.path + '/') && item.path !== '/');
            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors ${
                  isActive ? "bg-slate-800 text-white" : "text-slate-300 hover:bg-slate-800 hover:text-white"
                }`}
              >
                <item.icon className={`mr-3 h-5 w-5 flex-shrink-0 ${isActive ? "text-indigo-400" : "text-slate-400 group-hover:text-slate-300"}`} />
                {item.name}
              </Link>
            );
          })}

          {visibleStaffItems.length > 0 && (
            <>
              <div className="pt-4 pb-1">
                <p className="px-3 text-xs font-semibold text-slate-500 uppercase tracking-wider">Администрирование</p>
              </div>
              {visibleStaffItems.map((item) => {
                const isActive = location.pathname === item.path || location.pathname.startsWith(item.path + '/');
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    className={`flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors ${
                      isActive ? "bg-slate-800 text-white" : "text-slate-300 hover:bg-slate-800 hover:text-white"
                    }`}
                  >
                    <item.icon className={`mr-3 h-5 w-5 flex-shrink-0 ${isActive ? "text-indigo-400" : "text-slate-400 group-hover:text-slate-300"}`} />
                    {item.name}
                  </Link>
                );
              })}
            </>
          )}
        </nav>
        <div className="p-4 border-t border-slate-800">
          <button onClick={() => logout()} className="w-full flex items-center px-3 py-2 text-sm font-medium text-slate-300 rounded-md hover:bg-slate-800 transition-colors">
            <LogOut className="mr-3 h-5 w-5 flex-shrink-0 text-slate-400" />
            Выйти
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top Header */}
        <header className="bg-white border-b border-gray-200 h-16 flex items-center justify-between px-6 shrink-0">
          <h1 className="text-xl font-semibold text-gray-800">
            {allNavItems.find(item => location.pathname.startsWith(item.path))?.name || 'Панель администратора'}
          </h1>
          <div className="flex items-center space-x-4">
            <button className="text-gray-400 hover:text-gray-600 transition-colors relative">
              <Bell className="h-6 w-6" />
              <span className="absolute top-0 right-0 block h-2 w-2 rounded-full bg-red-500 ring-2 ring-white"></span>
            </button>
            <div className="h-8 w-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-700 font-bold uppercase" title={user?.email}>
              {user?.email?.charAt(0) || 'A'}
            </div>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto p-6">
          <div className="max-w-7xl mx-auto">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}

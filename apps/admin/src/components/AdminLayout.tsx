import React, { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Store,
  Package,
  ShieldAlert,
  ShoppingCart,
  Boxes,
  Gavel,
  RotateCcw,
  Wallet,
  LogOut,
  BookOpen,
  CreditCard,
  Truck,
  ReceiptText,
  Star,
  Users,
  Shield,
  ClipboardList,
  FileText,
  PanelLeftClose,
  PanelLeftOpen,
} from 'lucide-react';

import { useAdminAuth } from '../contexts/AdminAuthContext';
import { NotificationBell } from './notifications/NotificationBell';

interface NavItem {
  name: string;
  path: string;
  icon: React.ElementType;
  permission?: string | string[];
}

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout, user, staff, hasPermission, hasAnyPermission } = useAdminAuth();

  // Collapsed sidebar state from localStorage ONLY (Manual control)
  const [isCollapsed, setIsCollapsed] = useState<boolean>(() => {
    try {
      return localStorage.getItem('adminSidebarCollapsed') === 'true';
    } catch {
      return false;
    }
  });

  const toggleSidebar = () => {
    setIsCollapsed(prev => {
      const next = !prev;
      try {
        localStorage.setItem('adminSidebarCollapsed', String(next));
      } catch {}
      return next;
    });
  };

  const isPermissionVisible = (permission?: string | string[]) => {
    if (!permission) return true;
    if (staff === null) return false;
    if (Array.isArray(permission)) return hasAnyPermission(permission);
    return hasPermission(permission);
  };

  const baseNavItems: NavItem[] = [
    { name: 'Главная', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Продавцы', path: '/sellers', icon: Store, permission: 'sellers.read' },
    { name: 'Аукционы', path: '/auctions', icon: Gavel, permission: 'auctions.read' },
    { name: 'Товары', path: '/products', icon: Package, permission: 'products.read' },
    { name: 'Модерация', path: '/moderation', icon: ShieldAlert, permission: 'products.moderate' },
    { name: 'Категории и бренды', path: '/catalog', icon: BookOpen, permission: ['categories.read', 'brands.read'] },
    { name: 'Заказы', path: '/orders', icon: ShoppingCart, permission: 'orders.read' },
    { name: 'Доставка / Отгрузки', path: '/shipments', icon: Truck, permission: 'shipments.read' },
    { name: 'Остатки / Склад', path: '/inventory', icon: Boxes, permission: 'inventory.read' },
    { name: 'Приемка поставок', path: '/supplies/receiving', icon: Truck, permission: 'inventory.read' },
    { name: 'Платежи покупателей', path: '/payments', icon: CreditCard, permission: 'payments.read' },
    { name: 'Возвраты', path: '/returns', icon: RotateCcw, permission: 'returns.read' },
    { name: 'Возмещения', path: '/refunds', icon: ReceiptText, permission: 'refunds.read' },
    { name: 'Выплаты продавцам', path: '/payouts', icon: Wallet, permission: 'payouts.read' },
    { name: 'Отзывы', path: '/reviews', icon: Star, permission: 'reviews.read' },
  ];

  const staffNavItems: NavItem[] = [
    { name: 'Сводные отчеты', path: '/reports', icon: FileText, permission: 'reports.read' },
    { name: 'Доступы и роли', path: '/roles', icon: Shield, permission: 'roles.read' },
    { name: 'Сотрудники', path: '/staff', icon: Users, permission: 'staff.read' },
    { name: 'Журнал действий', path: '/audit', icon: ClipboardList, permission: 'audit.read' },
  ];

  const visibleBaseItems = baseNavItems.filter(item => isPermissionVisible(item.permission));
  const visibleStaffItems = staffNavItems.filter(item => isPermissionVisible(item.permission));
  const allNavItems = [...visibleBaseItems, ...visibleStaffItems];

  return (
    <div data-testid="admin-layout" className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside
        className={`${
          isCollapsed ? 'w-[72px]' : 'w-64'
        } bg-slate-900 text-white flex flex-col hidden md:flex shrink-0 transition-all duration-300 ease-in-out`}
      >
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <Link to="/dashboard" className="font-bold tracking-wider truncate">
            {isCollapsed ? 'ZAMK' : 'ZAMK Admin'}
          </Link>
          <button
            onClick={toggleSidebar}
            title={isCollapsed ? 'Развернуть меню' : 'Свернуть меню'}
            className="p-1 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors ml-1"
          >
            {isCollapsed ? <PanelLeftOpen className="w-5 h-5" /> : <PanelLeftClose className="w-5 h-5" />}
          </button>
        </div>

        <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
          {visibleBaseItems.map((item) => {
            const isActive = location.pathname === item.path || (location.pathname.startsWith(item.path + '/') && item.path !== '/');
            return (
              <Link
                key={item.path}
                to={item.path}
                title={isCollapsed ? item.name : undefined}
                className={`flex items-center px-3 py-2.5 text-sm font-medium rounded-xl group transition-colors ${
                  isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                } ${isCollapsed ? 'justify-center' : ''}`}
              >
                <item.icon className={`h-5 w-5 flex-shrink-0 ${isActive ? 'text-indigo-400' : 'text-slate-400 group-hover:text-slate-300'} ${!isCollapsed ? 'mr-3' : ''}`} />
                {!isCollapsed && <span className="truncate">{item.name}</span>}
              </Link>
            );
          })}

          {visibleStaffItems.length > 0 && (
            <>
              <div className="pt-4 pb-1">
                {!isCollapsed ? (
                  <p className="px-3 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">Администрирование</p>
                ) : (
                  <div className="w-full h-px bg-slate-800 my-2" />
                )}
              </div>
              {visibleStaffItems.map((item) => {
                const isActive = location.pathname === item.path || location.pathname.startsWith(item.path + '/');
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={isCollapsed ? item.name : undefined}
                    className={`flex items-center px-3 py-2.5 text-sm font-medium rounded-xl group transition-colors ${
                      isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                    } ${isCollapsed ? 'justify-center' : ''}`}
                  >
                    <item.icon className={`h-5 w-5 flex-shrink-0 ${isActive ? 'text-indigo-400' : 'text-slate-400 group-hover:text-slate-300'} ${!isCollapsed ? 'mr-3' : ''}`} />
                    {!isCollapsed && <span className="truncate">{item.name}</span>}
                  </Link>
                );
              })}
            </>
          )}
        </nav>

        <div className="p-3 border-t border-slate-800">
          <button
            onClick={() => logout()}
            title={isCollapsed ? 'Выйти' : undefined}
            className={`w-full flex items-center px-3 py-2.5 text-sm font-medium text-slate-300 rounded-xl hover:bg-slate-800 transition-colors ${
              isCollapsed ? 'justify-center' : ''
            }`}
          >
            <LogOut className={`h-5 w-5 flex-shrink-0 text-slate-400 ${!isCollapsed ? 'mr-3' : ''}`} />
            {!isCollapsed && <span>Выйти</span>}
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top Header */}
        <header className="bg-white border-b border-gray-200 h-16 flex items-center justify-between px-6 shrink-0">
          <div className="flex items-center text-sm font-medium text-gray-500">
            <span className="hidden sm:inline">ZAMK Admin</span>
            <span className="hidden sm:inline mx-2 text-gray-300">/</span>
            <span className="text-gray-800 font-semibold">
              {allNavItems.find(item => location.pathname.startsWith(item.path))?.name || 'Панель администратора'}
            </span>
          </div>
          <div className="flex items-center space-x-4">
            <NotificationBell />
            <div className="h-8 w-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-700 font-bold uppercase" title={user?.email}>
              {user?.email?.charAt(0) || 'A'}
            </div>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto p-6 bg-gray-50 dark:bg-gray-900">
          {children}
        </main>
      </div>
    </div>
  );
}

import React, { useState, useEffect } from 'react';
import { Link, useLocation, Outlet } from 'react-router-dom';
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
  Users,
  Shield,
  ClipboardList,
  FileText,
  PanelLeftClose,
  PanelLeftOpen,
  PackageCheck,
  Search,
} from 'lucide-react';

import { useAdminAuth } from '../contexts/AdminAuthContext';
import { NotificationBell } from './notifications/NotificationBell';
import { AdminSearchPalette } from './search/AdminSearchPalette';
import { useAdminGlobalSearchShortcut } from './search/useAdminGlobalSearchShortcut';
import { getAdminSellers } from '@zamk/api-client/src/admin';
import { getModerationProducts } from '../api/adminProducts';
import { getAdminReviews } from '../api/adminReviews';
import { getAdminPickingQueue } from '../api/adminPicking';

interface NavItem {
  name: string;
  path: string;
  icon: React.ElementType;
  permission?: string | string[];
}

export function AdminLayout({ children }: { children?: React.ReactNode }) {
  const location = useLocation();
  const { logout, user, staff, hasPermission, hasAnyPermission } = useAdminAuth();
  const [isSearchOpen, setIsSearchOpen] = useState(false);

  // Global shortcut listener for Cmd+K / Ctrl+K with input safety
  useAdminGlobalSearchShortcut(isSearchOpen, setIsSearchOpen);

  const isMac = typeof window !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

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

  // Moderation pending counts
  const [moderationCounts, setModerationCounts] = useState({
    total: 0,
    sellers: 0,
    products: 0,
    reviews: 0,
  });
  const [pickingCount, setPickingCount] = useState<number>(0);

  useEffect(() => {
    let isMounted = true;
    const loadCounts = async () => {
      try {
        const canReadSellers = isPermissionVisible('sellers.read');
        const canModerateProducts = isPermissionVisible('products.moderate');
        const canReadReviews = isPermissionVisible('reviews.read');
        const canReadOrders = isPermissionVisible('orders.read');

        const [sellersRes, productsRes, reviewsRes, pickingRes] = await Promise.allSettled([
          canReadSellers ? getAdminSellers({ limit: 100 }) : Promise.resolve({ items: [] }),
          canModerateProducts ? getModerationProducts({ status: 'pending_moderation', limit: 1 }) : Promise.resolve({ items: [], totalCount: 0 }),
          canReadReviews ? getAdminReviews() : Promise.resolve([]),
          canReadOrders ? getAdminPickingQueue() : Promise.resolve([]),
        ]);

        let pCount = 0;
        if (pickingRes.status === 'fulfilled' && Array.isArray(pickingRes.value)) {
          pCount = pickingRes.value.length;
        }

        let sellersCount = 0;
        if (sellersRes.status === 'fulfilled') {
          const items = sellersRes.value?.items || [];
          sellersCount = items.filter((s: any) => s.status === 'pending' || s.status === 'pending_setup' || s.status === 'pending_review').length;
        }

        let productsCount = 0;
        if (productsRes.status === 'fulfilled') {
          productsCount = productsRes.value?.totalCount ?? (productsRes.value?.items?.length || 0);
        }

        let reviewsCount = 0;
        if (reviewsRes.status === 'fulfilled') {
          const items = Array.isArray(reviewsRes.value) ? reviewsRes.value : (reviewsRes.value as any)?.items || [];
          reviewsCount = items.filter((r: any) => r.status === 'pending_moderation').length;
        }

        if (isMounted) {
          setPickingCount(pCount);
          setModerationCounts({
            sellers: sellersCount,
            products: productsCount,
            reviews: reviewsCount,
            total: sellersCount + productsCount + reviewsCount,
          });
        }
      } catch {}
    };

    loadCounts();
    const interval = setInterval(loadCounts, 30_000);
    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, [location.pathname, staff]);

  const baseNavItems: NavItem[] = [
    { name: 'Главная', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Продавцы', path: '/sellers', icon: Store, permission: 'sellers.read' },
    { name: 'Аукционы', path: '/auctions', icon: Gavel, permission: 'auctions.read' },
    { name: 'Товары', path: '/products', icon: Package, permission: 'products.read' },
    { name: 'Модерация', path: '/moderation', icon: ShieldAlert, permission: ['products.moderate', 'reviews.read', 'sellers.read'] },
    { name: 'Категории и бренды', path: '/catalog', icon: BookOpen, permission: ['categories.read', 'brands.read'] },
    { name: 'Заказы', path: '/orders', icon: ShoppingCart, permission: 'orders.read' },
    { name: 'Сборка заказов', path: '/fulfillment/picking', icon: PackageCheck, permission: 'orders.read' },
    { name: 'Доставка / Отгрузки', path: '/shipments', icon: Truck, permission: 'shipments.read' },
    { name: 'Остатки / Склад', path: '/inventory', icon: Boxes, permission: 'inventory.read' },
    { name: 'Приемка поставок', path: '/supplies/receiving', icon: Truck, permission: 'inventory.read' },
    { name: 'Платежи покупателей', path: '/payments', icon: CreditCard, permission: 'payments.read' },
    { name: 'Возвраты', path: '/returns', icon: RotateCcw, permission: ['returns.read', 'warehouse.returns'] },
    { name: 'Возмещения', path: '/refunds', icon: ReceiptText, permission: 'refunds.read' },
    { name: 'Выплаты продавцам', path: '/payouts', icon: Wallet, permission: 'payouts.read' },
  ];

  const staffNavItems: NavItem[] = [
    { name: 'Сводные отчеты', path: '/reports', icon: FileText, permission: 'reports.read' },
    { name: 'Доступы и роли', path: '/roles', icon: Shield, permission: 'roles.read' },
    { name: 'Сотрудники', path: '/staff', icon: Users, permission: 'staff.read' },
    { name: 'Журнал действий', path: '/audit', icon: ClipboardList, permission: 'audit.read' },
  ];

  const canReadSellers = isPermissionVisible('sellers.read');
  const canModerateProducts = isPermissionVisible('products.moderate');
  const canReadReviews = isPermissionVisible('reviews.read');

  const moderationSubItems = [
    { name: 'Очередь', path: '/moderation/queue', count: moderationCounts.total, visible: canReadSellers || canModerateProducts || canReadReviews },
    { name: 'Продавцы', path: '/moderation/sellers', count: moderationCounts.sellers, visible: canReadSellers },
    { name: 'Товары', path: '/moderation/products', count: moderationCounts.products, visible: canModerateProducts },
    { name: 'Отзывы', path: '/moderation/reviews', count: moderationCounts.reviews, visible: canReadReviews },
  ].filter((s) => s.visible);

  const visibleBaseItems = baseNavItems.filter(item => isPermissionVisible(item.permission));
  const visibleStaffItems = staffNavItems.filter(item => isPermissionVisible(item.permission));
  const allNavItems = [...visibleBaseItems, ...visibleStaffItems];

  const isModerationActive = location.pathname.startsWith('/moderation');

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
            const isModerationItem = item.path === '/moderation';
            const isActive = isModerationItem
              ? isModerationActive
              : (location.pathname === item.path || (location.pathname.startsWith(item.path + '/') && item.path !== '/'));

            return (
              <div key={item.path}>
                <Link
                  to={item.path}
                  title={isCollapsed ? item.name : undefined}
                  className={`flex items-center justify-between px-3 py-2.5 text-sm font-medium rounded-xl group transition-colors ${
                    isActive ? 'bg-slate-800 text-white shadow-sm' : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                  } ${isCollapsed ? 'justify-center' : ''}`}
                >
                  <div className="flex items-center min-w-0">
                    <item.icon className={`h-5 w-5 flex-shrink-0 ${isActive ? 'text-indigo-400' : 'text-slate-400 group-hover:text-slate-300'} ${!isCollapsed ? 'mr-3' : ''}`} />
                    {!isCollapsed && <span className="truncate">{item.name}</span>}
                  </div>

                  {!isCollapsed && isModerationItem && moderationCounts.total > 0 && (
                    <span className="ml-2 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/30">
                      {moderationCounts.total}
                    </span>
                  )}

                  {!isCollapsed && item.path === '/fulfillment/picking' && pickingCount > 0 && (
                    <span
                      data-testid="sidebar-picking-count"
                      className="ml-2 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-indigo-500/20 text-indigo-300 border border-indigo-500/30"
                    >
                      {pickingCount}
                    </span>
                  )}
                </Link>

                {/* Sub-items for Moderation */}
                {!isCollapsed && isModerationItem && isModerationActive && (
                  <div className="ml-5 pl-3 border-l border-slate-700/60 my-1 space-y-0.5">
                    {moderationSubItems.map((sub) => {
                      const isSubActive =
                        location.pathname === sub.path ||
                        (sub.path === '/moderation/queue' && (location.pathname === '/moderation' || location.pathname === '/moderation/')) ||
                        (sub.path === '/moderation/products' && location.pathname.startsWith('/moderation/products')) ||
                        (sub.path === '/moderation/sellers' && location.pathname.startsWith('/moderation/sellers')) ||
                        (sub.path === '/moderation/reviews' && location.pathname.startsWith('/moderation/reviews'));

                      return (
                        <Link
                          key={sub.path}
                          to={sub.path}
                          className={`flex items-center justify-between px-2.5 py-1.5 text-xs font-medium rounded-lg transition-colors ${
                            isSubActive
                              ? 'bg-indigo-600/30 text-indigo-300 font-semibold'
                              : 'text-slate-400 hover:bg-slate-800/80 hover:text-slate-200'
                          }`}
                        >
                          <span className="truncate">{sub.name}</span>
                          {sub.count > 0 && (
                            <span className="ml-1.5 px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-slate-800 text-slate-300 border border-slate-700">
                              {sub.count}
                            </span>
                          )}
                        </Link>
                      );
                    })}
                  </div>
                )}
              </div>
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
              {isModerationActive
                ? 'Модерация'
                : (allNavItems.find(item => location.pathname.startsWith(item.path))?.name || 'Панель администратора')}
            </span>
          </div>
          <div className="flex items-center space-x-3 sm:space-x-4">
            {/* Global Search Button */}
            <button
              type="button"
              data-testid="admin-global-search-trigger"
              onClick={() => setIsSearchOpen(true)}
              className="flex items-center space-x-2 px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700 bg-gray-100/80 hover:bg-gray-200/80 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700 rounded-xl border border-gray-200/60 dark:border-slate-700/60 transition-colors shadow-2xs"
              title={`Поиск (${isMac ? '⌘K' : 'Ctrl+K'})`}
            >
              <Search className="w-4 h-4 text-gray-400 dark:text-slate-400 shrink-0" />
              <span className="hidden md:inline font-normal text-gray-600 dark:text-slate-300">Поиск...</span>
              <span className="md:hidden font-normal text-gray-600 dark:text-slate-300">Поиск</span>
              <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold text-gray-400 dark:text-slate-500 bg-white dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded font-mono">
                {isMac ? '⌘K' : 'Ctrl+K'}
              </kbd>
            </button>

            <NotificationBell />
            <div className="h-8 w-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-700 font-bold uppercase" title={user?.email}>
              {user?.email?.charAt(0) || 'A'}
            </div>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto p-6 bg-gray-50 dark:bg-gray-900">
          {children || <Outlet />}
        </main>
      </div>

      {/* Global Search Palette */}
      <AdminSearchPalette isOpen={isSearchOpen} onClose={() => setIsSearchOpen(false)} />
    </div>
  );
}

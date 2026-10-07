import React, { useState, useEffect } from 'react';
import { useLocation, Outlet } from 'react-router-dom';
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
  PackageCheck,
  PackageSearch,
  Search,
  Menu,
  MessageSquare,
  Megaphone,
  BarChart3,
} from 'lucide-react';

import { useAdminAuth } from '../contexts/AdminAuthContext';
import { NotificationBell } from './notifications/NotificationBell';
import { AdminSearchPalette, NavGroup, NavItem } from './search/AdminSearchPalette';
import { useAdminGlobalSearchShortcut } from './search/useAdminGlobalSearchShortcut';
import { getAdminSellers } from '@zamk/api-client/src/admin';
import { getModerationProducts } from '../api/adminProducts';
import { getAdminReviews } from '../api/adminReviews';
import { getAdminPickingQueue, getAdminPackingQueue } from '../api/adminPicking';
import {
  getStaffScreenVisibility,
  isScreenRuleVisible,
} from '../config/staffWorkModules';

export function AdminLayout({ children }: { children?: React.ReactNode }) {
  const location = useLocation();
  const { logout, user, staff, hasPermission, hasAnyPermission } = useAdminAuth();
  const [isSearchOpen, setIsSearchOpen] = useState(false);

  // Global shortcut listener for Cmd+K / Ctrl+K with input safety
  useAdminGlobalSearchShortcut(isSearchOpen, setIsSearchOpen);

  const isMac = typeof window !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform || navigator.userAgent);

  const isPermissionVisible = (permission?: string | string[]) => {
    if (!permission) return true;
    if (staff === null) return false;
    return isScreenRuleVisible(permission, hasPermission, hasAnyPermission);
  };

  const isNavItemVisible = (item: NavItem) => {
    if (staff === null) return false;
    const visibility = item.permission ?? getStaffScreenVisibility(item.path);
    return isPermissionVisible(visibility);
  };

  // Moderation pending counts
  const [moderationCounts, setModerationCounts] = useState({
    total: 0,
    sellers: 0,
    products: 0,
    reviews: 0,
  });
  const [pickingCount, setPickingCount] = useState<number>(0);
  const [packingCount, setPackingCount] = useState<number>(0);

  useEffect(() => {
    let isMounted = true;
    const loadCounts = async () => {
      try {
        const canReadSellers = isPermissionVisible(getStaffScreenVisibility('/sellers'));
        const canModerateProducts = isPermissionVisible('products.moderate');
        const canReadReviews = isPermissionVisible('reviews.read');
        const canPick = isPermissionVisible(getStaffScreenVisibility('/fulfillment/picking'));
        const canPack = isPermissionVisible(getStaffScreenVisibility('/fulfillment/packing'));

        const [sellersRes, productsRes, reviewsRes, pickingRes, packingRes] = await Promise.allSettled([
          canReadSellers ? getAdminSellers({ limit: 100 }) : Promise.resolve({ items: [] }),
          canModerateProducts ? getModerationProducts({ status: 'pending_moderation', limit: 1 }) : Promise.resolve({ items: [], totalCount: 0 }),
          canReadReviews ? getAdminReviews() : Promise.resolve([]),
          canPick ? getAdminPickingQueue() : Promise.resolve([]),
          canPack ? getAdminPackingQueue() : Promise.resolve([]),
        ]);

        let pCount = 0;
        if (pickingRes.status === 'fulfilled' && Array.isArray(pickingRes.value)) {
          pCount = pickingRes.value.length;
        }

        let pkCount = 0;
        if (packingRes.status === 'fulfilled' && Array.isArray(packingRes.value)) {
          pkCount = packingRes.value.length;
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
          setPackingCount(pkCount);
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

  const isModerationActive = location.pathname.startsWith('/moderation');

  const moderationSubItems: NavItem[] = isModerationActive ? [
    { name: 'Очередь', path: '/moderation/queue', icon: ShieldAlert, count: moderationCounts.total, permission: ['products.moderate', 'reviews.read', 'sellers.read'] },
    { name: 'Продавцы', path: '/moderation/sellers', icon: Store, count: moderationCounts.sellers, permission: 'sellers.read' },
    { name: 'Товары', path: '/moderation/products', icon: Package, count: moderationCounts.products, permission: 'products.moderate' },
    { name: 'Отзывы', path: '/moderation/reviews', icon: ShieldAlert, count: moderationCounts.reviews, permission: 'reviews.read' },
  ].filter(isNavItemVisible) : [];

  const baseCommerceItems: NavItem[] = [
    { name: 'Главная', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Продавцы', path: '/sellers', icon: Store },
    { name: 'Аукционы', path: '/auctions', icon: Gavel },
    { name: 'Товары', path: '/products', icon: Package },
    { name: 'Модерация', path: '/moderation', icon: ShieldAlert, count: moderationCounts.total },
    ...moderationSubItems,
    { name: 'Категории и бренды', path: '/catalog', icon: BookOpen },
    { name: 'Заказы', path: '/orders', icon: ShoppingCart },
    { name: 'Отправления', path: '/shipments', icon: Truck },
    { name: 'Платежи покупателей', path: '/payments', icon: CreditCard },
    { name: 'Возвраты', path: '/returns', icon: RotateCcw },
    { name: 'Возмещения', path: '/refunds', icon: ReceiptText },
    { name: 'Выплаты продавцам', path: '/payouts', icon: Wallet },
  ].filter(isNavItemVisible);

  const marketingNavItems: NavItem[] = [
    { name: 'Сводка', path: '/marketing', icon: BarChart3, permission: 'marketing.campaigns.read' },
    { name: 'Кампании', path: '/marketing/campaigns', icon: Megaphone, permission: 'marketing.campaigns.read' },
  ].filter(isNavItemVisible);

  const warehouseNavItems: NavItem[] = [
    { name: 'Сборка', path: '/fulfillment/picking', icon: PackageCheck, count: pickingCount },
    { name: 'Упаковка', path: '/fulfillment/packing', icon: Package, count: packingCount },
    { name: 'Отгрузка', path: '/fulfillment/dispatch', icon: Truck },
    { name: 'Приёмка поставок', path: '/supplies/receiving', icon: Truck },
    { name: 'Приёмка возвратов', path: '/returns/receiving', icon: RotateCcw },
    { name: 'Остатки', path: '/inventory', icon: Boxes },
    { name: 'Свободный сканер', path: '/warehouse/free-scan', icon: PackageSearch },
  ].filter(isNavItemVisible);

  const staffNavItems: NavItem[] = [
    { name: 'Сводные отчеты', path: '/reports', icon: FileText },
    { name: 'Доступы и роли', path: '/roles', icon: Shield },
    { name: 'Сотрудники', path: '/staff', icon: Users },
    { name: 'Журнал действий', path: '/audit', icon: ClipboardList },
  ].filter(isNavItemVisible);

  const supportNavItems: NavItem[] = [
    { name: 'Поддержка', path: '/support', icon: MessageSquare, permission: 'support.read' },
  ].filter(isNavItemVisible);

  const navGroups: NavGroup[] = [
    { title: 'Коммерция и сервисы', items: baseCommerceItems },
    { title: 'Маркетинг', items: marketingNavItems },
    { title: 'Поддержка', items: supportNavItems },
    { title: 'СКЛАД', items: warehouseNavItems },
    { title: 'Администрирование', items: staffNavItems },
  ].filter((g) => g.items.length > 0);

  const allNavItems = navGroups.flatMap((g) => g.items);

  const isRouteActive = (itemPath: string) => {
    if (itemPath === '/moderation') {
      return isModerationActive;
    }
    if (itemPath === '/marketing') {
      return location.pathname === '/marketing' || location.pathname === '/marketing/overview';
    }
    if (itemPath === '/marketing/campaigns') {
      return location.pathname === '/marketing/campaigns' || (location.pathname.startsWith('/marketing/') && location.pathname !== '/marketing/overview');
    }
    if (itemPath === '/support') {
      return location.pathname === '/support' || location.pathname.startsWith('/support/');
    }
    if (itemPath === '/returns') {
      if (location.pathname === '/returns/receiving' || location.pathname.startsWith('/returns/receiving/')) {
        return false;
      }
      if (location.pathname.startsWith('/returns/') && location.pathname.endsWith('/receiving')) {
        return false;
      }
      return location.pathname === '/returns' || location.pathname.startsWith('/returns/');
    }
    if (itemPath === '/returns/receiving') {
      if (location.pathname === '/returns/receiving' || location.pathname.startsWith('/returns/receiving/')) {
        return true;
      }
      if (location.pathname.startsWith('/returns/') && location.pathname.endsWith('/receiving')) {
        return true;
      }
      return false;
    }
    if (itemPath === '/shipments') {
      return location.pathname === '/shipments' || location.pathname.startsWith('/shipments/');
    }
    if (itemPath === '/fulfillment/dispatch') {
      return (
        location.pathname === '/fulfillment/dispatch' ||
        location.pathname.startsWith('/fulfillment/dispatch/')
      );
    }
    if (itemPath === '/fulfillment/picking') {
      return location.pathname === '/fulfillment/picking' || location.pathname.startsWith('/fulfillment/picking/');
    }
    if (itemPath === '/fulfillment/packing') {
      return location.pathname === '/fulfillment/packing' || location.pathname.startsWith('/fulfillment/packing/');
    }
    if (itemPath === '/supplies/receiving') {
      return location.pathname === '/supplies/receiving' || location.pathname.startsWith('/supplies/receiving/');
    }
    if (itemPath === '/inventory') {
      return location.pathname === '/inventory' || location.pathname.startsWith('/inventory/');
    }
    if (itemPath === '/warehouse/free-scan') {
      return location.pathname === '/warehouse/free-scan' || location.pathname.startsWith('/warehouse/free-scan/');
    }
    return location.pathname === itemPath || (location.pathname.startsWith(itemPath + '/') && itemPath !== '/');
  };

  const currentPageTitle = isModerationActive
    ? 'Модерация'
    : (allNavItems.find((item) => isRouteActive(item.path))?.name || 'Панель администратора');

  return (
    <div data-testid="admin-layout" className="flex h-screen bg-gray-50 flex-col overflow-hidden">
      {/* Compact Top Header Shell */}
      <header className="bg-white border-b border-gray-200 h-16 flex items-center justify-between px-4 sm:px-6 shrink-0 z-10 relative">
        <div className="flex items-center space-x-3">
          {/* Upper-left Compact Navigation Trigger */}
          <button
            type="button"
            data-testid="admin-nav-trigger"
            onClick={() => setIsSearchOpen(true)}
            className="flex items-center space-x-2 px-3 py-1.5 text-slate-700 hover:text-indigo-600 bg-slate-100 hover:bg-indigo-50 border border-slate-200 rounded-xl transition-colors shrink-0"
            title={`Меню и каталог (${isMac ? '⌘K' : 'Ctrl+K'})`}
          >
            <Menu className="w-4 h-4 text-indigo-600" />
            <span className="text-sm font-semibold">Меню</span>
          </button>

          {/* Current Page Context Breadcrumb */}
          <div className="flex items-center text-sm font-medium text-gray-500">
            <span className="hidden sm:inline font-bold text-gray-900 tracking-wider">ZAMK Admin</span>
            <span className="hidden sm:inline mx-2 text-gray-300">/</span>
            <span className="text-gray-800 font-semibold truncate max-w-[200px] sm:max-w-xs">
              {currentPageTitle}
            </span>
          </div>
        </div>

        <div className="flex items-center space-x-3 sm:space-x-4">
          {/* Global Search Button / Trigger */}
          <button
            type="button"
            data-testid="admin-global-search-trigger"
            onClick={() => setIsSearchOpen(true)}
            className="flex items-center space-x-2 px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700 bg-gray-100/80 hover:bg-gray-200/80 rounded-xl border border-gray-200/60 transition-colors shadow-2xs"
            title={`Поиск (${isMac ? '⌘K' : 'Ctrl+K'})`}
          >
            <Search className="w-4 h-4 text-gray-400 shrink-0" />
            <span className="hidden md:inline font-normal text-gray-600">Поиск...</span>
            <span className="md:hidden font-normal text-gray-600">Поиск</span>
            <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold text-gray-400 bg-white border border-gray-200 rounded font-mono">
              {isMac ? '⌘K' : 'Ctrl+K'}
            </kbd>
          </button>

          <NotificationBell />
          <div
            className="h-8 w-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-700 font-bold uppercase"
            title={user?.email}
          >
            {user?.email?.charAt(0) || 'A'}
          </div>

          <button
            type="button"
            data-testid="admin-logout-trigger"
            onClick={() => logout()}
            title="Выйти"
            className="p-2 text-gray-400 hover:text-rose-500 hover:bg-rose-50 rounded-lg transition-colors"
          >
            <LogOut className="w-5 h-5" />
          </button>
        </div>
      </header>

      {/* Main Content Workspace — Full Horizontal Space */}
      <main className="flex-1 overflow-y-auto p-4 sm:p-6 bg-gray-50 dark:bg-gray-900 w-full">
        {children || <Outlet />}
      </main>

      {/* Unified On-Demand Search & Navigation Catalog Surface */}
      <AdminSearchPalette
        isOpen={isSearchOpen}
        onClose={() => setIsSearchOpen(false)}
        navGroups={navGroups}
        isRouteActive={isRouteActive}
      />
    </div>
  );
}

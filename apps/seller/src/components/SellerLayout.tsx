import React, { useState, useEffect, useRef } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { 
  Package, 
  ShoppingCart, 
  BarChart2, 
  Wallet, 
  Store,
  LogOut,
  Archive,
  RotateCcw,
  MessageSquare,
  AlertTriangle,
  Truck,
  Menu,
  X,
  ChevronDown,
  LayoutDashboard
} from 'lucide-react';
import { cn } from '../lib/utils';
import { NotificationBell } from './notifications/NotificationBell';
import { getSellerMe, getSellerBalance } from '@zamk/api-client/src/seller';

export function formatAvailableBalance(cents: number): string {
  const rubles = Math.abs(cents / 100);
  const formatted = rubles.toLocaleString('ru-RU');
  if (cents < 0) {
    return `−${formatted} ₽`;
  }
  return `${formatted} ₽`;
}

type BalanceState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'success'; availableCents: number };

export function HeaderBalanceWidget() {
  const [state, setState] = useState<BalanceState>({ status: 'loading' });

  useEffect(() => {
    let active = true;
    getSellerBalance()
      .then((data) => {
        if (!active) return;
        if (typeof data?.availableCents === 'number') {
          setState({ status: 'success', availableCents: data.availableCents });
        } else {
          setState({ status: 'error' });
        }
      })
      .catch(() => {
        if (active) {
          setState({ status: 'error' });
        }
      });
    return () => {
      active = false;
    };
  }, []);

  let title = 'Баланс недоступен. Открыть финансы';
  let ariaLabel = 'Баланс недоступен. Открыть финансы';

  if (state.status === 'loading') {
    title = 'Загрузка баланса';
    ariaLabel = 'Загрузка баланса';
  } else if (state.status === 'success') {
    const isNegative = state.availableCents < 0;
    const formatted = formatAvailableBalance(state.availableCents);
    title = isNegative
      ? `Баланс: ${formatted}. Открыть финансы`
      : `Доступно к выплате: ${formatted}. Открыть финансы`;
    ariaLabel = title;
  }

  return (
    <Link
      to="/payouts"
      className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-gray-700 hover:text-gray-900 hover:bg-gray-100 transition-colors group focus:outline-none focus-visible:ring-2 focus-visible:ring-black"
      title={title}
      aria-label={ariaLabel}
    >
      <Wallet className="h-4 w-4 text-gray-500 group-hover:text-gray-700 shrink-0" />
      {state.status === 'loading' && (
        <span
          data-testid="header-balance-skeleton"
          className="inline-block w-14 h-4 bg-gray-200 animate-pulse rounded"
          aria-hidden="true"
        />
      )}
      {state.status === 'error' && (
        <span className="text-sm font-medium text-gray-400">— ₽</span>
      )}
      {state.status === 'success' && (
        <span className="text-sm font-medium text-gray-800 group-hover:text-gray-900 transition-colors whitespace-nowrap">
          {formatAvailableBalance(state.availableCents)}
        </span>
      )}
    </Link>
  );
}

export interface NavItem {
  name: string;
  path: string;
  icon: React.ComponentType<{ className?: string }>;
}

export interface NavDropdownGroup {
  id: 'assortment' | 'sales' | 'data' | 'control';
  label: string;
  matchPaths: string[];
  items: NavItem[];
}

export const NAV_GROUPS: NavDropdownGroup[] = [
  {
    id: 'assortment',
    label: 'Ассортимент',
    matchPaths: ['/products', '/inventory', '/supplies'],
    items: [
      { name: 'Товары', path: '/products', icon: Package },
      { name: 'Остатки', path: '/inventory', icon: Archive },
      { name: 'Поставки', path: '/supplies', icon: Truck },
    ],
  },
  {
    id: 'sales',
    label: 'Продажи',
    matchPaths: ['/orders', '/returns', '/reviews'],
    items: [
      { name: 'Заказы', path: '/orders', icon: ShoppingCart },
      { name: 'Возвраты', path: '/returns', icon: RotateCcw },
      { name: 'Отзывы', path: '/reviews', icon: MessageSquare },
    ],
  },
  {
    id: 'data',
    label: 'Данные',
    matchPaths: ['/payouts', '/analytics'],
    items: [
      { name: 'Финансы', path: '/payouts', icon: Wallet },
      { name: 'Аналитика', path: '/analytics', icon: BarChart2 },
    ],
  },
  {
    id: 'control',
    label: 'Контроль',
    matchPaths: ['/warnings'],
    items: [
      { name: 'Предупреждения', path: '/warnings', icon: AlertTriangle },
    ],
  },
];

export function isPathActive(itemPath: string, currentPath: string): boolean {
  if (itemPath === '/dashboard') {
    return currentPath === '/dashboard' || currentPath === '/';
  }
  return currentPath === itemPath || currentPath.startsWith(itemPath + '/');
}

export function isGroupActive(group: NavDropdownGroup, currentPath: string): boolean {
  return group.matchPaths.some((p) => currentPath === p || currentPath.startsWith(p + '/'));
}

export function SellerLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sellerStatus, setSellerStatus] = useState<string | null>(null);
  const [openMenu, setOpenMenu] = useState<string | null>(null);

  const activeMenuRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    getSellerMe()
      .then((data) => {
        setSellerStatus(data.seller.status);
        if (data.seller.status === 'pending_setup') {
          navigate('/onboarding', { replace: true });
        }
      })
      .catch(console.error);
  }, [navigate]);

  // Close dropdown on route change
  useEffect(() => {
    setOpenMenu(null);
  }, [location.pathname]);

  // Click outside and escape key handling
  useEffect(() => {
    if (!openMenu) return;

    const handleClickOutside = (event: MouseEvent) => {
      if (activeMenuRef.current && !activeMenuRef.current.contains(event.target as Node)) {
        setOpenMenu(null);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpenMenu(null);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [openMenu]);

  const isProfileActive = isPathActive('/settings', location.pathname);

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Mobile drawer */}
      <aside
        data-testid="mobile-drawer"
        className={cn(
          "fixed inset-y-0 left-0 z-50 w-72 bg-white border-r border-gray-200 flex flex-col transition-transform duration-200 md:hidden",
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <Link
            to="/dashboard"
            className="text-lg font-bold text-gray-900"
            onClick={() => setMobileOpen(false)}
          >
            ZAMK Seller
          </Link>
          <button
            type="button"
            onClick={() => setMobileOpen(false)}
            className="p-1 rounded-md text-gray-500 hover:bg-gray-100"
            aria-label="Закрыть меню"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <nav className="flex-1 p-3 space-y-4 overflow-y-auto">
          <div className="space-y-1">
            <Link
              to="/dashboard"
              onClick={() => setMobileOpen(false)}
              className={cn(
                "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
                isPathActive('/dashboard', location.pathname)
                  ? "bg-black text-white"
                  : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
              )}
            >
              <LayoutDashboard className={cn(
                "mr-3 h-5 w-5 flex-shrink-0",
                isPathActive('/dashboard', location.pathname) ? "text-white" : "text-gray-400 group-hover:text-gray-500"
              )} />
              Обзор
            </Link>
          </div>

          {NAV_GROUPS.map((group) => (
            <div key={group.id} className="space-y-1">
              <div className="px-3 pt-1 pb-1 text-[11px] font-semibold uppercase tracking-wider text-gray-400 select-none">
                {group.label}
              </div>
              {group.items.map((item) => {
                const active = isPathActive(item.path, location.pathname);
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    onClick={() => setMobileOpen(false)}
                    className={cn(
                      "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
                      active
                        ? "bg-black text-white"
                        : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
                    )}
                  >
                    <item.icon className={cn(
                      "mr-3 h-5 w-5 flex-shrink-0",
                      active
                        ? "text-white"
                        : "text-gray-400 group-hover:text-gray-500"
                    )} />
                    {item.name}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="p-3 border-t border-gray-200 space-y-1">
          <Link
            to="/settings"
            onClick={() => setMobileOpen(false)}
            className={cn(
              "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
              isProfileActive
                ? "bg-black text-white"
                : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
            )}
          >
            <Store className={cn(
              "mr-3 h-5 w-5 flex-shrink-0",
              isProfileActive
                ? "text-white"
                : "text-gray-400 group-hover:text-gray-500"
            )} />
            Профиль магазина
          </Link>
          <button
            type="button"
            onClick={() => {
              setMobileOpen(false);
              logout();
            }}
            className="w-full flex items-center px-3 py-2 text-sm font-medium text-red-600 rounded-md hover:bg-red-50 transition-colors"
          >
            <LogOut className="mr-3 h-5 w-5 flex-shrink-0 text-red-400" />
            Выйти
          </button>
        </div>
      </aside>

      {/* Desktop Top Navigation Header */}
      <header
        data-testid="desktop-top-nav"
        className="sticky top-0 z-30 bg-white border-b border-gray-200"
      >
        <div className="w-full px-4 sm:px-6 lg:px-8 flex items-center justify-between h-14">
          {/* Left: Mobile hamburger + Wordmark */}
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setMobileOpen(true)}
              className="p-1.5 -ml-1.5 rounded-md text-gray-600 hover:bg-gray-100 md:hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-black"
              aria-label="Открыть меню"
            >
              <Menu className="h-5 w-5" />
            </button>
            <Link
              to="/dashboard"
              className="text-base font-bold tracking-tight text-gray-900 shrink-0 hover:opacity-90 transition-opacity"
            >
              ZAMK Seller
            </Link>
          </div>

          {/* Middle: Desktop navigation */}
          <nav className="hidden md:flex items-center gap-1 lg:gap-1.5" aria-label="Основная навигация">
            {/* Direct Link: Обзор */}
            <Link
              to="/dashboard"
              data-active={isPathActive('/dashboard', location.pathname)}
              className={cn(
                "px-3 py-1.5 rounded-md text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-black",
                isPathActive('/dashboard', location.pathname)
                  ? "bg-gray-100 text-gray-900 font-semibold"
                  : "text-gray-600 hover:text-gray-900 hover:bg-gray-100/70 font-medium"
              )}
            >
              Обзор
            </Link>

            {/* Dropdown Groups */}
            {NAV_GROUPS.map((group) => {
              const isOpen = openMenu === group.id;
              const isActive = isGroupActive(group, location.pathname);
              return (
                <div
                  key={group.id}
                  ref={isOpen ? activeMenuRef : undefined}
                  className="relative"
                >
                  <button
                    type="button"
                    id={`nav-trigger-${group.id}`}
                    aria-expanded={isOpen}
                    aria-haspopup="menu"
                    aria-controls={`nav-menu-${group.id}`}
                    data-active={isActive}
                    onClick={() => setOpenMenu(isOpen ? null : group.id)}
                    className={cn(
                      "flex items-center gap-1 px-3 py-1.5 rounded-md text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-black",
                      isActive
                        ? "bg-gray-100 text-gray-900 font-semibold"
                        : "text-gray-600 hover:text-gray-900 hover:bg-gray-100/70 font-medium"
                    )}
                  >
                    <span>{group.label}</span>
                    <ChevronDown
                      className={cn("h-3.5 w-3.5 text-gray-400 transition-transform duration-150", isOpen && "rotate-180")}
                      aria-hidden="true"
                    />
                  </button>

                  {isOpen && (
                    <div
                      id={`nav-menu-${group.id}`}
                      role="menu"
                      aria-labelledby={`nav-trigger-${group.id}`}
                      className="absolute left-0 top-full mt-1.5 w-52 bg-white rounded-lg border border-gray-200 shadow-lg py-1.5 z-50 focus:outline-none"
                    >
                      {group.items.map((item) => {
                        const active = isPathActive(item.path, location.pathname);
                        return (
                          <Link
                            key={item.path}
                            to={item.path}
                            role="menuitem"
                            onClick={() => setOpenMenu(null)}
                            className={cn(
                              "flex items-center gap-2.5 mx-1 px-2.5 py-1.5 rounded-md text-sm transition-colors group focus:outline-none focus-visible:ring-2 focus-visible:ring-black",
                              active
                                ? "bg-black text-white font-medium"
                                : "text-gray-700 hover:bg-gray-100 hover:text-gray-900 font-medium"
                            )}
                          >
                            <item.icon
                              className={cn("h-4 w-4 shrink-0", active ? "text-white" : "text-gray-400 group-hover:text-gray-600")}
                              aria-hidden="true"
                            />
                            <span>{item.name}</span>
                          </Link>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </nav>

          {/* Right: Balance + NotificationBell + Profile dropdown */}
          <div className="flex items-center gap-2">
            <HeaderBalanceWidget />
            <NotificationBell />

            {/* Profile Menu (Desktop) */}
            <div
              ref={openMenu === 'profile' ? activeMenuRef : undefined}
              className="relative hidden md:block"
            >
              <button
                type="button"
                id="nav-trigger-profile"
                aria-expanded={openMenu === 'profile'}
                aria-haspopup="menu"
                aria-controls="nav-menu-profile"
                data-active={isProfileActive}
                onClick={() => setOpenMenu(openMenu === 'profile' ? null : 'profile')}
                className={cn(
                  "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-black",
                  isProfileActive
                    ? "bg-gray-100 text-gray-900 font-semibold"
                    : "text-gray-600 hover:text-gray-900 hover:bg-gray-100/70 font-medium"
                )}
              >
                <Store className="h-4 w-4 text-gray-500 shrink-0" aria-hidden="true" />
                <span>Профиль</span>
                <ChevronDown
                  className={cn("h-3.5 w-3.5 text-gray-400 transition-transform duration-150", openMenu === 'profile' && "rotate-180")}
                  aria-hidden="true"
                />
              </button>

              {openMenu === 'profile' && (
                <div
                  id="nav-menu-profile"
                  role="menu"
                  aria-labelledby="nav-trigger-profile"
                  className="absolute right-0 top-full mt-1.5 w-52 bg-white rounded-lg border border-gray-200 shadow-lg py-1.5 z-50 focus:outline-none"
                >
                  <Link
                    to="/settings"
                    role="menuitem"
                    onClick={() => setOpenMenu(null)}
                    className={cn(
                      "flex items-center gap-2.5 mx-1 px-2.5 py-1.5 rounded-md text-sm transition-colors group focus:outline-none focus-visible:ring-2 focus-visible:ring-black",
                      isProfileActive
                        ? "bg-black text-white font-medium"
                        : "text-gray-700 hover:bg-gray-100 hover:text-gray-900 font-medium"
                    )}
                  >
                    <Store
                      className={cn("h-4 w-4 shrink-0", isProfileActive ? "text-white" : "text-gray-400 group-hover:text-gray-600")}
                      aria-hidden="true"
                    />
                    <span>Профиль магазина</span>
                  </Link>
                  <div className="my-1 border-t border-gray-100" />
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      setOpenMenu(null);
                      logout();
                    }}
                    className="w-[calc(100%-8px)] flex items-center gap-2.5 mx-1 px-2.5 py-1.5 rounded-md text-sm font-medium text-red-600 hover:bg-red-50 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500"
                  >
                    <LogOut className="h-4 w-4 shrink-0 text-red-400" aria-hidden="true" />
                    <span>Выйти</span>
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 w-full overflow-y-auto">
        {sellerStatus === 'pending' && (
          <div className="bg-yellow-50 p-4 text-sm text-yellow-800 border-b border-yellow-200">
            Магазин на проверке. Заполните профиль магазина. После проверки администратор активирует продавца.
          </div>
        )}

        {sellerStatus === 'blocked' && (
          <div className="bg-red-50 p-4 text-sm text-red-800 border-b border-red-200">
            Магазин заблокирован. Обратитесь к администрации.
          </div>
        )}
        {sellerStatus === 'archived' && (
          <div className="bg-gray-50 p-4 text-sm text-gray-800 border-b border-gray-200">
            Магазин архивирован.
          </div>
        )}
        {children}
      </main>
    </div>
  );
}

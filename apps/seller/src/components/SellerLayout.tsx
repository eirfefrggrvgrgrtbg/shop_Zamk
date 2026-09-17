import React, { useState, useEffect } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { 
  LayoutDashboard, 
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
  X
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

interface NavItem {
  name: string;
  path: string;
  icon: React.ComponentType<{ className?: string }>;
}

interface NavGroup {
  title: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    title: 'Ориентация',
    items: [
      { name: 'Панель продавца', path: '/dashboard', icon: LayoutDashboard },
    ],
  },
  {
    title: 'Ассортимент',
    items: [
      { name: 'Товары', path: '/products', icon: Package },
      { name: 'Остатки', path: '/inventory', icon: Archive },
      { name: 'Поставки', path: '/supplies', icon: Truck },
    ],
  },
  {
    title: 'Продажи',
    items: [
      { name: 'Заказы', path: '/orders', icon: ShoppingCart },
      { name: 'Возвраты', path: '/returns', icon: RotateCcw },
      { name: 'Отзывы', path: '/reviews', icon: MessageSquare },
    ],
  },
  {
    title: 'Данные',
    items: [
      { name: 'Финансы', path: '/payouts', icon: Wallet },
      { name: 'Аналитика', path: '/analytics', icon: BarChart2 },
    ],
  },
  {
    title: 'Контроль',
    items: [
      { name: 'Предупреждения', path: '/warnings', icon: AlertTriangle },
    ],
  },
];

export function SellerLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sellerStatus, setSellerStatus] = useState<string | null>(null);

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

  const isLinkActive = (path: string) => {
    return location.pathname === path || (location.pathname.startsWith(path + '/') && path !== '/');
  };

  const NavLinks = ({ onNavigate }: { onNavigate?: () => void }) => (
    <>
      <nav className="flex-1 p-3 space-y-4 overflow-y-auto">
        {navGroups.map((group) => (
          <div key={group.title} className="space-y-1">
            <div className="px-3 pt-1 pb-1 text-[11px] font-semibold uppercase tracking-wider text-gray-400 select-none">
              {group.title}
            </div>
            {group.items.map((item) => {
              const active = isLinkActive(item.path);
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={onNavigate}
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
          onClick={onNavigate}
          className={cn(
            "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
            isLinkActive('/settings')
              ? "bg-black text-white"
              : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
          )}
        >
          <Store className={cn(
            "mr-3 h-5 w-5 flex-shrink-0",
            isLinkActive('/settings')
              ? "text-white"
              : "text-gray-400 group-hover:text-gray-500"
          )} />
          Профиль магазина
        </Link>
        <button
          onClick={() => logout()}
          className="w-full flex items-center px-3 py-2 text-sm font-medium text-red-600 rounded-md hover:bg-red-50 transition-colors"
        >
          <LogOut className="mr-3 h-5 w-5 flex-shrink-0 text-red-400" />
          Выйти
        </button>
      </div>
    </>
  );

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Desktop sidebar */}
      <aside className="w-64 bg-white border-r border-gray-200 flex-col hidden md:flex shrink-0">
        <div className="p-4 border-b border-gray-200 flex items-center justify-between">
          <Link to="/dashboard" className="text-xl font-bold text-gray-900">ZAMK Seller</Link>
        </div>
        <NavLinks />
      </aside>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Mobile drawer */}
      <aside className={cn(
        "fixed inset-y-0 left-0 z-50 w-64 bg-white border-r border-gray-200 flex flex-col transition-transform duration-200 md:hidden",
        mobileOpen ? "translate-x-0" : "-translate-x-full"
      )}>
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <Link to="/dashboard" className="text-xl font-bold text-gray-900" onClick={() => setMobileOpen(false)}>
            ZAMK Seller
          </Link>
          <button onClick={() => setMobileOpen(false)} className="p-1 rounded-md text-gray-500 hover:bg-gray-100">
            <X className="h-5 w-5" />
          </button>
        </div>
        <NavLinks onNavigate={() => setMobileOpen(false)} />
      </aside>

      <div className="flex flex-1 flex-col overflow-hidden">
        <header className="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-3 md:py-3">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setMobileOpen(true)}
              className="p-1 rounded-md text-gray-600 hover:bg-gray-100 md:hidden"
              aria-label="Открыть меню"
            >
              <Menu className="h-5 w-5" />
            </button>
            <span className="text-base font-semibold text-gray-900 md:hidden">ZAMK Seller</span>
          </div>
          <div className="flex items-center gap-2 ml-auto">
            <HeaderBalanceWidget />
            <NotificationBell />
          </div>
        </header>

        <main className="flex-1 overflow-y-auto">
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
    </div>
  );
}

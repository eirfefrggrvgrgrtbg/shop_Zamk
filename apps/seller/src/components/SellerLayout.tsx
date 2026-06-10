import React, { useState, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { 
  LayoutDashboard, 
  Package, 
  Plus, 
  ShoppingCart, 
  BarChart2, 
  Wallet, 
  FileText, 
  Store,
  LogOut,
  Archive,
  RotateCcw,
  MessageSquare,
  AlertTriangle,
  Menu,
  X
} from 'lucide-react';
import { cn } from '../lib/utils';

export function SellerLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sellerStatus, setSellerStatus] = useState<string | null>(null);

  useEffect(() => {
    import('@zamk/api-client/src/seller').then(({ getSellerMe }) => {
      getSellerMe()
        .then((data) => setSellerStatus(data.seller.status))
        .catch(console.error);
    });
  }, []);

  const navItems = [
    { name: 'Панель продавца', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Профиль магазина', path: '/settings', icon: Store },
    { name: 'Товары', path: '/products', icon: Package },
    { name: 'Добавить товар', path: '/products/new', icon: Plus },
    { name: 'Заказы', path: '/orders', icon: ShoppingCart },
    { name: 'Остатки', path: '/inventory', icon: Archive },
    { name: 'Возвраты', path: '/returns', icon: RotateCcw },
    { name: 'Отзывы', path: '/reviews', icon: MessageSquare },
    { name: 'Аналитика', path: '/analytics', icon: BarChart2 },
    { name: 'Выплаты', path: '/payouts', icon: Wallet },
    { name: 'Предупреждения', path: '/warnings', icon: AlertTriangle },
    { name: 'Шаблоны', path: '/templates', icon: FileText },
  ];

  const NavLinks = ({ onNavigate }: { onNavigate?: () => void }) => (
    <>
      <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
        {navItems.map((item) => (
          <Link
            key={item.path}
            to={item.path}
            onClick={onNavigate}
            className={cn(
              "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
              location.pathname === item.path || (location.pathname.startsWith(item.path + '/') && item.path !== '/')
                ? "bg-black text-white"
                : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
            )}
          >
            <item.icon className={cn(
              "mr-3 h-5 w-5 flex-shrink-0",
              location.pathname === item.path || (location.pathname.startsWith(item.path + '/') && item.path !== '/')
                ? "text-white"
                : "text-gray-400 group-hover:text-gray-500"
            )} />
            {item.name}
          </Link>
        ))}
      </nav>
      <div className="p-4 border-t border-gray-200">
        <button onClick={() => logout()} className="w-full flex items-center px-3 py-2 text-sm font-medium text-red-600 rounded-md hover:bg-red-50 transition-colors">
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
        <div className="p-4 border-b border-gray-200">
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
        {/* Mobile top bar */}
        <header className="flex items-center gap-3 border-b border-gray-200 bg-white px-4 py-3 md:hidden">
          <button onClick={() => setMobileOpen(true)} className="p-1 rounded-md text-gray-600 hover:bg-gray-100">
            <Menu className="h-5 w-5" />
          </button>
          <span className="text-base font-semibold text-gray-900">ZAMK Seller</span>
        </header>

        <main className="flex-1 overflow-y-auto">
          {sellerStatus === 'pending' && (
            <div className="bg-yellow-50 p-4 text-sm text-yellow-800 border-b border-yellow-200">
              Магазин на проверке. Заполните профиль магазина. После проверки администратор активирует продавца.
            </div>
          )}
          {sellerStatus === 'active' && (
            <div className="bg-green-50 p-4 text-sm text-green-800 border-b border-green-200">
              Магазин активен. Вы можете добавлять товары и отправлять их на модерацию.
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

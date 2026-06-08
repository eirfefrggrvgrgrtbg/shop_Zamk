import React from 'react';
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
  MessageSquare
} from 'lucide-react';
import { cn } from '../lib/utils';

export function SellerLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout } = useAuth();

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
    { name: 'Шаблоны', path: '/templates', icon: FileText },
  ];

  return (
    <div className="flex h-screen bg-gray-50">
      <aside className="w-64 bg-white border-r border-gray-200 flex flex-col hidden md:flex shrink-0">
        <div className="p-4 border-b border-gray-200">
          <Link to="/dashboard" className="text-xl font-bold text-gray-900">ZAMK Seller</Link>
        </div>
        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {navItems.map((item) => (
            <Link
              key={item.path}
              to={item.path}
              className={cn(
                "flex items-center px-3 py-2 text-sm font-medium rounded-md group transition-colors",
                location.pathname === item.path || location.pathname.startsWith(item.path + '/') && item.path !== '/'
                  ? "bg-black text-white"
                  : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
              )}
            >
              <item.icon className={cn(
                "mr-3 h-5 w-5 flex-shrink-0",
                location.pathname === item.path || location.pathname.startsWith(item.path + '/') && item.path !== '/'
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
      </aside>

      <main className="flex-1 overflow-y-auto">
        {children}
      </main>
    </div>
  );
}

import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { 
  LayoutDashboard, 
  Package, 
  Plus, 
  ShoppingCart, 
  BarChart2, 
  Wallet, 
  FileText, 
  Settings,
  LogOut
} from 'lucide-react';
import { cn } from '../lib/utils';

export function SellerLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();

  const navItems = [
    { name: 'Dashboard', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Products', path: '/products', icon: Package },
    { name: 'Add product', path: '/products/new', icon: Plus },
    { name: 'Orders', path: '/orders', icon: ShoppingCart },
    { name: 'Analytics', path: '/analytics', icon: BarChart2 },
    { name: 'Payouts', path: '/payouts', icon: Wallet },
    { name: 'Templates', path: '/templates', icon: FileText },
    { name: 'Settings', path: '/settings', icon: Settings },
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
          <Link to="/login" className="flex items-center px-3 py-2 text-sm font-medium text-red-600 rounded-md hover:bg-red-50 transition-colors">
            <LogOut className="mr-3 h-5 w-5 flex-shrink-0 text-red-400" />
            Logout
          </Link>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto">
        {children}
      </main>
    </div>
  );
}

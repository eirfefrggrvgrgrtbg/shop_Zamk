import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Users,
  Store,
  Package,
  ShieldAlert,
  ShoppingCart,
  Boxes,
  RotateCcw,
  Wallet,
  ClipboardList,
  Settings,
  LogOut,
  Bell,
  LayoutGrid,
  Tag,
  CreditCard,
  Truck,
  ReceiptText,
  Star
} from 'lucide-react';

import { useAdminAuth } from '../contexts/AdminAuthContext';

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout, user } = useAdminAuth();

  const navItems = [
    { name: 'Dashboard', path: '/dashboard', icon: LayoutDashboard },
    { name: 'Users', path: '/users', icon: Users },
    { name: 'Sellers', path: '/sellers', icon: Store },
    { name: 'Categories', path: '/categories', icon: LayoutGrid },
    { name: 'Brands', path: '/brands', icon: Tag },
    { name: 'Products', path: '/products', icon: Package },
    { name: 'Moderation', path: '/moderation', icon: ShieldAlert },
    { name: 'Orders', path: '/orders', icon: ShoppingCart },
    { name: 'Payments', path: '/payments', icon: CreditCard },
    { name: 'Shipments', path: '/shipments', icon: Truck },
    { name: 'Inventory', path: '/inventory', icon: Boxes },
    { name: 'Returns', path: '/returns', icon: RotateCcw },
    { name: 'Refunds', path: '/refunds', icon: ReceiptText },
    { name: 'Payouts', path: '/payouts', icon: Wallet },
    { name: 'Reviews', path: '/reviews', icon: Star },
    { name: 'Audit Logs', path: '/audit-logs', icon: ClipboardList },
    { name: 'Settings', path: '/settings', icon: Settings },
  ];

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 text-white flex flex-col hidden md:flex shrink-0">
        <div className="p-4 border-b border-slate-800">
          <Link to="/dashboard" className="text-xl font-bold tracking-wider">ZAMK Admin</Link>
        </div>
        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {navItems.map((item) => {
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
        </nav>
        <div className="p-4 border-t border-slate-800">
          <button onClick={() => logout()} className="w-full flex items-center px-3 py-2 text-sm font-medium text-slate-300 rounded-md hover:bg-slate-800 transition-colors">
            <LogOut className="mr-3 h-5 w-5 flex-shrink-0 text-slate-400" />
            Logout
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top Header */}
        <header className="bg-white border-b border-gray-200 h-16 flex items-center justify-between px-6 shrink-0">
          <h1 className="text-xl font-semibold text-gray-800">
            {navItems.find(item => location.pathname.startsWith(item.path))?.name || 'Admin Panel'}
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

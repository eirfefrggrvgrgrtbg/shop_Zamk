import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAdminAuth } from '../contexts/AdminAuthContext';

interface AdminProtectedRouteProps {
  children: React.ReactNode;
  permission?: string | string[];
}

export function AdminProtectedRoute({ children, permission }: AdminProtectedRouteProps) {
  const { user, isAuthenticated, isLoading, hasPermission, hasAnyPermission, staff } = useAdminAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen bg-slate-900 flex justify-center items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white"></div>
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return <Navigate to="/login" replace />;
  }

  if (user.role !== 'admin') {
    return <Navigate to="/login" replace />;
  }

  if (user.mustChangePassword) {
    return <Navigate to="/change-password" replace />;
  }

  if (permission) {
    const allowed = Array.isArray(permission)
      ? hasAnyPermission(permission)
      : hasPermission(permission);

    if (!allowed) {
      return (
        <div className="flex items-center justify-center h-64">
          <div className="text-center">
            <h2 className="text-xl font-semibold text-gray-700 mb-2">Недостаточно прав</h2>
            <p className="text-gray-500">У вас нет доступа к этому разделу.</p>
            {staff === null && (
              <p className="mt-2 text-sm text-gray-400">Этот аккаунт не настроен как сотрудник.</p>
            )}
          </div>
        </div>
      );
    }
  }

  return <>{children}</>;
}

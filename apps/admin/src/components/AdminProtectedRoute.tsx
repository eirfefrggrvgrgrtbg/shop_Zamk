import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAdminAuth } from '../contexts/AdminAuthContext';

export function AdminProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated, isLoading } = useAdminAuth();

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

  return <>{children}</>;
}

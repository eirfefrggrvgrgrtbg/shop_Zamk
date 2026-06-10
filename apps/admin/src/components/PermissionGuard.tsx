import React from 'react';
import { useAdminAuth } from '../contexts/AdminAuthContext';

interface PermissionGuardProps {
  permission: string | string[];
  fallback?: React.ReactNode;
  children: React.ReactNode;
}

export const PermissionGuard: React.FC<PermissionGuardProps> = ({ permission, fallback, children }) => {
  const { hasPermission, hasAnyPermission } = useAdminAuth();
  const allowed = Array.isArray(permission)
    ? hasAnyPermission(permission)
    : hasPermission(permission);

  if (!allowed) {
    return fallback ? <>{fallback}</> : null;
  }
  return <>{children}</>;
};

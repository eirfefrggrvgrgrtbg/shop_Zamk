import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { UserDTO as User } from '@zamk/api-client/src/types';
import type { AdminMeResponse } from '@zamk/api-client/src/types';
import { login as apiLogin, logout as apiLogout, refresh as apiRefresh, changePassword as apiChangePassword } from '@zamk/api-client/src/auth';
import { getAdminMe } from '@zamk/api-client/src/admin';
import { setAccessToken, clearAccessToken } from '@zamk/api-client/src/tokenStore';
import '../lib/api'; // Ensure API URL is set

type StaffInfo = AdminMeResponse['staff'];

interface AdminAuthContextType {
  user: User | null;
  staff: StaffInfo | null;
  permissions: string[];
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  hasPermission: (permission: string) => boolean;
  hasAnyPermission: (permissions: string[]) => boolean;
  isOwner: () => boolean;
  isCoOwner: () => boolean;
  login: (credentials: any) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
  changePassword: (data: any) => Promise<void>;
  reloadStaff: () => Promise<void>;
}

const AdminAuthContext = createContext<AdminAuthContextType | undefined>(undefined);

export function AdminAuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [staff, setStaff] = useState<StaffInfo | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadStaffInfo = useCallback(async () => {
    try {
      const me = await getAdminMe();
      setStaff(me.staff ?? null);
      setPermissions(me.staff?.permissions ?? []);
    } catch {
      // Non-fatal — legacy admin without staff row or backend down
      setStaff(null);
      setPermissions([]);
    }
  }, []);

  const refreshSession = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const res = await apiRefresh();
      if (res.user.role !== 'admin') {
        throw new Error('This account does not have admin access');
      }
      setAccessToken(res.accessToken);
      setUser(res.user);
      await loadStaffInfo();
    } catch (err: any) {
      clearAccessToken();
      setUser(null);
      setStaff(null);
      setPermissions([]);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    refreshSession();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const login = async (credentials: any) => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await apiLogin(credentials);
      if (res.user.role !== 'admin') {
        clearAccessToken();
        throw new Error('This account does not have admin access');
      }
      setAccessToken(res.accessToken);
      setUser(res.user);
      await loadStaffInfo();
    } catch (err: any) {
      clearAccessToken();
      setUser(null);
      setStaff(null);
      setPermissions([]);
      setError(err.message || 'Login failed');
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const logout = async () => {
    setIsLoading(true);
    try {
      await apiLogout();
    } catch (err) {
      console.error('Logout error', err);
    } finally {
      clearAccessToken();
      setUser(null);
      setStaff(null);
      setPermissions([]);
      setIsLoading(false);
    }
  };

  const changePassword = async (data: any) => {
    setIsLoading(true);
    setError(null);
    try {
      await apiChangePassword(data);
      await logout();
    } catch (err: any) {
      setError(err.message || 'Password change failed');
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const reloadStaff = async () => {
    await loadStaffInfo();
  };

  const hasPermission = (permission: string) => permissions.includes(permission);

  const hasAnyPermission = (perms: string[]) => perms.some(p => permissions.includes(p));

  const isOwner = () => staff?.roleCode === 'owner';

  const isCoOwner = () => staff?.roleCode === 'co_owner';

  return (
    <AdminAuthContext.Provider value={{
      user,
      staff,
      permissions,
      isAuthenticated: !!user,
      isLoading,
      error,
      hasPermission,
      hasAnyPermission,
      isOwner,
      isCoOwner,
      login,
      logout,
      refreshSession,
      changePassword,
      reloadStaff,
    }}>
      {children}
    </AdminAuthContext.Provider>
  );
}

export function useAdminAuth() {
  const context = useContext(AdminAuthContext);
  if (context === undefined) {
    throw new Error('useAdminAuth must be used within an AdminAuthProvider');
  }
  return context;
}

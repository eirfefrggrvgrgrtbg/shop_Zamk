import { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { UserDTO as User } from '@zamk/api-client/src/types';
import { login as apiLogin, logout as apiLogout, refresh as apiRefresh, changePassword as apiChangePassword } from '@zamk/api-client/src/auth';
import { setAccessToken, clearAccessToken } from '@zamk/api-client/src/tokenStore';
import '../lib/api'; // Ensure API URL is set

interface AdminAuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  login: (credentials: any) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
  changePassword: (data: any) => Promise<void>;
}

const AdminAuthContext = createContext<AdminAuthContextType | undefined>(undefined);

export function AdminAuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
    } catch (err: any) {
      clearAccessToken();
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    refreshSession();
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
    } catch (err: any) {
      clearAccessToken();
      setUser(null);
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
      setIsLoading(false);
    }
  };

  const changePassword = async (data: any) => {
    setIsLoading(true);
    setError(null);
    try {
      await apiChangePassword(data);
      await logout(); // Backend usually revokes tokens after password change
    } catch (err: any) {
      setError(err.message || 'Password change failed');
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AdminAuthContext.Provider value={{ user, isAuthenticated: !!user, isLoading, error, login, logout, refreshSession, changePassword }}>
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

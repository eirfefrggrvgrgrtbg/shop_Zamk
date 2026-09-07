import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { me, refresh, logout as apiLogout } from '@zamk/api-client/src/auth';
import type { UserDTO } from '@zamk/api-client/src/types';

import '../lib/api'; // Ensure API URL is initialized

interface AuthContextType {
  isAuthenticated: boolean;
  isInitializing: boolean;
  user: UserDTO | null;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isInitializing, setIsInitializing] = useState(true);
  const [user, setUser] = useState<UserDTO | null>(null);

  const refreshUser = useCallback(async () => {
    try {
      const response = await me();
      const userData = response.user;
      if (userData && userData.role === 'seller') {
        setUser(userData);
        setIsAuthenticated(true);
      } else {
        setUser(null);
        setIsAuthenticated(false);
      }
    } catch (e) {
      setUser(null);
      setIsAuthenticated(false);
    }
  }, []);

  const initAuth = useCallback(async () => {
    setIsInitializing(true);
    try {
      const res = await refresh();
      let userData = res?.user;
      if (!userData && res?.accessToken) {
        try {
          const meRes = await me();
          userData = meRes.user;
        } catch {
          userData = null;
        }
      }
      if (userData && userData.role === 'seller') {
        setUser(userData);
        setIsAuthenticated(true);
      } else {
        setUser(null);
        setIsAuthenticated(false);
      }
    } catch (e) {
      setUser(null);
      setIsAuthenticated(false);
    } finally {
      setIsInitializing(false);
    }
  }, []);

  useEffect(() => {
    initAuth();
  }, [initAuth]);

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch (e) {
      console.error('Logout failed', e);
    } finally {
      setUser(null);
      setIsAuthenticated(false);
      window.location.href = '/login';
    }
  }, []);

  return (
    <AuthContext.Provider
      value={{
        isAuthenticated,
        isInitializing,
        user,
        logout,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

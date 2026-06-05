import React, { createContext, useContext, useState, type ReactNode, useEffect } from 'react';

export interface User {
  id: string;
  name: string;
  email: string;
  avatar?: string;
  role?: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isAuthModalOpen: boolean;
  isInitializing: boolean;
  authView: 'login' | 'register' | 'forgot_password' | 'change_password';
  openAuthModal: (view?: 'login' | 'register' | 'forgot_password' | 'change_password') => void;
  closeAuthModal: () => void;
  setAuthView: (view: 'login' | 'register' | 'forgot_password' | 'change_password') => void;
  login: (email: string, pass: string) => Promise<void>;
  register: (name: string, email: string, pass: string) => Promise<void>;
  resetPassword: (email: string) => Promise<void>;
  changePassword: (currentPass: string, newPass: string) => Promise<void>;
  logout: () => Promise<void>;
}

import { login as apiLogin, register as apiRegister, refresh, me, logout as apiLogout, changePassword as apiChangePassword } from '@zamk/api-client/src/auth';

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const [isInitializing, setIsInitializing] = useState(true);
  const [authView, setAuthView] = useState<'login' | 'register' | 'forgot_password' | 'change_password'>('login');

  useEffect(() => {
    async function initAuth() {
      try {
        const res = await refresh();
        if (res.user) {
          if (res.user.mustChangePassword) {
            setAuthView('change_password');
            setIsAuthModalOpen(true);
          }
          setUser({
            id: res.user.id,
            name: res.user.name || res.user.email.split('@')[0],
            email: res.user.email,
            role: res.user.role,
          });
        }
      } catch (err) {
        // Not authenticated, perfectly fine
      } finally {
        setIsInitializing(false);
      }
    }
    initAuth();
  }, []);

  const openAuthModal = (view: 'login' | 'register' | 'forgot_password' | 'change_password' = 'login') => {
    setAuthView(view);
    setIsAuthModalOpen(true);
  };

  const closeAuthModal = () => setIsAuthModalOpen(false);

  const login = async (email: string, pass: string) => {
    try {
      const res = await apiLogin({ email, password: pass });
      
      if (res.user.mustChangePassword) {
        setAuthView('change_password');
        setIsAuthModalOpen(true);
      } else {
        closeAuthModal();
      }

      setUser({
        id: res.user.id,
        name: res.user.name || res.user.email.split('@')[0],
        email: res.user.email,
        role: res.user.role,
      });
    } catch (err: any) {
      throw new Error(err.message || 'Неверный email или пароль');
    }
  };

  const register = async (name: string, email: string, pass: string) => {
    try {
      const res = await apiRegister({ name, email, password: pass });
      
      setUser({
        id: res.user.id,
        name: res.user.name || res.user.email.split('@')[0],
        email: res.user.email,
        role: res.user.role,
      });
      closeAuthModal();
    } catch (err: any) {
      throw new Error(err.message || 'Ошибка регистрации');
    }
  };

  const resetPassword = async (email: string) => {
    return new Promise<void>((resolve, reject) => {
      setTimeout(() => {
        if (!email || !email.includes('@')) {
          reject(new Error('Пожалуйста, введите корректный email'));
          return;
        }
        resolve();
      }, 800);
    });
  };

  const changePassword = async (currentPass: string, newPass: string) => {
    try {
      await apiChangePassword({ currentPassword: currentPass, newPassword: newPass });
      // On success, we generally logout or prompt re-login. The backend might revoke other sessions.
      // If we are currently in mustChangePassword flow, this sets the password. We should logout and force re-login.
      await logout();
      openAuthModal('login');
    } catch (err: any) {
      throw new Error(err.message || 'Ошибка изменения пароля');
    }
  };

  const logout = async () => {
    try {
      await apiLogout();
    } catch (e) {
      console.error(e);
    } finally {
      setUser(null);
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isAuthModalOpen,
        isInitializing,
        authView,
        openAuthModal,
        closeAuthModal,
        setAuthView,
        login,
        register,
        resetPassword,
        changePassword,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within an AuthProvider');
  return context;
};
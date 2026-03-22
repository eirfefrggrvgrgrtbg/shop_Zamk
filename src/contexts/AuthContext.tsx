import React, { createContext, useContext, useState, type ReactNode, useEffect } from 'react';

export interface User {
  id: string;
  name: string;
  email: string;
  avatar?: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isAuthModalOpen: boolean;
  authView: 'login' | 'register' | 'forgot_password';
  openAuthModal: (view?: 'login' | 'register' | 'forgot_password') => void;
  closeAuthModal: () => void;
  setAuthView: (view: 'login' | 'register' | 'forgot_password') => void;
  login: (email: string, pass: string) => Promise<void>;
  register: (name: string, email: string, pass: string) => Promise<void>;
  resetPassword: (email: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const [authView, setAuthView] = useState<'login' | 'register' | 'forgot_password'>('login');

  useEffect(() => {
    // Имитация восстановления сессии, например из localStorage
    const savedUser = localStorage.getItem('zamk_mock_user');
    if (savedUser) {
      try {
        setUser(JSON.parse(savedUser));
      } catch (e) {
        // failed to parse
      }
    }
  }, []);

  const openAuthModal = (view: 'login' | 'register' | 'forgot_password' = 'login') => {
    setAuthView(view);
    setIsAuthModalOpen(true);
  };

  const closeAuthModal = () => setIsAuthModalOpen(false);

  const login = async (email: string, pass: string) => {
    return new Promise<void>((resolve, reject) => {
      setTimeout(() => {
        // Фейковая авторизация
        if (email.includes('@') && pass.length >= 6) {
          const userObj = { id: '1', name: email.split('@')[0], email };
          setUser(userObj);
          localStorage.setItem('zamk_mock_user', JSON.stringify(userObj));
          closeAuthModal();
          resolve();
        } else {
          reject(new Error('Неверный email или пароль'));
        }
      }, 800);
    });
  };

  const register = async (name: string, email: string, pass: string) => {
    return new Promise<void>((resolve, reject) => {
      setTimeout(() => {
        if (!name || !email.includes('@') || pass.length < 6) {
          reject(new Error('Пожалуйста, заполните все поля корректно'));
          return;
        }
        const userObj = { id: '2', name, email };
        setUser(userObj);
        localStorage.setItem('zamk_mock_user', JSON.stringify(userObj));
        closeAuthModal();
        resolve();
      }, 800);
    });
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

  const logout = () => {
    setUser(null);
    localStorage.removeItem('zamk_mock_user');
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isAuthModalOpen,
        authView,
        openAuthModal,
        closeAuthModal,
        setAuthView,
        login,
        register,
        resetPassword,
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
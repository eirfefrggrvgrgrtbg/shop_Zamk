import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../ui/Button';

const RETURN_PATH_KEY = 'zamk_auth_return_path';

export function setAuthReturnPath(path: string) {
  sessionStorage.setItem(RETURN_PATH_KEY, path);
}

export function consumeAuthReturnPath(): string | null {
  const path = sessionStorage.getItem(RETURN_PATH_KEY);
  if (path) sessionStorage.removeItem(RETURN_PATH_KEY);
  return path;
}

interface CustomerProtectedRouteProps {
  children: React.ReactNode;
  title?: string;
  description?: string;
}

export function CustomerProtectedRoute({
  children,
  title = 'Войдите в аккаунт',
  description = 'Для доступа к этому разделу необходимо войти или зарегистрироваться.',
}: CustomerProtectedRouteProps) {
  const { user, isInitializing, openAuthModal } = useAuth();
  const location = useLocation();

  useEffect(() => {
    if (!isInitializing && !user) {
      setAuthReturnPath(location.pathname + location.search);
    }
  }, [isInitializing, user, location.pathname, location.search]);

  if (isInitializing) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center pt-24">
        <Loader2 className="w-8 h-8 animate-spin text-graphite/40" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="relative z-10 min-h-[60vh] pt-32 md:pt-40 pb-20">
        <div className="container mx-auto px-4 sm:px-6 max-w-lg text-center">
          <h1 className="text-2xl font-serif text-graphite dark:text-white mb-3">{title}</h1>
          <p className="text-sm text-ash dark:text-white/60 mb-8">{description}</p>
          <Button onClick={() => openAuthModal('login')}>Войти</Button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}

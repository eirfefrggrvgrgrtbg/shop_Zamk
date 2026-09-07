import { Navigate, useNavigate, useLocation, useSearchParams } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { getSafeReturnPath } from '../lib/authRedirect';
import { SellerLoginForm } from '../components/SellerAuthGate';

export function SellerLogin() {
  const { isAuthenticated, isInitializing } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();

  const redirectTarget = getSafeReturnPath(
    (location.state as any)?.from ?? searchParams.get('returnTo'),
    '/dashboard'
  );

  if (isInitializing) {
    return (
      <div className="min-h-screen bg-ice flex items-center justify-center dark:bg-black">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black dark:border-white"></div>
      </div>
    );
  }

  if (isAuthenticated) {
    return <Navigate to={redirectTarget} replace />;
  }

  return (
    <SellerLoginForm
      onSuccess={() => {
        navigate(redirectTarget, { replace: true });
      }}
    />
  );
}

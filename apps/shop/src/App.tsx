import { BrowserRouter as Router, Routes, Route, useLocation } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ThemeProvider } from './contexts/ThemeContext';
import { CartProvider } from './contexts/CartContext';
import { FavoritesProvider } from './contexts/FavoritesContext';
import { SearchProvider } from './contexts/SearchContext';
import { ToastProvider } from './contexts/ToastContext';
import { ToastContainer } from './components/ui/ToastContainer';
import { Layout } from './components/layout/Layout';
import { AuthModal } from './components/auth/AuthModal';
import { SearchOverlay } from './components/search/SearchOverlay';
import { Home } from './pages/Home';
import { Catalog } from './pages/Catalog';
import { DirectSale } from './pages/DirectSale';
import { ProductDetail } from './pages/ProductDetail';
import { ProductCardPreview } from './pages/ProductCardPreview';
import { SellerDetail } from './pages/SellerDetail';
import { BrandDetail } from './pages/BrandDetail';
import { Cart } from './pages/Cart';
import { Checkout } from './pages/Checkout';
import { DevMockPayment } from './pages/DevMockPayment';
import { Profile } from './pages/Profile';
import { CustomerReviews } from './pages/CustomerReviews';
import { Orders } from './pages/Orders';
import { Settings } from './pages/Settings';
import { Favorites } from './pages/Favorites';
import { Brands } from './pages/Brands';
import { NewArrivals } from './pages/NewArrivals';
import { About } from './pages/About';
import { Collections } from './pages/Collections';
import { Returns } from './pages/Returns';
import { ReturnDetail } from './pages/ReturnDetail';
import { Delivery } from './pages/Delivery';
import { Help } from './pages/Help';
import { Contacts } from './pages/Contacts';
import { Privacy } from './pages/Privacy';
import { AuctionPage } from './pages/Auction';
import { AuctionLotDetail } from './pages/AuctionLotDetail';
import { AuctionWins } from './pages/AuctionWins';
import { useEffect, useRef } from 'react';
import {
  startBehaviorTracking,
  stopBehaviorTracking,
  trackPageView,
  trackSessionStarted,
  parseAttributionMetadata,
  getNavigationAttributionMetadata,
  getOrRenewSession,
  getOrCreateVisitorId,
  isSessionStartedEmitted,
  markSessionStartedEmitted,
} from './lib/behavior';
import type { AttributionMetadata } from '@zamk/api-client/src/behavior';

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => { window.scrollTo(0, 0); }, [pathname]);
  return null;
}

function BehaviorRouterTracker() {
  const location = useLocation();
  const lastTrackedKeyRef = useRef<string | null>(null);

  useEffect(() => {
    // 1. StrictMode duplicate protection:
    // If navigation location key hasn't changed since last tracked navigation in this mount lifecycle, skip
    const currentKey = `${location.pathname}?${location.search}#${location.key}`;
    if (lastTrackedKeyRef.current === currentKey) {
      return;
    }
    lastTrackedKeyRef.current = currentKey;

    const visitorId = getOrCreateVisitorId();
    const { sessionId, isNew } = getOrRenewSession(visitorId);

    // 2. Initial / new session start:
    // Guarantees session_started is emitted exactly once per session
    if (isNew || !isSessionStartedEmitted(sessionId)) {
      markSessionStartedEmitted(sessionId);
      const meta = parseAttributionMetadata();
      trackSessionStarted(meta);
    }

    // 3. Check for new non-direct UTM touch on an existing active session
    // If not a brand new session, check if this navigation has new UTM parameters
    let pageViewMeta: AttributionMetadata | undefined;
    if (!isNew) {
      const navMeta = getNavigationAttributionMetadata(location.search);
      if (navMeta && (navMeta.utm_source || navMeta.source)) {
        pageViewMeta = navMeta;
      }
    }

    // 4. Track page view with normalized route (query-free)
    trackPageView(location.pathname, pageViewMeta ? { metadata: pageViewMeta } : undefined);
  }, [location.pathname, location.search, location.key]);

  return null;
}

function App() {
  useEffect(() => {
    startBehaviorTracking();
    return () => {
      stopBehaviorTracking();
    };
  }, []);

  return (
    <ThemeProvider>
      <AuthProvider>
        <SearchProvider>
        <ToastProvider>
          <CartProvider>
            <FavoritesProvider>
              <Router>
                <ScrollToTop />
                <BehaviorRouterTracker />
                <Layout>
                  <Routes>
                    <Route path="/" element={<Home />} />
                    <Route path="/catalog" element={<Catalog />} />
                    <Route path="/zamk" element={<DirectSale />} />
                    <Route path="/product/:id" element={<ProductDetail />} />
                    <Route path="/preview/products/:token" element={<ProductDetail />} />
                    <Route path="/preview/products/:token/card" element={<ProductCardPreview />} />
                    <Route path="/auction" element={<AuctionPage />} />
                    <Route path="/auction/lots/:id" element={<AuctionLotDetail />} />
                    <Route path="/auction/wins" element={<AuctionWins />} />
                    <Route path="/seller/:slugOrId" element={<SellerDetail />} />
                    <Route path="/brand/:id" element={<BrandDetail />} />
                    <Route path="/cart" element={<Cart />} />
                    <Route path="/checkout" element={<Checkout />} />
                    <Route path="/dev/payments/mock/:paymentId" element={<DevMockPayment />} />
                    <Route path="/favorites" element={<Favorites />} />
                    <Route path="/account" element={<Profile />} />
                    <Route path="/profile" element={<Profile />} />
                    <Route path="/reviews" element={<CustomerReviews />} />
                    <Route path="/settings" element={<Settings />} />
                    <Route path="/orders" element={<Orders />} />
                    <Route path="/brands" element={<Brands />} />
                    <Route path="/new" element={<NewArrivals />} />
                    <Route path="/about" element={<About />} />
                    <Route path="/collections" element={<Collections />} />
                    <Route path="/returns" element={<Returns />} />
                    <Route path="/returns/:returnId" element={<ReturnDetail />} />
                    <Route path="/profile/returns" element={<Returns />} />
                    <Route path="/profile/returns/:returnId" element={<ReturnDetail />} />
                    <Route path="/delivery" element={<Delivery />} />
                    <Route path="/help" element={<Help />} />
                    <Route path="/contacts" element={<Contacts />} />
                    <Route path="/privacy" element={<Privacy />} />
                    <Route path="*" element={<Home />} />
                  </Routes>
                </Layout>
                <AuthModal />
                <SearchOverlay />
              </Router>
            </FavoritesProvider>
          </CartProvider>
        </ToastProvider>
      </SearchProvider>
      </AuthProvider>
    </ThemeProvider>
  );
}

export default App;

import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
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
import { ProductDetail } from './pages/ProductDetail';
import { SellerDetail } from './pages/SellerDetail';
import { BrandDetail } from './pages/BrandDetail';
import { Cart } from './pages/Cart';
import { Checkout } from './pages/Checkout';
import { Profile } from './pages/Profile';
import { Orders } from './pages/Orders';
import { Settings } from './pages/Settings';
import { Favorites } from './pages/Favorites';
import { Brands } from './pages/Brands';
import { NewArrivals } from './pages/NewArrivals';
import { About } from './pages/About';
import { Collections } from './pages/Collections';
import { Returns } from './pages/Returns';
import { Delivery } from './pages/Delivery';
import { Help } from './pages/Help';
import { Contacts } from './pages/Contacts';
import { Privacy } from './pages/Privacy';
import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => { window.scrollTo(0, 0); }, [pathname]);
  return null;
}

function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <SearchProvider>
        <ToastProvider>
          <CartProvider>
            <FavoritesProvider>
              <Router>
                <ScrollToTop />
                <Layout>
                  <Routes>
                    <Route path="/" element={<Home />} />
                    <Route path="/catalog" element={<Catalog />} />
                    <Route path="/product/:id" element={<ProductDetail />} />
                    <Route path="/seller/:slug" element={<SellerDetail />} />
                    <Route path="/brand/:id" element={<BrandDetail />} />
                    <Route path="/cart" element={<Cart />} />
                    <Route path="/checkout" element={<Checkout />} />
                    <Route path="/favorites" element={<Favorites />} />
                    <Route path="/profile" element={<Profile />} />
                    <Route path="/settings" element={<Settings />} />
                    <Route path="/orders" element={<Orders />} />
                    <Route path="/brands" element={<Brands />} />
                    <Route path="/new" element={<NewArrivals />} />
                    <Route path="/about" element={<About />} />
                    <Route path="/collections" element={<Collections />} />
                    <Route path="/returns" element={<Returns />} />
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

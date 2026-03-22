import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { CartProvider } from './contexts/CartContext';
import { FavoritesProvider } from './contexts/FavoritesContext';
import { ToastProvider } from './contexts/ToastContext';
import { Layout } from './components/layout/Layout';
import { AuthModal } from './components/auth/AuthModal';
import { Home } from './pages/Home';
import { Catalog } from './pages/Catalog';
import { ProductDetail } from './pages/ProductDetail';
import { Cart } from './pages/Cart';
import { Checkout } from './pages/Checkout';
import { Profile } from './pages/Profile';
import { Favorites } from './pages/Favorites';
import { Brands } from './pages/Brands';
import { BrandDetail } from './pages/BrandDetail';
import { NewArrivals } from './pages/NewArrivals';
import { About } from './pages/About';
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
    <AuthProvider>
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
                  <Route path="/cart" element={<Cart />} />
                  <Route path="/checkout" element={<Checkout />} />
                  <Route path="/favorites" element={<Favorites />} />
                  <Route path="/profile" element={<Profile />} />
                  <Route path="/brands" element={<Brands />} />
                  <Route path="/brand/:id" element={<BrandDetail />} />
                  <Route path="/new" element={<NewArrivals />} />
                  <Route path="/about" element={<About />} />
                  <Route path="/delivery" element={<Delivery />} />
                  <Route path="/help" element={<Help />} />
                  <Route path="/contacts" element={<Contacts />} />
                  <Route path="/privacy" element={<Privacy />} />
                  <Route path="*" element={<Home />} />
                </Routes>
              </Layout>
              <AuthModal />
            </Router>
          </FavoritesProvider>
        </CartProvider>
      </ToastProvider>
    </AuthProvider>
  );
}

export default App;

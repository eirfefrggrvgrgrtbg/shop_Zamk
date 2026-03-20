import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { Home } from './pages/Home';
import { Catalog } from './pages/Catalog';
import { ProductDetail } from './pages/ProductDetail';
import { Profile } from './pages/Profile';
import { Cart } from './pages/Cart';

// Small wrapper adjusting Navbar Links for Router (simulated in demo)
// Normally Navbar would use React Router Links natively.
// Here we inject the Router at the very top.

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/shop" element={<Catalog />} />
          <Route path="/product/:id" element={<ProductDetail />} />
          <Route path="/profile" element={<Profile />} />
          <Route path="/cart" element={<Cart />} />
          {/* Catch all to go Home for demo purposes */}
          <Route path="*" element={<Home />} />
        </Routes>
      </Layout>
    </Router>
  );
}

export default App;

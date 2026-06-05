import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { SellerLayout } from './components/SellerLayout';
import { SellerLogin } from './pages/SellerLogin';
import { SellerDashboard } from './pages/SellerDashboard';
import { SellerProducts } from './pages/SellerProducts';
import { SellerProductNew } from './pages/SellerProductNew';
import { SellerProductEdit } from './pages/SellerProductEdit';
import { SellerOrders } from './pages/SellerOrders';
import { SellerInventory } from './pages/SellerInventory';
import { SellerReturns } from './pages/SellerReturns';
import { SellerReviews } from './pages/SellerReviews';
import { SellerAnalytics } from './pages/SellerAnalytics';
import { SellerPayouts } from './pages/SellerPayouts';
import { SellerTemplates } from './pages/SellerTemplates';
import { SellerSettings } from './pages/SellerSettings';
import { AuthProvider } from './contexts/AuthContext';
import { SellerProtectedRoute } from './components/SellerProtectedRoute';

import './index.css';

export default function App() {
  return (
    <AuthProvider>
      <Router>
        <Routes>
          <Route path="/login" element={<SellerLogin />} />
          
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          
          <Route path="/dashboard" element={<SellerProtectedRoute><SellerLayout><SellerDashboard /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/products" element={<SellerProtectedRoute><SellerLayout><SellerProducts /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/products/new" element={<SellerProtectedRoute><SellerLayout><SellerProductNew /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/products/:id/edit" element={<SellerProtectedRoute><SellerLayout><SellerProductEdit /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/orders" element={<SellerProtectedRoute><SellerLayout><SellerOrders /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/inventory" element={<SellerProtectedRoute><SellerLayout><SellerInventory /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/returns" element={<SellerProtectedRoute><SellerLayout><SellerReturns /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/reviews" element={<SellerProtectedRoute><SellerLayout><SellerReviews /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/analytics" element={<SellerProtectedRoute><SellerLayout><SellerAnalytics /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/payouts" element={<SellerProtectedRoute><SellerLayout><SellerPayouts /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/templates" element={<SellerProtectedRoute><SellerLayout><SellerTemplates /></SellerLayout></SellerProtectedRoute>} />
          <Route path="/settings" element={<SellerProtectedRoute><SellerLayout><SellerSettings /></SellerLayout></SellerProtectedRoute>} />

          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </Router>
    </AuthProvider>
  );
}

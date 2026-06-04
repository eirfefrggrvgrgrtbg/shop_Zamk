import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { SellerLayout } from './components/SellerLayout';
import { SellerLogin } from './pages/SellerLogin';
import { SellerDashboard } from './pages/SellerDashboard';
import { SellerProducts } from './pages/SellerProducts';
import { SellerProductNew } from './pages/SellerProductNew';
import { SellerProductEdit } from './pages/SellerProductEdit';
import { SellerOrders } from './pages/SellerOrders';
import { SellerAnalytics } from './pages/SellerAnalytics';
import { SellerPayouts } from './pages/SellerPayouts';
import { SellerTemplates } from './pages/SellerTemplates';
import { SellerSettings } from './pages/SellerSettings';

import './index.css';

export default function App() {
  return (
    <Router>
      <Routes>
        <Route path="/login" element={<SellerLogin />} />
        
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        
        <Route path="/dashboard" element={<SellerLayout><SellerDashboard /></SellerLayout>} />
        <Route path="/products" element={<SellerLayout><SellerProducts /></SellerLayout>} />
        <Route path="/products/new" element={<SellerLayout><SellerProductNew /></SellerLayout>} />
        <Route path="/products/:id/edit" element={<SellerLayout><SellerProductEdit /></SellerLayout>} />
        <Route path="/orders" element={<SellerLayout><SellerOrders /></SellerLayout>} />
        <Route path="/analytics" element={<SellerLayout><SellerAnalytics /></SellerLayout>} />
        <Route path="/payouts" element={<SellerLayout><SellerPayouts /></SellerLayout>} />
        <Route path="/templates" element={<SellerLayout><SellerTemplates /></SellerLayout>} />
        <Route path="/settings" element={<SellerLayout><SellerSettings /></SellerLayout>} />

        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Router>
  );
}

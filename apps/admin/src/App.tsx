import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { AdminLayout } from './components/AdminLayout';
import { AdminLogin } from './pages/AdminLogin';
import { AdminDashboard } from './pages/AdminDashboard';
import { AdminUsers } from './pages/AdminUsers';
import { AdminSellers } from './pages/AdminSellers';
import { AdminProducts } from './pages/AdminProducts';
import { AdminModeration } from './pages/AdminModeration';
import { AdminOrders } from './pages/AdminOrders';
import { AdminInventory } from './pages/AdminInventory';
import { AdminReturns } from './pages/AdminReturns';
import { AdminPayouts } from './pages/AdminPayouts';
import { AdminAuditLogs } from './pages/AdminAuditLogs';
import { AdminSettings } from './pages/AdminSettings';

import './styles/index.css';

export default function App() {
  return (
    <Router>
      <Routes>
        <Route path="/login" element={<AdminLogin />} />
        
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        
        <Route path="/dashboard" element={<AdminLayout><AdminDashboard /></AdminLayout>} />
        <Route path="/users" element={<AdminLayout><AdminUsers /></AdminLayout>} />
        <Route path="/sellers" element={<AdminLayout><AdminSellers /></AdminLayout>} />
        <Route path="/products" element={<AdminLayout><AdminProducts /></AdminLayout>} />
        <Route path="/moderation" element={<AdminLayout><AdminModeration /></AdminLayout>} />
        <Route path="/orders" element={<AdminLayout><AdminOrders /></AdminLayout>} />
        <Route path="/inventory" element={<AdminLayout><AdminInventory /></AdminLayout>} />
        <Route path="/returns" element={<AdminLayout><AdminReturns /></AdminLayout>} />
        <Route path="/payouts" element={<AdminLayout><AdminPayouts /></AdminLayout>} />
        <Route path="/audit-logs" element={<AdminLayout><AdminAuditLogs /></AdminLayout>} />
        <Route path="/settings" element={<AdminLayout><AdminSettings /></AdminLayout>} />

        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Router>
  );
}

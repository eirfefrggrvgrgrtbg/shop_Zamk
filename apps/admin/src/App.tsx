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
import { AdminChangePassword } from './pages/AdminChangePassword';
import { AdminAuthProvider } from './contexts/AdminAuthContext';
import { AdminProtectedRoute } from './components/AdminProtectedRoute';

import './styles/index.css';

export default function App() {
  return (
    <AdminAuthProvider>
      <Router>
        <Routes>
          <Route path="/login" element={<AdminLogin />} />
          <Route path="/change-password" element={<AdminChangePassword />} />
          
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          
          <Route path="/dashboard" element={<AdminProtectedRoute><AdminLayout><AdminDashboard /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/users" element={<AdminProtectedRoute><AdminLayout><AdminUsers /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/sellers" element={<AdminProtectedRoute><AdminLayout><AdminSellers /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/products" element={<AdminProtectedRoute><AdminLayout><AdminProducts /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/moderation" element={<AdminProtectedRoute><AdminLayout><AdminModeration /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders" element={<AdminProtectedRoute><AdminLayout><AdminOrders /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/inventory" element={<AdminProtectedRoute><AdminLayout><AdminInventory /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/returns" element={<AdminProtectedRoute><AdminLayout><AdminReturns /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/payouts" element={<AdminProtectedRoute><AdminLayout><AdminPayouts /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/audit-logs" element={<AdminProtectedRoute><AdminLayout><AdminAuditLogs /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/settings" element={<AdminProtectedRoute><AdminLayout><AdminSettings /></AdminLayout></AdminProtectedRoute>} />

          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </Router>
    </AdminAuthProvider>
  );
}

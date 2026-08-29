import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { AdminLayout } from './components/AdminLayout';
import { AdminLogin } from './pages/AdminLogin';
import { AdminDashboard } from './pages/AdminDashboard';
import { AdminUsers } from './pages/AdminUsers';
import { AdminSellers } from './pages/AdminSellers';
import { AdminSellerDetail } from './pages/AdminSellerDetail';
import { AdminAuctionsList } from './pages/AdminAuctionsList';
import { AdminAuctionCreate } from './pages/AdminAuctionCreate';
import { AdminAuctionDetail } from './pages/AdminAuctionDetail';
import { AdminProducts } from './pages/AdminProducts';
import { AdminModeration } from './pages/AdminModeration';
import { AdminProductDetail } from './pages/AdminProductDetail';
import { AdminModerationProductDetail } from './pages/AdminModerationProductDetail';
import { AdminOrders } from './pages/AdminOrders';
import { AdminOrderDetail } from './pages/AdminOrderDetail';
import { AdminFulfillmentsList } from './pages/AdminFulfillmentsList';
import { AdminPickingQueue } from './pages/AdminPickingQueue';
import { AdminPickingDetail } from './pages/AdminPickingDetail';
import { AdminPackingDetail } from './pages/AdminPackingDetail';
import { AdminDispatchDetail } from './pages/AdminDispatchDetail';
import { AdminReceivingScanner } from './pages/AdminReceivingScanner';
import { AdminSupplyReceiving } from './pages/AdminSupplyReceiving';
import { AdminFreeScanner } from './pages/AdminFreeScanner';
import { AdminOrderProblems } from './pages/AdminOrderProblems';
import { AdminPayments } from './pages/AdminPayments';
import { AdminPaymentDetail } from './pages/AdminPaymentDetail';
import { AdminShipments } from './pages/AdminShipments';
import { AdminInventory } from './pages/AdminInventory';
import { AdminReturns } from './pages/AdminReturns';
import { AdminRefunds } from './pages/AdminRefunds';
import { AdminPayouts } from './pages/AdminPayouts';
import { AdminReviews } from './pages/AdminReviews';
import { AdminAuditLogs } from './pages/AdminAuditLogs';
import { AdminReports } from './pages/AdminReports';
import { AdminSettings } from './pages/AdminSettings';
import { AdminCategories } from './pages/AdminCategories';
import { AdminBrands } from './pages/AdminBrands';
import { AdminCatalog } from './pages/AdminCatalog';
import { AdminChangePassword } from './pages/AdminChangePassword';
import { AdminRoles } from './pages/AdminRoles';
import { AdminStaff } from './pages/AdminStaff';
import { AdminAuthProvider } from './contexts/AdminAuthContext';
import { AdminProtectedRoute } from './components/AdminProtectedRoute';

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
          <Route path="/sellers" element={<AdminProtectedRoute permission="sellers.read"><AdminLayout><AdminSellers /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/sellers/:id" element={<AdminProtectedRoute permission="sellers.read"><AdminLayout><AdminSellerDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/auctions" element={<AdminProtectedRoute permission="auctions.read"><AdminLayout><AdminAuctionsList /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/auctions/new" element={<AdminProtectedRoute permission="auctions.create"><AdminLayout><AdminAuctionCreate /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/auctions/:id" element={<AdminProtectedRoute permission="auctions.read"><AdminLayout><AdminAuctionDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/catalog" element={<AdminProtectedRoute permission={['categories.read', 'brands.read']}><AdminLayout><AdminCatalog /></AdminLayout></AdminProtectedRoute>} />
          {/* Legacy routes kept for direct navigation; sidebar uses /catalog */}
          <Route path="/categories" element={<AdminProtectedRoute permission="categories.read"><AdminLayout><AdminCategories /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/brands" element={<AdminProtectedRoute permission="brands.read"><AdminLayout><AdminBrands /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/products" element={<AdminProtectedRoute permission="products.read"><AdminLayout><AdminProducts /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/products/:productId" element={<AdminProtectedRoute permission="products.read"><AdminLayout><AdminProductDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/moderation" element={<AdminProtectedRoute permission="products.moderate"><AdminLayout><AdminModeration /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/moderation/products/:productId" element={<AdminProtectedRoute permission="products.moderate"><AdminLayout><AdminModerationProductDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminOrders /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders/fulfillments" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminFulfillmentsList /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/fulfillment/picking" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminPickingQueue /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/fulfillment/picking/:id" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminPickingDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/fulfillment/packing/:id" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminPackingDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/fulfillment/dispatch/:id" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminDispatchDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders/receiving" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminReceivingScanner /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/supplies/receiving" element={<AdminProtectedRoute permission="inventory.read"><AdminLayout><AdminSupplyReceiving /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/warehouse/free-scan" element={<AdminProtectedRoute permission="inventory.read"><AdminLayout><AdminFreeScanner /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders/problems" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminOrderProblems /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/orders/:orderId" element={<AdminProtectedRoute permission="orders.read"><AdminLayout><AdminOrderDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/payments" element={<AdminProtectedRoute permission="payments.read"><AdminLayout><AdminPayments /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/payments/:paymentId" element={<AdminProtectedRoute permission="payments.read"><AdminLayout><AdminPaymentDetail /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/shipments" element={<AdminProtectedRoute permission="shipments.read"><AdminLayout><AdminShipments /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/inventory" element={<AdminProtectedRoute permission="inventory.read"><AdminLayout><AdminInventory /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/returns" element={<AdminProtectedRoute permission="returns.read"><AdminLayout><AdminReturns /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/refunds" element={<AdminProtectedRoute permission="refunds.read"><AdminLayout><AdminRefunds /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/payouts" element={<AdminProtectedRoute permission="payouts.read"><AdminLayout><AdminPayouts /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/reviews" element={<AdminProtectedRoute permission="reviews.read"><AdminLayout><AdminReviews /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/audit-logs" element={<AdminProtectedRoute permission="audit.read"><AdminLayout><AdminAuditLogs /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/audit" element={<AdminProtectedRoute permission="audit.read"><AdminLayout><AdminAuditLogs /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/reports" element={<AdminProtectedRoute permission="reports.read"><AdminLayout><AdminReports /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/roles" element={<AdminProtectedRoute permission="roles.read"><AdminLayout><AdminRoles /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/staff" element={<AdminProtectedRoute permission="staff.read"><AdminLayout><AdminStaff /></AdminLayout></AdminProtectedRoute>} />
          <Route path="/settings" element={<AdminProtectedRoute><AdminLayout><AdminSettings /></AdminLayout></AdminProtectedRoute>} />

          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </Router>
    </AdminAuthProvider>
  );
}

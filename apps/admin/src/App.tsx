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
import { AdminModerationQueue } from './pages/AdminModerationQueue';
import { AdminModerationSellers } from './pages/AdminModerationSellers';
import { AdminModerationReviews } from './pages/AdminModerationReviews';
import { AdminOrders } from './pages/AdminOrders';
import { AdminOrderDetail } from './pages/AdminOrderDetail';
import { AdminFulfillmentsList } from './pages/AdminFulfillmentsList';
import { AdminPickingQueue } from './pages/AdminPickingQueue';
import { AdminPickingDetail } from './pages/AdminPickingDetail';
import { AdminPackingQueue } from './pages/AdminPackingQueue';
import { AdminPackingDetail } from './pages/AdminPackingDetail';
import { AdminDispatchQueue } from './pages/AdminDispatchQueue';
import { AdminDispatchDetail } from './pages/AdminDispatchDetail';
import { AdminReceivingScanner } from './pages/AdminReceivingScanner';
import { AdminSupplyReceiving } from './pages/AdminSupplyReceiving';
import { AdminFreeScanner } from './pages/AdminFreeScanner';
import { AdminOrderProblems } from './pages/AdminOrderProblems';
import { AdminPayments } from './pages/AdminPayments';
import { AdminPaymentDetail } from './pages/AdminPaymentDetail';
import { AdminShipments } from './pages/AdminShipments';
import { AdminInventory } from './pages/AdminInventory';
import { AdminInventoryReconciliation } from './pages/AdminInventoryReconciliation';
import { AdminReturns } from './pages/AdminReturns';
import { AdminReturnReceiving } from './pages/AdminReturnReceiving';
import { AdminReturnReceivingQueue } from './pages/AdminReturnReceivingQueue';
import { AdminRefunds } from './pages/AdminRefunds';
import { AdminPayouts } from './pages/AdminPayouts';
import { AdminAuditLogs } from './pages/AdminAuditLogs';
import { AdminReports } from './pages/AdminReports';
import { AdminSettings } from './pages/AdminSettings';
import { AdminCategories } from './pages/AdminCategories';
import { AdminBrands } from './pages/AdminBrands';
import { AdminCatalog } from './pages/AdminCatalog';
import { AdminChangePassword } from './pages/AdminChangePassword';
import { AdminRoles } from './pages/AdminRoles';
import { AdminStaff } from './pages/AdminStaff';
import { AdminStaffDetail } from './pages/AdminStaffDetail';
import { AdminAuthProvider } from './contexts/AdminAuthContext';
import { AdminProtectedRoute } from './components/AdminProtectedRoute';
import { getStaffScreenVisibility } from './config/staffWorkModules';

export default function App() {
  return (
    <AdminAuthProvider>
      <Router>
        <Routes>
          <Route path="/login" element={<AdminLogin />} />
          <Route path="/change-password" element={<AdminChangePassword />} />

          {/* Persistent Authenticated Layout Shell */}
          <Route element={<AdminProtectedRoute><AdminLayout /></AdminProtectedRoute>}>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/dashboard')}><AdminDashboard /></AdminProtectedRoute>} />
            <Route path="/users" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/users')}><AdminUsers /></AdminProtectedRoute>} />
            <Route path="/sellers" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/sellers')}><AdminSellers /></AdminProtectedRoute>} />
            <Route path="/sellers/:id" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/sellers')}><AdminSellerDetail /></AdminProtectedRoute>} />
            <Route path="/auctions" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/auctions')}><AdminAuctionsList /></AdminProtectedRoute>} />
            <Route path="/auctions/new" element={<AdminProtectedRoute permission="auctions.create"><AdminAuctionCreate /></AdminProtectedRoute>} />
            <Route path="/auctions/:id" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/auctions')}><AdminAuctionDetail /></AdminProtectedRoute>} />
            <Route path="/catalog" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/catalog')}><AdminCatalog /></AdminProtectedRoute>} />
            <Route path="/categories" element={<AdminProtectedRoute permission="categories.read"><AdminCategories /></AdminProtectedRoute>} />
            <Route path="/brands" element={<AdminProtectedRoute permission="brands.read"><AdminBrands /></AdminProtectedRoute>} />
            <Route path="/products" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/products')}><AdminProducts /></AdminProtectedRoute>} />
            <Route path="/products/:productId" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/products')}><AdminProductDetail /></AdminProtectedRoute>} />

            {/* Moderation Unified Inbox & Sub-routes */}
            <Route path="/moderation" element={<Navigate to="/moderation/queue" replace />} />
            <Route path="/moderation/queue" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/moderation')}><AdminModerationQueue /></AdminProtectedRoute>} />
            <Route path="/moderation/sellers" element={<AdminProtectedRoute permission="sellers.read"><AdminModerationSellers /></AdminProtectedRoute>} />
            <Route path="/moderation/products" element={<AdminProtectedRoute permission="products.moderate"><AdminModeration /></AdminProtectedRoute>} />
            <Route path="/moderation/products/:productId" element={<AdminProtectedRoute permission="products.moderate"><AdminModerationProductDetail /></AdminProtectedRoute>} />
            <Route path="/moderation/reviews" element={<AdminProtectedRoute permission="reviews.read"><AdminModerationReviews /></AdminProtectedRoute>} />

            {/* Legacy Reviews Route Redirect */}
            <Route path="/reviews" element={<Navigate to="/moderation/reviews" replace />} />

            <Route path="/orders" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/orders')}><AdminOrders /></AdminProtectedRoute>} />
            <Route path="/orders/fulfillments" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/orders')}><AdminFulfillmentsList /></AdminProtectedRoute>} />
            <Route path="/fulfillment/picking" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/picking')}><AdminPickingQueue /></AdminProtectedRoute>} />
            <Route path="/fulfillment/picking/:id" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/picking')}><AdminPickingDetail /></AdminProtectedRoute>} />
            <Route path="/fulfillment/packing" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/packing')}><AdminPackingQueue /></AdminProtectedRoute>} />
            <Route path="/fulfillment/packing/:id" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/packing')}><AdminPackingDetail /></AdminProtectedRoute>} />
            <Route path="/fulfillment/dispatch" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/dispatch')}><AdminDispatchQueue /></AdminProtectedRoute>} />
            <Route path="/fulfillment/dispatch/:id" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/fulfillment/dispatch')}><AdminDispatchDetail /></AdminProtectedRoute>} />
            <Route path="/orders/receiving" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/orders/receiving')}><AdminReceivingScanner /></AdminProtectedRoute>} />
            <Route path="/supplies/receiving" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/supplies/receiving')}><AdminSupplyReceiving /></AdminProtectedRoute>} />
            <Route path="/warehouse/free-scan" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/warehouse/free-scan')}><AdminFreeScanner /></AdminProtectedRoute>} />
            <Route path="/orders/problems" element={<AdminProtectedRoute permission="orders.read"><AdminOrderProblems /></AdminProtectedRoute>} />
            <Route path="/orders/:orderId" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/orders')}><AdminOrderDetail /></AdminProtectedRoute>} />
            <Route path="/payments" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/payments')}><AdminPayments /></AdminProtectedRoute>} />
            <Route path="/payments/:paymentId" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/payments')}><AdminPaymentDetail /></AdminProtectedRoute>} />
            <Route path="/shipments" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/shipments')}><AdminShipments /></AdminProtectedRoute>} />
            <Route path="/inventory" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/inventory')}><AdminInventory /></AdminProtectedRoute>} />
            <Route path="/inventory/reconciliation/:id" element={<AdminProtectedRoute permission="inventory.adjust"><AdminInventoryReconciliation /></AdminProtectedRoute>} />
            <Route path="/returns" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/returns')}><AdminReturns /></AdminProtectedRoute>} />
            <Route path="/returns/receiving" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/returns/receiving')}><AdminReturnReceivingQueue /></AdminProtectedRoute>} />
            <Route path="/returns/:id/receiving" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/returns/receiving')}><AdminReturnReceiving /></AdminProtectedRoute>} />
            <Route path="/refunds" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/refunds')}><AdminRefunds /></AdminProtectedRoute>} />
            <Route path="/payouts" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/payouts')}><AdminPayouts /></AdminProtectedRoute>} />
            <Route path="/audit-logs" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/audit')}><AdminAuditLogs /></AdminProtectedRoute>} />
            <Route path="/audit" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/audit')}><AdminAuditLogs /></AdminProtectedRoute>} />
            <Route path="/reports" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/reports')}><AdminReports /></AdminProtectedRoute>} />
            <Route path="/roles" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/roles')}><AdminRoles /></AdminProtectedRoute>} />
            <Route path="/staff" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/staff')}><AdminStaff /></AdminProtectedRoute>} />
            <Route path="/staff/:userId" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/staff')}><AdminStaffDetail /></AdminProtectedRoute>} />
            <Route path="/admin/staff/:userId" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/staff')}><AdminStaffDetail /></AdminProtectedRoute>} />
            <Route path="/settings" element={<AdminProtectedRoute permission={getStaffScreenVisibility('/settings')}><AdminSettings /></AdminProtectedRoute>} />

            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Routes>
      </Router>
    </AdminAuthProvider>
  );
}

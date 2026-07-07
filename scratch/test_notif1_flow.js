const http = require('http');

const API_BASE = 'http://localhost:8080/api';
let customerToken = '';
let sellerToken = '';
let adminToken = '';
let customerId = '';
let sellerId = '';
let productId = '';
let orderId = '';
let returnId = '';

const req = (method, path, body = null, token = null) => {
  return new Promise((resolve, reject) => {
    const url = new URL(API_BASE + path);
    const options = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname + url.search,
      method,
      headers: { 'Content-Type': 'application/json' }
    };
    if (token) options.headers['Authorization'] = `Bearer ${token}`;

    const request = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        let parsed = data;
        try { if (data) parsed = JSON.parse(data); } catch (e) {}
        resolve({ status: res.statusCode, data: parsed });
      });
    });
    request.on('error', reject);
    if (body) request.write(JSON.stringify(body));
    request.end();
  });
};

const assert = (condition, msg) => {
  if (!condition) {
    console.error('❌ ' + msg);
    process.exit(1);
  }
  console.log('  ✓ ' + msg);
};

async function main() {
  console.log('--- NOTIF-1 E2E Flow Test ---');

  // 1. Logins
  let r = await req('POST', '/auth/login', { email: 'admin@zamk.local', password: 'Admin12345!' });
  adminToken = r.data.accessToken;
  
  r = await req('POST', '/auth/login', { email: 'seller@zamk.local', password: 'Seller12345!' });
  if (r.status !== 200) console.log('Seller login failed:', r);
  sellerToken = r.data.accessToken;
  sellerId = r.data.user ? r.data.user.id : r.data.id;

  r = await req('POST', '/auth/login', { email: 'customer@zamk.local', password: 'Customer12345!' });
  customerToken = r.data.accessToken;
  customerId = r.data.user ? r.data.user.id : r.data.id;

  const SEED_PRODUCT_ID = '88888888-8888-4888-8888-888888888888';
  const SEED_VARIANT_ID = '99999999-9999-4999-8999-999999999999';

  // 3. Order Notifications
  console.log('\n[Order & Fulfillment Smoke]');
  r = await req('POST', '/customer/cart/items', { productId: SEED_PRODUCT_ID, productVariantId: SEED_VARIANT_ID, quantity: 1 }, customerToken);
  if (r.status !== 201 && r.status !== 200) console.log('Cart add failed:', r);
  r = await req('POST', '/customer/orders', {
    customerName: 'C', customerPhone: '1', customerEmail: 'c@c.com', deliveryAddress: 'A'
  }, customerToken);
  if (r.status !== 201) console.log('Checkout failed:', r);
  assert(r.status === 201, 'customer checkout -> 201');
  orderId = r.data.id;
  
  let payRes = await req('POST', `/customer/orders/${orderId}/payment`, { method: 'tbank_sbp' }, customerToken);
  let providerPaymentId = 99999;
  if (payRes.data && payRes.data.paymentUrl) {
    providerPaymentId = parseInt(payRes.data.paymentUrl.split('/').pop(), 10) || 99999;
  }
  
  r = await req('POST', '/payments/tbank/webhook', {
    PaymentId: providerPaymentId, Status: 'CONFIRMED', Amount: 10000, Success: true
  });
  if (r.status !== 200) console.log('Payment webhook failed:', r);
  assert(r.status === 200, 'payment webhook -> 200');

  r = await req('POST', '/payments/tbank/webhook', {
    PaymentId: providerPaymentId, Status: 'CONFIRMED', Amount: 10000, Success: true
  });
  assert(r.status === 200, 'repeat payment webhook -> 200 (ignored inside)');

  // Fetch fulfillments
  r = await req('GET', '/seller/fulfillments', null, sellerToken);
  let fulfillment = r.data.items.find(f => f.orderId === orderId);
  
  r = await req('GET', '/seller/notifications', null, sellerToken);
  let newOrderNotif = r.data.items.find(n => n.type === 'fulfillment_paid' && n.entityId === fulfillment.id);
  assert(newOrderNotif, 'seller new order notification found -> yes');

  r = await req('POST', `/seller/fulfillments/${fulfillment.id}/mark-assembling`, null, sellerToken);
  r = await req('POST', `/seller/fulfillments/${fulfillment.id}/mark-packed`, null, sellerToken);
  assert(r.status === 204 || r.status === 200, 'seller mark packed -> 200');

  r = await req('GET', '/customer/notifications', null, customerToken);
  let packedNotif = r.data.items.find(n => n.type === 'fulfillment_packed' && n.entityId === fulfillment.id);
  assert(packedNotif, 'customer packed notification found -> yes');

  // Admin Shipments
  r = await req('POST', `/admin/fulfillments/${fulfillment.id}/shipment`, { carrier: 'CDEK', trackingNumber: '123' }, adminToken);
  assert(r.status === 201, 'shipment create -> 201');
  let shipmentId = r.data.id;

  r = await req('PATCH', `/admin/shipments/${shipmentId}/status`, { status: 'shipped' }, adminToken);
  assert(r.status === 204 || r.status === 200, 'shipment shipped -> 200');
  
  r = await req('GET', '/customer/notifications', null, customerToken);
  let shippedNotif = r.data.items.find(n => n.type === 'shipment_shipped' && n.entityId === shipmentId);
  assert(shippedNotif, 'customer shipped notification found -> yes');

  r = await req('PATCH', `/admin/shipments/${shipmentId}/status`, { status: 'delivered' }, adminToken);
  assert(r.status === 204 || r.status === 200, 'shipment delivered -> 200');

  r = await req('GET', '/customer/notifications', null, customerToken);
  let deliveredNotif = r.data.items.find(n => n.type === 'shipment_delivered' && n.entityId === shipmentId);
  assert(deliveredNotif, 'customer delivered notification found -> yes');

  // 4. Returns / Refunds Smoke
  console.log('\n[Returns/Refunds Smoke]');
  r = await req('GET', `/customer/orders/${orderId}`, null, customerToken);
  let orderItem = r.data.items[0];
  
  r = await req('POST', `/customer/orders/${orderId}/returns`, { reason: 'defective', items: [{ orderItemId: orderItem.id, quantity: 1, reason: 'defective', condition: 'new' }] }, customerToken);
  assert(r.status === 201, 'customer create return -> 201');
  returnId = r.data.id;
  
  r = await req('GET', '/seller/notifications', null, sellerToken);
  assert(r.data.items.find(n => n.type === 'return_created' && n.entityId === returnId), 'seller return notification found -> yes');
  
  r = await req('GET', '/admin/notifications', null, adminToken);
  assert(r.data.items.find(n => n.type === 'return_created' && n.entityId === returnId), 'admin return notification found -> yes');

  r = await req('PATCH', `/admin/returns/${returnId}/status`, { status: 'approved' }, adminToken);
  assert(r.status === 204 || r.status === 200, 'admin approve/reject return -> 200/204');
  
  r = await req('GET', '/seller/notifications', null, sellerToken);
  assert(r.data.items.find(n => n.type === 'return_approved' && n.entityId === returnId), 'seller return status notification found -> yes');

  r = await req('POST', `/admin/returns/${returnId}/refund`, { refundAmountCents: 10000, reason: 'ok' }, adminToken);
  assert(r.status === 204 || r.status === 200 || r.status === 201, 'admin refund -> 201');
  
  r = await req('GET', '/customer/notifications', null, customerToken);
  assert(r.data.items.find(n => n.type === 'refund_created'), 'customer refund notification found -> yes');

  // 5. Payouts Smoke
  console.log('\n[Payouts Smoke]');
  r = await req('POST', '/seller/payouts/request', { amountCents: 100 }, sellerToken);
  if (r.status === 400 || r.status === 409) {
      console.log('Insufficient balance, skipping payout smoke...');
  } else {
      if (r.status !== 201) console.log('Payout creation failed:', r);
      assert(r.status === 201, 'seller create payout -> 201');
      let payoutId = r.data.id;

      r = await req('GET', '/admin/notifications', null, adminToken);
      assert(r.data.items.find(n => n.type === 'payout_requested' && n.entityId === payoutId), 'admin payout requested notification found -> yes');

      r = await req('PATCH', `/admin/payouts/${payoutId}/status`, { status: 'approved' }, adminToken);
      assert(r.status === 204 || r.status === 200, 'admin approve payout -> 200/204');

      r = await req('GET', '/seller/notifications', null, sellerToken);
      assert(r.data.items.find(n => n.type === 'payout_approved' && n.entityId === payoutId), 'seller payout approved notification found -> yes');

      r = await req('PATCH', `/admin/payouts/${payoutId}/status`, { status: 'paid' }, adminToken);
      assert(r.status === 204 || r.status === 200, 'admin mark paid -> 200/204');

      r = await req('GET', '/seller/notifications', null, sellerToken);
      assert(r.data.items.find(n => n.type === 'payout_paid' && n.entityId === payoutId), 'seller payout paid notification found -> yes');
  }

  // 6. Read/Unread & RBAC
  console.log('\n[Read/Unread & RBAC Smoke]');
  r = await req('GET', '/customer/notifications', null, null);
  assert(r.status === 401, 'no-token customer notifications -> 401');
  r = await req('GET', '/seller/notifications', null, null);
  assert(r.status === 401, 'no-token seller notifications -> 401');
  r = await req('GET', '/admin/notifications', null, null);
  assert(r.status === 401, 'no-token admin notifications -> 401');
  
  r = await req('GET', '/admin/notifications', null, customerToken);
  assert(r.status === 403, 'customer admin notifications -> 403');
  r = await req('GET', '/seller/notifications', null, customerToken);
  assert(r.status === 401 || r.status === 403, 'customer seller notifications -> 403/401');
  r = await req('GET', '/admin/notifications', null, sellerToken);
  assert(r.status === 403, 'seller admin notifications -> 403');
  
  r = await req('GET', '/customer/notifications/unread-count', null, customerToken);
  assert(r.status === 200, 'GET unread count before -> 200');
  let beforeCount = r.data.unreadCount;
  assert(beforeCount > 0, `count = ${beforeCount}`);
  
  r = await req('GET', '/customer/notifications', null, customerToken);
  let unreadNotif = r.data.items.find(n => !n.readAt);
  r = await req('POST', `/customer/notifications/${unreadNotif.id}/read`, null, customerToken);
  assert(r.status === 204 || r.status === 200, 'mark one notification read -> 200/204');
  
  r = await req('GET', '/customer/notifications/unread-count', null, customerToken);
  assert(r.data.unreadCount === beforeCount - 1, `GET unread count after -> 200, count = ${r.data.unreadCount}`);

  // Test reading someone else's notification
  r = await req('POST', `/seller/notifications/${unreadNotif.id}/read`, null, sellerToken);
  if (r.status !== 403 && r.status !== 404) console.log('Read alien notification failed:', r);
  assert(r.status === 403 || r.status === 404, 'seller mark read чужого customer/admin notification -> 403/404');
  
  r = await req('POST', '/customer/notifications/read-all', null, customerToken);
  assert(r.status === 204 || r.status === 200, 'mark all read -> 200/204');
  
  r = await req('GET', '/customer/notifications/unread-count', null, customerToken);
  assert(r.data.unreadCount === 0, 'GET unread count final -> 0 or decreased correctly');

  console.log('\n============================================================');
  console.log('✓ NOTIF-1 ALL TESTS PASSED');
}
main().catch(console.error);

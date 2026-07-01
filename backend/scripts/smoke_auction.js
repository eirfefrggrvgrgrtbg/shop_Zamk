/**
 * DEV-ONLY: smoke_auction.js
 * 
 * This script runs a full End-to-End smoke test for the auction lifecycle and winner payment flow.
 * It uses local developer credentials (admin@zamk.local / customer@zamk.local).
 * 
 * Pre-requisites:
 * 1. ZAMK backend must be running on http://localhost:8080.
 * 2. Database container must be named "dev-postgres" (or update the script to match).
 * 3. Seed data must exist (admin, customer, and a Dev Wool Coat product).
 * 
 * Usage:
 * node smoke_auction.js
 */

const http = require('http');

async function request(method, path, body = null, token = null) {
  const url = `http://localhost:8080${path}`;
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  
  const options = {
    method,
    headers,
  };
  
  return new Promise((resolve, reject) => {
    const req = http.request(url, options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode, data: data ? JSON.parse(data) : null });
        } catch(e) {
          resolve({ status: res.statusCode, data });
        }
      });
    });
    req.on('error', reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

async function run() {
  console.log("1. Login Admin...");
  const adminRes = await request('POST', '/api/auth/login', { email: 'admin@zamk.local', password: 'Admin12345!' });
  const adminToken = adminRes.data.accessToken;

  console.log("2. Login Customer A...");
  const custARes = await request('POST', '/api/auth/login', { email: 'customer@zamk.local', password: 'Customer12345!' });
  console.log("Customer login:", custARes.data);
  const custAToken = custARes.data.accessToken;

  console.log("3. Create Auction...");
  const startsAt = new Date(Date.now() - 3600000).toISOString();
  const endsAt = new Date(Date.now() + 3600000).toISOString();
  
  const auctionRes = await request('POST', '/api/admin/auctions', {
    title: "Smoke Test Auction",
    startsAt, endsAt, bidStepCents: 100, paymentDeadlineHours: 24,
    isPublic: true, biddingEnabled: true
  }, adminToken);
  const auctionId = auctionRes.data.id;

  console.log("4. Create Lot...");
  const lotRes = await request('POST', `/api/admin/auctions/${auctionId}/lots`, {
    title: "Smoke Lot",
    startPriceCents: 1000
  }, adminToken);
  const lotId = lotRes.data.id;

  console.log("5. Publish Auction...");
  await request('POST', `/api/admin/auctions/${auctionId}/publish`, null, adminToken);

  console.log("5b. Simulate worker activating auction and lots...");
  require('child_process').execSync(`docker exec dev-postgres psql -U postgres -d zamk -c "UPDATE auction_events SET status='live'; UPDATE auction_lots SET status='active';"`);

  console.log("6. Place Bid Customer A...");
  const bidRes = await request('POST', `/api/customer/auction-lots/${lotId}/bid`, {
    idempotencyKey: 'smoke123'
  }, custAToken);
  console.log("Bid Response:", bidRes.data);

  console.log("7. Finalize Auction...");
  await request('POST', `/api/admin/auctions/${auctionId}/finalize`, null, adminToken);

  console.log("8. Check /auction-wins for Customer A...");
  const winsA = await request('GET', '/api/customer/auction-wins', null, custAToken);
  console.log("Wins A count:", winsA.data.length);
  const smokeLotA = winsA.data.find(l => l.id === lotId);
  console.log("Lot A Status:", smokeLotA?.status);

  console.log("9. Unauth create order...");
  const unauthOrd = await request('POST', `/api/customer/auction-lots/${lotId}/create-order`);
  console.log("Unauth response:", unauthOrd.status);

  console.log("10. Customer A create order...");
  const orderRes = await request('POST', `/api/customer/auction-lots/${lotId}/create-order`, null, custAToken);
  console.log("Create Order Response:", orderRes.data);
  const orderId = orderRes.data.OrderID;

  console.log("11. Idempotency test...");
  const orderRes2 = await request('POST', `/api/customer/auction-lots/${lotId}/create-order`, null, custAToken);
  console.log("Same order id?", orderId === orderRes2.data.OrderID);

  console.log("11.b Initialize Payment...");
  const payInit = await request('POST', `/api/customer/orders/${orderId}/payment`, null, custAToken);
  console.log("Payment Init status:", payInit.status, payInit.data);

  console.log("12. Verify T-Bank Payment Webhook...");
  const tbankPaymentId = parseInt(payInit.data.paymentUrl.split('/').pop());
  // Simulate successful payment webhook
  const webhookBody = {
    Success: true,
    Status: "CONFIRMED",
    OrderId: payInit.data.paymentId || orderId,
    Amount: 1000,
    PaymentId: tbankPaymentId
  };
  const webhookRes = await request('POST', '/api/payments/tbank/webhook', webhookBody);
  console.log("Webhook response status:", webhookRes.status, webhookRes.data);

  console.log("13. Check lot status again...");
  const winsA2 = await request('GET', '/api/customer/auction-wins', null, custAToken);
  const lotA2 = winsA2.data.find((l) => l.id === lotId);
  console.log("Lot A Status after payment:", lotA2?.status);

  console.log("14. Normal catalog checkout test...");
  const cartRes = await request('POST', '/api/customer/cart/items', {
    productId: '88888888-8888-4888-8888-888888888888',
    productVariantId: '99999999-9999-4999-8999-999999999999',
    quantity: 1
  }, custAToken);
  console.log("Cart Add status:", cartRes.status, cartRes.data);

  const normOrder = await request('POST', '/api/customer/orders', {
    customerName: "Smoke Normal",
    customerPhone: "+79991234567",
    customerEmail: "smoke@zamk.local",
    deliveryAddress: "Smoke Address 123"
  }, custAToken);
  console.log("Normal Order status:", normOrder.status, normOrder.data);

  console.log("Done.");
}

run().catch(console.error);

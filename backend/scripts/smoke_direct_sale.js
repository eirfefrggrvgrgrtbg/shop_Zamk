const http = require('http');

async function request(method, path, body = null, token = null) {
  const url = `http://localhost:8080${path}`;
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  
  const options = { method, headers };
  
  return new Promise((resolve, reject) => {
    const req = http.request(url, options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try { resolve({ status: res.statusCode, data: data ? JSON.parse(data) : null }); }
        catch(e) { resolve({ status: res.statusCode, data }); }
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

  console.log("2. Login Customer...");
  const custRes = await request('POST', '/api/auth/login', { email: 'customer@zamk.local', password: 'Customer12345!' });
  const custToken = custRes.data.accessToken;

  console.log("3. Create Auction...");
  const startsAt = new Date(Date.now() - 3600000).toISOString();
  const endsAt = new Date(Date.now() + 3600000).toISOString();
  
  const auctionRes = await request('POST', '/api/admin/auctions', {
    title: "Smoke Direct Sale Auction",
    startsAt, endsAt, bidStepCents: 100, paymentDeadlineHours: 24,
    isPublic: true, biddingEnabled: true
  }, adminToken);
  const auctionId = auctionRes.data.id;

  console.log("4. Create Lot (canMoveToDirectSale=true, directSalePriceCents=15000)...");
  const lotRes = await request('POST', `/api/admin/auctions/${auctionId}/lots`, {
    title: "Smoke Direct Sale Lot",
    startPriceCents: 1000
  }, adminToken);
  const lotId = lotRes.data.id;

  console.log("4b. Update Lot to enable direct sale...");
  await request('PATCH', `/api/admin/auction-lots/${lotId}`, {
    title: "Smoke Direct Sale Lot",
    startPriceCents: 1000,
    canMoveToDirectSale: true,
    directSalePriceCents: 15000
  }, adminToken);

  console.log("5. Publish Auction & activate via DB...");
  await request('POST', `/api/admin/auctions/${auctionId}/publish`, null, adminToken);
  require('child_process').execSync(`docker exec dev-postgres psql -U postgres -d zamk -c "UPDATE auction_events SET status='live'; UPDATE auction_lots SET status='active';"`);

  console.log("6. Finalize Auction (no bids)...");
  await request('POST', `/api/admin/auctions/${auctionId}/finalize`, null, adminToken);

  console.log("7. Move to Direct Sale...");
  const moveRes = await request('POST', `/api/admin/auction-lots/${lotId}/move-to-direct-sale`, null, adminToken);
  console.log("Move response:", moveRes.status);
  
  console.log("8. Check /api/public/direct-sale...");
  const pubRes = await request('GET', '/api/public/direct-sale');
  console.log("pubRes status:", pubRes.status, "data.items:", pubRes.data?.items ? "exists" : "no", "data.products:", pubRes.data?.products ? "exists" : "no");
  let dsList = pubRes.data?.products || pubRes.data?.items || pubRes.data;
  const dsProd = Array.isArray(dsList) ? dsList.find(p => p.title === "Smoke Direct Sale Lot") : null;
  console.log("Direct Sale Product found:", !!dsProd, dsProd?.id);

  if (!dsProd) {
    console.error("Failed to find direct sale product in public API!");
    return;
  }

  console.log("9. Add to Cart...");
  // We need to fetch the variant ID. Direct sale creates one variant.
  const prodRes = await request('GET', `/api/public/products/${dsProd.slug}`);
  const variantId = prodRes.data.variants[0].id;
  
  const cartRes = await request('POST', '/api/customer/cart/items', {
    productId: dsProd.id,
    productVariantId: variantId,
    quantity: 1
  }, custToken);
  console.log("Cart Add status:", cartRes.status);

  console.log("10. Add to Cart AGAIN to test OVERSELL...");
  const cartRes2 = await request('POST', '/api/customer/cart/items', {
    productId: dsProd.id,
    productVariantId: variantId,
    quantity: 1
  }, custToken);
  console.log("Cart Add 2 status (expected 4xx):", cartRes2.status, cartRes2.data?.error);

  console.log("11. Checkout to create order...");
  const orderRes = await request('POST', '/api/customer/orders', {
    customerName: "Smoke Buyer",
    customerPhone: "+79991234567",
    customerEmail: "smoke@zamk.local",
    deliveryAddress: "Smoke Addr"
  }, custToken);
  console.log("Checkout status:", orderRes.status);
  const orderId = orderRes.data?.id;

  if (orderId) {
    console.log("12. Initialize payment...");
    const payRes = await request('POST', `/api/customer/orders/${orderId}/payment`, null, custToken);
    console.log("Payment init:", payRes.status);
  }

  console.log("Done.");
}

run().catch(console.error);

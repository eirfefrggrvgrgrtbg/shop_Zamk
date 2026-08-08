const { chromium } = require('playwright');
const { Client } = require('pg');
const crypto = require('crypto');
const fs = require('fs');

const db = new Client({
  connectionString: process.env.TEST_DATABASE_URL || 'postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable'
});

async function run() {
  await db.connect();
  const runId = Date.now();
  const ns = 'E2E_RET_' + runId;
  
  const proofsDir = 'scratch/playwright/proofs/seller-returns';
  if (!fs.existsSync(proofsDir)) fs.mkdirSync(proofsDir, { recursive: true });

  const sellerEmail = 'seller_ret@zamk.ru';
  const customerEmail = 'cust_ret@zamk.ru';
  
  const sellerUserId = crypto.randomUUID(); 
  const sellerId = crypto.randomUUID();
  const customerId = crypto.randomUUID();
  
  try {
    // 1. Cleanup
    const userEmails = [sellerEmail, customerEmail, 'admin_ret_test_10@zamk.ru'];
    await db.query(`DELETE FROM refunds WHERE order_id = ANY(SELECT id FROM orders WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1)))`, [userEmails]);
    await db.query(`DELETE FROM returns WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1))`, [userEmails]);
    await db.query(`DELETE FROM order_status_history WHERE order_id = ANY(SELECT id FROM orders WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1)))`, [userEmails]);
    await db.query(`DELETE FROM order_items WHERE order_id = ANY(SELECT id FROM orders WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1)))`, [userEmails]);
    await db.query(`DELETE FROM payments WHERE order_id = ANY(SELECT id FROM orders WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1)))`, [userEmails]);
    await db.query(`DELETE FROM orders WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1))`, [userEmails]);
    await db.query(`DELETE FROM seller_ledger_entries WHERE seller_id = $1`, [sellerId]);
    await db.query(`DELETE FROM seller_users WHERE seller_id = $1`, [sellerId]);
    await db.query(`DELETE FROM sellers WHERE id = $1`, [sellerId]);
    await db.query(`DELETE FROM staff_members WHERE user_id = ANY(SELECT id FROM users WHERE email = ANY($1))`, [userEmails]);
    await db.query(`DELETE FROM users WHERE email = ANY($1)`, [userEmails]);

    // 2. Insert Users
    await db.query(`INSERT INTO users (id, email, name, password_hash, role, status, must_change_password, created_at, updated_at) 
      VALUES ($1, $2, 'Seller Ret', '$2a$10$JrIzuyQUmO1mx1FnBmvPXOsHtmVpG0IUydsydp6wVv9u93SB82fmm', 'seller', 'active', false, now(), now()),
             ($3, $4, 'Customer Ret', '$2a$10$JrIzuyQUmO1mx1FnBmvPXOsHtmVpG0IUydsydp6wVv9u93SB82fmm', 'customer', 'active', false, now(), now())`,
      [sellerUserId, sellerEmail, customerId, customerEmail]);

    // 3. Insert Seller
    await db.query(`INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, $2, $3, $4, 'active', now(), now())`, [sellerId, `${ns} Seller`, ns.toLowerCase(), sellerEmail]);
    await db.query(`INSERT INTO seller_users (id, seller_id, user_id, role, created_at) VALUES ($1, $2, $3, 'owner', now())`, 
      [crypto.randomUUID(), sellerId, sellerUserId]);

    // 4. Create Order & Items
    const oId = crypto.randomUUID();
    const oiId = crypto.randomUUID();
    const pId = crypto.randomUUID();
    const pvId = crypto.randomUUID();
    const ofId = crypto.randomUUID();
    const payId = crypto.randomUUID();

    await db.query(`INSERT INTO products (id, seller_id, title, slug, status, price_cents, created_at) VALUES ($1, $2, 'Returnable Product', $3, 'published', 50000, now())`, [pId, sellerId, `ret-prod-${runId}`]);
    await db.query(`INSERT INTO product_variants (id, product_id, sku, price_cents, created_at) VALUES ($1, $2, 'RET-100', 50000, now())`, [pvId, pId]);
    await db.query(`INSERT INTO orders (id, user_id, total_price_cents, customer_name, customer_phone, customer_email, delivery_address, status) VALUES ($1, $2, 50000, 'C', '1', 'c@z', 'A', 'paid')`, [oId, customerId]);
    await db.query(`INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, payment_number, idempotency_key, integration_mode, payment_method, created_at) VALUES ($1, $2, 'tbank', 'mock_prov_id_acc', 'succeeded', 50000, 'RUB', 'PAY-RET-ACC-1', 'mock_idem_acc', 'mock', 'tpay', now())`, [payId, oId]);
    await db.query(`INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents) VALUES ($1, $2, $3, 'paid', 50000, 900, 45500)`, [ofId, oId, sellerId]);
    await db.query(`INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, sku, quantity, price_cents, subtotal_price_cents, order_fulfillment_id) VALUES ($1, $2, $3, $4, $5, 'Returnable Product', 'ret-prod', 'RET-100', 1, 50000, 50000, $6)`, [oiId, oId, pId, pvId, sellerId, ofId]);
    
    // 5. Add ledger entry for sale (450 RUB earned)
    const availAt = new Date(Date.now() + 14 * 86400000).toISOString();
    await db.query(`INSERT INTO seller_ledger_entries (id, seller_id, order_id, order_item_id, type, amount_cents, currency, available_at, created_at) VALUES ($1, $2, $3, $4, 'seller_earning', 45000, 'RUB', $5, now())`, [crypto.randomUUID(), sellerId, oId, oiId, availAt]);
    
    // 6. Create Return (status item_received, ready for refund)
    const rId = crypto.randomUUID();
    const riId = crypto.randomUUID();
    await db.query(`INSERT INTO returns (id, order_id, user_id, status, reason, created_at) VALUES ($1, $2, $3, 'item_received', 'Does not fit', now())`, [rId, oId, customerId]);
    await db.query(`INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition) VALUES ($1, $2, $3, 1, 'Does not fit', 'new')`, [riId, rId, oiId]);

    // UI Verification
    const browser = await chromium.launch({ headless: true });
    
    // ======== ADMIN TRIGGER REFUND ========
    const adminEmail = 'admin@zamk.local';
    
    const adminContext = await browser.newContext();
    const adminReq = adminContext.request;
    const loginRes = await adminReq.post('http://127.0.0.1:8080/api/auth/login', {
      data: { email: adminEmail, password: 'Admin12345!' }
    });
    const loginDataText = await loginRes.text();
    console.log("Login Status:", loginRes.status(), "Response:", loginDataText);
    const loginData = JSON.parse(loginDataText);
    const adminToken = loginData.accessToken;

    console.log("Before Refund:");
    const beforeBal = await db.query(`SELECT type, sum(amount_cents) FROM seller_ledger_entries WHERE seller_id = $1 GROUP BY type`, [sellerId]);
    console.log(beforeBal.rows);

    const checkRet = await db.query(`SELECT id FROM returns WHERE id = $1`, [rId]);
    console.log("Check Return in DB:", checkRet.rows);

    const refRes = await adminReq.post(`http://127.0.0.1:8080/api/admin/returns/${rId}/refund`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: { reason: "Real acceptance refund" }
    });
    if (!refRes.ok()) {
      throw new Error("Refund API failed: " + await refRes.text());
    }
    
    console.log("After Refund:");
    console.log(refRes.status());
    console.log(await refRes.text());
    
    const afterBal = await db.query(`SELECT type, amount_cents, order_item_id, order_id FROM seller_ledger_entries WHERE seller_id = $1`, [sellerId]);
    console.log(afterBal.rows);

    await adminContext.close();

    // ======== SELLER UI ========
    const sellerContext = await browser.newContext({ 
        viewport: { width: 1440, height: 900 },
        deviceScaleFactor: 2
    });
    const sellerPage = await sellerContext.newPage();

    await sellerPage.goto('http://127.0.0.1:3001/login');
    await sellerPage.fill('input[type="email"]', sellerEmail);
    await sellerPage.fill('input[type="password"]', 'Seller12345!');
    await Promise.all([
        sellerPage.waitForURL('http://127.0.0.1:3001/dashboard'),
        sellerPage.click('button[type="submit"]')
    ]);
    
    await sellerPage.waitForTimeout(2000);
    await sellerPage.goto('http://127.0.0.1:3001/returns');
    await sellerPage.waitForTimeout(1000);
    await sellerPage.screenshot({ path: `${proofsDir}/01-returns-list.png`, fullPage: true });

    await sellerPage.goto('http://127.0.0.1:3001/payouts');
    await sellerPage.waitForTimeout(1000);
    await sellerPage.screenshot({ path: `${proofsDir}/02-payouts-deduction.png`, fullPage: true });

    await sellerPage.goto('http://127.0.0.1:3001/orders');
    await sellerPage.waitForTimeout(1000);
    await sellerPage.screenshot({ path: `${proofsDir}/03-orders-list.png`, fullPage: true });

    console.log(`Acceptance tests generated at: ${proofsDir}`);
    await browser.close();

  } catch (err) {
    console.error('Acceptance test failed:', err);
    process.exit(1);
  } finally {
    await db.end();
  }
}

run();

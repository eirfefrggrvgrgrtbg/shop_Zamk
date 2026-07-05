// DEV-ONLY SCRIPT: Used for local smoke testing
const http = require('http');

const API_URL = 'http://localhost:8080/api';
const ADMIN_EMAIL = 'admin@zamk.local';
const ADMIN_PASSWORD = 'Admin12345!';
const CUSTOMER_EMAIL = 'customer@zamk.local';
const CUSTOMER_PASSWORD = 'Customer12345!';

async function fetchJson(method, url, body, token) {
  return new Promise((resolve, reject) => {
    const { URL } = require('url');
    const parsedUrl = new URL(url);
    const options = {
      hostname: parsedUrl.hostname,
      port: parsedUrl.port,
      path: parsedUrl.pathname + parsedUrl.search,
      method: method,
      headers: {
        'Content-Type': 'application/json'
      }
    };

    if (token) {
      options.headers['Authorization'] = `Bearer ${token}`;
    }

    const req = http.request(options, res => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          try {
            resolve(JSON.parse(data));
          } catch (e) {
            resolve(data); // might be empty
          }
        } else {
          reject(new Error(`HTTP ${res.statusCode}: ${data}`));
        }
      });
    });

    req.on('error', e => reject(e));

    if (body) {
      req.write(JSON.stringify(body));
    }
    req.end();
  });
}

async function run() {
  try {
    console.log('1. Admin Login');
    const adminLogin = await fetchJson('POST', `${API_URL}/auth/login`, { email: ADMIN_EMAIL, password: ADMIN_PASSWORD });
    const adminToken = adminLogin.accessToken;

    console.log('2. Customer Login');
    const customerLogin = await fetchJson('POST', `${API_URL}/auth/login`, { email: CUSTOMER_EMAIL, password: CUSTOMER_PASSWORD });
    const customerToken = customerLogin.accessToken;

    console.log('3. Create Auction');
    const auctionRes = await fetchJson('POST', `${API_URL}/admin/auctions`, {
      title: 'Smoke Test Expiry Auction',
      startsAt: new Date(Date.now() - 1000 * 60).toISOString(),
      endsAt: new Date(Date.now() + 1000 * 60 * 60).toISOString(),
      bidStepCents: 1000,
      paymentDeadlineHours: 24, // doesn't matter, we will override DB
      unpaidWinnerPolicy: 'manual_review',
      isPublic: true,
      biddingEnabled: true
    }, adminToken);
    const auctionId = auctionRes.id;

    console.log('4. Create Lot');
    const lotRes = await fetchJson('POST', `${API_URL}/admin/auctions/${auctionId}/lots`, {
      title: 'Expiry Lot',
      startPriceCents: 5000,
    }, adminToken);
    const lotId = lotRes.id;

    console.log('5. Publish Auction');
    await fetchJson('POST', `${API_URL}/admin/auctions/${auctionId}/publish`, {}, adminToken);

    console.log('5b. Simulate worker activating auction and lots...');
    require('child_process').execSync(`docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE auction_events SET status='live'; UPDATE auction_lots SET status='active';"`);

    console.log('6. Place Bid');
    await fetchJson('POST', `${API_URL}/customer/auction-lots/${lotId}/bid`, { amountCents: 5000 }, customerToken);

    console.log('7. Finalize Auction');
    await fetchJson('POST', `${API_URL}/admin/auctions/${auctionId}/finalize`, {}, adminToken);

    console.log('8. Update payment_deadline_at in DB');
    require('child_process').execSync(`docker exec zamk_postgres psql -U zamk -d zamk -c "UPDATE auction_lots SET payment_deadline_at = NOW() - INTERVAL '1 hour' WHERE id = '${lotId}';"`);

    console.log('9. Trigger Expiry Endpoint');
    const expiredRes = await fetchJson('POST', `${API_URL}/admin/auctions/expire-unpaid`, {}, adminToken);
    console.log('Expired response:', expiredRes);

    console.log('10. Check Lot Status');
    const updatedLot = await fetchJson('GET', `${API_URL}/public/auction-lots/${lotId}`);
    console.log(`Lot status is now: ${updatedLot.status}`);

    if (updatedLot.status === 'unpaid_manual_review') {
      console.log('SUCCESS: Lot expired correctly.');
    } else {
      console.log('FAILURE: Lot did not expire.');
      process.exit(1);
    }
  } catch (err) {
    console.error('Smoke test failed:', err);
    process.exit(1);
  }
}

run();

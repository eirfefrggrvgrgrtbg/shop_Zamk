const fs = require('fs');
let file = fs.readFileSync('src/App.tsx', 'utf8');

if (!file.includes('SellerProfile')) {
  file = file.replace("import { ProductDetail } from './pages/ProductDetail';", "import { ProductDetail } from './pages/ProductDetail';\nimport { SellerProfile } from './pages/SellerProfile';");
  file = file.replace('<Route path="/product/:id" element={<ProductDetail />} />', '<Route path="/product/:id" element={<ProductDetail />} />\n              <Route path="/seller/:slug" element={<SellerProfile />} />');
  fs.writeFileSync('src/App.tsx', file, 'utf8');
  console.log('App.tsx updated');
}
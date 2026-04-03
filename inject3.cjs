
const fs = require('fs');
let file = fs.readFileSync('src/lib/mock-data.ts', 'utf8');

if (!file.includes('export function getSellerById')) {
  file = file.replace('export function getBrandById', 
\xport function getSellerById(id: string): Seller | undefined {
  return SELLERS.find((s) => s.id === id);
}
export function getSellerBySlug(slug: string): Seller | undefined {
  return SELLERS.find((s) => s.slug === slug);
}

export function getBrandById\);
  fs.writeFileSync('src/lib/mock-data.ts', file, 'utf8');
  console.log('Added seller helpers');
}


const fs = require('fs');
let file = fs.readFileSync('src/lib/mock-data.ts', 'utf8');

if (!file.includes('export function getSellerById')) {
  file += `
export function getSellerById(id: string): Seller | undefined {
  return SELLERS.find((s) => s.id === id);
}

export function getSellerBySlug(slug: string): Seller | undefined {
  return SELLERS.find((s) => s.slug === slug);
}
`;
  fs.writeFileSync('src/lib/mock-data.ts', file, 'utf8');
}
console.log('Added seller helpers to mock-data.ts');

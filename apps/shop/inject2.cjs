
const fs = require('fs');
let file = fs.readFileSync('src/lib/mock-data.ts', 'utf8');

const sellers = ['s-1', 's-2', 's-3'];
let inProducts = false;
let currentSeller = 0;
const lines = file.split('\n');

for (let i = 0; i < lines.length; i++) {
  if (lines[i].includes('export const PRODUCTS: Product[] = [')) {
    inProducts = true;
  }
  if (inProducts && lines[i].match(/^\s+id: 'p[0-9]+',/)) {
    // If we haven't already inserted sellerId
    if (!lines[i+1].includes('sellerId:')) {
      lines.splice(i + 1, 0, \    sellerId: '\',\);
      currentSeller++;
      i++;
    }
  }
}
fs.writeFileSync('src/lib/mock-data.ts', lines.join('\n'), 'utf8');
console.log('Fixed sellerIds on products');


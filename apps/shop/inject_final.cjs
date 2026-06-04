const fs = require('fs');
let file = fs.readFileSync('src/lib/mock-data.ts', 'utf8');

let count = 0;
const sellers = ['s-1', 's-2', 's-3'];
file = file.replace(/id: 'p([0-9]+)',/g, (match) => {
  const seller = sellers[count % 3];
  count++;
  return match + '\n    sellerId: \'' + seller + '\',';
});

fs.writeFileSync('src/lib/mock-data.ts', file, 'utf8');
console.log('Injected sellerId: ' + count + ' products updated.');

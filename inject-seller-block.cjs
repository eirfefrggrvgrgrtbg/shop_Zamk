const fs = require('fs');
let file = fs.readFileSync('src/pages/ProductDetail.tsx', 'utf8');

if (!file.includes('getSellerById')) {
  file = file.replace(/getBrandById, getProductsByBrand } from '\.\.\/lib\/mock-data';/, 'getBrandById, getProductsByBrand, getSellerById } from \'../lib/mock-data\';');
}

if (!file.includes('const seller = getSellerById')) {
  file = file.replace("const product = getProductById(id || '');", "const product = getProductById(id || '');\n    const seller = product ? getSellerById(product.sellerId || '') : null;");
}

const sellerBlock = `
              {/* Seller Block */}
              {seller && (
                <Link to={\`/seller/\${seller.slug}\`} className="block mt-8 p-4 rounded-[14px] bg-white dark:bg-white/[0.02] border border-border-lighter dark:border-white/10 hover:border-graphite/20 dark:hover:border-white/20 transition-all group shadow-sm hover:shadow-md">
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-full overflow-hidden shrink-0">
                      <img src={seller.avatar} alt={seller.name} className="w-full h-full object-cover" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <h3 className="text-sm font-semibold text-graphite dark:text-white truncate pr-4 group-hover:underline decoration-1 underline-offset-4">{seller.name}</h3>
                        <ChevronRight className="w-4 h-4 text-graphite/40 group-hover:text-graphite dark:text-white/40 dark:group-hover:text-white transition-transform group-hover:translate-x-0.5" />
                      </div>
                      <div className="flex items-center gap-2 text-xs">
                        <div className="flex items-center text-graphite dark:text-white font-medium">
                          <Star className="w-3 h-3 fill-amber-400 text-amber-400 mr-1" />
                          {seller.rating.toFixed(1)}
                        </div>
                        <span className="text-graphite/20 dark:text-white/20">•</span>
                        <span className="text-ash truncate">{seller.reviewCount} отзывов</span>
                      </div>
                    </div>
                  </div>
                </Link>
              )}
`;

if (!file.includes('{/* Seller Block */}')) {
  file = file.replace('{/* Benefits */}', sellerBlock + '\n              <div className="mt-8">\n              {/* Benefits */}');
  // I need to close that extra div around benefits if I added one. Wait, I shouldn't add an extra div. Just:
  file = file.replace('\n              <div className="mt-8">\n              {/* Benefits */}', '\n              {/* Benefits */}');
}

fs.writeFileSync('src/pages/ProductDetail.tsx', file, 'utf8');
console.log('Done modifying ProductDetail');
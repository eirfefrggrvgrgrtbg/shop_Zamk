const fs = require('fs');

// 1. adminOperations.ts
let op = fs.readFileSync('apps/admin/src/api/adminOperations.ts', 'utf8');
op = op.replace('AdminProduct, ', '');
fs.writeFileSync('apps/admin/src/api/adminOperations.ts', op, 'utf8');

// 2. adminProducts.ts interface
let ap = fs.readFileSync('apps/admin/src/api/adminProducts.ts', 'utf8');
ap = ap.replace('export interface AdminProductView {', 'export interface AdminProductView {\n  source?: string;');
fs.writeFileSync('apps/admin/src/api/adminProducts.ts', ap, 'utf8');

// 3. AdminProducts.tsx pagination
let ap_tsx = fs.readFileSync('apps/admin/src/pages/AdminProducts.tsx', 'utf8');
if (!ap_tsx.includes('{totalCount > 0 &&')) {
  // Find the last </div>
  const lastIndex = ap_tsx.lastIndexOf('</div>');
  const paginationCode = `
      {totalCount > 0 && (
        <div className="flex justify-center mt-4 space-x-2">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-3 py-1 border border-gray-300 rounded-md disabled:opacity-50"
          >
            Назад
          </button>
          <span className="px-3 py-1 text-gray-700">Страница {page} из {Math.max(1, Math.ceil(totalCount / 20))}</span>
          <button
            onClick={() => setPage((p) => Math.min(Math.ceil(totalCount / 20), p + 1))}
            disabled={page >= Math.ceil(totalCount / 20)}
            className="px-3 py-1 border border-gray-300 rounded-md disabled:opacity-50"
          >
            Вперед
          </button>
        </div>
      )}
    </div>
`;
  ap_tsx = ap_tsx.substring(0, lastIndex) + paginationCode + ap_tsx.substring(lastIndex + 6);
  fs.writeFileSync('apps/admin/src/pages/AdminProducts.tsx', ap_tsx, 'utf8');
}
console.log('Fixed');

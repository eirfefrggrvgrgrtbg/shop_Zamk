import os
import re

# Fix adminOperations.ts
f1 = 'apps/admin/src/api/adminOperations.ts'
with open(f1, 'r', encoding='utf-8') as f:
    d1 = f.read()

# Replace const products = await getAdminProducts();
d1 = re.sub(
    r'const \{ items: products \} = await getAdminProducts\(\);',
    'const response = await getAdminProducts();\n    const products = response.items || [];',
    d1
)
with open(f1, 'w', encoding='utf-8') as f:
    f.write(d1)

# Fix AdminProducts.tsx
f2 = 'apps/admin/src/pages/AdminProducts.tsx'
with open(f2, 'r', encoding='utf-8') as f:
    d2 = f.read()

# Normalize line endings
d2 = d2.replace('\r\n', '\n')

if 'import { HelpTooltip }' not in d2:
    d2 = d2.replace("import { Package", "import { HelpTooltip } from '../components/HelpTooltip';\nimport { Package")

if 'import { Package, Search' not in d2:
    d2 = d2.replace("import { Package", "import { Package, Search")

if '{totalCount > 20 && (' not in d2:
    d2 = d2.replace('</tbody>\n              </table>\n            </div>\n          </div>\n        </div>\n      )}',
    '''</tbody>
              </table>
            </div>
          </div>
        </div>
      )}
      
      {totalCount > 20 && (
        <div className="flex justify-center mt-4 space-x-2">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="px-3 py-1 border rounded-md disabled:opacity-50"
          >
            Назад
          </button>
          <span className="px-3 py-1 text-gray-700">Страница {page} из {Math.ceil(totalCount / 20)}</span>
          <button
            onClick={() => setPage((p) => Math.min(Math.ceil(totalCount / 20), p + 1))}
            disabled={page === Math.ceil(totalCount / 20)}
            className="px-3 py-1 border rounded-md disabled:opacity-50"
          >
            Вперед
          </button>
        </div>
      )}''')

with open(f2, 'w', encoding='utf-8') as f:
    f.write(d2)

# Fix AdminProductView to have source
f3 = 'apps/admin/src/api/adminProducts.ts'
with open(f3, 'r', encoding='utf-8') as f:
    d3 = f.read()

if 'source?: string;' not in d3:
    d3 = d3.replace('totalStock: number;\n  createdAt:', 'source?: string;\n  totalStock: number;\n  createdAt:')

with open(f3, 'w', encoding='utf-8') as f:
    f.write(d3)

# Fix unused import
f4 = 'packages/api-client/src/admin.ts'
with open(f4, 'r', encoding='utf-8') as f:
    d4 = f.read()

d4 = d4.replace('AdminProduct, ', '')

with open(f4, 'w', encoding='utf-8') as f:
    f.write(d4)

print("Fixed")

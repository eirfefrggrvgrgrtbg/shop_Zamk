import re

# 1. adminOperations.ts
f1 = 'apps/admin/src/api/adminOperations.ts'
with open(f1, 'r', encoding='utf-8') as f:
    d1 = f.read()

# It imports `getAdminProducts` from `@zamk/api-client/src/admin`
# Which now returns PaginatedAdminProductsResponse
# Replace `const products = await getAdminProducts();` or `const response = await getAdminProducts();`
d1 = re.sub(
    r'const response = await getAdminProducts\(\);\s*const products = response\.items \|\| \[\];',
    'const response = await getAdminProducts();\n  const products = response.items || [];',
    d1
)
if 'const products = response.items || [];' not in d1:
    d1 = re.sub(
        r'const products = await getAdminProducts\(\);',
        'const response = await getAdminProducts();\n  const products = response.items || [];',
        d1
    )
    d1 = re.sub(
        r'const \{ items: products \} = await getAdminProducts\(\);',
        'const response = await getAdminProducts();\n  const products = response.items || [];',
        d1
    )

with open(f1, 'w', encoding='utf-8') as f:
    f.write(d1)

# 2. AdminProducts.tsx pagination UI and totalCount use
f2 = 'apps/admin/src/pages/AdminProducts.tsx'
with open(f2, 'r', encoding='utf-8') as f:
    d2 = f.read()

d2 = d2.replace('\r\n', '\n')

# Check if totalCount > 20 is actually in there. If not, add it.
if '{totalCount > 20 && (' not in d2:
    # Find the end of the table
    target = '</tbody>\n              </table>\n            </div>\n          </div>\n        </div>\n      )}'
    repl = '''</tbody>
              </table>
            </div>
          </div>
        </div>
      )}
      
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
      )}'''
    d2 = d2.replace(target, repl)
    # If the target wasn't found, try a looser match
    if '{totalCount > 0 && (' not in d2:
        d2 = re.sub(
            r'</tbody>\s*</table>\s*</div>\s*</div>\s*</div>\s*\)\}',
            repl,
            d2
        )

with open(f2, 'w', encoding='utf-8') as f:
    f.write(d2)

# 3. adminProducts.ts AdminProductView type
f3 = 'apps/admin/src/api/adminProducts.ts'
with open(f3, 'r', encoding='utf-8') as f:
    d3 = f.read()

# Add source to AdminProductView interface
if 'source?: string;' not in d3:
    d3 = re.sub(
        r'(export interface AdminProductView \{[\s\S]*?)priceCents: number;',
        r'\1source?: string;\n  priceCents: number;',
        d3
    )

with open(f3, 'w', encoding='utf-8') as f:
    f.write(d3)

print("Done")

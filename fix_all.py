import os
import re

# 1. Fix adminProducts.ts
f1 = 'apps/admin/src/api/adminProducts.ts'
with open(f1, 'r', encoding='utf-8') as f:
    d1 = f.read()

# Add getAdminProductModerationHistory to imports
d1 = re.sub(
    r"uploadAdminProductImage as apiUploadAdminProductImage,\s*\} from '@zamk/api-client/src/admin';",
    "uploadAdminProductImage as apiUploadAdminProductImage,\n  getAdminProductModerationHistory as apiGetAdminProductModerationHistory,\n} from '@zamk/api-client/src/admin';",
    d1
)

# Export getAdminProductModerationHistory
mod_export = """export const getAdminProductModerationHistory = async (productId: string) => {
  try {
    const data = await apiGetAdminProductModerationHistory(productId);
    return data.items;
  } catch (err) {
    throw err;
  }
};

export const getModerationProducts"""
d1 = re.sub(r"export const getModerationProducts", mod_export, d1, count=1)

with open(f1, 'w', encoding='utf-8') as f:
    f.write(d1)


# 2. Fix AdminProducts.tsx
f2 = 'apps/admin/src/pages/AdminProducts.tsx'
with open(f2, 'r', encoding='utf-8') as f:
    d2 = f.read()

# Fix setProducts
d2 = re.sub(
    r"const data = await getAdminProducts\(\);\s*setProducts\(data\);",
    "const data = await getAdminProducts();\n      setProducts(data.items);\n      setTotalCount(data.totalCount);",
    d2
)

# Add pagination state
d2 = re.sub(
    r"const \[error, setError\] = useState<string \| null>\(null\);",
    "const [error, setError] = useState<string | null>(null);\n  const [page, setPage] = useState(1);\n  const [totalCount, setTotalCount] = useState(0);\n  const [searchQuery, setSearchQuery] = useState('');\n  const [statusFilter, setStatusFilter] = useState('');\n  const [sourceFilter, setSourceFilter] = useState('');",
    d2
)

# Update fetchProducts definition
fetch_repl = """const fetchProducts = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminProducts(page, 20, {
        q: searchQuery,
        status: statusFilter,
        source: sourceFilter,
      });
      setProducts(data.items);
      setTotalCount(data.totalCount);
    } catch (err: unknown) {
      setError(getAdminProductErrorMessage(err, 'Не удалось загрузить товары.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      fetchProducts();
    }, 300);
    return () => clearTimeout(delayDebounceFn);
  }, [page, searchQuery, statusFilter, sourceFilter]);"""

# Replace old fetchProducts
old_fetch = r"const fetchProducts = async \(\) => \{[\s\S]*?useEffect\(\(\) => \{\s*fetchProducts\(\);\s*\}, \[\]\);"
d2 = re.sub(old_fetch, fetch_repl, d2)

# Update UI elements
title_repl = """<div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Каталог товаров <HelpTooltip content="Управление всеми товарами продавцов и товарами платформы (ZAMK)." /></h1>
      </div>

      <div className="flex flex-col sm:flex-row gap-4 mb-4">
        <div className="relative flex-1">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <Search className="h-5 w-5 text-gray-400" />
          </div>
          <input
            type="text"
            className="block w-full pl-10 pr-3 py-2 border border-gray-300 rounded-md leading-5 bg-white placeholder-gray-500 focus:outline-none focus:placeholder-gray-400 focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
            placeholder="Поиск по названию или ID..."
            value={searchQuery}
            onChange={(e) => { setSearchQuery(e.target.value); setPage(1); }}
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="block w-full sm:w-48 pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
        >
          <option value="">Все статусы</option>
          <option value="published">Опубликован</option>
          <option value="approved">Одобрен</option>
          <option value="pending_moderation">На модерации</option>
          <option value="rejected">Отклонен</option>
          <option value="hidden">Скрыт</option>
          <option value="blocked">Заблокирован</option>
        </select>
        <select
          value={sourceFilter}
          onChange={(e) => { setSourceFilter(e.target.value); setPage(1); }}
          className="block w-full sm:w-48 pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
        >
          <option value="">Все источники</option>
          <option value="platform">ZAMK</option>
          <option value="seller">Продавцы</option>
        </select>
      </div>"""

d2 = re.sub(
    r'<div className="sm:flex sm:items-center sm:justify-between">\s*<h1 className="text-2xl font-bold text-gray-900">All Products</h1>\s*</div>',
    title_repl,
    d2
)

# Add pagination bottom UI
pagination_ui = """</tbody>
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
      )}
    </div>"""

d2 = re.sub(
    r'</tbody>\s*</table>\s*</div>\s*</div>\s*</div>\s*\)\}\s*</div>',
    pagination_ui,
    d2
)

# Add HelpTooltip import if not exists
if "HelpTooltip" not in d2:
    d2 = d2.replace("import { Package", "import { HelpTooltip } from '../components/HelpTooltip';\nimport { Package")

with open(f2, 'w', encoding='utf-8') as f:
    f.write(d2)

# 3. Fix adminOperations.ts
f3 = 'apps/admin/src/api/adminOperations.ts'
with open(f3, 'r', encoding='utf-8') as f:
    d3 = f.read()

# In adminOperations.ts, getAdminProducts is used:
# `const products = await getAdminProducts();` -> now it's { items, totalCount }
d3 = re.sub(
    r'const products = await getAdminProducts\(\);',
    'const { items: products } = await getAdminProducts();',
    d3
)
with open(f3, 'w', encoding='utf-8') as f:
    f.write(d3)

# 4. Fix api-client/src/admin.ts unused import AdminProduct
f4 = 'packages/api-client/src/admin.ts'
with open(f4, 'r', encoding='utf-8') as f:
    d4 = f.read()
d4 = d4.replace("AdminProduct, ", "")
with open(f4, 'w', encoding='utf-8') as f:
    f.write(d4)

print("Done fixing everything.")

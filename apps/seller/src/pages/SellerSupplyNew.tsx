import { useState, useEffect, useMemo, useRef } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import {
  ArrowLeft,
  AlertCircle,
  Search,
  Truck,
  Image as ImageIcon,
  ChevronDown,
  ChevronUp,
  Filter,
  ArrowUpDown,
  Check,
  X,
} from 'lucide-react';
import { getSellerProducts, createSellerSupply, getSellerCategories, type SellerCategory } from '@zamk/api-client/src/seller';
import type { SellerProduct } from '@zamk/api-client/src/types';

function getVariantsWord(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'вариантов';
  if (mod10 === 1) return 'вариант';
  if (mod10 >= 2 && mod10 <= 4) return 'варианта';
  return 'вариантов';
}

interface BoxItem {
  variantId: string;
  sku: string;
  title: string;
  options: string;
  barcode: string;
  quantity: number;
  imageUrl?: string;
}

export type VariantEditorLayout =
  | { type: 'single'; variant: any }
  | {
      type: 'matrix_2d';
      rowDimName: string;
      colDimName: string;
      rowValues: string[];
      colValues: string[];
      cellMap: Map<string, any>;
    }
  | {
      type: 'list_1d';
      dimName: string;
      items: { value: string; variant: any }[];
    }
  | {
      type: 'fallback';
      variants: any[];
    };

export function detectProductVariantLayout(variants: any[]): VariantEditorLayout {
  if (!variants || variants.length === 0) {
    return { type: 'fallback', variants: [] };
  }
  if (variants.length === 1) {
    return { type: 'single', variant: variants[0] };
  }

  const hasColors = variants.some(v => Boolean((v.colorName || v.color || '').trim()));
  const hasSizes = variants.some(v => Boolean((v.sizeName || v.size || '').trim()));
  const allHaveColors = variants.every(v => Boolean((v.colorName || v.color || '').trim()));
  const allHaveSizes = variants.every(v => Boolean((v.sizeName || v.size || '').trim()));

  const hasOtherOptions = variants.some(v => {
    if (!v.optionValues || typeof v.optionValues !== 'object') return false;
    const keys = Object.keys(v.optionValues).filter(k => {
      const lower = k.toLowerCase();
      return lower !== 'color' && lower !== 'цвет' && lower !== 'size' && lower !== 'размер';
    });
    return keys.length > 0;
  });

  if (!hasOtherOptions && (hasColors || hasSizes)) {
    const uniqueColors: string[] = [];
    const uniqueSizes: string[] = [];

    variants.forEach(v => {
      const c = (v.colorName || v.color || '').trim();
      const s = (v.sizeName || v.size || '').trim();
      if (c && !uniqueColors.includes(c)) uniqueColors.push(c);
      if (s && !uniqueSizes.includes(s)) uniqueSizes.push(s);
    });

    if (allHaveColors && allHaveSizes && uniqueColors.length >= 2 && uniqueSizes.length >= 2) {
      const cellMap = new Map<string, any>();
      let collision = false;
      for (const v of variants) {
        const c = (v.colorName || v.color || '').trim();
        const s = (v.sizeName || v.size || '').trim();
        const key = `${c}:::${s}`;
        if (cellMap.has(key)) {
          collision = true;
          break;
        }
        cellMap.set(key, v);
      }
      if (!collision) {
        return {
          type: 'matrix_2d',
          rowDimName: 'Цвет',
          colDimName: 'Размер',
          rowValues: uniqueColors,
          colValues: uniqueSizes,
          cellMap,
        };
      }
    }

    if (uniqueSizes.length >= 2 && (!hasColors || uniqueColors.length <= 1)) {
      return {
        type: 'list_1d',
        dimName: 'Размер',
        items: variants.map(v => ({
          value: (v.sizeName || v.size || '').trim() || 'Стандарт',
          variant: v,
        })),
      };
    }

    if (uniqueColors.length >= 2 && (!hasSizes || uniqueSizes.length <= 1)) {
      return {
        type: 'list_1d',
        dimName: 'Цвет',
        items: variants.map(v => ({
          value: (v.colorName || v.color || '').trim(),
          variant: v,
        })),
      };
    }
  }

  if (!hasColors && !hasSizes && variants.every(v => v.optionValues && typeof v.optionValues === 'object')) {
    const allKeysSet = new Set<string>();
    variants.forEach(v => Object.keys(v.optionValues).forEach(k => allKeysSet.add(k)));
    const allKeys = Array.from(allKeysSet);

    if (allKeys.length === 1) {
      const dimKey = allKeys[0];
      return {
        type: 'list_1d',
        dimName: dimKey,
        items: variants.map(v => ({
          value: String(v.optionValues[dimKey] ?? '').trim(),
          variant: v,
        })),
      };
    }

    if (allKeys.length === 2) {
      const [key1, key2] = allKeys;
      const vals1: string[] = [];
      const vals2: string[] = [];
      variants.forEach(v => {
        const v1 = String(v.optionValues[key1] ?? '').trim();
        const v2 = String(v.optionValues[key2] ?? '').trim();
        if (v1 && !vals1.includes(v1)) vals1.push(v1);
        if (v2 && !vals2.includes(v2)) vals2.push(v2);
      });

      if (vals1.length >= 2 && vals2.length >= 2) {
        const cellMap = new Map<string, any>();
        let collision = false;
        for (const v of variants) {
          const v1 = String(v.optionValues[key1] ?? '').trim();
          const v2 = String(v.optionValues[key2] ?? '').trim();
          const key = `${v1}:::${v2}`;
          if (cellMap.has(key)) {
            collision = true;
            break;
          }
          cellMap.set(key, v);
        }
        if (!collision) {
          return {
            type: 'matrix_2d',
            rowDimName: key1,
            colDimName: key2,
            rowValues: vals1,
            colValues: vals2,
            cellMap,
          };
        }
      } else if (vals1.length >= 2 && vals2.length <= 1) {
        return {
          type: 'list_1d',
          dimName: key1,
          items: variants.map(v => ({
            value: String(v.optionValues[key1] ?? '').trim(),
            variant: v,
          })),
        };
      } else if (vals2.length >= 2 && vals1.length <= 1) {
        return {
          type: 'list_1d',
          dimName: key2,
          items: variants.map(v => ({
            value: String(v.optionValues[key2] ?? '').trim(),
            variant: v,
          })),
        };
      }
    }
  }

  return {
    type: 'fallback',
    variants,
  };
}

const NO_SPINNER_CLASS = '[appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none';

function ProductVariantAdaptiveEditor({
  product,
  selectedItems,
  onQuantityChange,
}: {
  product: any;
  selectedItems: BoxItem[];
  onQuantityChange: (variant: any, qty: number, imageUrl?: string) => void;
}) {
  const layout = useMemo(() => detectProductVariantLayout(product.variants || []), [product.variants]);

  if (layout.type === 'single') {
    const v = layout.variant;
    const selectedQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
    const sku = v.sellerSku || v.sku || 'нет';
    const barcode = v.barcode || 'нет';
    return (
      <div className="p-4 sm:p-5 bg-gray-50/40 border-t border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Единственный вариант</div>
          <div className="flex flex-wrap items-center gap-2.5 mt-1">
            <span className="text-xs text-gray-500">
              Артикул: <span className="font-mono text-gray-800 font-medium">{sku}</span>
            </span>
            <span className="text-gray-300">·</span>
            <span className="text-xs text-gray-500">
              Штрихкод: <span className="font-mono text-gray-800 font-medium">{barcode}</span>
            </span>
          </div>
        </div>
        <div className="flex items-center gap-3 self-end sm:self-center">
          <label className="text-xs font-bold text-gray-500 uppercase tracking-wider">Количество:</label>
          <div className="inline-flex items-center rounded-lg border border-gray-200 bg-white shadow-xs overflow-hidden">
            <button
              type="button"
              onClick={() => onQuantityChange(v, Math.max(0, selectedQty - 1), product.mainImage)}
              className="w-8 h-8 flex items-center justify-center font-bold text-gray-500 hover:text-black hover:bg-gray-100 transition-colors cursor-pointer"
            >
              -
            </button>
            <input
              type="number"
              min="0"
              step="1"
              value={selectedQty || ''}
              placeholder="0"
              aria-label="Количество"
              onChange={(e) => onQuantityChange(v, parseInt(e.target.value) || 0, product.mainImage)}
              onFocus={(e) => e.target.select()}
              className={`w-14 h-8 text-center text-sm font-bold border-x border-gray-200 py-0 focus:outline-none focus:ring-1 focus:ring-black ${NO_SPINNER_CLASS} ${
                selectedQty > 0 ? 'text-gray-900 bg-white' : 'text-gray-400 bg-white'
              }`}
            />
            <button
              type="button"
              onClick={() => onQuantityChange(v, selectedQty + 1, product.mainImage)}
              className="w-8 h-8 flex items-center justify-center font-bold text-gray-500 hover:text-black hover:bg-gray-100 transition-colors cursor-pointer"
            >
              +
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (layout.type === 'matrix_2d') {
    return (
      <div className="p-4 sm:p-5 bg-gray-50/40 border-t border-gray-100 overflow-x-auto">
        <div className="mb-2.5 flex items-center justify-between">
          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
            Матрица: {layout.rowDimName} × {layout.colDimName}
          </span>
        </div>
        <table className="min-w-full divide-y divide-gray-200 border border-gray-200/80 rounded-xl overflow-hidden bg-white shadow-xs">
          <thead className="bg-gray-50/80 border-b border-gray-200">
            <tr>
              <th scope="col" className="px-3.5 py-2.5 text-left text-xs font-bold text-gray-600 uppercase tracking-wider bg-gray-100/70 border-r border-gray-200 sticky left-0 z-10 min-w-[120px]">
                {layout.rowDimName} / {layout.colDimName}
              </th>
              {layout.colValues.map(col => (
                <th key={col} scope="col" className="px-2 py-2.5 text-center text-xs font-bold text-gray-700 uppercase tracking-wider min-w-[64px]">
                  {col}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 bg-white">
            {layout.rowValues.map(row => (
              <tr key={row} className="hover:bg-gray-50/40 transition-colors">
                <td className="px-3.5 py-2 whitespace-nowrap text-xs sm:text-sm font-semibold text-gray-900 bg-gray-50/80 border-r border-gray-200 sticky left-0 z-10">
                  {row}
                </td>
                {layout.colValues.map(col => {
                  const v = layout.cellMap.get(`${row}:::${col}`);
                  if (!v) {
                    return (
                      <td key={col} className="px-1.5 py-1.5 text-center bg-gray-50/30 select-none">
                        <span className="text-gray-300 font-bold text-xs" title="Вариант недоступен">—</span>
                      </td>
                    );
                  }
                  const selectedQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
                  const isSelected = selectedQty > 0;
                  const sku = v.sellerSku || v.sku || '';
                  return (
                    <td key={col} className={`px-1.5 py-1.5 text-center transition-colors ${isSelected ? 'bg-black/[0.03]' : ''}`}>
                      <input
                        type="number"
                        min="0"
                        step="1"
                        value={selectedQty || ''}
                        placeholder="0"
                        title={`${product.title} · ${row} · ${col}${sku ? ` (${sku})` : ''}`}
                        aria-label={`${row} ${col}`}
                        onChange={(e) => onQuantityChange(v, parseInt(e.target.value) || 0, product.mainImage)}
                        onFocus={(e) => e.target.select()}
                        className={`w-14 sm:w-16 h-8 text-center rounded-lg border text-sm font-semibold transition-all focus:outline-none focus:ring-2 focus:ring-black focus:border-black ${NO_SPINNER_CLASS} ${
                          isSelected
                            ? 'border-black bg-white shadow-xs font-bold text-gray-900'
                            : 'border-gray-200 bg-white/90 text-gray-600 hover:border-gray-300'
                        }`}
                      />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (layout.type === 'list_1d') {
    return (
      <div className="p-4 sm:p-5 bg-gray-50/40 border-t border-gray-100 overflow-x-auto">
        <div className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2.5">
          Варианты ({layout.dimName})
        </div>
        <div className="border border-gray-200/80 rounded-xl overflow-hidden bg-white shadow-xs">
          <table className="min-w-full divide-y divide-gray-100">
            <thead className="bg-gray-50/80 border-b border-gray-200">
              <tr>
                <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-600 uppercase tracking-wider">
                  {layout.dimName}
                </th>
                <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                  Артикул
                </th>
                <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                  Штрихкод
                </th>
                <th scope="col" className="px-4 py-2.5 text-right text-xs font-bold text-gray-600 uppercase tracking-wider w-36">
                  Количество (шт)
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 bg-white">
              {layout.items.map(({ value, variant }) => {
                const selectedQty = selectedItems.find(i => i.variantId === variant.id)?.quantity || 0;
                const isSelected = selectedQty > 0;
                return (
                  <tr key={variant.id} className={`hover:bg-gray-50/50 transition-colors ${isSelected ? 'bg-black/[0.02]' : ''}`}>
                    <td className="px-4 py-2.5 whitespace-nowrap text-sm font-semibold text-gray-900">
                      {value}
                    </td>
                    <td className="px-4 py-2.5 whitespace-nowrap text-xs font-mono text-gray-500">
                      {variant.sellerSku || variant.sku || '—'}
                    </td>
                    <td className="px-4 py-2.5 whitespace-nowrap text-xs font-mono text-gray-500">
                      {variant.barcode || '—'}
                    </td>
                    <td className="px-4 py-1.5 text-right">
                      <input
                        type="number"
                        min="0"
                        step="1"
                        value={selectedQty || ''}
                        placeholder="0"
                        aria-label={`${layout.dimName} ${value}`}
                        onChange={(e) => onQuantityChange(variant, parseInt(e.target.value) || 0, product.mainImage)}
                        onFocus={(e) => e.target.select()}
                        className={`w-20 h-8 text-right rounded-lg border py-1 px-2.5 text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-black focus:border-black ${NO_SPINNER_CLASS} ${
                          isSelected ? 'border-black bg-white font-bold text-gray-900 shadow-xs' : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300'
                        }`}
                      />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  // Fallback editor
  return (
    <div className="p-4 sm:p-5 bg-gray-50/40 border-t border-gray-100 overflow-x-auto">
      <div className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2.5">
        Варианты товара
      </div>
      <div className="border border-gray-200/80 rounded-xl overflow-hidden bg-white shadow-xs">
        <table className="min-w-full divide-y divide-gray-100">
          <thead className="bg-gray-50/80 border-b border-gray-200">
            <tr>
              <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-600 uppercase tracking-wider">
                Характеристики
              </th>
              <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                Артикул
              </th>
              <th scope="col" className="px-4 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                Штрихкод
              </th>
              <th scope="col" className="px-4 py-2.5 text-right text-xs font-bold text-gray-600 uppercase tracking-wider w-36">
                Количество (шт)
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 bg-white">
            {layout.variants.map((v: any) => {
              const selectedQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
              const isSelected = selectedQty > 0;
              return (
                <tr key={v.id} className={`hover:bg-gray-50/50 transition-colors ${isSelected ? 'bg-black/[0.02]' : ''}`}>
                  <td className="px-4 py-2.5 whitespace-nowrap text-sm font-semibold text-gray-900">
                    {v.optionsStr}
                  </td>
                  <td className="px-4 py-2.5 whitespace-nowrap text-xs font-mono text-gray-500">
                    {v.sellerSku || v.sku || '—'}
                  </td>
                  <td className="px-4 py-2.5 whitespace-nowrap text-xs font-mono text-gray-500">
                    {v.barcode || '—'}
                  </td>
                  <td className="px-4 py-1.5 text-right">
                    <input
                      type="number"
                      min="0"
                      step="1"
                      value={selectedQty || ''}
                      placeholder="0"
                      aria-label={v.optionsStr}
                      onChange={(e) => onQuantityChange(v, parseInt(e.target.value) || 0, product.mainImage)}
                      onFocus={(e) => e.target.select()}
                      className={`w-20 h-8 text-right rounded-lg border py-1 px-2.5 text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-black focus:border-black ${NO_SPINNER_CLASS} ${
                        isSelected ? 'border-black bg-white font-bold text-gray-900 shadow-xs' : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300'
                      }`}
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export type SortMode = 'default' | 'name_asc' | 'name_desc' | 'added_first' | 'qty_desc';

export const SORT_OPTIONS: { id: SortMode; label: string }[] = [
  { id: 'default', label: 'По умолчанию' },
  { id: 'name_asc', label: 'По названию А–Я' },
  { id: 'name_desc', label: 'По названию Я–А' },
  { id: 'added_first', label: 'Сначала добавленные' },
  { id: 'qty_desc', label: 'По количеству: больше → меньше' },
];

export const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function SellerSupplyNew() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [step, setStep] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('');
  const [selectedColor, setSelectedColor] = useState('');
  const [selectedSize, setSelectedSize] = useState('');
  const [onlySelectedFilter, setOnlySelectedFilter] = useState(false);
  const [selectedSort, setSelectedSort] = useState<SortMode>('default');
  const [expandedProductIds, setExpandedProductIds] = useState<Set<string>>(new Set());

  // Filter Popover & Sort Popover State
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [isSortOpen, setIsSortOpen] = useState(false);
  const [draftCategory, setDraftCategory] = useState('');
  const [draftColor, setDraftColor] = useState('');
  const [draftSize, setDraftSize] = useState('');
  const [draftOnlySelected, setDraftOnlySelected] = useState(false);

  const filterRef = useRef<HTMLDivElement>(null);
  const sortRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsFilterOpen(false);
        setIsSortOpen(false);
      }
    };
    const handleClickOutside = (e: MouseEvent) => {
      if (filterRef.current && !filterRef.current.contains(e.target as Node)) {
        setIsFilterOpen(false);
      }
      if (sortRef.current && !sortRef.current.contains(e.target as Node)) {
        setIsSortOpen(false);
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  // Step 1: Products
  const [selectedItems, setSelectedItems] = useState<BoxItem[]>([]);

  // Step 2: Handoff
  const carrierCompany = 'СДЭК';
  const [trackingNumber, setTrackingNumber] = useState('');
  const [trackingError, setTrackingError] = useState<string | null>(null);

  const [submitting, setSubmitting] = useState(false);

  const handleStep2Continue = () => {
    if (!trackingNumber.trim()) {
      setTrackingError('Укажите трек-номер отправления.');
      return;
    }
    setTrackingError(null);
    setStep(3);
  };

  const [categories, setCategories] = useState<SellerCategory[]>([]);

  useEffect(() => {
    fetchProducts();
  }, []);

  const fetchProducts = async () => {
    try {
      setLoading(true);
      const [data, catData] = await Promise.all([
        getSellerProducts(),
        getSellerCategories().catch(() => []),
      ]);
      setProducts(data.filter(p => p.status === 'approved' || p.status === 'published'));
      setCategories(catData || []);
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки товаров');
    } finally {
      setLoading(false);
    }
  };

  const toggleProductExpansion = (productId: string) => {
    setExpandedProductIds(prev => {
      const next = new Set(prev);
      if (next.has(productId)) {
        next.delete(productId);
      } else {
        next.add(productId);
      }
      return next;
    });
  };

  const categoryMap = useMemo(() => {
    const map = new Map<string, string>();
    categories.forEach(c => {
      if (c.id && c.name && !UUID_REGEX.test(c.name.trim())) {
        map.set(c.id, c.name.trim());
      }
    });
    return map;
  }, [categories]);

  const getCategoryLabel = (id?: string, product?: SellerProduct): string => {
    if (!id) return '';
    if (categoryMap.has(id)) {
      return categoryMap.get(id)!;
    }
    if (product) {
      const rawName = (product as any).categoryName || (product as any).category;
      if (rawName && typeof rawName === 'string' && !UUID_REGEX.test(rawName.trim())) {
        return rawName.trim();
      }
    }
    if (!UUID_REGEX.test(id.trim())) {
      return id.trim();
    }
    return '';
  };

  const categoryOptions = useMemo(() => {
    const map = new Map<string, string>();
    products.forEach(p => {
      const id = p.categoryId || (p as any).category || (p as any).categoryName;
      if (id && typeof id === 'string' && id.trim()) {
        const trimmedId = id.trim();
        const label = getCategoryLabel(trimmedId, p);
        if (label && !UUID_REGEX.test(label) && !map.has(trimmedId)) {
          map.set(trimmedId, label);
        }
      }
    });
    return Array.from(map.entries()).map(([id, name]) => ({ id, name }));
  }, [products, categoryMap]);

  const selectedCategoryName = useMemo(() => {
    if (!selectedCategory) return '';
    const fromOptions = categoryOptions.find(c => c.id === selectedCategory)?.name;
    if (fromOptions) return fromOptions;
    const fromMap = categoryMap.get(selectedCategory);
    if (fromMap) return fromMap;
    if (!UUID_REGEX.test(selectedCategory)) return selectedCategory;
    return '';
  }, [selectedCategory, categoryOptions, categoryMap]);

  const colorOptions = useMemo(() => {
    const set = new Set<string>();
    products.forEach(p => {
      (p.variants || []).forEach(v => {
        const c = (v.colorName || v.color || '').trim();
        if (c) set.add(c);
      });
    });
    return Array.from(set);
  }, [products]);

  const sizeOptions = useMemo(() => {
    const set = new Set<string>();
    products.forEach(p => {
      (p.variants || []).forEach(v => {
        const s = (v.sizeName || v.size || '').trim();
        if (s) set.add(s);
      });
    });
    return Array.from(set);
  }, [products]);

  const activeFiltersCount =
    (selectedCategory ? 1 : 0) +
    (selectedColor ? 1 : 0) +
    (selectedSize ? 1 : 0) +
    (onlySelectedFilter ? 1 : 0);

  const handleOpenFilters = () => {
    if (!isFilterOpen) {
      setDraftCategory(selectedCategory);
      setDraftColor(selectedColor);
      setDraftSize(selectedSize);
      setDraftOnlySelected(onlySelectedFilter);
      setIsSortOpen(false);
    }
    setIsFilterOpen(!isFilterOpen);
  };

  const handleApplyFilters = () => {
    setSelectedCategory(draftCategory);
    setSelectedColor(draftColor);
    setSelectedSize(draftSize);
    setOnlySelectedFilter(draftOnlySelected);
    setIsFilterOpen(false);
  };

  const handleResetFilters = () => {
    setDraftCategory('');
    setDraftColor('');
    setDraftSize('');
    setDraftOnlySelected(false);
    setSelectedCategory('');
    setSelectedColor('');
    setSelectedSize('');
    setOnlySelectedFilter(false);
    setIsFilterOpen(false);
  };

  const groupedProducts = useMemo(() => {
    let filtered = products;
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = products.filter(p =>
        p.title.toLowerCase().includes(query) ||
        p.variants?.some(v =>
          (v.sku && v.sku.toLowerCase().includes(query)) ||
          (v.sellerSku && v.sellerSku.toLowerCase().includes(query)) ||
          (v.barcode && v.barcode.toLowerCase().includes(query))
        )
      );
    }

    return filtered.map(p => {
      const mainImage = p.images?.find(i => i.isMain)?.url || p.images?.[0]?.url;
      return {
        ...p,
        mainImage,
        variants: (p.variants || []).map(v => {
          const color = v.colorName || v.color;
          const size = v.sizeName || v.size;
          const optionsStr = (color || size)
            ? [color, size].filter(Boolean).join(' · ')
            : (v.optionValues && Object.keys(v.optionValues).length > 0
                ? Object.entries(v.optionValues).map(([_, val]) => `${val}`).join(' · ')
                : 'Стандарт');

          return {
            ...v,
            optionsStr,
            productTitle: p.title
          };
        })
      };
    });
  }, [products, searchQuery]);

  const filteredProductsList = useMemo(() => {
    let list = [...groupedProducts];

    if (selectedCategory) {
      list = list.filter(p =>
        p.categoryId === selectedCategory ||
        (p as any).category === selectedCategory ||
        (p as any).categoryName === selectedCategory
      );
    }

    if (selectedColor) {
      list = list.filter(p =>
        (p.variants || []).some((v: any) => (v.colorName || v.color || '').trim() === selectedColor)
      );
    }

    if (selectedSize) {
      list = list.filter(p =>
        (p.variants || []).some((v: any) => (v.sizeName || v.size || '').trim() === selectedSize)
      );
    }

    if (onlySelectedFilter) {
      list = list.filter(p => {
        return (p.variants || []).some((v: any) => {
          const item = selectedItems.find(i => i.variantId === v.id);
          return (item?.quantity || 0) > 0;
        });
      });
    }

    if (selectedSort === 'name_asc') {
      list.sort((a, b) => a.title.localeCompare(b.title, 'ru'));
    } else if (selectedSort === 'name_desc') {
      list.sort((a, b) => b.title.localeCompare(a.title, 'ru'));
    } else if (selectedSort === 'added_first') {
      list.sort((a, b) => {
        const aQty = (a.variants || []).reduce((sum: number, v: any) => sum + (selectedItems.find(i => i.variantId === v.id)?.quantity || 0), 0);
        const bQty = (b.variants || []).reduce((sum: number, v: any) => sum + (selectedItems.find(i => i.variantId === v.id)?.quantity || 0), 0);
        const aHas = aQty > 0 ? 1 : 0;
        const bHas = bQty > 0 ? 1 : 0;
        if (bHas !== aHas) return bHas - aHas;
        return 0;
      });
    } else if (selectedSort === 'qty_desc') {
      list.sort((a, b) => {
        const aQty = (a.variants || []).reduce((sum: number, v: any) => sum + (selectedItems.find(i => i.variantId === v.id)?.quantity || 0), 0);
        const bQty = (b.variants || []).reduce((sum: number, v: any) => sum + (selectedItems.find(i => i.variantId === v.id)?.quantity || 0), 0);
        if (bQty !== aQty) return bQty - aQty;
        return 0;
      });
    }

    return list;
  }, [groupedProducts, selectedCategory, selectedColor, selectedSize, onlySelectedFilter, selectedSort, selectedItems]);

  const selectedProductsCount = useMemo(() => {
    const prodIdsWithQty = new Set<string>();
    for (const p of products) {
      const hasQty = (p.variants || []).some(v => {
        const item = selectedItems.find(i => i.variantId === v.id);
        return (item?.quantity || 0) > 0;
      });
      if (hasQty) {
        prodIdsWithQty.add(p.id);
      }
    }
    return prodIdsWithQty.size;
  }, [products, selectedItems]);

  // Step 1 Handlers
  const handleItemQuantityChange = (variant: any, qty: number, imageUrl?: string) => {
    const validQty = Math.max(0, qty);
    setSelectedItems(prev => {
      const existing = prev.find(i => i.variantId === variant.id);
      if (validQty <= 0) {
        return prev.filter(i => i.variantId !== variant.id);
      }
      if (existing) {
        return prev.map(i => i.variantId === variant.id ? { ...i, quantity: validQty } : i);
      }
      return [...prev, {
        variantId: variant.id,
        sku: variant.sellerSku || variant.sku || '',
        title: variant.productTitle,
        options: variant.optionsStr,
        barcode: variant.barcode || '',
        quantity: validQty,
        imageUrl
      }];
    });
  };

  const declaredTotal = selectedItems.reduce((sum, i) => sum + i.quantity, 0);

  const handleSubmit = async () => {
    const payloadItems = selectedItems.map(i => ({
      variantId: i.variantId,
      expectedQuantity: i.quantity
    }));

    try {
      setSubmitting(true);
      setError(null);
      const res = await createSellerSupply({
        handoffMethod: 'carrier_delivery',
        carrierName: carrierCompany,
        trackingNumber: trackingNumber,
        items: payloadItems
      });
      navigate(`/supplies/${res.id}`);
    } catch (err: any) {
      setError(err.message || 'Ошибка при создании поставки');
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-6xl mx-auto py-6 px-4 sm:px-6">
      {/* Coherent Header Section: Title & Stepper */}
      <div className="mb-6 pb-5 border-b border-gray-200/80 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Link
            to="/supplies"
            className="w-9 h-9 rounded-xl border border-gray-200 hover:border-gray-300 bg-white hover:bg-gray-50 flex items-center justify-center text-gray-500 hover:text-black transition-colors shrink-0"
            title="Назад к поставкам"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Новая поставка</h1>
        </div>

        {/* Compact Stepper */}
        <div className="flex items-center gap-1.5 sm:gap-2 self-start sm:self-auto">
          <div
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors ${
              step >= 1 ? 'bg-black text-white font-bold' : 'bg-gray-100 text-gray-400'
            }`}
          >
            <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${step >= 1 ? 'bg-white/20 text-white' : 'bg-gray-200 text-gray-500'}`}>
              1
            </span>
            <span>Товары</span>
          </div>
          <div className="w-4 h-px bg-gray-200" />
          <div
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors ${
              step >= 2 ? 'bg-black text-white font-bold' : 'bg-gray-100 text-gray-400'
            }`}
          >
            <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${step >= 2 ? 'bg-white/20 text-white' : 'bg-gray-200 text-gray-500'}`}>
              2
            </span>
            <span>Доставка</span>
          </div>
          <div className="w-4 h-px bg-gray-200" />
          <div
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors ${
              step >= 3 ? 'bg-black text-white font-bold' : 'bg-gray-100 text-gray-400'
            }`}
          >
            <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${step >= 3 ? 'bg-white/20 text-white' : 'bg-gray-200 text-gray-500'}`}>
              3
            </span>
            <span>Проверка</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-50 text-red-700 rounded-xl flex items-center border border-red-100">
          <AlertCircle className="w-5 h-5 mr-3 shrink-0" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* STEP 1: Products */}
      {step === 1 && (
        <div className="space-y-4">
          <div className="bg-white shadow-xs border border-gray-200/90 rounded-2xl p-5 sm:p-6">
            {/* Header & Counters */}
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center pb-5 border-b border-gray-100 gap-4">
              <div>
                <h3 className="text-xl font-bold text-gray-900 tracking-tight">Товары для поставки</h3>
                <p className="mt-0.5 text-xs text-gray-500">Укажите количество товаров, которое отправляете на склад ZAMK.</p>
              </div>
              <div className="flex items-center gap-2 sm:gap-3 self-stretch sm:self-auto justify-end">
                <div className="bg-gray-50/90 border border-gray-200/70 rounded-xl px-3.5 py-2 flex flex-col items-end min-w-[125px]">
                  <span className="text-lg font-bold text-gray-900 leading-tight">{selectedProductsCount}</span>
                  <span className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">Товаров с количеством</span>
                </div>
                <div className="bg-gray-50/90 border border-gray-200/70 rounded-xl px-3.5 py-2 flex flex-col items-end min-w-[125px]">
                  <span className="text-xl font-black text-gray-900 leading-tight">{declaredTotal}</span>
                  <span className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">Всего единиц</span>
                </div>
              </div>
            </div>

            {/* Compact Filter & Search Toolbar */}
            <div className="pt-4 pb-2 flex flex-col sm:flex-row items-stretch sm:items-center gap-2.5">
              {/* Search */}
              <div className="relative flex-1">
                <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-gray-400">
                  <Search className="h-4 w-4" />
                </div>
                <input
                  type="text"
                  className="h-10 text-sm bg-white border border-gray-200 rounded-xl pl-9 pr-4 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-colors w-full"
                  placeholder="Поиск по названию, артикулу или штрихкоду"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>

              {/* Action Buttons */}
              <div className="flex items-center gap-2">
                {/* Filter Button & Anchored Popover */}
                <div className="relative" ref={filterRef}>
                  <button
                    type="button"
                    onClick={handleOpenFilters}
                    aria-expanded={isFilterOpen}
                    aria-haspopup="dialog"
                    className={`inline-flex items-center justify-center px-3.5 h-10 rounded-xl text-sm font-semibold border transition-colors cursor-pointer shrink-0 ${
                      activeFiltersCount > 0
                        ? 'bg-black text-white border-black shadow-xs font-bold'
                        : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50 hover:border-gray-300'
                    }`}
                  >
                    <Filter className="w-3.5 h-3.5 mr-2" />
                    Фильтры
                    {activeFiltersCount > 0 && (
                      <span className="ml-2 px-1.5 py-0.5 rounded-full text-xs font-black bg-white text-black leading-none">
                        {activeFiltersCount}
                      </span>
                    )}
                  </button>

                  {isFilterOpen && (
                    <div className="absolute right-0 sm:right-auto sm:left-0 top-full mt-2 w-80 bg-white border border-gray-200/90 rounded-2xl shadow-xl z-30 p-4 space-y-4">
                      <div className="flex items-center justify-between pb-3 border-b border-gray-100">
                        <span className="text-sm font-bold text-gray-900">Фильтры</span>
                        <button
                          type="button"
                          onClick={() => setIsFilterOpen(false)}
                          aria-label="Закрыть фильтры"
                          className="text-gray-400 hover:text-gray-600 p-1 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>

                      <div className="space-y-3">
                        {/* Category */}
                        <div>
                          <label className="block text-xs font-semibold text-gray-500 mb-1">Категория</label>
                          <div className="relative">
                            <select
                              aria-label="Категория"
                              value={draftCategory}
                              onChange={(e) => setDraftCategory(e.target.value)}
                              className="w-full h-10 text-sm font-medium rounded-xl border border-gray-200 bg-gray-50/50 hover:bg-gray-50 focus:bg-white text-gray-900 py-2 pl-3 pr-9 appearance-none focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-colors cursor-pointer"
                            >
                              <option value="">Все категории</option>
                              {categoryOptions.map(cat => (
                                <option key={cat.id} value={cat.id}>
                                  {cat.name}
                                </option>
                              ))}
                            </select>
                            <ChevronDown className="w-4 h-4 text-gray-400 absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                          </div>
                        </div>

                        {/* Color */}
                        <div>
                          <label className="block text-xs font-semibold text-gray-500 mb-1">Цвет</label>
                          <div className="relative">
                            <select
                              aria-label="Цвет"
                              value={draftColor}
                              onChange={(e) => setDraftColor(e.target.value)}
                              className="w-full h-10 text-sm font-medium rounded-xl border border-gray-200 bg-gray-50/50 hover:bg-gray-50 focus:bg-white text-gray-900 py-2 pl-3 pr-9 appearance-none focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-colors cursor-pointer"
                            >
                              <option value="">Все цвета</option>
                              {colorOptions.map(c => (
                                <option key={c} value={c}>
                                  {c}
                                </option>
                              ))}
                            </select>
                            <ChevronDown className="w-4 h-4 text-gray-400 absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                          </div>
                        </div>

                        {/* Size */}
                        <div>
                          <label className="block text-xs font-semibold text-gray-500 mb-1">Размер</label>
                          <div className="relative">
                            <select
                              aria-label="Размер"
                              value={draftSize}
                              onChange={(e) => setDraftSize(e.target.value)}
                              className="w-full h-10 text-sm font-medium rounded-xl border border-gray-200 bg-gray-50/50 hover:bg-gray-50 focus:bg-white text-gray-900 py-2 pl-3 pr-9 appearance-none focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-colors cursor-pointer"
                            >
                              <option value="">Все размеры</option>
                              {sizeOptions.map(s => (
                                <option key={s} value={s}>
                                  {s}
                                </option>
                              ))}
                            </select>
                            <ChevronDown className="w-4 h-4 text-gray-400 absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                          </div>
                        </div>

                        {/* Only Added */}
                        <label className="flex items-center gap-2.5 pt-1 cursor-pointer select-none text-sm font-medium text-gray-800 hover:text-black transition-colors">
                          <input
                            type="checkbox"
                            aria-label="Только добавленные"
                            checked={draftOnlySelected}
                            onChange={(e) => setDraftOnlySelected(e.target.checked)}
                            className="w-4 h-4 rounded text-black focus:ring-black border-gray-300 cursor-pointer"
                          />
                          <span>Только добавленные</span>
                          {selectedProductsCount > 0 && (
                            <span className="text-xs px-2 py-0.5 rounded-full bg-gray-100 text-gray-700 font-bold ml-auto">
                              +{selectedProductsCount}
                            </span>
                          )}
                        </label>
                      </div>

                      {/* Footer Actions */}
                      <div className="flex items-center justify-between pt-3 border-t border-gray-100">
                        <button
                          type="button"
                          onClick={handleResetFilters}
                          className="text-xs font-semibold text-gray-500 hover:text-red-600 px-2.5 py-1.5 rounded-lg hover:bg-red-50/50 transition-colors cursor-pointer"
                        >
                          Сбросить
                        </button>
                        <button
                          type="button"
                          onClick={handleApplyFilters}
                          className="px-4 py-2 bg-black text-white text-xs font-bold rounded-xl hover:bg-gray-800 transition-colors cursor-pointer shadow-xs"
                        >
                          Применить
                        </button>
                      </div>
                    </div>
                  )}
                </div>

                {/* Sort Button & Anchored Menu */}
                <div className="relative" ref={sortRef}>
                  <button
                    type="button"
                    onClick={() => {
                      if (!isSortOpen) {
                        setIsFilterOpen(false);
                      }
                      setIsSortOpen(!isSortOpen);
                    }}
                    aria-expanded={isSortOpen}
                    aria-haspopup="menu"
                    className={`inline-flex items-center justify-center px-3.5 h-10 rounded-xl text-sm font-semibold border transition-colors cursor-pointer shrink-0 ${
                      selectedSort !== 'default'
                        ? 'bg-black text-white border-black shadow-xs font-bold'
                        : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50 hover:border-gray-300'
                    }`}
                  >
                    <ArrowUpDown className="w-3.5 h-3.5 mr-2" />
                    Сортировка
                  </button>

                  {isSortOpen && (
                    <div className="absolute right-0 top-full mt-2 w-64 bg-white border border-gray-200/90 rounded-2xl shadow-xl z-30 p-2 space-y-1">
                      <div className="px-3 py-1.5 text-[10px] font-bold text-gray-400 uppercase tracking-wider border-b border-gray-100 mb-1">
                        Сортировка
                      </div>
                      {SORT_OPTIONS.map(opt => {
                        const isSelected = selectedSort === opt.id;
                        return (
                          <button
                            key={opt.id}
                            type="button"
                            role="menuitem"
                            onClick={() => {
                              setSelectedSort(opt.id);
                              setIsSortOpen(false);
                            }}
                            className={`w-full flex items-center justify-between px-3 py-2 rounded-xl text-xs sm:text-sm font-medium transition-colors cursor-pointer text-left ${
                              isSelected
                                ? 'bg-black/5 text-black font-bold'
                                : 'text-gray-700 hover:bg-gray-50'
                            }`}
                          >
                            <span>{opt.label}</span>
                            {isSelected && <Check className="w-4 h-4 text-black shrink-0 ml-2" />}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Active Filter Chips */}
            {activeFiltersCount > 0 && (
              <div className="pt-2 pb-2 flex flex-wrap items-center gap-2">
                {selectedCategory && (
                  <span className="inline-flex items-center gap-1.5 pl-3 pr-1.5 py-1 rounded-full text-xs font-medium bg-gray-100/90 text-gray-800 border border-gray-200/80 transition-colors">
                    <span>Категория: {selectedCategoryName}</span>
                    <button
                      type="button"
                      aria-label="Удалить фильтр категории"
                      onClick={() => setSelectedCategory('')}
                      className="hover:bg-gray-200/80 rounded-full p-0.5 text-gray-400 hover:text-black transition-colors cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </span>
                )}
                {selectedColor && (
                  <span className="inline-flex items-center gap-1.5 pl-3 pr-1.5 py-1 rounded-full text-xs font-medium bg-gray-100/90 text-gray-800 border border-gray-200/80 transition-colors">
                    <span>{selectedColor}</span>
                    <button
                      type="button"
                      aria-label="Удалить фильтр цвета"
                      onClick={() => setSelectedColor('')}
                      className="hover:bg-gray-200/80 rounded-full p-0.5 text-gray-400 hover:text-black transition-colors cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </span>
                )}
                {selectedSize && (
                  <span className="inline-flex items-center gap-1.5 pl-3 pr-1.5 py-1 rounded-full text-xs font-medium bg-gray-100/90 text-gray-800 border border-gray-200/80 transition-colors">
                    <span>Размер: {selectedSize}</span>
                    <button
                      type="button"
                      aria-label="Удалить фильтр размера"
                      onClick={() => setSelectedSize('')}
                      className="hover:bg-gray-200/80 rounded-full p-0.5 text-gray-400 hover:text-black transition-colors cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </span>
                )}
                {onlySelectedFilter && (
                  <span className="inline-flex items-center gap-1.5 pl-3 pr-1.5 py-1 rounded-full text-xs font-medium bg-gray-100/90 text-gray-800 border border-gray-200/80 transition-colors">
                    <span>Только добавленные</span>
                    <button
                      type="button"
                      aria-label="Удалить фильтр только добавленные"
                      onClick={() => setOnlySelectedFilter(false)}
                      className="hover:bg-gray-200/80 rounded-full p-0.5 text-gray-400 hover:text-black transition-colors cursor-pointer"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </span>
                )}
                <button
                  type="button"
                  onClick={() => {
                    setSelectedCategory('');
                    setSelectedColor('');
                    setSelectedSize('');
                    setOnlySelectedFilter(false);
                  }}
                  className="text-xs font-semibold text-gray-500 hover:text-red-600 transition-colors py-1 px-2.5 rounded-lg hover:bg-red-50/50 cursor-pointer ml-1"
                >
                  Сбросить фильтры
                </button>
              </div>
            )}

            {/* Product List */}
            {loading ? (
              <div className="py-16 text-center text-sm font-medium text-gray-400">Загрузка товаров...</div>
            ) : filteredProductsList.length === 0 ? (
              <div className="py-16 text-center text-sm font-medium text-gray-500 bg-gray-50/60 rounded-xl border border-gray-100">
                {onlySelectedFilter && !selectedCategory && !selectedColor && !selectedSize && !searchQuery.trim() && declaredTotal === 0
                  ? 'В поставку пока не добавлено ни одного товара.'
                  : 'Нет товаров, подходящих под выбранные фильтры.'}
              </div>
            ) : (
              <div className="space-y-3 pt-2">
                {filteredProductsList.map(product => {
                  const isExpanded = expandedProductIds.has(product.id);
                  const productTotalQty = (product.variants || []).reduce((sum: number, v: any) => {
                    const found = selectedItems.find(i => i.variantId === v.id);
                    return sum + (found?.quantity || 0);
                  }, 0);
                  const hasSelected = productTotalQty > 0;
                  const variantsCount = product.variants?.length || 0;
                  const productCategory = getCategoryLabel(product.categoryId, product);

                  return (
                    <div
                      key={product.id}
                      className={`border rounded-xl overflow-hidden transition-all bg-white ${
                        hasSelected
                          ? 'border-gray-900/30 bg-gray-50/20 shadow-xs'
                          : 'border-gray-200/90 hover:border-gray-300'
                      }`}
                    >
                      {/* Collapsed Product Header */}
                      <div
                        onClick={() => toggleProductExpansion(product.id)}
                        className={`p-3.5 sm:p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 cursor-pointer select-none transition-colors ${
                          hasSelected ? 'bg-gray-50/50 hover:bg-gray-50/80' : 'hover:bg-gray-50/40'
                        }`}
                      >
                        <div className="flex items-center min-w-0">
                          <div className="w-12 h-12 rounded-lg bg-gray-100 border border-gray-200/80 overflow-hidden flex-shrink-0 flex items-center justify-center">
                            {product.mainImage ? (
                              <img src={product.mainImage} alt={product.title} className="w-full h-full object-cover" />
                            ) : (
                              <ImageIcon className="w-5 h-5 text-gray-300 stroke-[1.5]" />
                            )}
                          </div>
                          <div className="ml-3.5 min-w-0">
                            <h4 className="text-sm sm:text-base font-bold text-gray-900 truncate leading-snug">
                              {product.title}
                            </h4>
                            <div className="flex flex-wrap items-center gap-2 mt-0.5 text-xs text-gray-500 font-medium">
                              <span>
                                {variantsCount} {getVariantsWord(variantsCount)}
                              </span>
                              {productCategory && (
                                <>
                                  <span className="text-gray-300">·</span>
                                  <span className="text-gray-500">{productCategory}</span>
                                </>
                              )}
                            </div>
                          </div>
                        </div>

                        <div className="flex items-center justify-between sm:justify-end gap-3 self-end sm:self-center shrink-0 w-full sm:w-auto">
                          {hasSelected ? (
                            <span className="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-bold bg-black text-white shadow-xs">
                              Добавлено: {productTotalQty} шт.
                            </span>
                          ) : (
                            <span className="text-xs text-gray-400 font-medium px-2 py-1">
                              Не добавлено
                            </span>
                          )}
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              toggleProductExpansion(product.id);
                            }}
                            className={`inline-flex items-center text-xs font-semibold px-3 py-1.5 rounded-lg border transition-colors cursor-pointer ${
                              isExpanded
                                ? 'bg-gray-100 text-gray-900 border-gray-300 hover:bg-gray-200'
                                : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50 hover:border-gray-300'
                            }`}
                          >
                            {isExpanded ? (
                              <>
                                <ChevronUp className="w-3.5 h-3.5 mr-1 text-gray-500" />
                                Свернуть
                              </>
                            ) : (
                              <>
                                <ChevronDown className="w-3.5 h-3.5 mr-1 text-gray-500" />
                                Развернуть
                              </>
                            )}
                          </button>
                        </div>
                      </div>

                      {/* Expanded Variant Rows / Matrix */}
                      {isExpanded && (
                        <ProductVariantAdaptiveEditor
                          product={product}
                          selectedItems={selectedItems}
                          onQuantityChange={handleItemQuantityChange}
                        />
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Bottom Action Area */}
          <div className="sticky bottom-4 z-20 mt-4 p-4 rounded-2xl bg-white/95 backdrop-blur-md border border-gray-200/90 shadow-lg flex flex-col sm:flex-row items-center justify-between gap-3">
            <div className="text-xs sm:text-sm text-gray-500 flex items-center gap-2">
              {declaredTotal > 0 ? (
                <>
                  <span className="font-semibold text-gray-900">
                    Выбрано:{' '}
                    <span className="font-bold text-black">
                      {selectedProductsCount} {selectedProductsCount === 1 ? 'товар' : 'товаров'}
                    </span>
                  </span>
                  <span className="text-gray-300">·</span>
                  <span className="font-semibold text-gray-900">
                    Всего <span className="font-bold text-black">{declaredTotal} шт.</span>
                  </span>
                </>
              ) : (
                <span className="text-gray-400">
                  Укажите количество товаров для продолжения
                </span>
              )}
            </div>
            <button
              type="button"
              onClick={() => setStep(2)}
              disabled={declaredTotal <= 0}
              className="w-full sm:w-auto px-6 py-2.5 h-10 bg-black text-white text-sm font-bold rounded-xl disabled:bg-gray-200 disabled:text-gray-400 disabled:cursor-not-allowed hover:bg-gray-800 transition-colors shrink-0 cursor-pointer shadow-xs"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 2: Handoff */}
      {step === 2 && (
        <div className="space-y-4 max-w-3xl mx-auto">
          <div className="bg-white shadow-xs border border-gray-200/90 rounded-2xl p-6 sm:p-8">
            <h3 className="text-xl font-bold text-gray-900 mb-6 tracking-tight">Доставка на склад ZAMK</h3>

            <div className="space-y-6">
              <div className="p-4 border border-black bg-gray-50/60 rounded-xl flex items-start">
                <Truck className="w-5 h-5 text-black mt-0.5 mr-3.5 shrink-0" />
                <div>
                  <span className="block text-sm font-bold text-gray-900">Транспортная компания</span>
                  <span className="block text-xs text-gray-500 mt-0.5">Передайте поставку перевозчику для доставки на склад ZAMK.</span>
                </div>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-xs font-bold text-gray-700 uppercase tracking-wider mb-1.5">
                    Транспортная компания <span className="text-red-500">*</span>
                  </label>
                  <div className="border border-black bg-white rounded-xl p-3.5 flex items-center justify-between shadow-xs">
                    <div className="flex items-center">
                      <div className="w-8 h-8 rounded-lg bg-green-600 flex items-center justify-center text-white font-black text-xs tracking-wider mr-3 shrink-0">
                        СДЭК
                      </div>
                      <div>
                        <p className="font-bold text-gray-900 text-sm">СДЭК</p>
                        <p className="text-[11px] text-gray-500">Доставка до склада ZAMK</p>
                      </div>
                    </div>
                    <span className="px-2.5 py-0.5 bg-black text-white text-[11px] font-bold rounded-full">
                      Выбрано
                    </span>
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-bold text-gray-700 uppercase tracking-wider mb-1.5">
                    Трек-номер отправления <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={trackingNumber}
                    onChange={e => {
                      setTrackingNumber(e.target.value);
                      if (e.target.value.trim()) setTrackingError(null);
                    }}
                    placeholder="Например: 121212123241"
                    className={`block w-full h-10 px-3 text-sm font-mono rounded-xl shadow-xs focus:border-black focus:ring-black ${
                      trackingError ? 'border-red-500 bg-red-50/20' : 'border-gray-200'
                    }`}
                  />
                  {trackingError && <p className="mt-1 text-xs font-semibold text-red-600">{trackingError}</p>}
                </div>
              </div>

              <div className="pt-6 border-t border-gray-100">
                <span className="block text-xs font-bold text-gray-400 uppercase tracking-wider mb-2.5">Получатель</span>
                <div className="bg-gray-50/70 rounded-xl p-4 border border-gray-100">
                  <p className="text-sm font-bold text-gray-900">Склад ZAMK</p>
                  <p className="text-xs text-gray-500 mt-0.5">Ожидает доставки вашей транспортной компанией.</p>
                </div>
              </div>
            </div>
          </div>

          <div className="sticky bottom-4 z-20 mt-4 p-4 rounded-2xl bg-white/95 backdrop-blur-md border border-gray-200/90 shadow-lg flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={() => setStep(1)}
              className="px-5 py-2.5 h-10 border border-gray-200 text-gray-700 text-sm font-semibold rounded-xl hover:bg-gray-50 transition-colors cursor-pointer"
            >
              Назад
            </button>
            <button
              type="button"
              onClick={handleStep2Continue}
              className="px-6 py-2.5 h-10 bg-black text-white text-sm font-bold rounded-xl hover:bg-gray-800 transition-colors cursor-pointer shadow-xs"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: Review */}
      {step === 3 && (
        <div className="space-y-4 max-w-4xl mx-auto">
          <div className="bg-white shadow-xs border border-gray-200/90 rounded-2xl p-6 sm:p-8">
            <h3 className="text-xl font-bold text-gray-900 mb-6 tracking-tight">Проверьте поставку</h3>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
              <div className="bg-gray-50/80 p-4 rounded-xl border border-gray-100">
                <span className="block text-[11px] font-bold text-gray-400 uppercase tracking-wider mb-1">Товары</span>
                <p className="text-base font-semibold text-gray-900">
                  {selectedItems.length} SKU · <span className="font-bold text-black">{declaredTotal} единиц</span>
                </p>
              </div>

              <div className="bg-gray-50/80 p-4 rounded-xl border border-gray-100">
                <span className="block text-[11px] font-bold text-gray-400 uppercase tracking-wider mb-1">Упаковка</span>
                <p className="text-base font-semibold text-gray-900">1 грузоместо</p>
              </div>

              <div className="bg-gray-50/80 p-4 rounded-xl border border-gray-100">
                <span className="block text-[11px] font-bold text-gray-400 uppercase tracking-wider mb-1">Доставка</span>
                <p className="text-base font-semibold text-gray-900">
                  {carrierCompany || 'Транспортная компания'}
                </p>
                {trackingNumber && (
                  <p className="text-xs font-mono text-gray-500 mt-0.5">Трек: {trackingNumber}</p>
                )}
              </div>
            </div>

            <div className="border-t border-gray-100 pt-6">
              <h4 className="text-base font-bold text-gray-900 mb-4">Спецификация</h4>
              <div className="bg-white border border-gray-200/90 rounded-xl overflow-hidden shadow-xs">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50/80">
                    <tr>
                      <th scope="col" className="px-5 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                        Товар и вариант
                      </th>
                      <th scope="col" className="px-5 py-2.5 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">
                        SKU
                      </th>
                      <th scope="col" className="px-5 py-2.5 text-right text-xs font-bold text-gray-500 uppercase tracking-wider">
                        Количество
                      </th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-100">
                    {selectedItems.map(i => (
                      <tr key={i.variantId}>
                        <td className="px-5 py-3 whitespace-nowrap">
                          <div className="flex items-center">
                            {i.imageUrl ? (
                              <div className="h-9 w-9 mr-3 rounded-lg overflow-hidden border border-gray-100 bg-gray-50 shrink-0">
                                <img src={i.imageUrl} alt="" className="h-full w-full object-cover" />
                              </div>
                            ) : (
                              <div className="h-9 w-9 mr-3 rounded-lg border border-gray-100 bg-gray-50 flex items-center justify-center shrink-0">
                                <ImageIcon className="w-4 h-4 text-gray-300 stroke-[1.5]" />
                              </div>
                            )}
                            <div>
                              <div className="text-sm font-semibold text-gray-900">{i.title}</div>
                              <div className="text-xs text-gray-500">{i.options}</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-5 py-3 whitespace-nowrap text-xs font-mono text-gray-500">
                          {i.sku || '-'}
                        </td>
                        <td className="px-5 py-3 whitespace-nowrap text-right text-sm font-bold text-gray-900">
                          {i.quantity} шт
                        </td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot className="bg-gray-50/80 border-t border-gray-200">
                    <tr>
                      <td colSpan={2} className="px-5 py-3 text-right text-xs font-bold text-gray-600 uppercase">
                        Всего:
                      </td>
                      <td className="px-5 py-3 text-right text-base font-black text-gray-900">
                        {declaredTotal} единиц
                      </td>
                    </tr>
                  </tfoot>
                </table>
              </div>
            </div>
          </div>

          <div className="sticky bottom-4 z-20 mt-4 p-4 rounded-2xl bg-white/95 backdrop-blur-md border border-gray-200/90 shadow-lg flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={() => setStep(2)}
              className="px-5 py-2.5 h-10 border border-gray-200 text-gray-700 text-sm font-semibold rounded-xl hover:bg-gray-50 transition-colors cursor-pointer"
            >
              Назад
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={submitting}
              className="px-6 py-2.5 h-10 bg-black text-white text-sm font-bold rounded-xl hover:bg-gray-800 disabled:opacity-50 transition-colors cursor-pointer shadow-xs"
            >
              {submitting ? 'Создание...' : 'Создать поставку'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

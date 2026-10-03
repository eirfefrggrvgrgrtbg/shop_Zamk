import React, { useEffect, useState, useCallback } from 'react';
import {
  getSellerPromotions,
  createSellerPromotion,
  updateSellerPromotion,
  getSellerProducts,
  getSellerProductsPaginated,
  getSellerCategories,
  type SellerCategory,
} from '@zamk/api-client/src/seller';
import type {
  SellerPromotion,
  SellerPromoDiscountType,
  CreateSellerPromotionRequest,
  UpdateSellerPromotionRequest,
  SellerProduct,
  SellerPromoProductScope,
  GetSellerProductsParams,
  SellerProductListResponse,
} from '@zamk/api-client/src/types';
import {
  Tag,
  Plus,
  AlertCircle,
  Pause,
  Play,
  Edit2,
  CheckCircle2,
  Clock,
  Ban,
  Archive,
  X,
  Search,
  Check,
  Package,
  ChevronRight,
  ChevronDown,
} from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerModal } from '../components/SellerSurface';
import { SellerDateTimePicker } from '../components/SellerDateTimePicker';
import { cn, parseRubToCentsExact } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const fetchSellerProductsList = async (
  params?: GetSellerProductsParams
): Promise<SellerProductListResponse> => {
  if (typeof getSellerProductsPaginated === 'function') {
    return await getSellerProductsPaginated(params);
  }
  const items = await getSellerProducts(params);
  const safeItems = Array.isArray(items) ? items : [];
  return { items: safeItems, totalCount: safeItems.length };
};

const STATUS_CONFIG: Record<
  string,
  { label: string; badgeClass: string; icon: React.ComponentType<{ className?: string }> }
> = {
  active: {
    label: 'Активен',
    badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    icon: CheckCircle2,
  },
  scheduled: {
    label: 'Запланирован',
    badgeClass: 'bg-blue-50 text-blue-700 border-blue-200',
    icon: Clock,
  },
  paused: {
    label: 'На паузе',
    badgeClass: 'bg-amber-50 text-amber-700 border-amber-200',
    icon: Pause,
  },
  expired: {
    label: 'Истёк',
    badgeClass: 'bg-gray-100 text-gray-700 border-gray-200',
    icon: Archive,
  },
  exhausted: {
    label: 'Исчерпан',
    badgeClass: 'bg-purple-50 text-purple-700 border-purple-200',
    icon: Ban,
  },
};

type SelectorMode = 'INCLUDE' | 'EXCLUDE' | null;

interface CategoryTreeNode {
  category: SellerCategory;
  children: CategoryTreeNode[];
  level: number;
}

const buildCategoryTree = (
  categoriesList: SellerCategory[],
  lookup: Record<string, SellerCategory>
): CategoryTreeNode[] => {
  const childrenMap = new Map<string, SellerCategory[]>();
  const rootItems: SellerCategory[] = [];

  for (const cat of categoriesList) {
    if (cat.parentId && lookup[cat.parentId]) {
      const existing = childrenMap.get(cat.parentId) || [];
      existing.push(cat);
      childrenMap.set(cat.parentId, existing);
    } else {
      rootItems.push(cat);
    }
  }

  const sortCats = (a: SellerCategory, b: SellerCategory) => {
    if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder;
    return a.name.localeCompare(b.name, 'ru');
  };

  rootItems.sort(sortCats);
  childrenMap.forEach((list) => list.sort(sortCats));

  const buildNodes = (list: SellerCategory[], level: number): CategoryTreeNode[] => {
    return list.map((cat) => {
      const kids = childrenMap.get(cat.id) || [];
      return {
        category: cat,
        children: buildNodes(kids, level + 1),
        level,
      };
    });
  };

  return buildNodes(rootItems, 0);
};

const getCategoryAncestors = (
  catId: string,
  lookup: Record<string, SellerCategory>
): string[] => {
  const ancestors: string[] = [];
  let cur = lookup[catId]?.parentId ? lookup[lookup[catId].parentId!] : undefined;
  while (cur) {
    ancestors.push(cur.id);
    cur = cur.parentId ? lookup[cur.parentId] : undefined;
  }
  return ancestors;
};

const isAncestorSelected = (
  cat: SellerCategory,
  selectedIds: string[],
  lookup: Record<string, SellerCategory>
): boolean => {
  let cur = cat.parentId ? lookup[cat.parentId] : undefined;
  while (cur) {
    if (selectedIds.includes(cur.id)) return true;
    cur = cur.parentId ? lookup[cur.parentId] : undefined;
  }
  return false;
};

export function SellerPromotions() {
  const [promotions, setPromotions] = useState<SellerPromotion[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  // Drawer / Modal states
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [editPromo, setEditPromo] = useState<SellerPromotion | null>(null);

  // Products state & lookup dictionary for persistent badge metadata
  const [productLookup, setProductLookup] = useState<Record<string, SellerProduct>>({});
  const [sellerProducts, setSellerProducts] = useState<SellerProduct[]>([]);

  // Categories state & lookup dictionary
  const [categories, setCategories] = useState<SellerCategory[]>([]);
  const [categoryLookup, setCategoryLookup] = useState<Record<string, SellerCategory>>({});
  const [isCategoriesLoading, setIsCategoriesLoading] = useState(false);

  // Advanced Rules Form State
  const [createScope, setCreateScope] = useState<SellerPromoProductScope>('ENTIRE_STORE');
  const [createIncludedProductIds, setCreateIncludedProductIds] = useState<string[]>([]);
  const [createExcludedProductIds, setCreateExcludedProductIds] = useState<string[]>([]);
  const [createIncludedCategoryIds, setCreateIncludedCategoryIds] = useState<string[]>([]);
  const [createExcludedCategoryIds, setCreateExcludedCategoryIds] = useState<string[]>([]);
  const [createMaxDiscountRub, setCreateMaxDiscountRub] = useState('');

  // Catalog Selector Modal State
  const [selectorMode, setSelectorMode] = useState<SelectorMode>(null);
  const [draftProductIds, setDraftProductIds] = useState<string[]>([]);
  const [selectorSearchQuery, setSelectorSearchQuery] = useState('');
  const [selectorDebouncedQuery, setSelectorDebouncedQuery] = useState('');
  const [selectorProducts, setSelectorProducts] = useState<SellerProduct[]>([]);
  const [selectorTotalCount, setSelectorTotalCount] = useState(0);
  const [selectorPage, setSelectorPage] = useState(1);
  const [isSelectorLoading, setIsSelectorLoading] = useState(false);
  const [isSelectorLoadingMore, setIsSelectorLoadingMore] = useState(false);
  const [selectorError, setSelectorError] = useState('');

  // Category Selector Modal State
  const [categorySelectorMode, setCategorySelectorMode] = useState<SelectorMode>(null);
  const [draftCategoryIds, setDraftCategoryIds] = useState<string[]>([]);
  const [categorySearchQuery, setCategorySearchQuery] = useState('');
  const [categorySelectorError, setCategorySelectorError] = useState('');
  const [expandedCategoryIds, setExpandedCategoryIds] = useState<Set<string>>(new Set());

  // Create Form State
  const [createCode, setCreateCode] = useState('');
  const [createDiscountType, setCreateDiscountType] = useState<SellerPromoDiscountType>('percent');
  const [createDiscountPercent, setCreateDiscountPercent] = useState('');
  const [createDiscountFixedRub, setCreateDiscountFixedRub] = useState('');
  const [createMinOrderSubtotalRub, setCreateMinOrderSubtotalRub] = useState('');
  const [createMinEligibleQuantity, setCreateMinEligibleQuantity] = useState('');
  const [createFirstPaidOnly, setCreateFirstPaidOnly] = useState(false);
  const [createStartsAt, setCreateStartsAt] = useState('');
  const [createEndsAt, setCreateEndsAt] = useState('');
  const [createGlobalLimit, setCreateGlobalLimit] = useState('');
  const [createPerCustomerLimit, setCreatePerCustomerLimit] = useState('1');
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createError, setCreateError] = useState('');
  const [createDateError, setCreateDateError] = useState('');

  // Edit Form State (Mutable operational fields only)
  const [editIsActive, setEditIsActive] = useState(true);
  const [editStartsAt, setEditStartsAt] = useState('');
  const [editEndsAt, setEditEndsAt] = useState('');
  const [editGlobalLimit, setEditGlobalLimit] = useState('');
  const [editPerCustomerLimit, setEditPerCustomerLimit] = useState('1');
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState('');
  const [editDateError, setEditDateError] = useState('');

  useEffect(() => {
    const timer = setTimeout(() => {
      setSelectorDebouncedQuery(selectorSearchQuery);
    }, 200);
    return () => clearTimeout(timer);
  }, [selectorSearchQuery]);

  const loadSellerProducts = useCallback(async () => {
    try {
      const res = await fetchSellerProductsList({ limit: 50 });
      setSellerProducts(res.items || []);
      setProductLookup((prev) => {
        const next = { ...prev };
        (res.items || []).forEach((p) => {
          next[p.id] = p;
        });
        return next;
      });
    } catch {
      // Non-blocking
    }
  }, []);

  const loadSelectorProducts = useCallback(async (query: string, page = 1) => {
    if (page === 1) {
      setIsSelectorLoading(true);
    } else {
      setIsSelectorLoadingMore(true);
    }
    setSelectorError('');
    try {
      const res = await fetchSellerProductsList({
        q: query.trim() || undefined,
        page,
        limit: 20,
      });
      setProductLookup((prev) => {
        const next = { ...prev };
        (res.items || []).forEach((p) => {
          next[p.id] = p;
        });
        return next;
      });
      setSelectorProducts((prev) => (page === 1 ? res.items || [] : [...prev, ...(res.items || [])]));
      setSelectorTotalCount(res.totalCount);
      setSelectorPage(page);
    } catch {
      setSelectorError('Не удалось загрузить список товаров');
    } finally {
      setIsSelectorLoading(false);
      setIsSelectorLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    if (isCreateOpen && sellerProducts.length === 0) {
      loadSellerProducts();
    }
  }, [isCreateOpen, sellerProducts.length, loadSellerProducts]);

  const loadCategories = useCallback(async () => {
    setIsCategoriesLoading(true);
    setCategorySelectorError('');
    try {
      const list = await getSellerCategories();
      const safeList = Array.isArray(list) ? list : [];
      setCategories(safeList);
      const lookup: Record<string, SellerCategory> = {};
      safeList.forEach((c) => {
        lookup[c.id] = c;
      });
      setCategoryLookup(lookup);
    } catch {
      setCategorySelectorError('Не удалось загрузить категории');
    } finally {
      setIsCategoriesLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isCreateOpen && categories.length === 0) {
      loadCategories();
    }
  }, [isCreateOpen, categories.length, loadCategories]);

  useEffect(() => {
    if (selectorMode) {
      loadSelectorProducts(selectorDebouncedQuery, 1);
    }
  }, [selectorDebouncedQuery, selectorMode, loadSelectorProducts]);

  useEffect(() => {
    if (createStartsAt && createEndsAt) {
      if (new Date(createEndsAt) <= new Date(createStartsAt)) {
        setCreateDateError('Дата окончания должна быть позже даты начала.');
        return;
      }
    }
    setCreateDateError('');
  }, [createStartsAt, createEndsAt]);

  useEffect(() => {
    if (editStartsAt && editEndsAt) {
      if (new Date(editEndsAt) <= new Date(editStartsAt)) {
        setEditDateError('Дата окончания должна быть позже даты начала.');
        return;
      }
    }
    setEditDateError('');
  }, [editStartsAt, editEndsAt]);

  const fetchPromotions = useCallback(async () => {
    try {
      const data = await getSellerPromotions();
      setPromotions(data.items || []);
      setError('');
    } catch (err: any) {
      if (err.status === 403) {
        setError('Недостаточно прав для управления промокодами.');
      } else {
        setError('Не удалось загрузить список промокодов.');
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPromotions();
  }, [fetchPromotions]);

  const openSelector = (mode: 'INCLUDE' | 'EXCLUDE') => {
    setSelectorMode(mode);
    if (mode === 'INCLUDE') {
      setDraftProductIds([...createIncludedProductIds]);
    } else {
      setDraftProductIds([...createExcludedProductIds]);
    }
    setSelectorSearchQuery('');
    setSelectorDebouncedQuery('');
    setSelectorError('');
    setSelectorPage(1);
    if (sellerProducts.length > 0) {
      setSelectorProducts(sellerProducts);
      setSelectorTotalCount(sellerProducts.length);
    } else {
      loadSelectorProducts('', 1);
    }
  };

  const closeSelector = () => {
    setSelectorMode(null);
    setDraftProductIds([]);
    setSelectorSearchQuery('');
    setSelectorDebouncedQuery('');
    setSelectorError('');
  };

  const applySelector = () => {
    if (selectorMode === 'INCLUDE') {
      setCreateIncludedProductIds(draftProductIds);
    } else if (selectorMode === 'EXCLUDE') {
      setCreateExcludedProductIds(draftProductIds);
    }
    closeSelector();
  };

  const handleToggleDraft = (productId: string, product: SellerProduct) => {
    setProductLookup((prev) => ({ ...prev, [productId]: product }));
    setDraftProductIds((prev) => {
      if (prev.includes(productId)) {
        return prev.filter((id) => id !== productId);
      } else {
        return [...prev, productId];
      }
    });
  };

  const openCategorySelector = (mode: 'INCLUDE' | 'EXCLUDE') => {
    setCategorySelectorMode(mode);
    const initialDraft = mode === 'INCLUDE' ? [...createIncludedCategoryIds] : [...createExcludedCategoryIds];
    setDraftCategoryIds(initialDraft);
    setCategorySearchQuery('');
    setCategorySelectorError('');

    const expanded = new Set<string>();
    const initialIdsToExpand = [
      ...initialDraft,
      ...(mode === 'INCLUDE' ? createExcludedCategoryIds : createIncludedCategoryIds),
    ];

    initialIdsToExpand.forEach((id) => {
      const ancestors = getCategoryAncestors(id, categoryLookup);
      ancestors.forEach((ancId) => expanded.add(ancId));
    });

    setExpandedCategoryIds(expanded);

    if (categories.length === 0) {
      loadCategories();
    }
  };

  useEffect(() => {
    if (categorySelectorMode && categories.length > 0) {
      setExpandedCategoryIds((prev) => {
        const next = new Set(prev);
        const idsToExpand = [
          ...draftCategoryIds,
          ...(categorySelectorMode === 'INCLUDE' ? createExcludedCategoryIds : createIncludedCategoryIds),
        ];
        idsToExpand.forEach((id) => {
          const ancestors = getCategoryAncestors(id, categoryLookup);
          ancestors.forEach((ancId) => next.add(ancId));
        });
        return next;
      });
    }
  }, [categorySelectorMode, categories, categoryLookup]);

  const closeCategorySelector = () => {
    setCategorySelectorMode(null);
    setDraftCategoryIds([]);
    setCategorySearchQuery('');
    setCategorySelectorError('');
  };

  const applyCategorySelector = () => {
    if (categorySelectorMode === 'INCLUDE') {
      setCreateIncludedCategoryIds(draftCategoryIds);
    } else if (categorySelectorMode === 'EXCLUDE') {
      setCreateExcludedCategoryIds(draftCategoryIds);
    }
    closeCategorySelector();
  };

  const handleToggleCategoryDraft = (categoryId: string) => {
    setDraftCategoryIds((prev) => {
      if (prev.includes(categoryId)) {
        return prev.filter((id) => id !== categoryId);
      } else {
        return [...prev, categoryId];
      }
    });
    const ancestors = getCategoryAncestors(categoryId, categoryLookup);
    if (ancestors.length > 0) {
      setExpandedCategoryIds((prev) => {
        const next = new Set(prev);
        ancestors.forEach((aId) => next.add(aId));
        return next;
      });
    }
  };

  const toggleExpandCategory = (categoryId: string, e?: React.MouseEvent) => {
    if (e) {
      e.stopPropagation();
    }
    setExpandedCategoryIds((prev) => {
      const next = new Set(prev);
      if (next.has(categoryId)) {
        next.delete(categoryId);
      } else {
        next.add(categoryId);
      }
      return next;
    });
  };

  const resetCreateForm = () => {
    setCreateCode('');
    setCreateDiscountType('percent');
    setCreateDiscountPercent('');
    setCreateDiscountFixedRub('');
    setCreateMinOrderSubtotalRub('');
    setCreateMinEligibleQuantity('');
    setCreateFirstPaidOnly(false);
    setCreateStartsAt('');
    setCreateEndsAt('');
    setCreateGlobalLimit('');
    setCreatePerCustomerLimit('1');
    setCreateScope('ENTIRE_STORE');
    setCreateIncludedProductIds([]);
    setCreateExcludedProductIds([]);
    setCreateIncludedCategoryIds([]);
    setCreateExcludedCategoryIds([]);
    setCreateMaxDiscountRub('');
    closeSelector();
    closeCategorySelector();
    setCreateError('');
    setCreateDateError('');
  };

  const openEditDrawer = (promo: SellerPromotion) => {
    setEditPromo(promo);
    setEditIsActive(promo.isActive);
    setEditStartsAt(promo.startsAt ? promo.startsAt.substring(0, 16) : '');
    setEditEndsAt(promo.endsAt ? promo.endsAt.substring(0, 16) : '');
    setEditGlobalLimit(promo.globalUsageLimit != null ? String(promo.globalUsageLimit) : '');
    setEditPerCustomerLimit(String(promo.perCustomerUsageLimit || 1));
    setEditError('');
    setEditDateError('');
  };

  const scrollToAndFocus = (testIdOrId: string) => {
    const el =
      document.querySelector<HTMLElement>(`[data-testid="${testIdOrId}"]`) ||
      document.getElementById(testIdOrId);
    if (el) {
      if (typeof el.scrollIntoView === 'function') {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      if (typeof el.focus === 'function') {
        el.focus();
      }
    }
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateSubmitting(true);
    setCreateError('');

    try {
      const codeClean = createCode.trim().toUpperCase();
      if (!codeClean) {
        scrollToAndFocus('input-promo-code');
        throw new Error('Укажите код промокода.');
      }

      const req: CreateSellerPromotionRequest = {
        code: codeClean,
        discountType: createDiscountType,
        productScope: createScope,
      };

      if (createDiscountType === 'percent') {
        const pct = parseFloat(createDiscountPercent);
        if (isNaN(pct) || pct <= 0 || pct > 100) {
          scrollToAndFocus('input-discount-percent');
          throw new Error('Скидка должна быть от 1% до 100%.');
        }
        req.discountValueBps = Math.round(pct * 100);
      } else {
        const fixedRub = parseFloat(createDiscountFixedRub);
        if (isNaN(fixedRub) || fixedRub <= 0) {
          scrollToAndFocus('input-discount-fixed');
          throw new Error('Укажите фиксированную сумму скидки в рублях.');
        }
        req.discountValueFixedCents = Math.round(fixedRub * 100);
      }

      if (createMinOrderSubtotalRub) {
        const minRub = parseFloat(createMinOrderSubtotalRub);
        if (isNaN(minRub) || minRub < 0) {
          scrollToAndFocus('input-min-order');
          throw new Error('Минимальная сумма заказа должна быть положительным числом.');
        }
        if (minRub > 0) {
          req.minOrderSubtotalCents = Math.round(minRub * 100);
        }
      }

      if (createMinEligibleQuantity) {
        const trimmed = createMinEligibleQuantity.trim();
        if (trimmed !== '') {
          const qty = Number(trimmed);
          if (!/^\d+$/.test(trimmed) || !Number.isInteger(qty) || qty <= 0) {
            scrollToAndFocus('input-min-quantity');
            throw new Error('Минимальное количество товаров должно быть целым положительным числом.');
          }
          req.minEligibleQuantity = qty;
        }
      }

      if (createMaxDiscountRub.trim()) {
        try {
          req.maxDiscountCents = parseRubToCentsExact(createMaxDiscountRub);
        } catch (err: any) {
          scrollToAndFocus('input-max-discount');
          throw err;
        }
      }

      if (createScope === 'SELECTED_PRODUCTS') {
        if (createIncludedProductIds.length === 0) {
          scrollToAndFocus('btn-edit-included');
          throw new Error('Выберите хотя бы один товар для области действия «На выбранные товары».');
        }
        req.includedProductIds = createIncludedProductIds;
      } else if (createScope === 'SELECTED_CATEGORIES') {
        if (createIncludedCategoryIds.length === 0) {
          scrollToAndFocus('btn-edit-included-categories');
          throw new Error('Выберите хотя бы одну категорию для области действия «По категориям».');
        }
        req.includedCategoryIds = createIncludedCategoryIds;
        if (createExcludedCategoryIds.length > 0) {
          req.excludedCategoryIds = createExcludedCategoryIds;
        }
      } else if (createScope === 'ENTIRE_STORE') {
        if (createExcludedCategoryIds.length > 0) {
          req.excludedCategoryIds = createExcludedCategoryIds;
        }
      }

      if (createExcludedProductIds.length > 0) {
        req.excludedProductIds = createExcludedProductIds;
      }

      req.firstPaidOrderOnly = createFirstPaidOnly;

      if (createStartsAt && createEndsAt) {
        if (new Date(createEndsAt) <= new Date(createStartsAt)) {
          scrollToAndFocus('input-ends-at');
          throw new Error('Дата окончания должна быть позже даты начала.');
        }
      }

      if (createStartsAt) {
        req.startsAt = new Date(createStartsAt).toISOString();
      }
      if (createEndsAt) {
        req.endsAt = new Date(createEndsAt).toISOString();
      }

      if (createGlobalLimit) {
        const gl = parseInt(createGlobalLimit, 10);
        if (isNaN(gl) || gl <= 0) {
          scrollToAndFocus('input-global-limit');
          throw new Error('Общий лимит должен быть положительным числом.');
        }
        req.globalUsageLimit = gl;
      }

      if (createPerCustomerLimit) {
        const pcl = parseInt(createPerCustomerLimit, 10);
        if (isNaN(pcl) || pcl <= 0) {
          scrollToAndFocus('input-per-customer-limit');
          throw new Error('Лимит на клиента должен быть положительным числом.');
        }
        req.perCustomerUsageLimit = pcl;
      }

      await createSellerPromotion(req);
      setIsCreateOpen(false);
      resetCreateForm();
      await fetchPromotions();
    } catch (err: any) {
      if (err.data?.code === 'promo_code_duplicate' || err.code === 'promo_code_duplicate') {
        setCreateError('Промокод с таким кодом уже существует. Выберите другой код.');
      } else if (err.data?.code === 'product_not_owned_by_seller' || err.code === 'product_not_owned_by_seller') {
        setCreateError('Один или несколько выбранных товаров не принадлежат вашему магазину.');
      } else if (err.data?.code === 'product_conflict' || err.code === 'product_conflict') {
        setCreateError('Товар не может одновременно находиться в списке включений и исключений.');
      } else if (err.data?.code === 'category_conflict' || err.code === 'category_conflict') {
        setCreateError('Категория не может одновременно находиться в списке включений и исключений.');
      } else if (err.data?.code === 'invalid_category' || err.code === 'invalid_category') {
        setCreateError('Одна или несколько категорий не найдены или неактивны.');
      } else if (err.data?.code === 'category_requires_include' || err.code === 'category_requires_include') {
        setCreateError('Для области действия «По категориям» необходимо выбрать хотя бы одну категорию.');
      } else if (err.data?.code === 'invalid_product_scope' || err.code === 'invalid_product_scope') {
        setCreateError('Некорректная область действия промокода.');
      } else if (err.data?.code === 'invalid_max_discount' || err.code === 'invalid_max_discount') {
        setCreateError('Максимальная скидка должна быть положительным числом.');
      } else {
        setCreateError(err.message || err.data?.message || 'Не удалось создать промокод.');
      }
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editPromo) return;
    setEditSubmitting(true);
    setEditError('');

    try {
      const req: UpdateSellerPromotionRequest = {
        isActive: editIsActive,
      };

      if (editStartsAt && editEndsAt) {
        if (new Date(editEndsAt) <= new Date(editStartsAt)) {
          throw new Error('Дата окончания должна быть позже даты начала.');
        }
      }

      if (editStartsAt) {
        req.startsAt = new Date(editStartsAt).toISOString();
      } else {
        req.startsAt = null;
      }

      if (editEndsAt) {
        req.endsAt = new Date(editEndsAt).toISOString();
      } else {
        req.endsAt = null;
      }

      if (editGlobalLimit) {
        const gl = parseInt(editGlobalLimit, 10);
        if (isNaN(gl) || gl <= 0) {
          throw new Error('Общий лимит должен быть положительным числом.');
        }
        req.globalUsageLimit = gl;
      } else {
        req.globalUsageLimit = null;
      }

      if (editPerCustomerLimit) {
        const pcl = parseInt(editPerCustomerLimit, 10);
        if (isNaN(pcl) || pcl <= 0) {
          throw new Error('Лимит на клиента должен быть положительным числом.');
        }
        req.perCustomerUsageLimit = pcl;
      }

      await updateSellerPromotion(editPromo.id, req);
      setEditPromo(null);
      await fetchPromotions();
    } catch (err: any) {
      if (err.data?.code === 'global_limit_below_usage' || err.code === 'global_limit_below_usage') {
        setEditError('Общий лимит не может быть меньше уже зафиксированного использования.');
      } else if (
        err.data?.code === 'customer_limit_below_usage' ||
        err.code === 'customer_limit_below_usage'
      ) {
        setEditError('Лимит на клиента не может быть меньше уже зафиксированного использования покупателя.');
      } else if (err.data?.code === 'invalid_dates' || err.code === 'invalid_dates') {
        setEditError('Дата окончания должна быть позже даты начала.');
      } else {
        setEditError(err.message || err.data?.message || 'Не удалось обновить промокод.');
      }
    } finally {
      setEditSubmitting(false);
    }
  };

  const handleQuickToggleActive = async (promo: SellerPromotion) => {
    try {
      await updateSellerPromotion(promo.id, { isActive: !promo.isActive });
      await fetchPromotions();
    } catch (err: any) {
      setError(err.message || 'Не удалось изменить статус промокода.');
    }
  };

  if (isLoading) {
    return (
      <SellerPageFrame variant="summary">
        <SellerSurface className="py-20 flex flex-col justify-center items-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-gray-900 mb-4"></div>
          <div className="text-sm text-gray-500">Загружаем промокоды...</div>
        </SellerSurface>
      </SellerPageFrame>
    );
  }

  if (error && promotions.length === 0) {
    return (
      <SellerPageFrame variant="summary">
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 flex items-center gap-3 text-red-600">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Маркетинг"
        title="Промокоды"
        description="Собственные промокоды продавца. Скидка финансируется вами и применяется ко всем вашим товарам."
        action={
          <button
            type="button"
            onClick={() => {
              resetCreateForm();
              setIsCreateOpen(true);
            }}
            data-testid="create-promo-button"
            className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Создать промокод
          </button>
        }
      />

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 flex items-center gap-3 text-red-600 text-sm">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Promotions List / Table */}
      {promotions.length === 0 ? (
        <SellerSurface className="py-16 px-6 text-center">
          <div className="inline-flex p-4 rounded-full bg-gray-100 text-gray-500 mb-4">
            <Tag className="w-8 h-8" />
          </div>
          <h3 className="text-lg font-medium text-gray-900 mb-1">У вас пока нет промокодов</h3>
          <p className="text-sm text-gray-500 max-w-md mx-auto mb-6">
            Создайте свой первый промокод, чтобы привлечь покупателей и увеличить продажи ваших товаров.
          </p>
          <button
            type="button"
            onClick={() => {
              resetCreateForm();
              setIsCreateOpen(true);
            }}
            data-testid="create-first-promo-button"
            className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Создать первый промокод
          </button>
        </SellerSurface>
      ) : (
        <SellerSurface className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm" data-testid="promotions-table">
              <thead className="bg-gray-50 border-b border-gray-200 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3">Промокод</th>
                  <th className="px-6 py-3">Скидка</th>
                  <th className="px-6 py-3">Мин. заказ</th>
                  <th className="px-6 py-3">Использовано</th>
                  <th className="px-6 py-3">Лимиты</th>
                  <th className="px-6 py-3">Период действия</th>
                  <th className="px-6 py-3">Статус</th>
                  <th className="px-6 py-3 text-right">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {promotions.map((p) => {
                  const statusConf = STATUS_CONFIG[p.status] || STATUS_CONFIG.active;
                  const StatusIcon = statusConf.icon;

                  let discountText = '';
                  if (p.discountType === 'percent') {
                    discountText = `${p.discountValueBps / 100}%`;
                  } else {
                    discountText = currencyFormatter.format(p.discountValueFixedCents / 100);
                  }

                  const minOrderText =
                    p.minOrderSubtotalCents > 0
                      ? currencyFormatter.format(p.minOrderSubtotalCents / 100)
                      : 'Без ограничений';

                  const formatWindowDate = (dt: string | null) => {
                    if (!dt) return null;
                    return new Date(dt).toLocaleDateString('ru-RU', {
                      day: 'numeric',
                      month: 'short',
                      year: 'numeric',
                    });
                  };

                  const startsFormatted = formatWindowDate(p.startsAt);
                  const endsFormatted = formatWindowDate(p.endsAt);

                  return (
                    <tr
                      key={p.id}
                      data-testid={`promo-row-${p.code}`}
                      className="hover:bg-gray-50/50 transition-colors"
                    >
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-mono font-bold text-gray-900 tracking-wide text-base">
                            {p.code}
                          </span>
                          <span
                            data-testid={`promo-scope-badge-${p.code}`}
                            className="px-1.5 py-0.5 text-[10px] font-medium bg-gray-100 text-gray-700 border border-gray-200 rounded"
                          >
                            {p.productScope === 'SELECTED_PRODUCTS'
                              ? `${(p.includedProductIds || []).length} тов.`
                              : p.productScope === 'SELECTED_CATEGORIES'
                              ? `${(p.includedCategoryIds || []).length} кат.`
                              : ((p.excludedProductIds || []).length > 0 || (p.excludedCategoryIds || []).length > 0)
                              ? `Все товары (искл. ${(p.excludedProductIds?.length || 0) + (p.excludedCategoryIds?.length || 0)})`
                              : 'Все товары'}
                          </span>
                          {p.firstPaidOrderOnly && (
                            <span
                              title="Только для первого заказа покупателя"
                              className="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-50 text-blue-700 border border-blue-200 rounded"
                            >
                              1-й заказ
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 font-semibold text-gray-900">{discountText}</td>
                      <td className="px-6 py-4 text-gray-600">
                        <div>{minOrderText}</div>
                        {p.minEligibleQuantity && p.minEligibleQuantity > 0 && (
                          <div className="text-xs text-gray-500 font-medium">
                            от {p.minEligibleQuantity} товаров
                          </div>
                        )}
                      </td>
                      <td className="px-6 py-4 text-gray-600">
                        <span className="font-medium text-gray-900">{p.consumedUsageCount}</span>
                        {p.reservedUsageCount > 0 && (
                          <span
                            title="В оформлении / резерве"
                            className="text-xs text-amber-600 ml-1.5"
                          >
                            (+{p.reservedUsageCount} в резерве)
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500 space-y-0.5">
                        <div>
                          Общий:{' '}
                          <span className="font-medium text-gray-900">
                            {p.globalUsageLimit != null ? p.globalUsageLimit : '∞'}
                          </span>
                        </div>
                        <div>
                          На клиента:{' '}
                          <span className="font-medium text-gray-900">
                            {p.perCustomerUsageLimit || 1}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500">
                        {startsFormatted || endsFormatted ? (
                          <div className="space-y-0.5">
                            {startsFormatted && <div>С {startsFormatted}</div>}
                            {endsFormatted && <div>По {endsFormatted}</div>}
                          </div>
                        ) : (
                          <span className="text-gray-400">Бессрочно</span>
                        )}
                      </td>
                      <td className="px-6 py-4">
                        <span
                          data-testid={`promo-status-badge-${p.code}`}
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${statusConf.badgeClass}`}
                        >
                          <StatusIcon className="w-3.5 h-3.5" />
                          {statusConf.label}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => handleQuickToggleActive(p)}
                            title={p.isActive ? 'Поставить на паузу' : 'Возобновить'}
                            data-testid={`promo-toggle-active-${p.code}`}
                            className={`p-1.5 rounded-lg border text-xs font-medium transition-colors cursor-pointer ${
                              p.isActive
                                ? 'border-amber-200 text-amber-700 hover:bg-amber-50'
                                : 'border-emerald-200 text-emerald-700 hover:bg-emerald-50'
                            }`}
                          >
                            {p.isActive ? (
                              <Pause className="w-3.5 h-3.5" />
                            ) : (
                              <Play className="w-3.5 h-3.5" />
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={() => openEditDrawer(p)}
                            title="Редактировать параметры"
                            data-testid={`promo-edit-button-${p.code}`}
                            className="p-1.5 rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-100 transition-colors cursor-pointer"
                          >
                            <Edit2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </SellerSurface>
      )}

      {/* CREATE MODAL */}
      <SellerModal
        isOpen={isCreateOpen}
        onClose={() => {
          if (Boolean(categorySelectorMode) || Boolean(selectorMode)) return;
          setIsCreateOpen(false);
        }}
        title="Создание промокода"
        data-testid="create-promo-modal"
      >
        <form onSubmit={handleCreateSubmit} className="space-y-6">
          {createError && (
            <div
              data-testid="create-promo-error"
              className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2"
            >
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
              <span>{createError}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
              Код промокода <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              required
              data-testid="input-promo-code"
              value={createCode}
              onChange={(e) => setCreateCode(e.target.value.toUpperCase())}
              placeholder="НАПРИМЕР, SPRING20"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono uppercase tracking-wider focus:outline-none focus:ring-2 focus:ring-black"
            />
            <p className="text-xs text-gray-400 mt-1">
              После создания код изменить нельзя. Нечувствителен к регистру.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                Тип скидки <span className="text-red-500">*</span>
              </label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  data-testid="radio-discount-percent"
                  onClick={() => setCreateDiscountType('percent')}
                  className={`py-2 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer ${
                    createDiscountType === 'percent'
                      ? 'border-black bg-black text-white'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  Процентная (%)
                </button>
                <button
                  type="button"
                  data-testid="radio-discount-fixed"
                  onClick={() => setCreateDiscountType('fixed')}
                  className={`py-2 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer ${
                    createDiscountType === 'fixed'
                      ? 'border-black bg-black text-white'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  Фиксированная (₽)
                </button>
              </div>
            </div>

            {createDiscountType === 'percent' ? (
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Размер скидки (%) <span className="text-red-500">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    max="100"
                    step="0.01"
                    required
                    data-testid="input-discount-percent"
                    value={createDiscountPercent}
                    onChange={(e) => setCreateDiscountPercent(e.target.value)}
                    placeholder="10"
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <span className="absolute right-3 top-2 text-gray-400 text-sm">%</span>
                </div>
              </div>
            ) : (
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Размер скидки (₽) <span className="text-red-500">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    step="1"
                    required
                    data-testid="input-discount-fixed"
                    value={createDiscountFixedRub}
                    onChange={(e) => setCreateDiscountFixedRub(e.target.value)}
                    placeholder="500"
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <span className="absolute right-3 top-2 text-gray-400 text-sm">₽</span>
                </div>
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 items-start">
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Минимальная сумма заказа (₽)
              </label>
              <input
                type="number"
                min="0"
                step="1"
                data-testid="input-min-order"
                value={createMinOrderSubtotalRub}
                onChange={(e) => setCreateMinOrderSubtotalRub(e.target.value)}
                placeholder="0 (без ограничений)"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
              />
            </div>

            <div>
              <label htmlFor="create-min-quantity" className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Минимальное количество товаров
              </label>
              <input
                id="create-min-quantity"
                type="text"
                inputMode="numeric"
                data-testid="input-min-quantity"
                aria-describedby="create-min-quantity-help create-min-quantity-biz-help"
                aria-invalid={createError.includes('Минимальное количество товаров') ? 'true' : 'false'}
                value={createMinEligibleQuantity}
                onChange={(e) => setCreateMinEligibleQuantity(e.target.value)}
                placeholder="Без ограничений"
                className={cn(
                  "w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black",
                  createError.includes('Минимальное количество товаров')
                    ? "border-red-500 focus:ring-red-500"
                    : "border-gray-300"
                )}
              />
              <p id="create-min-quantity-help" className="text-[11px] text-gray-500 mt-1">
                Только целые числа от 1. Оставьте пустым, если ограничения нет.
              </p>
              <p id="create-min-quantity-biz-help" className="text-[11px] text-gray-400 mt-0.5">
                Считаются только товары, на которые действует промокод.
              </p>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
              Максимальная скидка (₽) <span className="text-gray-400 font-normal">(опционально)</span>
            </label>
            <div className="relative">
              <input
                type="text"
                inputMode="decimal"
                data-testid="input-max-discount"
                value={createMaxDiscountRub}
                onChange={(e) => setCreateMaxDiscountRub(e.target.value)}
                placeholder="Без ограничения"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
              />
              <span className="absolute right-3 top-2 text-gray-400 text-sm">₽</span>
            </div>
            <p className="text-[11px] text-gray-400 mt-1">
              Ограничение максимальной суммы скидки на один заказ.
            </p>
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="create-first-paid-only"
              data-testid="input-first-paid-only"
              checked={createFirstPaidOnly}
              onChange={(e) => setCreateFirstPaidOnly(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-black focus:ring-black"
            />
            <label htmlFor="create-first-paid-only" className="text-sm text-gray-700">
              Только для первого заказа покупателя
            </label>
          </div>

          {/* Product Scope Section */}
          <div className="border-t border-gray-200 pt-4">
            <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
              Область действия промокода <span className="text-red-500">*</span>
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 mb-3">
              <button
                type="button"
                data-testid="radio-scope-entire-store"
                onClick={() => {
                  setCreateScope('ENTIRE_STORE');
                  setCreateIncludedProductIds([]);
                  setCreateIncludedCategoryIds([]);
                  setCreateExcludedCategoryIds([]);
                }}
                className={`py-2.5 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer text-left ${
                  createScope === 'ENTIRE_STORE'
                    ? 'border-black bg-black text-white'
                    : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                }`}
              >
                <div className="font-semibold">На все товары магазина</div>
                <div className={`text-xs mt-0.5 ${createScope === 'ENTIRE_STORE' ? 'text-gray-300' : 'text-gray-400'}`}>
                  Применяется ко всему каталогу
                </div>
              </button>
              <button
                type="button"
                data-testid="radio-scope-selected-products"
                onClick={() => {
                  setCreateScope('SELECTED_PRODUCTS');
                  setCreateIncludedCategoryIds([]);
                  setCreateExcludedCategoryIds([]);
                  openSelector('INCLUDE');
                }}
                className={`py-2.5 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer text-left ${
                  createScope === 'SELECTED_PRODUCTS'
                    ? 'border-black bg-black text-white'
                    : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                }`}
              >
                <div className="font-semibold">На выбранные товары</div>
                <div className={`text-xs mt-0.5 ${createScope === 'SELECTED_PRODUCTS' ? 'text-gray-300' : 'text-gray-400'}`}>
                  Только на указанные позиции
                </div>
              </button>
              <button
                type="button"
                data-testid="radio-scope-selected-categories"
                onClick={() => {
                  setCreateScope('SELECTED_CATEGORIES');
                  setCreateIncludedProductIds([]);
                  openCategorySelector('INCLUDE');
                }}
                className={`py-2.5 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer text-left ${
                  createScope === 'SELECTED_CATEGORIES'
                    ? 'border-black bg-black text-white'
                    : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                }`}
              >
                <div className="font-semibold">По категориям</div>
                <div className={`text-xs mt-0.5 ${createScope === 'SELECTED_CATEGORIES' ? 'text-gray-300' : 'text-gray-400'}`}>
                  На выбранные категории
                </div>
              </button>
            </div>

            {/* Compact Included Products State */}
            {createScope === 'SELECTED_PRODUCTS' && (
              <div className="mt-3 p-3.5 bg-gray-50 rounded-lg border border-gray-200" data-testid="selected-products-section">
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider" data-testid="selected-products-summary">
                    {createIncludedProductIds.length > 0 ? (
                      <>Выбрано товаров: <span className="text-gray-900 font-bold" data-testid="selected-summary-count">{createIncludedProductIds.length}</span></>
                    ) : (
                      <span className="text-amber-700">Товары не выбраны</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {createIncludedProductIds.length > 0 && (
                      <button
                        type="button"
                        data-testid="btn-clear-included"
                        onClick={() => setCreateIncludedProductIds([])}
                        className="text-xs text-gray-500 hover:text-red-600 font-medium cursor-pointer"
                      >
                        Очистить
                      </button>
                    )}
                    <button
                      type="button"
                      data-testid="btn-edit-included"
                      onClick={() => openSelector('INCLUDE')}
                      className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                    >
                      {createIncludedProductIds.length > 0 ? 'Изменить' : 'Выбрать товары'}
                    </button>
                  </div>
                </div>

                {createIncludedProductIds.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {createIncludedProductIds.slice(0, 3).map((id) => {
                      const prod = productLookup[id] || sellerProducts.find((p) => p.id === id);
                      return (
                        <span
                          key={id}
                          data-testid={`selected-included-product-${id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs bg-black text-white"
                        >
                          <span className="max-w-[180px] truncate">{prod ? prod.title : id}</span>
                          <button
                            type="button"
                            data-testid={`remove-included-product-${id}`}
                            onClick={() => setCreateIncludedProductIds((prev) => prev.filter((pId) => pId !== id))}
                            className="hover:text-red-300 cursor-pointer ml-0.5"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </span>
                      );
                    })}
                    {createIncludedProductIds.length > 3 && (
                      <span className="inline-flex items-center px-2 py-1 rounded-full text-xs bg-gray-200 text-gray-700 font-medium">
                        +{createIncludedProductIds.length - 3} ещё
                      </span>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Compact Included Categories State */}
            {createScope === 'SELECTED_CATEGORIES' && (
              <div className="mt-3 p-3.5 bg-gray-50 rounded-lg border border-gray-200" data-testid="selected-categories-section">
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider" data-testid="selected-categories-summary">
                    {createIncludedCategoryIds.length > 0 ? (
                      <>Выбрано категорий: <span className="text-gray-900 font-bold" data-testid="selected-categories-count">{createIncludedCategoryIds.length}</span></>
                    ) : (
                      <span className="text-amber-700">Категории не выбраны</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {createIncludedCategoryIds.length > 0 && (
                      <button
                        type="button"
                        data-testid="btn-clear-included-categories"
                        onClick={() => setCreateIncludedCategoryIds([])}
                        className="text-xs text-gray-500 hover:text-red-600 font-medium cursor-pointer"
                      >
                        Очистить
                      </button>
                    )}
                    <button
                      type="button"
                      data-testid="btn-edit-included-categories"
                      onClick={() => openCategorySelector('INCLUDE')}
                      className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                    >
                      {createIncludedCategoryIds.length > 0 ? 'Изменить' : 'Выбрать категории'}
                    </button>
                  </div>
                </div>

                {createIncludedCategoryIds.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {createIncludedCategoryIds.map((id) => {
                      const cat = categoryLookup[id];
                      return (
                        <span
                          key={id}
                          data-testid={`selected-included-category-${id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs bg-black text-white"
                        >
                          <span className="max-w-[180px] truncate">{cat ? cat.name : id}</span>
                          <button
                            type="button"
                            data-testid={`remove-included-category-${id}`}
                            onClick={() => setCreateIncludedCategoryIds((prev) => prev.filter((cId) => cId !== id))}
                            className="hover:text-red-300 cursor-pointer ml-0.5"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </span>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Exclusions Section */}
          <div className="border-t border-gray-200 pt-4">
            <div className="flex items-center justify-between mb-2">
              <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider">
                Товары-исключения (опционально)
              </h4>
              {createExcludedProductIds.length === 0 ? (
                <button
                  type="button"
                  data-testid="btn-toggle-exclusions"
                  onClick={() => openSelector('EXCLUDE')}
                  className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                >
                  + Добавить исключения
                </button>
              ) : (
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    data-testid="btn-clear-excluded"
                    onClick={() => setCreateExcludedProductIds([])}
                    className="text-xs text-gray-500 hover:text-red-600 font-medium cursor-pointer"
                  >
                    Очистить
                  </button>
                  <button
                    type="button"
                    data-testid="btn-edit-exclusions"
                    onClick={() => openSelector('EXCLUDE')}
                    className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                  >
                    Изменить
                  </button>
                </div>
              )}
            </div>
            <p className="text-xs text-gray-400 mb-3">
              Исключённые товары никогда не получат скидку (приоритет над включением).
            </p>

            {createExcludedProductIds.length > 0 && (
              <div className="space-y-2 p-3.5 bg-gray-50 rounded-lg border border-gray-200" data-testid="excluded-products-section">
                <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider" data-testid="excluded-products-summary">
                  Исключено товаров: <span className="text-gray-900 font-bold" data-testid="excluded-summary-count">{createExcludedProductIds.length}</span>
                </div>
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {createExcludedProductIds.slice(0, 3).map((id) => {
                    const prod = productLookup[id] || sellerProducts.find((p) => p.id === id);
                    return (
                      <span
                        key={id}
                        data-testid={`selected-excluded-product-${id}`}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs bg-red-100 text-red-800 border border-red-200"
                      >
                        <span className="max-w-[180px] truncate">{prod ? prod.title : id}</span>
                        <button
                          type="button"
                          data-testid={`remove-excluded-product-${id}`}
                          onClick={() => setCreateExcludedProductIds((prev) => prev.filter((pId) => pId !== id))}
                          className="hover:text-red-900 cursor-pointer ml-0.5"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </span>
                    );
                  })}
                  {createExcludedProductIds.length > 3 && (
                    <span className="inline-flex items-center px-2 py-1 rounded-full text-xs bg-red-50 text-red-700 font-medium border border-red-200">
                      +{createExcludedProductIds.length - 3} ещё
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Category Exclusions Section - only for ENTIRE_STORE and SELECTED_CATEGORIES */}
          {createScope !== 'SELECTED_PRODUCTS' && (
            <div className="border-t border-gray-200 pt-4">
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider">
                  Категории-исключения (опционально)
                </h4>
                {createExcludedCategoryIds.length === 0 ? (
                  <button
                    type="button"
                    data-testid="btn-toggle-category-exclusions"
                    onClick={() => openCategorySelector('EXCLUDE')}
                    className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                  >
                    + Добавить категории-исключения
                  </button>
                ) : (
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      data-testid="btn-clear-excluded-categories"
                      onClick={() => setCreateExcludedCategoryIds([])}
                      className="text-xs text-gray-500 hover:text-red-600 font-medium cursor-pointer"
                    >
                      Очистить
                    </button>
                    <button
                      type="button"
                      data-testid="btn-edit-excluded-categories"
                      onClick={() => openCategorySelector('EXCLUDE')}
                      className="text-xs text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
                    >
                      Изменить
                    </button>
                  </div>
                )}
              </div>
              <p className="text-xs text-gray-400 mb-3">
                Товары из исключённых категорий никогда не получат скидку (приоритет над включением категорий).
              </p>

              {createExcludedCategoryIds.length > 0 && (
                <div className="space-y-2 p-3.5 bg-gray-50 rounded-lg border border-gray-200" data-testid="excluded-categories-section">
                  <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider" data-testid="excluded-categories-summary">
                    Исключено категорий: <span className="text-gray-900 font-bold" data-testid="excluded-categories-count">{createExcludedCategoryIds.length}</span>
                  </div>
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {createExcludedCategoryIds.map((id) => {
                      const cat = categoryLookup[id];
                      return (
                        <span
                          key={id}
                          data-testid={`selected-excluded-category-${id}`}
                          className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs bg-red-100 text-red-800 border border-red-200"
                        >
                          <span className="max-w-[180px] truncate">{cat ? cat.name : id}</span>
                          <button
                            type="button"
                            data-testid={`remove-excluded-category-${id}`}
                            onClick={() => setCreateExcludedCategoryIds((prev) => prev.filter((cId) => cId !== id))}
                            className="hover:text-red-900 cursor-pointer ml-0.5"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </span>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}

          <div className="border-t border-gray-200 pt-4">
            <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
              Период действия (опционально)
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-500 mb-1">Дата начала</label>
                <SellerDateTimePicker
                  data-testid="input-starts-at"
                  value={createStartsAt}
                  onChange={setCreateStartsAt}
                  placeholder="Выберите дату и время"
                  defaultTime="00:00"
                  align="left"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">Дата окончания</label>
                <SellerDateTimePicker
                  data-testid="input-ends-at"
                  value={createEndsAt}
                  onChange={setCreateEndsAt}
                  placeholder="Выберите дату и время"
                  defaultTime="23:59"
                  align="right"
                />
              </div>
            </div>
            {createDateError && (
              <div className="mt-2 text-xs text-red-600 flex items-center gap-1.5" data-testid="create-date-error">
                <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                <span>{createDateError}</span>
              </div>
            )}
          </div>

          <div className="border-t border-gray-200 pt-4">
            <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
              Ограничения использования
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-500 mb-1">Общий лимит (всего)</label>
                <input
                  type="number"
                  min="1"
                  data-testid="input-global-limit"
                  value={createGlobalLimit}
                  onChange={(e) => setCreateGlobalLimit(e.target.value)}
                  placeholder="Без лимита"
                  className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">Лимит на одного клиента</label>
                <input
                  type="number"
                  min="1"
                  required
                  data-testid="input-per-customer-limit"
                  value={createPerCustomerLimit}
                  onChange={(e) => setCreatePerCustomerLimit(e.target.value)}
                  className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                />
              </div>
            </div>
          </div>

          <div data-testid="promo-summary-card" className="p-3.5 bg-gray-50 border border-gray-200 rounded-lg text-xs space-y-1.5">
            <div className="font-semibold text-gray-900">Итоговые условия промокода:</div>
            <div className="text-gray-700">
              <span className="font-medium text-gray-500">Скидка:</span>{' '}
              <span className="font-semibold text-gray-900">
                {createDiscountType === 'percent'
                  ? `${createDiscountPercent || 0}%${createMaxDiscountRub ? ` (макс. ${createMaxDiscountRub} ₽)` : ''}`
                  : `${createDiscountFixedRub || 0} ₽`}
              </span>
            </div>
            <div className="text-gray-700">
              <span className="font-medium text-gray-500">Где действует:</span>{' '}
              <span className="font-semibold text-gray-900">
                {createScope === 'ENTIRE_STORE' && (
                  <>
                    Все товары магазина
                    {createExcludedProductIds.length > 0 && ` (искл. ${createExcludedProductIds.length})`}
                    {createExcludedCategoryIds.length > 0 && ` (искл. ${createExcludedCategoryIds.length} категорий)`}
                  </>
                )}
                {createScope === 'SELECTED_PRODUCTS' && (
                  <>
                    Выбранные товары ({createIncludedProductIds.length} шт.)
                    {createExcludedProductIds.length > 0 && ` (искл. ${createExcludedProductIds.length})`}
                  </>
                )}
                {createScope === 'SELECTED_CATEGORIES' && (
                  <>
                    Выбранные категории ({createIncludedCategoryIds.length} шт.)
                    {createExcludedProductIds.length > 0 && ` (искл. ${createExcludedProductIds.length})`}
                    {createExcludedCategoryIds.length > 0 && ` (искл. ${createExcludedCategoryIds.length} категорий)`}
                  </>
                )}
              </span>
            </div>
            {createMinOrderSubtotalRub && (
              <div className="text-gray-700">
                <span className="font-medium text-gray-500">Мин. заказ:</span>{' '}
                <span className="font-semibold text-gray-900">{createMinOrderSubtotalRub} ₽</span>
              </div>
            )}
            {createMinEligibleQuantity && (
              <div className="text-gray-700">
                <span className="font-medium text-gray-500">Мин. количество:</span>{' '}
                <span className="font-semibold text-gray-900" data-testid="summary-min-quantity">
                  от {createMinEligibleQuantity} товаров
                </span>
              </div>
            )}
            {createFirstPaidOnly && (
              <div className="text-gray-700">
                <span className="font-medium text-gray-500">Ограничение:</span>{' '}
                <span className="font-semibold text-gray-900">Только первый заказ</span>
              </div>
            )}
          </div>

          <div className="pt-4 border-t border-gray-200 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setIsCreateOpen(false)}
              className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={createSubmitting || Boolean(createDateError)}
              data-testid="submit-create-promo"
              className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer disabled:opacity-50"
            >
              {createSubmitting ? 'Создание...' : 'Создать'}
            </button>
          </div>
        </form>
      </SellerModal>

      {/* CATALOG-STYLE PRODUCT SELECTOR MODAL */}
      <SellerModal
        isOpen={Boolean(selectorMode)}
        onClose={closeSelector}
        title={selectorMode === 'INCLUDE' ? 'Выберите товары' : 'Исключить товары'}
        maxWidthClass="max-w-4xl"
        data-testid="product-selector-modal"
      >
        <div className="p-1 space-y-4">
          {/* Search Input */}
          <div className="relative">
            <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
            <input
              type="text"
              data-testid={
                selectorMode === 'INCLUDE'
                  ? 'input-search-included-products'
                  : 'input-search-excluded-products'
              }
              placeholder="Поиск по названию, артикулу или ID"
              value={selectorSearchQuery}
              onChange={(e) => setSelectorSearchQuery(e.target.value)}
              className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm bg-white focus:outline-none focus:ring-2 focus:ring-black"
            />
          </div>

          {/* Error Banner */}
          {selectorError && (
            <div
              data-testid={selectorMode === 'INCLUDE' ? 'include-products-error' : 'exclude-products-error'}
              className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-lg flex items-center justify-between"
            >
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{selectorError}</span>
              </div>
              <button
                type="button"
                data-testid={selectorMode === 'INCLUDE' ? 'btn-retry-included' : 'btn-retry-excluded'}
                onClick={() => loadSelectorProducts(selectorSearchQuery, selectorPage)}
                className="text-xs font-semibold text-red-700 underline hover:text-red-900 cursor-pointer ml-2"
              >
                Повторить
              </button>
            </div>
          )}

          {/* Cards Grid */}
          <div className="max-h-[55vh] overflow-y-auto pr-1">
            {isSelectorLoading ? (
              <div className="py-16 text-center text-sm text-gray-400">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto mb-2"></div>
                Загрузка каталога...
              </div>
            ) : selectorProducts.length === 0 ? (
              <div
                data-testid={selectorMode === 'INCLUDE' ? 'empty-included-search' : 'empty-excluded-search'}
                className="py-16 text-center text-sm text-gray-400"
              >
                {selectorSearchQuery.trim() ? 'Ничего не найдено' : 'Нет доступных товаров'}
              </div>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
                {selectorProducts.map((p) => {
                  const isSelected = draftProductIds.includes(p.id);
                  const isConflicting =
                    selectorMode === 'INCLUDE'
                      ? createExcludedProductIds.includes(p.id)
                      : createIncludedProductIds.includes(p.id);
                  const conflictReason =
                    selectorMode === 'INCLUDE'
                      ? 'Товар находится в исключениях'
                      : 'Товар уже выбран для участия';

                  const imgUrl =
                    p.mainImageUrl ||
                    p.images?.find((img) => img.isMain)?.imageUrl ||
                    p.images?.find((img) => img.isMain)?.url ||
                    p.images?.[0]?.imageUrl ||
                    p.images?.[0]?.url ||
                    (p as any).imageUrl;

                  const sku =
                    (p.variants?.[0] as any)?.sellerSku ||
                    (p.variants?.[0] as any)?.sku ||
                    (p as any).sku ||
                    (p as any).sellerSku ||
                    p.slug;

                  return (
                    <button
                      type="button"
                      key={p.id}
                      disabled={isConflicting}
                      onClick={() => handleToggleDraft(p.id, p)}
                      data-testid={
                        selectorMode === 'INCLUDE'
                          ? `product-option-include-${p.id}`
                          : `product-option-exclude-${p.id}`
                      }
                      aria-selected={isSelected}
                      className={cn(
                        "group relative flex flex-col text-left p-2.5 rounded-xl border transition-all text-xs cursor-pointer select-none bg-white",
                        isConflicting && "opacity-40 cursor-not-allowed bg-gray-50 border-gray-200",
                        !isConflicting && !isSelected && "border-gray-200 hover:border-gray-300 hover:shadow-xs",
                        !isConflicting && isSelected && selectorMode === 'INCLUDE' && "border-black ring-1 ring-black bg-gray-50/50 shadow-xs",
                        !isConflicting && isSelected && selectorMode === 'EXCLUDE' && "border-red-500 ring-1 ring-red-500 bg-red-50/30 shadow-xs"
                      )}
                    >
                      {/* Image Container */}
                      <div className="relative w-full aspect-square bg-gray-100 rounded-lg overflow-hidden mb-2 flex items-center justify-center">
                        {imgUrl ? (
                          <img src={imgUrl} alt={p.title} className="w-full h-full object-cover" />
                        ) : (
                          <div className="flex flex-col items-center justify-center text-gray-400 p-2 text-center">
                            <Package className="w-6 h-6 mb-1 opacity-50" />
                            <span className="text-[10px] text-gray-400 font-medium">Нет фото</span>
                          </div>
                        )}

                        {/* Top-Right Badge: Checkmark or Cross */}
                        {isSelected && selectorMode === 'INCLUDE' && (
                          <div
                            data-testid={`product-card-check-${p.id}`}
                            className="absolute top-1.5 right-1.5 w-6 h-6 rounded-full bg-black text-white flex items-center justify-center shadow-md text-xs font-bold"
                          >
                            <Check className="w-3.5 h-3.5 stroke-[3]" />
                          </div>
                        )}

                        {isSelected && selectorMode === 'EXCLUDE' && (
                          <div
                            data-testid={`product-card-cross-${p.id}`}
                            className="absolute top-1.5 right-1.5 w-6 h-6 rounded-full bg-red-600 text-white flex items-center justify-center shadow-md text-xs font-bold"
                          >
                            <X className="w-3.5 h-3.5 stroke-[3]" />
                          </div>
                        )}
                      </div>

                      {/* Product Meta */}
                      <div className="flex-1 flex flex-col justify-between min-w-0">
                        <div>
                          <div className="font-medium text-gray-900 line-clamp-2 leading-tight mb-1" title={p.title}>
                            {p.title}
                          </div>
                          {sku && (
                            <div className="text-[10px] text-gray-400 truncate mb-1">
                              Арт: {sku}
                            </div>
                          )}
                        </div>

                        <div className="mt-1 pt-1 border-t border-gray-100 flex items-center justify-between">
                          <span className="font-semibold text-gray-900 text-xs">
                            {p.priceCents > 0 ? currencyFormatter.format(p.priceCents / 100) : '0 ₽'}
                          </span>
                        </div>

                        {isConflicting && (
                          <div className="mt-1 text-[10px] font-medium text-amber-700 bg-amber-50 px-1.5 py-0.5 rounded leading-tight">
                            {conflictReason}
                          </div>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}

            {/* Load More Button */}
            {selectorProducts.length < selectorTotalCount && (
              <div className="pt-4 pb-2 text-center">
                <button
                  type="button"
                  data-testid={
                    selectorMode === 'INCLUDE'
                      ? 'btn-load-more-included'
                      : 'btn-load-more-excluded'
                  }
                  disabled={isSelectorLoadingMore}
                  onClick={() => loadSelectorProducts(selectorDebouncedQuery, selectorPage + 1)}
                  className="text-xs text-blue-600 hover:text-blue-800 font-medium py-1.5 px-4 rounded-lg border border-blue-200 hover:bg-blue-50 transition-colors disabled:opacity-50 cursor-pointer"
                >
                  {isSelectorLoadingMore ? 'Загрузка...' : 'Загрузить ещё'}
                </button>
              </div>
            )}
          </div>

          {/* Modal Actions */}
          <div className="pt-4 border-t border-gray-200 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              {selectorMode === 'INCLUDE' ? (
                <>Выбрано: <span className="font-semibold text-gray-900">{draftProductIds.length}</span></>
              ) : (
                <>Исключено: <span className="font-semibold text-gray-900">{draftProductIds.length}</span></>
              )}
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                data-testid="btn-cancel-selector"
                onClick={closeSelector}
                className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="button"
                data-testid="btn-apply-selector"
                onClick={applySelector}
                className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
              >
                Применить
              </button>
            </div>
          </div>
        </div>
      </SellerModal>

      {/* CATEGORY SELECTOR MODAL */}
      <SellerModal
        isOpen={Boolean(categorySelectorMode)}
        onClose={closeCategorySelector}
        title={categorySelectorMode === 'INCLUDE' ? 'Выберите категории' : 'Исключить категории'}
        maxWidthClass="max-w-2xl"
        data-testid="category-selector-modal"
      >
        <div className="p-1 space-y-4">
          {/* Search Input */}
          <div className="relative">
            <Search className="w-4 h-4 text-gray-400 absolute left-3 top-3" />
            <input
              type="text"
              data-testid="category-search-input"
              placeholder="Поиск категорий"
              value={categorySearchQuery}
              onChange={(e) => setCategorySearchQuery(e.target.value)}
              className="w-full pl-9 pr-4 py-2 border border-gray-300 rounded-lg text-sm bg-white focus:outline-none focus:ring-2 focus:ring-black"
            />
          </div>

          {/* Error Banner */}
          {categorySelectorError && (
            <div
              className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-lg flex items-center justify-between"
            >
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{categorySelectorError}</span>
              </div>
              <button
                type="button"
                onClick={loadCategories}
                className="text-xs font-semibold text-red-700 underline hover:text-red-900 cursor-pointer ml-2"
              >
                Повторить
              </button>
            </div>
          )}

          {/* Category Tree */}
          <div className="max-h-[55vh] overflow-y-auto pr-1" data-testid="category-tree-container">
            {isCategoriesLoading ? (
              <div className="py-16 text-center text-sm text-gray-400">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto mb-2"></div>
                Загрузка категорий...
              </div>
            ) : categories.length === 0 ? (
              <div className="py-16 text-center text-sm text-gray-400">
                Нет доступных категорий
              </div>
            ) : (() => {
              const query = categorySearchQuery.trim().toLowerCase();
              const isSearchActive = query.length > 0;

              const matchingOrAncestorIds = new Set<string>();
              if (isSearchActive) {
                const addDescendants = (parentId: string) => {
                  categories.forEach((cat) => {
                    if (cat.parentId === parentId) {
                      matchingOrAncestorIds.add(cat.id);
                      addDescendants(cat.id);
                    }
                  });
                };

                categories.forEach((cat) => {
                  if (cat.name.toLowerCase().includes(query)) {
                    matchingOrAncestorIds.add(cat.id);
                    const ancestors = getCategoryAncestors(cat.id, categoryLookup);
                    ancestors.forEach((ancId) => matchingOrAncestorIds.add(ancId));
                    addDescendants(cat.id);
                  }
                });

                if (matchingOrAncestorIds.size === 0) {
                  return (
                    <div
                      data-testid={categorySelectorMode === 'INCLUDE' ? 'empty-included-category-search' : 'empty-excluded-category-search'}
                      className="py-16 text-center text-sm text-gray-400"
                    >
                      Ничего не найдено
                    </div>
                  );
                }
              }

              const tree = buildCategoryTree(categories, categoryLookup);

              const renderTree = (nodes: CategoryTreeNode[]): React.ReactNode => {
                return nodes.map((node) => {
                  const c = node.category;
                  const hasChildren = node.children.length > 0;

                  if (isSearchActive && !matchingOrAncestorIds.has(c.id)) {
                    return null;
                  }

                  const isExpanded = isSearchActive ? true : expandedCategoryIds.has(c.id);
                  const isSelected = draftCategoryIds.includes(c.id);
                  const isConflicting =
                    categorySelectorMode === 'INCLUDE'
                      ? createExcludedCategoryIds.includes(c.id)
                      : createIncludedCategoryIds.includes(c.id);
                  const conflictReason =
                    categorySelectorMode === 'INCLUDE'
                      ? 'Категория уже находится в списке исключений'
                      : 'Категория уже находится в списке включений';

                  const isInherited = isAncestorSelected(c, draftCategoryIds, categoryLookup);

                  return (
                    <div key={c.id} className="flex flex-col">
                      <div
                        data-testid={`category-option-${c.id}`}
                        role="treeitem"
                        aria-selected={isSelected}
                        aria-expanded={hasChildren ? isExpanded : undefined}
                        className={cn(
                          "group relative flex items-stretch justify-between p-1 rounded-xl border transition-all text-xs select-none bg-white mb-1.5",
                          isConflicting && "opacity-50 bg-gray-50 border-gray-200",
                          !isConflicting && !isSelected && "border-gray-200 hover:border-gray-300 hover:shadow-xs",
                          !isConflicting && isSelected && categorySelectorMode === 'INCLUDE' && "border-black ring-1 ring-black bg-gray-50/50 shadow-xs",
                          !isConflicting && isSelected && categorySelectorMode === 'EXCLUDE' && "border-red-500 ring-1 ring-red-500 bg-red-50/30 shadow-xs"
                        )}
                        style={{ paddingLeft: `${node.level * 20 + 8}px` }}
                      >
                        {hasChildren ? (
                          <>
                            {/* LEFT / MAIN DISCLOSURE ZONE (~65-75% width hit target) */}
                            <button
                              type="button"
                              data-testid={`category-disclosure-${c.id}`}
                              aria-label={isExpanded ? `Свернуть ${c.name}` : `Раскрыть ${c.name}`}
                              aria-expanded={isExpanded}
                              onClick={(e) => toggleExpandCategory(c.id, e)}
                              className="flex items-center gap-2 flex-1 min-w-0 pr-3 p-1.5 text-left rounded-lg hover:bg-gray-100/70 transition-colors cursor-pointer focus:outline-none focus:ring-2 focus:ring-black"
                            >
                              <div className="p-0.5 text-gray-400 group-hover:text-gray-900 shrink-0 -ml-0.5">
                                {isExpanded ? (
                                  <ChevronDown className="w-4 h-4" />
                                ) : (
                                  <ChevronRight className="w-4 h-4" />
                                )}
                              </div>

                              <div className="flex flex-col min-w-0">
                                <div
                                  data-testid={`category-title-${c.id}`}
                                  className={cn("font-medium text-sm text-gray-900 truncate", isConflicting && "text-gray-400")}
                                >
                                  {c.name}
                                </div>

                                {isSelected && (
                                  <div
                                    data-testid={`category-parent-hint-${c.id}`}
                                    className={cn(
                                      "text-[11px] font-medium mt-0.5",
                                      categorySelectorMode === 'INCLUDE' ? "text-blue-600" : "text-red-600"
                                    )}
                                  >
                                    {categorySelectorMode === 'INCLUDE'
                                      ? "Включает все подкатегории"
                                      : "Исключает все подкатегории"}
                                  </div>
                                )}

                                {!isSelected && isInherited && (
                                  <div
                                    data-testid={`category-inherited-hint-${c.id}`}
                                    className="text-[11px] text-gray-400 font-medium mt-0.5"
                                  >
                                    {categorySelectorMode === 'INCLUDE'
                                      ? "Включено родительской категорией"
                                      : "Исключено родительской категорией"}
                                  </div>
                                )}

                                {isConflicting && (
                                  <div
                                    data-testid={`category-conflict-notice-${c.id}`}
                                    className="mt-1 text-[10px] font-medium text-amber-700 bg-amber-50 px-1.5 py-0.5 rounded leading-tight inline-block"
                                  >
                                    {conflictReason}
                                  </div>
                                )}
                              </div>
                            </button>

                            {/* RIGHT SELECTION ZONE (~25-30% width hit target) */}
                            <button
                              type="button"
                              data-testid={`category-select-${c.id}`}
                              disabled={isConflicting}
                              aria-label={isSelected ? `Отменить выбор ${c.name}` : `Выбрать ${c.name}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                if (!isConflicting) handleToggleCategoryDraft(c.id);
                              }}
                              className={cn(
                                "shrink-0 flex items-center justify-end px-3 py-1.5 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-black",
                                !isConflicting && "hover:bg-gray-100 cursor-pointer",
                                isConflicting && "cursor-not-allowed"
                              )}
                            >
                              <div className="flex items-center gap-2">
                                {isSelected && categorySelectorMode === 'INCLUDE' && (
                                  <div
                                    data-testid={`category-item-check-${c.id}`}
                                    className="w-6 h-6 rounded-full bg-black text-white flex items-center justify-center shadow-xs text-xs font-bold"
                                  >
                                    <Check className="w-3.5 h-3.5 stroke-[3]" />
                                  </div>
                                )}
                                {isSelected && categorySelectorMode === 'EXCLUDE' && (
                                  <div
                                    data-testid={`category-item-cross-${c.id}`}
                                    className="w-6 h-6 rounded-full bg-red-600 text-white flex items-center justify-center shadow-xs text-xs font-bold"
                                  >
                                    <X className="w-3.5 h-3.5 stroke-[3]" />
                                  </div>
                                )}
                                {!isSelected && !isConflicting && (
                                  <div className="w-6 h-6 rounded-full border border-gray-300 group-hover:border-gray-400" />
                                )}
                                {isConflicting && (
                                  <div className="w-6 h-6 rounded-full border border-gray-200 bg-gray-100 opacity-50" />
                                )}
                              </div>
                            </button>
                          </>
                        ) : (
                          /* LEAF CATEGORY (NO EXPAND/COLLAPSE - DIRECT SELECT) */
                          <button
                            type="button"
                            data-testid={`category-select-${c.id}`}
                            disabled={isConflicting}
                            aria-label={isSelected ? `Отменить выбор ${c.name}` : `Выбрать ${c.name}`}
                            onClick={() => !isConflicting && handleToggleCategoryDraft(c.id)}
                            className={cn(
                              "flex items-center justify-between w-full p-1.5 text-left rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-black",
                              !isConflicting && "hover:bg-gray-100/70 cursor-pointer",
                              isConflicting && "cursor-not-allowed"
                            )}
                          >
                            <div className="flex items-center gap-2 flex-1 min-w-0 pr-3">
                              <div className="w-6 h-4 shrink-0 -ml-1" aria-hidden="true" />
                              <div className="flex flex-col min-w-0">
                                <div
                                  data-testid={`category-title-${c.id}`}
                                  className={cn("font-medium text-sm text-gray-900 truncate", isConflicting && "text-gray-400")}
                                >
                                  {c.name}
                                </div>

                                {!isSelected && isInherited && (
                                  <div
                                    data-testid={`category-inherited-hint-${c.id}`}
                                    className="text-[11px] text-gray-400 font-medium mt-0.5"
                                  >
                                    {categorySelectorMode === 'INCLUDE'
                                      ? "Включено родительской категорией"
                                      : "Исключено родительской категорией"}
                                  </div>
                                )}

                                {isConflicting && (
                                  <div
                                    data-testid={`category-conflict-notice-${c.id}`}
                                    className="mt-1 text-[10px] font-medium text-amber-700 bg-amber-50 px-1.5 py-0.5 rounded leading-tight inline-block"
                                  >
                                    {conflictReason}
                                  </div>
                                )}
                              </div>
                            </div>

                            <div className="shrink-0 flex items-center justify-end px-1">
                              {isSelected && categorySelectorMode === 'INCLUDE' && (
                                <div
                                  data-testid={`category-item-check-${c.id}`}
                                  className="w-6 h-6 rounded-full bg-black text-white flex items-center justify-center shadow-xs text-xs font-bold"
                                >
                                  <Check className="w-3.5 h-3.5 stroke-[3]" />
                                </div>
                              )}
                              {isSelected && categorySelectorMode === 'EXCLUDE' && (
                                <div
                                  data-testid={`category-item-cross-${c.id}`}
                                  className="w-6 h-6 rounded-full bg-red-600 text-white flex items-center justify-center shadow-xs text-xs font-bold"
                                >
                                  <X className="w-3.5 h-3.5 stroke-[3]" />
                                </div>
                              )}
                              {!isSelected && !isConflicting && (
                                <div className="w-6 h-6 rounded-full border border-gray-300 group-hover:border-gray-400" />
                              )}
                              {isConflicting && (
                                <div className="w-6 h-6 rounded-full border border-gray-200 bg-gray-100 opacity-50" />
                              )}
                            </div>
                          </button>
                        )}
                      </div>

                      {hasChildren && isExpanded && (
                        <div className="flex flex-col">
                          {renderTree(node.children)}
                        </div>
                      )}
                    </div>
                  );
                });
              };

              return (
                <div className="space-y-1">
                  {renderTree(tree)}
                </div>
              );
            })()}
          </div>

          {/* Modal Actions */}
          <div className="pt-4 border-t border-gray-200 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              {categorySelectorMode === 'INCLUDE' ? (
                <>Выбрано: <span className="font-semibold text-gray-900">{draftCategoryIds.length}</span></>
              ) : (
                <>Исключено: <span className="font-semibold text-gray-900">{draftCategoryIds.length}</span></>
              )}
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                data-testid="btn-cancel-category-selector"
                onClick={closeCategorySelector}
                className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="button"
                data-testid="btn-apply-category-selector"
                onClick={applyCategorySelector}
                className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
              >
                Применить
              </button>
            </div>
          </div>
        </div>
      </SellerModal>

      {/* EDIT MODAL (MUTABLE FIELDS ONLY) */}
      <SellerModal
        isOpen={Boolean(editPromo)}
        onClose={() => setEditPromo(null)}
        title={editPromo ? `Параметры промокода: ${editPromo.code}` : ''}
        data-testid="edit-promo-modal"
      >
        {editPromo && (
          <form onSubmit={handleEditSubmit} className="space-y-6">
            {editError && (
              <div
                data-testid="edit-promo-error"
                className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2"
              >
                <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                <span>{editError}</span>
              </div>
            )}

            {/* Read-Only Economics Section */}
            <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg space-y-2">
              <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                Неизменяемые экономические параметры
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <span className="text-gray-500">Скидка: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.discountType === 'percent'
                      ? `${editPromo.discountValueBps / 100}%`
                      : currencyFormatter.format(editPromo.discountValueFixedCents / 100)}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Мин. заказ: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.minOrderSubtotalCents > 0
                      ? currencyFormatter.format(editPromo.minOrderSubtotalCents / 100)
                      : '0 ₽'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Мин. количество: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-min-quantity">
                    {editPromo.minEligibleQuantity && editPromo.minEligibleQuantity > 0
                      ? `от ${editPromo.minEligibleQuantity} товаров`
                      : 'Без ограничений'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">1-й заказ: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.firstPaidOrderOnly ? 'Да' : 'Нет'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Финансирование: </span>
                  <span className="font-semibold text-gray-900">Продавец (SELLER)</span>
                </div>
                <div>
                  <span className="text-gray-500">Область действия: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-scope">
                    {editPromo.productScope === 'SELECTED_PRODUCTS'
                      ? `${(editPromo.includedProductIds || []).length} выбранных товаров`
                      : editPromo.productScope === 'SELECTED_CATEGORIES'
                      ? `${(editPromo.includedCategoryIds || []).length} выбранных категорий`
                      : 'Все товары'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Исключения: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-exclusions">
                    {(editPromo.excludedProductIds || []).length > 0
                      ? `${editPromo.excludedProductIds!.length} товаров`
                      : 'Нет'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Выбранные категории: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-selected-categories">
                    {editPromo.productScope === 'SELECTED_CATEGORIES'
                      ? `${(editPromo.includedCategoryIds || []).length} категорий`
                      : 'Не применимо'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Категории-исключения: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-excluded-categories">
                    {editPromo.productScope === 'SELECTED_PRODUCTS'
                      ? 'Не применимо'
                      : (editPromo.excludedCategoryIds || []).length > 0
                      ? `${editPromo.excludedCategoryIds!.length} категорий`
                      : 'Нет'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Максимальная скидка: </span>
                  <span className="font-semibold text-gray-900" data-testid="edit-promo-max-discount">
                    {editPromo.maxDiscountCents
                      ? currencyFormatter.format(editPromo.maxDiscountCents / 100)
                      : 'Без ограничения'}
                  </span>
                </div>
              </div>
              <p className="text-[11px] text-gray-400 mt-1">
                Экономические параметры зафиксированы для защиты уже созданных заказов и расчётов.
              </p>
            </div>

            {/* Editable Operational Fields */}
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                Активность промокода
              </label>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  data-testid="edit-toggle-active"
                  onClick={() => setEditIsActive(!editIsActive)}
                  className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium border transition-colors cursor-pointer ${
                    editIsActive
                      ? 'border-emerald-300 bg-emerald-50 text-emerald-800'
                      : 'border-amber-300 bg-amber-50 text-amber-800'
                  }`}
                >
                  {editIsActive ? (
                    <>
                      <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                      Активен (применяется при оформлении)
                    </>
                  ) : (
                    <>
                      <Pause className="w-4 h-4 text-amber-600" />
                      На паузе (новые заказы не принимаются)
                    </>
                  )}
                </button>
              </div>
            </div>

            <div className="border-t border-gray-200 pt-4">
              <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
                Период действия
              </h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Дата начала</label>
                  <SellerDateTimePicker
                    data-testid="edit-input-starts-at"
                    value={editStartsAt}
                    onChange={setEditStartsAt}
                    placeholder="Выберите дату и время"
                    defaultTime="00:00"
                    align="left"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Дата окончания</label>
                  <SellerDateTimePicker
                    data-testid="edit-input-ends-at"
                    value={editEndsAt}
                    onChange={setEditEndsAt}
                    placeholder="Выберите дату и время"
                    defaultTime="23:59"
                    align="right"
                  />
                </div>
              </div>
              {editDateError && (
                <div className="mt-2 text-xs text-red-600 flex items-center gap-1.5" data-testid="edit-date-error">
                  <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                  <span>{editDateError}</span>
                </div>
              )}
            </div>

            <div className="border-t border-gray-200 pt-4">
              <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
                Ограничения использования
              </h4>
              <div className="text-xs text-gray-500 mb-2">
                Текущее использование:{' '}
                <span className="font-semibold text-gray-900">
                  {editPromo.consumedUsageCount} завершено, {editPromo.reservedUsageCount} в резерве
                </span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Общий лимит</label>
                  <input
                    type="number"
                    min="1"
                    data-testid="edit-input-global-limit"
                    value={editGlobalLimit}
                    onChange={(e) => setEditGlobalLimit(e.target.value)}
                    placeholder="Без лимита"
                    className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <p className="text-[11px] text-gray-400 mt-0.5">
                    Не может быть меньше{' '}
                    {editPromo.consumedUsageCount + editPromo.reservedUsageCount}.
                  </p>
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Лимит на клиента</label>
                  <input
                    type="number"
                    min="1"
                    required
                    data-testid="edit-input-per-customer-limit"
                    value={editPerCustomerLimit}
                    onChange={(e) => setEditPerCustomerLimit(e.target.value)}
                    className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                  />
                </div>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-200 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setEditPromo(null)}
                className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="submit"
                disabled={editSubmitting || Boolean(editDateError)}
                data-testid="submit-edit-promo"
                className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer disabled:opacity-50"
              >
                {editSubmitting ? 'Сохранение...' : 'Сохранить изменения'}
              </button>
            </div>
          </form>
        )}
      </SellerModal>
    </SellerPageFrame>
  );
}

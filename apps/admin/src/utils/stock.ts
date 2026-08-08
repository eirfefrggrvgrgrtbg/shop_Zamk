export interface StockInfo {
  totalStock: number;
  reservedStock: number;
  availableStock: number;
  hasInventoryRecord: boolean;
  hasVariants: boolean;
  isNegative: boolean;
  label: string;
  badgeClass: string;
}

export function computeStockInfo(
  stock?: number,
  reservedStock = 0,
  variantsCount = 1,
  hasInventory = true
): StockInfo {
  if (variantsCount === 0) {
    return {
      totalStock: 0,
      reservedStock: 0,
      availableStock: 0,
      hasInventoryRecord: false,
      hasVariants: false,
      isNegative: false,
      label: 'Нет вариантов',
      badgeClass: 'bg-rose-50 text-rose-700 border-rose-200',
    };
  }

  if (stock === undefined || stock === null || !hasInventory) {
    return {
      totalStock: 0,
      reservedStock: 0,
      availableStock: 0,
      hasInventoryRecord: false,
      hasVariants: true,
      isNegative: false,
      label: 'Нет складской записи',
      badgeClass: 'bg-amber-50 text-amber-700 border-amber-200',
    };
  }

  const available = stock - reservedStock;

  if (available < 0) {
    return {
      totalStock: stock,
      reservedStock,
      availableStock: available,
      hasInventoryRecord: true,
      hasVariants: true,
      isNegative: true,
      label: `Ошибка остатка (${available} шт.)`,
      badgeClass: 'bg-red-100 text-red-900 border-red-300 font-bold',
    };
  }

  if (available === 0) {
    return {
      totalStock: stock,
      reservedStock,
      availableStock: 0,
      hasInventoryRecord: true,
      hasVariants: true,
      isNegative: false,
      label: 'Нет в наличии',
      badgeClass: 'bg-orange-50 text-orange-700 border-orange-200',
    };
  }

  return {
    totalStock: stock,
    reservedStock,
    availableStock: available,
    hasInventoryRecord: true,
    hasVariants: true,
    isNegative: false,
    label: `В наличии · ${available} шт.`,
    badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  };
}

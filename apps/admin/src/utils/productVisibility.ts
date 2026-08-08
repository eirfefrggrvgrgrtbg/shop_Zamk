export interface ActualVisibilityInfo {
  isVisible: boolean;
  reasonLabel: string;
  badgeClass: string;
  canOpenPublicUrl: boolean;
}

export function computeActualVisibility(product: {
  status: string;
  sellerStatus?: string;
  stock?: number;
  variantsCount?: number;
}): ActualVisibilityInfo {
  if (product.status !== 'published') {
    return {
      isVisible: false,
      reasonLabel: `Не на витрине`,
      badgeClass: 'bg-slate-100 text-slate-700 border-slate-200',
      canOpenPublicUrl: false,
    };
  }

  if (product.sellerStatus && product.sellerStatus !== 'active') {
    return {
      isVisible: false,
      reasonLabel: 'Не видим: Продавец неактивен',
      badgeClass: 'bg-amber-50 text-amber-800 border-amber-300 font-medium',
      canOpenPublicUrl: false,
    };
  }

  if (product.variantsCount === 0) {
    return {
      isVisible: false,
      reasonLabel: 'Не видим: Нет вариантов',
      badgeClass: 'bg-rose-50 text-rose-800 border-rose-300 font-medium',
      canOpenPublicUrl: false,
    };
  }

  if (product.stock !== undefined && product.stock <= 0) {
    return {
      isVisible: false,
      reasonLabel: 'Не видим: Нет в наличии',
      badgeClass: 'bg-orange-50 text-orange-800 border-orange-300 font-medium',
      canOpenPublicUrl: false,
    };
  }

  return {
    isVisible: true,
    reasonLabel: 'Видим на витрине',
    badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-300 font-medium',
    canOpenPublicUrl: true,
  };
}

export interface ProductPresentationMediaItem {
  url: string;
  colorId?: string;
}

export interface ProductPresentationSizeChartField {
  code: string;
  name: string;
  unit?: string;
  isRequired?: boolean;
  sortOrder?: number;
}

export interface ProductPresentationSizeChartRow {
  sizeValueId?: string;
  sizeValueName: string;
  measurements?: Record<string, string | number>;
}

export interface ProductPresentationSizeChart {
  fields?: ProductPresentationSizeChartField[];
  rows: ProductPresentationSizeChartRow[];
}

export interface ProductPresentationCoreProduct {
  id?: string;
  name: string;
  brand?: string;
  brandId?: string;
  category?: string;
  materials?: string;
  materialComposition?: Array<{
    materialName?: string;
    material?: string;
    percentage: number;
  }>;
  careInstructions?: string;
  description?: string;
  isNew?: boolean;
  discountPrice?: number;
  price: number;
  rating?: number;
  sellerName?: string;
  sellerSlug?: string;
  sizeChart?: ProductPresentationSizeChart;
  isPreview?: boolean;
  attributes?: Array<{
    label: string;
    value: string;
  }>;
}

export interface ProductPresentationColorOption {
  id: string;
  name: string;
  shadeName?: string;
  hex?: string;
  hasInStock: boolean;
}

export type ProductPresentationSizeState =
  | 'AVAILABLE'
  | 'SOLD_OUT'
  | 'NOT_OFFERED';

export type ProductPresentationDimensionType =
  | 'COLOR_AND_SIZE'
  | 'COLOR_ONLY'
  | 'SIZE_ONLY'
  | 'SINGLE_VARIANT';

export interface ProductPresentationSizeOption {
  id: string;
  label: string;
  accessibleLabel?: string;
  state: ProductPresentationSizeState;
  disabled?: boolean;
}

export interface MeasurementMeta {
  label: string;
  shortLabel: string;
  instruction: string;
}

export interface ProductPresentationSelectedVariant {
  id?: string;
  sellerSku?: string;
  sku?: string;
  priceCents?: number;
  size?: string;
}

export interface ProductPresentationCoreProps {
  product: ProductPresentationCoreProduct;
  visibleImages: ProductPresentationMediaItem[];
  activeImage: number;
  onActiveImageChange: (index: number) => void;
  displayPrice: number;
  colors: ProductPresentationColorOption[];
  sizes: ProductPresentationSizeOption[];
  selectedColorId: string | null;
  selectedSizeId: string | null;
  selectedColor: ProductPresentationColorOption | null;
  selectedSize: ProductPresentationSizeOption | null;
  selectedVariant: ProductPresentationSelectedVariant | null;
  isResolved: boolean;
  canAddToCart: boolean;
  requiresColor: boolean;
  requiresSize: boolean;
  isAddingToCart: boolean;
  isProductUnavailable: boolean;
  ctaText: string;
  sizeSelectionNotice: string | null;
  refreshErrorNotice: string | null;
  sizeError: string;
  reviewsCountText?: string;
  onColorChange: (colorId: string) => void;
  onSizeChange: (sizeId: string) => void;
  onAddToCart: () => void;
  isFavorite: boolean;
  onToggleFavorite: () => void;
  onScrollToReviews: () => void;
  onBrandClick?: (brandId: string) => void;
  brandHref?: string | null;
  onDeliveryClick?: () => void;
  deliveryHref?: string | null;
  onReturnsClick?: () => void;
  returnsHref?: string | null;
  onSellerClick?: (sellerSlug: string) => void;
  sellerHref?: string | null;
  emptyMediaSlot?: React.ReactNode;
  emptyPricePlaceholder?: string;
  colorLabelSuffix?: React.ReactNode;
  colorLabelSuffixClassName?: string;
  sizeLabelSuffix?: React.ReactNode;
  sizeLabelSuffixClassName?: string;
  mediaAddSlot?: React.ReactNode;
  colorAddSlot?: React.ReactNode;
  sizeAddSlot?: React.ReactNode;
  compositionAddSlot?: React.ReactNode;
  compositionSlot?: React.ReactNode;
  characteristicsAddSlot?: React.ReactNode;
  sizeChartSlot?: React.ReactNode;
  titleSlot?: React.ReactNode;
  priceSlot?: React.ReactNode;
  descriptionSlot?: React.ReactNode;
  galleryExtraSlot?: React.ReactNode;
  renderThumbnailOverlay?: (image: ProductPresentationMediaItem, index: number) => React.ReactNode;
  onColorRemove?: (id: string) => void;
  onSizeRemove?: (id: string) => void;
}

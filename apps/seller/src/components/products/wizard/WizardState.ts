
export type VariantDraft = {
  id?: string;
  colorId?: string;
  sizeValueId?: string;
  active: boolean;
  sellerSku: string;
  barcode: string;
  priceCents?: number;
  attributes?: Record<string, string | number | boolean | string[]>;
};

export type WizardState = {
  id?: string;
  title: string;
  description: string;
  categoryId: string;
  productAttributes: Record<string, string | boolean | number | string[]>;
  materialComposition: { materialId: string; percentage: number }[];
  selectedColorIds: string[];
  shadeNamesByColor: Record<string, string>;
  selectedSizeSystemId: string;
  selectedSizeValueIds: string[];
  variants: VariantDraft[];
  sizeChartRows: Record<string, Record<string, number>>;
  commonImages: { url: string; sortOrder: number }[];
  colorImages: Record<string, { url: string; sortOrder: number }[]>;
};

export const initialWizardState: WizardState = {
  title: '',
  description: '',
  categoryId: '',
  productAttributes: {},
  materialComposition: [],
  selectedColorIds: [],
  shadeNamesByColor: {},
  selectedSizeSystemId: '',
  selectedSizeValueIds: [],
  variants: [],
  sizeChartRows: {},
  commonImages: [],
  colorImages: {}
};

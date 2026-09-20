import { ProductStudioProvider, type ProductStudioDraft } from '../contexts/ProductStudioContext';
import { ProductStudio } from '../components/product-studio/ProductStudio';

export const INITIAL_EMPTY_STUDIO_DRAFT: ProductStudioDraft = {
  title: '',
  description: '',
  categoryId: '',
  images: [],
  variants: [],
};

export function SellerProductStudioNew() {
  return (
    <ProductStudioProvider entryMode="create" initialDraft={INITIAL_EMPTY_STUDIO_DRAFT}>
      <ProductStudio />
    </ProductStudioProvider>
  );
}

export default SellerProductStudioNew;

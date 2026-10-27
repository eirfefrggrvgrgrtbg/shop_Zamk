import { ProductDetail, type ProductDetailV5Slots } from '../../pages/ProductDetail';
import { V5PdpSkeleton } from './V5PdpSkeleton';
import { V5ProductPassport } from './V5ProductPassport';
import './v5-pdp.css';

const V5_SLOTS: ProductDetailV5Slots = {
  loading: <V5PdpSkeleton />,
  archive: (
    <p className="zv5-archive" data-testid="v5-archive-label">
      ZAMK ARCHIVE
    </p>
  ),
  passport: ({ product, selectedVariant }) => <V5ProductPassport product={product} selectedVariant={selectedVariant} />,
};

/**
 * Dev-only Shop V5 PDP preview (mounted at /v5-pdp-preview/:id). It is the production ProductDetail data flow
 * (variants, cart, favorites, auth) with only the presentation switched to 'v5'. All V5-only components are imported
 * here, so the production bundle (which never imports this file) does not contain them.
 */
export function ShopV5ProductDetail() {
  return <ProductDetail presentation="v5" v5Slots={V5_SLOTS} />;
}

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Heart } from 'lucide-react';
import type { Product } from '../../types/catalog';
import { formatPrice, getProductEffectivePrice } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';
import { useCatalogImpression } from '../../lib/behavior';
import { archiveNumber, articleCode, photoFit, type V5PhotoFit } from './v5Selection';

interface V5ProductCardProps {
  product: Product;
  index: number;
  placement: string;
  /** Hero-adjacent images load eagerly, everything below is lazy. */
  eager?: boolean;
  showNew?: boolean;
  /** The leading card of the «Для вас» chapter: larger photo and type. */
  lead?: boolean;
}

/**
 * Editorial home card: a borderless rectangular photo and a small paper tag that overlaps its lower edge.
 * The whole card targets the real PDP. The card never adds to the cart: the summary API carries no
 * variant id, so a purchase decision (size/colour) always happens on the PDP.
 */
export function V5ProductCard({ product, index, placement, eager = false, showNew = false, lead = false }: V5ProductCardProps) {
  const ref = useCatalogImpression<HTMLElement>({ productId: product.id, placement, disabled: Boolean(product.isPreview) });
  const { toggleFavorite, isFavorite } = useFavorites();
  const [fit, setFit] = useState<V5PhotoFit>('cover');
  const favorited = isFavorite(product.id);
  const href = `/product/${product.id}`;
  const price = getProductEffectivePrice(product);
  const oldPrice = product.discountPrice !== undefined ? product.oldPrice : undefined;
  const brandLabel = product.sellerName || product.brand;
  const number = archiveNumber(index);

  return (
    <article ref={ref} className={lead ? 'v5-card v5-card--lead' : 'v5-card'} data-testid={`v5-card-${product.id}`}>
      <div className="v5-card__photo" data-fit={fit}>
        <Link to={href} aria-label={`${product.name}, ${brandLabel}`} tabIndex={-1}>
          <img
            src={product.image}
            alt={product.name}
            loading={eager ? 'eager' : 'lazy'}
            decoding="async"
            width={900}
            height={1200}
            onLoad={(e) => setFit(photoFit(e.currentTarget.naturalWidth, e.currentTarget.naturalHeight))}
          />
        </Link>
        {showNew && <span className="v5-card__new">NEW</span>}
        <button
          type="button"
          className="v5-fav"
          aria-pressed={favorited}
          aria-label={favorited ? 'Убрать из избранного' : 'Добавить в избранное'}
          onClick={() => void toggleFavorite(product.id)}
        >
          <Heart aria-hidden="true" />
        </button>
      </div>
      <Link to={href} className="v5-tag">
        <span className="v5-tag__rc">
          <span>{brandLabel}</span>
          <span>ZK-{number}</span>
        </span>
        <span className="v5-tag__name">{product.name}</span>
        <span className="v5-tag__price">
          <b>{formatPrice(price)}</b>
          {oldPrice !== undefined && <s>{formatPrice(oldPrice)}</s>}
        </span>
        <span className="v5-tag__foot">
          <span>АРТИКУЛ {articleCode(product)}</span>
          <span className="v5-bars" aria-hidden="true" />
        </span>
      </Link>
    </article>
  );
}

export function V5CardSkeleton() {
  return <div className="v5-skel v5-skel--photo" aria-hidden="true" />;
}

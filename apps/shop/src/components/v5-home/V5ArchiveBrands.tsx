import { Link } from 'react-router-dom';
import type { Brand } from '../../types/catalog';
import { archiveNumber, brandHref, pickBrandsForHome } from './v5Selection';

interface V5ArchiveBrandsProps {
  brands: Brand[];
  /** Brand of the newest arrival (real data): becomes the leading brand when present in the brand list. */
  featuredBrandId?: string;
  isLoading: boolean;
  error: string;
  onRetry: () => void;
}

/**
 * Typographic index: one leading brand set large, the rest as a compact numbered list.
 * Only names, descriptions and logos that the brands API returns are shown.
 */
export function V5ArchiveBrands({ brands, featuredBrandId, isLoading, error, onRetry }: V5ArchiveBrandsProps) {
  const { lead, rest, hiddenCount } = pickBrandsForHome(brands, featuredBrandId);

  return (
    <section id="v5-brands" className="v5-sec" aria-labelledby="v5-brands-title" data-testid="v5-brands">
      <div className="v5-wrap">
        <div className="v5-sec__hd">
          <div>
            <p className="v5-kick">03 — бренды</p>
            <h2 id="v5-brands-title">
              Бренды <em>в архиве</em>
            </h2>
            <p>Независимые бренды, чьи вещи есть в каталоге ZAMK.</p>
          </div>
          <Link to="/brands" className="v5-more">
            Все бренды <span aria-hidden="true">→</span>
          </Link>
        </div>

        {isLoading ? (
          <div className="v5-bx" aria-busy="true" aria-label="Загрузка брендов">
            <div className="v5-skel v5-bx__lead" aria-hidden="true" />
            <div className="v5-skel v5-bx__list" aria-hidden="true" />
          </div>
        ) : error ? (
          <div className="v5-state" role="alert" data-testid="v5-brands-error">
            <p>{error}</p>
            <button type="button" className="v5-btn v5-btn--line" onClick={onRetry}>
              Повторить
            </button>
          </div>
        ) : !lead ? (
          <div className="v5-state" data-testid="v5-brands-empty">
            <p>Бренды пока не опубликованы.</p>
          </div>
        ) : (
          <div className={`v5-bx${rest.length === 0 ? ' v5-bx--solo' : ''}`} data-testid="v5-brands-index">
            <Link to={brandHref(lead.brand)} className="v5-bx__lead" data-testid={`v5-brand-${lead.brand.id}`}>
              <span className="v5-bx__no">
                <span>BR-{archiveNumber(lead.index)}</span>
                {featuredBrandId === lead.brand.id && <span>Последнее поступление</span>}
              </span>
              <span className="v5-bx__body">
                {lead.brand.image && (
                  <img className="v5-brand__logo" src={lead.brand.image} alt="" loading="lazy" decoding="async" />
                )}
                <span className={`v5-bx__name${lead.brand.name.length > 32 ? ' v5-bx__name--xlong' : lead.brand.name.length > 18 ? ' v5-bx__name--long' : ''}`}>{lead.brand.name}</span>
                {lead.brand.description && <span className="v5-brand__desc">{lead.brand.description}</span>}
              </span>
              <span className="v5-bx__go">
                <span>Страница бренда</span>
                <span aria-hidden="true">→</span>
              </span>
            </Link>

            {rest.length > 0 && (
              <ol className="v5-bx__list">
                {rest.map(({ brand, index }) => (
                  <li key={brand.id}>
                    <Link to={brandHref(brand)} className="v5-bx__row" data-testid={`v5-brand-${brand.id}`}>
                      <span className="v5-bx__rno">BR-{archiveNumber(index)}</span>
                      <span className="v5-bx__rname">{brand.name}</span>
                      <span className="v5-bx__arrow" aria-hidden="true">→</span>
                    </Link>
                  </li>
                ))}
                {hiddenCount > 0 && (
                  <li>
                    <Link to="/brands" className="v5-bx__row v5-bx__row--more" data-testid="v5-brands-more">
                      <span className="v5-bx__rno">+{hiddenCount}</span>
                      <span className="v5-bx__rname">Ещё бренды</span>
                      <span className="v5-bx__arrow" aria-hidden="true">→</span>
                    </Link>
                  </li>
                )}
              </ol>
            )}
          </div>
        )}
      </div>
    </section>
  );
}

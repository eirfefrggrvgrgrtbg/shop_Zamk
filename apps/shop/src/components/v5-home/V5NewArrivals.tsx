import { Link } from 'react-router-dom';
import type { Product } from '../../types/catalog';
import { V5CardSkeleton, V5ProductCard } from './V5ProductCard';
import { V5_ARRIVALS_LIMIT } from './v5Selection';

interface V5NewArrivalsProps {
  products: Product[];
  totalCount: number;
  isLoading: boolean;
  error: string;
  onRetry: () => void;
}

export function V5NewArrivals({ products, totalCount, isLoading, error, onRetry }: V5NewArrivalsProps) {
  return (
    <section id="v5-arrivals" className="v5-sec" aria-labelledby="v5-arrivals-title" data-testid="v5-arrivals">
      <div className="v5-wrap">
        <div className="v5-sec__hd">
          <div>
            <p className="v5-kick">01 — новые поступления</p>
            <h2 id="v5-arrivals-title">
              Новинки <em>архива</em>
            </h2>
            <p>Последние вещи, добавленные в каталог ZAMK.</p>
          </div>
          <Link to="/new" className="v5-more">
            Все новинки <span aria-hidden="true">→</span>
          </Link>
        </div>

        {isLoading ? (
          <div className="v5-cards" aria-busy="true" aria-label="Загрузка новинок">
            {Array.from({ length: V5_ARRIVALS_LIMIT }, (_, i) => (
              <V5CardSkeleton key={i} />
            ))}
          </div>
        ) : error ? (
          <div className="v5-state" role="alert" data-testid="v5-arrivals-error">
            <p>{error}</p>
            <button type="button" className="v5-btn v5-btn--line" onClick={onRetry}>
              Повторить
            </button>
          </div>
        ) : products.length === 0 ? (
          <div className="v5-state" data-testid="v5-arrivals-empty">
            <p>Новинок пока нет. Загляните в каталог позже.</p>
            <Link to="/catalog" className="v5-btn v5-btn--line" style={{ marginTop: 18 }}>
              Открыть каталог <span aria-hidden="true">→</span>
            </Link>
          </div>
        ) : (
          <div className="v5-cards">
            {products.map((product, i) => (
              <V5ProductCard key={product.id} product={product} index={i} placement="v5_home_new_arrivals" showNew />
            ))}
            <Link to="/catalog" className="v5-allnew" data-testid="v5-arrivals-catalog-cta">
              <div>
                <p className="v5-kick">Каталог</p>
                {totalCount > 0 && <p className="v5-allnew__big">{totalCount}</p>}
                <h3>
                  Больше вещей <em>в каталоге</em>
                </h3>
                <p>Откройте полный каталог и найдите свою вещь.</p>
              </div>
              <span className="v5-allnew__go">
                <span>Перейти в каталог</span>
                <span aria-hidden="true">→</span>
              </span>
            </Link>
          </div>
        )}
      </div>
    </section>
  );
}

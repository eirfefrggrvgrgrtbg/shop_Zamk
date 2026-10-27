import { Link } from 'react-router-dom';
import { V5CardSkeleton, V5ProductCard } from './V5ProductCard';
import type { V5ForYouSection } from './v5Selection';
import { V5_FOR_YOU_LIMIT } from './v5Selection';

interface V5ForYouProps {
  section: V5ForYouSection | null;
  isLoading: boolean;
}

function splitTitle(title: string): [string, string] {
  const [first, ...rest] = title.split(' ');
  return rest.length > 0 ? [first, rest.join(' ')] : [title, ''];
}

const KICKERS = {
  for_you: '02 — персональная подборка',
  popular: '02 — популярное',
  selection: '02 — выбор ZAMK',
} as const;

/** Editorial chapter: text and action on the left, one leading card and up to two supporting cards on the right. */
export function V5ForYou({ section, isLoading }: V5ForYouProps) {
  if (!isLoading && !section) return null;

  const [head, tail] = splitTitle(section?.title ?? 'Подборка');
  const items = section?.items ?? [];

  return (
    <section
      id="v5-foryou"
      className="v5-sec"
      aria-labelledby="v5-foryou-title"
      data-testid="v5-for-you"
      data-kind={section?.kind ?? 'loading'}
    >
      <div className="v5-wrap">
        <div className="v5-fy">
          <div className="v5-fy__side">
            <p className="v5-kick">{section ? KICKERS[section.kind] : '02 — подборка'}</p>
            <h2 id="v5-foryou-title">
              {tail ? (
                <>
                  {head} <em>{tail}</em>
                </>
              ) : (
                <em>{head}</em>
              )}
            </h2>
            <p>{section?.description ?? ''}</p>
            <Link to="/catalog" className="v5-btn v5-btn--solid">
              Смотреть каталог <span aria-hidden="true">→</span>
            </Link>
          </div>

          {isLoading || !section ? (
            <div className="v5-fy__rail" data-count={V5_FOR_YOU_LIMIT} aria-busy="true" aria-label="Загрузка подборки">
              {Array.from({ length: V5_FOR_YOU_LIMIT }, (_, i) => (
                <V5CardSkeleton key={i} />
              ))}
            </div>
          ) : (
            <div className="v5-fy__rail" data-count={items.length} data-testid="v5-for-you-rail">
              {items.map((product, i) => (
                <V5ProductCard
                  key={product.id}
                  product={product}
                  index={i + 10}
                  placement={`v5_home_${section.kind}`}
                  lead={i === 0}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

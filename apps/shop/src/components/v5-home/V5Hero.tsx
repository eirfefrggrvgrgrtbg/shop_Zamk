import { Link } from 'react-router-dom';
import type { Category } from '../../types/catalog';

interface V5HeroProps {
  categories: Category[];
  totalCount: number;
}

/** Static silhouette: one path, flat gradients, no filters, so the first paint stays cheap. */
function Silhouette() {
  return (
    <svg viewBox="780 90 540 940" preserveAspectRatio="xMidYMax meet" aria-hidden="true" focusable="false">
      <defs>
        <linearGradient id="v5-fig-fill" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#1a1420" />
          <stop offset="0.38" stopColor="#4a3550" />
          <stop offset="0.55" stopColor="#2c2032" />
          <stop offset="1" stopColor="#120e16" />
        </linearGradient>
        <linearGradient id="v5-fig-shade" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#000" stopOpacity="0" />
          <stop offset="1" stopColor="#000" stopOpacity="0.55" />
        </linearGradient>
        <linearGradient id="v5-rim" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#f2eaf0" stopOpacity="0.75" />
          <stop offset="0.5" stopColor="#f2eaf0" stopOpacity="0.35" />
          <stop offset="1" stopColor="#b696a8" stopOpacity="0.7" />
        </linearGradient>
        <radialGradient id="v5-head" cx="70%" cy="35%" r="80%">
          <stop offset="0" stopColor="#4c414b" />
          <stop offset="0.55" stopColor="#1a161a" />
          <stop offset="1" stopColor="#0d0b0e" />
        </radialGradient>
        <radialGradient id="v5-floor" cx="50%" cy="50%" r="50%">
          <stop offset="0" stopColor="#000" stopOpacity="0.7" />
          <stop offset="1" stopColor="#000" stopOpacity="0" />
        </radialGradient>
        <clipPath id="v5-fig-clip">
          <path d="M 948 180 L 948 236 C 944 262 930 276 900 284 C 870 292 846 298 842 318 C 840 350 860 372 868 400 C 878 440 896 470 896 500 C 896 540 876 570 874 600 C 870 690 846 800 818 920 C 812 950 806 980 800 1010 L 1290 1010 C 1270 990 1250 960 1230 930 C 1160 840 1082 740 1050 620 C 1044 600 1024 560 1024 500 C 1024 470 1042 440 1052 400 C 1060 372 1080 350 1078 318 C 1074 298 1050 292 1020 284 C 990 276 976 262 974 236 L 974 180 Z" />
        </clipPath>
      </defs>
      <ellipse cx="1040" cy="1000" rx="380" ry="46" fill="url(#v5-floor)" />
      <g clipPath="url(#v5-fig-clip)">
        <rect x="760" y="160" width="560" height="880" fill="url(#v5-fig-fill)" />
        <rect x="760" y="160" width="560" height="880" fill="url(#v5-fig-shade)" />
        <g stroke="#d8bdd0" strokeOpacity="0.2" strokeWidth="1">
          {Array.from({ length: 22 }, (_, i) => {
            const x = 800 + i * 24;
            return <line key={x} x1={x} y1="160" x2={x + (i - 11) * 5} y2="1040" />;
          })}
        </g>
      </g>
      <path
        d="M 948 180 L 948 236 C 944 262 930 276 900 284 C 870 292 846 298 842 318 C 840 350 860 372 868 400 C 878 440 896 470 896 500 C 896 540 876 570 874 600 C 870 690 846 800 818 920 C 812 950 806 980 800 1010 L 1290 1010 C 1270 990 1250 960 1230 930 C 1160 840 1082 740 1050 620 C 1044 600 1024 560 1024 500 C 1024 470 1042 440 1052 400 C 1060 372 1080 350 1078 318 C 1074 298 1050 292 1020 284 C 990 276 976 262 974 236 L 974 180 Z"
        fill="none"
        stroke="url(#v5-rim)"
        strokeWidth="1.6"
      />
      <ellipse cx="961" cy="138" rx="30" ry="42" fill="url(#v5-head)" stroke="url(#v5-rim)" strokeWidth="1.2" transform="rotate(-8 961 138)" />
    </svg>
  );
}

export function V5Hero({ categories, totalCount }: V5HeroProps) {
  return (
    <section className="v5-hero" aria-labelledby="v5-hero-title">
      <div className="v5-wrap v5-hero__inner">
        <div className="v5-hero__figure">
          <Silhouette />
        </div>
        <div className="v5-hero__figtag" aria-hidden="true">СИЛУЭТ ZK·001</div>

        <div className="v5-hero__copy">
          <p className="v5-kick">ZAMK · архив независимой моды</p>
          <h1 id="v5-hero-title">
            Строгость, <em>которую</em> хочется носить
          </h1>
          <p className="v5-hero__lede">Вещи независимых брендов в одном каталоге. Выбирайте размер и цвет на странице товара.</p>
          <div className="v5-ctas">
            <Link to="/catalog" className="v5-btn v5-btn--solid">
              Открыть каталог <span aria-hidden="true">→</span>
            </Link>
            <a href="#v5-arrivals" className="v5-btn v5-btn--line">
              Новинки
            </a>
          </div>
        </div>

        <nav className="v5-hero__index" aria-label="Разделы каталога">
          <span className="v5-hero__thread" aria-hidden="true" />
          <Link to="/catalog" className="v5-ctag">
            <span className="v5-ctag__h">ZAMK | КАТАЛОГ</span>
            <b>Каталог</b>
            <span className="v5-ctag__n">{totalCount > 0 ? `Вещей: ${totalCount}` : '\u00A0'}</span>
            <span className="v5-bars" aria-hidden="true" />
          </Link>
          {categories.length > 0 && (
            <ol className="v5-hero__list">
              {categories.map((category, i) => (
                <li key={category.id}>
                  <Link to={`/catalog?categoryId=${encodeURIComponent(category.id)}`}>
                    <span>{String(i + 1).padStart(2, '0')}</span>
                    <span>{category.name}</span>
                  </Link>
                </li>
              ))}
            </ol>
          )}
          <Link to="/catalog" className="v5-hero__all">
            <span>Весь каталог</span>
            <span aria-hidden="true">→</span>
          </Link>
        </nav>
      </div>

      <div className="v5-jump">
        <div className="v5-wrap">
          <nav className="v5-jump__row" aria-label="Разделы страницы">
            <a href="#v5-arrivals"><b>01</b>Новинки</a>
            <a href="#v5-foryou"><b>02</b>Подборка</a>
            <a href="#v5-brands"><b>03</b>Бренды</a>
          </nav>
        </div>
      </div>
    </section>
  );
}

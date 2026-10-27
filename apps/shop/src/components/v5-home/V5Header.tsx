import { Link } from 'react-router-dom';
import { Heart, Search, ShoppingBag } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { useCart } from '../../contexts/CartContext';
import { useFavorites } from '../../contexts/FavoritesContext';
import { useSearch } from '../../contexts/SearchContext';

export const V5_HOME_PATH = '/v5-preview';

export function V5Header() {
  const { isAuthenticated, openAuthModal } = useAuth();
  const { totalItems } = useCart();
  const { favoriteIds } = useFavorites();
  const { openSearch } = useSearch();

  return (
    <div className="v5-top">
      <div className="v5-wrap">
        <header className="v5-nav">
          <nav className="v5-nav__links" aria-label="Основная навигация">
            <Link to="/catalog">Каталог</Link>
            <Link to="/zamk">Вещи ZAMK</Link>
            <Link to="/brands">Бренды</Link>
          </nav>
          <Link to={V5_HOME_PATH} className="v5-wordmark" aria-label="ZAMK — главная">
            ZAMK
          </Link>
          <div className="v5-nav__tools">
            <button type="button" className="v5-tool" onClick={openSearch} aria-label="Поиск">
              <Search aria-hidden="true" />
            </button>
            {isAuthenticated ? (
              <Link to="/account" className="v5-tool v5-tool--text">
                Кабинет
              </Link>
            ) : (
              <button type="button" className="v5-tool v5-tool--text" onClick={() => openAuthModal('login')}>
                Вход
              </button>
            )}
            <Link to="/favorites" className="v5-tool" aria-label="Избранное">
              <Heart aria-hidden="true" />
              {favoriteIds.length > 0 && <span className="v5-badge">{favoriteIds.length}</span>}
            </Link>
            <Link to="/cart" className="v5-tool" aria-label={`Корзина, товаров: ${totalItems}`}>
              <ShoppingBag aria-hidden="true" />
              {totalItems > 0 && <span className="v5-badge">{totalItems}</span>}
            </Link>
          </div>
        </header>
      </div>
    </div>
  );
}

import { Link } from 'react-router-dom';
import { V5_HOME_PATH } from './V5Header';

export function V5Footer() {
  return (
    <footer className="v5-footer">
      <div className="v5-wrap">
        <div className="v5-footer__cols">
          <div>
            <Link to={V5_HOME_PATH} className="v5-wordmark" aria-label="ZAMK — главная">
              ZAMK
            </Link>
            <p className="v5-footer__about">Цифровая витрина независимых брендов и альтернативной эстетики.</p>
          </div>
          <nav aria-label="Магазин">
            <h3>Магазин</h3>
            <ul>
              <li><Link to="/catalog">Каталог</Link></li>
              <li><Link to="/new">Новинки</Link></li>
              <li><Link to="/collections">Подборки</Link></li>
              <li><Link to="/brands">Бренды</Link></li>
              <li><Link to="/zamk">Вещи ZAMK</Link></li>
            </ul>
          </nav>
          <nav aria-label="Информация">
            <h3>Информация</h3>
            <ul>
              <li><Link to="/about">О витрине</Link></li>
              <li><Link to="/delivery">Доставка</Link></li>
              <li><Link to="/returns">Возврат и обмен</Link></li>
              <li><Link to="/help">Вопросы и ответы</Link></li>
              <li><Link to="/contacts">Контакты</Link></li>
            </ul>
          </nav>
        </div>
        <div className="v5-footer__base">
          <span>© {new Date().getFullYear()} ZAMK</span>
          <Link to="/privacy">Политика конфиденциальности</Link>
        </div>
      </div>
    </footer>
  );
}

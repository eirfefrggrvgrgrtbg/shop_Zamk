import { NavLink } from 'react-router-dom';
import './marketing.css';

export function AdminMarketingTabs() {
  return <nav className="m-tabs" aria-label="Разделы маркетинга">
    <NavLink to="/marketing/overview">Сводка</NavLink>
    <NavLink to="/marketing/campaigns">Кампании</NavLink>
    <NavLink to="/marketing/sources">Источники</NavLink>
    <NavLink to="/marketing/products">Товары</NavLink>
    <NavLink to="/marketing/designers">Дизайнеры</NavLink>
    <NavLink to="/marketing/query-builder">Конструктор отчетов</NavLink>
  </nav>;
}

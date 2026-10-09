import { useEffect, useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { getAdminCampaigns, getAdminMarketingCampaignMetrics } from '@zamk/api-client/src/admin';
import type { AdminCampaign, AdminCampaignMetrics, CampaignChannel, CampaignType } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { money, number, percent, date, statusLabels, channelLabels, typeLabels, purposeLabels } from '../components/marketing/marketingPresentation';

export function AdminMarketingCampaigns() {
  const navigate = useNavigate();
  const { hasPermission } = useAdminAuth();
  const canWrite = hasPermission('marketing.campaigns.write');
  const [revision, setRevision] = useState(0);

  const [metrics, setMetrics] = useState<AdminCampaignMetrics[] | null>(null);
  const [campaigns, setCampaigns] = useState<AdminCampaign[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [query, setQuery] = useState('');
  const [purposeFilter, setPurposeFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [channelFilter, setChannelFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');

  // Canonical rolling 30-day half-open period for campaign analytics
  const { fromStr, toStr } = useMemo(() => {
    const toDate = new Date();
    const fromDate = new Date(toDate);
    fromDate.setUTCDate(fromDate.getUTCDate() - 30);
    return { fromStr: fromDate.toISOString(), toStr: toDate.toISOString() };
  }, [revision]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const [camps, met] = await Promise.all([
          getAdminCampaigns(),
          getAdminMarketingCampaignMetrics(fromStr, toStr)
        ]);
        if (active) {
          setCampaigns(camps);
          setMetrics(met.campaigns);
        }
      } catch (err: any) {
        if (active) setError(err.message || 'Не удалось загрузить кампании');
      } finally {
        if (active) setIsLoading(false);
      }
    };
    load();
    return () => { active = false; };
  }, [revision, fromStr, toStr]);

  const handleRowClick = (e: React.MouseEvent, campaignId: string) => {
    const selection = window.getSelection();
    if (selection && selection.toString().length > 0) return;
    const target = e.target as HTMLElement | null;
    if (target?.closest('a, button, input, select, textarea')) return;
    navigate(`/marketing/campaigns/${campaignId}`);
  };

  const filteredCampaigns = useMemo(() => {
    return campaigns.filter(c => {
      if (purposeFilter && purposeFilter !== 'all' && c.purpose !== purposeFilter) return false;
      if (query && !c.title.toLowerCase().includes(query.toLowerCase())) return false;
      if (statusFilter && statusFilter !== 'all' && c.status !== statusFilter) return false;
      if (channelFilter && channelFilter !== 'all' && c.campaignChannel !== channelFilter) return false;
      if (typeFilter && typeFilter !== 'all' && c.campaignType !== typeFilter) return false;
      return true;
    });
  }, [campaigns, query, purposeFilter, statusFilter, channelFilter, typeFilter]);

  const formatPeriod = (startsAt?: string | null, endsAt?: string | null) => {
    if (startsAt && endsAt) return `${date(startsAt)} — ${date(endsAt)}`;
    if (startsAt) return `с ${date(startsAt)}`;
    if (endsAt) return `до ${date(endsAt)}`;
    return 'Бессрочно';
  };

  return (
    <div className="marketing-workspace space-y-6 pb-12" data-testid="admin-marketing-campaigns-page">
      <AdminMarketingTabs />

      <div className="m-header">
        <div>
          <h1>Кампании</h1>
          <p>Управление рекламными и промо-активностями платформы</p>
        </div>
        <div className="flex items-center gap-3">
          {canWrite && (
            <Link
              to="/marketing/campaigns/new"
              data-testid="create-campaign-button"
              className="m-button m-primary"
            >
              Создать кампанию
            </Link>
          )}
        </div>
      </div>

      <div className="m-filters">
        <input
          type="search"
          role="searchbox"
          placeholder="Поиск кампаний..."
          value={query}
          onChange={e => setQuery(e.target.value)}
        />
        <select
          aria-label="Назначение"
          value={purposeFilter}
          onChange={e => setPurposeFilter(e.target.value)}
        >
          <option value="">Все назначения</option>
          <option value="advertising">Рекламные кампании</option>
          <option value="promotion">Промо-кампании</option>
        </select>
        <select
          aria-label="Статус"
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
        >
          <option value="">Все статусы</option>
          <option value="active">Активные</option>
          <option value="draft">Черновики</option>
          <option value="ended">Завершенные</option>
        </select>
        <select
          aria-label="Канал"
          value={channelFilter}
          onChange={e => setChannelFilter(e.target.value)}
        >
          <option value="">Все каналы</option>
          <option value="telegram">Telegram</option>
          <option value="vk">VK</option>
          <option value="instagram">Instagram</option>
          <option value="influencer">Блогер</option>
          <option value="search_ads">Поисковая реклама</option>
          <option value="direct">Прямой трафик</option>
          <option value="email">Email</option>
        </select>
        <select
          aria-label="Тип"
          value={typeFilter}
          onChange={e => setTypeFilter(e.target.value)}
        >
          <option value="">Все типы</option>
          <option value="influencer">Инфлюенсер</option>
          <option value="drop">Дроп / Запуск</option>
          <option value="seasonal_sale">Распродажа</option>
          <option value="brand_awareness">Узнаваемость</option>
          <option value="retargeting">Ретаргетинг</option>
          <option value="special_promo">Специальная акция</option>
        </select>
      </div>

      {error && !isLoading && (
        <div className="m-error" role="alert">
          <span>{error}</span>
          <button onClick={() => setRevision(r => r + 1)}>Повторить</button>
        </div>
      )}

      {isLoading ? (
        <div data-testid="campaigns-loading" className="p-12 text-center text-sm text-gray-500">Загрузка кампаний...</div>
      ) : error ? null : campaigns.length === 0 ? (
        <div className="m-panel">
          <div className="m-empty">
            <strong className="text-base text-gray-900 font-medium">Кампаний пока нет</strong>
            <p className="text-gray-500 text-sm max-w-md mx-auto mt-2 mb-6">
              Создайте первую рекламную кампанию, сгенерируйте трекинговую ссылку и отслеживайте отдачу в реальном времени.
            </p>
            {canWrite && (
              <Link to="/marketing/campaigns/new" data-testid="create-campaign-button" className="m-button m-primary">
                Создать кампанию
              </Link>
            )}
          </div>
        </div>
      ) : (
        <div className="m-panel">
          <div className="m-table-scroll">
            <table className="m-table" role="table">
              <thead>
                <tr>
                  <th>Кампания</th>
                  <th>Статус</th>
                  <th>Период</th>
                  <th className="numeric">Плановый бюджет</th>
                  <th className="numeric">Сессии</th>
                  <th className="numeric">Заказы</th>
                  <th className="numeric">Выручка</th>
                  <th className="numeric">Конверсия</th>
                </tr>
              </thead>
              <tbody>
                {filteredCampaigns.map(c => {
                  const met = metrics?.find(m => m.campaignId === c.id);
                  const humanPurpose = purposeLabels[c.purpose] ?? (c.purpose === 'promotion' ? 'Промо-кампания' : 'Рекламная кампания');

                  return (
                    <tr
                      key={c.id}
                      data-testid={`campaign-row-${c.id}`}
                      onClick={(e) => handleRowClick(e, c.id)}
                      className="m-clickable"
                    >
                      <td className="max-w-md">
                        <Link
                          to={`/marketing/campaigns/${c.id}`}
                          className="m-name block truncate"
                          title={c.title}
                        >
                          {c.title}
                        </Link>
                        <div className="m-meta flex items-center gap-1.5 flex-wrap">
                          <span className="text-gray-700 font-medium">{humanPurpose}</span>
                          <span className="text-gray-300">•</span>
                          <span>{c.sellerId ? 'Продавец' : 'ZAMK Платформа'}</span>
                          {c.campaignChannel && (
                            <>
                              <span className="text-gray-300">•</span>
                              <span>{channelLabels[c.campaignChannel as CampaignChannel] ?? c.campaignChannel}</span>
                            </>
                          )}
                          {c.campaignType && (
                            <>
                              <span className="text-gray-300">•</span>
                              <span>{typeLabels[c.campaignType as CampaignType] ?? c.campaignType}</span>
                            </>
                          )}
                          {c.trackingLinkCount !== undefined && c.trackingLinkCount > 0 && (
                            <>
                              <span className="text-gray-300">•</span>
                              <span>{c.trackingLinkCount} {c.trackingLinkCount === 1 ? 'ссылка' : c.trackingLinkCount < 5 ? 'ссылки' : 'ссылок'}</span>
                            </>
                          )}
                        </div>
                      </td>
                      <td>
                        <span className="m-chip" data-status={c.status}>
                          {statusLabels[c.status] ?? c.status}
                        </span>
                      </td>
                      <td className="tabular-nums text-gray-600">
                        {formatPeriod(c.startsAt, c.endsAt)}
                      </td>
                      <td className="numeric">
                        {c.plannedBudgetCents ? money(c.plannedBudgetCents) : '—'}
                      </td>
                      <td className="numeric">
                        {number(met?.visits)}
                      </td>
                      <td className="numeric">
                        {number(met?.paidOrders)}
                      </td>
                      <td className="numeric">
                        {money(met?.revenueCents)}
                      </td>
                      <td className="numeric">
                        {met?.visits && met.visits > 0 && met.conversionRateBps !== undefined ? percent(met.conversionRateBps) : '—'}
                      </td>
                    </tr>
                  );
                })}
                {filteredCampaigns.length === 0 && (
                  <tr>
                    <td colSpan={8} className="m-empty">
                      <strong>Ничего не найдено</strong>
                      <p className="text-gray-500 text-xs mt-1">Попробуйте изменить параметры поиска или сбросить фильтры</p>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

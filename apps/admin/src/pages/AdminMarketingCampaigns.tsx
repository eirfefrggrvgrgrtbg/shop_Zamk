import { useEffect, useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { getAdminCampaigns, getAdminMarketingCampaignMetrics } from '@zamk/api-client/src/admin';
import type { AdminCampaign, AdminCampaignMetrics, CampaignChannel, CampaignType } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { money, number, percent, date, channelLabels, typeLabels } from '../components/marketing/marketingPresentation';

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
          setCampaigns(camps.filter(c => c.purpose === 'advertising'));
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
      if (c.purpose !== 'advertising') return false;
      if (query && !c.title.toLowerCase().includes(query.toLowerCase())) return false;
      if (statusFilter && statusFilter !== 'all' && c.status !== statusFilter) return false;
      if (channelFilter && channelFilter !== 'all' && c.campaignChannel !== channelFilter) return false;
      if (typeFilter && typeFilter !== 'all' && c.campaignType !== typeFilter) return false;
      return true;
    });
  }, [campaigns, query, statusFilter, channelFilter, typeFilter]);

  return (
    <div className="max-w-[1200px] mx-auto px-8 py-8 min-h-screen" data-testid="admin-marketing-campaigns-page">
      <div className="mb-8">
        <AdminMarketingTabs />
      </div>

      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-xl font-medium tracking-tight text-gray-900">Кампании</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-1">Управление рекламными активностями</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-4">
          {canWrite && (
            <Link
              to="/marketing/campaigns/new"
              data-testid="create-campaign-button"
              className="bg-gray-900 text-white px-4 py-2 text-[11px] font-bold uppercase tracking-widest hover:bg-gray-800 transition-colors"
            >
              Создать кампанию
            </Link>
          )}
        </div>
      </div>

      <div className="mb-12">
        <div className="flex flex-wrap lg:flex-nowrap items-end gap-6 mb-8">
          <div className="flex-1 max-w-sm">
            <div className="relative">
              <input
                type="search"
                role="searchbox"
                placeholder="ПОИСК КАМПАНИЙ..."
                value={query}
                onChange={e => setQuery(e.target.value)}
                className="w-full bg-transparent border-b border-gray-900 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-gray-900"
              />
              <svg className="w-3 h-3 absolute right-0 top-1 text-gray-900" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="square" strokeLinejoin="miter" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path></svg>
            </div>
          </div>
          <div className="flex flex-wrap items-baseline gap-6">
            <select
              aria-label="Статус"
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value)}
              className="w-48 bg-transparent border-b border-gray-200 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 outline-none focus:border-gray-900 cursor-pointer"
            >
              <option value="">ВСЕ СТАТУСЫ</option>
              <option value="active">АКТИВНЫЕ</option>
              <option value="draft">ЧЕРНОВИК</option>
              <option value="ended">ЗАВЕРШЕННЫЕ</option>
            </select>
            <select
              aria-label="Канал"
              value={channelFilter}
              onChange={e => setChannelFilter(e.target.value)}
              className="w-48 bg-transparent border-b border-gray-200 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 outline-none focus:border-gray-900 cursor-pointer"
            >
              <option value="">ВСЕ КАНАЛЫ</option>
              <option value="telegram">TELEGRAM</option>
              <option value="vk">VK</option>
              <option value="instagram">INSTAGRAM</option>
              <option value="influencer">БЛОГЕР</option>
            </select>
            <select
              aria-label="Тип"
              value={typeFilter}
              onChange={e => setTypeFilter(e.target.value)}
              className="w-48 bg-transparent border-b border-gray-200 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 outline-none focus:border-gray-900 cursor-pointer"
            >
              <option value="">ВСЕ ТИПЫ</option>
              <option value="influencer">ИНФЛЮЕНСЕР</option>
              <option value="drop">ДРОП / ЗАПУСК</option>
              <option value="seasonal_sale">РАСПРОДАЖА</option>
              <option value="brand_awareness">УЗНАВАЕМОСТЬ</option>
            </select>
          </div>
        </div>
      </div>

      {error && !isLoading && (
        <div className="p-4 bg-red-50 text-red-800 border-b border-red-100 flex justify-between items-center" role="alert">
          <span>{error}</span>
          <button onClick={() => setRevision(r => r + 1)} className="font-medium hover:underline">Повторить</button>
        </div>
      )}

      {isLoading ? (
        <div data-testid="campaigns-loading" className="p-8 text-center text-sm text-gray-500">Загрузка кампаний...</div>
      ) : error ? null : campaigns.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center p-8 bg-white">
          <div className="w-full max-w-md text-center">
            <div className="w-16 h-16 bg-purple-50 rounded-full flex items-center justify-center mx-auto mb-6">
              <svg className="w-8 h-8 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M11 3.055A9.001 9.001 0 1020.945 13H11V3.055z"></path><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M20.488 9H15V3.512A9.025 9.025 0 0120.488 9z"></path></svg>
            </div>
            <h2 className="text-xl font-serif text-gray-900 mb-3">Кампаний пока нет</h2>
            <p className="text-gray-500 mb-8 text-sm leading-relaxed">
              Создайте первую рекламную кампанию, получите трекинговую ссылку и начните измерять результат. Кампания → Трекинговая ссылка → Трафик → Заказы
            </p>
            {canWrite && (
              <Link to="/marketing/campaigns/new" data-testid="create-campaign-button" className="inline-flex items-center justify-center bg-[#5B21B6] text-white px-6 py-2.5 rounded font-medium hover:bg-purple-800 transition-colors shadow-sm w-full">
                Создать кампанию
              </Link>
            )}
          </div>
        </div>
      ) : (
        <div className="flex-1 overflow-auto bg-white relative">
          <table className="w-full text-left border-collapse min-w-[1000px]" role="table">
            <thead className="bg-gray-50/80 sticky top-0 border-b border-gray-200 z-10 text-xs uppercase tracking-wider text-gray-500 font-medium">
              <tr>
                <th className="px-6 py-3 font-medium">Кампания</th>
                <th className="px-6 py-3 font-medium">Статус</th>
                <th className="px-6 py-3 font-medium">Период</th>
                <th className="px-6 py-3 font-medium text-right">Бюджет</th>
                <th className="px-6 py-3 font-medium text-right">Сессии</th>
                <th className="px-6 py-3 font-medium text-right">Заказы</th>
                <th className="px-6 py-3 font-medium text-right">Выручка</th>
                <th className="px-6 py-3 font-medium text-right">Конверсия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 text-sm">
              {filteredCampaigns.map(c => {
                const met = metrics?.find(m => m.campaignId === c.id);
                const isActive = c.status === 'active';
                const isDraft = c.status === 'draft';
                const isCompleted = c.status === 'ended';

                return (
                  <tr
                    key={c.id}
                    data-testid={`campaign-row-${c.id}`}
                    onClick={(e) => handleRowClick(e, c.id)}
                    className="hover:bg-gray-50 cursor-pointer group"
                  >
                    <td className="px-6 py-4">
                      <Link to={`/marketing/campaigns/${c.id}`} className="font-medium text-gray-900 group-hover:text-purple-700 transition-colors focus:outline-none">{c.title}</Link>
                      <div className="text-xs text-gray-500 mt-0.5">
                        <span>{c.sellerId ? 'Продавец' : 'ZAMK Платформа'}</span> <span className="text-gray-300">•</span> <span>{channelLabels[c.campaignChannel as CampaignChannel] ?? c.campaignChannel}</span> <span className="text-gray-300">•</span> <span>{typeLabels[c.campaignType as CampaignType] ?? c.campaignType}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex px-2.5 py-0.5 rounded-full text-[11px] font-medium border ${isActive ? 'bg-green-50 text-green-700 border-green-200' : isCompleted ? 'bg-gray-100 text-gray-600 border-gray-200' : isDraft ? 'bg-gray-50 text-gray-500 border-gray-200' : 'bg-gray-50 text-gray-500 border-gray-200'}`}>
                        {isActive ? 'Активна' : isCompleted ? 'Завершена' : isDraft ? 'Черновик' : 'Неизвестно'}
                      </span>
                    </td>
                    <td className="px-6 py-4 tabular-nums text-gray-600">
                      {c.startsAt ? date(c.startsAt) : 'с —'} — {c.endsAt ? date(c.endsAt) : '∞'}
                    </td>
                    <td className="px-6 py-4 text-right tabular-nums">
                      {c.plannedBudgetCents ? money(c.plannedBudgetCents) : '—'}
                    </td>
                    <td className="px-6 py-4 text-right tabular-nums">
                      {number(met?.visits)}
                    </td>
                    <td className="px-6 py-4 text-right tabular-nums">
                      {number(met?.paidOrders)}
                    </td>
                    <td className="px-6 py-4 text-right tabular-nums">
                      {money(met?.revenueCents)}
                    </td>
                    <td className="px-6 py-4 text-right tabular-nums">
                      {percent(met?.conversionRateBps)}
                    </td>
                  </tr>
                );
              })}
              {filteredCampaigns.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-6 py-8 text-center text-gray-500 text-sm">
                    Ничего не найдено
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

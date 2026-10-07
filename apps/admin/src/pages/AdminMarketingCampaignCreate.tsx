import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { createAdminCampaign } from '@zamk/api-client/src/admin';
import type { CampaignChannel, CampaignType, AdminSeller } from '@zamk/api-client/src/types';
import { getAdminSellers } from '@zamk/api-client/src/admin';

export function AdminMarketingCampaignCreate() {
  const navigate = useNavigate();

  // Form State
  const [title, setTitle] = useState('');
  const [isPlatform, setIsPlatform] = useState(true);
  const [sellerId, setSellerId] = useState('');
  const [channel, setChannel] = useState<CampaignChannel>('telegram');
  const [campaignType, setCampaignType] = useState<CampaignType>('brand_awareness');
  const [plannedBudgetRub, setPlannedBudgetRub] = useState('');
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [description, setDescription] = useState('');

  // UI State
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Sellers
  const [sellers, setSellers] = useState<AdminSeller[]>([]);
  const [sellerQuery, setSellerQuery] = useState('');
  const [isSellerDropdownOpen, setIsSellerDropdownOpen] = useState(false);
  const [selectedSellerName, setSelectedSellerName] = useState('');

  useEffect(() => {
    let active = true;
    getAdminSellers({ limit: 100, status: ['active'], search: '' }).then(data => {
      if (active) setSellers(data.items);
    }).catch(console.error);
    return () => { active = false; };
  }, []);

  const filteredSellers = sellers.filter(s => {
    const q = sellerQuery.toLowerCase();
    return (s.brandName && s.brandName.toLowerCase().includes(q)) ||
           (s.ownerName && s.ownerName.toLowerCase().includes(q)) ||
           (s.ownerEmail && s.ownerEmail.toLowerCase().includes(q));
  });

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { setFormError('Укажите название кампании'); return; }
    if (!isPlatform && !sellerId) { setFormError('Выберите продавца'); return; }
    if (startsAt && endsAt && new Date(endsAt) < new Date(startsAt)) { setFormError('Дата окончания не может быть раньше даты начала'); return; }

    setIsSubmitting(true);
    setFormError(null);
    try {
      const bCents = plannedBudgetRub ? Math.round(parseFloat(plannedBudgetRub) * 100) : null;
      const res = await createAdminCampaign({
        title: title.trim(),
        fundingMode: isPlatform ? 'zamk' : 'seller',
        discountType: 'percent',
        sellerDiscountBps: 0,
        sellerId: isPlatform ? null : sellerId,
        campaignChannel: channel,
        campaignType: campaignType,
        plannedBudgetCents: bCents ?? undefined,
        startsAt: startsAt ? new Date(startsAt).toISOString() : undefined,
        endsAt: endsAt ? new Date(endsAt).toISOString() : undefined,
        description: description.trim() || undefined,
      });
      navigate(`/marketing/campaigns/${res.id}`);
    } catch (err: any) {
      setFormError(err.message || 'Не удалось создать кампанию');
      setIsSubmitting(false);
    }
  };

  const channelLabels: Record<string, string> = {
    telegram: 'Telegram', vk: 'VK', instagram: 'Instagram', influencer: 'Блогер',
    email: 'Email', direct: 'Прямой трафик', search_ads: 'Поисковая реклама',
  };
  const typeLabels: Record<string, string> = {
    influencer: 'Инфлюенсер', drop: 'Дроп / Запуск', seasonal_sale: 'Сезонная распродажа',
    brand_awareness: 'Узнаваемость бренда', retargeting: 'Ретаргетинг', special_promo: 'Специальная акция',
  };

  const budgetFormatted = plannedBudgetRub ? new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 }).format(parseFloat(plannedBudgetRub)) : '0 ₽';

  return (
    <div className="flex-1 flex flex-col h-full bg-[#FAFAFA]" data-testid="admin-marketing-campaign-create-page">
      <header className="px-8 py-6 border-b border-gray-200 bg-white shrink-0">
        <div className="flex items-center gap-3 mb-6 text-sm">
          <Link to="/marketing/campaigns" className="text-gray-500 hover:text-gray-900 transition-colors">Кампании</Link>
          <span className="text-gray-300">/</span>
          <span className="text-gray-900 font-medium">Новая кампания</span>
        </div>
        <div className="flex justify-between items-end">
          <h1 className="text-3xl font-serif text-gray-900">Создание кампании</h1>
        </div>
      </header>

      <div className="flex-1 overflow-auto flex flex-col md:flex-row relative">
        <div className="flex-1 p-8 lg:p-12 overflow-auto relative">
          <div className="max-w-3xl mx-auto pb-12">
            {formError && (
              <div className="bg-red-50 text-red-800 p-4 rounded-md mb-8 border border-red-100 flex items-start gap-3" role="alert">
                <svg className="w-5 h-5 mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                <div className="text-sm font-medium">{formError}</div>
              </div>
            )}

            {/* Section 1: Campaign */}
            <section className="mb-12">
              <h2 className="text-lg font-serif mb-5 text-gray-900">Кампания</h2>
              <div className="space-y-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Название кампании</label>
                  <input type="text" data-testid="campaign-title-input" value={title} onChange={e => setTitle(e.target.value)} className="w-full text-base bg-white border border-gray-300 rounded-md px-4 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow text-gray-900" placeholder="Например: Autumn Sale 2024" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Владелец</label>
                  <div className="flex gap-6 mb-4">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="radio" data-testid="campaign-owner-platform" checked={isPlatform} onChange={() => setIsPlatform(true)} className="text-purple-600 focus:ring-purple-600" />
                      <span className="text-sm text-gray-900">ZAMK Платформа</span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="radio" data-testid="campaign-owner-seller" checked={!isPlatform} onChange={() => setIsPlatform(false)} className="text-purple-600 focus:ring-purple-600" />
                      <span className="text-sm text-gray-900">Продавец</span>
                    </label>
                  </div>
                  {!isPlatform && (
                    <div className="relative">
                      <input type="text" data-testid="seller-search-input" value={isSellerDropdownOpen ? sellerQuery : selectedSellerName} onChange={e => { setSellerQuery(e.target.value); setIsSellerDropdownOpen(true); }} onFocus={() => setIsSellerDropdownOpen(true)} className="w-full text-base bg-white border border-gray-300 rounded-md px-4 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow text-gray-900" placeholder="Найти продавца..." />
                      <svg className="w-4 h-4 text-gray-400 absolute right-3 top-3.5 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path></svg>
                      {isSellerDropdownOpen && (
                        <div className="absolute top-full left-0 w-full mt-1 bg-white border border-gray-100 shadow-lg rounded-md py-2 z-20 max-h-60 overflow-y-auto">
                          {filteredSellers.map(s => (
                            <div key={s.id} data-testid={`seller-option-${s.id}`} className="px-4 py-2 hover:bg-gray-50 cursor-pointer" onClick={() => { setSellerId(s.id); setSelectedSellerName(s.brandName || s.ownerName); setIsSellerDropdownOpen(false); }}>
                              <p className="text-sm font-medium text-gray-900">{s.brandName || s.ownerName}</p>
                              <p className="text-xs text-gray-500">{s.ownerEmail}</p>
                            </div>
                          ))}
                          {filteredSellers.length === 0 && <div className="px-4 py-2 text-sm text-gray-500">Ничего не найдено</div>}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </section>

            {/* Section 2: Placement */}
            <section className="mb-12">
              <h2 className="text-lg font-serif mb-5 text-gray-900">Размещение</h2>
              <div className="grid grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Канал</label>
                  <div className="relative">
                    <select aria-label="Канал" value={channel} onChange={e => setChannel(e.target.value as CampaignChannel)} className="w-full text-base bg-white border border-gray-300 rounded-md px-4 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow appearance-none text-gray-900">
                      <option value="telegram">Telegram</option>
                      <option value="vk">VK</option>
                      <option value="instagram">Instagram</option>
                      <option value="search_ads">Поисковая реклама</option>
                      <option value="influencer">Инфлюенсер</option>
                      <option value="direct">Direct</option>
                    </select>
                    <svg className="w-4 h-4 text-gray-400 absolute right-3 top-3.5 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path></svg>
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Тип кампании</label>
                  <div className="relative">
                    <select aria-label="Тип кампании" value={campaignType} onChange={e => setCampaignType(e.target.value as CampaignType)} className="w-full text-base bg-white border border-gray-300 rounded-md px-4 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow appearance-none text-gray-900">
                      <option value="brand_awareness">Узнаваемость бренда</option>
                      <option value="seasonal_sale">Сезонная распродажа</option>
                      <option value="drop">Дроп / Запуск</option>
                      <option value="retargeting">Ретаргетинг</option>
                    </select>
                    <svg className="w-4 h-4 text-gray-400 absolute right-3 top-3.5 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path></svg>
                  </div>
                </div>
              </div>
            </section>

            {/* Section 3: Plan */}
            <section className="mb-12">
              <h2 className="text-lg font-serif mb-5 text-gray-900">План</h2>
              <div className="grid grid-cols-2 gap-6">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Плановый бюджет</label>
                  <div className="relative">
                    <input type="text" inputMode="numeric" data-testid="campaign-budget-input" value={plannedBudgetRub} onChange={e => setPlannedBudgetRub(e.target.value.replace(/\D/g, ''))} className="w-full text-base bg-white border border-gray-300 rounded-md pl-4 pr-8 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow tabular-nums text-gray-900" placeholder="0" />
                    <span className="absolute right-4 top-2.5 text-gray-400">₽</span>
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Период</label>
                  <div className="flex items-center gap-2">
                    <input type="date" aria-label="Дата начала" value={startsAt} onChange={e => setStartsAt(e.target.value)} className="w-full text-base bg-white border border-gray-300 rounded-md px-3 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow text-sm text-gray-900" />
                    <span className="text-gray-400">—</span>
                    <input type="date" aria-label="Дата окончания" value={endsAt} onChange={e => setEndsAt(e.target.value)} className="w-full text-base bg-white border border-gray-300 rounded-md px-3 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow text-sm text-gray-900" />
                  </div>
                </div>
              </div>
            </section>

            {/* Section 4: Intent/Goal */}
            <section className="mb-12">
              <h2 className="text-lg font-serif mb-5 text-gray-900">Цель (необязательно)</h2>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Описание или бизнес-цель</label>
                <textarea value={description} onChange={e => setDescription(e.target.value)} className="w-full text-base bg-white border border-gray-300 rounded-md px-4 py-3 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 transition-shadow min-h-[100px] resize-y text-gray-900" placeholder="Дополнительное описание кампании..."></textarea>
              </div>
            </section>

            <div className="flex items-center gap-4 pt-6 border-t border-gray-200">
              <button onClick={handleCreate} disabled={isSubmitting} data-testid="submit-create-campaign" className="bg-[#5B21B6] text-white px-6 py-2.5 rounded font-medium hover:bg-purple-800 transition-colors shadow-sm disabled:opacity-50">
                {isSubmitting ? 'Создание...' : 'Создать кампанию'}
              </button>
              <Link to="/marketing/campaigns" className="px-6 py-2.5 text-sm font-medium text-gray-500 hover:text-gray-900 transition-colors">
                Отмена
              </Link>
            </div>
          </div>
        </div>

        {/* Right: Live Summary (30-35%) */}
        <div className="w-80 lg:w-96 bg-white border-l border-gray-200 p-8 flex flex-col shadow-[-4px_0_24px_rgba(0,0,0,0.02)] z-10 shrink-0 sticky top-0 h-full overflow-y-auto">
          <h3 className="text-sm font-medium uppercase tracking-wider text-gray-500 mb-6">Сводка кампании</h3>

          <div className="flex-1 space-y-6">
            <div>
              <div className="text-xs text-gray-400 mb-1">Название</div>
              <div className="text-gray-900 font-medium break-words">{title || '—'}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400 mb-1">Владелец</div>
              <div className="text-gray-900">{isPlatform ? 'ZAMK Платформа' : (selectedSellerName || 'Не выбран')}</div>
            </div>
            <hr className="border-gray-100" />
            <div>
              <div className="text-xs text-gray-400 mb-1">Канал / Тип</div>
              <div className="text-gray-500 text-sm">{channelLabels[channel]} / {typeLabels[campaignType]}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400 mb-1">Период</div>
              <div className="text-gray-900 tabular-nums">
                {startsAt ? new Date(startsAt).toLocaleDateString('ru-RU') : 'с —'} — {endsAt ? new Date(endsAt).toLocaleDateString('ru-RU') : '∞'}
              </div>
            </div>
            <div>
              <div className="text-xs text-gray-400 mb-1">Плановый бюджет</div>
              <div className="text-gray-900 font-medium tabular-nums">{budgetFormatted}</div>
            </div>
          </div>

          <div className="mt-8 pt-6 border-t border-gray-100">
            <h4 className="text-xs font-medium text-gray-900 mb-3">После создания будут доступны:</h4>
            <ul className="space-y-2 text-xs text-gray-600">
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-600 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7"></path></svg>
                Трекинговая ссылка
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-600 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7"></path></svg>
                Атрибуция заказов
              </li>
              <li className="flex items-center gap-2">
                <svg className="w-4 h-4 text-green-600 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7"></path></svg>
                Аналитика трафика
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { createAdminCampaign, getAdminSellers } from '@zamk/api-client/src/admin';
import type { CampaignChannel, CampaignType, AdminSeller } from '@zamk/api-client/src/types';
import { money } from '../components/marketing/marketingPresentation';

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
    getAdminSellers({ limit: 100, status: ['active'], search: '' })
      .then(data => {
        if (active) setSellers(data.items);
      })
      .catch(console.error);
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
    const scrollToTop = () => {
      if (typeof document !== 'undefined') {
        const m = document.querySelector('main');
        if (m) m.scrollTop = 0;
      }
      if (typeof window !== 'undefined' && window.scrollTo) window.scrollTo({ top: 0, behavior: 'smooth' });
    };

    if (!title.trim()) {
      setFormError('Укажите название кампании');
      scrollToTop();
      return;
    }
    if (!isPlatform && !sellerId) {
      setFormError('Выберите продавца');
      scrollToTop();
      return;
    }
    if (startsAt && endsAt && new Date(endsAt) < new Date(startsAt)) {
      setFormError('Дата окончания не может быть раньше даты начала');
      scrollToTop();
      return;
    }

    setIsSubmitting(true);
    setFormError(null);
    try {
      const bCents = plannedBudgetRub ? Math.round(parseFloat(plannedBudgetRub) * 100) : null;
      const res = await createAdminCampaign({
        purpose: 'advertising',
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
    telegram: 'Telegram',
    vk: 'VK',
    instagram: 'Instagram',
    influencer: 'Блогер',
    email: 'Email',
    direct: 'Прямой трафик',
    search_ads: 'Поисковая реклама',
  };

  const typeLabels: Record<string, string> = {
    influencer: 'Инфлюенсер',
    drop: 'Дроп / Запуск',
    seasonal_sale: 'Сезонная распродажа',
    brand_awareness: 'Узнаваемость бренда',
    retargeting: 'Ретаргетинг',
    special_promo: 'Специальная акция',
  };

  const budgetCents = plannedBudgetRub ? Math.round(parseFloat(plannedBudgetRub) * 100) : null;
  const budgetFormatted = budgetCents !== null ? money(budgetCents) : '0 ₽';

  return (
    <div className="marketing-workspace space-y-6 pb-12" data-testid="admin-marketing-campaign-create-page">
      {/* Breadcrumb Navigation */}
      <div className="flex items-center gap-2 text-xs text-gray-500 pt-4">
        <Link to="/marketing/campaigns" className="hover:text-gray-900 transition-colors">Кампании</Link>
        <span className="text-gray-300">/</span>
        <span className="text-gray-900 font-medium">Новая кампания</span>
      </div>

      {/* Header with human advertising context */}
      <div className="m-header border-b border-gray-100 pb-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900">Создание кампании</h1>
          <div className="flex items-center gap-2 mt-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-purple-700 bg-purple-50 px-2 py-0.5 rounded border border-purple-100">
              Рекламная кампания
            </span>
            <span className="text-xs text-gray-500">Привлечение трафика и аналитика переходов платформы</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Left: Form Content */}
        <div className="lg:col-span-2">
          {formError && (
            <div className="m-error mb-6" role="alert">
              <span>{formError}</span>
            </div>
          )}

          <form onSubmit={handleCreate} className="space-y-8">
            {/* Section 1: Campaign Basic Info */}
            <section className="border-b border-gray-100 pb-8 space-y-5">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Основное</h2>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1.5">Название кампании</label>
                <input
                  type="text"
                  data-testid="campaign-title-input"
                  value={title}
                  onChange={e => setTitle(e.target.value)}
                  className="w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                  placeholder="Например: Autumn Sale 2026"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-gray-700 mb-2">Владелец</label>
                <div className="flex gap-6 mb-3">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="radio"
                      data-testid="campaign-owner-platform"
                      name="campaign-owner"
                      checked={isPlatform}
                      onChange={() => setIsPlatform(true)}
                      className="text-purple-600 focus:ring-purple-600"
                    />
                    <span className="text-sm text-gray-900">ZAMK Платформа</span>
                  </label>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="radio"
                      data-testid="campaign-owner-seller"
                      name="campaign-owner"
                      checked={!isPlatform}
                      onChange={() => setIsPlatform(false)}
                      className="text-purple-600 focus:ring-purple-600"
                    />
                    <span className="text-sm text-gray-900">Продавец</span>
                  </label>
                </div>

                {!isPlatform && (
                  <div className="relative">
                    <input
                      type="text"
                      data-testid="seller-search-input"
                      value={isSellerDropdownOpen ? sellerQuery : selectedSellerName}
                      onChange={e => {
                        setSellerQuery(e.target.value);
                        setIsSellerDropdownOpen(true);
                      }}
                      onFocus={() => setIsSellerDropdownOpen(true)}
                      className="w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                      placeholder="Найти продавца..."
                    />
                    {isSellerDropdownOpen && (
                      <div className="absolute top-full left-0 w-full mt-1 bg-white border border-gray-200 shadow-lg rounded-lg py-1.5 z-20 max-h-60 overflow-y-auto">
                        {filteredSellers.map(s => (
                          <div
                            key={s.id}
                            data-testid={`seller-option-${s.id}`}
                            className="px-3.5 py-2 hover:bg-purple-50 cursor-pointer"
                            onClick={() => {
                              setSellerId(s.id);
                              setSelectedSellerName(s.brandName || s.ownerName);
                              setIsSellerDropdownOpen(false);
                            }}
                          >
                            <p className="text-sm font-medium text-gray-900">{s.brandName || s.ownerName}</p>
                            <p className="text-xs text-gray-500">{s.ownerEmail}</p>
                          </div>
                        ))}
                        {filteredSellers.length === 0 && (
                          <div className="px-3.5 py-2 text-xs text-gray-500">Ничего не найдено</div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </section>

            {/* Section 2: Placement */}
            <section className="border-b border-gray-100 pb-8 space-y-5">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Размещение</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Канал</label>
                  <select
                    aria-label="Канал"
                    value={channel}
                    onChange={e => setChannel(e.target.value as CampaignChannel)}
                    className="w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                  >
                    <option value="telegram">Telegram</option>
                    <option value="vk">VK</option>
                    <option value="instagram">Instagram</option>
                    <option value="influencer">Блогер</option>
                    <option value="search_ads">Поисковая реклама</option>
                    <option value="direct">Прямой трафик</option>
                    <option value="email">Email</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Тип кампании</label>
                  <select
                    aria-label="Тип кампании"
                    value={campaignType}
                    onChange={e => setCampaignType(e.target.value as CampaignType)}
                    className="w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                  >
                    <option value="brand_awareness">Узнаваемость бренда</option>
                    <option value="seasonal_sale">Сезонная распродажа</option>
                    <option value="drop">Дроп / Запуск</option>
                    <option value="retargeting">Ретаргетинг</option>
                    <option value="influencer">Инфлюенсер</option>
                    <option value="special_promo">Специальная акция</option>
                  </select>
                </div>
              </div>
            </section>

            {/* Section 3: Plan */}
            <section className="border-b border-gray-100 pb-8 space-y-5">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">План</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Плановый бюджет</label>
                  <div className="relative">
                    <input
                      type="text"
                      inputMode="numeric"
                      data-testid="campaign-budget-input"
                      value={plannedBudgetRub}
                      onChange={e => setPlannedBudgetRub(e.target.value.replace(/\D/g, ''))}
                      className="w-full text-sm border border-gray-200 rounded-lg pl-3.5 pr-8 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 tabular-nums text-gray-900 bg-white"
                      placeholder="0"
                    />
                    <span className="absolute right-3 top-2.5 text-gray-400 text-sm">₽</span>
                  </div>
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-700 mb-1.5">Период</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="date"
                      aria-label="Дата начала"
                      value={startsAt}
                      onChange={e => setStartsAt(e.target.value)}
                      className="w-full text-xs border border-gray-200 rounded-lg px-2.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                    />
                    <span className="text-gray-300">—</span>
                    <input
                      type="date"
                      aria-label="Дата окончания"
                      value={endsAt}
                      onChange={e => setEndsAt(e.target.value)}
                      className="w-full text-xs border border-gray-200 rounded-lg px-2.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white"
                    />
                  </div>
                </div>
              </div>
            </section>

            {/* Section 4: Intent/Goal */}
            <section className="space-y-5">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Цель (необязательно)</h2>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1.5">Описание или бизнес-цель</label>
                <textarea
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  className="w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 min-h-[90px] resize-y text-gray-900 bg-white"
                  placeholder="Дополнительное описание кампании..."
                />
              </div>
            </section>

            <div className="flex items-center gap-4 pt-4 border-t border-gray-100">
              <button
                type="submit"
                disabled={isSubmitting}
                data-testid="submit-create-campaign"
                className="m-button m-primary"
              >
                {isSubmitting ? 'Создание...' : 'Создать кампанию'}
              </button>
              <Link to="/marketing/campaigns" className="m-button">
                Отмена
              </Link>
            </div>
          </form>
        </div>

        {/* Right: Live Summary Sidebar */}
        <div className="lg:col-span-1">
          <div className="m-panel p-6 sticky top-6 space-y-5">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500">Сводка кампании</h3>

            <div className="space-y-4 text-xs">
              <div>
                <div className="text-gray-400 mb-1">Название</div>
                <div className="text-gray-900 font-medium break-words">{title || '—'}</div>
              </div>

              <div>
                <div className="text-gray-400 mb-1">Назначение</div>
                <div className="text-purple-700 font-medium">Рекламная кампания</div>
              </div>

              <div>
                <div className="text-gray-400 mb-1">Владелец</div>
                <div className="text-gray-900">{isPlatform ? 'ZAMK Платформа' : (selectedSellerName || 'Не выбран')}</div>
              </div>

              <hr className="border-gray-100" />

              <div>
                <div className="text-gray-400 mb-1">Канал / Тип</div>
                <div className="text-gray-700">{channelLabels[channel]} / {typeLabels[campaignType]}</div>
              </div>

              <div>
                <div className="text-gray-400 mb-1">Период</div>
                <div className="text-gray-900 tabular-nums">
                  {startsAt ? new Date(startsAt).toLocaleDateString('ru-RU') : 'с —'} — {endsAt ? new Date(endsAt).toLocaleDateString('ru-RU') : '∞'}
                </div>
              </div>

              <div>
                <div className="text-gray-400 mb-1">Плановый бюджет</div>
                <div className="text-gray-900 font-medium tabular-nums">{budgetFormatted}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

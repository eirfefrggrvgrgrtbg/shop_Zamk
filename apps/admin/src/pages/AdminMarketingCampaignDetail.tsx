import '../components/marketing/marketing.css';
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Megaphone,
  Plus,
  Copy,
  Check,
  Ban,
  Store,
  Compass,
  X,
  Calendar,
} from 'lucide-react';

import {
  getAdminCampaign,
  updateAdminCampaign,
  getAdminCampaignTrackingLinks,
  createAdminTrackingLink,
  disableAdminTrackingLink,
} from '@zamk/api-client/src/admin';
import type {
  AdminCampaign,
  AdminCampaignTrackingLink,
  CampaignTargetType,
  CampaignStatus,
} from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  draft: { label: 'Черновик', color: 'bg-gray-100 text-gray-800' },
  submitted: { label: 'На рассмотрении', color: 'bg-yellow-100 text-yellow-800' },
  counter_offered: { label: 'Контрпредложение', color: 'bg-purple-100 text-purple-800' },
  approved: { label: 'Одобрена', color: 'bg-blue-100 text-blue-800' },
  active: { label: 'Активна', color: 'bg-green-100 text-green-800' },
  ended: { label: 'Завершена', color: 'bg-gray-200 text-gray-700' },
  rejected: { label: 'Отклонена', color: 'bg-red-100 text-red-800' },
  cancelled: { label: 'Отменена', color: 'bg-red-200 text-red-900' },
};

const CHANNEL_LABELS: Record<string, string> = {
  telegram: 'Telegram',
  vk: 'VKontakte',
  instagram: 'Instagram',
  influencer: 'Блогер / Инфлюенсер',
  email: 'Email-рассылка',
  direct: 'Прямой трафик',
  search_ads: 'Поисковая реклама',
};

const TYPE_LABELS: Record<string, string> = {
  influencer: 'Инфлюенсер',
  drop: 'Дроп / Запуск',
  seasonal_sale: 'Сезонная распродажа',
  brand_awareness: 'Узнаваемость бренда',
  retargeting: 'Ретаргетинг',
  special_promo: 'Специальная акция',
};

export function AdminMarketingCampaignDetail() {
  const { id } = useParams<{ id: string }>();
  const [campaign, setCampaign] = useState<AdminCampaign | null>(null);
  const [trackingLinks, setTrackingLinks] = useState<AdminCampaignTrackingLink[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [copiedToken, setCopiedToken] = useState<string | null>(null);

  // Link create modal
  const [showCreateLinkModal, setShowCreateLinkModal] = useState(false);
  const [targetType, setTargetType] = useState<CampaignTargetType>('landing');
  const [targetProductId, setTargetProductId] = useState('');
  const [targetSellerId, setTargetSellerId] = useState('');
  const [landingPath, setLandingPath] = useState('/catalog');
  const [promoCodeId, setPromoCodeId] = useState('');
  const [linkFormError, setLinkFormError] = useState<string | null>(null);
  const [isSubmittingLink, setIsSubmittingLink] = useState(false);

  const { hasPermission } = useAdminAuth();

  const loadData = async () => {
    if (!id) return;
    try {
      setIsLoading(true);
      setError(null);
      const [camp, links] = await Promise.all([
        getAdminCampaign(id),
        getAdminCampaignTrackingLinks(id),
      ]);
      setCampaign(camp);
      setTrackingLinks(Array.isArray(links) ? links : []);
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить данные кампании');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [id]);

  const handleCopyLink = (token: string) => {
    const fullUrl = `${window.location.origin}/r/${token}`;
    navigator.clipboard.writeText(fullUrl);
    setCopiedToken(token);
    setTimeout(() => setCopiedToken(null), 2000);
  };

  const handleStatusChange = async (newStatus: CampaignStatus) => {
    if (!id) return;
    try {
      setError(null);
      const updated = await updateAdminCampaign(id, { status: newStatus });
      setCampaign(updated);
      setSuccess(`Статус кампании обновлен: ${STATUS_LABELS[newStatus]?.label || newStatus}`);
      setTimeout(() => setSuccess(null), 3000);
    } catch (err: any) {
      setError(err?.message || 'Не удалось обновить статус кампании');
    }
  };

  const handleDisableLink = async (linkId: string) => {
    if (!id) return;
    if (!window.confirm('Отключить эту трекинговую ссылку? Переходы по ней будут заблокированы.')) {
      return;
    }

    try {
      setError(null);
      await disableAdminTrackingLink(id, linkId);
      setSuccess('Трекинговая ссылка успешно отключена');
      setTimeout(() => setSuccess(null), 3000);
      loadData();
    } catch (err: any) {
      setError(err?.message || 'Не удалось отключить ссылку');
    }
  };

  const handleCreateLink = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setLinkFormError(null);

    if (targetType === 'product' && !targetProductId.trim()) {
      setLinkFormError('Укажите ID товара');
      return;
    }
    if (targetType === 'seller' && !targetSellerId.trim()) {
      setLinkFormError('Укажите ID продавца');
      return;
    }
    if (targetType === 'landing' && !landingPath.trim()) {
      setLinkFormError('Укажите относительный путь посадочной страницы (начинается с /)');
      return;
    }

    try {
      setIsSubmittingLink(true);
      await createAdminTrackingLink(id, {
        targetType,
        targetProductId: targetType === 'product' ? targetProductId.trim() : undefined,
        targetSellerId: targetType === 'seller' ? targetSellerId.trim() : undefined,
        landingPath: targetType === 'landing' ? landingPath.trim() : undefined,
        promoCodeId: promoCodeId.trim() || undefined,
      });

      setSuccess('Трекинговая ссылка успешно создана');
      setShowCreateLinkModal(false);
      resetLinkForm();
      loadData();
      setTimeout(() => setSuccess(null), 3000);
    } catch (err: any) {
      setLinkFormError(err?.message || 'Не удалось создать трекинговую ссылку');
    } finally {
      setIsSubmittingLink(false);
    }
  };

  const resetLinkForm = () => {
    setTargetType('landing');
    setTargetProductId('');
    setTargetSellerId('');
    setLandingPath('/catalog');
    setPromoCodeId('');
    setLinkFormError(null);
  };

  const formatMoney = (cents?: number | null) => {
    if (cents === undefined || cents === null) return '—';
    return `₽ ${(cents / 100).toLocaleString('ru-RU', { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`;
  };

  const formatDate = (dateStr?: string | null) => {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64" data-testid="campaign-detail-loading">
        <p className="text-gray-500">Загрузка информации о кампании…</p>
      </div>
    );
  }

  if (!campaign) {
    return (
      <div className="p-8 text-center text-gray-500" data-testid="campaign-not-found">
        <h3 className="text-lg font-bold text-gray-900 mb-2">Кампания не найдена</h3>
        <Link to="/marketing/campaigns" className="text-indigo-600 hover:text-indigo-800">
          Вернуться к списку кампаний
        </Link>
      </div>
    );
  }

  const statusInfo = STATUS_LABELS[campaign.status] || { label: campaign.status, color: 'bg-gray-100 text-gray-800' };

  return (
    <div className="marketing-workspace space-y-5" data-testid="admin-marketing-campaign-detail-page">
      {/* Back button and Header */}
      <div>
        <Link
          to="/marketing/campaigns"
          className="inline-flex items-center text-sm font-medium text-gray-500 hover:text-gray-700 mb-3"
          data-testid="back-to-campaigns"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Назад к списку кампаний
        </Link>

        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center space-x-3">
            <div className="p-2.5 bg-indigo-50 border border-indigo-200 rounded-xl text-indigo-600">
              <Megaphone className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h1 className="text-2xl font-bold text-gray-900" data-testid="campaign-title">
                  {campaign.title}
                </h1>
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusInfo.color}`} data-testid="campaign-status-badge">
                  {statusInfo.label}
                </span>
              </div>
              <p className="text-sm text-gray-500 mt-0.5">
                ID: <span className="font-mono text-xs">{campaign.id}</span>
              </p>
            </div>
          </div>

          {/* Quick status actions for write users */}
          {hasPermission('marketing.campaigns.write') && (
            <div className="flex items-center space-x-2">
              {campaign.status !== 'active' && (
                <button
                  onClick={() => handleStatusChange('active')}
                  data-testid="activate-campaign-button"
                  className="px-3 py-1.5 bg-green-600 text-white rounded-md text-xs font-medium hover:bg-green-700 transition-colors shadow-2xs"
                >
                  Активировать
                </button>
              )}
              {campaign.status === 'active' && (
                <button
                  onClick={() => handleStatusChange('approved')}
                  data-testid="pause-campaign-button"
                  className="px-3 py-1.5 bg-amber-600 text-white rounded-md text-xs font-medium hover:bg-amber-700 transition-colors shadow-2xs"
                >
                  Приостановить
                </button>
              )}
              {campaign.status !== 'ended' && campaign.status !== 'cancelled' && (
                <button
                  onClick={() => handleStatusChange('ended')}
                  data-testid="end-campaign-button"
                  className="px-3 py-1.5 bg-gray-600 text-white rounded-md text-xs font-medium hover:bg-gray-700 transition-colors shadow-2xs"
                >
                  Завершить
                </button>
              )}
            </div>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm" data-testid="detail-error">
          {error}
        </div>
      )}

      {success && (
        <div className="p-4 bg-green-50 border border-green-200 rounded-md text-green-700 text-sm" data-testid="detail-success">
          {success}
        </div>
      )}

      {/* Campaign Details Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-2xs">
          <div className="text-xs font-medium text-gray-500 uppercase">Владелец</div>
          <div className="mt-1 font-semibold text-gray-900 flex items-center">
            {campaign.sellerId ? (
              <span className="text-amber-700 flex items-center">
                <Store className="h-4 w-4 mr-1 text-amber-500" />
                Продавец ({campaign.sellerId.substring(0, 8)}…)
              </span>
            ) : (
              <span className="text-indigo-700">ZAMK Платформа</span>
            )}
          </div>
        </div>

        <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-2xs">
          <div className="text-xs font-medium text-gray-500 uppercase">Канал и тип</div>
          <div className="mt-1 font-semibold text-gray-900">
            {campaign.campaignChannel ? CHANNEL_LABELS[campaign.campaignChannel] || campaign.campaignChannel : '—'}
            {' · '}
            <span className="text-gray-500 font-normal">
              {campaign.campaignType ? TYPE_LABELS[campaign.campaignType] || campaign.campaignType : '—'}
            </span>
          </div>
        </div>

        <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-2xs">
          <div className="text-xs font-medium text-gray-500 uppercase">Планируемый бюджет</div>
          <div className="mt-1 font-bold text-gray-900 text-lg">
            {formatMoney(campaign.plannedBudgetCents)}
          </div>
        </div>

        <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-2xs">
          <div className="text-xs font-medium text-gray-500 uppercase">Сроки проведения</div>
          <div className="mt-1 text-sm text-gray-700 flex items-center">
            <Calendar className="h-4 w-4 mr-1 text-gray-400" />
            {formatDate(campaign.startsAt)} — {formatDate(campaign.endsAt)}
          </div>
        </div>
      </div>

      {campaign.description && (
        <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-2xs">
          <div className="text-xs font-medium text-gray-500 uppercase mb-1">Описание кампании</div>
          <p className="text-sm text-gray-700 whitespace-pre-wrap">{campaign.description}</p>
        </div>
      )}

      {/* Tracking Links Section */}
      <div className="bg-white rounded-lg border border-gray-200 shadow-2xs overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center bg-gray-50/50">
          <div>
            <h2 className="text-base font-bold text-gray-900">
              Трекинговые ссылки ({trackingLinks.length})
            </h2>
            <p className="text-xs text-gray-500 mt-0.5">
              Канонические короткие ссылки с защищенным токеном перенаправления и UTM-разметкой
            </p>
          </div>

          {hasPermission('marketing.campaigns.write') && (
            <button
              onClick={() => {
                resetLinkForm();
                setShowCreateLinkModal(true);
              }}
              data-testid="create-tracking-link-button"
              className="flex items-center px-3.5 py-1.5 bg-violet-700 text-white rounded-md text-sm font-medium hover:bg-violet-800 transition-colors shadow-2xs"
            >
              <Plus className="h-4 w-4 mr-1" />
              Создать ссылку
            </button>
          )}
        </div>

        {trackingLinks.length === 0 ? (
          <div className="p-8 text-center text-gray-500" data-testid="no-tracking-links-message">
            <Compass className="mx-auto h-10 w-10 text-gray-300 mb-2" />
            <p className="text-sm font-medium text-gray-700">У этой кампании пока нет трекинговых ссылок</p>
            <p className="text-xs text-gray-400 mt-1">Создайте ссылку для распределения трафика и отслеживания заказов ADS.1</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 text-sm">
              <thead className="bg-gray-50 text-gray-600 uppercase text-xs">
                <tr>
                  <th className="px-6 py-3 text-left font-semibold">Токен и ссылка</th>
                  <th className="px-6 py-3 text-left font-semibold">Тип цели</th>
                  <th className="px-6 py-3 text-left font-semibold">Параметры цели</th>
                  <th className="px-6 py-3 text-left font-semibold">Промокод</th>
                  <th className="px-6 py-3 text-left font-semibold">Статус</th>
                  <th className="px-6 py-3 text-left font-semibold">Создана</th>
                  <th className="px-6 py-3 text-right font-semibold">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {trackingLinks.map((link) => {
                  const fullUrl = `${window.location.origin}/r/${link.token}`;
                  return (
                    <tr key={link.id} className="hover:bg-gray-50 transition-colors" data-testid={`tracking-link-row-${link.id}`}>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="font-mono text-xs font-semibold text-gray-900 bg-gray-100 px-2 py-1 rounded inline-block">
                          {link.token}
                        </div>
                        <div className="flex items-center space-x-2 mt-1.5">
                          <span className="text-xs text-gray-500 font-mono truncate max-w-xs">{fullUrl}</span>
                          <button
                            onClick={() => handleCopyLink(link.token)}
                            data-testid={`copy-link-${link.id}`}
                            className="inline-flex items-center text-xs text-indigo-600 hover:text-indigo-800"
                            title="Скопировать ссылку"
                          >
                            {copiedToken === link.token ? (
                              <span className="text-green-600 flex items-center font-medium">
                                <Check className="h-3 w-3 mr-0.5" />
                                Скопировано!
                              </span>
                            ) : (
                              <span className="flex items-center">
                                <Copy className="h-3 w-3 mr-0.5" />
                                Копировать
                              </span>
                            )}
                          </button>
                        </div>
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-gray-700">
                        {link.targetType === 'product' && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200">
                            Товар
                          </span>
                        )}
                        {link.targetType === 'seller' && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-50 text-amber-700 border border-amber-200">
                            Продавец
                          </span>
                        )}
                        {link.targetType === 'landing' && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-50 text-purple-700 border border-purple-200">
                            Лендинг
                          </span>
                        )}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-xs font-mono text-gray-600">
                        {link.targetType === 'product' && (link.targetProductId || '—')}
                        {link.targetType === 'seller' && (link.targetSellerId || '—')}
                        {link.targetType === 'landing' && (link.landingPath || '—')}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-500 font-mono">
                        {link.promoCodeId ? link.promoCodeId.substring(0, 8) + '…' : '—'}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap">
                        {link.isActive ? (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                            Активна
                          </span>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600">
                            Отключена
                          </span>
                        )}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-xs text-gray-500">
                        {formatDate(link.createdAt)}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-right text-xs">
                        {link.isActive && hasPermission('marketing.campaigns.write') && (
                          <button
                            onClick={() => handleDisableLink(link.id)}
                            data-testid={`disable-link-${link.id}`}
                            className="inline-flex items-center text-red-600 hover:text-red-900 font-medium"
                          >
                            <Ban className="h-3 w-3 mr-1" />
                            Отключить
                          </button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Create Tracking Link Modal */}
      {showCreateLinkModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/50 flex items-center justify-center p-4" data-testid="create-tracking-link-modal">
          <div className="bg-white rounded-xl shadow-xl max-w-md w-full p-6 border border-gray-200">
            <div className="flex justify-between items-center pb-3 border-b border-gray-100">
              <h3 className="text-lg font-bold text-gray-900">Создать трекинговую ссылку</h3>
              <button
                onClick={() => setShowCreateLinkModal(false)}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {linkFormError && (
              <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-xs" data-testid="link-modal-error">
                {linkFormError}
              </div>
            )}

            <form onSubmit={handleCreateLink} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                  Тип целевой страницы *
                </label>
                <select
                  value={targetType}
                  onChange={(e) => setTargetType(e.target.value as CampaignTargetType)}
                  data-testid="link-target-type-select"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500"
                >
                  <option value="landing">Произвольный URL магазина (Landing path)</option>
                  <option value="product">Товар (Product ID)</option>
                  <option value="seller">Витрина продавца (Seller ID)</option>
                </select>
              </div>

              {targetType === 'product' && (
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    ID Товара *
                  </label>
                  <input
                    type="text"
                    required
                    value={targetProductId}
                    onChange={(e) => setTargetProductId(e.target.value)}
                    placeholder="00000000-0000-0000-0000-000000000000"
                    data-testid="link-product-id-input"
                    className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500 font-mono text-xs"
                  />
                </div>
              )}

              {targetType === 'seller' && (
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    ID Продавца *
                  </label>
                  <input
                    type="text"
                    required
                    value={targetSellerId}
                    onChange={(e) => setTargetSellerId(e.target.value)}
                    placeholder="00000000-0000-0000-0000-000000000000"
                    data-testid="link-seller-id-input"
                    className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500 font-mono text-xs"
                  />
                </div>
              )}

              {targetType === 'landing' && (
                <div>
                  <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                    Относительный путь посадочной страницы *
                  </label>
                  <input
                    type="text"
                    required
                    value={landingPath}
                    onChange={(e) => setLandingPath(e.target.value)}
                    placeholder="/catalog/summer-sale"
                    data-testid="link-landing-path-input"
                    className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500 font-mono text-xs"
                  />
                  <p className="text-[11px] text-gray-400 mt-1">
                    Должен начинаться с <code>/</code>, не содержать протокол (<code>http:</code>) или сетевые пути (<code>//</code>).
                  </p>
                </div>
              )}

              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase mb-1">
                  ID Промокода (опционально)
                </label>
                <input
                  type="text"
                  value={promoCodeId}
                  onChange={(e) => setPromoCodeId(e.target.value)}
                  placeholder="ID промокода"
                  data-testid="link-promo-id-input"
                  className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500 font-mono text-xs"
                />
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-gray-100">
                <button
                  type="button"
                  onClick={() => setShowCreateLinkModal(false)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isSubmittingLink}
                  data-testid="submit-create-link"
                  className="px-4 py-2 bg-violet-700 text-white rounded-md text-sm font-medium hover:bg-violet-800 disabled:opacity-50"
                >
                  {isSubmittingLink ? 'Создание…' : 'Создать ссылку'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

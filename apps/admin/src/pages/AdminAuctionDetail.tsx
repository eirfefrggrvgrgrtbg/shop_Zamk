import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Play, Pause, XCircle, CheckCircle, Plus, Eye, Edit2, ShoppingCart } from 'lucide-react';
import { 
  getAdminAuction, 
  updateAdminAuction, 
  publishAdminAuction, 
  pauseAdminAuction, 
  resumeAdminAuction, 
  cancelAdminAuction, 
  finalizeAdminAuction,
  getAdminLots,
  moveLotToDirectSale
} from '@zamk/api-client/src/admin';
import type { AdminAuction, AdminAuctionLot } from '@zamk/api-client/src/types';
import { HelpTooltip } from '../components/HelpTooltip';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import { LotModal } from '../components/auctions/LotModal';
import { BidsModal } from '../components/auctions/BidsModal';

const STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  scheduled: 'Запланирован',
  live: 'Идёт',
  paused: 'На паузе',
  ended: 'Завершён',
  cancelled: 'Отменён',
};

const LOT_STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик',
  active: 'Активен',
  ended_no_bids: 'Без ставок',
  won_pending_payment: 'Победитель ждёт оплаты',
  paid: 'Оплачен',
  unpaid_manual_review: 'Ручное решение',
  moved_to_direct_sale: 'Переведён в продажу',
  cancelled: 'Отменён'
};

export function AdminAuctionDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAdminAuth();
  
  const [activeTab, setActiveTab] = useState<'settings' | 'lots'>('settings');
  const [auction, setAuction] = useState<AdminAuction | null>(null);
  const [lots, setLots] = useState<AdminAuctionLot[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const [isSubmitting, setIsSubmitting] = useState(false);
  
  // Modals state
  const [isLotModalOpen, setIsLotModalOpen] = useState(false);
  const [editingLot, setEditingLot] = useState<AdminAuctionLot | null>(null);
  const [isBidsModalOpen, setIsBidsModalOpen] = useState(false);
  const [selectedLotId, setSelectedLotId] = useState<string | null>(null);
  const [selectedLotTitle, setSelectedLotTitle] = useState('');

  // Form state
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [bidStepRubles, setBidStepRubles] = useState('100');
  const [paymentDeadlineHours, setPaymentDeadlineHours] = useState('24');
  const [antiSnipingEnabled, setAntiSnipingEnabled] = useState(false);
  const [antiSnipingTriggerSeconds, setAntiSnipingTriggerSeconds] = useState('300');
  const [antiSnipingExtensionSeconds, setAntiSnipingExtensionSeconds] = useState('300');
  const [maxBidsPerUserPerLotPerMinute, setMaxBidsPerUserPerLotPerMinute] = useState('10');
  const [maxRejectedBidsPerUserPerMinute, setMaxRejectedBidsPerUserPerMinute] = useState('5');
  const [noBidsPolicy, setNoBidsPolicy] = useState('manual_review');
  const [unpaidWinnerPolicy, setUnpaidWinnerPolicy] = useState('manual_review');
  const [isPublic, setIsPublic] = useState(false);
  const [showOnHomepage, setShowOnHomepage] = useState(false);
  const [highlightInNav, setHighlightInNav] = useState(false);
  const [biddingEnabled, setBiddingEnabled] = useState(true);

  const loadAuctionData = async () => {
    if (!id) return;
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminAuction(id);
      setAuction(data);
      
      setTitle(data.title);
      setDescription(data.description || '');
      // HTML datetime-local input expects YYYY-MM-DDThh:mm format
      setStartsAt(data.startsAt.substring(0, 16));
      setEndsAt(data.endsAt.substring(0, 16));
      
      setBidStepRubles((data.bidStepCents / 100).toString());
      setPaymentDeadlineHours(data.paymentDeadlineHours.toString());
      setAntiSnipingEnabled(data.antiSnipingEnabled);
      setAntiSnipingTriggerSeconds(data.antiSnipingTriggerSeconds.toString());
      setAntiSnipingExtensionSeconds(data.antiSnipingExtensionSeconds.toString());
      setMaxBidsPerUserPerLotPerMinute(data.maxBidsPerUserPerLotPerMinute.toString());
      setMaxRejectedBidsPerUserPerMinute(data.maxRejectedBidsPerUserPerMinute.toString());
      setNoBidsPolicy(data.noBidsPolicy);
      setUnpaidWinnerPolicy(data.unpaidWinnerPolicy);
      setIsPublic(data.isPublic);
      setShowOnHomepage(data.showOnHomepage);
      setHighlightInNav(data.highlightInNav);
      setBiddingEnabled(data.biddingEnabled);

      loadLots();
    } catch (err) {
      setError('Не удалось загрузить аукцион.');
    } finally {
      setIsLoading(false);
    }
  };

  const loadLots = async () => {
    if (!id) return;
    try {
      const data = await getAdminLots(id);
      setLots(data.items || []);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    loadAuctionData();
  }, [id]);

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setError(null);

    const stepCents = Math.round(parseFloat(bidStepRubles) * 100);
    const deadlineHours = parseInt(paymentDeadlineHours, 10);

    try {
      setIsSubmitting(true);
      const data: Partial<AdminAuction> = {
        title,
        description,
        startsAt: new Date(startsAt).toISOString(),
        endsAt: new Date(endsAt).toISOString(),
        bidStepCents: stepCents,
        paymentDeadlineHours: deadlineHours,
        antiSnipingEnabled,
        antiSnipingTriggerSeconds: parseInt(antiSnipingTriggerSeconds, 10) || 0,
        antiSnipingExtensionSeconds: parseInt(antiSnipingExtensionSeconds, 10) || 0,
        maxBidsPerUserPerLotPerMinute: parseInt(maxBidsPerUserPerLotPerMinute, 10) || 10,
        maxRejectedBidsPerUserPerMinute: parseInt(maxRejectedBidsPerUserPerMinute, 10) || 5,
        noBidsPolicy,
        unpaidWinnerPolicy,
        isPublic,
        showOnHomepage,
        highlightInNav,
        biddingEnabled
      };

      await updateAdminAuction(id, data);
      alert('Сохранено.');
      loadAuctionData();
    } catch (err) {
      setError('Не удалось сохранить.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleStatusAction = async (action: 'publish' | 'pause' | 'resume' | 'cancel' | 'finalize') => {
    if (!id) return;
    if (action === 'cancel' && !window.confirm('Отменить аукцион?')) return;
    if (action === 'finalize' && !window.confirm('Завершить аукцион?\nПосле завершения лоты получат итоговые статусы.')) return;

    try {
      if (action === 'publish') await publishAdminAuction(id);
      if (action === 'pause') await pauseAdminAuction(id);
      if (action === 'resume') await resumeAdminAuction(id);
      if (action === 'cancel') await cancelAdminAuction(id);
      if (action === 'finalize') await finalizeAdminAuction(id);
      alert('Действие выполнено.');
      loadAuctionData();
    } catch (err) {
      alert('Не удалось выполнить действие.');
    }
  };

  const handleDirectSale = async (lotId: string) => {
    if (!window.confirm('Лот будет помечен для прямой продажи. Полная витрина прямой продажи будет добавлена позже.')) return;
    try {
      await moveLotToDirectSale(lotId);
      alert('Лот помечен для прямой продажи.');
      loadLots();
    } catch (err) {
      alert('Не удалось выполнить действие.');
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-gray-500">Загрузка...</p>
      </div>
    );
  }

  if (!auction) {
    return (
      <div className="text-center py-12">
        <h3 className="text-lg font-medium text-gray-900">Аукцион не найден.</h3>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-6xl mx-auto pb-12">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <button onClick={() => navigate('/auctions')} className="text-gray-500 hover:text-gray-900">
            <ArrowLeft className="h-6 w-6" />
          </button>
          <div>
            <h1 className="text-2xl font-bold text-gray-900 flex items-center">
              {auction.title}
              <span className={`ml-3 px-2.5 py-0.5 rounded-full text-xs font-medium 
                ${auction.status === 'live' ? 'bg-green-100 text-green-800' : 
                  auction.status === 'draft' ? 'bg-gray-100 text-gray-800' :
                  auction.status === 'scheduled' ? 'bg-blue-100 text-blue-800' :
                  auction.status === 'paused' ? 'bg-yellow-100 text-yellow-800' :
                  'bg-red-100 text-red-800'}`}>
                {STATUS_LABELS[auction.status] || auction.status}
              </span>
            </h1>
            <p className="text-sm text-gray-500 mt-1">ID: {auction.id}</p>
          </div>
        </div>

        <div className="flex space-x-3">
          {auction.status === 'draft' && hasPermission('auctions.publish') && (
            <button onClick={() => handleStatusAction('publish')} className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700">
              <Play className="h-4 w-4 mr-2" /> Опубликовать
            </button>
          )}
          {(auction.status === 'scheduled' || auction.status === 'live') && hasPermission('auctions.pause') && (
            <button onClick={() => handleStatusAction('pause')} className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-yellow-500 hover:bg-yellow-600">
              <Pause className="h-4 w-4 mr-2" /> Пауза
            </button>
          )}
          {auction.status === 'paused' && hasPermission('auctions.pause') && (
            <button onClick={() => handleStatusAction('resume')} className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-green-600 hover:bg-green-700">
              <Play className="h-4 w-4 mr-2" /> Возобновить
            </button>
          )}
          {(auction.status === 'live' || auction.status === 'paused') && hasPermission('auctions.finalize') && (
            <button onClick={() => handleStatusAction('finalize')} className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-purple-600 hover:bg-purple-700">
              <CheckCircle className="h-4 w-4 mr-2" /> Завершить
            </button>
          )}
          {(auction.status === 'draft' || auction.status === 'scheduled' || auction.status === 'paused') && hasPermission('auctions.cancel') && (
            <button onClick={() => handleStatusAction('cancel')} className="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-red-700 bg-white hover:bg-red-50">
              <XCircle className="h-4 w-4 mr-2 text-red-500" /> Отменить
            </button>
          )}
        </div>
      </div>

      <div className="bg-white shadow rounded-lg">
        <div className="border-b border-gray-200">
          <nav className="-mb-px flex space-x-8 px-6" aria-label="Tabs">
            <button
              onClick={() => setActiveTab('settings')}
              className={`${activeTab === 'settings' ? 'border-indigo-500 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'} whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`}
            >
              Настройки аукциона
            </button>
            <button
              onClick={() => setActiveTab('lots')}
              className={`${activeTab === 'lots' ? 'border-indigo-500 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'} whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`}
            >
              Лоты ({lots.length})
            </button>
          </nav>
        </div>

        <div className="p-6">
          {activeTab === 'settings' && (
            <form onSubmit={handleUpdate} className="space-y-8 max-w-4xl">
              {error && (
                <div className="bg-red-50 text-red-700 p-4 rounded-md">
                  {error}
                </div>
              )}

              {/* Basic Fields */}
              <section>
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                  <div className="sm:col-span-2">
                    <label className="block text-sm font-medium text-gray-700">Название <span className="text-red-500">*</span></label>
                    <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} required className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                  <div className="sm:col-span-2">
                    <label className="block text-sm font-medium text-gray-700">Описание</label>
                    <textarea rows={3} value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Дата начала <span className="text-red-500">*</span></label>
                    <input type="datetime-local" value={startsAt} onChange={(e) => setStartsAt(e.target.value)} required className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Дата окончания <span className="text-red-500">*</span></label>
                    <input type="datetime-local" value={endsAt} onChange={(e) => setEndsAt(e.target.value)} required className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                </div>
              </section>

              {/* Finance */}
              <section className="border-t pt-6">
                <h3 className="text-lg font-medium text-gray-900 mb-4">Финансы и ставки</h3>
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                  <div>
                    <label className="flex items-center text-sm font-medium text-gray-700">
                      Шаг ставки по умолчанию (₽) <HelpTooltip content="На сколько рублей увеличивается текущая цена при новой ставке." />
                    </label>
                    <input type="number" min="1" value={bidStepRubles} onChange={(e) => setBidStepRubles(e.target.value)} required className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Дедлайн оплаты (часов)</label>
                    <input type="number" min="1" value={paymentDeadlineHours} onChange={(e) => setPaymentDeadlineHours(e.target.value)} required className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm" />
                  </div>
                  <div className="sm:col-span-2">
                    <div className="flex items-center mb-4">
                      <input id="antiSnipingEnabledDetail" type="checkbox" checked={antiSnipingEnabled} onChange={(e) => setAntiSnipingEnabled(e.target.checked)} className="h-4 w-4 text-indigo-600 border-gray-300 rounded" />
                      <label htmlFor="antiSnipingEnabledDetail" className="ml-2 block text-sm text-gray-900 font-medium">
                        Антиснайпинг включён
                        <HelpTooltip content="Если ставка сделана в конце аукциона, время автоматически продлевается." />
                      </label>
                    </div>
                    {antiSnipingEnabled && (
                      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 pl-6 border-l-2 border-indigo-100">
                        <div>
                          <label className="block text-sm font-medium text-gray-700">Сработать за N секунд до конца</label>
                          <input type="number" min="1" value={antiSnipingTriggerSeconds} onChange={(e) => setAntiSnipingTriggerSeconds(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm" />
                        </div>
                        <div>
                          <label className="block text-sm font-medium text-gray-700">Продлить на N секунд</label>
                          <input type="number" min="1" value={antiSnipingExtensionSeconds} onChange={(e) => setAntiSnipingExtensionSeconds(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm" />
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </section>

              {/* Policies */}
              <section className="border-t pt-6">
                <h3 className="text-lg font-medium text-gray-900 mb-4">Политики и лимиты</h3>
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                  <div>
                    <label className="flex items-center text-sm font-medium text-gray-700">Лимит ставок в минуту <HelpTooltip content="Ограничивает частые ставки." /></label>
                    <input type="number" min="1" value={maxBidsPerUserPerLotPerMinute} onChange={(e) => setMaxBidsPerUserPerLotPerMinute(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm" />
                  </div>
                  <div>
                    <label className="flex items-center text-sm font-medium text-gray-700">Лимит отклонённых ставок <HelpTooltip content="Помогает отслеживать подозрительные попытки." /></label>
                    <input type="number" min="1" value={maxRejectedBidsPerUserPerMinute} onChange={(e) => setMaxRejectedBidsPerUserPerMinute(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Если нет ставок <HelpTooltip content="Если нет ставок" /></label>
                    <select value={noBidsPolicy} onChange={(e) => setNoBidsPolicy(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm">
                      <option value="manual_review">Ручное решение</option>
                      <option value="direct_sale">Перевести в прямую продажу</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Если победитель не оплатил <HelpTooltip content="Победитель не оплатил" /></label>
                    <select value={unpaidWinnerPolicy} onChange={(e) => setUnpaidWinnerPolicy(e.target.value)} className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 sm:text-sm">
                      <option value="manual_review">Ручное решение</option>
                      <option value="pass_to_second">Передать второму участнику</option>
                    </select>
                  </div>
                </div>
              </section>

              {/* Visibility */}
              <section className="border-t pt-6">
                <h3 className="text-lg font-medium text-gray-900 mb-4">Видимость</h3>
                <div className="space-y-4">
                  <div className="flex items-center">
                    <input id="isPublicDetail" type="checkbox" checked={isPublic} onChange={(e) => setIsPublic(e.target.checked)} className="h-4 w-4 text-indigo-600 rounded" />
                    <label htmlFor="isPublicDetail" className="ml-2 flex text-sm text-gray-900">Публичный аукцион <HelpTooltip content="Доступен покупателям." /></label>
                  </div>
                  <div className="flex items-center">
                    <input id="showOnHomepageDetail" type="checkbox" checked={showOnHomepage} onChange={(e) => setShowOnHomepage(e.target.checked)} className="h-4 w-4 text-indigo-600 rounded" />
                    <label htmlFor="showOnHomepageDetail" className="ml-2 flex text-sm text-gray-900">Показывать на главной <HelpTooltip content="Блок на главной." /></label>
                  </div>
                  <div className="flex items-center">
                    <input id="highlightInNavDetail" type="checkbox" checked={highlightInNav} onChange={(e) => setHighlightInNav(e.target.checked)} className="h-4 w-4 text-indigo-600 rounded" />
                    <label htmlFor="highlightInNavDetail" className="ml-2 flex text-sm text-gray-900">Выделить в меню <HelpTooltip content="Badge в навигации." /></label>
                  </div>
                  <div className="flex items-center">
                    <input id="biddingEnabledDetail" type="checkbox" checked={biddingEnabled} onChange={(e) => setBiddingEnabled(e.target.checked)} className="h-4 w-4 text-indigo-600 rounded" />
                    <label htmlFor="biddingEnabledDetail" className="ml-2 flex text-sm text-gray-900">Ставки включены <HelpTooltip content="Если выключено - режим просмотра." /></label>
                  </div>
                </div>
              </section>

              <div className="pt-5 border-t border-gray-200">
                <div className="flex justify-end">
                  <button type="submit" disabled={isSubmitting || !hasPermission('auctions.update')} className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50">
                    {isSubmitting ? 'Сохраняем…' : 'Сохранить изменения'}
                  </button>
                </div>
              </div>
            </form>
          )}

          {activeTab === 'lots' && (
            <div>
              <div className="flex justify-between mb-4">
                <h3 className="text-lg font-medium text-gray-900">Список лотов</h3>
                {hasPermission('auctions.update') && (
                  <button
                    onClick={() => {
                      setEditingLot(null);
                      setIsLotModalOpen(true);
                    }}
                    className="inline-flex items-center px-3 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700"
                  >
                    <Plus className="h-4 w-4 mr-1" /> Добавить лот
                  </button>
                )}
              </div>

              {lots.length === 0 ? (
                <div className="text-center py-10 bg-gray-50 rounded-lg">
                  <p className="text-sm text-gray-500">Нет лотов в этом аукционе.</p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Название</th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Статус</th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Цена (Старт / Тек)</th>
                        <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Действия</th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {lots.map(lot => (
                        <tr key={lot.id}>
                          <td className="px-4 py-4 text-sm font-medium text-gray-900">{lot.title}</td>
                          <td className="px-4 py-4 text-sm">
                            <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                              ${lot.status === 'active' ? 'bg-green-100 text-green-800' :
                                lot.status === 'sold' ? 'bg-purple-100 text-purple-800' :
                                'bg-gray-100 text-gray-800'}`}>
                              {LOT_STATUS_LABELS[lot.status] || lot.status}
                            </span>
                          </td>
                          <td className="px-4 py-4 text-sm text-gray-500">
                            {(lot.startPriceCents / 100).toLocaleString('ru-RU')} ₽ / 
                            {lot.currentBidCents ? ` ${(lot.currentBidCents / 100).toLocaleString('ru-RU')} ₽` : ' -'}
                          </td>
                          <td className="px-4 py-4 text-right text-sm font-medium">
                            <div className="flex items-center justify-end space-x-3">
                              <button
                                onClick={() => {
                                  setSelectedLotId(lot.id);
                                  setSelectedLotTitle(lot.title);
                                  setIsBidsModalOpen(true);
                                }}
                                className="text-blue-600 hover:text-blue-900"
                                title="Посмотреть ставки"
                              >
                                <Eye className="h-4 w-4 inline" />
                              </button>

                              {hasPermission('auctions.update') && (
                                <button
                                  onClick={() => {
                                    setEditingLot(lot);
                                    setIsLotModalOpen(true);
                                  }}
                                  className="text-indigo-600 hover:text-indigo-900"
                                  title="Редактировать лот"
                                >
                                  <Edit2 className="h-4 w-4 inline" />
                                </button>
                              )}

                              {hasPermission('auctions.move_to_direct_sale') && lot.canMoveToDirectSale && (lot.status === 'ended_no_bids' || lot.status === 'unpaid_manual_review') && (
                                <button
                                  onClick={() => handleDirectSale(lot.id)}
                                  className="text-green-600 hover:text-green-900"
                                  title="В прямую продажу"
                                >
                                  <ShoppingCart className="h-4 w-4 inline" />
                                </button>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <LotModal
        isOpen={isLotModalOpen}
        onClose={() => setIsLotModalOpen(false)}
        onSuccess={() => {
          setIsLotModalOpen(false);
          loadLots();
        }}
        auctionId={id || ''}
        lot={editingLot}
      />

      <BidsModal
        isOpen={isBidsModalOpen}
        onClose={() => setIsBidsModalOpen(false)}
        lotId={selectedLotId}
        lotTitle={selectedLotTitle}
      />
    </div>
  );
}

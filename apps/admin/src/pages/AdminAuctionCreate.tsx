import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { createAdminAuction } from '@zamk/api-client/src/admin';
import { HelpTooltip } from '../components/HelpTooltip';

export function AdminAuctionCreate() {
  const navigate = useNavigate();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form state
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  
  // Initialize with some default future times
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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!title.trim()) {
      setError('Введите название.');
      return;
    }
    if (!startsAt) {
      setError('Введите дату начала.');
      return;
    }
    if (!endsAt) {
      setError('Введите дату окончания.');
      return;
    }
    const start = new Date(startsAt).getTime();
    const end = new Date(endsAt).getTime();
    if (end <= start) {
      setError('Дата окончания должна быть позже даты начала.');
      return;
    }

    const stepCents = Math.round(parseFloat(bidStepRubles) * 100);
    if (isNaN(stepCents) || stepCents <= 0) {
      setError('Шаг ставки должен быть больше нуля.');
      return;
    }
    const deadlineHours = parseInt(paymentDeadlineHours, 10);
    if (isNaN(deadlineHours) || deadlineHours <= 0) {
      setError('Дедлайн оплаты должен быть больше нуля.');
      return;
    }

    try {
      setIsSubmitting(true);
      const data = {
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

      const result = await createAdminAuction(data);
      alert('Сохранено.');
      navigate(`/auctions/${result.id}`);
    } catch (err) {
      setError('Не удалось сохранить.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="space-y-6 max-w-4xl mx-auto pb-12">
      <div className="flex items-center space-x-4">
        <button onClick={() => navigate('/auctions')} className="text-gray-500 hover:text-gray-900">
          <ArrowLeft className="h-6 w-6" />
        </button>
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Новый аукцион</h1>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 text-red-700 p-4 rounded-md">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="bg-white shadow rounded-lg p-6 space-y-8">
        
        {/* Основная информация */}
        <section>
          <h2 className="text-lg font-medium text-gray-900 mb-4 border-b pb-2">Основная информация</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <label className="block text-sm font-medium text-gray-700">
                Название <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                required
              />
            </div>
            
            <div className="sm:col-span-2">
              <label className="block text-sm font-medium text-gray-700">Описание</label>
              <textarea
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700">
                Дата и время начала <span className="text-red-500">*</span>
              </label>
              <input
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700">
                Дата и время окончания <span className="text-red-500">*</span>
              </label>
              <input
                type="datetime-local"
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                required
              />
            </div>
          </div>
        </section>

        {/* Финансы и ставки */}
        <section>
          <h2 className="text-lg font-medium text-gray-900 mb-4 border-b pb-2">Финансы и ставки</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Шаг ставки (₽) <span className="text-red-500 ml-1">*</span>
                <HelpTooltip content="На сколько рублей увеличивается текущая цена при новой ставке." />
              </label>
              <input
                type="number"
                min="1"
                step="1"
                value={bidStepRubles}
                onChange={(e) => setBidStepRubles(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                required
              />
            </div>

            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Дедлайн оплаты (часов) <span className="text-red-500 ml-1">*</span>
              </label>
              <input
                type="number"
                min="1"
                step="1"
                value={paymentDeadlineHours}
                onChange={(e) => setPaymentDeadlineHours(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                required
              />
            </div>
            
            <div className="sm:col-span-2">
              <div className="flex items-center mb-4">
                <input
                  id="antiSnipingEnabled"
                  type="checkbox"
                  checked={antiSnipingEnabled}
                  onChange={(e) => setAntiSnipingEnabled(e.target.checked)}
                  className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
                />
                <label htmlFor="antiSnipingEnabled" className="ml-2 block text-sm text-gray-900 font-medium">
                  Антиснайпинг включён
                  <HelpTooltip content="Если ставка сделана в конце аукциона, время автоматически продлевается, чтобы участники успели ответить." />
                </label>
              </div>
              
              {antiSnipingEnabled && (
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 pl-6 border-l-2 border-indigo-100">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Сработать за N секунд до конца</label>
                    <input
                      type="number"
                      min="1"
                      value={antiSnipingTriggerSeconds}
                      onChange={(e) => setAntiSnipingTriggerSeconds(e.target.value)}
                      className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Продлить на N секунд</label>
                    <input
                      type="number"
                      min="1"
                      value={antiSnipingExtensionSeconds}
                      onChange={(e) => setAntiSnipingExtensionSeconds(e.target.value)}
                      className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        </section>

        {/* Безопасность и политики */}
        <section>
          <h2 className="text-lg font-medium text-gray-900 mb-4 border-b pb-2">Безопасность и политики</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Лимит ставок в минуту
                <HelpTooltip content="Ограничивает слишком частые ставки от одного пользователя." />
              </label>
              <input
                type="number"
                min="1"
                value={maxBidsPerUserPerLotPerMinute}
                onChange={(e) => setMaxBidsPerUserPerLotPerMinute(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              />
            </div>
            
            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Лимит отклонённых ставок
                <HelpTooltip content="Помогает отслеживать подозрительные попытки сделать неправильные ставки." />
              </label>
              <input
                type="number"
                min="1"
                value={maxRejectedBidsPerUserPerMinute}
                onChange={(e) => setMaxRejectedBidsPerUserPerMinute(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              />
            </div>

            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Если нет ставок
                <HelpTooltip content="Что делать с лотами, если аукцион закончился без ставок." />
              </label>
              <select
                value={noBidsPolicy}
                onChange={(e) => setNoBidsPolicy(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              >
                <option value="manual_review">Ручное решение</option>
                <option value="direct_sale">Авто-перевод в прямую продажу (если разрешено)</option>
              </select>
            </div>

            <div>
              <label className="flex items-center text-sm font-medium text-gray-700">
                Если победитель не оплатил
                <HelpTooltip content="Что делать, если победитель не оплатил лот вовремя." />
              </label>
              <select
                value={unpaidWinnerPolicy}
                onChange={(e) => setUnpaidWinnerPolicy(e.target.value)}
                className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              >
                <option value="manual_review">Ручное решение</option>
                <option value="pass_to_second">Передать второму участнику (будущее)</option>
              </select>
            </div>
          </div>
        </section>

        {/* Настройки видимости */}
        <section>
          <h2 className="text-lg font-medium text-gray-900 mb-4 border-b pb-2">Видимость</h2>
          <div className="space-y-4">
            <div className="flex items-center">
              <input
                id="isPublic"
                type="checkbox"
                checked={isPublic}
                onChange={(e) => setIsPublic(e.target.checked)}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <label htmlFor="isPublic" className="ml-2 flex items-center text-sm text-gray-900 font-medium">
                Публичный аукцион
                <HelpTooltip content="Если включено, аукцион может отображаться на сайте покупателям." />
              </label>
            </div>
            
            <div className="flex items-center">
              <input
                id="showOnHomepage"
                type="checkbox"
                checked={showOnHomepage}
                onChange={(e) => setShowOnHomepage(e.target.checked)}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <label htmlFor="showOnHomepage" className="ml-2 flex items-center text-sm text-gray-900 font-medium">
                Показывать на главной
                <HelpTooltip content="Аукцион будет показан в отдельном блоке на главной странице." />
              </label>
            </div>
            
            <div className="flex items-center">
              <input
                id="highlightInNav"
                type="checkbox"
                checked={highlightInNav}
                onChange={(e) => setHighlightInNav(e.target.checked)}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <label htmlFor="highlightInNav" className="ml-2 flex items-center text-sm text-gray-900 font-medium">
                Выделить в меню
                <HelpTooltip content="В будущем рядом с пунктом «Аукцион» можно будет показать активный бейдж." />
              </label>
            </div>
            
            <div className="flex items-center">
              <input
                id="biddingEnabled"
                type="checkbox"
                checked={biddingEnabled}
                onChange={(e) => setBiddingEnabled(e.target.checked)}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <label htmlFor="biddingEnabled" className="ml-2 flex items-center text-sm text-gray-900 font-medium">
                Ставки включены
                <HelpTooltip content="Если выключено, покупатели видят аукцион, но не могут делать ставки." />
              </label>
            </div>
          </div>
        </section>

        <div className="pt-5 border-t border-gray-200">
          <div className="flex justify-end">
            <button
              type="button"
              onClick={() => navigate('/auctions')}
              className="bg-white py-2 px-4 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="ml-3 inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none disabled:bg-indigo-400 disabled:cursor-not-allowed"
            >
              {isSubmitting ? 'Сохраняем…' : 'Сохранить аукцион'}
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}

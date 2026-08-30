import { useState, useEffect } from 'react';
import { Truck, MapPin, ArrowLeft, RefreshCw, CheckCircle2 } from 'lucide-react';
import { getCustomerReturnShipment, createCustomerReturnShipment, getCDEKOffices } from '@zamk/api-client/src/customer';
import {
  type ReturnShipment,
  type CDEKOffice,
  formatReturnShipmentStatus,
  formatReturnShipmentMethod,
} from '@zamk/api-client/src/types';

interface ReturnLogisticsProps {
  returnId: string;
  initialShipment?: ReturnShipment | null;
  onShipmentUpdated?: (shipment: ReturnShipment) => void;
}

export function ReturnLogistics({ returnId, initialShipment, onShipmentUpdated }: ReturnLogisticsProps) {
  const [shipment, setShipment] = useState<ReturnShipment | null>(initialShipment || null);
  const [offices, setOffices] = useState<CDEKOffice[]>([]);
  const [loading, setLoading] = useState(!initialShipment);
  const [loadingOffices, setLoadingOffices] = useState(false);
  const [officeError, setOfficeError] = useState('');
  const [courierError, setCourierError] = useState('');
  const [mode, setMode] = useState<'select' | 'cdek_office' | 'cdek_courier'>('select');
  const [selectedOffice, setSelectedOffice] = useState('');

  // Courier form
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [city, setCity] = useState('');
  const [street, setStreet] = useState('');
  const [house, setHouse] = useState('');
  const [flat, setFlat] = useState('');

  useEffect(() => {
    if (initialShipment) {
      setShipment(initialShipment);
      setLoading(false);
    } else {
      loadShipment();
    }
  }, [returnId, initialShipment]);

  const loadShipment = async () => {
    try {
      setLoading(true);
      const res = await getCustomerReturnShipment(returnId);
      if (res && res.shipment) {
        setShipment(res.shipment);
        onShipmentUpdated?.(res.shipment);
      }
    } catch (err: unknown) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const loadOffices = async () => {
    try {
      setLoadingOffices(true);
      setOfficeError('');
      const res = await getCDEKOffices(returnId);
      if (res && res.offices) {
        setOffices(res.offices);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('not_configured') || msg.includes('not configured') || msg.includes('503')) {
        setOfficeError('Логистика СДЭК временно недоступна.');
      } else {
        setOfficeError('Логистика СДЭК временно недоступна.');
      }
    } finally {
      setLoadingOffices(false);
    }
  };

  const handleSelectOffice = async () => {
    setOfficeError('');
    setCourierError('');
    setMode('cdek_office');
    await loadOffices();
  };

  const handleSelectCourier = () => {
    setOfficeError('');
    setCourierError('');
    setMode('cdek_courier');
  };

  const handleBackToSelect = () => {
    setOfficeError('');
    setCourierError('');
    setMode('select');
  };

  const handleSubmitOffice = async () => {
    if (!selectedOffice) {
      setOfficeError('Выберите отделение СДЭК');
      return;
    }
    try {
      setOfficeError('');
      const res = await createCustomerReturnShipment(returnId, {
        method: 'cdek_office',
        cdekOfficeCode: selectedOffice,
      });
      if (res?.shipment) {
        setShipment(res.shipment);
        onShipmentUpdated?.(res.shipment);
      } else {
        await loadShipment();
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('not_configured') || msg.includes('not configured') || msg.includes('503')) {
        setOfficeError('Логистика СДЭК временно недоступна.');
      } else {
        setOfficeError(msg || 'Ошибка при выборе отделения');
      }
    }
  };

  const handleSubmitCourier = async () => {
    if (!name.trim() || !phone.trim() || !city.trim() || !street.trim() || !house.trim()) {
      setCourierError('Заполните все обязательные поля');
      return;
    }
    try {
      setCourierError('');
      const res = await createCustomerReturnShipment(returnId, {
        method: 'cdek_courier',
        customerName: name.trim(),
        customerPhone: phone.trim(),
        pickupAddress: {
          city: city.trim(),
          street: street.trim(),
          house: house.trim(),
          flat: flat.trim() || undefined,
        },
      });
      if (res?.shipment) {
        setShipment(res.shipment);
        onShipmentUpdated?.(res.shipment);
      } else {
        await loadShipment();
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes('not_configured') || msg.includes('not configured') || msg.includes('503')) {
        setCourierError('Логистика СДЭК временно недоступна.');
      } else {
        setCourierError(msg || 'Ошибка при вызове курьера');
      }
    }
  };

  if (loading) {
    return (
      <div className="py-6 flex justify-center items-center gap-2 text-sm text-ash dark:text-white/60">
        <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
        <span>Загрузка логистики...</span>
      </div>
    );
  }

  if (shipment) {
    const pickup = shipment.pickupAddress;
    const addressStr = pickup
      ? `${pickup.city}, ${pickup.street}, д. ${pickup.house}${pickup.flat ? ', кв. ' + pickup.flat : ''}`
      : undefined;

    return (
      <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm font-sans">
        <div className="flex items-center gap-3 mb-5">
          <div className="w-8 h-8 rounded-full bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
            <CheckCircle2 className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-base font-semibold text-graphite dark:text-white">
              Логистика возврата
            </h3>
            <p className="text-xs text-graphite/70 dark:text-white/70">
              Способ отправки выбран и зарегистрирован
            </p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
          <div className="p-3.5 rounded-xl bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10">
            <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">Способ доставки</span>
            <span className="font-medium text-graphite dark:text-white">
              {formatReturnShipmentMethod(shipment.method)}
            </span>
          </div>

          <div className="p-3.5 rounded-xl bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10">
            <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">Статус отправления</span>
            <span className="font-medium text-graphite dark:text-white">
              {formatReturnShipmentStatus(shipment.status)}
            </span>
          </div>

          {shipment.trackingNumber && (
            <div className="p-3.5 rounded-xl bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10 sm:col-span-2">
              <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">Трек-номер</span>
              <span className="font-mono font-medium text-graphite dark:text-white">
                {shipment.trackingNumber}
              </span>
            </div>
          )}

          {shipment.method === 'cdek_office' && (shipment.selectedCdekOfficeCode || shipment.cdekOfficeAddress) && (
            <div className="p-3.5 rounded-xl bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10 sm:col-span-2">
              <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">Отделение СДЭК</span>
              <span className="font-medium text-graphite dark:text-white">
                {shipment.cdekOfficeAddress || `Код отделения: ${shipment.selectedCdekOfficeCode}`}
              </span>
            </div>
          )}

          {shipment.method === 'cdek_courier' && addressStr && (
            <div className="p-3.5 rounded-xl bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10 sm:col-span-2">
              <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">Адрес забора курьером</span>
              <span className="font-medium text-graphite dark:text-white">
                {addressStr}
                {shipment.customerName && ` (${shipment.customerName}, ${shipment.customerPhone || ''})`}
              </span>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm font-sans">
      <div className="mb-5">
        <h3 className="text-base font-semibold text-graphite dark:text-white">
          Как отправить товар обратно?
        </h3>
        <p className="text-xs md:text-sm text-graphite/70 dark:text-white/70 mt-1">
          Возврат одобрен. Выберите удобный способ отправки.
        </p>
      </div>

      {mode === 'select' && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <button
            type="button"
            onClick={handleSelectCourier}
            className="flex items-start gap-3.5 p-4 text-left rounded-xl bg-graphite/5 dark:bg-white/5 hover:bg-black/5 dark:hover:bg-white/10 border border-graphite/10 dark:border-white/10 hover:border-black/30 dark:hover:border-white/30 transition-all group cursor-pointer"
          >
            <div className="w-9 h-9 rounded-lg bg-white dark:bg-gray-800 flex items-center justify-center flex-shrink-0 shadow-sm group-hover:scale-105 transition-transform border border-graphite/10 dark:border-white/10">
              <Truck className="w-4 h-4 text-graphite dark:text-white" />
            </div>
            <div>
              <h4 className="font-medium text-sm text-graphite dark:text-white">
                Заберёт курьер СДЭК
              </h4>
              <p className="text-xs text-graphite/70 dark:text-white/70 mt-0.5 leading-relaxed">
                Курьер заберёт посылку по указанному адресу.
              </p>
            </div>
          </button>

          <button
            type="button"
            onClick={handleSelectOffice}
            className="flex items-start gap-3.5 p-4 text-left rounded-xl bg-graphite/5 dark:bg-white/5 hover:bg-black/5 dark:hover:bg-white/10 border border-graphite/10 dark:border-white/10 hover:border-black/30 dark:hover:border-white/30 transition-all group cursor-pointer"
          >
            <div className="w-9 h-9 rounded-lg bg-white dark:bg-gray-800 flex items-center justify-center flex-shrink-0 shadow-sm group-hover:scale-105 transition-transform border border-graphite/10 dark:border-white/10">
              <MapPin className="w-4 h-4 text-graphite dark:text-white" />
            </div>
            <div>
              <h4 className="font-medium text-sm text-graphite dark:text-white">
                Отнести в отделение СДЭК
              </h4>
              <p className="text-xs text-graphite/70 dark:text-white/70 mt-0.5 leading-relaxed">
                Отнесите посылку в удобное отделение СДЭК.
              </p>
            </div>
          </button>
        </div>
      )}

      {mode === 'cdek_office' && (
        <div className="space-y-4 max-w-lg">
          <div className="flex items-center gap-2 pb-1">
            <button
              type="button"
              onClick={handleBackToSelect}
              className="inline-flex items-center gap-1.5 text-xs font-medium text-graphite/70 dark:text-white/70 hover:text-graphite dark:hover:text-white transition-colors"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              Назад к выбору способа
            </button>
          </div>

          <h4 className="text-sm font-semibold text-graphite dark:text-white">
            Выбор отделения СДЭК
          </h4>

          {loadingOffices && (
            <div className="flex items-center gap-2 py-4 text-sm text-graphite/70 dark:text-white/70">
              <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              <span>Загрузка отделений...</span>
            </div>
          )}

          {!loadingOffices && officeError && (
            <div className="space-y-3">
              <div className="p-3.5 rounded-xl bg-red-50/90 dark:bg-red-950/40 border border-red-200/80 dark:border-red-900/50 text-red-700 dark:text-red-300 text-xs md:text-sm font-medium">
                {officeError}
              </div>
              <div className="flex gap-2.5">
                <button
                  type="button"
                  onClick={handleBackToSelect}
                  className="px-4 py-2 rounded-full border border-graphite/20 dark:border-white/20 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
                >
                  Назад
                </button>
                <button
                  type="button"
                  onClick={loadOffices}
                  className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-medium text-graphite dark:text-white hover:bg-graphite/5 dark:hover:bg-white/5 transition-colors"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  Повторить попытку
                </button>
              </div>
            </div>
          )}

          {!loadingOffices && !officeError && (
            <>
              <div>
                <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1.5">
                  Пункт выдачи / приёма
                </label>
                <select
                  className="w-full px-3 py-2.5 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white"
                  value={selectedOffice}
                  onChange={(e) => setSelectedOffice(e.target.value)}
                >
                  <option value="">Выберите отделение СДЭК</option>
                  {offices.map((o) => (
                    <option key={o.code} value={o.code}>
                      {o.name} — {o.address} {o.workingHours ? `(${o.workingHours})` : ''}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex gap-2.5 pt-1">
                <button
                  type="button"
                  onClick={handleSubmitOffice}
                  className="px-5 py-2.5 rounded-full bg-black text-white dark:bg-white dark:text-black text-xs font-medium hover:opacity-90 transition-opacity cursor-pointer"
                >
                  Подтвердить
                </button>
                <button
                  type="button"
                  onClick={handleBackToSelect}
                  className="px-4 py-2.5 rounded-full border border-graphite/20 dark:border-white/20 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer"
                >
                  Назад
                </button>
              </div>
            </>
          )}
        </div>
      )}

      {mode === 'cdek_courier' && (
        <div className="space-y-3.5 max-w-lg">
          <div className="flex items-center gap-2 pb-1">
            <button
              type="button"
              onClick={handleBackToSelect}
              className="inline-flex items-center gap-1.5 text-xs font-medium text-graphite/70 dark:text-white/70 hover:text-graphite dark:hover:text-white transition-colors"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              Назад к выбору способа
            </button>
          </div>

          <h4 className="text-sm font-semibold text-graphite dark:text-white">
            Адрес для забора курьером
          </h4>

          {courierError && (
            <div className="p-3.5 rounded-xl bg-red-50/90 dark:bg-red-950/40 border border-red-200/80 dark:border-red-900/50 text-red-700 dark:text-red-300 text-xs md:text-sm font-medium">
              {courierError}
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Имя
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="Иван Иванов"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Телефон
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="+7 (999) 000-00-00"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Город
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="Москва"
                value={city}
                onChange={(e) => setCity(e.target.value)}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Улица
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="Тверская"
                value={street}
                onChange={(e) => setStreet(e.target.value)}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Дом
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="10"
                value={house}
                onChange={(e) => setHouse(e.target.value)}
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-graphite/75 dark:text-white/75 mb-1">
                Квартира (необязательно)
              </label>
              <input
                className="w-full px-3 py-2 rounded-xl border border-graphite/20 dark:border-white/20 bg-white dark:bg-gray-900 text-graphite dark:text-white text-sm"
                placeholder="42"
                value={flat}
                onChange={(e) => setFlat(e.target.value)}
              />
            </div>
          </div>

          <div className="flex gap-2.5 pt-2">
            <button
              type="button"
              onClick={handleSubmitCourier}
              className="px-5 py-2.5 rounded-full bg-black text-white dark:bg-white dark:text-black text-xs font-medium hover:opacity-90 transition-opacity cursor-pointer"
            >
              Подтвердить вызов курьера
            </button>
            <button
              type="button"
              onClick={handleBackToSelect}
              className="px-4 py-2.5 rounded-full border border-graphite/20 dark:border-white/20 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer"
            >
              Назад
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

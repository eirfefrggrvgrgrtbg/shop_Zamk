import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Moon, Sun, Monitor, MapPin, ChevronRight, Plus } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { getAddresses, createAddress, updateAddress, deleteAddress, setDefaultAddress } from '@zamk/api-client/src/customer';
import { useToast } from '../contexts/ToastContext';
import { AccountLayout } from '../components/account/AccountLayout';
import { ADDRESS_MAX, toAddressPayload, validateAddress } from '../lib/addressValidation';

// --- Компоненты UI для настроек ---

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-10 last:mb-0">
      <h3 className="text-[11px] font-semibold tracking-[0.15em] uppercase text-graphite/40 dark:text-white/40 mb-6 pl-2">
        {title}
      </h3>
      <div className="bg-white/40 dark:bg-white/5 backdrop-blur-xl border border-white/50 dark:border-white/10 rounded-[2rem] overflow-hidden shadow-[0_8px_32px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_32px_rgba(0,0,0,0.3)]">
        {children}
      </div>
    </section>
  );
}

// --- Главная страница ---

export function Settings() {
  const { theme, setTheme } = useTheme();
  const { showToast } = useToast();
  
  interface Address {
    id: string;
    label?: string;
    recipientName: string;
    phone: string;
    city: string;
    street: string;
    house: string;
    apartment?: string;
    postalCode?: string;
    comment?: string;
    isDefault: boolean;
  }
  
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [isLoadingAddresses, setIsLoadingAddresses] = useState(true);

  const fetchAddresses = async () => {
    try {
      setIsLoadingAddresses(true);
      const data = await getAddresses();
      setAddresses(data || []);
    } catch (err: any) {
      showToast('Не удалось загрузить адреса');
    } finally {
      setIsLoadingAddresses(false);
    }
  };

  useEffect(() => {
    fetchAddresses();
  }, []);

  const [isAddressModalOpen, setAddressModalOpen] = useState(false);
  const [editingAddress, setEditingAddress] = useState<Partial<Address> | null>(null);
  const [addressErrors, setAddressErrors] = useState<Record<string, string>>({});
  const [isSavingAddress, setIsSavingAddress] = useState(false);
  const setField = (name: string, value: string) => {
    setEditingAddress(prev => (prev ? { ...prev, [name]: value } : {}));
    setAddressErrors(prev => {
      if (!prev[name]) return prev;
      const { [name]: _removed, ...rest } = prev;
      return rest;
    });
  };

  const handleOpenAddressModal = (address?: Address) => {
    setAddressErrors({});
    if (address) {
      setEditingAddress(address);
    } else {
      setEditingAddress({
        label: '',
        recipientName: '',
        phone: '',
        city: '',
        street: '',
        house: '',
        apartment: '',
        postalCode: '',
        comment: '',
      });
    }
    setAddressModalOpen(true);
  };

  const handleSaveAddress = async () => {
    if (!editingAddress || isSavingAddress) return;
    const errors = validateAddress(editingAddress);
    if (Object.keys(errors).length > 0) {
      setAddressErrors(errors);
      return;
    }
    const payload = toAddressPayload(editingAddress);
    try {
      setIsSavingAddress(true);
      if (editingAddress.id) {
        await updateAddress(editingAddress.id, payload);
        showToast('Адрес обновлен');
      } else {
        await createAddress(payload);
        showToast('Адрес добавлен');
      }
      setAddressModalOpen(false);
      fetchAddresses();
    } catch (err: any) {
      const field = err?.data?.error?.field;
      const message = err?.rawMessage || err?.message || 'Ошибка при сохранении адреса';
      if (field && ADDRESS_MAX[field]) setAddressErrors({ [field]: message });
      else showToast(message);
    } finally {
      setIsSavingAddress(false);
    }
  };

  const handleDeleteAddress = async (id: string) => {
    if (!window.confirm('Удалить этот адрес?')) return;
    try {
      await deleteAddress(id);
      showToast('Адрес удален');
      fetchAddresses();
    } catch (err: any) {
      showToast('Ошибка при удалении адреса');
    }
  };

  const handleSetDefault = async (id: string) => {
    try {
      await setDefaultAddress(id);
      fetchAddresses();
    } catch (err: any) {
      showToast('Ошибка при установке адреса по умолчанию');
    }
  };

  return (
    <AccountLayout title="Настройки">
          <div className="mt-8">
            {/* 2. Внешний вид */}
        <Section title="Внешний вид">
          <div className="p-5 md:p-6 flex flex-col gap-4">
            <p className="text-[15px] font-medium text-graphite dark:text-white mb-2">Тема интерфейса</p>
            <div className="bg-graphite/5 dark:bg-white/10 p-1.5 rounded-2xl flex relative w-full overflow-hidden">
              <motion.div
                layoutId="theme-highlighter"
                initial={false}
                className="absolute top-1.5 bottom-1.5 w-[calc(33.333%-4px)] bg-white dark:bg-black rounded-xl shadow-[0_2px_8px_rgba(0,0,0,0.04)]"
                animate={{
                  x: theme === 'light' ? 0 : theme === 'dark' ? '100%' : '200%',
                }}
                transition={{ type: "spring", stiffness: 400, damping: 30 }}
              />
              {(['light', 'dark', 'system'] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => setTheme(t)}
                  className={`relative z-10 flex-1 flex items-center justify-center gap-2 py-2.5 text-[13px] font-medium transition-colors ${theme === t ? 'text-graphite dark:text-white' : 'text-graphite/40 hover:text-graphite/70 dark:text-white/40 dark:hover:text-white/70'}`}
                >
                  {t === 'light' && <Sun className="w-4 h-4" />}
                  {t === 'dark' && <Moon className="w-4 h-4" />}
                  {t === 'system' && <Monitor className="w-4 h-4" />}
                  <span className="capitalize">{t === 'system' ? 'Авто' : t === 'light' ? 'Светлая' : 'Тёмная'}</span>
                </button>
              ))}
            </div>
          </div>
        </Section>

        {/* 2.5 Способы оплаты */}
        <Section title="Способы оплаты">
          <div className="p-5 md:p-6">
            <p className="text-[14px] text-graphite/50 dark:text-white/50 text-center py-4">
              Сохранённые способы оплаты пока не подключены.
            </p>
          </div>
        </Section>

        {/* 4. Доставка */}
        <Section title="Доставка (Адреса)">
          <div className="p-5 md:p-6 grid gap-4">
            {isLoadingAddresses ? (
              <p className="text-[14px] text-graphite/50 dark:text-white/50 text-center py-4">Загрузка адресов...</p>
            ) : addresses.length === 0 ? (
              <p className="text-[14px] text-graphite/50 dark:text-white/50 text-center py-4">У вас пока нет сохранённых адресов.</p>
            ) : (
              addresses.map((item) => (
                <div key={item.id} data-testid="address-card" data-default={item.isDefault ? 'true' : 'false'} className={`p-4 rounded-2xl border transition-all ${item.isDefault ? 'border-graphite/30 dark:border-white/20 bg-white/60 dark:bg-white/5' : 'border-white/50 dark:border-white/10 bg-white/20 dark:bg-white/5'}`}>
                  <div className="flex justify-between items-start mb-2">
                    <div className="flex items-center gap-2">
                      <MapPin className="w-4 h-4 text-graphite/50 dark:text-white/50" />
                      <h5 className="font-medium text-graphite dark:text-white text-[14px]">{item.label || 'Без названия'}</h5>
                      {item.isDefault && (
                        <span className="bg-graphite/10 dark:bg-white/10 text-graphite dark:text-white px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wider font-semibold">Основной</span>
                      )}
                    </div>
                    <div className="flex gap-3 text-[12px] font-medium">
                      <button onClick={() => handleOpenAddressModal(item)} className="text-graphite/40 dark:text-white/40 hover:text-graphite dark:hover:text-white transition-colors">Изм.</button>
                      <button data-testid="address-delete" onClick={() => handleDeleteAddress(item.id)} className="text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 transition-colors">Удал.</button>
                    </div>
                  </div>
                  <p className="text-[13px] text-graphite/60 dark:text-white/60 pl-6 leading-relaxed">
                    {item.recipientName}, {item.phone}<br/>
                    г. {item.city}, ул. {item.street}, д. {item.house}
                    {item.apartment && `, кв. ${item.apartment}`}
                    {item.postalCode && <><br/>Индекс {item.postalCode}</>}
                    {item.comment && <><br/>{item.comment}</>}
                  </p>
                  {!item.isDefault && (
                     <div className="pl-6 mt-3">
                       <button onClick={() => handleSetDefault(item.id)} className="text-[12px] text-graphite dark:text-white hover:underline underline-offset-4">
                         Сделать основным
                       </button>
                     </div>
                  )}
                </div>
              ))
            )}
            
            <button onClick={() => handleOpenAddressModal()} className="flex items-center justify-center gap-2 w-full py-4 mt-2 border border-dashed border-graphite/20 dark:border-white/20 rounded-2xl text-graphite/50 dark:text-white/50 hover:text-graphite dark:hover:text-white hover:border-graphite/40 dark:hover:border-white/40 hover:bg-graphite/5 dark:hover:bg-white/5 transition-all text-[14px] font-medium">
              <Plus className="w-4 h-4" />
              Добавить адрес
            </button>
          </div>
        </Section>
      </div>

      {/* Адрес Модалка */}
      <Modal isOpen={isAddressModalOpen} onClose={() => setAddressModalOpen(false)} title={editingAddress?.id ? "Редактировать адрес" : "Новый адрес"}>
        <div className="pt-2 flex flex-col gap-4 max-h-[70vh] overflow-y-auto px-1">
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Название (например, Дом или Офис)</label>
            <Input 
              value={editingAddress?.label || ''} 
              onChange={(e) => setField('label', e.target.value)}
              maxLength={ADDRESS_MAX.label}
              aria-invalid={Boolean(addressErrors.label)}
              data-testid="addr-label"
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Дом"
            />
            {addressErrors.label && <p role="alert" data-testid="addr-error-label" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.label}</p>}
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">ФИО получателя *</label>
            <Input 
              value={editingAddress?.recipientName || ''} 
              onChange={(e) => setField('recipientName', e.target.value)}
              maxLength={ADDRESS_MAX.recipientName}
              aria-invalid={Boolean(addressErrors.recipientName)}
              data-testid="addr-recipientName"
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Иванов Иван"
            />
            {addressErrors.recipientName && <p role="alert" data-testid="addr-error-recipientName" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.recipientName}</p>}
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Телефон *</label>
            <Input 
              value={editingAddress?.phone || ''} 
              onChange={(e) => setField('phone', e.target.value)}
              maxLength={ADDRESS_MAX.phone}
              aria-invalid={Boolean(addressErrors.phone)}
              data-testid="addr-phone"
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="+7 999 000-00-00"
            />
            {addressErrors.phone && <p role="alert" data-testid="addr-error-phone" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.phone}</p>}
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Город *</label>
            <Input 
              value={editingAddress?.city || ''} 
              onChange={(e) => setField('city', e.target.value)}
              maxLength={ADDRESS_MAX.city}
              aria-invalid={Boolean(addressErrors.city)}
              data-testid="addr-city"
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Москва"
            />
            {addressErrors.city && <p role="alert" data-testid="addr-error-city" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.city}</p>}
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2">
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Улица *</label>
              <Input 
                value={editingAddress?.street || ''} 
                onChange={(e) => setField('street', e.target.value)}
              maxLength={ADDRESS_MAX.street}
              aria-invalid={Boolean(addressErrors.street)}
              data-testid="addr-street"
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="Пушкина"
              />
            {addressErrors.street && <p role="alert" data-testid="addr-error-street" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.street}</p>}
            </div>
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Дом *</label>
              <Input 
                value={editingAddress?.house || ''} 
                onChange={(e) => setField('house', e.target.value)}
              maxLength={ADDRESS_MAX.house}
              aria-invalid={Boolean(addressErrors.house)}
              data-testid="addr-house"
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="10"
              />
            {addressErrors.house && <p role="alert" data-testid="addr-error-house" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.house}</p>}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Квартира</label>
              <Input 
                value={editingAddress?.apartment || ''} 
                onChange={(e) => setField('apartment', e.target.value)}
              maxLength={ADDRESS_MAX.apartment}
              aria-invalid={Boolean(addressErrors.apartment)}
              data-testid="addr-apartment"
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="123"
              />
            {addressErrors.apartment && <p role="alert" data-testid="addr-error-apartment" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.apartment}</p>}
            </div>
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Индекс</label>
              <Input 
                value={editingAddress?.postalCode || ''} 
                onChange={(e) => setField('postalCode', e.target.value)}
              maxLength={ADDRESS_MAX.postalCode}
              aria-invalid={Boolean(addressErrors.postalCode)}
              data-testid="addr-postalCode"
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="101000"
              />
            {addressErrors.postalCode && <p role="alert" data-testid="addr-error-postalCode" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.postalCode}</p>}
            </div>
          </div>
          
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Комментарий для курьера</label>
            <Input
              value={editingAddress?.comment || ''}
              onChange={(e) => setField('comment', e.target.value)}
              maxLength={ADDRESS_MAX.comment}
              aria-invalid={Boolean(addressErrors.comment)}
              data-testid="addr-comment"
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Домофон, этаж, удобное время"
            />
            {addressErrors.comment && <p role="alert" data-testid="addr-error-comment" className="mt-1.5 text-[12.5px] text-red-600 dark:text-red-400">{addressErrors.comment}</p>}
          </div>

          <Button onClick={handleSaveAddress} disabled={isSavingAddress} data-testid="addr-save" className="w-full mt-4">
            {isSavingAddress ? 'Сохранение…' : 'Сохранить адрес'}
          </Button>
        </div>
      </Modal>
    </AccountLayout>
  );
}
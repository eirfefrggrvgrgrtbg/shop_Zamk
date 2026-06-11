import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Moon, Sun, Monitor, MapPin, ChevronRight, Plus } from 'lucide-react';
import { useTheme } from '../contexts/ThemeContext';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { getAddresses, createAddress, updateAddress, deleteAddress, setDefaultAddress } from '@zamk/api-client/src/customer';
import { useToast } from '../contexts/ToastContext';

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

  const handleOpenAddressModal = (address?: Address) => {
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
    if (!editingAddress || !editingAddress.recipientName || !editingAddress.phone || !editingAddress.city || !editingAddress.street || !editingAddress.house) {
      showToast('Заполните обязательные поля: ФИО, Телефон, Город, Улица, Дом');
      return;
    }

    try {
      if (editingAddress.id) {
        await updateAddress(editingAddress.id, editingAddress);
        showToast('Адрес обновлен');
      } else {
        await createAddress(editingAddress);
        showToast('Адрес добавлен');
      }
      setAddressModalOpen(false);
      fetchAddresses();
    } catch (err: any) {
      showToast(err.message || 'Ошибка при сохранении адреса');
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
    <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[800px]'>
        
        {/* Заголовок */}
        <div className="mb-12 pl-2">
          <h1 className="font-serif text-4xl md:text-5xl text-graphite dark:text-white mb-2">Настройки</h1>
          <p className="text-graphite/50 dark:text-white/50 text-[15px]">Управление аккаунтом и предпочтениями</p>
        </div>

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
                <div key={item.id} className={`p-4 rounded-2xl border transition-all ${item.isDefault ? 'border-graphite/30 dark:border-white/20 bg-white/60 dark:bg-white/5' : 'border-white/50 dark:border-white/10 bg-white/20 dark:bg-white/5'}`}>
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
                      {!item.isDefault && <button onClick={() => handleDeleteAddress(item.id)} className="text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 transition-colors">Удал.</button>}
                    </div>
                  </div>
                  <p className="text-[13px] text-graphite/60 dark:text-white/60 pl-6 leading-relaxed">
                    {item.recipientName}, {item.phone}<br/>
                    г. {item.city}, ул. {item.street}, д. {item.house}
                    {item.apartment && `, кв. ${item.apartment}`}
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
              onChange={(e) => setEditingAddress(prev => prev ? { ...prev, label: e.target.value } : {})}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Дом"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">ФИО получателя *</label>
            <Input 
              value={editingAddress?.recipientName || ''} 
              onChange={(e) => setEditingAddress(prev => prev ? { ...prev, recipientName: e.target.value } : {})}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Иванов Иван"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Телефон *</label>
            <Input 
              value={editingAddress?.phone || ''} 
              onChange={(e) => setEditingAddress(prev => prev ? { ...prev, phone: e.target.value } : {})}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="+7 999 000-00-00"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Город *</label>
            <Input 
              value={editingAddress?.city || ''} 
              onChange={(e) => setEditingAddress(prev => prev ? { ...prev, city: e.target.value } : {})}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
              placeholder="Москва"
            />
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2">
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Улица *</label>
              <Input 
                value={editingAddress?.street || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, street: e.target.value } : {})}
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="Пушкина"
              />
            </div>
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Дом *</label>
              <Input 
                value={editingAddress?.house || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, house: e.target.value } : {})}
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="10"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Квартира</label>
              <Input 
                value={editingAddress?.apartment || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, apartment: e.target.value } : {})}
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="123"
              />
            </div>
            <div>
              <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Индекс</label>
              <Input 
                value={editingAddress?.postalCode || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, postalCode: e.target.value } : {})}
                className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20"
                placeholder="101000"
              />
            </div>
          </div>
          
          <Button onClick={handleSaveAddress} className="w-full mt-4">
            Сохранить адрес
          </Button>
        </div>
      </Modal>

    </div>
  );
}
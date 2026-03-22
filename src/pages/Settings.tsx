import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Lock, LogOut, Trash2, Moon, Sun, Monitor, MapPin, ChevronRight, X, Plus } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

// --- Компоненты UI для настроек ---

function ToggleSwitch({ isOn, onToggle }: { isOn: boolean; onToggle: () => void }) {
  return (
    <div 
      onClick={onToggle} 
      className={`w-11 h-6 flex items-center rounded-full p-1 cursor-pointer transition-colors duration-300 ${isOn ? 'bg-graphite' : 'bg-graphite/20'}`}
    >
      <motion.div 
        layout 
        className="w-4 h-4 bg-white rounded-full shadow-sm"
        transition={{ type: "spring", stiffness: 500, damping: 30 }}
        style={{ marginLeft: isOn ? 'auto' : '0' }}
      />
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-10 last:mb-0">
      <h3 className="text-[11px] font-semibold tracking-[0.15em] uppercase text-graphite/40 mb-6 pl-2">
        {title}
      </h3>
      <div className="bg-white/40 backdrop-blur-xl border border-white/50 rounded-[2rem] overflow-hidden shadow-[0_8px_32px_rgba(100,130,170,0.06)]">
        {children}
      </div>
    </section>
  );
}

function SectionRow({ icon: Icon, title, description, children, danger, onClick }: any) {
  return (
    <div 
      onClick={onClick}
      className={`flex items-center justify-between p-5 md:p-6 border-b border-white/40 last:border-b-0 transition-colors ${onClick ? 'cursor-pointer hover:bg-white/30' : ''}`}
    >
      <div className="flex items-center gap-4">
        {Icon && (
          <div className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${danger ? 'bg-red-50 text-red-500' : 'bg-graphite/5 text-graphite/60'}`}>
            <Icon className="w-5 h-5 stroke-[1.5]" />
          </div>
        )}
        <div>
          <h4 className={`text-[15px] font-medium ${danger ? 'text-red-500' : 'text-graphite'}`}>{title}</h4>
          {description && <p className="text-[13px] text-graphite/50 mt-0.5">{description}</p>}
        </div>
      </div>
      <div>
        {children || (onClick && <ChevronRight className="w-5 h-5 text-graphite/30" />)}
      </div>
    </div>
  );
}

// --- Главная страница ---

export function Settings() {
  const { user, logout } = useAuth();
  
  // States - Оформление
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>('system');
  
  // States - Уведомления
  const [notifOrders, setNotifOrders] = useState(true);
  const [notifDelivery, setNotifDelivery] = useState(true);
  const [notifNews, setNotifNews] = useState(false);
  const [notifBrands, setNotifBrands] = useState(true);

  // States - Адреса
  interface Address {
    id: number;
    title: string;
    address: string;
    isDefault: boolean;
  }
  const [addresses, setAddresses] = useState<Address[]>(() => {
    const saved = localStorage.getItem('zamk_addresses');
    if (saved) {
      try {
        return JSON.parse(saved);
      } catch (e) {}
    }
    return [
      { id: 1, title: 'Дом', address: 'г. Москва, ул. Тверская, д. 15, кв. 42', isDefault: true },
      { id: 2, title: 'Офис', address: 'г. Москва, Пресненская наб., д. 12', isDefault: false }
    ];
  });

  useEffect(() => {
    localStorage.setItem('zamk_addresses', JSON.stringify(addresses));
  }, [addresses]);

  // Modals
  const [isPasswordModalOpen, setPasswordModalOpen] = useState(false);
  const [isLogoutModalOpen, setLogoutModalOpen] = useState(false);
  const [isDeleteModalOpen, setDeleteModalOpen] = useState(false);
  const [isAddressModalOpen, setAddressModalOpen] = useState(false);

  // Address editing state
  const [editingAddress, setEditingAddress] = useState<{ id?: number; title: string; address: string } | null>(null);

  const handleOpenAddressModal = (address?: { id: number; title: string; address: string }) => {
    if (address) {
      setEditingAddress(address);
    } else {
      setEditingAddress({ title: '', address: '' });
    }
    setAddressModalOpen(true);
  };

  const handleSaveAddress = () => {
    if (!editingAddress || !editingAddress.title.trim() || !editingAddress.address.trim()) return;

    if (editingAddress.id) {
      setAddresses(addresses.map(a => a.id === editingAddress.id ? { ...a, title: editingAddress.title, address: editingAddress.address } : a));
    } else {
      const newId = addresses.length > 0 ? Math.max(...addresses.map(a => a.id)) + 1 : 1;
      setAddresses([...addresses, { id: newId, title: editingAddress.title, address: editingAddress.address, isDefault: addresses.length === 0 }]);
    }
    setAddressModalOpen(false);
  };

  const handleDeleteAddress = (id: number) => {
    setAddresses(addresses.filter(a => a.id !== id));
  };

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[800px]'>
        
        {/* Заголовок */}
        <div className="mb-12 pl-2">
          <h1 className="font-serif text-4xl md:text-5xl text-graphite mb-2">Настройки</h1>
          <p className="text-graphite/50 text-[15px]">Управление аккаунтом и предпочтениями</p>
        </div>

        {/* 1. Безопасность */}
        <Section title="Безопасность">
          <SectionRow 
            icon={Lock} 
            title="Сменить пароль" 
            description="Обновить текущий пароль учетной записи"
            onClick={() => setPasswordModalOpen(true)}
          />
          <SectionRow 
            icon={LogOut} 
            title="Выйти со всех устройств" 
            description="Завершить сеансы на других устройствах"
            onClick={() => setLogoutModalOpen(true)}
          />
          <SectionRow 
            icon={Trash2} 
            title="Удалить аккаунт" 
            description="Навсегда удалить профиль и все данные"
            danger
            onClick={() => setDeleteModalOpen(true)}
          />
        </Section>

        {/* 2. Внешний вид */}
        <Section title="Внешний вид">
          <div className="p-5 md:p-6 flex flex-col gap-4">
            <p className="text-[15px] font-medium text-graphite mb-2">Тема интерфейса</p>
            <div className="bg-graphite/5 p-1.5 rounded-2xl flex relative w-full overflow-hidden">
              {/* Ползунок */}
              <motion.div
                layoutId="theme-highlighter"
                initial={false}
                className="absolute top-1.5 bottom-1.5 w-[calc(33.333%-4px)] bg-white rounded-xl shadow-[0_2px_8px_rgba(0,0,0,0.04)]"
                animate={{
                  x: theme === 'light' ? 0 : theme === 'dark' ? '100%' : '200%',
                }}
                transition={{ type: "spring", stiffness: 400, damping: 30 }}
              />
              {/* Опции */}
              {(['light', 'dark', 'system'] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => setTheme(t)}
                  className={`relative z-10 flex-1 flex items-center justify-center gap-2 py-2.5 text-[13px] font-medium transition-colors ${theme === t ? 'text-graphite' : 'text-graphite/40 hover:text-graphite/70'}`}
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

        {/* 3. Уведомления */}
        <Section title="Уведомления">
          <SectionRow title="Статусы заказов" description="Изменения в статусе обработки">
            <ToggleSwitch isOn={notifOrders} onToggle={() => setNotifOrders(!notifOrders)} />
          </SectionRow>
          <SectionRow title="Доставка" description="Оповещения курьерских служб">
            <ToggleSwitch isOn={notifDelivery} onToggle={() => setNotifDelivery(!notifDelivery)} />
          </SectionRow>
          <SectionRow title="Новые поступления" description="Дропы и свежие коллекции">
            <ToggleSwitch isOn={notifBrands} onToggle={() => setNotifBrands(!notifBrands)} />
          </SectionRow>
          <SectionRow title="Подборки и новости" description="Редакционные материалы ZAMK">
            <ToggleSwitch isOn={notifNews} onToggle={() => setNotifNews(!notifNews)} />
          </SectionRow>
        </Section>

        {/* 4. Доставка */}
        <Section title="Доставка (Адреса)">
          <div className="p-5 md:p-6 grid gap-4">
            {addresses.map((item) => (
              <div key={item.id} className={`p-4 rounded-2xl border transition-all ${item.isDefault ? 'border-graphite/30 bg-white/60' : 'border-white/50 bg-white/20'}`}>
                <div className="flex justify-between items-start mb-2">
                  <div className="flex items-center gap-2">
                    <MapPin className="w-4 h-4 text-graphite/50" />
                    <h5 className="font-medium text-graphite text-[14px]">{item.title}</h5>
                    {item.isDefault && (
                      <span className="bg-graphite/10 text-graphite px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wider font-semibold">Основной</span>
                    )}
                  </div>
                  <div className="flex gap-3 text-[12px] font-medium">
                    <button onClick={() => handleOpenAddressModal(item)} className="text-graphite/40 hover:text-graphite transition-colors">Изм.</button>
                    {!item.isDefault && <button onClick={() => handleDeleteAddress(item.id)} className="text-red-400 hover:text-red-500 transition-colors">Удал.</button>}
                  </div>
                </div>
                <p className="text-[13px] text-graphite/60 pl-6 leading-relaxed">{item.address}</p>
                {!item.isDefault && (
                   <div className="pl-6 mt-3">
                     <button onClick={() => setAddresses(addresses.map(a => ({...a, isDefault: a.id === item.id})))} className="text-[12px] text-graphite hover:underline underline-offset-4">
                       Сделать основным
                     </button>
                   </div>
                )}
              </div>
            ))}
            
            <button onClick={() => handleOpenAddressModal()} className="flex items-center justify-center gap-2 w-full py-4 mt-2 border border-dashed border-graphite/20 rounded-2xl text-graphite/50 hover:text-graphite hover:border-graphite/40 hover:bg-graphite/5 transition-all text-[14px] font-medium">
              <Plus className="w-4 h-4" />
              Добавить адрес
            </button>
          </div>
        </Section>

      </div>

      {/* Модалки */}
      
      {/* Смена пароля */}
      <Modal isOpen={isPasswordModalOpen} onClose={() => setPasswordModalOpen(false)} title="Смена пароля">
        <div className="space-y-4 pt-2">
          <Input type="password" placeholder="Текущий пароль" className="bg-white/50" />
          <Input type="password" placeholder="Новый пароль" className="bg-white/50" />
          <Input type="password" placeholder="Повторите новый пароль" className="bg-white/50" />
          <Button className="w-full mt-4 bg-graphite" onClick={() => setPasswordModalOpen(false)}>Сохранить пароль</Button>
        </div>
      </Modal>

      {/* Выход со всех устройств */}
      <Modal isOpen={isLogoutModalOpen} onClose={() => setLogoutModalOpen(false)} title="Завершение сеансов">
        <div className="pt-2 text-center pb-2">
          <p className="text-graphite/70 text-[15px] mb-6">Вы уверены, что хотите выйти из аккаунта на всех других устройствах? Текущий сеанс будет сохранен.</p>
          <div className="grid grid-cols-2 gap-3">
            <Button variant="outline" onClick={() => setLogoutModalOpen(false)}>Отмена</Button>
            <Button className="bg-graphite" onClick={() => setLogoutModalOpen(false)}>Подтвердить</Button>
          </div>
        </div>
      </Modal>

      {/* Удаление аккаунта */}
      <Modal isOpen={isDeleteModalOpen} onClose={() => setDeleteModalOpen(false)} title="Удаление аккаунта">
        <div className="pt-2 text-center pb-2">
          <div className="w-16 h-16 bg-red-50 text-red-500 rounded-full flex items-center justify-center mx-auto mb-4">
            <Trash2 className="w-8 h-8" />
          </div>
          <p className="text-graphite mb-2 font-medium">Это действие необратимо</p>
          <p className="text-graphite/60 text-[14px] mb-8">Вся информация, включая заказы, избранное и адреса, будет удалена навсегда без возможности восстановления.</p>
          
          <div className="flex flex-col gap-3">
            <Button className="w-full bg-red-500 hover:bg-red-600 text-white border-0" onClick={() => { setDeleteModalOpen(false); logout(); }}>
              Да, удалить аккаунт
            </Button>
            <button 
              onClick={() => setDeleteModalOpen(false)}
              className="py-3 text-[14px] font-medium text-graphite/60 hover:text-graphite transition-colors mt-2"
            >
              Отменить
            </button>
          </div>
        </div>
      </Modal>

      {/* Адрес Модалка */}
      <Modal isOpen={isAddressModalOpen} onClose={() => setAddressModalOpen(false)} title={editingAddress?.id ? "Редактировать адрес" : "Новый адрес"}>
        <div className="pt-2 flex flex-col gap-4">
          <div>
            <label className="text-[12px] font-medium text-graphite/60 uppercase tracking-wider mb-2 block">Название (например, Дом или Офис)</label>
            <Input 
              value={editingAddress?.title || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, title: e.target.value } : { title: e.target.value, address: '' })}
              className="bg-white/50 border-white/40 shadow-sm"
              placeholder="Дом"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 uppercase tracking-wider mb-2 block">Полный адрес</label>
            <textarea
              value={editingAddress?.address || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, address: e.target.value } : { title: '', address: e.target.value })}
              className="w-full flex min-h-[80px] rounded-xl border border-white/50 bg-white/50 px-4 py-3 text-[14px] placeholder:text-graphite/30 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-graphite/20 focus-visible:border-graphite/30 disabled:cursor-not-allowed disabled:opacity-50 transition-all font-light resize-none shadow-[0_8px_32px_rgba(20,30,40,0.04)]"
              placeholder="г. Москва, ул. Пушкина, д. Колотушкина, кв. 100"
            />
          </div>
          <Button onClick={handleSaveAddress} className="w-full mt-4">
            Сохранить адрес
          </Button>
        </div>
      </Modal>

    </div>
  );
}
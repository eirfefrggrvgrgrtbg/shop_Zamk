import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Moon, Sun, Monitor, MapPin, ChevronRight, X, Plus, CreditCard } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { useTheme } from '../contexts/ThemeContext';
import { Modal } from '../components/ui/Modal';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

// --- Компоненты UI для настроек ---

function ToggleSwitch({ isOn, onToggle }: { isOn: boolean; onToggle: () => void }) {
  return (
    <div 
      onClick={onToggle} 
      className={`w-11 h-6 flex items-center rounded-full p-1 cursor-pointer transition-colors duration-300 ${isOn ? 'bg-graphite dark:bg-white' : 'bg-graphite/20 dark:bg-white/20'}`}
    >
      <motion.div 
        layout 
        className="w-4 h-4 bg-white dark:bg-black rounded-full shadow-sm"
        transition={{ type: "spring", stiffness: 500, damping: 30 }}
        style={{ marginLeft: isOn ? 'auto' : '0' }}
      />
    </div>
  );
}

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

function SectionRow({ icon: Icon, title, description, children, danger, onClick }: any) {
  return (
    <div 
      onClick={onClick}
      className={`flex items-center justify-between p-5 md:p-6 border-b border-white/40 dark:border-white/10 last:border-b-0 transition-colors ${onClick ? 'cursor-pointer hover:bg-white/30 dark:hover:bg-white/10' : ''}`}
    >
      <div className="flex items-center gap-4">
        {Icon && (
          <div className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${danger ? 'bg-red-50 dark:bg-red-950/30 text-red-500 dark:text-red-400' : 'bg-graphite/5 dark:bg-white/10 text-graphite/60 dark:text-white/60'}`}>
            <Icon className="w-5 h-5 stroke-[1.5]" />
          </div>
        )}
        <div>
          <h4 className={`text-[15px] font-medium ${danger ? 'text-red-500 dark:text-red-400' : 'text-graphite dark:text-white'}`}>{title}</h4>
          {description && <p className="text-[13px] text-graphite/50 dark:text-white/50 mt-0.5">{description}</p>}
        </div>
      </div>
      <div>
        {children || (onClick && <ChevronRight className="w-5 h-5 text-graphite/30 dark:text-white/30" />)}
      </div>
    </div>
  );
}

// --- Главная страница ---

export function Settings() {
  const { user, logout } = useAuth();
  
  // States - Оформление
  const { theme, setTheme } = useTheme();

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

  // States - Способы оплаты
  interface PaymentMethod {
    id: number;
    cardNumber: string;
    expiry: string;
    isDefault: boolean;
  }
  const [payments, setPayments] = useState<PaymentMethod[]>(() => {
    const saved = localStorage.getItem('zamk_payments');
    if (saved) {
      try {
        return JSON.parse(saved);
      } catch (e) {}
    }
    return [
      { id: 1, cardNumber: '**** **** **** 4242', expiry: '12/25', isDefault: true }
    ];
  });

  useEffect(() => {
    localStorage.setItem('zamk_payments', JSON.stringify(payments));
  }, [payments]);

  // Modals
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

  // Payment editing state
  const [isPaymentModalOpen, setPaymentModalOpen] = useState(false);
  const [editingPayment, setEditingPayment] = useState<{ id?: number; cardNumber: string; expiry: string } | null>(null);

  const handleOpenPaymentModal = (payment?: PaymentMethod) => {
    if (payment) {
      setEditingPayment(payment);
    } else {
      setEditingPayment({ cardNumber: '', expiry: '' });
    }
    setPaymentModalOpen(true);
  };

  const handleSavePayment = () => {
    if (!editingPayment || !editingPayment.cardNumber.trim() || !editingPayment.expiry.trim()) return;

    if (editingPayment.id) {
      setPayments(payments.map(p => p.id === editingPayment.id ? { ...p, cardNumber: editingPayment.cardNumber, expiry: editingPayment.expiry } : p));
    } else {
      const newId = payments.length > 0 ? Math.max(...payments.map(p => p.id)) + 1 : 1;
      setPayments([...payments, { id: newId, cardNumber: editingPayment.cardNumber, expiry: editingPayment.expiry, isDefault: payments.length === 0 }]);
    }
    setPaymentModalOpen(false);
  };

  const handleDeletePayment = (id: number) => {
    setPayments(payments.filter(p => p.id !== id));
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
              {/* Ползунок */}
              <motion.div
                layoutId="theme-highlighter"
                initial={false}
                className="absolute top-1.5 bottom-1.5 w-[calc(33.333%-4px)] bg-white dark:bg-black rounded-xl shadow-[0_2px_8px_rgba(0,0,0,0.04)]"
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
          <div className="p-5 md:p-6 grid gap-4">
            {payments.map((item) => (
              <div key={item.id} className={`p-4 rounded-2xl border transition-all ${item.isDefault ? 'border-graphite/30 dark:border-white/20 bg-white/60 dark:bg-white/5' : 'border-white/50 dark:border-white/10 bg-white/20 dark:bg-white/5'}`}>
                <div className="flex justify-between items-start mb-2">
                  <div className="flex items-center gap-2">
                    <CreditCard className="w-4 h-4 text-graphite/50 dark:text-white/50" />
                    <h5 className="font-medium text-graphite dark:text-white text-[14px]">{item.cardNumber}</h5>
                    {item.isDefault && (
                      <span className="bg-graphite/10 dark:bg-white/10 text-graphite dark:text-white px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wider font-semibold">Основной</span>
                    )}
                  </div>
                  <div className="flex gap-3 text-[12px] font-medium">
                    <button onClick={() => handleOpenPaymentModal(item)} className="text-graphite/40 dark:text-white/40 hover:text-graphite dark:hover:text-white transition-colors">Изм.</button>
                    {!item.isDefault && <button onClick={() => handleDeletePayment(item.id)} className="text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 transition-colors">Удал.</button>}
                  </div>
                </div>
                <p className="text-[13px] text-graphite/60 dark:text-white/60 pl-6 leading-relaxed">Срок действия: {item.expiry}</p>
                {!item.isDefault && (
                   <div className="pl-6 mt-3">
                     <button onClick={() => setPayments(payments.map(a => ({...a, isDefault: a.id === item.id})))} className="text-[12px] text-graphite dark:text-white hover:underline underline-offset-4">
                       Сделать основным
                     </button>
                   </div>
                )}
              </div>
            ))}
            
            <button onClick={() => handleOpenPaymentModal()} className="flex items-center justify-center gap-2 w-full py-4 mt-2 border border-dashed border-graphite/20 dark:border-white/20 rounded-2xl text-graphite/50 dark:text-white/50 hover:text-graphite dark:hover:text-white hover:border-graphite/40 dark:hover:border-white/40 hover:bg-graphite/5 dark:hover:bg-white/5 transition-all text-[14px] font-medium">
              <Plus className="w-4 h-4" />
              Добавить способ оплаты
            </button>
          </div>
        </Section>

        {/* 4. Доставка */}
        <Section title="Доставка (Адреса)">
          <div className="p-5 md:p-6 grid gap-4">
            {addresses.map((item) => (
              <div key={item.id} className={`p-4 rounded-2xl border transition-all ${item.isDefault ? 'border-graphite/30 dark:border-white/20 bg-white/60 dark:bg-white/5' : 'border-white/50 dark:border-white/10 bg-white/20 dark:bg-white/5'}`}>
                <div className="flex justify-between items-start mb-2">
                  <div className="flex items-center gap-2">
                    <MapPin className="w-4 h-4 text-graphite/50 dark:text-white/50" />
                    <h5 className="font-medium text-graphite dark:text-white text-[14px]">{item.title}</h5>
                    {item.isDefault && (
                      <span className="bg-graphite/10 dark:bg-white/10 text-graphite dark:text-white px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wider font-semibold">Основной</span>
                    )}
                  </div>
                  <div className="flex gap-3 text-[12px] font-medium">
                    <button onClick={() => handleOpenAddressModal(item)} className="text-graphite/40 dark:text-white/40 hover:text-graphite dark:hover:text-white transition-colors">Изм.</button>
                    {!item.isDefault && <button onClick={() => handleDeleteAddress(item.id)} className="text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 transition-colors">Удал.</button>}
                  </div>
                </div>
                <p className="text-[13px] text-graphite/60 dark:text-white/60 pl-6 leading-relaxed">{item.address}</p>
                {!item.isDefault && (
                   <div className="pl-6 mt-3">
                     <button onClick={() => setAddresses(addresses.map(a => ({...a, isDefault: a.id === item.id})))} className="text-[12px] text-graphite dark:text-white hover:underline underline-offset-4">
                       Сделать основным
                     </button>
                   </div>
                )}
              </div>
            ))}
            
            <button onClick={() => handleOpenAddressModal()} className="flex items-center justify-center gap-2 w-full py-4 mt-2 border border-dashed border-graphite/20 dark:border-white/20 rounded-2xl text-graphite/50 dark:text-white/50 hover:text-graphite dark:hover:text-white hover:border-graphite/40 dark:hover:border-white/40 hover:bg-graphite/5 dark:hover:bg-white/5 transition-all text-[14px] font-medium">
              <Plus className="w-4 h-4" />
              Добавить адрес
            </button>
          </div>
        </Section>

        

        

      </div>

      {/* Модалки */}
      {/* Адрес Модалка */}
      <Modal isOpen={isAddressModalOpen} onClose={() => setAddressModalOpen(false)} title={editingAddress?.id ? "Редактировать адрес" : "Новый адрес"}>
        <div className="pt-2 flex flex-col gap-4">
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Название (например, Дом или Офис)</label>
            <Input 
              value={editingAddress?.title || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, title: e.target.value } : { title: e.target.value, address: '' })}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20 shadow-sm"
              placeholder="Дом"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Полный адрес</label>
            <textarea
              value={editingAddress?.address || ''} 
                onChange={(e) => setEditingAddress(prev => prev ? { ...prev, address: e.target.value } : { title: '', address: e.target.value })}
              className="w-full flex min-h-[80px] rounded-xl border border-white/50 dark:border-white/20 bg-white/50 dark:bg-white/10 px-4 py-3 text-[14px] text-graphite dark:text-white placeholder:text-graphite/30 dark:placeholder:text-white/30 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-graphite/20 dark:focus-visible:ring-white/30 focus-visible:border-graphite/30 dark:focus-visible:border-white/30 disabled:cursor-not-allowed disabled:opacity-50 transition-all font-light resize-none shadow-[0_8px_32px_rgba(20,30,40,0.04)] dark:shadow-[0_8px_32px_rgba(0,0,0,0.3)]"
              placeholder="г. Москва, ул. Пушкина, д. Колотушкина, кв. 100"
            />
          </div>
          <Button onClick={handleSaveAddress} className="w-full mt-4">
            Сохранить адрес
          </Button>
        </div>
      </Modal>

      {/* Оплата Модалка */}
      <Modal isOpen={isPaymentModalOpen} onClose={() => setPaymentModalOpen(false)} title={editingPayment?.id ? "Редактировать карту" : "Добавить карту"}>
        <div className="pt-2 flex flex-col gap-4">
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Номер карты</label>
            <Input 
              value={editingPayment?.cardNumber || ''} 
              onChange={(e) => setEditingPayment(prev => prev ? { ...prev, cardNumber: e.target.value } : { cardNumber: e.target.value, expiry: '' })}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20 shadow-sm"
              placeholder="0000 0000 0000 0000"
            />
          </div>
          <div>
            <label className="text-[12px] font-medium text-graphite/60 dark:text-white/60 uppercase tracking-wider mb-2 block">Срок действия</label>
            <Input
              value={editingPayment?.expiry || ''} 
              onChange={(e) => setEditingPayment(prev => prev ? { ...prev, expiry: e.target.value } : { cardNumber: '', expiry: e.target.value })}
              className="bg-white/50 dark:bg-white/10 border-white/40 dark:border-white/20 shadow-sm w-1/2"
              placeholder="ММ/ГГ"
            />
          </div>
          <Button onClick={handleSavePayment} className="w-full mt-4">
            Сохранить способ оплаты
          </Button>
        </div>
      </Modal>

    </div>
  );
}
import { useState } from 'react';
import { MapPin, Mail, Phone, Clock, Check } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { motion, AnimatePresence } from 'framer-motion';

export function Contacts() {
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [subject, setSubject] = useState('');
  const [message, setMessage] = useState('');
  const [isSubmitted, setIsSubmitted] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (email.includes('@')) {
      setIsSubmitted(true);
      setName('');
      setEmail('');
      setSubject('');
      setMessage('');
      setTimeout(() => setIsSubmitted(false), 5000);
    }
  };

  return (
    <div className="min-h-screen relative z-10 pt-32 md:pt-40 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="mb-12 max-w-[980px] mx-auto border-b border-border-lighter dark:border-white/10 pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash dark:text-white/50 uppercase mb-3">
            Связаться с нами
          </p>
          <div className="flex items-end justify-between">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Контакты
            </h1>
          </div>
        </section>

        <div className="mt-8 grid grid-cols-1 sm:grid-cols-2 gap-6 mb-12 max-w-[980px] mx-auto">
          {[
            { icon: Mail, label: 'Эл. почта', value: 'hello@zamk.store' },
            { icon: Phone, label: 'Телефон', value: '+7 (495) 123-45-67' },
            { icon: MapPin, label: 'Адрес', value: 'Москва, ул. Пречистенка, 12' },
            { icon: Clock, label: 'Часы работы', value: 'Пн-Пт 10:00 — 20:00' },
          ].map(item => (
            <div key={item.label} className="bg-white/60 dark:bg-white/5 backdrop-blur-md rounded-[2rem] border border-white dark:border-white/10 p-6 md:p-8 flex items-start gap-5 shadow-sm hover:shadow-cloud dark:hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] transition-all duration-300">
              <div className="w-12 h-12 rounded-[1rem] bg-white dark:bg-white/10 border border-border-lighter dark:border-white/20 flex items-center justify-center shrink-0 shadow-sm">
                <item.icon className="w-6 h-6 text-primary dark:text-white" />
              </div>
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-ash dark:text-white/60 mb-1.5">{item.label}</p>
                <p className="text-base font-medium text-graphite dark:text-white">{item.value}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="bg-white/70 dark:bg-white/5 backdrop-blur-xl rounded-[2.5rem] border border-white dark:border-white/10 p-8 md:p-12 shadow-cloud dark:shadow-[0_8px_32px_rgba(0,0,0,0.3)] max-w-[980px] mx-auto relative overflow-hidden">
          <h2 className="text-2xl font-serif text-graphite dark:text-white mb-8 text-center">Написать нам</h2>
          
          <form className="space-y-5" onSubmit={handleSubmit}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <Input 
                placeholder="Имя" 
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="bg-white/50 dark:bg-white/10 border-white/60 dark:border-white/20 focus:bg-white dark:focus:bg-white/20" 
              />
              <Input 
                placeholder="Эл. почта" 
                type="email" 
                required
                pattern=".*@.*"
                title="Почта должна содержать символ @"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="bg-white/50 dark:bg-white/10 border-white/60 dark:border-white/20 focus:bg-white dark:focus:bg-white/20" 
              />
            </div>
            <Input 
              placeholder="Тема обращения" 
              required
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              className="bg-white/50 dark:bg-white/10 border-white/60 dark:border-white/20 focus:bg-white dark:focus:bg-white/20" 
            />
            <textarea
              placeholder="Ваше сообщение"
              required
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              rows={4}
              className="w-full rounded-[18px] border border-border-soft dark:border-white/20 bg-white/50 dark:bg-white/10 px-5 py-4 text-[14px] text-graphite dark:text-white placeholder:text-ash-light dark:placeholder:text-white/50 focus:bg-white dark:focus:bg-white/20 focus:outline-none focus:ring-1 focus:ring-graphite/20 dark:focus:ring-white/30 focus:border-graphite/40 dark:focus:border-white/40 transition-all resize-none shadow-sm"
            />
            <div className="pt-2 text-center relative">
              <Button type="submit" variant="primary" size="lg" className="px-10 shadow-sm relative z-10 w-full sm:w-auto">
                Отправить сообщение
              </Button>
              
              <AnimatePresence>
                {isSubmitted && (
                  <motion.div
                    initial={{ opacity: 0, scale: 0.9, y: 10 }}
                    animate={{ opacity: 1, scale: 1, y: 0 }}
                    exit={{ opacity: 0, scale: 0.9, y: 10 }}
                    className="absolute left-1/2 -translate-x-1/2 -top-14 bg-white/90 dark:bg-white/10 backdrop-blur-md border border-white dark:border-white/20 px-5 py-3 rounded-2xl shadow-[0_8px_30px_rgba(100,130,170,0.15)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] flex items-center gap-3 z-20 whitespace-nowrap"
                  >
                    <div className="w-6 h-6 rounded-full bg-graphite/5 dark:bg-white/20 flex items-center justify-center text-graphite dark:text-white flex-shrink-0">
                      <Check className="w-3.5 h-3.5" />
                    </div>
                    <span className="text-[13.5px] font-medium text-graphite dark:text-white">Сообщение успешно отправлено</span>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

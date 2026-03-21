import { MapPin, Mail, Phone, Clock } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

export function Contacts() {
  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl py-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-8">Контакты</h1>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-10">
          {[
            { icon: Mail, label: 'Эл. почта', value: 'hello@zamk.store' },
            { icon: Phone, label: 'Телефон', value: '+7 (495) 123-45-67' },
            { icon: MapPin, label: 'Адрес', value: 'Москва, ул. Пречистенка, 12' },
            { icon: Clock, label: 'Часы работы', value: 'Пн-Пт 10:00 — 20:00' },
          ].map(item => (
            <div key={item.label} className="bg-white rounded-2xl border border-border-lighter p-5 flex items-start gap-3">
              <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center shrink-0">
                <item.icon className="w-5 h-5 text-primary" />
              </div>
              <div>
                <p className="text-xs text-ash mb-0.5">{item.label}</p>
                <p className="text-sm font-medium text-graphite">{item.value}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="bg-white rounded-3xl border border-border-lighter p-6 sm:p-8">
          <h2 className="text-xl font-semibold text-graphite mb-5">Написать нам</h2>
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Input placeholder="Имя" />
              <Input placeholder="Эл. почта" type="email" />
            </div>
            <Input placeholder="Тема обращения" />
            <textarea
              placeholder="Ваше сообщение"
              rows={4}
              className="w-full rounded-2xl border border-border-soft bg-white px-4 py-3 text-sm text-graphite placeholder:text-ash-light focus:outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary/50 transition-all resize-none"
            />
            <Button variant="primary">Отправить сообщение</Button>
          </div>
        </div>
      </div>
    </div>
  );
}

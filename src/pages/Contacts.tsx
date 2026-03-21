import { MapPin, Mail, Phone, Clock } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';

export function Contacts() {
  return (
    <div className="min-h-screen relative z-10 pt-16 md:pt-20 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="overflow-hidden rounded-[0.8rem] border border-white/45 bg-white/16 backdrop-blur-sm">
          <div className="relative h-[190px] md:h-[240px]">
            <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)]" />
            <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))]" />
            <div className="relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7">
              <h2 className="font-serif text-[clamp(2.1rem,6.1vw,6.4rem)] text-white/43 leading-[0.8] tracking-[-0.03em]">КОНТАКТЫ</h2>
              <h3 className="font-serif text-[clamp(1.7rem,4.9vw,4.8rem)] text-white/42 leading-[0.82] tracking-[-0.03em] text-center">СВЯЗАТЬСЯ С НАМИ</h3>
            </div>
          </div>
        </section>

        <div className="mt-8 grid grid-cols-1 sm:grid-cols-2 gap-6 mb-12 max-w-[980px] mx-auto">
          {[
            { icon: Mail, label: 'Эл. почта', value: 'hello@zamk.store' },
            { icon: Phone, label: 'Телефон', value: '+7 (495) 123-45-67' },
            { icon: MapPin, label: 'Адрес', value: 'Москва, ул. Пречистенка, 12' },
            { icon: Clock, label: 'Часы работы', value: 'Пн-Пт 10:00 — 20:00' },
          ].map(item => (
            <div key={item.label} className="bg-white/60 backdrop-blur-md rounded-[2rem] border border-white p-6 md:p-8 flex items-start gap-5 shadow-sm hover:shadow-cloud transition-all duration-300">
              <div className="w-12 h-12 rounded-[1rem] bg-white border border-border-lighter flex items-center justify-center shrink-0 shadow-sm">
                <item.icon className="w-6 h-6 text-primary" />
              </div>
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-ash mb-1.5">{item.label}</p>
                <p className="text-base font-medium text-graphite">{item.value}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="bg-white/70 backdrop-blur-xl rounded-[2.5rem] border border-white p-8 md:p-12 shadow-cloud max-w-[980px] mx-auto">
          <h2 className="text-2xl font-serif text-graphite mb-8 text-center">Написать нам</h2>
          <div className="space-y-5">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <Input placeholder="Имя" className="bg-white/50 border-white/60 focus:bg-white" />
              <Input placeholder="Эл. почта" type="email" className="bg-white/50 border-white/60 focus:bg-white" />
            </div>
            <Input placeholder="Тема обращения" className="bg-white/50 border-white/60 focus:bg-white" />
            <textarea
              placeholder="Ваше сообщение"
              rows={4}
              className="w-full rounded-2xl border border-white bg-white/50 px-5 py-4 text-base text-graphite placeholder:text-ash-light focus:bg-white focus:outline-none focus:ring-2 focus:ring-primary/30 transition-all resize-none shadow-sm"
            />
            <div className="pt-2 text-center">
              <Button variant="primary" size="lg" className="px-10 shadow-sm">Отправить сообщение</Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

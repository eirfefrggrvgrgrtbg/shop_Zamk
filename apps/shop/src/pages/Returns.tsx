import { ArrowLeft } from 'lucide-react';
import { Link } from 'react-router-dom';
import { SectionHeader } from '../components/editorial/StudioKit';
import { Button } from '../components/ui/Button';

export function Returns() {
  return (
    <div className="relative z-10 min-h-screen pt-16 md:pt-20 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl">
        <Link to="/" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite mb-10">
          <ArrowLeft className="w-4 h-4" /> На главную
        </Link>

        {/* Header */}
        <section className="mb-12">
          <h1 className="font-serif text-[clamp(2.5rem,5vw,4.5rem)] text-graphite leading-[1] tracking-tight mb-4">
            Возврат<br />и обмен
          </h1>
          <p className="text-graphite-light text-base md:text-lg leading-relaxed">
            Процесс оформления возврата или обмена на платформе ZAMK. Мы хотим, чтобы архивные вещи и локальные марки радовали вас. Если вещь не подошла, мы сделали процедуру максимально прозрачной.
          </p>
        </section>

        {/* Content Blocks */}
        <div className="space-y-10">
          <div className="glass-panel p-6 md:p-8">
            <SectionHeader
              label="Условия"
              title="Сроки возврата"
              description="Вы можете вернуть вещь в течение 14 дней с момента получения заказа."
            />
            <ul className="mt-4 space-y-3 text-graphite-light text-sm">
              <li className="flex gap-3">
                <span className="text-primary mt-1">•</span>
                <span>Вещь не была в употреблении, сохранены товарный вид и потребительские свойства.</span>
              </li>
              <li className="flex gap-3">
                <span className="text-primary mt-1">•</span>
                <span>Сохранены все фабричные ярлыки, пломбы и оригинальная упаковка кураторской витрины.</span>
              </li>
              <li className="flex gap-3">
                <span className="text-primary mt-1">•</span>
                <span>У вас есть документ, подтверждающий факт и условия покупки.</span>
              </li>
            </ul>
          </div>

          <div className="glass-panel p-6 md:p-8">
            <SectionHeader
              label="Инструкция"
              title="Как оформить возврат"
              description="Три простых шага для отправки вещи обратно в архив."
            />
            <div className="mt-6 space-y-6">
              <div className="flex gap-4">
                <div className="w-8 h-8 rounded-full border border-border-soft flex items-center justify-center font-serif text-lg flex-shrink-0 text-graphite">1</div>
                <div>
                  <h4 className="font-medium text-graphite mb-1">Заполните заявку</h4>
                  <p className="text-sm text-graphite-light">Перейдите в личный кабинет в раздел «Возвраты» или свяжитесь со службой поддержки.</p>
                </div>
              </div>
              <div className="flex gap-4">
                <div className="w-8 h-8 rounded-full border border-border-soft flex items-center justify-center font-serif text-lg flex-shrink-0 text-graphite">2</div>
                <div>
                  <h4 className="font-medium text-graphite mb-1">Упакуйте товар</h4>
                  <p className="text-sm text-graphite-light">Поместите вещь в оригинальную упаковку вместе со всеми ярлыками и заполненным заявлением.</p>
                </div>
              </div>
              <div className="flex gap-4">
                <div className="w-8 h-8 rounded-full border border-border-soft flex items-center justify-center font-serif text-lg flex-shrink-0 text-graphite">3</div>
                <div>
                  <h4 className="font-medium text-graphite mb-1">Передайте курьеру</h4>
                  <p className="text-sm text-graphite-light">Отправьте посылку курьерской службой. Стоимость обратной пересылки оплачивается покупателем (за исключением брака).</p>
                </div>
              </div>
            </div>
          </div>

          <div className="glass-panel p-6 md:p-8 bg-error/5 border-error/20">
            <h3 className="font-serif text-2xl text-graphite mb-3">Что нельзя вернуть</h3>
            <p className="text-sm text-graphite-light leading-relaxed">
              Согласно Постановлению Правительства, возврату и обмену не подлежат: нижнее белье, чулочно-носочные изделия, косметика, парфюмерия и ювелирные украшения.
            </p>
          </div>
          
          <div className="pt-4 flex justify-between items-center border-t border-border-lighter">
            <p className="text-sm text-graphite-light">Нужна помощь с возвратом?</p>
            <Link to="/contacts">
              <Button variant="secondary">Служба поддержки</Button>
            </Link>
          </div>
        </div>

      </div>
    </div>
  );
}

export function Privacy() {
  return (
    <div className="min-h-screen relative z-10 pt-16 md:pt-20 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="overflow-hidden rounded-[0.8rem] border border-white/45 dark:border-white/20 bg-white/16 dark:bg-white/5 backdrop-blur-sm">
          <div className="relative h-[190px] md:h-[240px]">
            <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_18%_50%,rgba(166,194,223,0.54),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(198,217,238,0.68),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(170,197,226,0.55),transparent_52%)] dark:bg-[radial-gradient(ellipse_at_18%_50%,rgba(100,120,140,0.3),transparent_50%),radial-gradient(ellipse_at_56%_48%,rgba(80,100,120,0.2),transparent_56%),radial-gradient(ellipse_at_84%_52%,rgba(90,110,130,0.25),transparent_52%)]" />
            <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(242,247,252,0.74),rgba(236,242,249,0.64))] dark:bg-[linear-gradient(180deg,rgba(20,20,25,0.5),rgba(15,15,20,0.4))]" />
            <div className="relative z-10 h-full px-5 md:px-10 flex flex-col items-start justify-center gap-2 md:flex-row md:items-center md:justify-between md:gap-7">
              <h2 className="font-serif text-[clamp(2rem,5.8vw,6rem)] text-white/43 dark:text-white/50 leading-[0.8] tracking-[-0.03em]">ПОЛИТИКА</h2>
              <h3 className="font-serif text-[clamp(1.55rem,4.5vw,4.2rem)] text-white/42 dark:text-white/40 leading-[0.82] tracking-[-0.03em] text-center">КОНФИДЕНЦИАЛЬНОСТИ</h3>
            </div>
          </div>
        </section>

        <div className="mt-8 bg-white/70 dark:bg-white/5 backdrop-blur-xl rounded-[2.5rem] border border-white dark:border-white/10 p-8 md:p-12 space-y-8 text-base text-ash dark:text-white/70 leading-relaxed shadow-cloud dark:shadow-[0_8px_32px_rgba(0,0,0,0.3)] max-w-[980px] mx-auto">
          <section>
            <h2 className="text-xl font-serif text-graphite dark:text-white mb-4">1. Общие положения</h2>
            <p>Настоящая Политика конфиденциальности устанавливает порядок получения, хранения, обработки, использования и защиты персональных данных пользователей платформы ЗАМК.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite dark:text-white mb-4">2. Сбор информации</h2>
            <p>Мы собираем информацию, которую вы предоставляете при оформлении заказа: имя, эл. почта, телефон, адрес доставки. Мы также автоматически собираем данные о вашем устройстве и браузере для улучшения работы сайта.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite dark:text-white mb-4">3. Использование информации</h2>
            <p>Персональные данные используются для обработки заказов, связи с вами, улучшения сервиса и отправки информации о новых коллекциях и акциях (с вашего согласия).</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite dark:text-white mb-4">4. Защита данных</h2>
            <p>Мы используем современные средства защиты информации, включая шифрование SSL. Доступ к персональным данным имеют только уполномоченные сотрудники.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite dark:text-white mb-4">5. Ваши права</h2>
            <p>Вы можете запросить доступ к вашим данным, их исправление или удаление, обратившись по адресу privacy@zamk.store.</p>
          </section>

          <p className="text-sm font-medium text-ash-light dark:text-white/50 pt-6 border-t border-white/60 dark:border-white/10">Последнее обновление: март 2026</p>
        </div>
      </div>
    </div>
  );
}

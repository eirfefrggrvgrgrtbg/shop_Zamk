export function Privacy() {
  return (
    <div className="min-h-screen relative z-10 pt-28 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-4xl">
        <h1 className="text-4xl sm:text-5xl font-serif text-graphite mb-10 text-center">Политика конфиденциальности</h1>

        <div className="bg-white/70 backdrop-blur-xl rounded-[2.5rem] border border-white p-8 md:p-12 space-y-8 text-base text-ash leading-relaxed shadow-cloud">
          <section>
            <h2 className="text-xl font-serif text-graphite mb-4">1. Общие положения</h2>
            <p>Настоящая Политика конфиденциальности устанавливает порядок получения, хранения, обработки, использования и защиты персональных данных пользователей платформы ЗАМК.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite mb-4">2. Сбор информации</h2>
            <p>Мы собираем информацию, которую вы предоставляете при оформлении заказа: имя, эл. почта, телефон, адрес доставки. Мы также автоматически собираем данные о вашем устройстве и браузере для улучшения работы сайта.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite mb-4">3. Использование информации</h2>
            <p>Персональные данные используются для обработки заказов, связи с вами, улучшения сервиса и отправки информации о новых коллекциях и акциях (с вашего согласия).</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite mb-4">4. Защита данных</h2>
            <p>Мы используем современные средства защиты информации, включая шифрование SSL. Доступ к персональным данным имеют только уполномоченные сотрудники.</p>
          </section>

          <section>
            <h2 className="text-xl font-serif text-graphite mb-4">5. Ваши права</h2>
            <p>Вы можете запросить доступ к вашим данным, их исправление или удаление, обратившись по адресу privacy@zamk.store.</p>
          </section>

          <p className="text-sm font-medium text-ash-light pt-6 border-t border-white/60">Последнее обновление: март 2026</p>
        </div>
      </div>
    </div>
  );
}

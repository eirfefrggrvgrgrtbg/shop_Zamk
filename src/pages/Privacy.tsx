export function Privacy() {
  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-3xl py-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-8">Политика конфиденциальности</h1>

        <div className="bg-white rounded-3xl border border-border-lighter p-6 sm:p-8 space-y-6 text-sm text-ash leading-relaxed">
          <section>
            <h2 className="text-lg font-semibold text-graphite mb-3">1. Общие положения</h2>
            <p>Настоящая Политика конфиденциальности устанавливает порядок получения, хранения, обработки, использования и защиты персональных данных пользователей платформы ЗАМК.</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-graphite mb-3">2. Сбор информации</h2>
            <p>Мы собираем информацию, которую вы предоставляете при оформлении заказа: имя, эл. почта, телефон, адрес доставки. Мы также автоматически собираем данные о вашем устройстве и браузере для улучшения работы сайта.</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-graphite mb-3">3. Использование информации</h2>
            <p>Персональные данные используются для обработки заказов, связи с вами, улучшения сервиса и отправки информации о новых коллекциях и акциях (с вашего согласия).</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-graphite mb-3">4. Защита данных</h2>
            <p>Мы используем современные средства защиты информации, включая шифрование SSL. Доступ к персональным данным имеют только уполномоченные сотрудники.</p>
          </section>

          <section>
            <h2 className="text-lg font-semibold text-graphite mb-3">5. Ваши права</h2>
            <p>Вы можете запросить доступ к вашим данным, их исправление или удаление, обратившись по адресу privacy@zamk.store.</p>
          </section>

          <p className="text-xs text-ash-light pt-4 border-t border-border-lighter">Последнее обновление: март 2026</p>
        </div>
      </div>
    </div>
  );
}

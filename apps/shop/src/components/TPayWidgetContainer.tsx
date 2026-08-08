import React from 'react';

/*
При реальном подключении:
1. загрузить официальный Integration Script;
2. создать Payment Integration;
3. updateWidgetTypes(['tpay']);
4. установить PaymentStartCallback;
5. callback вызывает наш Backend;
6. Backend выполняет Init;
7. callback возвращает PaymentURL.
*/

export type TPayWidgetState = 'not_configured' | 'loading_script' | 'ready' | 'unavailable' | 'error';

interface TPayWidgetContainerProps {
  state?: TPayWidgetState;
  className?: string;
  errorMessage?: string;
}

export const TPayWidgetContainer: React.FC<TPayWidgetContainerProps> = ({
  state = 'not_configured',
  className = '',
  errorMessage,
}) => {
  return (
    <div className={`p-4 border rounded-xl bg-gray-50 dark:bg-white/5 border-border-lighter dark:border-white/10 ${className}`}>
      <div className="flex items-center justify-between mb-2">
        <span className="font-medium text-sm text-graphite dark:text-white">Контейнер T-Pay Виджета</span>
        <span className={`text-xs px-2 py-0.5 rounded font-mono ${
          state === 'ready' ? 'bg-green-100 text-green-800' :
          state === 'loading_script' ? 'bg-yellow-100 text-yellow-800' :
          state === 'error' ? 'bg-red-100 text-red-800' : 'bg-gray-200 text-gray-700'
        }`}>
          {state}
        </span>
      </div>

      {state === 'not_configured' && (
        <p className="text-xs text-graphite-light dark:text-white/60">
          Официальный виджет Т-Банка не подключен. В тестовом режиме используется редирект на mock-страницу.
        </p>
      )}

      {state === 'loading_script' && (
        <p className="text-xs text-graphite-light dark:text-white/60 animate-pulse">
          Загрузка интеграционного скрипта Т-Банка...
        </p>
      )}

      {state === 'ready' && (
        <div id="tpay-widget-root" className="min-h-[48px] flex items-center justify-center border border-dashed rounded p-2 text-xs text-graphite">
          [Контейнер готов для инициализации Т-Банк Виджета]
        </div>
      )}

      {state === 'unavailable' && (
        <p className="text-xs text-amber-700 dark:text-amber-400">
          Виджет T-Pay временно недоступен. Воспользуйтесь оплатой картой.
        </p>
      )}

      {state === 'error' && (
        <p className="text-xs text-error">
           Ошибка виджета: {errorMessage || 'Не удалось загрузить скрипт оплаты.'}
        </p>
      )}
    </div>
  );
};

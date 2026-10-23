import { PillFilter } from './editorial/StudioKit';
import type { AvailablePaymentMethods } from '@zamk/api-client/src/customer';

const PAYMENT_LABELS: Record<string, string> = { card: 'Карта онлайн', tpay: 'T-Pay', sbp: 'СБП' };

export function PaymentMethodPicker({ available, failed, selected, onSelect }: {
  available: AvailablePaymentMethods | null;
  failed: boolean;
  selected: string;
  onSelect: (method: string) => void;
}) {
  if (failed) {
    return <p className='text-sm text-error' role='alert' data-testid='payment-methods-empty'>Не удалось получить способы оплаты. Обновите страницу и попробуйте ещё раз.</p>;
  }
  if (!available) {
    return <p className='text-sm text-graphite-light dark:text-white/60' data-testid='payment-methods-loading'>Загружаем способы оплаты…</p>;
  }
  if (available.methods.length === 0) {
    return <p className='text-sm text-error' role='alert' data-testid='payment-methods-empty'>Сейчас нет доступных способов оплаты. Заказ не оформлен — попробуйте позже.</p>;
  }
  const mock = available.methods.find(m => m.method === selected)?.mock;
  return (
    <>
      <div className='flex flex-wrap gap-2 items-center' data-testid='payment-methods'>
        {available.methods.map(m => (
          <PillFilter key={m.method} label={PAYMENT_LABELS[m.method] ?? m.method} active={selected === m.method} onClick={() => onSelect(m.method)} />
        ))}
      </div>
      <p className='mt-4 text-[13px] text-graphite-light dark:text-white/60 bg-ice dark:bg-white/5 p-3 rounded-xl border border-border-lighter dark:border-white/10'>
        {mock
          ? 'Тестовый режим: используется эмулятор оплаты. После клика «Оформить заказ» открывается тестовая страница подтверждения, реальное списание не производится.'
          : 'Оплата проходит на защищённой странице платёжного сервиса.'}
      </p>
    </>
  );
}

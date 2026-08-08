import React, { useState } from 'react';
import { createPayment } from '@zamk/api-client/src/customer';

export type TPayButtonState =
  | 'idle'
  | 'creating_payment'
  | 'opening_payment'
  | 'awaiting_confirmation'
  | 'succeeded'
  | 'failed'
  | 'cancelled'
  | 'unavailable';

interface TPayButtonProps {
  orderId: string;
  amountCents: number;
  disabled?: boolean;
  onPaymentCreated?: (paymentUrl: string) => void;
  className?: string;
}

export const TPayButton: React.FC<TPayButtonProps> = ({
  orderId,
  amountCents,
  disabled = false,
  onPaymentCreated,
  className = '',
}) => {
  const [buttonState, setButtonState] = useState<TPayButtonState>('idle');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleClick = async () => {
    if (disabled || buttonState === 'creating_payment' || buttonState === 'opening_payment') {
      return;
    }

    setButtonState('creating_payment');
    setErrorMessage(null);

    try {
      const data = await createPayment(orderId, 'tpay');
      
      if (data.status === 'succeeded') {
        setButtonState('succeeded');
      } else {
        setButtonState('awaiting_confirmation');
      }

      if (data.paymentUrl) {
        setButtonState('opening_payment');
        if (onPaymentCreated) {
          onPaymentCreated(data.paymentUrl);
        } else {
          window.location.href = data.paymentUrl;
        }
      }
    } catch (err: any) {
      if (err?.message?.includes('unavailable')) {
        setButtonState('unavailable');
        setErrorMessage('T-Pay временно недоступен. Используйте оплату картой.');
      } else {
        setButtonState('failed');
        setErrorMessage(err?.message || 'Не удалось создать платёж T-Pay');
      }
    }
  };

  const isButtonDisabled =
    disabled ||
    buttonState === 'creating_payment' ||
    buttonState === 'opening_payment' ||
    buttonState === 'unavailable' ||
    buttonState === 'succeeded';

  const getButtonText = () => {
    switch (buttonState) {
      case 'creating_payment':
        return 'Создание платежа...';
      case 'opening_payment':
        return 'Переход к оплате...';
      case 'awaiting_confirmation':
        return 'Платёж создан. Ожидаем подтверждения';
      case 'succeeded':
        return 'Оплата успешна';
      case 'failed':
        return 'Ошибка оплаты. Попробовать снова';
      case 'cancelled':
        return 'Платёж отменён. Попробовать снова';
      case 'unavailable':
        return 'T-Pay недоступен';
      default:
        return `Оплатить T-Pay: ${(amountCents / 100).toLocaleString('ru-RU')} ₽`;
    }
  };

  return (
    <div className="w-full space-y-2">
      <button
        type="button"
        onClick={handleClick}
        disabled={isButtonDisabled}
        className={`w-full flex justify-center items-center py-3 px-4 border border-transparent rounded-lg shadow-sm text-sm font-medium text-black bg-[#FFDD2D] hover:bg-[#F2C900] disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-[#FFDD2D] transition-colors duration-200 ease-in-out ${className}`}
      >
        {getButtonText()}
      </button>

      {buttonState === 'awaiting_confirmation' && (
        <p className="text-xs text-center text-graphite-light dark:text-white/70">
          Платёж создан. Ожидаем подтверждения
        </p>
      )}

      {errorMessage && (
        <p className="text-xs text-center text-red-600 dark:text-red-400">
          {errorMessage}
        </p>
      )}
    </div>
  );
};

import { useState } from 'react';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { useAuth } from '../contexts/AuthContext';
import { usePromo } from '../contexts/promoContextValue';
import { formatPrice } from '../lib/utils';

export function PromoCodeBox() {
  const { isAuthenticated, openAuthModal } = useAuth();
  const { appliedCode, discount, isChecking, isUnconfirmed, error, apply, remove, retry } = usePromo();
  const [draft, setDraft] = useState('');
  const [isReplacing, setIsReplacing] = useState(false);

  if (!isAuthenticated) {
    return (
      <div className='border-t border-border-lighter dark:border-white/10 pt-3 mt-2' data-testid='promo-box'>
        <p className='text-sm font-medium text-graphite dark:text-white'>Промокод</p>
        <button
          type='button'
          className='mt-1 text-xs text-ash underline hover:text-graphite dark:hover:text-white'
          onClick={() => openAuthModal('login')}
        >
          Войдите, чтобы применить промокод
        </button>
      </div>
    );
  }

  const showForm = !appliedCode || isReplacing;

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (isChecking) return;
    const ok = await apply(draft);
    if (ok) {
      setDraft('');
      setIsReplacing(false);
    }
  };

  return (
    <div className='border-t border-border-lighter dark:border-white/10 pt-3 mt-2' data-testid='promo-box'>
      <p className='text-sm font-medium text-graphite dark:text-white'>Промокод</p>

      {appliedCode && (
        <div className='mt-2 flex items-center justify-between gap-2 text-sm'>
          <div className='min-w-0'>
            <span className='font-medium text-graphite dark:text-white break-all'>{appliedCode}</span>
            {isChecking ? (
              <span className='block text-xs text-ash'>Проверяем промокод…</span>
            ) : isUnconfirmed ? (
              <span className='block text-xs text-ash'>Скидка не подтверждена</span>
            ) : (
              <span className='block text-xs text-success'>Скидка {formatPrice(discount)}</span>
            )}
          </div>
          <div className='flex shrink-0 items-center gap-3 text-xs'>
            {isUnconfirmed && !isChecking && (
              <button type='button' className='text-ash underline hover:text-graphite dark:hover:text-white' onClick={retry}>
                Повторить
              </button>
            )}
            {!isReplacing && (
              <button
                type='button'
                className='text-ash underline hover:text-graphite dark:hover:text-white'
                onClick={() => setIsReplacing(true)}
                disabled={isChecking}
              >
                Заменить
              </button>
            )}
            <button
              type='button'
              className='text-ash underline hover:text-error'
              onClick={remove}
              aria-label={`Удалить промокод ${appliedCode}`}
            >
              Удалить
            </button>
          </div>
        </div>
      )}

      {showForm && (
        <form className='mt-2 flex flex-col gap-2' onSubmit={submit}>
          <div>
            <Input
              placeholder='Введите промокод'
              aria-label='Промокод'
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              autoComplete='off'
              className='h-11 placeholder:normal-case'
              disabled={isChecking}
            />
          </div>
          <Button type='submit' variant='secondary' className='w-full' disabled={isChecking || !draft.trim()}>
            {isChecking ? 'Проверяем…' : 'Применить'}
          </Button>
        </form>
      )}

      {error && (
        <p className='mt-2 text-xs text-error' role='alert'>
          {error}
        </p>
      )}
    </div>
  );
}

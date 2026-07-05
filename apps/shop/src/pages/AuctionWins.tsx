import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { getAuctionWins, createOrderForLot, createPayment } from '@zamk/api-client';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';

export function AuctionWins() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const { showToast } = useToast();
  
  const [wins, setWins] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [processingId, setProcessingId] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      navigate('/');
      return;
    }
    loadWins();
  }, [user]);

  const loadWins = async () => {
    try {
      setLoading(true);
      const data = await getAuctionWins();
      setWins(data || []);
    } catch (err: any) {
      showToast(err.message || 'Ошибка загрузки', 'error');
    } finally {
      setLoading(false);
    }
  };

  const handlePay = async (lotId: string) => {
    try {
      setProcessingId(lotId);
      // 1. Create specialized order
      const orderRes = await createOrderForLot(lotId);
      
      // 2. Request payment link
      const payRes = await createPayment(orderRes.OrderID);
      
      // 3. Redirect to payment gateway
      if (payRes.paymentUrl) {
        window.location.href = payRes.paymentUrl;
      }
    } catch (err: any) {
      showToast(err.message || 'Ошибка при оплате лота', 'error');
      setProcessingId(null);
    }
  };

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-8">
        <h1 className="text-2xl font-bold mb-6 text-[var(--color-text)]">Мои выигранные лоты</h1>
        <div className="animate-pulse space-y-4">
          <div className="h-24 bg-[var(--color-bg-secondary)] rounded-lg"></div>
          <div className="h-24 bg-[var(--color-bg-secondary)] rounded-lg"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold mb-6 text-[var(--color-text)]">Мои выигранные лоты</h1>
      
      {wins.length === 0 ? (
        <div className="text-center py-12 bg-[var(--color-bg-secondary)] rounded-xl">
          <p className="text-[var(--color-text-secondary)]">У вас пока нет выигранных лотов</p>
          <button
            onClick={() => navigate('/auction')}
            className="mt-4 px-6 py-2 bg-[var(--color-accent)] text-white rounded-lg hover:opacity-90"
          >
            Перейти к аукционами
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {wins.map((lot) => (
            <div key={lot.id} className="flex items-center p-4 bg-[var(--color-bg-secondary)] rounded-xl border border-[var(--color-border)]">
              {lot.imageUrl && (
                <img src={lot.imageUrl} alt={lot.title} className="w-20 h-20 object-cover rounded-lg mr-4" />
              )}
              <div className="flex-1">
                <h3 className="font-semibold text-[var(--color-text)]">{lot.title}</h3>
                <div className="text-sm text-[var(--color-text-secondary)] mt-1">
                  Победная ставка: {((lot.currentBidCents || 0) / 100).toLocaleString('ru-RU')} ₽
                </div>
                {lot.status === 'won_pending_payment' && lot.paymentDeadlineAt && (
                  <div className="text-sm text-red-500 mt-1">
                    Оплатить до: {new Date(lot.paymentDeadlineAt).toLocaleString()}
                  </div>
                )}
              </div>
              
              <div className="ml-4 flex flex-col items-end">
                <span className={`px-3 py-1 rounded-full text-xs font-medium mb-2 ${
                  lot.status === 'paid' ? 'bg-green-100 text-green-800' :
                  lot.status === 'won_pending_payment' ? 'bg-yellow-100 text-yellow-800' :
                  lot.status === 'unpaid_manual_review' ? 'bg-red-100 text-red-800' :
                  lot.status === 'moved_to_direct_sale' ? 'bg-gray-100 text-gray-800' :
                  'bg-gray-100 text-gray-800'
                }`}>
                  {lot.status === 'paid' ? 'Оплачен' :
                   lot.status === 'won_pending_payment' ? 'Ожидает оплаты' :
                   lot.status === 'unpaid_manual_review' ? 'Срок оплаты истёк' :
                   lot.status === 'moved_to_direct_sale' ? 'Перенесён в магазин' :
                   lot.status}
                </span>

                {lot.status === 'won_pending_payment' && (
                  <button
                    onClick={() => handlePay(lot.id)}
                    disabled={processingId === lot.id}
                    className="px-4 py-2 bg-[var(--color-accent)] text-white text-sm rounded-lg hover:opacity-90 disabled:opacity-50"
                  >
                    {processingId === lot.id ? 'Обработка...' : 'Оплатить'}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

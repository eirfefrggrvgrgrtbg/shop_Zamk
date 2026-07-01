import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, Gavel, AlertCircle, Info, CheckCircle2 } from 'lucide-react';
import { getAuctionLot } from '@zamk/api-client/src/public';
import { placeBid } from '@zamk/api-client/src/customer';
import type { AuctionLot, AuctionEvent } from '@zamk/api-client';
import { useAuth } from '../contexts/AuthContext';
import { useToast } from '../contexts/ToastContext';
import { formatPrice } from '../lib/utils';
import { Countdown } from '../components/auctions/Countdown';
import { Button } from '../components/ui/Button';
import { useAuctionStream } from '../hooks/useAuctionStream';

export function AuctionLotDetail() {
  const { id } = useParams<{ id: string }>();
  const [lot, setLot] = useState<AuctionLot | null>(null);
  const [auction, setAuction] = useState<AuctionEvent | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isBidding, setIsBidding] = useState(false);
  const [error, setError] = useState('');
  const [bidSuccess, setBidSuccess] = useState(false);
  const [activeImage, setActiveImage] = useState<string>('');

  const { isAuthenticated, openAuthModal } = useAuth();
  const { showToast } = useToast();

  const fetchLotData = async () => {
    if (!id) return;
    try {
      const data = await getAuctionLot(id);
      setLot(data);
      // The backend response for getAuctionLot ideally includes auction details or we need to fetch them.
      // Assuming backend includes `auction` object in `lot.auction` based on standard REST expanding.
      // If not, we might need a separate call. We'll use lot.auction if it exists, otherwise we'll safely handle it.
      if ((data as any).auction) {
        setAuction((data as any).auction);
      }
      
      if (!activeImage && data.images && data.images.length > 0) {
        setActiveImage(data.images[0].imageUrl);
      }
      setError('');
    } catch (err: any) {
      console.error('Failed to load lot:', err);
      // Only set error if we haven't loaded the lot yet to avoid flashing error on background poll fail
      if (!lot) {
        setError(err.message || 'Не удалось загрузить лот.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchLotData();
    
    // Polling every 10 seconds
    const intervalId = setInterval(fetchLotData, 10000);
    return () => clearInterval(intervalId);
  }, [id]);

  const { lastEvent } = useAuctionStream(auction?.id);

  useEffect(() => {
    if (!lastEvent || !lot || !auction) return;

    if (lastEvent.eventType === 'bid_accepted' && lastEvent.lotId === lot.id) {
      setLot(prev => {
        if (!prev) return prev;
        const oldBid = prev.currentBidCents || 0;
        const newBid = lastEvent.currentBidCents !== undefined ? lastEvent.currentBidCents : oldBid;
        const safeBid = newBid < oldBid ? oldBid : newBid;
        return { 
          ...prev, 
          currentBidCents: safeBid, 
          status: (lastEvent.lotStatus as any) || prev.status 
        };
      });
      if (lastEvent.endsAt) {
        setAuction(prev => {
          if (!prev) return prev;
          if (!prev.endsAt || new Date(lastEvent.endsAt as string) > new Date(prev.endsAt)) {
            return { ...prev, endsAt: lastEvent.endsAt as string };
          }
          return prev;
        });
      }
    } else if (lastEvent.eventType === 'lot_extended' && lastEvent.lotId === lot.id) {
      if (lastEvent.endsAt) {
        setAuction(prev => {
          if (!prev) return prev;
          if (!prev.endsAt || new Date(lastEvent.endsAt as string) > new Date(prev.endsAt)) {
            return { ...prev, endsAt: lastEvent.endsAt as string };
          }
          return prev;
        });
      }
    } else if (lastEvent.eventType === 'auction_status_changed') {
      if (lastEvent.auctionStatus) {
        setAuction(prev => prev ? { ...prev, status: lastEvent.auctionStatus as any } : prev);
      }
    } else if (lastEvent.eventType === 'lot_status_changed' && lastEvent.lotId === lot.id) {
      if (lastEvent.lotStatus) {
        setLot(prev => prev ? { ...prev, status: lastEvent.lotStatus as any } : prev);
      }
    }
  }, [lastEvent]);

  const handlePlaceBid = async () => {
    if (!isAuthenticated) {
      showToast('Для участия в торгах необходимо войти в систему.');
      openAuthModal('login');
      return;
    }

    if (!lot || !id) return;

    // Calculate required amount
    const currentPrice = lot.currentBidCents || lot.startPriceCents;
    const bidStep = auction?.bidStepCents || 0;
    const requiredBidAmount = lot.currentBidCents ? currentPrice + bidStep : currentPrice;

    setIsBidding(true);
    setError('');
    setBidSuccess(false);

    try {
      const idempotencyKey = crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).substring(7);
      
      await placeBid(id, {
        amountCents: requiredBidAmount,
        idempotencyKey,
      });
      
      setBidSuccess(true);
      showToast('Ваша ставка успешно принята!');
      
      // Instantly refresh lot data to show new bid
      await fetchLotData();
    } catch (err: any) {
      console.error('Bid failed:', err);
      setError(err.message || 'Не удалось сделать ставку. Возможно, ваша ставка была перебита.');
      // Refresh to get latest state on failure
      fetchLotData();
    } finally {
      setIsBidding(false);
    }
  };

  if (isLoading && !lot) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center">
        <div className="text-gray-500 animate-pulse flex items-center gap-2">
          <Gavel className="w-5 h-5" />
          <span>Загрузка лота...</span>
        </div>
      </div>
    );
  }

  if (error && !lot) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center p-4">
        <div className="text-red-500 flex flex-col items-center gap-3 bg-red-50 dark:bg-red-950/20 p-8 rounded-2xl max-w-md text-center">
          <AlertCircle className="w-8 h-8" />
          <p>{error}</p>
          <Link to="/auction" className="mt-2 text-sm underline hover:text-red-600">
            Вернуться к аукционам
          </Link>
        </div>
      </div>
    );
  }

  if (!lot) return null;

  const currentPrice = lot.currentBidCents || lot.startPriceCents;
  const bidStep = auction?.bidStepCents || 0;
  const nextBidAmount = lot.currentBidCents ? currentPrice + bidStep : currentPrice;
  const isEnded = auction?.status === 'ended' || lot.status === 'ended_no_bids' || lot.status === 'won_pending_payment' || lot.status === 'paid';
  const isLive = lot.status === 'active' && auction?.status === 'live';

  return (
    <div className="max-w-[1400px] mx-auto px-4 sm:px-6 py-8 md:py-12 mt-16 md:mt-20">
      <Link to="/auction" className="inline-flex items-center gap-2 text-sm text-gray-500 hover:text-black dark:hover:text-white transition-colors mb-8">
        <ArrowLeft className="w-4 h-4" />
        К списку аукционов
      </Link>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 xl:gap-16">
        {/* Left Col: Images */}
        <div className="space-y-4">
          <div className="aspect-[4/5] bg-gray-100 dark:bg-zinc-900 rounded-3xl overflow-hidden border border-gray-200 dark:border-white/5 relative">
            {activeImage ? (
              <img src={activeImage} alt={lot.title} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full flex flex-col items-center justify-center text-gray-400 gap-2">
                <Gavel className="w-12 h-12 opacity-20" />
                <span>Фото отсутствует</span>
              </div>
            )}
            
            {/* Status Badges overlay */}
            <div className="absolute top-4 right-4 flex flex-col gap-2 z-10 pointer-events-none">
              {isLive && (
                <span className="inline-flex px-3 py-1.5 bg-primary text-white text-[10px] font-bold uppercase tracking-wider animate-pulse rounded-full shadow-lg">
                  LIVE АУКЦИОН
                </span>
              )}
              {isEnded && (
                <span className="inline-flex px-3 py-1.5 bg-gray-800 text-white text-[10px] font-bold uppercase tracking-wider rounded-full shadow-lg">
                  ТОРГИ ЗАВЕРШЕНЫ
                </span>
              )}
            </div>
          </div>
          
          {/* Thumbnails */}
          {lot.images && lot.images.length > 1 && (
            <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-hide">
              {lot.images.map((img) => (
                <button
                  key={img.id}
                  onClick={() => setActiveImage(img.imageUrl)}
                  className={`w-20 h-24 flex-shrink-0 rounded-xl overflow-hidden border-2 transition-all ${
                    activeImage === img.imageUrl ? 'border-primary' : 'border-transparent opacity-60 hover:opacity-100'
                  }`}
                >
                  <img src={img.imageUrl} alt="" className="w-full h-full object-cover" />
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Right Col: Info & Bidding */}
        <div className="flex flex-col">
          {auction && (
            <div className="flex items-center gap-2 text-xs font-mono uppercase tracking-widest text-gray-500 mb-4">
              <Gavel className="w-3.5 h-3.5" />
              <span>{auction.title}</span>
            </div>
          )}
          
          <h1 className="text-3xl md:text-4xl font-serif font-medium leading-tight mb-4 text-black dark:text-white">
            {lot.title}
          </h1>

          {/* Condition / Attributes */}
          <div className="flex gap-4 mb-8">
            {lot.attributes?.map((attr) => (
              <div key={attr.id} className="bg-gray-100 dark:bg-white/5 px-3 py-1 rounded-md text-xs font-medium text-gray-600 dark:text-gray-300">
                {attr.name}: {attr.value}
              </div>
            ))}
          </div>

          <div className="p-6 md:p-8 bg-white/50 dark:bg-zinc-900/50 backdrop-blur-xl border border-gray-200 dark:border-white/10 rounded-3xl mb-8 shadow-sm">
            
            {error && (
              <div className="mb-6 p-4 text-sm text-red-600 bg-red-50 dark:bg-red-950/30 border border-red-100 dark:border-red-900/50 rounded-2xl flex items-start gap-3">
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
                <span>{error}</span>
              </div>
            )}

            {bidSuccess && (
              <div className="mb-6 p-4 text-sm text-green-700 bg-green-50 dark:bg-green-950/30 border border-green-100 dark:border-green-900/50 rounded-2xl flex items-start gap-3">
                <CheckCircle2 className="w-5 h-5 flex-shrink-0 mt-0.5" />
                <span>Ваша ставка успешно принята! Вы лидируете.</span>
              </div>
            )}

            <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-6 mb-8">
              <div>
                <p className="text-sm text-gray-500 uppercase tracking-widest font-mono mb-2">
                  {lot.currentBidCents ? 'Текущая ставка' : 'Начальная цена'}
                </p>
                <p className="text-4xl md:text-5xl font-medium tracking-tight text-primary dark:text-white">
                  {formatPrice(currentPrice)}
                </p>
              </div>
              
              <div className="flex flex-col items-start md:items-end gap-2 text-sm text-gray-600 dark:text-gray-400">
                {auction && <span>Шаг ставки: <strong className="text-black dark:text-white">{formatPrice(auction.bidStepCents)}</strong></span>}
              </div>
            </div>

            {isLive && auction ? (
              <div className="space-y-4">
                <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-white/5 rounded-2xl">
                  <span className="text-sm font-medium">Окончание торгов:</span>
                  <Countdown endsAt={auction.endsAt} className="text-lg" />
                </div>
                
                <Button 
                  onClick={handlePlaceBid} 
                  disabled={isBidding}
                  className="w-full py-5 text-base font-bold tracking-widest uppercase"
                >
                  {isBidding ? 'Обработка...' : `Сделать ставку (${formatPrice(nextBidAmount)})`}
                </Button>
                
                <p className="text-xs text-center text-gray-500 flex items-center justify-center gap-1.5 mt-2">
                  <Info className="w-3.5 h-3.5" />
                  Размещая ставку, вы обязуетесь выкупить лот в случае победы.
                </p>
              </div>
            ) : (
              <div className="p-4 bg-gray-100 dark:bg-white/5 rounded-2xl text-center font-medium text-gray-600 dark:text-gray-300">
                {isEnded ? 'Аукцион завершен' : 'Ожидает начала торгов'}
              </div>
            )}
          </div>

          {/* Description */}
          <div className="prose prose-sm dark:prose-invert max-w-none text-gray-600 dark:text-gray-300">
            <h3 className="text-lg font-medium text-black dark:text-white mb-3">Описание лота</h3>
            {lot.description ? (
              <div className="whitespace-pre-wrap">{lot.description}</div>
            ) : (
              <p className="italic">Описание отсутствует.</p>
            )}
          </div>
          
          {/* Rules info block */}
          <div className="mt-8 p-5 rounded-2xl bg-gray-50 dark:bg-zinc-900 border border-gray-100 dark:border-zinc-800 text-sm text-gray-600 dark:text-gray-400">
            <h4 className="font-medium text-black dark:text-white mb-2 flex items-center gap-2">
              <Gavel className="w-4 h-4" />
              Правила проведения аукциона
            </h4>
            <ul className="list-disc pl-5 space-y-1 mt-3">
              <li>Шаг аукциона фиксирован. Ваша ставка автоматически рассчитывается как "Текущая цена + Шаг".</li>
              {auction?.antiSnipingEnabled && (
                <li>Включена защита от снайпинга: ставки на последних секундах продлевают время торгов.</li>
              )}
              <li>Победитель обязан оплатить лот в течение {auction?.paymentDeadlineHours || 24} часов.</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

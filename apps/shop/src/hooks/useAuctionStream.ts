import { useEffect, useState } from 'react';

export interface AuctionRealtimeEvent {
  eventType: string;
  auctionId: string;
  lotId?: string;
  bidId?: string;
  currentBidCents?: number;
  auctionStatus?: string;
  lotStatus?: string;
  endsAt?: string;
}

export function useAuctionStream(auctionId: string | undefined) {
  const [lastEvent, setLastEvent] = useState<AuctionRealtimeEvent | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  useEffect(() => {
    if (!auctionId) return;

    let sse: EventSource | null = null;
    let retryTimeout: ReturnType<typeof setTimeout>;

    const connect = () => {
      // Use absolute URL since frontend is on 3000 and backend on 8080
      const baseUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';
      sse = new EventSource(`${baseUrl}/public/auctions/${auctionId}/stream`);

      sse.onopen = () => {
        setIsConnected(true);
      };

      sse.onerror = () => {
        setIsConnected(false);
        if (sse) {
          sse.close();
        }
        // Reconnect after 3 seconds
        retryTimeout = setTimeout(connect, 3000);
      };

      // Listen for specific event types we broadcasted
      const eventTypes = [
        'auction_status_changed',
        'lot_status_changed',
        'bid_accepted',
        'lot_extended',
      ];

      eventTypes.forEach(eventType => {
        sse?.addEventListener(eventType, (e: MessageEvent) => {
          try {
            const data = JSON.parse(e.data) as AuctionRealtimeEvent;
            setLastEvent(data);
          } catch (err) {
            console.error('Failed to parse SSE event data', err);
          }
        });
      });
      
      // Hearbeat
      sse.addEventListener('ping', () => {
        // Just keeping connection alive
      });
    };

    connect();

    return () => {
      if (retryTimeout) clearTimeout(retryTimeout);
      if (sse) {
        sse.close();
      }
      setIsConnected(false);
    };
  }, [auctionId]);

  return { lastEvent, isConnected };
}

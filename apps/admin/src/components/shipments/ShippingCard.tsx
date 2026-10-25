import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import type { DispatchContext } from '../../api/adminPicking';
import { formatOrderNumber } from '../../utils/orderFormatters';

// FBO.SHIPPING.DOCS.1: the internal shipping card of one parcel, printed from the dispatch screen. It is built only
// from the dispatch-context already loaded for that parcel: nothing is invented (no tracking number, no waybill) and
// nothing is stored or sent anywhere. It lives in a portal that exists only while the screen shows a successfully
// loaded context, and is shown only by the print stylesheet (styles/index.css: @media print).

export const SHIPPING_CARD_ID = 'zamk-shipping-card';
/** Set on <html> only while a card is mounted: the print stylesheet hides the Admin app only then. */
export const PRINT_CARD_HTML_CLASS = 'zamk-print-card';
export const SHIPPING_CARD_DISCLAIMER = 'Внутренний документ ZAMK. Не является транспортной накладной или этикеткой перевозчика.';

function Row({ label, children, big }: { label: string; children: React.ReactNode; big?: boolean }) {
  return (
    <div className="sc-row">
      <div className="sc-label">{label}</div>
      <div className={big ? 'sc-value sc-value-big' : 'sc-value'}>{children}</div>
    </div>
  );
}

export function ShippingCard({ ctx }: { ctx: DispatchContext }) {
  useEffect(() => {
    const html = document.documentElement;
    html.classList.add(PRINT_CARD_HTML_CLASS);
    return () => html.classList.remove(PRINT_CARD_HTML_CLASS);
  }, []);

  const recipient = ctx.recipientName || ctx.customerName || '—';
  const phone = ctx.recipientPhone || ctx.customerPhone || '—';
  const parcel = ctx.parcelNumber && ctx.parcelsTotal ? `${ctx.parcelNumber} из ${ctx.parcelsTotal}` : '—';
  const printedAt = new Date().toLocaleString('ru-RU');

  return createPortal(
    <div id={SHIPPING_CARD_ID} data-testid="shipping-card" data-fulfillment-id={ctx.fulfillmentId}>
      <header className="sc-header">
        <div className="sc-brand">ZAMK</div>
        <div className="sc-title">Внутренняя карточка отправления</div>
      </header>

      <section className="sc-block">
        <Row label="Заказ"><span data-testid="sc-order">{formatOrderNumber({ id: ctx.orderId, orderNumber: ctx.orderNumber })}</span></Row>
        <Row label="Посылка"><span data-testid="sc-parcel">{parcel}</span></Row>
        <Row label="Отправление">
          <span data-testid="sc-shipment">{ctx.shipmentId ?? 'ещё не создано'}</span>
        </Row>
      </section>

      <section className="sc-block">
        <Row label="Получатель" big><span data-testid="sc-recipient">{recipient}</span></Row>
        <Row label="Телефон" big><span data-testid="sc-phone">{phone}</span></Row>
        <Row label="Адрес доставки" big><span data-testid="sc-address">{ctx.deliveryAddress || 'не указан'}</span></Row>
        {ctx.deliveryPostalCode && <Row label="Индекс"><span data-testid="sc-postal">{ctx.deliveryPostalCode}</span></Row>}
        {ctx.deliveryCourierComment && <Row label="Комментарий курьеру"><span data-testid="sc-comment">{ctx.deliveryCourierComment}</span></Row>}
        {ctx.deliveryMethodName && <Row label="Способ доставки"><span data-testid="sc-method">{ctx.deliveryMethodName}</span></Row>}
      </section>

      {(ctx.carrier || ctx.trackingNumber) && (
        <section className="sc-block">
          {ctx.carrier && <Row label="Служба доставки"><span data-testid="sc-carrier">{ctx.carrier}</span></Row>}
          {ctx.trackingNumber && <Row label="Трек-номер"><span data-testid="sc-tracking">{ctx.trackingNumber}</span></Row>}
        </section>
      )}

      <footer className="sc-footer">
        <p data-testid="sc-disclaimer" className="sc-disclaimer">{SHIPPING_CARD_DISCLAIMER}</p>
        <p>Содержит персональные данные получателя: не наклеивать на посылку, хранить и утилизировать как конфиденциальный документ.</p>
        <p>Напечатано: {printedAt}</p>
      </footer>
    </div>,
    document.body,
  );
}

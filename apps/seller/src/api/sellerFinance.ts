import { SellerBalance, PayoutBatchListResponse, LedgerListResponse } from '@zamk/api-client/src/types';

export function adaptBalance(balance: SellerBalance) {
  return {
    grossSales: balance.grossSalesCents / 100,
    commission: balance.commissionCents / 100,
    adjustments: balance.adjustmentsCents / 100,
    frozen: balance.frozenCents / 100,
    available: balance.availableCents / 100,
    paid: balance.paidCents / 100,
    currency: balance.currency || 'RUB',
    nextPayoutAt: balance.nextPayoutAt ? new Date(balance.nextPayoutAt).toLocaleDateString() : undefined,
  };
}

export function adaptPayoutBatches(payouts: PayoutBatchListResponse) {
  return payouts.items.map(p => ({
    id: p.id,
    amount: p.amountCents / 100,
    status: p.status, // scheduled, paid, held
    scheduledFor: new Date(p.scheduledFor).toLocaleDateString(),
    processedAt: p.processedAt ? new Date(p.processedAt).toLocaleDateString() : undefined,
    failureReason: p.failureReason,
  }));
}

export function adaptLedger(ledger: LedgerListResponse) {
  return ledger.items.map(l => ({
    id: l.id,
    type: l.type,
    amount: l.amountCents / 100,
    currency: l.currency,
    availableAt: l.availableAt ? new Date(l.availableAt).toLocaleDateString() : undefined,
    createdAt: new Date(l.createdAt).toLocaleDateString(),
    orderId: l.orderId,
  }));
}

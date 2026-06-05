import { Payout, SellerBalance } from '@zamk/api-client/src/types';

export function adaptBalance(balance: SellerBalance) {
  return {
    availableBalance: balance.availableBalanceCents / 100,
    pendingBalance: balance.pendingBalanceCents / 100,
    requestedPayouts: (balance.requestedPayoutsCents || 0) / 100,
    paidPayouts: (balance.paidPayoutsCents || 0) / 100,
    currency: balance.currency || 'RUB',
  };
}

export function adaptPayouts(payouts: Payout[]) {
  return payouts.map(p => ({
    id: p.id,
    amount: p.amountCents / 100,
    status: p.status, // requested, approved, rejected, paid, cancelled
    requestedAt: new Date(p.requestedAt).toLocaleDateString(),
    approvedAt: p.approvedAt ? new Date(p.approvedAt).toLocaleDateString() : undefined,
    rejectedAt: p.rejectedAt ? new Date(p.rejectedAt).toLocaleDateString() : undefined,
    paidAt: p.paidAt ? new Date(p.paidAt).toLocaleDateString() : undefined,
    comment: p.comment,
  }));
}

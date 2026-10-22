// CATALOG.C5.3 — Studio autosave of a published product keeps it on sale (continueSelling contract).
import { describe, it, expect, vi } from 'vitest';
import type { ProductStudioDraft } from '../../contexts/ProductStudioContext';
import {
  buildProductStudioUpdateRequest,
  orchestrateProductStudioEditSave,
  shouldContinueSelling,
} from './productStudioSaveProduct';

const draft = (status: string | undefined, extra: Partial<ProductStudioDraft> = {}): ProductStudioDraft =>
  ({ id: 'prod-1', title: 'Платье', description: 'Описание', status, images: [], variants: [], ...extra }) as ProductStudioDraft;

describe('shouldContinueSelling', () => {
  it('is true only for a published product (server status, any case)', () => {
    expect(shouldContinueSelling({ status: 'published' })).toBe(true);
    expect(shouldContinueSelling({ status: 'PUBLISHED' })).toBe(true);
  });

  it.each(['draft', 'pending_moderation', 'in_review', 'rejected', 'approved', 'hidden', 'blocked', 'archived', 'out_of_stock', '', undefined])(
    'is false for %s: continueSelling never puts a product on sale',
    (status) => {
      expect(shouldContinueSelling({ status })).toBe(false);
    },
  );
});

describe('buildProductStudioUpdateRequest', () => {
  it('a published product is edited with continueSelling=true', () => {
    const p = buildProductStudioUpdateRequest(draft('published', { description: 'Новое' }), draft('published'));
    expect(p.continueSelling).toBe(true);
    expect(p.description).toBe('Новое');
  });

  it('the decision follows the server baseline, not the local draft', () => {
    expect(buildProductStudioUpdateRequest(draft('published'), draft('draft')).continueSelling).toBeUndefined();
    expect(buildProductStudioUpdateRequest(draft('draft'), draft('published')).continueSelling).toBe(true);
  });

  it.each(['draft', 'rejected', 'archived', 'blocked', 'hidden'])('a %s product is not sent continueSelling', (status) => {
    expect(buildProductStudioUpdateRequest(draft(status), draft(status))).not.toHaveProperty('continueSelling');
  });
});

describe('orchestrateProductStudioEditSave (published product)', () => {
  const run = (baseline: ProductStudioDraft, working: ProductStudioDraft, updateProductFn: (id: string, input: any) => Promise<any>) => {
    const onError = vi.fn();
    const onRefreshError = vi.fn();
    return orchestrateProductStudioEditSave({
      productId: 'prod-1',
      draft: working,
      baselineDraft: baseline,
      categorySchema: null,
      canonicalColors: [],
      dictionaryValuesMap: {},
      stageImageFn: vi.fn(),
      updateProductFn,
      getProductFn: vi.fn().mockRejectedValue(new Error('refresh not needed here')),
      cropImageFn: vi.fn(),
      onStagingStart: vi.fn(),
      onStagedImagesPersisted: vi.fn(),
      onSavingStart: vi.fn(),
      onSaveSuccess: vi.fn(),
      onError,
      onRefreshError,
    }).then(() => ({ onError, onRefreshError }));
  };

  it('every autosave (and the first save after a Studio reload) sends exactly one PATCH with continueSelling=true', async () => {
    const update = vi.fn().mockResolvedValue({});
    await run(draft('published'), draft('published', { description: 'v2' }), update);
    await run(draft('published'), draft('published', { description: 'v3' }), update);
    // reload: the baseline comes from GET again; the product is still published with a pending revision
    await run(draft('published', { hasPendingRevision: true, description: 'v3' }), draft('published', { hasPendingRevision: true, description: 'v4' }), update);
    expect(update).toHaveBeenCalledTimes(3);
    for (const [id, body] of update.mock.calls) {
      expect(id).toBe('prod-1');
      expect(body.continueSelling).toBe(true);
    }
    expect(update.mock.calls.map(([, b]) => b.description)).toEqual(['v2', 'v3', 'v4']);
  });

  it('a refused PATCH reports the error and is not retried behind the seller', async () => {
    const update = vi.fn().mockRejectedValue(Object.assign(new Error('x'), { code: 'variant_deactivation_blocked', rawMessage: 'Есть резерв' }));
    const { onError } = await run(draft('published'), draft('published', { description: 'v2' }), update);
    expect(update).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith('Есть резерв');
  });

  it('a draft product keeps the ordinary draft save (no continueSelling)', async () => {
    const update = vi.fn().mockResolvedValue({});
    await run(draft('draft'), draft('draft', { description: 'v2' }), update);
    expect(update.mock.calls[0][1]).not.toHaveProperty('continueSelling');
  });
});

import type { QueryRequest, QueryFilter, QuerySort } from '@zamk/api-client/src/types';

export interface NormalizedQueryRequest {
  version: number;
  period: {
    from: string;
    to: string;
  };
  dimensions: string[];
  metrics: string[];
  filters: QueryFilter[];
  sort: QuerySort[];
  limit: number;
}

export function normalizeQueryRequest(req: QueryRequest): NormalizedQueryRequest {
  const fromIso = typeof req.period?.from === 'string'
    ? new Date(req.period.from).toISOString()
    : new Date(req.period?.from ?? 0).toISOString();
  const toIso = typeof req.period?.to === 'string'
    ? new Date(req.period.to).toISOString()
    : new Date(req.period?.to ?? 0).toISOString();

  const filters = (req.filters || [])
    .filter(f => f && Boolean(f.dimension) && (f.values?.length ?? 0) > 0)
    .map(f => ({
      dimension: f.dimension,
      operator: f.operator,
      values: [...f.values],
    }));

  const sort = (req.sort || [])
    .filter(s => s && Boolean(s.field))
    .map(s => ({
      field: s.field,
      direction: s.direction,
    }));

  return {
    version: req.version ?? 1,
    period: {
      from: fromIso,
      to: toIso,
    },
    dimensions: req.dimensions ? [...req.dimensions] : [],
    metrics: req.metrics ? [...req.metrics] : [],
    filters,
    sort,
    limit: req.limit ?? 100,
  };
}

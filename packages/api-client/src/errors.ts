export class ApiError extends Error {
  public code?: string;
  public status?: number;
  public data?: any;
  public rawMessage?: string;

  constructor(message: string, code?: string, status?: number, data?: any, rawMessage?: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.data = data;
    this.rawMessage = rawMessage ?? (typeof data?.error === 'object' ? data?.error?.message : (typeof data?.message === 'string' ? data.message : undefined));
  }
}

export const isInsufficientStockError = (error: unknown): boolean => {
  if (!error) return false;
  if (error instanceof ApiError || (typeof error === 'object' && error !== null && 'name' in error && (error as any).name === 'ApiError')) {
    const apiErr = error as ApiError;
    if (apiErr.code === 'insufficient_stock') return true;
    if (apiErr.code === 'invalid_item') {
      const raw = apiErr.rawMessage || apiErr.data?.error?.message;
      if (raw === 'insufficient stock') return true;
    }
  }
  const anyErr = error as any;
  if (anyErr?.code === 'insufficient_stock') return true;
  if (anyErr?.code === 'invalid_item') {
    const raw = anyErr?.rawMessage || anyErr?.data?.error?.message;
    if (raw === 'insufficient stock') return true;
  }
  return false;
};

export class ApiError extends Error {
  public code?: string;
  public status?: number;
  public data?: any;

  constructor(message: string, code?: string, status?: number, data?: any) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.data = data;
  }
}

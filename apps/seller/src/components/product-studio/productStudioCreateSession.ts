export const PRODUCT_STUDIO_CREATE_SESSION_KEY = 'zamk:product-studio:create-session:v1';

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export interface ProductStudioCreateRequestPayload {
  title: string;
  slug?: string;
  description?: string;
  categoryId?: string;
  brandId?: string;
  gender?: string;
  color?: string;
  material?: string;
  careInstructions?: string;
  priceCents: number;
  oldPriceCents?: number;
  currency: string;
  variants?: Array<{
    colorId?: string;
    sizeValueId?: string;
    sellerSku?: string;
    priceCents?: number;
  }>;
  materialComposition?: Array<{
    materialId: string;
    percentage: number;
  }>;
  sizeChartRows?: Array<{
    sizeValueId: string;
    measurements: Record<string, any>;
  }>;
  attributes?: Array<{
    attributeDefinitionId: string;
    enumValueId?: string;
    textValue?: string;
    numberValue?: number;
    boolValue?: boolean;
  }>;
}

export interface ProductStudioCreateSessionRecord {
  version: 1;
  clientCreateId: string;
  createRequestSnapshot: ProductStudioCreateRequestPayload;
  productId?: string;
  phase: 'identity_pending' | 'identity_established' | 'completed';
  createdAt: number;
}

export function validateProductStudioCreateRequestSnapshot(snapshot: any): snapshot is ProductStudioCreateRequestPayload {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) return false;
  if (typeof snapshot.title !== 'string' || !snapshot.title.trim()) return false;
  if (typeof snapshot.priceCents !== 'number' || !Number.isFinite(snapshot.priceCents) || snapshot.priceCents < 0) return false;
  if (typeof snapshot.currency !== 'string' || !snapshot.currency.trim()) return false;
  return true;
}

/**
 * Conservative runtime session validation:
 * Rejects non-objects, wrong versions, malformed clientCreateIds (must be UUID),
 * invalid createdAt timestamps, unsupported phases, or missing required fields.
 */
export function loadProductStudioCreateSession(): ProductStudioCreateSessionRecord | null {
  try {
    const raw = sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY);
    if (!raw || !raw.trim()) return null;
    let parsed: any;
    try {
      parsed = JSON.parse(raw);
    } catch {
      return null;
    }
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;
    if (parsed.version !== 1) return null;
    if (typeof parsed.clientCreateId !== 'string' || !UUID_REGEX.test(parsed.clientCreateId.trim())) return null;
    if (typeof parsed.createdAt !== 'number' || !Number.isFinite(parsed.createdAt)) return null;
    if (
      parsed.phase !== 'identity_pending' &&
      parsed.phase !== 'identity_established' &&
      parsed.phase !== 'completed'
    ) {
      return null;
    }

    if (parsed.phase === 'identity_pending') {
      if (!validateProductStudioCreateRequestSnapshot(parsed.createRequestSnapshot)) {
        return null;
      }
    } else if (parsed.phase === 'identity_established') {
      if (typeof parsed.productId !== 'string' || !parsed.productId.trim()) {
        return null;
      }
      if (!validateProductStudioCreateRequestSnapshot(parsed.createRequestSnapshot)) {
        return null;
      }
    } else if (parsed.phase === 'completed') {
      if (typeof parsed.productId !== 'string' || !parsed.productId.trim()) {
        return null;
      }
    }

    return parsed as ProductStudioCreateSessionRecord;
  } catch {
    return null;
  }
}

/**
 * Returns true if raw data exists in sessionStorage under create-session key
 * but fails conservative validation.
 */
export function isProductStudioCreateSessionMalformed(): boolean {
  try {
    const raw = sessionStorage.getItem(PRODUCT_STUDIO_CREATE_SESSION_KEY);
    if (!raw || !raw.trim()) return false;
    return loadProductStudioCreateSession() === null;
  } catch {
    return true;
  }
}

/**
 * Fails closed: either successfully writes to sessionStorage or throws an explicit Error.
 * Automatically injects version 1.
 */
export function saveProductStudioCreateSession(
  record: Omit<ProductStudioCreateSessionRecord, 'version'> & { version?: 1 }
): void {
  try {
    const fullRecord: ProductStudioCreateSessionRecord = {
      ...record,
      version: 1,
    };
    sessionStorage.setItem(PRODUCT_STUDIO_CREATE_SESSION_KEY, JSON.stringify(fullRecord));
  } catch (err: any) {
    throw new Error(err?.message || 'Не удалось сохранить сессию создания товара в локальном хранилище браузера.');
  }
}

/**
 * Safely attempts to clear the create session.
 * Exposes failure to caller and overwrites key if removeItem fails.
 */
export function clearProductStudioCreateSession(): boolean {
  try {
    sessionStorage.removeItem(PRODUCT_STUDIO_CREATE_SESSION_KEY);
    return true;
  } catch (err) {
    console.warn('Failed to clear product studio create session from sessionStorage:', err);
    return false;
  }
}

/**
 * Guard for "Добавить товар" navigation:
 * If an active unresolved session exists (identity_pending or identity_established),
 * do NOT clear it so recovery can resolve the outcome safely.
 * If completed or malformed or empty, safely clears before starting a new product.
 */
export function prepareAddProductNavigation(): boolean {
  const session = loadProductStudioCreateSession();
  if (session?.phase === 'identity_pending' || session?.phase === 'identity_established') {
    return true;
  }
  return clearProductStudioCreateSession();
}

export function generateClientCreateId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

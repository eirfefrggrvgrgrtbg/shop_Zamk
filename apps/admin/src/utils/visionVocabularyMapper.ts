/**
 * Mapping layer for Vision tokens, advisory wording, and presentation labels.
 * Enforces ZAMK UX rules:
 * - Advisory language only (never final decision).
 * - Human-readable Russian labels for all tokens.
 * - Strip internal "vision.*" prefixes and fallback gracefully on unknown tokens.
 * - Parity with canonical vocabularies.
 */

const KNOWN_TOKENS_RU: Record<string, string> = {
  // Silhouettes
  regular: 'Прямой / стандартный',
  oversized: 'Оверсайз',
  fitted: 'Приталенный',
  relaxed: 'Свободный',
  boxy: 'Квадратный (boxy)',
  straight: 'Прямой',
  a_line: 'А-силуэт',
  fit_and_flare: 'Приталенный с расклешением',
  asymmetrical: 'Асимметричный',
  cocoon: 'Кокон',

  // Patterns
  solid: 'Однотонный',
  striped: 'В полоску',
  plaid: 'В клетку',
  floral: 'Цветочный',
  graphic: 'Графический принт',
  animal: 'Анималистичный принт',
  abstract: 'Абстрактный узор',
  polka_dot: 'В горошек',
  geometric: 'Геометрический',
  tie_dye: 'Тай-дай',
  camo: 'Камуфляж',

  // Textures
  smooth: 'Гладкая',
  knitted: 'Вязаная / трикотаж',
  woven: 'Тканая',
  leather_like: 'Под кожу',
  fur_like: 'Под мех',
  denim: 'Деним',
  fuzzy: 'Фактурная / ворсистая',
  ribbed: 'В рубчик',
  quilted: 'Стёганая',
  sheer: 'Полупрозрачная',
  satin: 'Атласная / шелковистая',
  velvet: 'Бархатистая',
  corduroy: 'Вельвет',

  // Quality Issues
  blur: 'Размытое изображение',
  watermark_detected: 'Обнаружен водяной знак',
  poor_lighting: 'Плохое освещение',
  inappropriate_content: 'Недопустимый контент',
  low_resolution: 'Низкое разрешение',
  overexposed: 'Переэкспонировано',
  underexposed: 'Недоэкспонировано',
  cluttered_background: 'Засорённый фон',
  truncated_product: 'Товар обрезан в кадре',

  // Fit
  slim: 'Облегающий',
  loose: 'Свободный',
  tight: 'В обтяжку',

  // Length
  cropped: 'Укороченная',
  waist: 'До талии',
  hip: 'До бёдер',
  knee: 'До колена',
  midi: 'Миди',
  maxi: 'Макси',
  mini: 'Мини',
  floor: 'В пол',
  ankle: 'По щиколотку',

  // Sleeves
  sleeveless: 'Без рукавов',
  short: 'Короткий',
  long: 'Длинный',
  three_quarter: 'Три четверти',
  cap: 'Крылышко',
  raglan: 'Реглан',
  puff: 'Фонарик / пышный',
  batwing: 'Летучая мышь',

  // Neckline
  round: 'Круглый',
  crew: 'Круглый (crew)',
  v_neck: 'V-образный',
  boat: 'Лодочка',
  turtleneck: 'Высокий воротник (гольф)',
  collared: 'С воротником',
  hooded: 'С капюшоном',
  sweetheart: 'Сердечко',
  square: 'Квадратный',
  off_shoulder: 'С открытыми плечами',
  polo: 'Поло',
  cowl: 'Хомут',

  // Closure
  zipper: 'Молния',
  buttons: 'Пуговицы',
  snap: 'Кнопки',
  pullover: 'Без застёжки (через голову)',
  tie: 'Завязки',
  wrap: 'На запах',
  hooks: 'Крючки',
  none: 'Без застёжки',
  elastic: 'На резинке',

  // Formality
  casual: 'Повседневный',
  smart_casual: 'Смарт-кэжуал',
  formal: 'Официальный',
  evening: 'Вечерний',
  sport: 'Спортивный',
  business: 'Деловой',
  lounge: 'Домашний / отдых',

  // Seasons
  summer: 'Лето',
  winter: 'Зима',
  spring: 'Весна',
  autumn: 'Осень',
  spring_autumn: 'Демисезон',
  demiseason: 'Демисезон',
  all_season: 'Всесезонный',

  // Styles
  minimalist: 'Минимализм',
  streetwear: 'Стритвир',
  classic: 'Классика',
  vintage: 'Винтаж',
  romantic: 'Романтический',
  boho: 'Бохо',
  grunge: 'Гранж',
  sporty: 'Спортивный',
  preppy: 'Преппи',
  military: 'Милитари',
  retro: 'Ретро',
  avant_garde: 'Авангард',

  // Decorative details
  pockets: 'Карманы',
  hood: 'Капюшон',
  belt: 'Пояс / ремень',
  embroidery: 'Вышивка',
  ruffles: 'Оборки / рюши',
  slit: 'Разрез',
  drawstring: 'Кулиска / шнурок',
  fringes: 'Бахрома',
  sequins: 'Пайетки',
  studs: 'Заклёпки',
  lace: 'Кружево',
  pleats: 'Плиссе / складки',
};

const CONTRADICTION_FIELDS_RU: Record<string, string> = {
  category: 'Категория',
  dominantColors: 'Основные цвета',
  secondaryColors: 'Дополнительные цвета',
  silhouette: 'Силуэт',
  pattern: 'Узор',
  texture: 'Фактура',
  details: 'Детали',
  ocrText: 'Текст на фото',
  brand: 'Бренд',
  color: 'Цвет',
  gender: 'Пол',
  fit: 'Посадка',
  fitAppearance: 'Посадка',
  length: 'Длина',
  sleeves: 'Рукава',
  sleeve: 'Рукава',
  neckline: 'Вырез горловины',
  closure: 'Застёжка',
  style: 'Стиль',
  styleDescriptors: 'Стиль',
  formality: 'Формальность',
  season: 'Сезонность',
  seasonalCues: 'Сезонность',
  decorativeDetails: 'Детали',
  visibleTexture: 'Фактура',
};

const FAILURE_CODES_RU: Record<string, string> = {
  provider_transient: 'Временный сбой провайдера AI',
  provider_permanent: 'Недоступность провайдера AI',
  storage_transient: 'Ошибка чтения хранилища фото',
  media_missing: 'Изображения товара не найдены в хранилище',
  media_invalid: 'Некорректный формат изображений',
  contract_mismatch: 'Несовпадение формата контракта',
  invalid_model_output: 'Некорректный ответ модели AI',
  internal_error: 'Внутренняя ошибка обработки',
};

/**
 * Checks whether a given string is a canonical UUID.
 */
export function isUuid(value?: string | null): boolean {
  if (!value || typeof value !== 'string') return false;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value.trim());
}

/**
 * Strips vision.* prefixes and returns readable Russian label.
 * Never throws, never displays raw "vision.*", never returns raw UUIDs.
 */
export function formatVisionToken(token?: string | null): string {
  if (!token || typeof token !== 'string') return '';

  const clean = token.trim();
  if (!clean || isUuid(clean)) return '';

  // Extract suffix if token has dot notation e.g. "vision.silhouette.oversized" -> "oversized"
  const suffix = clean.includes('.') ? clean.split('.').pop() || clean : clean;
  const normalizedKey = suffix.toLowerCase().trim();

  // If suffix is literally "vision" or empty or a UUID, return empty
  if (!normalizedKey || normalizedKey === 'vision' || isUuid(normalizedKey)) return '';

  if (KNOWN_TOKENS_RU[normalizedKey]) {
    return KNOWN_TOKENS_RU[normalizedKey];
  }

  // Also check full string lowercase
  if (KNOWN_TOKENS_RU[clean.toLowerCase()]) {
    return KNOWN_TOKENS_RU[clean.toLowerCase()];
  }

  // Safe fallback: replace underscores with spaces, capitalize
  const humanized = suffix.replace(/_/g, ' ');
  return humanized.charAt(0).toUpperCase() + humanized.slice(1);
}

/**
 * Returns Russian label for contradiction field.
 */
export function formatContradictionField(field?: string | null): string {
  if (!field) return 'Поле';
  const key = field.trim();
  if (CONTRADICTION_FIELDS_RU[key]) {
    return CONTRADICTION_FIELDS_RU[key];
  }
  const lower = key.toLowerCase();
  for (const [k, v] of Object.entries(CONTRADICTION_FIELDS_RU)) {
    if (k.toLowerCase() === lower) {
      return v;
    }
  }
  return formatVisionToken(key) || key;
}

export interface FormatEvidenceOptions {
  declaredCategory?: string | null;
  colors?: Array<{ id?: string; code?: string; name?: string }>;
}

/**
 * Formats evidence field and value into a safe, human-readable Russian presentation string.
 * Strictly guarantees:
 * - Never returns raw UUIDs.
 * - Never returns raw vision.* tokens.
 * - For category: "Категория — <declaredCategory>" or "Категория".
 * - For colors: resolves against available color DTOs, or renders field label only.
 * - For vocabulary tokens: maps via KNOWN_TOKENS_RU or safe humanized fallback.
 * - For unknown technical values / UUIDs: falls back safely to field label only.
 */
export function formatEvidenceLabel(
  evidence: { field?: string | null; valueId?: string | null },
  options?: FormatEvidenceOptions
): string {
  const fieldKey = (evidence.field || '').trim();
  const fieldLabel = formatContradictionField(fieldKey);
  const rawValue = (evidence.valueId || '').trim();

  // 1. Category evidence
  if (fieldKey.toLowerCase() === 'category') {
    const declared = options?.declaredCategory?.trim();
    if (declared && !isUuid(declared)) {
      return `${fieldLabel} — ${declared}`;
    }
    return fieldLabel;
  }

  // 2. Color evidence (dominantColors, secondaryColors, color, colors)
  const isColorField =
    fieldKey.toLowerCase() === 'dominantcolors' ||
    fieldKey.toLowerCase() === 'secondarycolors' ||
    fieldKey.toLowerCase() === 'color' ||
    fieldKey.toLowerCase() === 'colors';

  if (isColorField) {
    if (options?.colors && options.colors.length > 0) {
      const match = options.colors.find(
        (c) => (c.id && c.id === rawValue) || (c.code && c.code === rawValue)
      );
      if (match && match.name && match.code !== 'unresolved' && match.name !== 'Неизвестный цвет') {
        return `${fieldLabel} — ${match.name}`;
      }
    }
    if (rawValue && !isUuid(rawValue)) {
      const formatted = formatVisionToken(rawValue);
      if (formatted) {
        return `${fieldLabel} — ${formatted}`;
      }
    }
    return fieldLabel;
  }

  // 3. UUID value protection for any other field: never leak raw UUID
  if (isUuid(rawValue)) {
    return fieldLabel;
  }

  // 4. Value with Vision tokens or text
  if (rawValue) {
    const formattedToken = formatVisionToken(rawValue);
    if (formattedToken) {
      return `${fieldLabel} — ${formattedToken}`;
    }
    return fieldLabel;
  }

  return fieldLabel;
}

/**
 * Maps CategoryConsistency enum to Russian advisory label and variant.
 */
export function formatCategoryConsistency(consistency?: string | null): {
  label: string;
  variant: 'consistent' | 'contradictory' | 'uncertain' | 'none';
  hint: string;
} {
  switch (consistency) {
    case 'consistent':
      return {
        label: 'Категория соответствует изображениям',
        variant: 'consistent',
        hint: 'Визуальные признаки на фотографиях совпадают с заявленной категорией.',
      };
    case 'contradictory':
      return {
        label: 'Возможное несоответствие категории',
        variant: 'contradictory',
        hint: 'На фотографиях распознаны признаки другой категории товара.',
      };
    case 'uncertain':
      return {
        label: 'Недостаточно данных для проверки категории',
        variant: 'uncertain',
        hint: 'AI не может однозначно подтвердить или опровергнуть категорию по предоставленным фото.',
      };
    default:
      return {
        label: 'Проверка категории не выполнялась',
        variant: 'none',
        hint: '',
      };
  }
}

/**
 * Maps ModerationRecommendation to advisory wording.
 */
export function formatModerationRecommendation(recommendation?: string | null): {
  label: string;
  variant: 'clear' | 'review_recommended' | 'uncertain' | 'none';
  advisoryText: string;
} {
  switch (recommendation) {
    case 'clear':
      return {
        label: 'Явных проблем не обнаружено',
        variant: 'clear',
        advisoryText: 'ИИ не выявил критических противоречий или дефектов качества.',
      };
    case 'review_recommended':
      return {
        label: 'Рекомендуется дополнительная проверка',
        variant: 'review_recommended',
        advisoryText: 'Обнаружены возможные несоответствия или особенности, требующие внимания модератора.',
      };
    case 'uncertain':
      return {
        label: 'Недостаточно данных для уверенной оценки',
        variant: 'uncertain',
        advisoryText: 'ИИ рекомендует проверить карточку вручную из-за низкой уверенности модели.',
      };
    default:
      return {
        label: 'Рекомендация отсутствует',
        variant: 'none',
        advisoryText: '',
      };
  }
}

/**
 * Maps Vision Status to advisory header status and message.
 */
export function formatVisionStatus(status?: string | null): {
  title: string;
  description: string;
  variant: 'not_scheduled' | 'pending' | 'processing' | 'succeeded' | 'failed' | 'stale';
} {
  switch (status) {
    case 'not_scheduled':
      return {
        title: 'AI-анализ ещё не запускался',
        description: 'Для данного товара анализ изображений ещё не проводился.',
        variant: 'not_scheduled',
      };
    case 'pending':
      return {
        title: 'AI-анализ ожидает обработки',
        description: 'Задача на анализ поставлена в очередь обработки.',
        variant: 'pending',
      };
    case 'processing':
      return {
        title: 'AI анализирует изображения',
        description: 'Модель обрабатывает фотографии товара.',
        variant: 'processing',
      };
    case 'succeeded':
      return {
        title: 'AI-анализ выполнен',
        description: 'Результаты визуального распознавания доступны для ознакомления.',
        variant: 'succeeded',
      };
    case 'failed':
      return {
        title: 'AI-анализ не завершён',
        description: 'Не удалось завершить автоматический анализ изображений.',
        variant: 'failed',
      };
    case 'stale':
      return {
        title: 'Устаревший AI-анализ',
        description: 'Есть более старый AI-анализ, но он не относится к текущей версии товара.',
        variant: 'stale',
      };
    default:
      return {
        title: 'AI-анализ недоступен',
        description: 'Данные анализа отсутствуют.',
        variant: 'not_scheduled',
      };
  }
}

/**
 * Returns safe failure code message for failed runs.
 */
export function formatFailureCode(code?: string | null): string {
  if (!code) return '';
  return FAILURE_CODES_RU[code] || code;
}

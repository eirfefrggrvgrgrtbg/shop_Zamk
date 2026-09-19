import type { MeasurementMeta } from './types';

export const MEASUREMENT_FIELDS_MAP: Record<string, MeasurementMeta> = {
  CHEST: {
    label: 'Грудь, см',
    shortLabel: 'Грудь',
    instruction: 'Горизонтально по наиболее выступающим точкам груди.',
  },
  WAIST: {
    label: 'Талия, см',
    shortLabel: 'Талия',
    instruction: 'Горизонтально вокруг самой узкой части талии.',
  },
  HIPS: {
    label: 'Бёдра, см',
    shortLabel: 'Бёдра',
    instruction: 'Горизонтально по наиболее выступающим точкам ягодиц.',
  },
  LENGTH: {
    label: 'Длина изделия, см',
    shortLabel: 'Длина изделия',
    instruction: 'Вертикально от высшей точки плеча до нижнего края изделия.',
  },
  SLEEVE: {
    label: 'Длина рукава, см',
    shortLabel: 'Длина рукава',
    instruction: 'От плечевого шва по внешней стороне руки до запястья.',
  },
  INSEAM: {
    label: 'Внутренний шов, см',
    shortLabel: 'Внутренний шов',
    instruction: 'По внутреннему шву брючины от промежности до низа изделия.',
  },
  FOOT_LENGTH: {
    label: 'Длина стопы, см',
    shortLabel: 'Длина стопы',
    instruction: 'От задней точки пятки до кончика самого длинного пальца стопы.',
  },
  HEAD_CIRCUMFERENCE: {
    label: 'Обхват головы, см',
    shortLabel: 'Обхват головы',
    instruction: 'По окружности головы над бровями и ушами.',
  },
};

export const getMeasurementMeta = (field: string): MeasurementMeta => {
  if (MEASUREMENT_FIELDS_MAP[field]) {
    return MEASUREMENT_FIELDS_MAP[field];
  }
  const formatted = field.toLowerCase().replace(/_/g, ' ');
  return {
    label: `${formatted}, см`,
    shortLabel: formatted,
    instruction: 'Измеряйте согласно стандартам производителя.',
  };
};

export function isLightColor(hex?: string): boolean {
  if (!hex) return false;
  let c = hex.trim().replace('#', '');
  if (c.length === 3) {
    c = c.split('').map((x) => x + x).join('');
  }
  if (c.length !== 6) return false;
  const r = parseInt(c.substring(0, 2), 16);
  const g = parseInt(c.substring(2, 4), 16);
  const b = parseInt(c.substring(4, 6), 16);
  if (isNaN(r) || isNaN(g) || isNaN(b)) return false;
  const brightness = (r * 299 + g * 587 + b * 114) / 1000;
  return brightness > 190;
}

export const isUUID = (str?: string) =>
  Boolean(str && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(str));

export function formatReviewsCount(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 19) return 'отзывов';
  if (mod10 === 1) return 'отзыв';
  if (mod10 >= 2 && mod10 <= 4) return 'отзыва';
  return 'отзывов';
}

export const SizeGuideIllustration = ({ activeFields }: { activeFields: string[] }) => {
  const isFootwear = activeFields.includes('FOOT_LENGTH');
  const isHeadwear =
    activeFields.includes('HEAD_CIRCUMFERENCE') &&
    !activeFields.includes('CHEST') &&
    !activeFields.includes('WAIST');
  const isBottoms =
    activeFields.includes('INSEAM') ||
    ((activeFields.includes('WAIST') || activeFields.includes('HIPS')) &&
      !activeFields.includes('CHEST') &&
      !activeFields.includes('SLEEVE'));

  if (isFootwear) {
    return (
      <svg
        viewBox="0 0 240 160"
        className="w-full max-w-[220px] h-auto"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M30 110 C30 85 45 75 75 75 C100 75 125 70 145 50 C160 35 180 35 195 55 C210 75 220 95 220 115 C220 125 210 130 190 130 C130 130 90 130 30 130 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
        <path d="M25 130 L220 130" className="stroke-graphite/40 dark:stroke-white/40" strokeWidth="1.5" />
        {activeFields.includes('FOOT_LENGTH') && (
          <g className="text-graphite dark:text-white">
            <line x1="30" y1="145" x2="220" y2="145" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="30" y1="140" x2="30" y2="150" stroke="currentColor" strokeWidth="1.5" />
            <line x1="220" y1="140" x2="220" y2="150" stroke="currentColor" strokeWidth="1.5" />
            <text x="125" y="157" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Длина стопы
            </text>
          </g>
        )}
      </svg>
    );
  }

  if (isHeadwear) {
    return (
      <svg
        viewBox="0 0 240 180"
        className="w-full max-w-[220px] h-auto"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M50 130 C45 70 75 35 120 35 C165 35 195 70 190 130 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
        />
        {activeFields.includes('HEAD_CIRCUMFERENCE') && (
          <g className="text-graphite dark:text-white">
            <line x1="45" y1="130" x2="195" y2="130" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <circle cx="45" cy="130" r="2.5" fill="currentColor" />
            <circle cx="195" cy="130" r="2.5" fill="currentColor" />
            <text x="120" y="148" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Обхват головы
            </text>
          </g>
        )}
      </svg>
    );
  }

  if (isBottoms) {
    return (
      <svg
        viewBox="0 0 240 240"
        className="w-full max-w-[220px] h-auto"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M70 35 L170 35 L175 75 L160 215 L125 215 L120 100 L115 215 L80 215 L65 75 Z"
          className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
        {activeFields.includes('WAIST') && (
          <g className="text-graphite dark:text-white">
            <line x1="65" y1="35" x2="175" y2="35" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="65" y1="31" x2="65" y2="39" stroke="currentColor" strokeWidth="1.5" />
            <line x1="175" y1="31" x2="175" y2="39" stroke="currentColor" strokeWidth="1.5" />
            <text x="120" y="27" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Талия
            </text>
          </g>
        )}
        {activeFields.includes('HIPS') && (
          <g className="text-graphite dark:text-white">
            <line x1="60" y1="75" x2="180" y2="75" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="60" y1="71" x2="60" y2="79" stroke="currentColor" strokeWidth="1.5" />
            <line x1="180" y1="71" x2="180" y2="79" stroke="currentColor" strokeWidth="1.5" />
            <text x="120" y="70" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
              Бёдра
            </text>
          </g>
        )}
        {activeFields.includes('INSEAM') && (
          <g className="text-graphite dark:text-white">
            <line x1="120" y1="100" x2="95" y2="215" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <circle cx="120" cy="100" r="2" fill="currentColor" />
            <circle cx="95" cy="215" r="2" fill="currentColor" />
            <text x="128" y="160" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="start">
              Шов
            </text>
          </g>
        )}
        {activeFields.includes('LENGTH') && (
          <g className="text-graphite dark:text-white">
            <line x1="50" y1="35" x2="50" y2="215" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
            <line x1="46" y1="35" x2="54" y2="35" stroke="currentColor" strokeWidth="1.5" />
            <line x1="46" y1="215" x2="54" y2="215" stroke="currentColor" strokeWidth="1.5" />
            <text x="42" y="125" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="end">
              Длина
            </text>
          </g>
        )}
      </svg>
    );
  }

  return (
    <svg
      viewBox="0 0 240 240"
      className="w-full max-w-[220px] h-auto"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M85 35 Q120 48 155 35 L205 75 L180 115 L160 95 L160 210 L80 210 L80 95 L60 115 L35 75 Z"
        className="fill-ice/70 dark:fill-white/5 stroke-graphite/40 dark:stroke-white/40"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      {activeFields.includes('CHEST') && (
        <g className="text-graphite dark:text-white">
          <line x1="75" y1="110" x2="165" y2="110" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="75" y1="105" x2="75" y2="115" stroke="currentColor" strokeWidth="1.5" />
          <line x1="165" y1="105" x2="165" y2="115" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="103" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Грудь
          </text>
        </g>
      )}
      {activeFields.includes('LENGTH') && (
        <g className="text-graphite dark:text-white">
          <line x1="70" y1="40" x2="70" y2="210" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="66" y1="40" x2="74" y2="40" stroke="currentColor" strokeWidth="1.5" />
          <line x1="66" y1="210" x2="74" y2="210" stroke="currentColor" strokeWidth="1.5" />
          <text x="62" y="135" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="end">
            Длина
          </text>
        </g>
      )}
      {activeFields.includes('SLEEVE') && (
        <g className="text-graphite dark:text-white">
          <line x1="155" y1="35" x2="195" y2="95" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <circle cx="155" cy="35" r="2" fill="currentColor" />
          <circle cx="195" cy="95" r="2" fill="currentColor" />
          <text x="185" y="60" fontSize="9" fill="currentColor" fontWeight="600" textAnchor="start">
            Рукав
          </text>
        </g>
      )}
      {activeFields.includes('WAIST') && (
        <g className="text-graphite dark:text-white">
          <line x1="80" y1="155" x2="160" y2="155" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="80" y1="151" x2="80" y2="159" stroke="currentColor" strokeWidth="1.5" />
          <line x1="160" y1="151" x2="160" y2="159" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="150" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Талия
          </text>
        </g>
      )}
      {activeFields.includes('HIPS') && (
        <g className="text-graphite dark:text-white">
          <line x1="80" y1="200" x2="160" y2="200" stroke="currentColor" strokeWidth="1.5" strokeDasharray="3 3" />
          <line x1="80" y1="196" x2="80" y2="204" stroke="currentColor" strokeWidth="1.5" />
          <line x1="160" y1="196" x2="160" y2="204" stroke="currentColor" strokeWidth="1.5" />
          <text x="120" y="195" fontSize="10" fill="currentColor" fontWeight="600" textAnchor="middle">
            Бёдра
          </text>
        </g>
      )}
    </svg>
  );
};

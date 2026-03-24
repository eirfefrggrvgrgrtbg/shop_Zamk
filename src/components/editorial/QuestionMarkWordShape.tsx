import React, { useMemo } from 'react';
import { motion } from 'framer-motion';

export interface QuestionMarkWordShapeProps {
  /** Слово для повторения по траектории */
  word?: string;
  /** Процент плотности (100 = стандартная заливка, меньше = реже) */
  density?: number;
  /** Глобальный множитель масштаба компонента */
  scale?: number;
  /** Глобальная прозрачность */
  opacity?: number;
  /** Внешний класс для контейнера */
  className?: string;
  /** Адаптация цвета под тему (графит для light, бледный для dark) */
  themeAware?: boolean;
}

interface Point { x: number; y: number }

// Кубическая интерполяция Безье для позиций
const getBezierPoint = (t: number, p0: Point, p1: Point, p2: Point, p3: Point): Point => {
  const u = 1 - t;
  const tt = t * t;
  const uu = u * u;
  const uuu = uu * u;
  const ttt = tt * t;

  return {
    x: uuu * p0.x + 3 * uu * t * p1.x + 3 * u * tt * p2.x + ttt * p3.x,
    y: uuu * p0.y + 3 * uu * t * p1.y + 3 * u * tt * p2.y + ttt * p3.y,
  };
};

// Вычисление данных кривой: касательная (угол) и вектор нормали (для ширины линии)
const getBezierData = (t: number, p0: Point, p1: Point, p2: Point, p3: Point) => {
  const pt = getBezierPoint(t, p0, p1, p2, p3);
  
  const u = 1 - t;
  const dx = 3 * u * u * (p1.x - p0.x) + 6 * u * t * (p2.x - p1.x) + 3 * t * t * (p3.x - p2.x);
  const dy = 3 * u * u * (p1.y - p0.y) + 6 * u * t * (p2.y - p1.y) + 3 * t * t * (p3.y - p2.y);
  
  let angle = Math.atan2(dy, dx) * (180 / Math.PI);
  // Защита от переворота слова вверх ногами
  if (dx < 0 && angle > 90) angle -= 180;
  if (dx < 0 && angle < -90) angle += 180;
  
  const len = Math.sqrt(dx * dx + dy * dy) || 1;
  // Нормаль указывает перпендикулярно кривой (исп. для ширины ленты)
  const nx = -dy / len;
  const ny = dx / len;

  return { pt, angle, nx, ny };
};

// Простой псевдорандом (deterministic)
const pseudoRandom = (seed: number) => {
  const x = Math.sin(seed * 999.99) * 10000;
  return x - Math.floor(x);
};

export const QuestionMarkWordShape: React.FC<QuestionMarkWordShapeProps> = ({
  word = 'ЗАМК',
  density = 100,
  scale = 1,
  opacity = 1,
  className = '',
  themeAware = true,
}) => {
  // Цвета: fashion-editorial style. Глубокий, читаемый графит и яркий пепел. 
  // Базовая прозрачность повышена до 60-70% для большей массы.
  const themeClasses = themeAware
    ? 'text-[#333333]/70 dark:text-[#cccccc]/70'
    : 'text-black/70 dark:text-white/70';

  const generatedWords = useMemo(() => {
    const list: any[] = [];
    
    // Плавная, уверенная, красивая дуга
    const arcCurve = {
      p0: { x: 0.15, y: 0.35 },
      p1: { x: 0.20, y: -0.05 },
      p2: { x: 0.85, y: 0.05 },
      p3: { x: 0.82, y: 0.45 },
    };

    // Шея (плавный переход в прямой хвост)
    const neckCurve = {
      p0: { x: 0.82, y: 0.45 },
      p1: { x: 0.80, y: 0.70 },
      p2: { x: 0.50, y: 0.60 },
      p3: { x: 0.50, y: 0.85 },
    };

    // Компактный кластер внизу
    const dotCenter = { x: 0.50, y: 0.96 };

    // Генерация аккуратной типографической ленты
    const generateSegment = (
      curve: any, 
      count: number, 
      seedBase: number, 
      thicknessFn: (t: number) => number
    ) => {
      for (let i = 0; i < count; i++) {
        // Равномерный шаг + очень аккуратный микро-сдвиг
        let t = count === 1 ? 0.5 : i / (count - 1);
        t += (pseudoRandom(seedBase + i) - 0.5) * (0.2 / count);
        t = Math.max(0, Math.min(1, t));

        const { pt, angle, nx, ny } = getBezierData(t, curve.p0, curve.p1, curve.p2, curve.p3);
        const thickness = thicknessFn(t);
        const r = pseudoRandom(seedBase + i * 7);

        // Распределяем слова в широкой полосе (-thickness до +thickness)
        // Чтобы в центре было гуще, берем скошенное распределение (сумма рандомов)
        const spread = (pseudoRandom(r * 2) + pseudoRandom(r * 3) - 1); 
        const bandOffset = spread * thickness;

        // Иерархия размеров (3 уровня: акценты, основа, заполнение)
        let sizeClass = 'text-sm md:text-base'; // Заполнение
        if (r > 0.85) sizeClass = 'text-xl md:text-2xl'; // Акценты (формируют ритм)
        else if (r > 0.6) sizeClass = 'text-lg md:text-xl'; // Основа
        else if (r < 0.15) sizeClass = 'text-xs md:text-sm'; // Микро-заполнение

        // Вес шрифта: тяжелее для крупных, легче для мелких
        let weightClass = 'font-medium';
        if (r > 0.8) weightClass = 'font-bold';
        else if (r > 0.4) weightClass = 'font-semibold';

        // Прозрачность: 0.6 - 1.0 (вместе с базовым 70% дает хорошую плотность)
        const opac = 0.6 + pseudoRandom(r * 4) * 0.4;

        // Вращение: СТРОГО по касательной с мизерным отклонением (до 4-х градусов)
        const rotationJitter = (pseudoRandom(r * 5) - 0.5) * 6;

        list.push({
          id: `seg-${seedBase}-${i}`,
          x: (pt.x + nx * bandOffset) * 100,
          y: (pt.y + ny * bandOffset) * 100,
          rotation: angle + rotationJitter,
          sizeClass,
          weightClass,
          opacity: opac,
        });
      }
    };

    const d = density / 100;

    // Дуга: толщина значительно больше (до 9-11% ширины контейнера)
    // Количество слов: 75 штук на массивную дугу
    const arcThickness = (t: number) => 0.09 + Math.sin(t * Math.PI) * 0.05; 
    generateSegment(arcCurve, Math.floor(75 * d), 1000, arcThickness);

    // Шея: сужается от 9% до 3%
    // 40 слов на шею
    const neckThickness = (t: number) => 0.09 * (1 - t * 0.75); 
    generateSegment(neckCurve, Math.floor(40 * d), 4000, neckThickness);

    // Точка внизу: плотный кластер (около 15 слов, собранная масса)
    const addDot = () => {
      if (d === 0) return;
      
      // Главное центральное слово (якорь)
      list.push({
        id: `dot-center`,
        x: dotCenter.x * 100,
        y: dotCenter.y * 100,
        rotation: 0,
        sizeClass: 'text-3xl md:text-4xl',
        weightClass: 'font-black',
        opacity: 0.95,
      });

      // Переплетенная масса вокруг
      const dotCount = Math.floor(16 * d);
      for (let i = 0; i < dotCount; i++) {
        const r1 = pseudoRandom(7000 + i);
        const r2 = pseudoRandom(8000 + i);
        
        // Чем ближе к центру, тем плотнее масса (квадратичное распределение радиуса)
        const radius = r1 * r1 * 0.045; 
        const angle = r2 * Math.PI * 2;
        
        let sizeClass = 'text-sm md:text-base';
        if (pseudoRandom(9000 + i) > 0.6) sizeClass = 'text-base md:text-lg';

        // Легкий излом внутри точки для создания ощущения объекта
        const rotJitter = (pseudoRandom(10000 + i) - 0.5) * 40; 

        list.push({
          id: `dot-support-${i}`,
          x: (dotCenter.x + radius * Math.cos(angle)) * 100,
          y: (dotCenter.y + radius * Math.sin(angle)) * 100,
          rotation: rotJitter,
          sizeClass,
          weightClass: 'font-semibold',
          opacity: 0.5 + pseudoRandom(11000 + i) * 0.4,
        });
      }
    };

    addDot();

    // Смешивание слоев (крупные и тяжелые слова поднимаем наверх, мелкие прячем вниз)
    // Это создает естественный объем (natural volume) и чистоту силуэта
    const getZValue = (size: string) => {
      if (size.includes('text-2xl') || size.includes('text-3xl') || size.includes('text-4xl')) return 3;
      if (size.includes('text-lg') || size.includes('text-xl')) return 2;
      return 1;
    };
    
    list.sort((a, b) => getZValue(a.sizeClass) - getZValue(b.sizeClass));

    return list;
  }, [density]);

  return (
    <div 
      className={`relative flex items-center justify-center font-serif uppercase tracking-tighter select-none ${className}`}
      style={{ opacity, transform: `scale(${scale})` }}
    >
      {/* 
        Медленный drift композиции. Буквы не бегают (это дешевит), 
        парит вся инсталляция целиком как fashion-объект.
      */}
      <motion.div 
        className="relative w-full h-full"
        animate={{ y: [-8, 8, -8], rotate: [-0.5, 0.5, -0.5] }}
        transition={{ duration: 15, repeat: Infinity, ease: 'easeInOut' }}
      >
        {generatedWords.map((item) => (
          <span
            key={item.id}
            className={`absolute whitespace-nowrap origin-center ${themeClasses} ${item.sizeClass} ${item.weightClass}`}
            style={{
              left: `${item.x}%`,
              top: `${item.y}%`,
              transform: `translate(-50%, -50%) rotate(${item.rotation}deg)`,
              opacity: item.opacity,
            }}
          >
            {word}
          </span>
        ))}
      </motion.div>
    </div>
  );
};

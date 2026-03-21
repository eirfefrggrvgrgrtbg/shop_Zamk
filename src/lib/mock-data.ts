// ─── Types ───
export interface Product {
  id: string;
  name: string;
  brand: string;
  brandId: string;
  price: number;
  oldPrice?: number;
  image: string;
  images?: string[];
  category: string;
  description?: string;
  materials?: string;
  sizes?: string[];
  colors?: { name: string; hex: string }[];
  isNew?: boolean;
  isBestseller?: boolean;
}

export interface Brand {
  id: string;
  name: string;
  description: string;
  country: string;
  image: string;
  logo?: string;
}

export interface Collection {
  id: string;
  title: string;
  subtitle: string;
  image: string;
  productIds: string[];
}

export interface Category {
  id: string;
  name: string;
  icon: string;
  count: number;
}

// ─── Categories ───
export const CATEGORIES: Category[] = [
  { id: 'all', name: 'Все', icon: '✦', count: 0 },
  { id: 'clothing', name: 'Одежда', icon: '👔', count: 48 },
  { id: 'bags', name: 'Сумки', icon: '👜', count: 32 },
  { id: 'shoes', name: 'Обувь', icon: '👟', count: 24 },
  { id: 'accessories', name: 'Аксессуары', icon: '⌚', count: 18 },
  { id: 'jewelry', name: 'Украшения', icon: '💎', count: 12 },
];

// ─── Brands ───
export const BRANDS: Brand[] = [
  {
    id: 'narra-studio',
    name: 'NARRA STUDIO',
    description: 'Российский бренд, создающий функциональные вещи из переработанных материалов. Каждая коллекция — это манифест осознанного потребления и современного минимализма.',
    country: 'Россия',
    image: 'https://images.unsplash.com/photo-1441984904996-e0b6ba687e04?w=800&auto=format',
  },
  {
    id: 'drop-blu',
    name: 'DROP BLU',
    description: 'Авангардный бренд уличной одежды с акцентом на графический дизайн и экспериментальные формы. Вдохновлён японским стритвиром и цифровой эстетикой.',
    country: 'Япония',
    image: 'https://images.unsplash.com/photo-1558171013-2442e067ac7e?w=800&auto=format',
  },
  {
    id: 'kenzo-forma',
    name: 'KENZO FORMA',
    description: 'Итальянское мастерство формы и линии. Бренд специализируется на архитектурных силуэтах и благородных тканях для современного гардероба.',
    country: 'Италия',
    image: 'https://images.unsplash.com/photo-1483985988355-763728e1935b?w=800&auto=format',
  },
  {
    id: 'novalis',
    name: 'NOVALIS',
    description: 'Скандинавская практичность встречает восточную сдержанность. Капсульные коллекции для тех, кто ценит тишину в дизайне.',
    country: 'Дания',
    image: 'https://images.unsplash.com/photo-1469334031218-e382a71b716b?w=800&auto=format',
  },
  {
    id: 'graavel',
    name: 'GRAAVEL',
    description: 'Outdoor-лакшери с европейскими корнями. Технологичные ткани, продуманная фурнитура и городская функциональность.',
    country: 'Германия',
    image: 'https://images.unsplash.com/photo-1445205170230-053b83016050?w=800&auto=format',
  },
  {
    id: 'monade',
    name: 'MONADE',
    description: 'Французская марка аксессуаров и кожаных изделий ручной работы. Каждая вещь — отдельная монада, законченная и совершенная.',
    country: 'Франция',
    image: 'https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=800&auto=format',
  },
  {
    id: 'stahl-air',
    name: 'STAHL AIR',
    description: 'Технологичная обувь нового поколения. Ультралёгкие материалы, 3D-конструкции и минималистичный дизайн.',
    country: 'Южная Корея',
    image: 'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=800&auto=format',
  },
  {
    id: 'atelier-nord',
    name: 'ATELIER NORD',
    description: 'Северная эстетика в каждой детали. Пальто, плащи и верхняя одежда из натуральных тканей с характером.',
    country: 'Норвегия',
    image: 'https://images.unsplash.com/photo-1558618666-fcd25c85f82e?w=800&auto=format',
  },
];

// ─── Products ───
export const PRODUCTS: Product[] = [
  {
    id: 'p1',
    name: 'Анорак из переработанного нейлона',
    brand: 'NARRA STUDIO',
    brandId: 'narra-studio',
    price: 14900,
    image: 'https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=600&auto=format',
    images: [
      'https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=800&auto=format',
      'https://images.unsplash.com/photo-1544022613-e87ca75a784a?w=800&auto=format',
    ],
    category: 'clothing',
    description: 'Лёгкий анорак из переработанного нейлона с капюшоном и регулируемой талией. Водоотталкивающее покрытие DWR. Идеален для переходного сезона.',
    materials: 'Основа: 100% переработанный нейлон. Подкладка: полиэстер. Покрытие: DWR.',
    sizes: ['XS', 'S', 'M', 'L', 'XL'],
    colors: [
      { name: 'Графит', hex: '#3D3D3D' },
      { name: 'Олива', hex: '#6B7B3A' },
      { name: 'Небесный', hex: '#7CB9E8' },
    ],
    isNew: true,
  },
  {
    id: 'p2',
    name: 'Кубы с ручной вышивкой',
    brand: 'DROP BLU',
    brandId: 'drop-blu',
    price: 9200,
    image: 'https://images.unsplash.com/photo-1622560480605-d83c853bc5c3?w=600&auto=format',
    images: [
      'https://images.unsplash.com/photo-1622560480605-d83c853bc5c3?w=800&auto=format',
    ],
    category: 'bags',
    description: 'Компактная сумка-куб с ручной вышивкой. Съёмный ремешок, внутренний карман на молнии.',
    materials: 'Хлопковый канвас, вышивка вручную, фурнитура — матовый никель.',
    sizes: ['One Size'],
    colors: [
      { name: 'Чёрный', hex: '#1A1A1A' },
      { name: 'Серый', hex: '#8C8C8C' },
    ],
    isNew: true,
  },
  {
    id: 'p3',
    name: 'Водонепроницаемая парка',
    brand: 'NOVALIS',
    brandId: 'novalis',
    price: 18600,
    oldPrice: 24800,
    image: 'https://images.unsplash.com/photo-1544022613-e87ca75a784a?w=600&auto=format',
    category: 'clothing',
    description: 'Удлинённая парка с мембраной 10 000mm. Проклеенные швы, регулируемый капюшон, вентиляционные молнии.',
    materials: 'Полиэстер с мембраной, утеплитель PrimaLoft® Silver.',
    sizes: ['S', 'M', 'L', 'XL', 'XXL'],
    colors: [
      { name: 'Тёмно-синий', hex: '#1B2A4A' },
      { name: 'Хаки', hex: '#6B6B4B' },
    ],
    isBestseller: true,
  },
  {
    id: 'p4',
    name: 'Минималистичный свитер',
    brand: 'KENZO FORMA',
    brandId: 'kenzo-forma',
    price: 12500,
    image: 'https://images.unsplash.com/photo-1576566588028-4147f3842f27?w=600&auto=format',
    category: 'clothing',
    description: 'Свитер оверсайз из мериносовой шерсти. Бесшовная вязка, рельефная текстура.',
    materials: '100% мериносовая шерсть экстра-файн.',
    sizes: ['XS', 'S', 'M', 'L'],
    colors: [
      { name: 'Молочный', hex: '#F5F0E8' },
      { name: 'Антрацит', hex: '#383838' },
      { name: 'Пыльная роза', hex: '#C4A4A0' },
    ],
  },
  {
    id: 'p5',
    name: 'Кроссовки GTene',
    brand: 'STAHL AIR',
    brandId: 'stahl-air',
    price: 21500,
    image: 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=600&auto=format',
    images: [
      'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=800&auto=format',
      'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=800&auto=format',
    ],
    category: 'shoes',
    description: 'Городские кроссовки с 3D-вязаным верхом и подошвой из пены EVA. Вес всего 240г.',
    materials: '3D-вязаный текстиль, пена EVA, резиновый аутсоль.',
    sizes: ['39', '40', '41', '42', '43', '44', '45'],
    colors: [
      { name: 'Белый', hex: '#FFFFFF' },
      { name: 'Чёрный', hex: '#1A1A1A' },
      { name: 'Небесный', hex: '#7CB9E8' },
    ],
    isNew: true,
  },
  {
    id: 'p6',
    name: 'Капсулы с графикой художника',
    brand: 'DROP BLU',
    brandId: 'drop-blu',
    price: 7800,
    image: 'https://images.unsplash.com/photo-1552374196-1ab2a1c593e8?w=600&auto=format',
    category: 'clothing',
    description: 'Лимитированная серия футболок с принтами коллаборации с московским художником. Сезонный релиз, ограниченный тираж 12 штук.',
    materials: '100% органический хлопок, 180 г/м².',
    sizes: ['S', 'M', 'L', 'XL'],
    colors: [
      { name: 'Белый', hex: '#FFFFFF' },
      { name: 'Чёрный', hex: '#1A1A1A' },
    ],
  },
  {
    id: 'p7',
    name: 'Дорожная сумка',
    brand: 'GRAAVEL',
    brandId: 'graavel',
    price: 23500,
    image: 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=600&auto=format',
    category: 'bags',
    description: 'Большая дорожная сумка из вощёной ткани с кожаными деталями. Объём 45 л, отделение для ноутбука.',
    materials: 'Вощёный хлопок, натуральная кожа, литая фурнитура.',
    sizes: ['One Size'],
    colors: [
      { name: 'Тан', hex: '#C8A882' },
      { name: 'Оливковый', hex: '#5D5E3C' },
    ],
    isBestseller: true,
  },
  {
    id: 'p8',
    name: 'Базовые слип-оны',
    brand: 'MONADE',
    brandId: 'monade',
    price: 8400,
    image: 'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=600&auto=format',
    category: 'shoes',
    description: 'Минималистичные слип-оны из мягкой кожи. Ортопедическая стелька, гибкая подошва.',
    materials: 'Натуральная кожа, текстильная подкладка.',
    sizes: ['36', '37', '38', '39', '40', '41', '42'],
    colors: [
      { name: 'Чёрный', hex: '#1A1A1A' },
      { name: 'Бежевый', hex: '#D4C5A9' },
    ],
  },
  {
    id: 'p9',
    name: 'Пальто-кокон из шерсти',
    brand: 'ATELIER NORD',
    brandId: 'atelier-nord',
    price: 34900,
    image: 'https://images.unsplash.com/photo-1539533113208-f6df8cc8b543?w=600&auto=format',
    category: 'clothing',
    description: 'Объёмное пальто-кокон из двойной шерстяной ткани. Бесподкладочное, с открытыми швами.',
    materials: '80% шерсть, 20% кашемир.',
    sizes: ['XS', 'S', 'M', 'L'],
    colors: [
      { name: 'Верблюжий', hex: '#C6A96C' },
      { name: 'Тёмно-серый', hex: '#4A4A4A' },
    ],
    isNew: true,
  },
  {
    id: 'p10',
    name: 'Серьги-капли из серебра',
    brand: 'MONADE',
    brandId: 'monade',
    price: 6200,
    image: 'https://images.unsplash.com/photo-1630019852942-f89202989a59?w=600&auto=format',
    category: 'jewelry',
    description: 'Лаконичные серьги-капли из серебра 925 пробы с сатиновой полировкой.',
    materials: 'Серебро 925 пробы.',
    sizes: ['One Size'],
    colors: [
      { name: 'Серебро', hex: '#C0C0C0' },
    ],
  },
  {
    id: 'p11',
    name: 'Кожаный ремень с пряжкой',
    brand: 'GRAAVEL',
    brandId: 'graavel',
    price: 5800,
    image: 'https://images.unsplash.com/photo-1624222247344-550fb60583dc?w=600&auto=format',
    category: 'accessories',
    description: 'Классический ремень из натуральной кожи с матовой стальной пряжкой. Ширина 3.5 см.',
    materials: 'Натуральная кожа вегетального дубления, сталь.',
    sizes: ['S', 'M', 'L'],
    colors: [
      { name: 'Чёрный', hex: '#1A1A1A' },
      { name: 'Коричневый', hex: '#6B4226' },
    ],
  },
  {
    id: 'p12',
    name: 'Тренч оверсайз',
    brand: 'KENZO FORMA',
    brandId: 'kenzo-forma',
    price: 28900,
    oldPrice: 38500,
    image: 'https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=600&auto=format',
    category: 'clothing',
    description: 'Деконструированный тренч в стиле оверсайз. Двухслойный хлопок, съёмный пояс.',
    materials: '100% египетский хлопок, подкладка купро.',
    sizes: ['S', 'M', 'L', 'XL'],
    colors: [
      { name: 'Песочный', hex: '#C2B280' },
      { name: 'Чёрный', hex: '#1A1A1A' },
    ],
  },
  {
    id: 'p13',
    name: 'Парка ветро-stop',
    brand: 'STAHL AIR',
    brandId: 'stahl-air',
    price: 19800,
    image: 'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=600&auto=format',
    category: 'shoes',
    description: 'Треккинговые ботинки с мембраной Gore-Tex и вибрам-подошвой. Для города и лёгких маршрутов.',
    materials: 'Нубук, Gore-Tex, подошва Vibram.',
    sizes: ['40', '41', '42', '43', '44', '45'],
    colors: [
      { name: 'Земляной', hex: '#8B7355' },
      { name: 'Чёрный', hex: '#1A1A1A' },
    ],
  },
  {
    id: 'p14',
    name: 'Очки в титановой оправе',
    brand: 'NOVALIS',
    brandId: 'novalis',
    price: 15200,
    image: 'https://images.unsplash.com/photo-1511499767150-a48a237f0083?w=600&auto=format',
    category: 'accessories',
    description: 'Солнцезащитные очки в минималистичной титановой оправе. Поляризационные линзы Carl Zeiss.',
    materials: 'Титан, линзы Carl Zeiss с поляризацией.',
    sizes: ['One Size'],
    colors: [
      { name: 'Серебро', hex: '#C0C0C0' },
      { name: 'Золото', hex: '#CFB53B' },
    ],
  },
  {
    id: 'p15',
    name: 'Шёлковый шарф',
    brand: 'MONADE',
    brandId: 'monade',
    price: 8900,
    image: 'https://images.unsplash.com/photo-1601924994987-69e26d50dc64?w=600&auto=format',
    category: 'accessories',
    description: 'Лёгкий шарф из натурального шёлка с авторским принтом. Размер 180×70 см.',
    materials: '100% натуральный шёлк.',
    sizes: ['One Size'],
    colors: [
      { name: 'Голубой', hex: '#7CB9E8' },
      { name: 'Пудровый', hex: '#E8C4C4' },
    ],
  },
  {
    id: 'p16',
    name: 'Outdoor-капюшон',
    brand: 'NARRA STUDIO',
    brandId: 'narra-studio',
    price: 11200,
    image: 'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=600&auto=format',
    category: 'clothing',
    description: 'Технологичная куртка-капюшон с системой вентиляции и светоотражающими деталями.',
    materials: 'Рипстоп нейлон, мембрана 5 000mm.',
    sizes: ['S', 'M', 'L', 'XL'],
    colors: [
      { name: 'Чёрный', hex: '#1A1A1A' },
      { name: 'Серебристый', hex: '#B0B0B0' },
    ],
    isNew: true,
  },
];

// ─── Collections ───
export const COLLECTIONS: Collection[] = [
  {
    id: 'col1',
    title: 'Весна 2026',
    subtitle: 'Переходный сезон: лёгкость форм и функциональность',
    image: 'https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=1200&auto=format',
    productIds: ['p1', 'p3', 'p9', 'p12'],
  },
  {
    id: 'col2',
    title: 'Urban Essentials',
    subtitle: 'Базовый гардероб для городских маршрутов',
    image: 'https://images.unsplash.com/photo-1441984904996-e0b6ba687e04?w=1200&auto=format',
    productIds: ['p4', 'p5', 'p8', 'p11'],
  },
  {
    id: 'col3',
    title: 'Редакция: Монохром',
    subtitle: 'Чёрное и белое никогда не выходят из моды',
    image: 'https://images.unsplash.com/photo-1558171013-2442e067ac7e?w=1200&auto=format',
    productIds: ['p6', 'p2', 'p10', 'p15'],
  },
];

// ─── Helper functions ───
export function getProductById(id: string): Product | undefined {
  return PRODUCTS.find(p => p.id === id);
}

export function getProductsByBrand(brandId: string): Product[] {
  return PRODUCTS.filter(p => p.brandId === brandId);
}

export function getBrandById(id: string): Brand | undefined {
  return BRANDS.find(b => b.id === id);
}

export function getNewProducts(): Product[] {
  return PRODUCTS.filter(p => p.isNew);
}

export function getBestsellers(): Product[] {
  return PRODUCTS.filter(p => p.isBestseller);
}

export function getProductsByCategory(categoryId: string): Product[] {
  if (categoryId === 'all') return PRODUCTS;
  return PRODUCTS.filter(p => p.category === categoryId);
}

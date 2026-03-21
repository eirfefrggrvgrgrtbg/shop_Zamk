export interface Product {
  id: string;
  name: string;
  brand: string;
  brandId: string;
  price: number;
  oldPrice?: number;
  discountPrice?: number;
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
}

export interface Collection {
  id: string;
  title: string;
  subtitle: string;
  description: string;
  image: string;
  productIds: string[];
  itemCount: number;
}

export interface Category {
  id: string;
  slug: string;
  name: string;
  icon: string;
  count: number;
}

export const CATEGORIES: Category[] = [
  { id: 'all', slug: 'all', name: 'Все капсулы', icon: '✦', count: 12 },
  { id: 'clothing', slug: 'clothing', name: 'Одежда', icon: '◌', count: 5 },
  { id: 'bags', slug: 'bags', name: 'Сумки', icon: '◍', count: 2 },
  { id: 'shoes', slug: 'shoes', name: 'Обувь', icon: '◐', count: 2 },
  { id: 'accessories', slug: 'accessories', name: 'Аксессуары', icon: '◒', count: 2 },
  { id: 'jewelry', slug: 'jewelry', name: 'Украшения', icon: '◓', count: 1 },
];

export const BRANDS: Brand[] = [
  {
    id: 'narra-studio',
    name: 'Нарра Студио',
    description: 'Функциональная одежда с архитектурным кроем и переработанными материалами.',
    country: 'Россия',
    image: 'https://images.unsplash.com/photo-1441984904996-e0b6ba687e04?w=1200&auto=format',
  },
  {
    id: 'drop-blu',
    name: 'Дроп Блю',
    description: 'Капсульный стритвир с мягкой футуристикой и графическими акцентами.',
    country: 'Япония',
    image: 'https://images.unsplash.com/photo-1558171013-2442e067ac7e?w=1200&auto=format',
  },
  {
    id: 'kenzo-forma',
    name: 'Форма Кэндзо',
    description: 'Скульптурные силуэты и чистая конструкция повседневных вещей.',
    country: 'Италия',
    image: 'https://images.unsplash.com/photo-1483985988355-763728e1935b?w=1200&auto=format',
  },
  {
    id: 'novalis',
    name: 'Новалис',
    description: 'Северная практичность и прозрачная эстетика материалов.',
    country: 'Дания',
    image: 'https://images.unsplash.com/photo-1469334031218-e382a71b716b?w=1200&auto=format',
  },
  {
    id: 'graavel',
    name: 'Граавель',
    description: 'Городской аутдор с эргономикой и премиальной фурнитурой.',
    country: 'Германия',
    image: 'https://images.unsplash.com/photo-1445205170230-053b83016050?w=1200&auto=format',
  },
  {
    id: 'monade',
    name: 'Монада',
    description: 'Нишевая мастерская сумок и украшений ручной работы.',
    country: 'Франция',
    image: 'https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=1200&auto=format',
  },
];

export const PRODUCTS: Product[] = [
  {
    id: 'p1',
    name: 'Анорак ледяной линии',
    brand: 'Нарра Студио',
    brandId: 'narra-studio',
    price: 14900,
    image: 'https://images.unsplash.com/photo-1591047139829-d91aecb6caea?w=800&auto=format',
    category: 'clothing',
    description: 'Легкий анорак для межсезонья и многослойных образов.',
    sizes: ['S', 'M', 'L', 'XL'],
    isNew: true,
  },
  {
    id: 'p2',
    name: 'Сумка-капсула с ручной стежкой',
    brand: 'Дроп Блю',
    brandId: 'drop-blu',
    price: 9200,
    image: 'https://images.unsplash.com/photo-1622560480605-d83c853bc5c3?w=800&auto=format',
    category: 'bags',
    sizes: ['Единый'],
    isNew: true,
  },
  {
    id: 'p3',
    name: 'Парка с мембраной 10 000',
    brand: 'Новалис',
    brandId: 'novalis',
    price: 18600,
    oldPrice: 24800,
    discountPrice: 17100,
    image: 'https://images.unsplash.com/photo-1544022613-e87ca75a784a?w=800&auto=format',
    category: 'clothing',
    sizes: ['S', 'M', 'L', 'XL'],
    isBestseller: true,
  },
  {
    id: 'p4',
    name: 'Свитер объемной вязки',
    brand: 'Форма Кэндзо',
    brandId: 'kenzo-forma',
    price: 12500,
    image: 'https://images.unsplash.com/photo-1576566588028-4147f3842f27?w=800&auto=format',
    category: 'clothing',
    sizes: ['XS', 'S', 'M', 'L'],
  },
  {
    id: 'p5',
    name: 'Кроссовки аэр-сетки',
    brand: 'Новалис',
    brandId: 'novalis',
    price: 21500,
    image: 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=800&auto=format',
    category: 'shoes',
    sizes: ['39', '40', '41', '42', '43'],
    isNew: true,
  },
  {
    id: 'p6',
    name: 'Дорожная сумка для уикенда',
    brand: 'Граавель',
    brandId: 'graavel',
    price: 23500,
    image: 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=800&auto=format',
    category: 'bags',
    sizes: ['Единый'],
    isBestseller: true,
  },
  {
    id: 'p7',
    name: 'Слипоны мягкой формы',
    brand: 'Монада',
    brandId: 'monade',
    price: 8400,
    image: 'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=800&auto=format',
    category: 'shoes',
    sizes: ['36', '37', '38', '39', '40'],
  },
  {
    id: 'p8',
    name: 'Пальто-кокон северной серии',
    brand: 'Нарра Студио',
    brandId: 'narra-studio',
    price: 34900,
    image: 'https://images.unsplash.com/photo-1539533113208-f6df8cc8b543?w=800&auto=format',
    category: 'clothing',
    sizes: ['S', 'M', 'L'],
    isNew: true,
  },
  {
    id: 'p9',
    name: 'Серьги-капли из серебра',
    brand: 'Монада',
    brandId: 'monade',
    price: 6200,
    image: 'https://images.unsplash.com/photo-1630019852942-f89202989a59?w=800&auto=format',
    category: 'jewelry',
    sizes: ['Единый'],
  },
  {
    id: 'p10',
    name: 'Ремень с матовой пряжкой',
    brand: 'Граавель',
    brandId: 'graavel',
    price: 5800,
    image: 'https://images.unsplash.com/photo-1624222247344-550fb60583dc?w=800&auto=format',
    category: 'accessories',
    sizes: ['S', 'M', 'L'],
  },
  {
    id: 'p11',
    name: 'Очки в титановой оправе',
    brand: 'Новалис',
    brandId: 'novalis',
    price: 15200,
    image: 'https://images.unsplash.com/photo-1511499767150-a48a237f0083?w=800&auto=format',
    category: 'accessories',
    sizes: ['Единый'],
  },
  {
    id: 'p12',
    name: 'Тренч с деконструкцией',
    brand: 'Форма Кэндзо',
    brandId: 'kenzo-forma',
    price: 28900,
    oldPrice: 38500,
    discountPrice: 26900,
    image: 'https://images.unsplash.com/photo-1585487000143-6519d58aef14?w=800&auto=format',
    category: 'clothing',
    sizes: ['S', 'M', 'L', 'XL'],
  },
];

export const COLLECTIONS: Collection[] = [
  {
    id: 'col1',
    title: 'Ледяная навигация',
    subtitle: 'Городской межсезонный гардероб',
    description: 'Техничные ткани, мягкие объемы и холодная палитра.',
    image: 'https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=1400&auto=format',
    productIds: ['p1', 'p3', 'p8', 'p12'],
    itemCount: 8,
  },
  {
    id: 'col2',
    title: 'Тихий рейв',
    subtitle: 'Мягкий андеграунд на каждый день',
    description: 'Капсулы для вечерних маршрутов и городского ритма.',
    image: 'https://images.unsplash.com/photo-1441984904996-e0b6ba687e04?w=1400&auto=format',
    productIds: ['p2', 'p4', 'p5', 'p9'],
    itemCount: 7,
  },
  {
    id: 'col3',
    title: 'Архив прозрачности',
    subtitle: 'Тактильные акценты',
    description: 'Сумки и аксессуары для выразительных повседневных образов.',
    image: 'https://images.unsplash.com/photo-1558171013-2442e067ac7e?w=1400&auto=format',
    productIds: ['p6', 'p7', 'p10', 'p11'],
    itemCount: 6,
  },
];

export function getProductById(id: string): Product | undefined {
  return PRODUCTS.find((p) => p.id === id);
}

export function getProductsByBrand(brandId: string): Product[] {
  return PRODUCTS.filter((p) => p.brandId === brandId);
}

export function getBrandById(id: string): Brand | undefined {
  return BRANDS.find((b) => b.id === id);
}

export function getNewProducts(): Product[] {
  return PRODUCTS.filter((p) => p.isNew);
}

export function getBestsellers(): Product[] {
  return PRODUCTS.filter((p) => p.isBestseller);
}

export function getProductsByCategory(categoryId: string): Product[] {
  if (categoryId === 'all') return PRODUCTS;
  return PRODUCTS.filter((p) => p.category === categoryId);
}

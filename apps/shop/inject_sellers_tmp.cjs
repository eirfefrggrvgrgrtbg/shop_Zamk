const fs = require('fs');
let file = fs.readFileSync('src/lib/mock-data.ts', 'utf8');

if (!file.includes('export interface Seller')) {
  file = file.replace('reviews?: Review[];', 'reviews?: Review[];\n  sellerId?: string;');
  
  const sellerMock = `
export interface Seller {
  id: string;
  slug: string;
  name: string;
  avatar: string;
  coverImage: string;
  rating: number;
  reviewCount: number;
  shortDescription: string;
  fullDescription: string;
  city?: string;
  country?: string;
  joinedAt: string;
}

export const SELLERS: Seller[] = [
  {
    id: 's-1',
    slug: 'archiv-studio',
    name: 'Archiv Studio',
    avatar: 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=400&q=80&auto=format',
    coverImage: 'https://images.unsplash.com/photo-1441986300917-64674bd600d8?w=1600&q=80&auto=format',
    rating: 4.9,
    reviewCount: 128,
    shortDescription: 'Кураторская подборка авангардной моды и редких архивных вещей.',
    fullDescription: 'Archiv Studio — это независимый концепт-стор и кураторский проект. Мы специализируемся на поиске уникальных предметов одежды авангардных дизайнеров. Мы верим, что одежда должна быть искусством и нести в себе историю.',
    city: 'Санкт-Петербург',
    country: 'Россия',
    joinedAt: '2022'
  },
  {
    id: 's-2',
    slug: 'form-null',
    name: 'Form Null',
    avatar: 'https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=400&q=80&auto=format',
    coverImage: 'https://images.unsplash.com/photo-1469334031218-e382a71b716b?w=1600&q=80&auto=format',
    rating: 5.0,
    reviewCount: 45,
    shortDescription: 'Минимализм, новые формы и устойчивые материалы.',
    fullDescription: 'Form Null фокусируется на деконструкции и новом минимализме. Каждая вещь в подборке проходит строгий визуальный отбор. Только чистые линии, монохром и сложный крой.',
    city: 'Москва',
    country: 'Россия',
    joinedAt: '2023'
  },
  {
    id: 's-3',
    slug: 'noir-concept',
    name: 'Noir Concept',
    avatar: 'https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=400&q=80&auto=format',
    coverImage: 'https://images.unsplash.com/photo-1550684848-fac1c5b4e853?w=1600&q=80&auto=format',
    rating: 4.8,
    reviewCount: 204,
    shortDescription: 'Темная эстетика и экспериментальный крой.',
    fullDescription: 'Проект, посвященный исследованию темных оттенков в одежде. Мы собираем лучшие примеры азиатского и европейского авангарда, отдавая предпочтение сложным фактурам и асимметрии.',
    city: 'Токио',
    country: 'Япония',
    joinedAt: '2021'
  }
];
`;

  file = file.replace('export const BRANDS: Brand[] = [', sellerMock + '\nexport const BRANDS: Brand[] = [');

  // Add sellerId to PRODUCTS
  let inProducts = false;
  const lines = file.split('\n');
  let currentSeller = 0;
  const sellers = ['s-1', 's-2', 's-3'];

  for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes('export const PRODUCTS: Product[] = [')) {
      inProducts = true;
    }
    if (inProducts && lines[i].startsWith('  id: ')) {
      // Just after id, insert sellerId
      lines.splice(i + 1, 0, `  sellerId: '${sellers[currentSeller % 3]}',`);
      currentSeller++;
      i++;
    }
  }

  fs.writeFileSync('src/lib/mock-data.ts', lines.join('\n'), 'utf8');
}
console.log("Mock data update script finished.");
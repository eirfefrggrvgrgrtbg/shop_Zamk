export interface Review {
  id: string;
  author: string;
  rating: number;
  text: string;
  date: string;
  fit?: 'маломерит' | 'в размер' | 'большемерит';
  quality?: 'отличное' | 'хорошее' | 'нормальное';
}

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
  styles?: string[];
  description?: string;
  materials?: string;
  sizes?: string[];
  colors?: { name: string; hex: string }[];
  variants?: {
    id: string;
    size?: string;
    color?: string;
    inStock?: boolean;
    isActive?: boolean;
    price?: number;
  }[];
  isNew?: boolean;
  isBestseller?: boolean;
  rating?: number;
  reviewsCount?: number;
  reviews?: Review[];
  sellerId?: string;
}

export interface Brand {
  id: string;
  name: string;
  description: string;
  history?: string;
  philosophy?: string;
  origin?: string;
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
  count?: number;
}

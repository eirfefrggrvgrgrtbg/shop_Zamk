import type { Product } from "../components/product/ProductCard";

export const MOCK_PRODUCTS: Product[] = [
  {
    id: "p1",
    name: "Structured Minimalist Coat",
    brand: "STUDIO NICHOLSON",
    price: 890,
    image: "https://images.unsplash.com/photo-1539533113208-f6df8cc8b543?q=80&w=1974&auto=format&fit=crop",
    category: "Clothing",
    isNew: true
  },
  {
    id: "p2",
    name: "Architectural Leather Bag",
    brand: "A.P.C.",
    price: 450,
    image: "https://images.unsplash.com/photo-1584916201218-f4242ceb4809?q=80&w=2030&auto=format&fit=crop",
    category: "Accessories"
  },
  {
    id: "p3",
    name: "Oversized Cashmere Sweater",
    brand: "JIL SANDER",
    price: 1200,
    image: "https://images.unsplash.com/photo-1576566588028-4147f3842f27?q=80&w=1964&auto=format&fit=crop",
    category: "Clothing"
  },
  {
    id: "p4",
    name: "Asymmetric Silk Midi Skirt",
    brand: "LEMAIRE",
    price: 680,
    image: "https://images.unsplash.com/photo-1551163943-3f6a855d1153?q=80&w=2002&auto=format&fit=crop",
    category: "Clothing",
    isNew: true
  },
  {
    id: "p5",
    name: "Sculptural Silver Ring",
    brand: "SOPHIE BUHAI",
    price: 320,
    image: "https://images.unsplash.com/photo-1605100804763-247f67b2548e?q=80&w=1974&auto=format&fit=crop",
    category: "Jewelry"
  },
  {
    id: "p6",
    name: "Square Toe Leather Boots",
    brand: "THE ROW",
    price: 1450,
    image: "https://images.unsplash.com/photo-1608256246200-53e635b5b65f?q=80&w=1974&auto=format&fit=crop",
    category: "Shoes"
  },
  {
    id: "p7",
    name: "Pleated Wide-Leg Trousers",
    brand: "ISSEY MIYAKE",
    price: 590,
    image: "https://images.unsplash.com/photo-1509631179647-0c9528c665e0?q=80&w=1976&auto=format&fit=crop",
    category: "Clothing"
  },
  {
    id: "p8",
    name: "Minimalist Frameless Sunglasses",
    brand: "GENTLE MONSTER",
    price: 280,
    image: "https://images.unsplash.com/photo-1511499767150-a48a237f0083?q=80&w=2080&auto=format&fit=crop",
    category: "Accessories"
  }
];

export const MOCK_BRANDS = [
  "STUDIO NICHOLSON", "A.P.C.", "JIL SANDER", "LEMAIRE", "THE ROW", "ISSEY MIYAKE", "ACNE STUDIOS"
];

export const MOCK_CATEGORIES = [
  "New Arrivals", "Clothing", "Bags", "Shoes", "Accessories", "Jewelry"
];

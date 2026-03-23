import { useState } from 'react';
import { ChevronDown, Search } from 'lucide-react';
import { Link } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Modal } from '../ui/Modal';
import { type Brand, type Category, type Product } from '../../lib/mock-data';
import { ProductCard } from '../product/ProductCard';

interface HeroBlockProps {
  label: string;
  title: React.ReactNode;
  description: string;
  primaryCta?: { label: string; to: string };
  secondaryCta?: { label: string; to: string };
  right?: React.ReactNode;
  ghostWords?: string[];
  className?: string;
}

export function HeroBlock({
  label,
  title,
  description,
  primaryCta,
  secondaryCta,
  right,
  ghostWords,
  className,
}: HeroBlockProps) {
  return (
    <section className={cn('glass-panel-strong relative overflow-hidden p-8 md:p-12 lg:p-14 rounded-[2.6rem]', className)}>
      {ghostWords && ghostWords.length > 0 && (
        <div className='pointer-events-none absolute inset-x-6 top-4 hidden md:flex items-start justify-between gap-8'>
          {ghostWords.slice(0, 3).map((word) => (
            <span
              key={word}
              className='font-serif text-[clamp(3.4rem,8vw,8.5rem)] leading-[0.8] tracking-[-0.02em] text-white/45 select-none'
            >
              {word}
            </span>
          ))}
        </div>
      )}
      <div className='grid grid-cols-1 lg:grid-cols-12 gap-8 items-center'>
        <div className='lg:col-span-7 relative z-10'>
          <FloatingBadge>{label}</FloatingBadge>
          <h1 className='mt-5 text-5xl md:text-6xl lg:text-7xl leading-[0.9] font-serif font-medium tracking-[-0.02em] text-graphite'>
            {title}
          </h1>
          <p className='mt-5 max-w-2xl text-base md:text-lg text-graphite-light leading-relaxed'>{description}</p>
          {(primaryCta || secondaryCta) && (
            <div className='mt-8 flex flex-wrap gap-3'>
              {primaryCta && (
                <Link to={primaryCta.to}>
                  <RoundedButton>{primaryCta.label}</RoundedButton>
                </Link>
              )}
              {secondaryCta && (
                <Link to={secondaryCta.to}>
                  <RoundedButton variant='secondary'>{secondaryCta.label}</RoundedButton>
                </Link>
              )}
            </div>
          )}
        </div>
        <div className='lg:col-span-5 relative z-10'>{right}</div>
      </div>
    </section>
  );
}

export function FloatingBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className='inline-flex items-center rounded-full border border-border-soft bg-white/80 px-4 py-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-graphite-light shadow-sm'>
      {children}
    </span>
  );
}

export function RoundedButton({ className, variant = 'primary', ...props }: React.ComponentProps<typeof Button>) {
  return <Button variant={variant} className={cn('rounded-full px-7', className)} {...props} />;
}

interface SectionHeaderProps {
  label?: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export function SectionHeader({ label, title, description, action }: SectionHeaderProps) {
  return (
    <div className='mb-7 flex flex-col gap-4 md:flex-row md:items-end md:justify-between'>
      <div>
        {label && <p className='studio-label mb-2'>{label}</p>}
        <h2 className='text-[1.75rem] md:text-[2rem] leading-[0.95] tracking-[-0.02em] font-serif text-graphite dark:text-white'>{title}</h2>
        {description && <p className='mt-2.5 max-w-2xl text-sm md:text-base text-graphite-light dark:text-white/70'>{description}</p>}
      </div>
      {action}
    </div>
  );
}

interface PillFilterProps {
  active: boolean;
  label: string;
  onClick?: () => void;
}

export function PillFilter({ active, label, onClick }: PillFilterProps) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'h-10 rounded-full border px-5 text-sm transition-all',
        active
          ? 'bg-graphite text-white dark:text-black border-graphite shadow-sm'
          : 'bg-white/80 text-graphite border-border-soft hover:border-graphite/35'
      )}
    >
      {label}
    </button>
  );
}

export function SearchField(props: React.ComponentProps<typeof Input>) {
  return <Input isSearch className='h-12 rounded-full bg-white/85 border-border-soft' {...props} />;
}

interface SortOption {
  value: string;
  label: string;
}

interface SortDropdownProps {
  value: string;
  options: SortOption[];
  onChange: (value: string) => void;
}

export function SortDropdown({ value, options, onChange }: SortDropdownProps) {
  const [open, setOpen] = useState(false);
  const active = options.find((o) => o.value === value) || options[0];

  return (
    <div className='relative'>
      <button
        className='flex h-10 items-center gap-2 rounded-lg border border-border-soft dark:border-white/20 bg-white dark:bg-transparent px-4 text-sm text-graphite dark:text-white'
        onClick={() => setOpen((v) => !v)}
      >
        {active.label}
        <ChevronDown className='h-4 w-4 text-ash' />
      </button>
      {open && (
        <>
          <div className='fixed inset-0 z-20' onClick={() => setOpen(false)} />
          <div className='absolute right-0 z-30 mt-2 w-56 overflow-hidden rounded-xl border border-border-lighter dark:border-white/10 bg-white dark:bg-[#1a1a1c] shadow-lg'>
            {options.map((opt) => (
              <button
                key={opt.value}
                onClick={() => {
                  onChange(opt.value);
                  setOpen(false);
                }}
                className={cn(
                  'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                  opt.value === value ? 'bg-ice dark:bg-white/10 text-graphite dark:text-white font-medium' : 'text-graphite-light dark:text-white/70 hover:bg-milk dark:hover:bg-white/5'
                )}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

interface BrandDropdownProps {
  value: string | null;
  onChange: (value: string | null) => void;
  brands: Brand[];
}

export function BrandDropdown({ value, onChange, brands }: BrandDropdownProps) {
  const [open, setOpen] = useState(false);
  const activeBrand = brands.find((b) => b.id === value);

  return (
    <div className='relative'>
      <button
        onClick={() => setOpen((v) => !v)}
        className={cn(
          'flex h-10 items-center gap-2 rounded-lg border px-4 text-sm transition-all',
          value
            ? 'bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white'
            : 'bg-white dark:bg-transparent text-graphite dark:text-white border-border-soft dark:border-white/20 hover:border-graphite/35 dark:hover:border-white/50'
        )}
      >
        {activeBrand ? activeBrand.name : 'Бренд'}
        <ChevronDown className='h-4 w-4' />
      </button>
      {open && (
        <>
          <div className='fixed inset-0 z-20' onClick={() => setOpen(false)} />
          <div className='absolute left-0 z-30 mt-2 w-56 overflow-hidden rounded-xl border border-border-lighter dark:border-white/10 bg-white dark:bg-[#1a1a1c] shadow-lg'>
            <button
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
              className={cn(
                'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                !value ? 'bg-ice dark:bg-white/10 text-graphite dark:text-white font-medium' : 'text-graphite-light dark:text-white/70 hover:bg-milk dark:hover:bg-white/5'
              )}
            >
              Все бренды
            </button>
            {brands.map((b) => (
              <button
                key={b.id}
                onClick={() => {
                  onChange(b.id);
                  setOpen(false);
                }}
                className={cn(
                  'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                  value === b.id ? 'bg-ice dark:bg-white/10 text-graphite dark:text-white font-medium' : 'text-graphite-light dark:text-white/70 hover:bg-milk dark:hover:bg-white/5'
                )}
              >
                {b.name}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

interface FilterGroupProps {
  title: string;
  children: React.ReactNode;
}

export function FilterGroup({ title, children }: FilterGroupProps) {
  return (
    <div className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white/80 dark:bg-white/[0.03] p-5'>
      <h4 className='mb-3 text-sm font-semibold text-graphite dark:text-white'>{title}</h4>
      {children}
    </div>
  );
}

export function BrandCard({ brand }: { brand: Brand }) {
  return (
    <Link to={`/brand/${brand.id}`} className='group block overflow-hidden rounded-[2rem] border border-border-lighter bg-white/85 p-2 shadow-sm transition-all hover:-translate-y-1 hover:shadow-cloud'>
      <div className='relative h-56 overflow-hidden rounded-[1.6rem]'>
        <img src={brand.image} alt={brand.name} className='h-full w-full object-cover transition-transform duration-700 group-hover:scale-105' />
        <div className='absolute inset-0 bg-gradient-to-t from-graphite/40 to-transparent' />
      </div>
      <div className='p-5'>
        <h3 className='text-2xl font-serif text-graphite'>{brand.name}</h3>
        <p className='mt-1 text-xs uppercase tracking-[0.14em] text-ash'>{brand.country}</p>
        <p className='mt-3 text-sm text-graphite-light line-clamp-2'>{brand.description}</p>
      </div>
    </Link>
  );
}

export function CategoryCard({ category }: { category: Category }) {
  return (
    <Link to={`/catalog?category=${category.slug}`} className='block rounded-[2rem] border border-border-lighter bg-white/85 p-6 shadow-sm transition-all hover:-translate-y-1 hover:shadow-cloud'>
      <p className='text-2xl'>{category.icon}</p>
      <h3 className='mt-4 text-xl font-serif text-graphite'>{category.name}</h3>
      <p className='mt-1 text-sm text-ash'>{category.count} позиций</p>
    </Link>
  );
}

export function InfoPanel({ title, children, className }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <section className={cn('rounded-[2.3rem] border border-border-lighter bg-white/80 p-7 md:p-9 shadow-sm', className)}>
      <h3 className='text-2xl font-serif text-graphite mb-4'>{title}</h3>
      <div className='text-graphite-light leading-relaxed'>{children}</div>
    </section>
  );
}

export function CheckoutPanel({ children }: { children: React.ReactNode }) {
  return <section className='rounded-[2.3rem] border border-border-lighter bg-white/82 p-7 md:p-9 shadow-cloud'>{children}</section>;
}

export function ProfilePanel({
  title,
  children,
  className,
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={cn('rounded-[2.3rem] border border-border-lighter bg-white/82 p-7 md:p-9 shadow-sm', className)}>
      <h3 className='text-2xl font-serif text-graphite mb-5'>{title}</h3>
      {children}
    </section>
  );
}

export function WishlistCard({ product }: { product: Product }) {
  return (
    <div className='rounded-[2rem] border border-border-lighter bg-white/88 p-2 shadow-sm'>
      <ProductCard product={product} />
    </div>
  );
}

export function RoundedModal(props: React.ComponentProps<typeof Modal>) {
  return <Modal {...props} />;
}

export function TopNav({ links }: { links: Array<{ to: string; label: string }> }) {
  return (
    <nav className='flex items-center gap-2 overflow-x-auto scrollbar-hide'>
      {links.map((link) => (
        <Link key={link.to} to={link.to} className='inline-flex h-10 items-center rounded-full border border-border-soft bg-white/80 px-5 text-sm text-graphite hover:border-graphite/35'>
          {link.label}
        </Link>
      ))}
    </nav>
  );
}

export function SearchInlineHint({ text }: { text: string }) {
  return (
    <div className='inline-flex items-center gap-2 rounded-full bg-white/85 px-4 py-2 text-xs text-ash'>
      <Search className='h-3.5 w-3.5' />
      {text}
    </div>
  );
}

import type { ReactNode } from 'react';
import { CustomerProtectedRoute } from './CustomerProtectedRoute';
import { AccountNav } from './AccountNav';

interface AccountLayoutProps {
  children: ReactNode;
  title: string;
}

export function AccountLayout({ children, title }: AccountLayoutProps) {
  return (
    <CustomerProtectedRoute
      title={title}
      description="Личный кабинет"
    >
      <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-[1200px]'>
          <section className="mb-6 max-w-[980px] mx-auto">
            <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">
              Личный кабинет
            </p>
            <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">
              {title}
            </h1>
          </section>

          <div className="max-w-[980px] mx-auto">
            <AccountNav />
          </div>

          <div className="max-w-[980px] mx-auto mt-4">
            {children}
          </div>
        </div>
      </div>
    </CustomerProtectedRoute>
  );
}

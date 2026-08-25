import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { ProductWizard } from '../components/products/ProductWizard';
import { getSellerProduct } from '@zamk/api-client/src/seller';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import { Loader2 } from 'lucide-react';

export default function SellerProductEdit() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const fetchProduct = useCallback(async (silent = false) => {
    if (!id) return;
    try {
      const res = await getSellerProduct(id);
      setData(res);
    } catch (err) {
      if (!silent) {
        console.error(err);
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, [id]);

  useEffect(() => {
    fetchProduct(false);
  }, [fetchProduct]);

  useVisibilityPolling(useCallback(() => fetchProduct(true), [fetchProduct]), 4000, Boolean(id));

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-gray-400" />
      </div>
    );
  }

  return <ProductWizard isEdit={true} initialData={data} />;
}

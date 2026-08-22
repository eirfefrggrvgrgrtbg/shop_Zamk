import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { ProductWizard } from '../components/products/ProductWizard';
import { getSellerProduct } from '@zamk/api-client/src/seller';
import { Loader2 } from 'lucide-react';

export default function SellerProductEdit() {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (id) {
      getSellerProduct(id).then((res) => {
        setData(res);
        setLoading(false);
      }).catch((err) => {
        console.error(err);
        setLoading(false);
      });
    }
  }, [id]);

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-gray-400" />
      </div>
    );
  }

  return <ProductWizard isEdit={true} initialData={data} />;
}

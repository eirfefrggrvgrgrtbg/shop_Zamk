import { useParams } from 'react-router-dom';

export function SellerProductEdit() {
  const { id } = useParams();

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-4">Edit Product {id}</h1>
      <p className="text-gray-600">
        Placeholder: Here the seller will edit their existing product.
      </p>
    </div>
  );
}

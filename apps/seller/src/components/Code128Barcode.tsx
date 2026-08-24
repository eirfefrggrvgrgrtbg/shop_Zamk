import React, { useEffect, useRef } from 'react';
import JsBarcode from 'jsbarcode';

interface Code128BarcodeProps {
  value: string;
  width?: number;
  height?: number;
  className?: string;
}

export const Code128Barcode: React.FC<Code128BarcodeProps> = ({
  value,
  width = 1.3,
  height = 32,
  className = '',
}) => {
  const svgRef = useRef<SVGSVGElement | null>(null);

  useEffect(() => {
    if (svgRef.current && value) {
      try {
        JsBarcode(svgRef.current, value, {
          format: 'CODE128',
          displayValue: false, // We render the human-readable ZMU text separately with custom typography
          margin: 0,
          height,
          width,
          background: 'transparent',
          lineColor: '#000000',
        });
      } catch (err) {
        console.error('Failed to render barcode:', err);
      }
    }
  }, [value, width, height]);

  return <svg ref={svgRef} className={className} />;
};

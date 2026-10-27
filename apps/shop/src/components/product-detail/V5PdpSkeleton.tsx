/** V5 loading layout: only empty blocks where the gallery, the title/price and the purchase panel will appear. */
export function V5PdpSkeleton() {
  return (
    <div className="zv5-pdp zv5-skel-page" data-testid="v5-pdp-loading" role="status" aria-busy="true" aria-live="polite">
      <span className="zv5-sr-only">Загрузка товара</span>
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 max-w-[1400px]">
        <div className="zv5-skel-crumbs zv5-skel" aria-hidden="true" />
        <div className="zv5-skel-grid" aria-hidden="true">
          <div className="zv5-skel-head">
            <div className="zv5-skel zv5-skel-line zv5-skel-line--s" />
            <div className="zv5-skel zv5-skel-line zv5-skel-line--title" />
            <div className="zv5-skel zv5-skel-line zv5-skel-line--m" />
          </div>
          <div className="zv5-skel-gallery">
            <div className="zv5-skel zv5-skel-stage" />
            <div className="zv5-skel-thumbs">
              <div className="zv5-skel zv5-skel-thumb" />
              <div className="zv5-skel zv5-skel-thumb" />
              <div className="zv5-skel zv5-skel-thumb" />
            </div>
          </div>
          <div className="zv5-skel-buy">
            <div className="zv5-skel zv5-skel-line zv5-skel-line--s" />
            <div className="zv5-skel zv5-skel-options" />
            <div className="zv5-skel zv5-skel-cta" />
          </div>
        </div>
      </div>
    </div>
  );
}

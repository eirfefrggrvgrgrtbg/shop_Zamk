import { useEffect } from 'react';

export function PreviewPageMetadata() {
  useEffect(() => {
    // Save previous values
    const prevReferrer = document.querySelector('meta[name="referrer"]')?.getAttribute('content');
    const prevRobots = document.querySelector('meta[name="robots"]')?.getAttribute('content');

    // Helper to set or create meta tag
    const setMeta = (name: string, content: string) => {
      let meta = document.querySelector(`meta[name="${name}"]`);
      if (!meta) {
        meta = document.createElement('meta');
        meta.setAttribute('name', name);
        document.head.appendChild(meta);
      }
      meta.setAttribute('content', content);
      return meta;
    };

    // Apply preview policies
    const referrerMeta = setMeta('referrer', 'no-referrer');
    const robotsMeta = setMeta('robots', 'noindex,nofollow');

    // Cleanup on unmount
    return () => {
      if (prevReferrer === undefined) {
        referrerMeta?.remove();
      } else {
        referrerMeta?.setAttribute('content', prevReferrer || '');
      }

      if (prevRobots === undefined) {
        robotsMeta?.remove();
      } else {
        robotsMeta?.setAttribute('content', prevRobots || '');
      }
    };
  }, []);

  return null;
}

/**
 * HeroShadow — soft elliptical shadow below the floating 3D object
 *
 * ── CONFIG PARAMETERS ────────────────────────────────────────────────────────
 */
const SHADOW_WIDTH   = '65%';
const SHADOW_HEIGHT  = '40px';
const SHADOW_OPACITY = 0.45;
const SHADOW_BLUR    = '32px';
const SHADOW_BOTTOM  = '5%';
/** ─────────────────────────────────────────────────────────────────────────── */

export function HeroShadow() {
  return (
    <div
      aria-hidden="true"
      style={{
        position: 'absolute',
        bottom: SHADOW_BOTTOM,
        left: '50%',
        transform: 'translateX(-50%)',
        width: SHADOW_WIDTH,
        height: SHADOW_HEIGHT,
        borderRadius: '50%',
        background: `radial-gradient(ellipse at center, rgba(34,52,79,${SHADOW_OPACITY}) 0%, transparent 70%)`,
        filter: `blur(${SHADOW_BLUR})`,
        pointerEvents: 'none',
        zIndex: 1,
      }}
    />
  );
}

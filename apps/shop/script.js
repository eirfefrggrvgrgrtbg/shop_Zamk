
const fs = require("fs");
let css = fs.readFileSync("src/index.css", "utf8");

const newCSS = `
@layer utilities {
  .premium-glass-card {
    @apply h-full bg-white dark:bg-[#070707] rounded-[1.4rem] transition-all duration-500 overflow-hidden relative group flex flex-col hover:-translate-y-0.5;
    
    /* Light theme glass feeling */
    box-shadow: 
      inset 0 1px 1px rgba(255, 255, 255, 0.9), 
      inset 0 -1px 1px rgba(0, 0, 0, 0.02),
      0 2px 12px rgba(120, 148, 180, 0.08);
  }
  
  .premium-glass-card::before {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: 1.4rem;
    padding: 1px;
    background: linear-gradient(180deg, rgba(230,235,245,0.7) 0%, rgba(200,210,225,0.2) 100%);
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    pointer-events: none;
    z-index: 10;
  }
  
  .premium-glass-card:hover {
    box-shadow: 
      inset 0 1px 1px rgba(255, 255, 255, 1),
      inset 0 -1px 1px rgba(0, 0, 0, 0.02),
      0 12px 40px rgba(100, 135, 175, 0.14);
  }

  .dark .premium-glass-card {
    box-shadow: 
      inset 0 1px 0 rgba(255, 255, 255, 0.04),
      0 4px 20px rgba(0, 0, 0, 0.6);
  }

  .dark .premium-glass-card::before {
    background: linear-gradient(180deg, rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.01) 100%);
  }

  .dark .premium-glass-card:hover {
    box-shadow: 
      inset 0 1px 0 rgba(255, 255, 255, 0.06),
      0 12px 40px rgba(0, 0, 0, 0.8);
  }
}
`;

css = css + newCSS;
fs.writeFileSync("src/index.css", css);

let pc = fs.readFileSync("src/components/product/ProductCard.tsx", "utf8");
pc = pc.replace(
  /className=\"relative group flex flex-col[^\"]+\"/,
  \"className=\\\"premium-glass-card\\\"\"
);
fs.writeFileSync("src/components/product/ProductCard.tsx", pc);

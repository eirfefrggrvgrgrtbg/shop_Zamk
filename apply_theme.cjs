const fs = require('fs');
let code = fs.readFileSync('src/index.css', 'utf8');

const themeReplacement = `@theme {
  --color-milk: var(--color-milk);
  --color-ice: var(--color-ice);
  --color-paper: var(--color-paper);
  --color-surface: var(--color-surface);
  --color-surface-hover: var(--color-surface-hover);

  --color-primary: var(--color-primary);
  --color-primary-hover: var(--color-primary-hover);
  --color-primary-soft: var(--color-primary-soft);

  --color-accent: var(--color-accent);
  --color-accent-hover: var(--color-accent-hover);
  --color-lavender: var(--color-lavender);

  --color-graphite: var(--color-graphite);
  --color-graphite-light: var(--color-graphite-light);
  --color-ash: var(--color-ash);
  --color-ash-light: var(--color-ash-light);

  --color-border-lighter: var(--color-border-lighter);
  --color-border-soft: var(--color-border-soft);

  --color-success: #10B981;
  --color-error: #EF4444;

  --color-white: var(--color-white);
  --col-white: var(--color-white);
  --color-black: var(--color-black);

  --font-sans: 'Manrope', 'Inter', 'Segoe UI', sans-serif;
  --font-serif: 'Cormorant Garamond', 'Times New Roman', serif;
}

@layer base {
  :root {
    --color-milk: #FBFCFE;
    --color-ice: #EEF4FA;
    --color-paper: #FFFFFF;
    --color-surface: #FFFFFF;
    --color-surface-hover: #F6FAFD;

    --color-primary: #1C2733;
    --color-primary-hover: #273646;
    --color-primary-soft: #1A222C0A;

    --color-accent: #88AFDA;
    --color-accent-hover: #789FCA;
    --color-lavender: #B6B4CE;

    --color-graphite: #1C2733;
    --color-graphite-light: #5F7085;
    --color-ash: #95A5BA;
    --color-ash-light: #CBD9E6;

    --color-border-lighter: #DFE8F2;
    --color-border-soft: #C9D6E3;
    
    --color-white: #FFFFFF;
    --color-black: #000000;
  }
  
  .dark {
    --color-milk: #111111;
    --color-ice: #0a0a0a;
    --color-paper: #161616;
    --color-surface: #141414;
    --color-surface-hover: #1f1f1f;

    --color-primary: #FFFFFF;
    --color-primary-hover: #E0E0E0;
    --color-primary-soft: #FFFFFF1A;

    --color-accent: #88AFDA;
    --color-accent-hover: #9CBDE5;
    --color-lavender: #82809C;

    --color-graphite: #FBFCFE;
    --color-graphite-light: #A0B0C0;
    --color-ash: #65758A;
    --color-ash-light: #344356;

    --color-border-lighter: #333333;
    --color-border-soft: #444444;
    
    --color-white: #171717;
    --color-black: #FFFFFF;
  }

  html {
    transition: all 0.5s ease;
  }
  
  * {
    transition: background-color 0.5s ease, border-color 0.5s ease, color 0.5s ease, box-shadow 0.5s ease;
  }
}`;

code = code.replace(/@theme \{[\s\S]*?\}/, themeReplacement);
fs.writeFileSync('src/index.css', code);

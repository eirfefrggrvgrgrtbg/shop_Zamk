const fs = require('fs');

const replaceInFile = (file, replacements) => {
  let content = fs.readFileSync(file, 'utf8');
  let changed = false;
  for (const [oldStr, newStr] of replacements) {
    if (content.includes(oldStr)) {
      content = content.replace(oldStr, newStr);
      changed = true;
    }
  }
  if (changed) {
    fs.writeFileSync(file, content);
    console.log(`Updated ${file}`);
  } else {
    console.log(`No changes in ${file}`);
  }
};

// 1. EditorialHero.tsx
replaceInFile('src/components/editorial/EditorialHero.tsx', [
  ['bg-graphite text-white hover:bg-primary-hover', 'bg-graphite text-white dark:text-black hover:bg-primary-hover']
]);

// 2. StudioKit.tsx
replaceInFile('src/components/editorial/StudioKit.tsx', [
  ["? 'bg-graphite text-white border-graphite shadow-sm'", "? 'bg-graphite text-white dark:text-black border-graphite shadow-sm'"]
]);

// 3. Footer.tsx
replaceInFile('src/components/layout/Footer.tsx', [
  ['bg-graphite text-white rounded-full', 'bg-graphite text-white dark:text-black rounded-full']
]);

// 4. ProductCard.tsx
replaceInFile('src/components/product/ProductCard.tsx', [
  ['bg-graphite text-white text-[10px]', 'bg-graphite text-white dark:text-black text-[10px]']
]);

// 5. Badge.tsx
replaceInFile('src/components/ui/Badge.tsx', [
  ['"bg-graphite text-white border-0 shadow-sm"', '"bg-graphite text-white dark:text-black border-0 shadow-sm"']
]);

// 6. Catalog.tsx (both spots)
replaceInFile('src/pages/Catalog.tsx', [
  ['bg-graphite text-white text-xs', 'bg-graphite text-white dark:text-black text-xs'],
  ['bg-graphite text-white text-sm', 'bg-graphite text-white dark:text-black text-sm']
]);

// 7. ProductDetail.tsx
replaceInFile('src/pages/ProductDetail.tsx', [
  ['bg-graphite text-white text-xs', 'bg-graphite text-white dark:text-black text-xs']
]);

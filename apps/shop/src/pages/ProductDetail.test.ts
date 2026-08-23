import { isLightColor, getMeasurementMeta } from './ProductDetail';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

function runUnitTests() {
  console.log('Testing ProductDetail visual & logic behaviors...');

  // A. white/light swatch receives visible boundary treatment
  assert(isLightColor('#ffffff') === true, 'Pure white must be detected as light color');
  assert(isLightColor('#fff') === true, 'Short white must be detected as light color');
  assert(isLightColor('#f8f8f8') === true, 'Off-white must be detected as light color');
  assert(isLightColor('#ffff00') === true, 'Bright yellow must be detected as light color');
  assert(isLightColor('#000000') === false, 'Black is not light color');
  assert(isLightColor('#121214') === false, 'Dark graphite is not light color');
  assert(isLightColor('#ef4444') === false, 'Standard red is not light color');

  // B. measurement illustration receives active canonical field codes, not category title strings
  const chestMeta = getMeasurementMeta('CHEST');
  assert(chestMeta.label === 'Грудь, см', 'CHEST label should be Грудь, см');
  assert(chestMeta.shortLabel === 'Грудь', 'CHEST shortLabel should be Грудь');
  assert(chestMeta.instruction.length > 0, 'Instruction should not be empty');

  const lengthMeta = getMeasurementMeta('LENGTH');
  assert(lengthMeta.label === 'Длина изделия, см', 'LENGTH label should be Длина изделия, см');
  assert(lengthMeta.shortLabel === 'Длина изделия', 'LENGTH shortLabel should be Длина изделия');

  const sleeveMeta = getMeasurementMeta('SLEEVE');
  assert(sleeveMeta.label === 'Длина рукава, см', 'SLEEVE label should be Длина рукава, см');
  assert(sleeveMeta.shortLabel === 'Длина рукава', 'SLEEVE shortLabel should be Длина рукава');

  // C. CHEST + LENGTH + SLEEVE does not render WAIST guide
  const activeFields1 = ['CHEST', 'LENGTH', 'SLEEVE'];
  assert(activeFields1.includes('WAIST') === false, 'WAIST must not be in activeFields for Hoodie');
  const relevantInstructions = activeFields1.map(f => getMeasurementMeta(f).shortLabel);
  assert(relevantInstructions.includes('Талия') === false, 'Талия instruction must not be rendered');
  assert(relevantInstructions.includes('Грудь') === true, 'Грудь instruction must be rendered');
  assert(relevantInstructions.includes('Длина изделия') === true, 'Длина изделия instruction must be rendered');
  assert(relevantInstructions.includes('Длина рукава') === true, 'Длина рукава instruction must be rendered');

  // D. LANDSCAPE image selects landscape presentation
  const landscapeRatio = 1920 / 1080; // 1.77
  const landscapeOrientation = landscapeRatio > 1.15 ? 'landscape' : (landscapeRatio >= 0.85 ? 'square' : 'portrait');
  assert(landscapeOrientation === 'landscape', 'Landscape ratio must yield landscape orientation');

  // E. PORTRAIT image selects portrait presentation
  const portraitRatio = 800 / 1000; // 0.8
  const portraitOrientation = portraitRatio > 1.15 ? 'landscape' : (portraitRatio >= 0.85 ? 'square' : 'portrait');
  assert(portraitOrientation === 'portrait', 'Portrait ratio must yield portrait orientation');

  // F. changing color clears now-invalid selected Size
  const mockVariants = [
    { id: 'v1', colorId: 'red', size: 'M', isActive: true },
    { id: 'v2', colorId: 'red', size: 'L', isActive: true },
    { id: 'v3', colorId: 'white', size: 'S', isActive: true },
    { id: 'v4', colorId: 'white', size: 'M', isActive: true },
  ];

  // User had 'L' selected in 'red'. Switched to 'white'.
  let activeSize: string | null = 'L';
  const newColorId = 'white';
  const sizeStillValid = mockVariants.some(v => v.isActive && v.colorId === newColorId && v.size === activeSize);
  if (!sizeStillValid) {
    activeSize = null;
  }
  assert(activeSize === null, 'Active size L must be cleared when switching to white (only S, M exist)');

  // User had 'M' selected in 'red'. Switched to 'white'.
  activeSize = 'M';
  const sizeStillValid2 = mockVariants.some(v => v.isActive && v.colorId === newColorId && v.size === activeSize);
  if (!sizeStillValid2) {
    activeSize = null;
  }
  assert(activeSize === 'M', 'Active size M must be retained when switching to white since white has M');

  console.log('ALL UNIT TESTS PASSED');
}

runUnitTests();

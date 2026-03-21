export function IcyBackground() {
  return (
    <div className="fixed inset-0 z-0 pointer-events-none">
      {/* Главный контейнер для неба */}
      <div className="absolute inset-0 bg-[#ffffff] overflow-hidden">
        {/* Базовый градиент неба - сдержанный серо-голубой, быстро переходящий в белый как во втором макете */}
        <div className="absolute inset-0 bg-[linear-gradient(180deg,#89a6c2_0%,#b2ccdf_15%,#e0eff8_30%,#ffffff_45%)]" />
        
        {/* --- Воздушные массы (Мягкие облака) --- */}
        
        {/* Левое верхнее мягкое облако */}
        <div className="absolute top-[-10%] left-[-10%] w-[50%] h-[35%] bg-white/70 blur-[100px] rounded-[100%]" />
        
        {/* Правое верхнее облако */}
        <div className="absolute top-[-5%] right-[-15%] w-[55%] h-[40%] bg-white/70 blur-[110px] rounded-[100%]" />
        
        {/* Центральное пушистое облако (на уровне больших букв) */}
        <div className="absolute top-[10%] left-[20%] right-[20%] h-[30%] bg-white/95 blur-[100px] rounded-[100%]" />
        
        {/* Тяжелые белые массивы, закрывающие низ неба */}
        <div className="absolute top-[22%] left-[-20%] w-[80%] h-[40%] bg-white blur-[100px] rounded-[100%]" />
        <div className="absolute top-[20%] right-[-20%] w-[80%] h-[40%] bg-white blur-[100px] rounded-[100%]" />
        
        {/* Абсолютно белый фундамент для контента, чтобы карточки были на белом фоне как на фото */}
        <div className="absolute top-[40%] left-0 right-0 bottom-0 bg-white" />
      </div>

      {/* Текстура шума для эффекта фотографии/волокнистой бумаги как на референсе */}
      <svg id="noise-overlay" xmlns="http://www.w3.org/2000/svg" className="absolute inset-0 w-full h-full opacity-[0.25] mix-blend-soft-light pointer-events-none">
        <filter id="noiseFilter">
          <feTurbulence 
            type="fractalNoise" 
            baseFrequency="0.65" 
            numOctaves="3" 
            stitchTiles="stitch" 
          />
        </filter>
        <rect width="100%" height="100%" filter="url(#noiseFilter)" />
      </svg>
    </div>
  );
}

const fs = require('fs');
const content = fs.readFileSync('src/pages/Settings.tsx', 'utf8');
const lines = content.split('\n');

const secStart = lines.findIndex(l => l.includes('{/* 1. Безопасность */}'));
const appStart = lines.findIndex(l => l.includes('{/* 2. Внешний вид */}'));
const notStart = lines.findIndex(l => l.includes('{/* 3. Уведомления */}'));
const delStart = lines.findIndex(l => l.includes('{/* 4. Доставка */}'));
const endSec = lines.findIndex(l => l.includes('Модалки */}')) - 2;

const security = lines.slice(secStart, appStart-1).join('\n');
const appearance = lines.slice(appStart, notStart-1).join('\n');
const notifications = lines.slice(notStart, delStart-1).join('\n');
const delivery = lines.slice(delStart, endSec+1).join('\n');

const payments = `        {/* 2.5 Способы оплаты */}
        <Section title="Способы оплаты">
          <div className="p-5 md:p-6 grid gap-4">
            {payments.map((item) => (
              <div key={item.id} className={\`p-4 rounded-2xl border transition-all \${item.isDefault ? 'border-graphite/30 bg-white/60' : 'border-white/50 bg-white/20'}\`}>
                <div className="flex justify-between items-start mb-2">
                  <div className="flex items-center gap-2">
                    <CreditCard className="w-4 h-4 text-graphite/50" />
                    <h5 className="font-medium text-graphite text-[14px]">{item.cardNumber}</h5>
                    {item.isDefault && (
                      <span className="bg-graphite/10 text-graphite px-2 py-0.5 rounded-full text-[10px] uppercase tracking-wider font-semibold">Основной</span>
                    )}
                  </div>
                  <div className="flex gap-3 text-[12px] font-medium">
                    <button onClick={() => handleOpenPaymentModal(item)} className="text-graphite/40 hover:text-graphite transition-colors">Изм.</button>
                    {!item.isDefault && <button onClick={() => handleDeletePayment(item.id)} className="text-red-400 hover:text-red-500 transition-colors">Удал.</button>}
                  </div>
                </div>
                <p className="text-[13px] text-graphite/60 pl-6 leading-relaxed">Срок действия: {item.expiry}</p>
                {!item.isDefault && (
                   <div className="pl-6 mt-3">
                     <button onClick={() => setPayments(payments.map(a => ({...a, isDefault: a.id === item.id})))} className="text-[12px] text-graphite hover:underline underline-offset-4">
                       Сделать основным
                     </button>
                   </div>
                )}
              </div>
            ))}
            
            <button onClick={() => handleOpenPaymentModal()} className="flex items-center justify-center gap-2 w-full py-4 mt-2 border border-dashed border-graphite/20 rounded-2xl text-graphite/50 hover:text-graphite hover:border-graphite/40 hover:bg-graphite/5 transition-all text-[14px] font-medium">
              <Plus className="w-4 h-4" />
              Добавить способ оплаты
            </button>
          </div>
        </Section>`;

const before = lines.slice(0, secStart).join('\n');
const after = lines.slice(endSec + 1).join('\n');

const newContent = before + appearance + '\n\n' + payments + '\n\n' + delivery + '\n\n' + notifications + '\n\n' + security + '\n' + after;

fs.writeFileSync('src/pages/Settings.tsx', newContent);
console.log('SUCCESS');
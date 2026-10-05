#!/usr/bin/env node

const fs = require('fs');

function main() {
  let input = '';
  try {
    input = fs.readFileSync(0, 'utf-8');
  } catch (err) {
    console.log(JSON.stringify({ decision: 'allow' }));
    process.exit(0);
  }

  if (!input || !input.trim()) {
    console.log(JSON.stringify({ decision: 'allow' }));
    process.exit(0);
  }

  try {
    const data = JSON.parse(input);
    const cmd = (data.toolCall && data.toolCall.args && data.toolCall.args.CommandLine) || '';

    // Check for commands that target dev DB "zamk" instead of "zamk_test"
    const isTest = /go\s+test/i.test(cmd);
    const targetsDevDb = /[:@/](zamk)([\s"'\/?&]|$)/i.test(cmd) && !/zamk_test/i.test(cmd);
    const isDestructiveSql = /\b(DROP\s+DATABASE|TRUNCATE\s+(TABLE\s+)?|DROP\s+TABLE)\b/i.test(cmd);

    // Case 1: Automated test targeting dev DB zamk
    if (isTest && targetsDevDb && /TEST_DATABASE_URL/i.test(cmd)) {
      console.log(JSON.stringify({
        decision: 'deny',
        reason: 'Blocked by ZAMK DB Safety Hook: Automated tests must target zamk_test, not the development database zamk.'
      }));
      process.exit(0);
    }

    // Case 2: Destructive SQL targeting dev DB zamk
    if (isDestructiveSql && targetsDevDb) {
      console.log(JSON.stringify({
        decision: 'deny',
        reason: 'Blocked by ZAMK DB Safety Hook: Destructive SQL operations against dev database zamk are prohibited.'
      }));
      process.exit(0);
    }

    // Default: allow
    console.log(JSON.stringify({ decision: 'allow' }));
  } catch (err) {
    // If JSON parsing fails, allow benign commands
    console.log(JSON.stringify({ decision: 'allow' }));
  }
}

main();

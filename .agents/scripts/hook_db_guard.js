#!/usr/bin/env node

const fs = require('fs');

const AUTH_TOKEN = 'ZAMK_OWNER_' + 'ACCEPTED_FINALIZE=YES';

function checkDbSafety(cmd) {
  const isTest = /go\s+test/i.test(cmd);
  const targetsDevDb = /(?:[:@/]|-(?:d\s*|-dbname[=\s]))\s*(zamk)([\s"'\/?&;]|$)/i.test(cmd) && !/zamk_test/i.test(cmd);
  const isDestructiveSql = /\b(DROP\s+DATABASE|TRUNCATE\s+(TABLE\s+)?|DROP\s+TABLE)\b/i.test(cmd);

  // Case 1: Automated test targeting dev DB zamk
  if (isTest && targetsDevDb && /TEST_DATABASE_URL/i.test(cmd)) {
    return {
      decision: 'deny',
      reason: 'Blocked by ZAMK DB Safety Hook: Automated tests must target zamk_test, not the development database zamk.'
    };
  }

  // Case 2: Destructive SQL targeting dev DB zamk
  if (isDestructiveSql && targetsDevDb) {
    return {
      decision: 'deny',
      reason: 'Blocked by ZAMK DB Safety Hook: Destructive SQL operations against dev database zamk are prohibited.'
    };
  }

  return null;
}

function isOwnerFinalizeAuthorized(transcriptPath) {
  if (!transcriptPath || typeof transcriptPath !== 'string') {
    return false;
  }

  try {
    let targetPath = transcriptPath;

    // Check if full transcript exists in the same directory
    if (targetPath.endsWith('transcript.jsonl')) {
      const fullPath = targetPath.replace(/transcript\.jsonl$/, 'transcript_full.jsonl');
      if (fs.existsSync(fullPath)) {
        targetPath = fullPath;
      }
    }

    if (!fs.existsSync(targetPath)) {
      return false;
    }

    const fileContent = fs.readFileSync(targetPath, 'utf-8');
    if (!fileContent || !fileContent.trim()) {
      return false;
    }

    const lines = fileContent.split('\n');
    let latestUserContent = null;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i].trim();
      if (!line) continue;

      let entry;
      try {
        entry = JSON.parse(line);
      } catch (e) {
        // Malformed line in transcript -> fail closed
        return false;
      }

      // Check for user turn
      if (entry && (entry.source === 'USER_EXPLICIT' || entry.type === 'USER_INPUT')) {
        latestUserContent = entry.content || '';
      }
    }

    if (latestUserContent === null) {
      return false;
    }

    // Check if latest user turn has an exact standalone authorization line
    const contentLines = latestUserContent.split(/\r?\n/);
    for (const rawLine of contentLines) {
      if (rawLine.trim() === AUTH_TOKEN) {
        return true;
      }
    }

    return false;
  } catch (err) {
    return false;
  }
}

// Tokenize a single shell command segment into words
function tokenize(segment) {
  const tokens = [];
  let current = '';
  let inSingleQuote = false;
  let inDoubleQuote = false;
  let escape = false;

  for (let i = 0; i < segment.length; i++) {
    const char = segment[i];
    if (escape) {
      current += char;
      escape = false;
      continue;
    }
    if (char === '\\') {
      escape = true;
      continue;
    }
    if (char === "'" && !inDoubleQuote) {
      inSingleQuote = !inSingleQuote;
      continue;
    }
    if (char === '"' && !inSingleQuote) {
      inDoubleQuote = !inDoubleQuote;
      continue;
    }
    if (/\s/.test(char) && !inSingleQuote && !inDoubleQuote) {
      if (current.length > 0) {
        tokens.push(current);
        current = '';
      }
      continue;
    }
    current += char;
  }
  if (current.length > 0) {
    tokens.push(current);
  }
  return tokens;
}

// Split a full command line on shell operators (&&, ||, ;, |, newline)
// while ignoring operators inside quotes
function splitCompoundCommands(cmd) {
  const segments = [];
  let current = '';
  let inSingleQuote = false;
  let inDoubleQuote = false;
  let escape = false;

  for (let i = 0; i < cmd.length; i++) {
    const char = cmd[i];
    if (escape) {
      current += char;
      escape = false;
      continue;
    }
    if (char === '\\') {
      current += char;
      escape = true;
      continue;
    }
    if (char === "'" && !inDoubleQuote) {
      inSingleQuote = !inSingleQuote;
      current += char;
      continue;
    }
    if (char === '"' && !inSingleQuote) {
      inDoubleQuote = !inDoubleQuote;
      current += char;
      continue;
    }
    if (!inSingleQuote && !inDoubleQuote) {
      if (char === ';' || char === '\n') {
        if (current.trim().length > 0) segments.push(current.trim());
        current = '';
        continue;
      }
      if (char === '&' && cmd[i + 1] === '&') {
        if (current.trim().length > 0) segments.push(current.trim());
        current = '';
        i++;
        continue;
      }
      if (char === '|' && cmd[i + 1] === '|') {
        if (current.trim().length > 0) segments.push(current.trim());
        current = '';
        i++;
        continue;
      }
      if (char === '|' && cmd[i + 1] !== '|') {
        if (current.trim().length > 0) segments.push(current.trim());
        current = '';
        continue;
      }
    }
    current += char;
  }
  if (current.trim().length > 0) {
    segments.push(current.trim());
  }
  return segments;
}

function isFinalizeInvocation(tokens) {
  if (!tokens || tokens.length === 0) return false;

  let idx = 0;
  while (idx < tokens.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(tokens[idx])) {
    idx++;
  }
  if (idx >= tokens.length) return false;

  while (idx < tokens.length && (tokens[idx] === 'sudo' || tokens[idx] === 'env' || tokens[idx] === 'command' || tokens[idx] === 'builtin')) {
    idx++;
    while (idx < tokens.length && (/^[A-Za-z_][A-Za-z0-9_]*=/.test(tokens[idx]) || tokens[idx].startsWith('-'))) {
      idx++;
    }
  }
  if (idx >= tokens.length) return false;

  const baseCmd = tokens[idx];

  // Case 1: Direct script execution
  if (baseCmd === 'finalize.sh' || baseCmd.endsWith('/finalize.sh')) {
    return true;
  }

  // Case 2: Shell execution (bash finalize.sh ..., sh finalize.sh ...)
  if (baseCmd === 'bash' || baseCmd === 'sh' || baseCmd === 'zsh' || baseCmd.endsWith('/bash') || baseCmd.endsWith('/sh') || baseCmd.endsWith('/zsh')) {
    for (let j = idx + 1; j < tokens.length; j++) {
      const arg = tokens[j];
      if (arg === 'finalize.sh' || arg.endsWith('/finalize.sh')) {
        return true;
      }
    }
  }

  // Case 3: eval / exec invocation
  if (baseCmd === 'eval' || baseCmd === 'exec') {
    for (let j = idx + 1; j < tokens.length; j++) {
      if (tokens[j].includes('finalize.sh')) {
        return true;
      }
    }
  }

  return false;
}

function checkGitSafety(cmd, transcriptPath) {
  if (!cmd || typeof cmd !== 'string') return null;

  const segments = splitCompoundCommands(cmd);

  for (const segment of segments) {
    const tokens = tokenize(segment);
    if (tokens.length === 0) continue;

    // Filter leading environment variables
    let idx = 0;
    while (idx < tokens.length && /^[A-Za-z_][A-Za-z0-9_]*=/.test(tokens[idx])) {
      idx++;
    }
    if (idx >= tokens.length) continue;

    // Handle wrappers like sudo, env, builtin, command
    while (idx < tokens.length && (tokens[idx] === 'sudo' || tokens[idx] === 'env' || tokens[idx] === 'command' || tokens[idx] === 'builtin')) {
      idx++;
      while (idx < tokens.length && (/^[A-Za-z_][A-Za-z0-9_]*=/.test(tokens[idx]) || tokens[idx].startsWith('-'))) {
        idx++;
      }
    }
    if (idx >= tokens.length) continue;

    const baseCmd = tokens[idx];

    // 1. Check if this is a finalize.sh execution
    if (isFinalizeInvocation(tokens)) {
      if (!isOwnerFinalizeAuthorized(transcriptPath)) {
        return {
          decision: 'deny',
          reason: `Blocked by ZAMK Git Safety Hook: Canonical 'finalize.sh' is locked. Finalization requires explicit Product Owner acceptance authorization ('${AUTH_TOKEN}') in the latest user request.`
        };
      }
      // If authorized, finalize.sh is allowed!
      continue;
    }

    // 2. Check if the command is direct git (or /usr/bin/git, etc.)
    if (baseCmd === 'git' || baseCmd.endsWith('/git')) {
      idx++;
      // Skip git global flags
      while (idx < tokens.length) {
        const tok = tokens[idx];
        if (tok === '-C' || tok === '-c' || tok === '--git-dir' || tok === '--work-tree' || tok === '--namespace' || tok === '--super-prefix' || tok === '--exec-path') {
          idx += 2;
        } else if (tok.startsWith('-')) {
          idx++;
        } else {
          break;
        }
      }

      if (idx < tokens.length) {
        const subCmd = tokens[idx].toLowerCase();

        // Staging mutations -> ALWAYS DENY direct git add/stage
        if (subCmd === 'add' || subCmd === 'stage') {
          return {
            decision: 'deny',
            reason: `Blocked by ZAMK Git Safety Hook: Direct 'git ${subCmd}' is prohibited. All staging must occur strictly via canonical 'zamk-finalize' (finalize.sh) with explicit Product Owner authorization.`
          };
        }

        // Direct commit -> ALWAYS DENY direct git commit
        if (subCmd === 'commit') {
          return {
            decision: 'deny',
            reason: "Blocked by ZAMK Git Safety Hook: Direct 'git commit' is prohibited. Commits must only be created via canonical 'zamk-finalize' (finalize.sh) with explicit Product Owner authorization."
          };
        }

        // Direct push -> ALWAYS DENY direct git push
        if (subCmd === 'push') {
          return {
            decision: 'deny',
            reason: "Blocked by ZAMK Git Safety Hook: Direct 'git push' is prohibited. Push operations must only be executed via canonical 'zamk-finalize' (finalize.sh) with explicit Product Owner authorization."
          };
        }

        // Direct merge / cherry-pick / rebase -> ALWAYS DENY
        if (subCmd === 'merge' || subCmd === 'cherry-pick' || subCmd === 'rebase') {
          return {
            decision: 'deny',
            reason: `Blocked by ZAMK Git Safety Hook: Direct 'git ${subCmd}' is prohibited in agent workflow.`
          };
        }
      }
    }

    // 3. Check for nested subshells like bash -c "git add ..." or eval "git commit ..."
    if (baseCmd === 'bash' || baseCmd === 'sh' || baseCmd === 'zsh' || baseCmd === 'eval' || baseCmd.endsWith('/bash') || baseCmd.endsWith('/sh') || baseCmd.endsWith('/zsh')) {
      for (let j = idx + 1; j < tokens.length; j++) {
        const innerTok = tokens[j];
        if (innerTok.endsWith('finalize.sh') || innerTok.endsWith('preflight.sh')) {
          continue;
        }
        if (innerTok.includes('git ') || innerTok.startsWith('git')) {
          const innerResult = checkGitSafety(innerTok, transcriptPath);
          if (innerResult) return innerResult;
        }
      }
    }
  }

  return null;
}

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
    const transcriptPath = data.transcriptPath || '';

    // 1. Check DB Safety
    const dbResult = checkDbSafety(cmd);
    if (dbResult) {
      console.log(JSON.stringify(dbResult));
      process.exit(0);
    }

    // 2. Check Git Safety with Transcript Authorization Check
    const gitResult = checkGitSafety(cmd, transcriptPath);
    if (gitResult) {
      console.log(JSON.stringify(gitResult));
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

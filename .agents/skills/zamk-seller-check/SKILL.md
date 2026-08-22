---
name: zamk-seller-check
description: Standard Seller frontend technical check. Use this to verify seller frontend build and source checks.
---

# zamk-seller-check

Purpose: standard Seller frontend technical check.

Steps:

run `npm run build --prefix apps/seller`

Then search Product/Seller source when relevant for:
alert(
confirm(
beforeunload
example.com
Math.random
"В разработке"

Use grep/rg.
Do not treat unrelated historical matches outside the changed feature as failure; report them separately.

Whenever a workflow runs terminal commands, execute EACH command as a separate terminal tool invocation. Do NOT combine commands using: ;, &&, ||.

Return:

SELLER BUILD:
PASS/FAIL

TASK-RELATED NATIVE ALERTS:
<count>

TASK-RELATED NATIVE CONFIRMS:
<count>

TASK-RELATED FAKE/PLACEHOLDER MATCHES:
<count>

---
name: zamk-finalize
description: Finalize an already Product Owner-accepted ZAMK milestone by exact-staging approved files and pushing safely.
---

# ZAMK Finalize

Use strictly after explicit Product Owner acceptance.

## Authorization Requirement
The latest Product Owner message must explicitly include the standalone authorization line:
`ZAMK_OWNER_ACCEPTED_FINALIZE=YES`

The pre-tool guard automatically verifies this authorization in the conversation transcript before allowing execution of `finalize.sh`.

## Execution
Run `bash .agents/scripts/finalize.sh "<commit message>" <file1> <file2> ...` with the exact approved files.
This will automatically verify the branch, stage only the requested paths, run a diff check, commit with your provided commit message, and push to main.

Stop and report if the script fails. Do not proceed with manual git commands if the script halts.

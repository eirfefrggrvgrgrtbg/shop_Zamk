---
name: zamk-finalize
description: Finalize an already Product Owner-accepted ZAMK milestone by exact-staging approved files and pushing safely.
---

# ZAMK Finalize

Use only after Product Owner acceptance.

Run `bash .agents/scripts/finalize.sh "<commit message>" <file1> <file2> ...` with the exact approved files.
This will automatically verify the branch, stage only the requested paths, run a diff check, commit with your provided short Russian message, and push to main.

Stop and report if the script fails. Do not proceed with manual git commands if the script halts.

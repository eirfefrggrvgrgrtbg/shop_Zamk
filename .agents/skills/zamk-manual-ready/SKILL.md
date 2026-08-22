---
name: zamk-manual-ready
description: Prepare environment for manual Product Owner review without visual acceptance.
---

# zamk-manual-ready

Purpose: prepare environment for manual Product Owner review without visual acceptance.

Steps:

1. Verify current HEAD/worktree.
2. Check required local services: PostgreSQL, Redis, MinIO when media is relevant, backend, Seller frontend.
3. Start only missing services using existing project conventions.
4. Do not reset databases.
5. Verify backend responds.
6. Verify Seller responds on its actual local URL.
7. If Product media is in scope: verify backend storage provider initialized successfully, not merely that MinIO container is running.
8. Do NOT create/modify Product business state just to claim readiness.
9. Do NOT perform visual acceptance.
10. Whenever a workflow runs terminal commands, execute EACH command as a separate terminal tool invocation. Do NOT combine commands using: ;, &&, ||.

Return:

HEAD:
...

WORKTREE CLEAN:
YES/NO

POSTGRES:
RUNNING/FAIL

REDIS:
RUNNING/FAIL

MINIO:
RUNNING/NOT REQUIRED/FAIL

BACKEND STORAGE:
INITIALIZED/NOT REQUIRED/FAIL

BACKEND:
RUNNING/FAIL

SELLER:
RUNNING/FAIL

SELLER URL:
...

READY FOR PRODUCT OWNER:
YES/NO

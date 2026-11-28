#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-full}"

case "$mode" in
  full | fast) ;;
  *)
    echo "Usage: $0 [full|fast]" >&2
    exit 2
    ;;
esac

cd "$root"

echo "[marketing-upstream] typecheck"
pnpm --dir frontend run typecheck

echo "[marketing-upstream] frontend reward flows"
pnpm --dir frontend exec vitest run \
  src/components/user/dashboard/__tests__/RewardActionPanel.spec.ts \
  src/components/user/dashboard/__tests__/ConsumptionRewards.spec.ts \
  src/components/user/dashboard/__tests__/UserDashboardActivity.spec.ts \
  src/components/user/dashboard/__tests__/UserLotteryModal.spec.ts \
  src/components/admin/payment/__tests__/AdminRewardMoments.spec.ts

echo "[marketing-upstream] backend contracts"
python3 -B -m unittest scripts.test_marketing_gate
(
  cd backend
  go build ./...
)

if [[ "$mode" == "full" ]]; then
  echo "[marketing-upstream] isolated backend contracts, login and HTTP flow"
  python3 scripts/test-activity-rewards-http.py --with-contracts
else
  python3 scripts/test-activity-rewards-http.py --contracts-only
fi

if rg -n -g 'activity_*.go' -g 'consumption_rewards.go' -g '!*_test.go' \
  '\b(FROM|JOIN|UPDATE|INSERT INTO)[[:space:]]+(users|usage_logs|payment_orders)\b' \
  backend/internal/service; then
  echo "[marketing-upstream] activity services must use the core data adapter" >&2
  exit 1
fi

git diff --check
echo "[marketing-upstream] passed ($mode)"

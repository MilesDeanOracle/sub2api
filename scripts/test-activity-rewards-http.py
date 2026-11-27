#!/usr/bin/env python3
"""Real login/HTTP/SQL regression in disposable PostgreSQL and Redis containers.

Run: python3 scripts/test-activity-rewards-http.py [--binary /path/to/server]
Add --with-contracts to run backend contracts before HTTP, or --contracts-only
to run only backend contracts in disposable PostgreSQL.
No existing database, Redis instance, .env or server is used.
"""

import argparse
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid


ROOT = Path(__file__).resolve().parents[1]
CONTRACT_PATTERN = "^Test(Activity|Checkin|Consumption|DrawLottery|Lottery|RewardMoments|Regression|Migration259)"
REQUIRED_CONTRACTS = {
    ("handler", "TestActivityCorrectionSignedEventID"),
    ("repository", "TestActivityMigrationLineagesPostgres"),
    ("repository", "TestActivityMaintenanceGate"),
    ("service", "TestActivityProductionPostgres"),
    ("service", "TestActivityPaymentProductionPostgres"),
    ("service", "TestActivityRefundUncertainResponses"),
    ("service", "TestActivityWalletExperiencePostgres"),
    ("service", "TestConsumptionPostgres"),
    ("service", "TestConsumptionMasterSwitchPostgres"),
    ("service", "TestConsumptionMasterSwitchConcurrentPostgres"),
    ("service", "TestDrawLotteryBudgetPauseNeverSilentlyAltersOdds"),
    ("service", "TestDrawLotteryIdempotentReplayReturnsSavedBatch"),
    ("service", "TestRegressionOutOfOrderUsageCommitPostgres"),
    ("service", "TestRegressionTransferMustNotAbsorbEarlierUsagePostgres"),
    ("service", "TestRegressionConsumptionPendingAndSignedEventsPostgres"),
    ("service", "TestRegressionRefundQueryUsesGatewayAmount"),
    ("service", "TestRegressionRefundWithoutDeductionRequiresForcePostgres"),
    ("service", "TestRewardMomentsPrivacyPostgres"),
    ("service", "TestRewardMomentsRevocationPostgres"),
    ("service", "TestRewardMomentsConsumptionPostgres"),
    ("service", "TestRewardMomentsConfigPostgres"),
    ("service", "TestRewardMomentsEarliestClaimPostgres"),
}


def run(*args, **kwargs):
    try:
        return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs).stdout.strip()
    except subprocess.CalledProcessError as error:
        if error.stderr:
            print(error.stderr.strip(), file=sys.stderr, flush=True)
        raise


def validate_contract_events(events):
    passed = set()
    for event in events:
        package = event["Package"].rsplit("/", 1)[-1]
        test = event.get("Test", "")
        if event["Action"] in ("skip", "fail"):
            raise RuntimeError(f"backend contract {event['Action']}: {package}/{test}")
        if event["Action"] == "pass" and test and "/" not in test:
            passed.add((package, test))
    missing = REQUIRED_CONTRACTS - passed
    if missing:
        names = ", ".join(f"{package}/{test}" for package, test in sorted(missing))
        raise RuntimeError("required backend contracts did not pass: " + names)
    return len(passed)


def run_contracts(dsn, password):
    print("Running backend contracts in disposable PostgreSQL", flush=True)
    result = subprocess.run(
        ["go", "test", "-json", "-tags", "unit", "./internal/handler", "./internal/service",
         "./internal/repository", "./internal/server/routes", "-run", CONTRACT_PATTERN,
         "-count=1", "-timeout=8m"],
        cwd=ROOT / "backend", env=dict(os.environ, ACTIVITY_TEST_DSN=dsn),
        capture_output=True, text=True,
    )
    try:
        events = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
        if result.returncode:
            raise RuntimeError(f"backend contract command exited with {result.returncode}")
        count = validate_contract_events(events)
    except (ValueError, RuntimeError):
        print((result.stdout + result.stderr).replace(password, "<REDACTED>"), file=sys.stderr, flush=True)
        raise
    print(f"PASS: {count} backend contract tests, no skips; all required contracts passed", flush=True)


def wait_until(check, process=None):
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        if process is not None and process.poll() is not None:
            raise AssertionError("test server exited before becoming healthy")
        try:
            if check():
                return
        except (OSError, urllib.error.URLError, subprocess.CalledProcessError):
            pass
        time.sleep(0.25)
    raise AssertionError("isolated test service did not become ready within 120 seconds")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path)
    contracts = parser.add_mutually_exclusive_group()
    contracts.add_argument("--with-contracts", action="store_true")
    contracts.add_argument("--contracts-only", action="store_true")
    args = parser.parse_args()
    # Fail with Docker's actual diagnostic before compiling or running any tests.
    run("docker", "info", "--format", "{{.ServerVersion}}", timeout=15)
    containers = []
    process = None
    with tempfile.TemporaryDirectory(prefix="activity-http-") as tmp:
        tmp = Path(tmp)
        binary = args.binary.resolve() if args.binary else tmp / "server"
        password = secrets.token_urlsafe(32)
        suffix = uuid.uuid4().hex[:12]
        pg, redis = "activity-http-pg-" + suffix, "activity-http-redis-" + suffix
        server_env = {}
        try:
            env = dict(os.environ, POSTGRES_PASSWORD=password)
            run("docker", "run", "-d", "--rm", "--name", pg,
                "-e", "POSTGRES_PASSWORD", "-e", "POSTGRES_USER=rewards",
                "-e", "POSTGRES_DB=rewards", "-p", "127.0.0.1::5432",
                "postgres:16-alpine", env=env)
            containers.append(pg)
            wait_until(lambda: run("docker", "exec", pg, "pg_isready", "-h", "127.0.0.1", "-U", "rewards", "-d", "rewards"))
            pg_port = run("docker", "port", pg, "5432/tcp").rsplit(":", 1)[1]
            if args.with_contracts or args.contracts_only:
                dsn = f"postgresql://rewards:{password}@127.0.0.1:{pg_port}/rewards?sslmode=disable"
                run_contracts(dsn, password)
            if args.contracts_only:
                return
            if args.binary is None:
                print("Building backend", flush=True)
                run("go", "build", "-o", str(binary), "./cmd/server", cwd=ROOT / "backend")
            run("docker", "run", "-d", "--rm", "--name", redis,
                "-p", "127.0.0.1::6379", "redis:7-alpine")
            containers.append(redis)
            redis_port = run("docker", "port", redis, "6379/tcp").rsplit(":", 1)[1]
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                port = sock.getsockname()[1]
            server_env = dict(os.environ, DATA_DIR=str(tmp), CONFIG_FILE=str(tmp / "config.yaml"),
                              DATABASE_HOST="127.0.0.1", DATABASE_PORT=pg_port,
                              DATABASE_USER="rewards", DATABASE_PASSWORD=password,
                              DATABASE_DBNAME="rewards", DATABASE_SSLMODE="disable",
                              REDIS_HOST="127.0.0.1", REDIS_PORT=redis_port,
                              REDIS_USERNAME="", REDIS_PASSWORD="", REDIS_DB="0",
                              AUTO_SETUP="true", SERVER_HOST="127.0.0.1", SERVER_PORT=str(port),
                              ADMIN_EMAIL="admin@example.com", ADMIN_PASSWORD=password,
                              JWT_SECRET=secrets.token_hex(32), TOTP_ENCRYPTION_KEY=secrets.token_hex(32),
                              ACTIVITY_MAINTENANCE_CONFIRMED="", RUN_MODE="standard", TZ="Asia/Shanghai")
            # CONFIG_FILE is created by auto-setup before normal startup loads it.
            with (tmp / "server.log").open("w") as log:
                process = subprocess.Popen([str(binary)], env=server_env, cwd=tmp,
                                           stdout=log, stderr=subprocess.STDOUT)
                base = "http://127.0.0.1:" + str(port)
                wait_until(lambda: urllib.request.urlopen(base + "/health", timeout=2).status == 200, process)

                def sql(statement):
                    return run("docker", "exec", "-i", pg, "psql", "-X", "-At", "-v",
                               "ON_ERROR_STOP=1", "-U", "rewards", "-d", "rewards", input=statement)

                def call(path, token=None, body=None, method=None, expected=200):
                    headers = {"Content-Type": "application/json", "User-Agent": "activity-http-regression"}
                    if token:
                        headers["Authorization"] = "Bearer " + token
                    req = urllib.request.Request(base + "/api/v1" + path, headers=headers,
                                                 data=None if body is None else json.dumps(body).encode(),
                                                 method=method)
                    try:
                        response = urllib.request.urlopen(req, timeout=15)
                    except urllib.error.HTTPError as error:
                        response = error
                    raw = response.read()
                    assert response.code == expected, (method, path, response.code, raw.decode())
                    payload = json.loads(raw)
                    return payload.get("data", payload), response.headers

                def login(email):
                    data, _ = call("/auth/login", body={"email": email, "password": password})
                    return data["access_token"]

                admin = login("admin@example.com")
                compliance, _ = call("/admin/compliance", admin)
                call("/admin/compliance/accept", admin,
                     {"phrase": compliance["ack_phrase_en"], "language": "en"})
                sql("""INSERT INTO users(id,email,password_hash,role,balance)
                    SELECT 2,'member@example.com',password_hash,'user',100 FROM users WHERE id=1;
                    INSERT INTO users(id,email,password_hash,role,balance)
                    SELECT 3,'viewer@example.com',password_hash,'user',100 FROM users WHERE id=1;
                    INSERT INTO activity_first_calls(user_id,request_id) VALUES(2,'http-fixture');""")
                member, viewer = login("member@example.com"), login("viewer@example.com")
                call("/activity/reward-moments", expected=401)
                call("/admin/activity/reward-moments", member, expected=403)
                config, _ = call("/admin/activity/reward-moments", admin)
                assert config == {"enabled": False, "user_ids": []}, config
                sharing, _ = call("/activity/reward-sharing", member)
                assert sharing["allowed"] is False
                for kind in ("checkin", "consumption", "lottery"):
                    call("/admin/activity/audience/" + kind, admin,
                         {"mode": "all", "user_ids": []}, "PUT")
                config, _ = call("/admin/activity/config", admin)
                config.update(checkin_enabled=True, checkin_require_recharge=False,
                              checkin_rollout_percent=100, checkin_min_spend_cents=0,
                              checkin_daily_cents=25, checkin_monthly_budget_cents=10000,
                              user_lifetime_cap_cents=10000)
                call("/admin/activity/config", admin, config, "PUT")
                call("/admin/activity/consumption/config", admin, {"enabled": True}, "PUT")
                call("/admin/activity/reward-moments", admin,
                     {"enabled": True, "user_ids": [2, 3]}, "PUT")
                lottery, _ = call("/admin/activity/lottery", admin)
                lottery.update(enabled=True, budget_cents=100000)
                for tier in lottery["tiers"]:
                    tier["reward_cents"] = 100
                call("/admin/activity/lottery", admin, lottery, "PUT")
                sql("""INSERT INTO payment_orders(id,user_id,amount,pay_amount,status,order_type,paid_at,completed_at,expires_at)
                    VALUES(100,2,100,20,'COMPLETED','balance',NOW(),NOW(),NOW()+interval '1 day');
                    INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at)
                    VALUES(1,'First campaign',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 day'),
                          (2,'Later campaign',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '4 days',100,10000,'active',NOW()-interval '1 day');
                    INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents)
                    VALUES(1,1,100,50),(2,1,100,100);
                    INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents)
                    VALUES(1,2,1,50),(2,2,1,100);""")
                # Existing check-in API awards before consent, never backfilled.
                call("/activity/checkin", member, {}, "POST")
                feed, _ = call("/activity/reward-moments", viewer)
                assert feed["items"] == []
                call("/activity/reward-sharing", member, {"allowed": True}, "PUT")
                status, _ = call("/activity/consumption", member)
                assert status["campaign"]["id"] == 1 and status["claimable_cents"] == 50, status
                call("/activity/consumption/claim", member, {"campaign_id": 1, "position": 0})
                call("/activity/consumption/claim", member, {"campaign_id": 1, "position": 0})
                status, _ = call("/activity/consumption", member)
                assert status["campaign"]["id"] == 2 and status["claimable_cents"] == 100, status
                lottery, _ = call("/activity/lottery", member)
                assert len(lottery["tiers"]) == 8 and lottery["remaining_draws"] == 2, lottery
                draw_body = {"count": 1, "idempotency_key": "http-regression-draw"}
                draw, _ = call("/activity/lottery/draw", member, draw_body)
                retry, _ = call("/activity/lottery/draw", member, draw_body)
                assert draw["items"] == retry["items"]
                assert draw["total_reward_cents"] == 100 and draw["status"]["remaining_draws"] == 1, draw
                feed, headers = call("/activity/reward-moments", viewer)
                assert headers["Cache-Control"] == "private, no-store"
                assert sorted(item["amount_cents"] for item in feed["items"]) == [50, 100], feed
                for private in ("user_id", "source_id", "ledger_id", "email", "balance", "alias_secret"):
                    assert private not in json.dumps(feed), private
                wallet, _ = call("/activity/wallet", member)
                assert wallet["available_cents"] == 175, wallet
                withdrawn, _ = call("/activity/wallet/withdraw", member, {})
                assert withdrawn["withdrawn_cents"] == 175 and withdrawn["available_cents"] == 0, withdrawn
                call("/activity/wallet/withdraw", member, {}, expected=400)
                assert sql("SELECT balance FROM users WHERE id=2") == "101.75000000"
                feed, _ = call("/activity/reward-moments", viewer)
                assert len(feed["items"]) == 2  # Transfers are never announcements.
                call("/activity/reward-sharing", member, {"allowed": False}, "PUT")
                feed, _ = call("/activity/reward-moments", viewer)
                assert feed["items"] == []
                call("/activity/reward-sharing", member, {"allowed": True}, "PUT")
                feed, _ = call("/activity/reward-moments", viewer)
                assert feed["items"] == []  # Re-consent cannot republish old rewards.
                call("/admin/activity/reward-moments", admin, {"enabled": False, "user_ids": [2, 3]}, "PUT")
                feed, _ = call("/activity/reward-moments", viewer)
                assert feed["enabled"] is False and feed["items"] == []
                # The feed switch does not disable personal money actions.
                call("/activity/consumption/claim", member, {"campaign_id": 2, "position": 0})
                wallet, _ = call("/activity/wallet", member)
                assert wallet["available_cents"] == 100
                call("/admin/activity/reward-moments", admin, {"enabled": True, "user_ids": [2]}, "PUT")
                feed, _ = call("/activity/reward-moments", viewer)
                assert feed["enabled"] is False and feed["items"] == []
                call("/activity/reward-sharing", member, {}, "PUT", expected=400)
                call("/admin/activity/reward-moments", admin, {"enabled": True, "user_ids": [99999]}, "PUT", expected=400)
                print("PASS: real login, auth boundaries, independent configs, earliest claim, claim/draw retries, wallet transfer, consent/revocation, feed privacy and admin switches", flush=True)
        except Exception:
            if (tmp / "server.log").exists():
                lines = (tmp / "server.log").read_text(errors="replace").splitlines()
                errors = [line for line in lines if '"ERROR"' in line or "failed:" in line]
                for line in (errors[-5:] or lines[-5:]):
                    for secret in (password, server_env.get("JWT_SECRET", ""), server_env.get("TOTP_ENCRYPTION_KEY", "")):
                        if secret:
                            line = line.replace(secret, "<REDACTED>")
                    print(line, flush=True)
            raise
        finally:
            if process is not None and process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            for container in reversed(containers):
                subprocess.run(["docker", "rm", "-f", container], capture_output=True)


if __name__ == "__main__":
    main()

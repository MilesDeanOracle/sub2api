from contextlib import redirect_stderr, redirect_stdout
from io import StringIO
import json
from pathlib import Path
import runpy
from types import SimpleNamespace
import unittest
from unittest.mock import patch


GATE = runpy.run_path(str(Path(__file__).resolve().parent / "test-activity-rewards-http.py"))


class MarketingGateTests(unittest.TestCase):
    def setUp(self):
        self.passes = [
            {"Action": "pass", "Package": "example/internal/" + package, "Test": test}
            for package, test in sorted(GATE["REQUIRED_CONTRACTS"])
        ]

    def test_required_contracts_all_match_selection(self):
        for _, test in GATE["REQUIRED_CONTRACTS"]:
            with self.subTest(test=test):
                self.assertRegex(test, GATE["CONTRACT_PATTERN"])

    def test_accepts_completed_required_contracts(self):
        self.assertEqual(len(self.passes), GATE["validate_contract_events"](self.passes))

    def test_rejects_database_tests_skipped_under_a_passing_parent(self):
        event = {"Action": "skip", "Package": "example/internal/service",
                 "Test": "TestActivityProductionPostgres/refund_rollback"}
        with self.assertRaisesRegex(RuntimeError, "skip"):
            GATE["validate_contract_events"]([event] + self.passes)

    def test_rejects_no_tests_selected(self):
        with self.assertRaisesRegex(RuntimeError, "did not pass"):
            GATE["validate_contract_events"]([])

    def test_rejects_a_required_contract_missing_from_otherwise_passing_output(self):
        with self.assertRaisesRegex(RuntimeError, "did not pass"):
            GATE["validate_contract_events"](self.passes[1:])

    def test_rejects_a_failed_test(self):
        event = {"Action": "fail", "Package": "example/internal/service", "Test": "TestActivityRefundUncertainResponses"}
        with self.assertRaisesRegex(RuntimeError, "fail"):
            GATE["validate_contract_events"](self.passes + [event])

    def test_runner_overrides_inherited_dsn_with_disposable_database(self):
        result = SimpleNamespace(returncode=0, stdout="\n".join(map(json.dumps, self.passes)), stderr="")
        with patch.dict(GATE["os"].environ, {"ACTIVITY_TEST_DSN": "existing-database"}), \
                patch.object(GATE["subprocess"], "run", return_value=result) as command, \
                redirect_stdout(StringIO()):
            GATE["run_contracts"]("disposable-database", "secret")
        self.assertEqual("disposable-database", command.call_args.kwargs["env"]["ACTIVITY_TEST_DSN"])

    def test_runner_rejects_command_failure_despite_pass_events(self):
        result = SimpleNamespace(returncode=1, stdout="\n".join(map(json.dumps, self.passes)), stderr="build failed")
        with patch.object(GATE["subprocess"], "run", return_value=result), \
                redirect_stdout(StringIO()), redirect_stderr(StringIO()), \
                self.assertRaisesRegex(RuntimeError, "command exited with 1"):
            GATE["run_contracts"]("disposable-database", "secret")

    def test_runner_rejects_skips_and_redacts_database_password(self):
        event = {"Action": "skip", "Package": "example/internal/service", "Test": "TestConsumptionPostgres"}
        result = SimpleNamespace(returncode=0, stdout=json.dumps(event), stderr="dsn password=secret")
        diagnostic = StringIO()
        with patch.object(GATE["subprocess"], "run", return_value=result), \
                redirect_stdout(StringIO()), redirect_stderr(diagnostic), \
                self.assertRaisesRegex(RuntimeError, "skip"):
            GATE["run_contracts"]("disposable-database", "secret")
        self.assertNotIn("secret", diagnostic.getvalue())
        self.assertIn("<REDACTED>", diagnostic.getvalue())

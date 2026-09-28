import fcntl
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import textwrap
import time
import unittest

WORKER = pathlib.Path(__file__).resolve().parent / "worker.sh"

HARNESS = textwrap.dedent("""\
    #!/bin/sh
    # Stand-in for the headless harness: the prompt's first line is the session id.
    read -r session
    echo "$session" >> "$STUB_DIR/harness-calls"
    printf '{"type":"system","session_id":"%s"}\\n{"type":"result","result":"done %s"}\\n' "$session" "$session"
""")
RESUME = textwrap.dedent("""\
    #!/bin/sh
    echo "$*" >> "$STUB_DIR/resume-calls"
""")


@unittest.skipUnless(shutil.which("tmux"), "needs tmux")
class Worker(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp())
        (self.tmp / "bin").mkdir()
        for name, body in (("harness", HARNESS), ("resume", RESUME), ("bin/claude", RESUME)):
            (self.tmp / name).write_text(body)
            (self.tmp / name).chmod(0o755)
        (self.tmp / "tmux").mkdir()
        (self.tmp / "home").mkdir()
        (self.tmp / "work").mkdir()
        self.run_dir = self.tmp / "run"
        env = {k: v for k, v in os.environ.items() if k != "TMUX"}
        self.env = {**env, "PATH": f"{self.tmp / 'bin'}:{env['PATH']}", "SLOPMACHINE_RUN": str(self.run_dir), "SLOPMACHINE_HARNESS": str(self.tmp / "harness"),
                    "SLOPMACHINE_RESUME": str(self.tmp / "resume"), "SLOPMACHINE_MAX_WORKERS": "5",
                    "STUB_DIR": str(self.tmp), "TMUX_TMPDIR": str(self.tmp / "tmux"), "HOME": str(self.tmp / "home"),
                    "SHELL": "/bin/sh"}

    def tearDown(self):
        subprocess.run(["tmux", "kill-server"], env=self.env, capture_output=True)
        shutil.rmtree(self.tmp)

    def worker(self, *args):
        return subprocess.run(["bash", WORKER, *args], env=self.env, text=True, capture_output=True, timeout=30)

    def start(self, worker_id, session, attempt=None, cwd=None):
        prompt = self.tmp / f"prompt-{session}"
        prompt.write_text(session + "\n")
        run = self.worker("start", worker_id, str(cwd or self.tmp / "work"), str(prompt), *([attempt] if attempt else []))
        self.assertEqual(run.returncode, 0, run.stderr)
        return run

    def wait_exit(self, worker_id):
        for _ in range(100):
            current = self.run_dir / worker_id / "current"
            if current.exists() and (self.run_dir / worker_id / current.read_text().strip() / "exit").exists():
                return
            time.sleep(0.1)
        self.fail(f"{worker_id} never exited")

    def status(self):
        return dict(line.split(" ", 1) for line in self.worker("status").stdout.splitlines())

    def calls(self, name):
        path = self.tmp / name
        return path.read_text().split("\n")[:-1] if path.exists() else []

    def test_ids_that_escape_the_run_directory_are_refused(self):
        sibling = self.tmp / "keep"
        sibling.mkdir()
        self.run_dir.mkdir()
        for args in (("stop", "../keep"), ("stop", ".."), ("status", "../"), ("log", "a/b"), ("resume", ".x", "hi"),
                     ("start", ".hidden", str(self.tmp), str(self.tmp / "harness")),
                     ("start", "ok", str(self.tmp), str(self.tmp / "harness"), "../up")):
            run = self.worker(*args)
            self.assertEqual(run.returncode, 2, args)
            self.assertIn("invalid", run.stderr)
        self.assertTrue(sibling.exists())

    def test_targets_match_the_exact_session_name(self):
        subprocess.run(["tmux", "new-session", "-d", "-s", "slop-w-16101", "sleep 60"], env=self.env, check=True)
        (self.run_dir / "w-161").mkdir(parents=True)
        self.assertEqual(self.status()["w-161"], "dead")
        self.worker("stop", "w-161")
        alive = subprocess.run(["tmux", "has-session", "-t", "=slop-w-16101"], env=self.env)
        self.assertEqual(alive.returncode, 0)
        self.start("w-161", "s1")

    def test_failed_cd_never_launches_the_harness(self):
        self.start("w-7", "s1", cwd=self.tmp / "missing")
        self.wait_exit("w-7")
        self.assertEqual(self.calls("harness-calls"), [])
        self.assertEqual(self.status()["w-7"], "startup-failed")
        self.assertIn("cannot cd", self.worker("log", "w-7").stdout)

    def test_each_attempt_has_its_own_log_and_resume_picks_the_current_one(self):
        self.start("w-9", "first")
        self.wait_exit("w-9")
        self.start("w-9", "second")
        self.wait_exit("w-9")
        self.assertEqual(self.worker("log", "w-9").stdout.strip(), "done second")
        self.assertEqual(self.worker("resume", "w-9", "post your report").returncode, 0)
        self.wait_exit("w-9")
        self.assertEqual(self.calls("resume-calls"), ["second"])
        self.assertEqual(self.worker("start", "w-9", str(self.tmp), str(self.tmp / "harness"),
                                     (self.run_dir / "w-9" / "current").read_text().strip()).returncode, 1)

    def test_stop_with_an_older_attempt_leaves_the_current_one(self):
        self.start("w-5", "first", attempt="a1")
        self.wait_exit("w-5")
        self.start("w-5", "second", attempt="a2")
        self.wait_exit("w-5")
        run = self.worker("stop", "w-5", "a1")
        self.assertEqual(run.returncode, 3)
        self.assertTrue((self.run_dir / "w-5" / "a2").exists())
        self.assertEqual(self.worker("stop", "w-5", "a2").returncode, 0)
        self.assertFalse((self.run_dir / "w-5").exists())

    def test_stop_removes_only_its_own_attempt(self):
        self.start("w-4", "first", attempt="a1")
        self.wait_exit("w-4")
        (self.run_dir / "w-4" / "a2").mkdir()
        self.assertEqual(self.worker("stop", "w-4", "a1").returncode, 0)
        self.assertEqual(sorted(p.name for p in (self.run_dir / "w-4").iterdir()), ["a2"])

    def test_stop_waits_for_a_start_of_the_same_id(self):
        self.start("w-3", "first", attempt="a1")
        self.wait_exit("w-3")
        self.env["SLOPMACHINE_WORKER_LOCK_WAIT"] = "0.3"
        with open(self.run_dir / ".w-3.lock", "a") as held:
            fcntl.flock(held, fcntl.LOCK_EX)
            run = self.worker("stop", "w-3", "a1")
        self.assertNotEqual(run.returncode, 0)
        self.assertIn("another session", run.stderr)
        self.assertTrue((self.run_dir / "w-3" / "a1").exists())
        self.assertEqual(self.worker("stop", "w-3", "a1").returncode, 0)

    def test_a_start_waits_for_another_admission(self):
        self.env["SLOPMACHINE_WORKER_LOCK_WAIT"] = "0.3"
        self.run_dir.mkdir()
        prompt = self.tmp / "prompt"
        prompt.write_text("s1\n")
        with open(self.run_dir / ".admission.lock", "a") as held:
            fcntl.flock(held, fcntl.LOCK_EX)
            run = self.worker("start", "w-1", str(self.tmp / "work"), str(prompt), "a1")
        self.assertNotEqual(run.returncode, 0)
        self.assertIn("another session", run.stderr)
        self.assertEqual(self.calls("harness-calls"), [])

    def test_a_failing_harness_reports_its_exit_code_under_an_errexit_profile(self):
        (self.tmp / "home" / ".profile").write_text("set -e\n")
        failing = self.tmp / "failing-harness"
        failing.write_text("#!/bin/sh\nexit 3\n")
        failing.chmod(0o755)
        self.env["SLOPMACHINE_HARNESS"] = str(failing)
        self.start("w-2", "s1", attempt="a1")
        self.wait_exit("w-2")
        self.assertEqual(self.status()["w-2"], "exit=3")

    def test_the_run_directory_is_private(self):
        self.start("w-3", "s1")
        self.assertEqual(self.run_dir.stat().st_mode & 0o777, 0o700)

    def test_a_failed_launch_is_a_startup_failure(self):
        failing = self.tmp / "failing"
        failing.mkdir()
        (failing / "tmux").write_text(f'#!/bin/sh\n[ "$1" = new-session ] && exit 1\nexec {shutil.which("tmux")} "$@"\n')
        (failing / "tmux").chmod(0o755)
        self.env["PATH"] = f"{failing}:{self.env['PATH']}"
        prompt = self.tmp / "prompt"
        prompt.write_text("s1\n")
        run = self.worker("start", "w-2", str(self.tmp / "work"), str(prompt), "a1")
        self.assertEqual(run.returncode, 1)
        self.assertEqual(self.status()["w-2"], "startup-failed")
        self.assertEqual((self.run_dir / "w-2" / "a1" / "exit").read_text().strip(), "1")

    def test_resume_reads_a_long_log_and_takes_the_latest_session(self):
        self.start("w-8", "s1", attempt="a1")
        self.wait_exit("w-8")
        log = self.run_dir / "w-8" / "a1" / "log.jsonl"
        with log.open("a") as f:
            for _ in range(20000):
                f.write(json.dumps({"type": "assistant", "session_id": "s1"}) + "\n")
            f.write(json.dumps({"type": "system", "session_id": "s2"}) + "\n")
        run = self.worker("resume", "w-8", "post your report")
        self.assertEqual(run.returncode, 0, run.stderr)
        self.wait_exit("w-8")
        self.assertEqual(self.calls("resume-calls"), ["s2"])

    def test_resume_without_a_session_fails_clearly(self):
        self.start("w-6", "s1", attempt="a1")
        self.wait_exit("w-6")
        (self.run_dir / "w-6" / "a1" / "log.jsonl").write_text("not json\n")
        run = self.worker("resume", "w-6", "post your report")
        self.assertNotEqual(run.returncode, 0)
        self.assertIn("no session id", run.stderr)
        self.assertEqual(self.calls("resume-calls"), [])


if __name__ == "__main__":
    unittest.main()

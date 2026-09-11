"""Bounded two-client combat test against the running Docker stack.

Uses two newly provisioned accounts; never resets shared database tables.
Clients run on the host, all three server components run in Docker.
"""
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

HERE = Path(__file__).resolve().parent
CLIENT = HERE.parent.parent / "3D_App"
COMPOSE = ["docker", "compose", "-f", str(HERE / "compose.yaml")]


def main():
    processes = []
    with tempfile.TemporaryDirectory(prefix="aigame-g4-") as temp:
        logs = []
        try:
            for role in ("A", "B"):
                username = "g4-" + role.lower() + "-" + secrets.token_hex(5)
                password = secrets.token_urlsafe(24)
                seed_env = os.environ | {"G0_TEST_PASSWORD": password}
                subprocess.run(COMPOSE + ["exec", "-T", "-e", "G0_TEST_PASSWORD", "backend",
                               "/app/backend", "seed", username], env=seed_env, check=True,
                               timeout=15, stdout=subprocess.DEVNULL)
                env = os.environ | {
                    "G0_BACKEND_URL": os.getenv("G4_BACKEND_URL", "http://127.0.0.1:18080"),
                    "G0_USERNAME": username, "G0_PASSWORD": password,
                    "G0_LOOPBACK_TEST": "1", "ROOM_PRESET": "coop_combat",
                    "G4_TEST_ROLE": role, "G1_CLIENT_ROLE": "", "G3_INTEGRATION": "0",
                }
                log = open(Path(temp) / f"{role}.log", "w+")
                logs.append(log)
                processes.append(subprocess.Popen(["godot", "--headless", "--path", str(CLIENT),
                                  "--script", "res://Tests/Godot/g4_runner.gd", "--", "--smoke-exit"],
                                  env=env, stdout=log, stderr=subprocess.STDOUT))
            deadline = time.monotonic() + 35
            while any(p.poll() is None for p in processes):
                if time.monotonic() > deadline:
                    raise TimeoutError("G4 clients exceeded 35 seconds")
                time.sleep(0.1)
            for role, process, log in zip(("A", "B"), processes, logs):
                log.seek(0)
                output = log.read()
                print(output)
                if process.returncode != 0 or f"G4_COMBAT_PASS role={role}" not in output or "SCRIPT ERROR" in output:
                    raise RuntimeError(f"Client {role} failed")
                if role == "A" and "G4_DUPLICATE_PASS" not in output:
                    raise RuntimeError("Duplicate attack was not verified")
            print("G4_DOCKER_BACKEND_TWO_HOST_CLIENTS_PASS")
        finally:
            for process in processes:
                if process.poll() is None:
                    process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            for log in logs:
                log.close()


if __name__ == "__main__":
    main()

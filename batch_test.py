import subprocess
import os
import pathlib
import sys
import time
from concurrent.futures import ProcessPoolExecutor
from datetime import datetime, timedelta
from typing import Tuple

ROOT = pathlib.Path(__file__).parent
RAFT_ROOT = ROOT.joinpath("src").joinpath("raft")
LOG_ROOT = ROOT.joinpath("logs")

ALL_TESTS = ["InitialElection",
             "ReElection",
             "BasicAgree",
             "FailAgree",
             "FailNoAgree",
             "ConcurrentStarts",
             "Rejoin",
             "Backup",
             "Count",
             "Persist1",
             "Persist2",
             "Persist3",
             "Figure8",
             "UnreliableAgree",
             "Figure8Unreliable",
             "ReliableChurn",
             "UnreliableChurn"]

LAB1 = ALL_TESTS[0:2]
LAB2 = ALL_TESTS[2:9]
LAB3 = ALL_TESTS[9:12]

TESTS = {
    "lab1": LAB1,
    "lab2": LAB2,
    "lab3": LAB3,
    "all": ALL_TESTS,
    **{str(k): v for k, v in enumerate(ALL_TESTS)}
}

if not LOG_ROOT.exists():
    os.mkdir(LOG_ROOT)


def runtest(name: str, id: str) -> Tuple[bool, timedelta]:
    start = time.time()
    result = subprocess.run(["go", "test", "-run", name], cwd=RAFT_ROOT, stdout=subprocess.PIPE, env={**os.environ, "DEBUG": "1"}, text=True, encoding="utf-8")
    end = time.time()
    if result.returncode != 0:
        logdir = LOG_ROOT.joinpath(name)
        if not logdir.exists():
            os.mkdir(logdir)
        logdir.joinpath(f"{id}.out").write_text(result.stdout)
    return result.returncode == 0, timedelta(seconds=end-start)


def paralleltest(args: Tuple[str, str]) -> Tuple[bool, timedelta]:
    name, i = args
    prompt = f"Test {name} ({i})"
    print(f"{prompt}...")
    ispass, tm = runtest(name, f"{name}-{i}")
    if ispass:
        print(f"{prompt} {tm}")
    else:
        print(f"{prompt} {tm} FAILED")
    return ispass, tm


def test(name: str, cnt: int = 10, workers = None):
    now = datetime.now()

    with ProcessPoolExecutor(workers) as pool:
        results = list(pool.map(paralleltest, [(name, i) for i in range(cnt)]))

    passed = sum((1 for p in results if p))

    logs = [f"Passed {passed}, failed {cnt - passed}, Passed {int(passed/cnt*10000)/100}%"]
    logs.extend((f"Case {i}: {'PASSED' if v[0] else 'FAILED'} {v[1]}" for i, v in enumerate(results)))

    nowstr = now.strftime('%Y-%m-%dT%H-%M-%S')

    LOG_ROOT.joinpath(
        f"test-{name}-{cnt}({nowstr}).log").write_text("\n".join(logs))


def main():
    argv = sys.argv[1:]
    if len(argv) == 0:
        print("No arguments")
        return
    name = argv[0]
    cnt = int(argv[1]) if len(argv) >= 2 else 10
    workers = int(argv[2]) if len(argv) >= 3 else None
    if name in TESTS:
        names = TESTS[name]
    else:
        names = [name]

    for name in names:
        test(name, cnt, workers)


if __name__ == "__main__":
    main()

import subprocess
import os
import pathlib
import sys
import time
from concurrent.futures import ProcessPoolExecutor
from datetime import datetime, timedelta
from typing import List, Tuple

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


TIMEOUT_RET = 99


def runtest(name: str, logroot: pathlib.Path, id: str) -> Tuple[bool, timedelta]:
    start = time.time()
    retcode = 0
    stdout = ""
    try:
        result = subprocess.run(["go", "test", "-run", name], cwd=RAFT_ROOT, stdout=subprocess.PIPE, env={
                                **os.environ, "DEBUG": "1"}, text=True, encoding="utf-8", timeout=3*60)
        retcode = result.returncode
        stdout = result.stdout
    except subprocess.TimeoutExpired as ex:
        retcode = TIMEOUT_RET
        stdout = ex.stdout.decode("utf-8")
    end = time.time()
    if retcode != 0:
        logdir = logroot.joinpath(name)
        if not logdir.exists():
            os.makedirs(logdir)
        if retcode == TIMEOUT_RET:
            logdir.joinpath(f"{id}.err").write_text(stdout)
        else:
            logdir.joinpath(f"{id}.out").write_text(stdout)
    return retcode == 0, timedelta(seconds=end-start)


def paralleltest(args: Tuple[str, str, str]) -> Tuple[bool, timedelta]:
    id, name, i = args
    prompt = f"{id}: Test {name} ({i})"
    print(f"{prompt}...")
    ispass, tm = runtest(name, LOG_ROOT.joinpath(id), f"{i}")
    if ispass:
        print(f"{prompt} {tm}")
    else:
        print(f"{prompt} {tm} FAILED")
    return ispass, tm


def test(id: str, name: str, cnt: int = 10, workers=None) -> int:
    now = datetime.now()

    with ProcessPoolExecutor(workers) as pool:
        results = list(
            pool.map(paralleltest, [(id, name, i) for i in range(cnt)]))

    passed = sum((1 for p in results if p[0]))

    logs = [f"{id}: {name}, {cnt} cases, {workers} workers, @ {now}",
            f"Passed {passed}, failed {cnt - passed}, Passed {int(passed/cnt*10000)/100}%"]
    logs.extend(
        (f"Case {i}: {'PASSED' if v[0] else 'FAILED'} {v[1]}" for i, v in enumerate(results)))
    
    if passed < cnt:
        logroot = LOG_ROOT.joinpath(id).joinpath(name)
        if not logroot.exists():
            os.makedirs(logroot)

        logroot.joinpath(
            f"{name}.log").write_text("\n".join(logs))

    return passed


def testall(id: str, names: List[str], cnt: int = 10, workers=None):
    result = {}
    logroot = LOG_ROOT.joinpath(id)
    if not logroot.exists():
        os.makedirs(logroot)
    for name in names:
        passed = test(id, name, cnt, workers)
        result[name] = passed
    LOG_ROOT.joinpath(id).joinpath(
        f"result.log").write_text("\n".join((f"{k}: {int(v/cnt*10000)/100}% ({v}/{cnt})" for k, v in result.items())))


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

    now = datetime.now()
    id = now.strftime("%Y-%m-%dT%H-%M-%S")

    testall(id, names, cnt, workers)


if __name__ == "__main__":
    main()

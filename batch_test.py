import subprocess
import os
import pathlib
import sys
import time
from concurrent.futures import ProcessPoolExecutor
from datetime import datetime, timedelta
from typing import List, Optional, Tuple
import argparse

BUILDED_TESTER_NAME = "tester.exe"

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
LAB = ALL_TESTS[0:12]

TESTS = {
    "lab1": LAB1,
    "lab2": LAB2,
    "lab3": LAB3,
    "lab": LAB,
    "all": ALL_TESTS,
    **{str(k): [v] for k, v in enumerate(ALL_TESTS)},
    ** {v.lower(): [v] for v in ALL_TESTS}
}

if not LOG_ROOT.exists():
    os.mkdir(LOG_ROOT)


TIMEOUT_RET = 99


def runtest(name: str, logroot: pathlib.Path, id: str, flags: str) -> Tuple[bool, timedelta]:
    start = time.time()
    retcode = 0
    stdout = ""
    stderr = ""
    tester = logroot.joinpath(BUILDED_TESTER_NAME).absolute()
    try:
        # Enable heartbeat log and persist
        result = subprocess.run([str(tester), "-test.run", f"{name}$", "-test.v"], cwd=RAFT_ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env={
                                **os.environ, "DEBUG": flags}, text=True, encoding="utf-8", timeout=3*60)
        retcode = result.returncode
        stdout = result.stdout
        stderr = result.stderr
    except subprocess.TimeoutExpired as ex:
        retcode = TIMEOUT_RET
        stdout = ex.stdout.decode("utf-8")
        stderr = ex.stderr.decode("utf-8")
    end = time.time()
    if retcode != 0:
        logdir = logroot.joinpath(name)
        if not logdir.exists():
            os.makedirs(logdir)
        if retcode == TIMEOUT_RET:
            stdout += "\n\nTIMEOUT"
        logdir.joinpath(f"{id}.err").write_text(stderr)
        logdir.joinpath(f"{id}.out").write_text(stdout)
    return retcode == 0, timedelta(seconds=end-start)


def paralleltest(args: Tuple[str, str, str]) -> Tuple[bool, timedelta]:
    id, name, i, flags = args
    prompt = f"{id}: Test {name} ({i})"
    print(f"{prompt}...")
    ispass, tm = runtest(name, LOG_ROOT.joinpath(id), f"{i}", flags)
    if ispass:
        print(f"{prompt} {tm}")
    else:
        print(f"{prompt} {tm} FAILED")
    return ispass, tm


def test(id: str, name: str, cnt: int, workers: Optional[int], flags: str) -> int:
    now = datetime.now()

    logs: List[str] = [f"{id}: {name}, {cnt} cases, {workers} workers, @ {now}"]
    print("\n".join(logs))

    with ProcessPoolExecutor(workers) as pool:
        results = list(
            pool.map(paralleltest, [(id, name, i, flags) for i in range(cnt)]))

    passed = sum((1 for p in results if p[0]))

    logs[0] += f" ~ {datetime.now()}"
    logs.append(f"Passed {passed}, failed {cnt - passed}, Passed {int(passed/cnt*10000)/100}%")
    print("\n".join(logs))
    logs.extend(
        (f"Case {i}: {'PASSED' if v[0] else 'FAILED'} {v[1]}" for i, v in enumerate(results)))

    if passed < cnt:
        logroot = LOG_ROOT.joinpath(id).joinpath(name)
        if not logroot.exists():
            os.makedirs(logroot)

        logroot.joinpath(
            f"result.log").write_text("\n".join(logs) + "\n")

    return passed


def testall(id: str, names: List[str], cnt: int, workers: Optional[int], flags: str) -> bool:
    result = {}
    logroot = LOG_ROOT.joinpath(id)
    if not logroot.exists():
        os.makedirs(logroot)

    testerPath = logroot.joinpath(BUILDED_TESTER_NAME).absolute()
    print(f"Building tester -> {testerPath}...")
    subprocess.check_call(
        ["go", "test", "-c", "-o", str(testerPath)], cwd=RAFT_ROOT)

    passedAll = True

    for name in names:
        passed = test(id, name, cnt, workers, flags)
        result[name] = passed
        if passed < cnt:
            passedAll = False
    items = list(result.items())
    items.sort(key=lambda x: (x[1], x[0]))
    resultlogs = "\n".join(
        (f"{k}: {int(v/cnt*10000)/100}% ({v}/{cnt})" for k, v in items))
    LOG_ROOT.joinpath(id).joinpath(
        f"result.log").write_text(resultlogs + "\n")
    print(resultlogs)

    if passedAll:
        print("All tests passed!")
    else:
        print("Some tests failed!")

    return passedAll


def main():

    parser = argparse.ArgumentParser()

    parser.add_argument("name", choices=list(TESTS))
    parser.add_argument("-c", "--count", default=10, type=int)
    parser.add_argument("-f", "--flag", default="HTL")
    parser.add_argument("-w", "--worker", default=None,
                        type=lambda x: int(x) if x else None)

    args = parser.parse_args()

    name = args.name
    cnt = args.count
    workers = args.worker
    flags = args.flag.strip()
    names = TESTS[name]

    now = datetime.now()
    id = now.strftime("%Y-%m-%dT%H-%M-%S")

    passed = testall(f"{name}-{id}", names, cnt, workers, flags)

    if passed:
        exit(0)
    else:
        exit(1)


if __name__ == "__main__":
    main()

![Raft](https://socialify.git.ci/StardustDL/raft-impl/image?description=1&font=Bitter&forks=1&issues=1&language=1&owner=1&pulls=1&stargazers=1&theme=Light)

![CI](https://github.com/StardustDL/raft-impl/workflows/CI/badge.svg) ![](https://img.shields.io/github/license/StardustDL/raft-impl.svg)

A demo **1-to-1** implementation with **high availability** in Golang for Raft Consensus algorithm. In all tests run in parallel, this implement has more than **99.999%** availability. It passed more than **200,000** parallel batch tests with default configuration.

> The implement is written according to the [paper](https://raft.github.io/raft.pdf), based on 6.824's raft labs (and tests).
> All commits are automatically tested (in small scale) by GitHub Actions, real-time testing results is at [here](https://github.com/StardustDL/raft-impl/actions).

The implement contains the following core features.

- Leader Election
- Log Replication
- Persistence (opt-out)

The implement also has the following extra features.

- Using mutex lock shortly.
- Using timer channel to avoid busy-waiting.
- Gracefully killing.
- Enabling big-step next-index descreasing (opt-out).
- Fully logging (opt-in).
- Detecting disconnecting to convert to follower (opt-in).
- Every important code part has comments from the related sentences in the raft paper (so called **1-to-1**).

> The opt-in/opt-out features can be configured by `DEBUG` flags, which are described in the following section.

## Running

The project detects a runtime environment variable **`DEBUG`**. If it exists and it is **not empty** (any non-empty values is OK), the debug mode and logging will enable. The following flags can be used in the variable.

|Flag|Effect|
|-|-|
|**`H`**|Enable heartbeat logging|
|**`T`**|Enable timer logging|
|**`L`**|Enable lock logging|
|**`D`**|Enable disconnection detecting|
|**`p`**|Disable persisting|
|**`b`**|Disable the optimization for decreasing next-index in big-step|

## Testing

> Using Go 1.17.5 with go-module enabled.

Run the following script.

```sh
cd src/raft
go test
```

### Group Tests

Run in `GO111MODULE=off` mode.

```sh
# Lab 1: Leader Election
pwsh -c ./tests/lab1.ps1

# Lab 2: Log Replication
pwsh -c ./tests/lab2.ps1

# Lab 3: Persistence
pwsh -c ./tests/lab3.ps1

# All tests
pwsh -c ./tests/all.ps1
```

### Batch Tests

```sh
python ./batch_test.py "test collection name"
  [-c <count of runs=10>]
  [-f <DEBUG Flags="HTL">]
  [-w <parallelism=the number of CPU cores>]
```

Space charaters in `-f` option will be striped. It exits with `0` if and only if all tests are passed.
The result will be under the directory `logs`. All failed tests' logs will be recorded.

### Correctness Checking Tests

`check.sh` use batch test script in 4 running stages (divided into the following 2 axes) to check correctness.

||**Logging**|**No Logging**|
|-|-|-|
|**Serial** (100 runs, 1 worker)|Stage 1|Stage 2|
|**Parallel** (1000 runs, 1x-processor workers)|Staget 3|Stage 4|

```sh
./check.sh
```

### Large-scale Tests

`test.sh` run all tests in parallel (10x-processor workers) and no-logging mode.

```sh
./test.sh <count of runs>
```

## Results

### Single Test

> The simplified output from the command `go test -v`.

```
Test: initial election ...
--- PASS: TestInitialElection (2.50s)
Test: election after network failure ...
--- PASS: TestReElection (5.00s)
Test: basic agreement ...
--- PASS: TestBasicAgree (0.64s)
Test: agreement despite follower failure ...
--- PASS: TestFailAgree (5.44s)
Test: no agreement if too many followers fail ...
--- PASS: TestFailNoAgree (3.58s)
Test: concurrent Start()s ...
--- PASS: TestConcurrentStarts (0.60s)
Test: rejoin of partitioned leader ...
--- PASS: TestRejoin (4.43s)
Test: leader backs up quickly over incorrect follower logs ...
--- PASS: TestBackup (13.12s)
Test: RPC counts aren't too high ...
--- PASS: TestCount (2.14s)
Test: basic persistence ...
--- PASS: TestPersist1 (3.93s)
Test: more persistence ...
--- PASS: TestPersist2 (15.80s)
Test: partitioned leader and one follower crash, leader restarts ...
--- PASS: TestPersist3 (1.61s)
Test: Figure 8 ...
--- PASS: TestFigure8 (26.19s)
Test: unreliable agreement ...
--- PASS: TestUnreliableAgree (3.14s)
Test: Figure 8 (unreliable) ...
--- PASS: TestFigure8Unreliable (25.96s)
Test: churn ...
--- PASS: TestReliableChurn (16.46s)
Test: unreliable churn ...
--- PASS: TestUnreliableChurn (16.12s)
PASS
ok  	raft	146.675s
```

### Batch Tests

> The simplified output from the command `./test.sh 100000`.

```
Start Large Scale (100000 cases, 120 workers) (Parallel (Disabled Logging))
Backup: 100.0% (100000/100000)
BasicAgree: 100.0% (100000/100000)
ConcurrentStarts: 100.0% (100000/100000)
Count: 100.0% (100000/100000)
FailAgree: 100.0% (100000/100000)
FailNoAgree: 100.0% (100000/100000)
Figure8: 100.0% (100000/100000)
Figure8Unreliable: 100.0% (100000/100000)
InitialElection: 100.0% (100000/100000)
Persist1: 100.0% (100000/100000)
Persist2: 100.0% (100000/100000)
Persist3: 100.0% (100000/100000)
ReElection: 100.0% (100000/100000)
Rejoin: 100.0% (100000/100000)
ReliableChurn: 100.0% (100000/100000)
UnreliableAgree: 100.0% (100000/100000)
UnreliableChurn: 100.0% (100000/100000)
```

![Raft](https://socialify.git.ci/StardustDL/raft-impl/image?description=1&font=Bitter&forks=1&issues=1&language=1&owner=1&pulls=1&stargazers=1&theme=Light)

![CI](https://github.com/StardustDL/raft-impl/workflows/CI/badge.svg) ![](https://img.shields.io/github/license/StardustDL/raft-impl.svg)

A demo 1-to-1 implementation in Golang for Raft Consensus algorithm according to the [paper](https://raft.github.io/raft.pdf), based on 6.824's raft labs, with the following core features supported.

- Leader Election
- Log Replication
- Persistence (opt-out)

This implement has the following extra features.

- Using mutex lock shortly.
- Using timer channel to avoid busy-waiting.
- Gracefully killing.
- Enabling big-step descreasing nextIndex (opt-out).
- Full logging (opt-in).
- Detecting disconnecting to convert to follower (opt-in).
- More than **99.999%** availability through all tests in parallel.
  - Passed more than 100000 parallel batch tests.

> All commits are automatically tested (in small scale) by GitHub Actions, you can see real-time testing results at [there](https://github.com/StardustDL/raft-impl/actions).

## Testing

```sh
cd src/raft
go test
```

The project detect a runtime environment variable **`DEBUG`**.

- Enable debug mode and logging, if it exists and it is not empty (any non-empty values is OK).
- Enable logging heartbeat, if it contains `H`.
- Enable logging timer, if it contains `T`.
- Enable logging lock, if it contains `L`.
- Enable detecting disconnect, if it contains `D`.
- Disable persisting, if it contains `p`.
- Disable the optimization for big-step to decrease nextIndex, if it contains `b`

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
  [-c <replication count=10>]
  [-f <DEBUG Flags (will be striped)="HTL">]
  [-w <parallelism=the number of CPU cores>]
```

The result will be under the directory `logs`. All failed tests' logs will be recorded.

### Correctness Checking Tests

`check.sh` use batch test script in 4 running stages (divided into the following 2 axes) to check correctness.

||**Logging**|**No Logging**|
|-|-|-|
|**Serial**|Stage 1|Stage 2|
|**Parallel**|Staget 3|Stage 4|

```sh
./check.sh
```

### Large-scale Tests

`test.sh` use parallel (10x-processor workers) and no logging mode to run all tests.

```sh
./test.sh <count>
```

## Results

### Single Test

> The output from the command `go test`.

```
Test: initial election ...
  ... Passed
Test: election after network failure ...
  ... Passed
Test: basic agreement ...
  ... Passed
Test: agreement despite follower failure ...
  ... Passed
Test: no agreement if too many followers fail ...
  ... Passed
Test: concurrent Start()s ...
  ... Passed
Test: rejoin of partitioned leader ...
  ... Passed
Test: leader backs up quickly over incorrect follower logs ...
  ... Passed
Test: RPC counts aren't too high ...
  ... Passed
Test: basic persistence ...
  ... Passed
Test: more persistence ...
  ... Passed
Test: partitioned leader and one follower crash, leader restarts ...
  ... Passed
Test: Figure 8 ...
  ... Passed
Test: unreliable agreement ...
  ... Passed
Test: Figure 8 (unreliable) ...
  ... Passed
Test: churn ...
  ... Passed
Test: unreliable churn ...
  ... Passed
PASS
ok      raft    166.841s
```

### Batch Tests

> The result from the command `./test.sh 100000`.

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

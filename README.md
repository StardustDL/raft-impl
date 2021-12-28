![Raft](https://socialify.git.ci/StardustDL/raft-impl/image?description=1&font=Bitter&forks=1&issues=1&language=1&owner=1&pulls=1&stargazers=1&theme=Light)

![CI](https://github.com/StardustDL/raft-impl/workflows/CI/badge.svg) ![](https://img.shields.io/github/license/StardustDL/raft-impl.svg)

A demo 1-to-1 implementation in Golang for Raft Consensus algorithm according to the [paper](https://raft.github.io/raft.pdf), based on 6.824's raft labs, with the following core features supported.

- Leader Election
- Log Replication
- Persistence (opt-out)

This implement has the following extra features.

- Use mutex lock shortly.
- Full logging (opt-in).
- Enable big-step descreasing nextIndex (opt-out).
- Check disconnecting and convert to follower (opt-in).

## Testing

```sh
cd src/raft
go test
```

The project detect a runtime environment variable **`DEBUG`**.

- Enable debug mode and logging, if it exists and it is not empty (any non-empty values is OK).
- Enable heartbeat logging, if it contains `H`.
- Enable timer logging, if it contains `T`.
- Enable lock logging, if it contains `L`.
- Enable follow when disconnected, if it contains `D`.
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
  [-f <DEBUG Flags="HTL">]
  [-w <parallelism=the number of CPU cores>]
```

Recommend to set `-w 1` to use serial testing since some tests will fail when run them parallel.

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

`test.sh` use parallel (use 3x-processor workers) and no logging mode to run all tests.

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

> The result from the command `python ./batch_test.py all -c 100 -w 1`.

```
Backup: 100.0% (100/100)
BasicAgree: 100.0% (100/100)
ConcurrentStarts: 100.0% (100/100)
Count: 100.0% (100/100)
FailAgree: 100.0% (100/100)
FailNoAgree: 100.0% (100/100)
Figure8: 100.0% (100/100)
Figure8Unreliable: 100.0% (100/100)
InitialElection: 100.0% (100/100)
Persist1: 100.0% (100/100)
Persist2: 100.0% (100/100)
Persist3: 100.0% (100/100)
ReElection: 100.0% (100/100)
Rejoin: 100.0% (100/100)
ReliableChurn: 100.0% (100/100)
UnreliableAgree: 100.0% (100/100)
UnreliableChurn: 100.0% (100/100)
```

# Raft

A demo 1-to-1 implementation in Golang for Raft Consensus algorithm according to the [paper](https://raft.github.io/raft.pdf), based on 6.824's raft labs, with the following core features supported.

- Leader Election
- Log Replication
- Persistence

## Run

```sh
cd src/raft
go test
```

The project detect a runtime environment variable **`DEBUG`**.
- Enable debug mode and logging, if it exists and it is not empty.
- Enable heartbeat logging, if it contains `H`.
- Disable persisting, if it contains `p`.

## Group Tests

Run in `GO111MODULE=off` mode.

```sh
# Lab 1: Leader Election
pwsh -c ./lab1.ps1

# Lab 2: Log Replication
pwsh -c ./lab2.ps1

# Lab 3: Persistence
pwsh -c ./lab3.ps1
```

## Batch Tests

```sh
python ./batch_test.py "test suite name" [-c <replication count=10>] [-f <DEBUG Flags="H">] [-w <parallelism=the number of CPU cores>]
```

Suggest set `-w 1` to use serial testing since some tests will fail when run them parallel.

The result will be under the directory `logs`. All failed tests' logs will be recorded.

## Test Results

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
--- FAIL: TestFigure8Unreliable (56.23s)
    config.go:424: one(4794) failed to reach agreement
Test: churn ...
  ... Passed
Test: unreliable churn ...
  ... Passed
FAIL
exit status 1
FAIL    raft    193.396s
```
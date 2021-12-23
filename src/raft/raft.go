package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"labrpc"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

var (
	DEBUG                     = false
	LOG_HEARTBEAT             = false
	LOG_TIMEOUT               = false
	LOG_LOCK                  = false
	DISABLE_PERSIST           = false
	DISABLE_NEXTINDEX_BIGSTEP = false // disable the optimization for big step to decrease nextIndex
)

// Raft’s RPCs typically require the recipient to persist information to stable storage,
// so the broadcast time may range from 0.5ms to 20ms, depending on storage technology.
// As a result, the election timeout is likely to be somewhere between 10ms and 500ms.

const (
	unvoted              = -1
	notfoundConflictTerm = -1
	heartbeatTimeout     = time.Duration(50) * time.Millisecond
	electionTimeoutMin   = 400 * time.Millisecond
	electionTimeoutMax   = 500 * time.Millisecond
	LOG_CLASS_HEARTBEAT  = "RPC:Heart"
	LOG_CLASS_APPEND     = "RPC:Entry"
	LOG_CLASS_VOTE       = "RPC:Voted"
	LOG_CLASS_CLIENT     = "RPC:Clien"
	LOG_CLASS_LOCK       = "Lock"
	LOG_CLASS_TIMEOUT    = "Time"
	LOG_CLASS_LIFECYCLE  = "Life"
	LOG_CLASS_COMMIT     = "Cmit"
	LOG_CLASS_LEADING    = "Lead"
	LOG_CLASS_ELECTION   = "Elec"
)

const (
	follower  = 0
	candidate = 1
	leader    = 2
)

func minInt(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func getRandomizedElectionTimeout() time.Duration {
	return electionTimeoutMin + time.Duration(rand.Int63())%(electionTimeoutMax-electionTimeoutMin)
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

// each entry contains command for state machine,
// and term when entry was received by leader (first index is 1)
type LogEntry struct {
	Term  int
	Value interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int           // index into peers[]
	logger    *log.Logger   // logger
	applyCh   chan ApplyMsg // a channel on which the tester or service expects Raft to send ApplyMsg messages
	killCh    chan bool     // a channel for kill

	// Persistent state on all servers (Updated on stable storage before responding to RPCs)

	currentTerm int        // latest term server has seen (initialized to 0 on first boot, increases monotonically)
	votedFor    int        // candidateId that received vote in current term (or null if none)
	logs        []LogEntry // log entries, index from [1..rf.len()]

	// Volatile state on all servers

	commitIndex int // index of highest log entry known to be committed (initialized to 0, increases monotonically)
	lastApplied int // index of highest log entry applied to state machine (initialized to 0, increases monotonically)

	// Volatile state on leaders (Reinitialized after election)

	nextIndex  []int // for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	matchIndex []int // for each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically)

	connected []bool // Can connected to followers

	// Volatile state on candidate

	voteGranted []bool // Grated votes

	// Role
	role int

	// Timers

	heartbeatTimer *time.Timer
	electionTimer  *time.Timer
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	return rf.currentTerm, rf.role == leader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	if DISABLE_PERSIST {
		return
	}
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if DISABLE_PERSIST {
		return
	}
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	d.Decode(&rf.currentTerm)
	d.Decode(&rf.votedFor)
	d.Decode(&rf.logs)
}

type RpcTransferData interface {
	term() int
}

type AppendEntriesArgs struct {
	Term         int        // leader’s term
	LeaderId     int        // so follower can redirect clients
	PrevLogIndex int        // index of log entry immediately preceding new ones
	PrevLogTerm  int        // term of prevLogIndex entry
	Entries      []LogEntry // log entries to store (empty for heartbeat; may send more than one for efficiency)
	LeaderCommit int        // leader’s commitIndex
}

func (data AppendEntriesArgs) term() int {
	return data.Term
}

func (data AppendEntriesArgs) isheartbeat() bool {
	return len(data.Entries) == 0
}

// If desired, the protocol can be optimized to reduce the number of rejected AppendEntries RPCs.
// For example, when rejecting an AppendEntries request, the follower
// can include the term of the conflicting entry
// and the first index it stores for that term.
// With this information, the leader can decrement nextIndex to bypass all of the conflicting entries in that term;
// one AppendEntries RPC will be required for each term with conflicting entries,
// rather than one RPC per entry.
// In practice, we doubt this optimization is necessary, since failures happen infrequently and it is unlikely that there will be many inconsistent entries.

type AppendEntriesReply struct {
	Term          int  // currentTerm, for leader to update itself
	Success       bool // true if follower contained entry matching prevLogIndex and prevLogTerm
	ConflictTerm  int  // the term of the conflicting entry if failed
	ConflictIndex int  // the first index it stores for that term
}

func (data AppendEntriesReply) term() int {
	return data.Term
}

//
// AppendEntries RPC handler.
//
func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	if args.isheartbeat() {
		rf.LogClass(LOG_CLASS_HEARTBEAT, "Recieve from %d: %+v", args.LeaderId, args)
	} else {
		rf.LogClass(LOG_CLASS_APPEND, "Recieve from %d: from (%d, %d) with %d entries: %+v", args.LeaderId, args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), args)
	}

	rf.InLock(func() {
		rf.checkFollow(args)
		// If AppendEntries RPC received from new leader: convert to follower
		if rf.role == candidate && rf.currentTerm == args.Term {
			rf.follow()
		}
	})

	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		// Reply false if term < currentTerm
		reply.Success = false
	} else { // assume rpc is from current leader
		// receiving AppendEntries RPC from current leader
		rf.resetElectionTimeout()
		if args.PrevLogIndex > rf.len() {
			// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
			reply.Success = false

			if !DISABLE_NEXTINDEX_BIGSTEP {
				// If desired, the protocol can be optimized to reduce the number of rejected AppendEntries RPCs.
				// For example, when rejecting an AppendEntries request, the follower
				// can include the term of the conflicting entry
				// and the first index it stores for that term.
				reply.ConflictIndex = rf.len() + 1
				reply.ConflictTerm = notfoundConflictTerm
			}
		} else {
			if args.PrevLogIndex >= 1 && rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
				// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
				reply.Success = false

				if !DISABLE_NEXTINDEX_BIGSTEP {
					// If desired, the protocol can be optimized to reduce the number of rejected AppendEntries RPCs.
					// For example, when rejecting an AppendEntries request, the follower
					// can include the term of the conflicting entry
					// and the first index it stores for that term.

					reply.ConflictTerm = rf.logs[args.PrevLogIndex].Term

					firstIndex := args.PrevLogIndex
					for firstIndex-1 >= 1 && rf.logs[firstIndex-1].Term == reply.ConflictTerm {
						firstIndex--
					}

					reply.ConflictIndex = firstIndex
				}

				// If an existing entry conflicts with a new one (same index but different terms)
				// delete the existing entry and all that follow it
				rf.logs = rf.logs[:args.PrevLogIndex]
			} else { // args.PrevLogIndex == 0 or rf.logs[args.PrevLogIndex].Term == args.PrevLogTerm
				reply.Success = true

				rf.InLock(func() {
					// Append any new entries not already in the log

					// If an existing entry conflicts with a new one (same index but different terms)
					// delete the existing entry and all that follow it

					firstUnmatch := 0

					for firstUnmatch < len(args.Entries) && args.PrevLogIndex+1+firstUnmatch <= rf.len() && rf.logs[args.PrevLogIndex+1+firstUnmatch].Term == args.Entries[firstUnmatch].Term {
						firstUnmatch++
					}

					if firstUnmatch <= len(args.Entries) {
						args.Entries = args.Entries[firstUnmatch:]
					}

					lastNewIndex := args.PrevLogIndex + firstUnmatch

					if len(args.Entries) > 0 {
						// must do this if exist new entry, because may recieve a short entries after a long entries
						if lastNewIndex+1 <= rf.len() {
							rf.logs = rf.logs[:lastNewIndex+1]
						}

						rf.logs = append(rf.logs, args.Entries...)
						lastNewIndex += len(args.Entries)
					}

					// If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
					if args.LeaderCommit > rf.commitIndex {
						newCommitIndex := minInt(args.LeaderCommit, lastNewIndex)
						if newCommitIndex > rf.commitIndex {
							rf.commitIndex = newCommitIndex
							rf.LogClass(LOG_CLASS_COMMIT, "Follower commit index updated: %d", rf.commitIndex)
						}
					}
				})
			}
		}
	}

	// Updated on stable storage before responding to RPCs
	rf.persist()

	issuccess := "SUCCESS"

	if !reply.Success {
		issuccess = "FAILED"
	}

	if args.isheartbeat() {
		rf.LogClass(LOG_CLASS_HEARTBEAT, "Reply %s to %d: %+v", issuccess, args.LeaderId, args)
	} else {
		rf.LogClass(LOG_CLASS_APPEND, "Reply %s to %d: from (%d, %d) with %d entries: %+v", issuccess, args.LeaderId, args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), args)
	}
}

//
// send a AppendEntries RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool {
	if len(args.Entries) == 0 {
		rf.LogClass(LOG_CLASS_HEARTBEAT, "Send to %d: %+v", server, args)
	} else {
		rf.LogClass(LOG_CLASS_APPEND, "Send from (%d, %d) with %d entries to %d: %+v", args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), server, args)
	}
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	if !ok {
		if len(args.Entries) == 0 {
			rf.LogClass(LOG_CLASS_HEARTBEAT, "Failed to send to %d: %+v", server, args)
		} else {
			rf.LogClass(LOG_CLASS_APPEND, "Failed to send from (%d, %d) with %d entries to %d: %+v", args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), server, args)
		}
	}
	return ok
}

//
// RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	Term         int // candidate’s term
	CandidateId  int // candidate requesting vote
	LastLogIndex int // index of candidate’s last log entry
	LastLogTerm  int // term of candidate’s last log entry
}

func (data RequestVoteArgs) term() int {
	return data.Term
}

//
// RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	Term        int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

func (data RequestVoteReply) term() int {
	return data.Term
}

func (rf *Raft) Log(format string, v ...interface{}) {
	if !DEBUG {
		return
	}

	role := "follow"
	switch rf.role {
	case candidate:
		role = "candid"
	case leader:
		role = "leader"
	}
	rf.logger.Printf("%d(%s)[%d,%d>%d>%d]: %s\n", rf.me, role, rf.currentTerm, rf.lastApplied, rf.commitIndex, rf.len(), fmt.Sprintf(format, v...))
}

func (rf *Raft) LogClass(class string, format string, v ...interface{}) {
	if !LOG_HEARTBEAT && class == LOG_CLASS_HEARTBEAT {
		return
	}

	if !LOG_TIMEOUT && class == LOG_CLASS_TIMEOUT {
		return
	}

	if !LOG_LOCK && class == LOG_CLASS_LOCK {
		return
	}

	rf.Log("(%s) %s", class, fmt.Sprintf(format, v...))
}

// return length of logs
func (rf *Raft) len() int {
	return len(rf.logs) - 1
}

// return last log entry's index and term
func (rf *Raft) lastLogSignature() (int, int) {
	index := rf.len()
	term := 0
	if index >= 1 {
		term = rf.logs[index].Term
	}
	return index, term
}

// return a follower's prev log entry's index and term
func (rf *Raft) prevLogSignature(server int) (int, int) {
	index := rf.nextIndex[server] - 1
	term := 0
	if index >= 1 {
		term = rf.logs[index].Term
	}
	return index, term
}

// Raft determines which of two logs is more up-to-date by comparing the index and term of the last entries in the logs.
// If the logs have last entries with different terms, then the log with the later term is more up-to-date.
// If the logs end with the same term, then whichever log is longer is more up-to-date.
func (rf *Raft) isUpToDate(lastLogIndex int, lastLogTerm int) bool {
	index, term := rf.lastLogSignature()
	return lastLogTerm > term || lastLogTerm == term && lastLogIndex >= index
}

//
// RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	rf.LogClass(LOG_CLASS_VOTE, "Recieve from %d: %+v", args.CandidateId, args)

	rf.InLock(func() { rf.checkFollow(args) })

	reply.Term = rf.currentTerm

	// Reply false if term < currentTerm
	// If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote

	rf.InLock(func() {
		if args.Term >= rf.currentTerm && (rf.votedFor == unvoted || rf.votedFor == args.CandidateId) && rf.isUpToDate(args.LastLogIndex, args.LastLogTerm) {
			rf.votedFor = args.CandidateId

			// granting vote to candidate
			rf.resetElectionTimeout()

			reply.VoteGranted = true
		}
	})

	// Updated on stable storage before responding to RPCs
	rf.persist()

	isyes := "YES"

	if !reply.VoteGranted {
		isyes = "NO"
	}
	rf.LogClass(LOG_CLASS_VOTE, "Reply %s to %d: %+v", isyes, args.CandidateId, args)
}

//
// send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	rf.LogClass(LOG_CLASS_VOTE, "Send to %d: %+v", server, args)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if !ok {
		rf.LogClass(LOG_CLASS_VOTE, "Failed to send to %d: %+v", server, args)
	}
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// isLeader may unexpected changed after testing if no lock
	rf.Lock()
	defer rf.Unlock()

	index := -1
	term := rf.currentTerm
	isLeader := rf.role == leader

	if isLeader {
		rf.LogClass(LOG_CLASS_CLIENT, "Recieve request: %+v", command)

		// If command received from client: append entry to local log, respond after entry applied to state machine

		entry := LogEntry{
			Term:  rf.currentTerm,
			Value: command,
		}

		rf.logs = append(rf.logs, entry)
		lastLogIndex := rf.len()
		rf.nextIndex[rf.me] = lastLogIndex + 1
		rf.matchIndex[rf.me] = lastLogIndex

		// Updated on stable storage before responding to RPCs
		// > Leader does not responding to RPCs, just send RPCs.
		rf.persist()

		index = lastLogIndex

		// start log entry sync
		rf.heartbeat()

		rf.LogClass(LOG_CLASS_CLIENT, "Reply request %+v: %d", command, index)
	}

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	rf.LogClass(LOG_CLASS_LIFECYCLE, "Killed")
	close(rf.killCh)
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {

	debugEnv := os.Getenv("DEBUG")
	DEBUG = debugEnv != ""
	LOG_HEARTBEAT = strings.Contains(debugEnv, "H")
	DISABLE_PERSIST = strings.Contains(debugEnv, "p")
	DISABLE_NEXTINDEX_BIGSTEP = strings.Contains(debugEnv, "b")
	LOG_TIMEOUT = strings.Contains(debugEnv, "T")
	LOG_LOCK = strings.Contains(debugEnv, "L")

	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh
	rf.killCh = make(chan bool)

	if DEBUG {
		rf.logger = log.New(os.Stderr, "", log.Ltime|log.Lmicroseconds)
	} else {
		rf.logger = log.New(io.Discard, "", log.Ltime|log.Lmicroseconds)
	}

	rf.currentTerm = 0
	rf.votedFor = unvoted

	rf.logs = make([]LogEntry, 0)
	rf.logs = append(rf.logs, LogEntry{})

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	rf.connected = make([]bool, len(peers))

	rf.heartbeatTimer = time.NewTimer(heartbeatTimeout)
	rf.electionTimer = time.NewTimer(getRandomizedElectionTimeout())

	rf.voteGranted = make([]bool, len(peers))

	rf.role = follower

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.longrun()

	return rf
}

func (rf *Raft) hasKilled() bool {
	select {
	case <-rf.killCh:
		return true
	default:
		return false
	}
}

// If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N, and log[N].term == currentTerm: set commitIndex = N
func (rf *Raft) updateCommitIndex() {
	newIndex := rf.commitIndex + 1
	for newIndex <= rf.len() && rf.logs[newIndex].Term != rf.currentTerm {
		newIndex++
	}
	if newIndex <= rf.len() {
		matchedCount := 0
		for _, v := range rf.matchIndex {
			if v >= newIndex {
				matchedCount++
			}
		}
		// rf.LogClass(LOG_CLASS_COMMIT, "Index %d matched %d of %d", newIndex, matchedCount, len(rf.peers))
		if matchedCount*2 > len(rf.peers) {
			rf.commitIndex = newIndex
			rf.LogClass(LOG_CLASS_COMMIT, "Leader commit index updated: %d", rf.commitIndex)
		}
	}
}

// If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine
func (rf *Raft) apply() {
	if rf.role == leader {
		rf.updateCommitIndex()
	}

	if rf.commitIndex > rf.lastApplied {
		cur := rf.lastApplied + 1
		msg := ApplyMsg{
			Index:       cur,
			Command:     rf.logs[cur].Value,
			UseSnapshot: false,
			Snapshot:    make([]byte, 0),
		}
		rf.LogClass(LOG_CLASS_COMMIT, "Applying %d: %+v", cur, msg)

		rf.applyCh <- msg

		rf.lastApplied = cur

		rf.LogClass(LOG_CLASS_COMMIT, "Applied %d: %+v", cur, msg)
	}
}

func (rf *Raft) resetElectionTimeout() {
	timeout := getRandomizedElectionTimeout()
	rf.LogClass(LOG_CLASS_TIMEOUT, "%d reset election timeout: %d", rf.me, timeout.Milliseconds())
	rf.electionTimer.Reset(timeout)
}

func (rf *Raft) resetHeartbeatTimeout() {
	rf.LogClass(LOG_CLASS_TIMEOUT, "%d reset heartbeat timeout: %d", rf.me, heartbeatTimeout.Milliseconds())
	rf.heartbeatTimer.Reset(heartbeatTimeout)
}

func (rf *Raft) Lock() {
	rf.mu.Lock()
	rf.LogClass(LOG_CLASS_LOCK, "Lock")
}

func (rf *Raft) Unlock() {
	rf.mu.Unlock()
	rf.LogClass(LOG_CLASS_LOCK, "Unlock")
}

func (rf *Raft) InLock(f func()) {
	rf.Lock()
	defer rf.Unlock()

	f()
}

func (rf *Raft) OutLock(f func()) {
	rf.Unlock()
	defer rf.Lock()

	f()
}

func (rf *Raft) longrun() {
	rf.LogClass(LOG_CLASS_LIFECYCLE, "Long running...")
	go func() {
		for {
			// If election timeout elapses: start new election
			select {
			case <-rf.electionTimer.C:
				go func() {
					if rf.hasKilled() {
						return
					}
					rf.InLock(func() { rf.campaign() })
				}()
			case <-rf.killCh:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-rf.heartbeatTimer.C:
				go func() {
					if rf.hasKilled() {
						return
					}
					rf.InLock(func() { rf.heartbeat() })
				}()
			case <-rf.killCh:
				return
			}
		}
	}()

	go func() {
		for {
			if rf.hasKilled() {
				return
			}
			rf.InLock(func() { rf.apply() })
		}
	}()
}

// If votes received from majority of servers: become leader
func (rf *Raft) isWinner() bool {
	votedCount := 0
	for i, v := range rf.voteGranted {
		if v || i == rf.me {
			votedCount++
		}
	}
	return votedCount*2 > len(rf.peers)
}

// If election timeout elapses: start new election
func (rf *Raft) campaign() {
	if rf.role == leader {
		return
	}

	rf.LogClass(LOG_CLASS_LIFECYCLE, "%d campaign at term %d", rf.me, rf.currentTerm+1)
	for i := range rf.voteGranted {
		rf.voteGranted[i] = false
	}
	rf.role = candidate

	// Increment currentTerm
	rf.currentTerm++

	// Vote for self
	rf.votedFor = rf.me

	// Reset election timer
	rf.resetElectionTimeout()

	currentTerm := rf.currentTerm
	index, term := rf.lastLogSignature()
	args := RequestVoteArgs{
		Term:         currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: index,
		LastLogTerm:  term,
	}

	// Send RequestVote RPCs to all other servers
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(i int) {
			reply := RequestVoteReply{}

			ok := false
			for !ok && rf.role == candidate && rf.currentTerm == currentTerm {
				if rf.hasKilled() {
					return
				}
				ok = rf.sendRequestVote(i, args, &reply)
			}

			rf.InLock(func() { rf.checkFollow(reply) })

			if rf.role == candidate && rf.currentTerm == currentTerm {
				if reply.VoteGranted {
					rf.LogClass(LOG_CLASS_ELECTION, "%d granted vote from %d at term %d", rf.me, i, currentTerm)

					rf.voteGranted[i] = true

					// If votes received from majority of servers: become leader
					if rf.isWinner() {
						rf.LogClass(LOG_CLASS_ELECTION, "%d win the election at term %d", rf.me, currentTerm)
						rf.lead()
					}
				} else {
					rf.LogClass(LOG_CLASS_ELECTION, "%d failed to grant vote from %d at term %d", rf.me, i, currentTerm)
				}
			}
		}(i)
	}
}

// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
func (rf *Raft) checkFollow(data RpcTransferData) {
	if data.term() > rf.currentTerm {
		rf.currentTerm = data.term()
		rf.follow()
	}
}

func (rf *Raft) follow() {
	if rf.role == follower {
		return
	}
	rf.LogClass(LOG_CLASS_LIFECYCLE, "%d follow at term %d", rf.me, rf.currentTerm)
	rf.role = follower

	rf.votedFor = unvoted
}

func (rf *Raft) lead() {
	if rf.role == leader {
		return
	}
	rf.LogClass(LOG_CLASS_LIFECYCLE, "%d lead at term %d", rf.me, rf.currentTerm)
	rf.role = leader
	lastIndex := rf.len()
	for i := range rf.peers {
		rf.nextIndex[i] = lastIndex + 1
		rf.matchIndex[i] = 0
		rf.connected[i] = true
	}

	// Upon election: send heartbeat to each server to prevent election timeouts
	rf.heartbeat()
}

// If lost from majority of servers: become follower
func (rf *Raft) isConnected() bool {
	connectCount := 0
	for i, v := range rf.connected {
		if v || i == rf.me {
			connectCount++
		}
	}
	return connectCount*2 > len(rf.peers)
}

// Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server; repeat during idle periods to prevent election timeouts
func (rf *Raft) heartbeat() {
	if rf.role != leader {
		return
	}

	rf.LogClass(LOG_CLASS_LIFECYCLE, "%d heartbeats at term %d", rf.me, rf.currentTerm)

	// If last log index ≥ nextIndex for a follower: send AppendEntries RPC with log entries starting at nextIndex
	//   If successful: update nextIndex and matchIndex for follower
	//   If AppendEntries fails because of log inconsistency: decrement nextIndex and retry

	lastLogIndex := rf.len()
	currentTerm := rf.currentTerm

	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(i int) {
			reply := AppendEntriesReply{}

			for rf.role == leader && rf.currentTerm == currentTerm {
				if rf.hasKilled() {
					return
				}

				index, term := rf.prevLogSignature(i)
				args := AppendEntriesArgs{
					Term:         currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: index,
					PrevLogTerm:  term,
					Entries:      rf.logs[index+1:],
					LeaderCommit: rf.commitIndex,
				}

				ok := false
				ok = rf.sendAppendEntries(i, args, &reply)
				rf.connected[i] = ok

				// If lost from majority of servers: become follower
				if !rf.isConnected() {
					rf.LogClass(LOG_CLASS_LIFECYCLE, "%d disconnected at term %d", rf.me, currentTerm)
					rf.InLock(func() { rf.follow() })
				}

				// Sended
				//	 Success -> break
				//   Fail
				//	   heartbeat -> retry
				//     normal -> retry
				// Unsended
				//   heartbeat -> break
				//   normal -> retry

				if ok {
					rf.InLock(func() { rf.checkFollow(reply) })

					if rf.role == leader && rf.currentTerm == currentTerm {
						if reply.Success {
							// If successful: update nextIndex and matchIndex for follower
							if args.isheartbeat() {
								rf.LogClass(LOG_CLASS_LEADING, "Heartbeat success for %d, nextIndex %d, matchIndex %d: %+v", i, rf.nextIndex[i], rf.matchIndex[i], args)
							} else {
								rf.InLock(func() {
									if rf.role != leader || rf.currentTerm != currentTerm || rf.matchIndex[i] >= lastLogIndex {
										return
									}
									rf.nextIndex[i] = lastLogIndex + 1
									rf.matchIndex[i] = lastLogIndex
								})
								rf.LogClass(LOG_CLASS_LEADING, "AppendEntries success for %d, nextIndex -> %d, matchIndex -> %d: %+v", i, rf.nextIndex[i], rf.matchIndex[i], args)
							}

							break
						} else {
							// If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
							if args.isheartbeat() {
								rf.LogClass(LOG_CLASS_LEADING, "Hearbeat fails at %d with term %d: response %+v, reply %+v", args.PrevLogIndex, args.PrevLogTerm, args, reply)
							} else {
								rf.LogClass(LOG_CLASS_LEADING, "AppendEntries fails at %d with term %d: response %+v, reply %+v", args.PrevLogIndex, args.PrevLogTerm, args, reply)
							}

							newNextIndex := index

							if !DISABLE_NEXTINDEX_BIGSTEP {
								// If desired, the protocol can be optimized to reduce the number of rejected AppendEntries RPCs.
								// For example, when rejecting an AppendEntries request, the follower
								// can include the term of the conflicting entry
								// and the first index it stores for that term.
								// With this information, the leader can decrement nextIndex to bypass all of the conflicting entries in that term;
								// one AppendEntries RPC will be required for each term with conflicting entries,
								// rather than one RPC per entry.
								// In practice, we doubt this optimization is necessary, since failures happen infrequently and it is unlikely that there will be many inconsistent entries.

								newNextIndex = reply.ConflictIndex
								if reply.ConflictTerm != notfoundConflictTerm {
									for newNextIndex <= lastLogIndex && rf.logs[newNextIndex].Term == reply.ConflictTerm {
										newNextIndex++
									}
								}
							}

							if newNextIndex < rf.nextIndex[i] {
								rf.InLock(func() {
									if rf.role != leader || rf.currentTerm != currentTerm || newNextIndex >= rf.nextIndex[i] {
										return
									}
									rf.nextIndex[i] = newNextIndex
									rf.LogClass(LOG_CLASS_LEADING, "nextIndex[%d] -> %d", i, rf.nextIndex[i])
								})
							} else { // another coroutine has changed nextIndex[i] to smaller
								rf.LogClass(LOG_CLASS_LEADING, "nextIndex[%d] -> %d (by another coroutine)", i, rf.nextIndex[i])
								break
							}
						}
					}
				} else {
					if args.isheartbeat() {
						break
					}
				}
			}
		}(i)
	}

	if rf.role == leader {
		// repeat during idle periods to prevent election timeouts
		rf.resetHeartbeatTimeout()
	}
}

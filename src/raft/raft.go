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

const (
	LOG_HEARTBEAT = false
)

const (
	unvoted            = -1
	heartbeatTimeout   = time.Duration(100) * time.Millisecond
	electionTimeoutMin = 200 * time.Millisecond
	electionTimeoutMax = 400 * time.Millisecond
)

const (
	follower  = 0
	candidate = 1
	leader    = 2
)

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

	// Persistent state on all servers (Updated on stable storage before responding to RPCs)

	currentTerm int        // latest term server has seen (initialized to 0 on first boot, increases monotonically)
	votedFor    int        // candidateId that received vote in current term (or null if none)
	logs        []LogEntry // log entries

	// Volatile state on all servers

	commitIndex int // index of highest log entry known to be committed (initialized to 0, increases monotonically)
	lastApplied int // index of highest log entry applied to state machine (initialized to 0, increases monotonically)

	// Volatile state on leaders (Reinitialized after election)

	nextIndex  []int // for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	matchIndex []int // for each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically)

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

type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching prevLogIndex and prevLogTerm
}

func (data AppendEntriesReply) term() int {
	return data.Term
}

//
// AppendEntries RPC handler.
//
func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	if len(args.Entries) == 0 {
		rf.Log("Recieve Heartbeat from %d (term: %d)", args.LeaderId, args.Term)
	} else {
		rf.Log("Recieve AppendEntries from %d: from (%d, %d) with %d entries", args.LeaderId, args.PrevLogIndex, args.PrevLogTerm, len(args.Entries))
	}

	rf.checkFollow(args)

	// If AppendEntries RPC received from new leader: convert to follower
	if rf.currentTerm == args.Term {
		rf.follow()
	}

	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		// Reply false if term < currentTerm
		reply.Success = false
	} else { // assume rpc is from current leader
		// receiving AppendEntries RPC from current leader
		rf.resetElectionTimeout()
		if args.PrevLogIndex >= len(rf.logs) {
			// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
			reply.Success = false
		} else {
			if args.PrevLogIndex >= 0 && rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
				// Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
				reply.Success = false

				// If an existing entry conflicts with a new one (same index but different terms)
				// delete the existing entry and all that follow it

				// rf.logs = rf.logs[:args.PrevLogIndex]
			} else { // args.PrevLogIndex == -1 or rf.logs[args.PrevLogIndex].Term == args.PrevLogTerm
				reply.Success = true

				// Append any new entries not already in the log

				firstUnmatch := 0

				for firstUnmatch < len(args.Entries) && args.PrevLogIndex+1+firstUnmatch < len(rf.logs) && rf.logs[args.PrevLogIndex+1+firstUnmatch].Term == args.Entries[firstUnmatch].Term {
					firstUnmatch++
				}

				args.Entries = args.Entries[firstUnmatch:]

				lastNewIndex := args.PrevLogIndex + firstUnmatch

				if len(args.Entries) > 0 {
					if args.PrevLogIndex+1+firstUnmatch < len(rf.logs) {
						rf.logs = rf.logs[:args.PrevLogIndex+1+firstUnmatch]
					}

					rf.logs = append(rf.logs, args.Entries...)

					rf.persist()

					lastNewIndex += len(args.Entries)
				}

				// If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
				if args.LeaderCommit > rf.commitIndex {
					if args.LeaderCommit < lastNewIndex {
						rf.commitIndex = args.LeaderCommit
					} else {
						rf.commitIndex = lastNewIndex
					}
				}
			}
		}
	}

	if len(args.Entries) == 0 {
		rf.Log("Reply Heartbeat from %d: %t", args.LeaderId, reply.Success)
	} else {
		rf.Log("Reply AppendEntries from %d: from (%d, %d) with %d entries: %t", args.LeaderId, args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), reply.Success)
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
		rf.Log("Send Heartbeat to %d", server)
	} else {
		rf.Log("Send AppendEntries from (%d, %d) with %d entries to %d", args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), server)
	}
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	if !ok {
		if len(args.Entries) == 0 {
			rf.Log("Failed to send Heartbeat to %d", server)
		} else {
			rf.Log("Failed to send AppendEntries from (%d, %d) with %d entries to %d", args.PrevLogIndex, args.PrevLogTerm, len(args.Entries), server)
		}
	}
	return ok
}

//
// example RequestVote RPC arguments structure.
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
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	Term        int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

func (data RequestVoteReply) term() int {
	return data.Term
}

func (rf *Raft) Log(format string, v ...interface{}) {
	if !LOG_HEARTBEAT && strings.Contains(format, "Heartbeat") {
		return
	}

	role := "follower"
	switch rf.role {
	case candidate:
		role = "candidate"
	case leader:
		role = "leader"
	}
	rf.logger.Printf("(%s, term: %d) %s\n", role, rf.currentTerm, fmt.Sprintf(format, v...))
}

func (rf *Raft) lastLogSignature() (int, int) {
	index := len(rf.logs) - 1
	term := 0
	if index >= 0 {
		term = rf.logs[index].Term
	}
	return index, term
}

func (rf *Raft) prevLogSignature(server int) (int, int) {
	index := rf.nextIndex[server] - 1
	term := 0
	if index >= 0 {
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

	rf.Log("Recieve RequestVote from %d (term: %d)", args.CandidateId, args.Term)

	rf.checkFollow(args)

	reply.Term = rf.currentTerm

	// Reply false if term < currentTerm
	// If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote
	if args.Term >= rf.currentTerm && (rf.votedFor == unvoted || rf.votedFor == args.CandidateId) && rf.isUpToDate(args.LastLogIndex, args.LastLogTerm) {
		rf.votedFor = args.CandidateId
		rf.persist()

		// granting vote to candidate
		rf.resetElectionTimeout()

		reply.VoteGranted = true
	}

	rf.Log("Reply RequestVote from %d: %t", args.CandidateId, reply.VoteGranted)
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
	rf.Log("Send RequestVote to %d", server)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if !ok {
		rf.Log("Failed to send RequestVote to %d", server)
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
	index := -1
	term := rf.currentTerm
	isLeader := rf.role == leader

	if isLeader {
		rf.Log("Recieve client request.")

		// If command received from client: append entry to local log, respond after entry applied to state machine

		entry := LogEntry{
			Term:  rf.currentTerm,
			Value: command,
		}

		rf.Lock()

		rf.logs = append(rf.logs, entry)
		lastLogIndex := len(rf.logs) - 1
		rf.nextIndex[rf.me] = lastLogIndex + 1
		rf.matchIndex[rf.me] = lastLogIndex

		rf.persist()

		rf.Unlock()

		index = lastLogIndex

		// If last log index ≥ nextIndex for a follower: send AppendEntries RPC with log entries starting at nextIndex
		//   If successful: update nextIndex and matchIndex for follower
		//   If AppendEntries fails because of log inconsistency: decrement nextIndex and retry

		for i := range rf.peers {
			if i == rf.me || lastLogIndex < rf.nextIndex[i] {
				continue
			}
			go func(rf *Raft, i int) {
				for rf.role == leader {
					var reply AppendEntriesReply

					index, term := rf.prevLogSignature(i)
					args := AppendEntriesArgs{
						Term:         rf.currentTerm,
						LeaderId:     rf.me,
						PrevLogIndex: index,
						PrevLogTerm:  term,
						Entries:      rf.logs[rf.nextIndex[i]:],
						LeaderCommit: rf.commitIndex,
					}

					ok := false
					for !ok && rf.role == leader {
						ok = rf.sendAppendEntries(i, args, &reply)
					}
					rf.checkFollow(reply)
					if rf.role == leader {
						if reply.Success {
							// If successful: update nextIndex and matchIndex for follower
							rf.nextIndex[i] = lastLogIndex + 1
							rf.matchIndex[i] = lastLogIndex
							break
						} else {
							// If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
							rf.nextIndex[i]--
						}
					}
				}
			}(rf, i)
		}

		rf.Log("Reply client request: %d.", index)
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
	// Your code here, if desired.
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

	isdebug := os.Getenv("DEBUG") != ""

	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh

	if isdebug {
		rf.logger = log.New(os.Stderr, fmt.Sprintf("Server %d: ", me), log.Ldate|log.Ltime)
	} else {
		rf.logger = log.New(io.Discard, fmt.Sprintf("Server %d: ", me), log.Ldate|log.Ltime)
	}

	rf.currentTerm = 0
	rf.votedFor = unvoted

	rf.logs = make([]LogEntry, 0)

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	rf.heartbeatTimer = time.NewTimer(heartbeatTimeout)
	rf.electionTimer = time.NewTimer(getRandomizedElectionTimeout())

	rf.voteGranted = make([]bool, len(peers))

	rf.role = follower

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.longrun()

	return rf
}

// If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N, and log[N].term == currentTerm: set commitIndex = N
func (rf *Raft) updateCommitIndex() {
	newIndex := rf.commitIndex + 1
	for newIndex < len(rf.logs) && rf.logs[newIndex].Term != rf.currentTerm {
		newIndex++
	}
	if newIndex < len(rf.logs) {
		matchedCount := 0
		for _, v := range rf.matchIndex {
			if v >= newIndex {
				matchedCount++
			}
		}
		// rf.Log("Index %d matched %d of %d", newIndex, matchedCount, len(rf.peers))
		if matchedCount*2 > len(rf.peers) {
			rf.commitIndex = newIndex
			rf.Log("Commit index updated: %d", newIndex)
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
		rf.Log("Applying: %d", cur)

		rf.applyCh <- msg

		rf.lastApplied = cur

		rf.Log("Applied: %d", cur)
	}
}

func (rf *Raft) resetElectionTimeout() {
	rf.electionTimer.Reset(getRandomizedElectionTimeout())
}

func (rf *Raft) resetHeartbeatTimeout() {
	rf.heartbeatTimer.Reset(heartbeatTimeout)
}

func (rf *Raft) Lock() {
	rf.mu.Lock()
	rf.Log("Lock")
}

func (rf *Raft) Unlock() {
	rf.mu.Unlock()
	rf.Log("Unlock")
}

func (rf *Raft) WithLock(f func()) {
	rf.Lock()
	defer rf.Unlock()

	f()

}

func (rf *Raft) longrun() {
	go func(rf *Raft) {
		for {
			<-rf.electionTimer.C
			go rf.election()
		}
	}(rf)

	go func(rf *Raft) {
		for {
			<-rf.heartbeatTimer.C
			go rf.heartbeat()
		}
	}(rf)

	go func(rf *Raft) {
		for {
			rf.apply()
		}
	}(rf)
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

func (rf *Raft) campaign() {
	if rf.role == leader {
		return
	}

	rf.Log("Campaign")
	for i := range rf.voteGranted {
		rf.voteGranted[i] = false
	}
	rf.role = candidate

	// Increment currentTerm
	rf.currentTerm++

	// Vote for self
	rf.votedFor = rf.me

	rf.persist()

	// Reset election timer
	rf.resetElectionTimeout()

	// Send RequestVote RPCs to all other servers
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(rf *Raft, i int) {
			var reply RequestVoteReply
			index, term := rf.lastLogSignature()
			args := RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: index,
				LastLogTerm:  term,
			}

			ok := false
			for !ok && rf.role == candidate {
				ok = rf.sendRequestVote(i, args, &reply)
			}
			rf.checkFollow(reply)
			if rf.role == candidate {
				if reply.VoteGranted {
					rf.voteGranted[i] = true

					// If votes received from majority of servers: become leader
					if rf.isWinner() {
						rf.lead()
					}
				}
			}
		}(rf, i)
	}
}

// If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
func (rf *Raft) checkFollow(data RpcTransferData) {
	if data.term() > rf.currentTerm {
		rf.currentTerm = data.term()
		rf.persist()
		rf.follow()
	}
}

func (rf *Raft) follow() {
	if rf.role == follower {
		return
	}
	rf.Log("Follow")
	rf.role = follower

	rf.votedFor = unvoted
	rf.persist()

	rf.resetElectionTimeout()
}

func (rf *Raft) lead() {
	if rf.role == leader {
		return
	}
	rf.Log("Lead")
	rf.role = leader
	lastIndex := len(rf.logs)
	for i := range rf.nextIndex {
		rf.nextIndex[i] = lastIndex
		rf.matchIndex[i] = 0
	}

	// Upon election: send heartbeat to each server to prevent election timeouts
	rf.heartbeat()
}

// If election timeout elapses: start new election
func (rf *Raft) election() {
	if rf.role == leader {
		return
	}

	rf.campaign()
}

// Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server; repeat during idle periods to prevent election timeouts
func (rf *Raft) heartbeat() {
	if rf.role != leader {
		return
	}

	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(rf *Raft, i int) {
			var reply AppendEntriesReply

			index, term := rf.prevLogSignature(i)
			args := AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: index,
				PrevLogTerm:  term,
				Entries:      make([]LogEntry, 0),
				LeaderCommit: rf.commitIndex,
			}

			ok := false
			for !ok && rf.role == leader {
				ok = rf.sendAppendEntries(i, args, &reply)
			}
			rf.checkFollow(reply)
		}(rf, i)
	}

	if rf.role == leader {
		// repeat during idle periods to prevent election timeouts
		rf.resetHeartbeatTimeout()
	}
}

package serf

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
Serf supports using a "snapshot" file that contains various
transactional data that is used to help Serf recover quickly
and gracefully from a failure. We append member events, as well
as the latest clock values to the file during normal operation,
and periodically checkpoint and roll over the file. During a restore,
we can replay the various member events to recall a list of known
nodes to re-join, as well as restore our clock values to avoid replaying
old events.
*/

const fsyncInterval = 100 * time.Millisecond
const clockUpdateInterval = 500 * time.Millisecond
const tmpExt = ".compact"

// Snapshotter is responsible for ingesting events and persisting
// them to disk, and providing a recovery mechanism at start time.
type Snapshotter struct {
	aliveNodes     map[string]string
	clock          *LamportClock
	fh             *os.File
	inCh           <-chan Event
	lastFsync      time.Time
	lastClock      LamportTime
	lastEventClock LamportTime
	leaveCh        chan struct{}
	leaving        bool
	logger         *log.Logger
	maxSize        int64
	path           string
	offset         int64
	outCh          chan<- Event
	shutdownCh     <-chan struct{}
	waitCh         chan struct{}
}

// PreviousNode is used to represent the previously known alive nodes
type PreviousNode struct {
	Name string
	Addr string
}

func (p PreviousNode) String() string {
	return fmt.Sprintf("%s: %s", p.Name, p.Addr)
}

// NewSnapshotter creates a new Snapshotter that records events up to a
// max byte size before rotating the file. It can also be used to
// recover old state. Snapshotter works by reading an event channel it returns,
// passing through to an output channel, and persisting relevant events to disk.
func NewSnapshotter(path string, maxSize int, logger *log.Logger, clock *LamportClock,
	outCh chan<- Event, shutdownCh <-chan struct{}) (chan<- Event, *Snapshotter, error) {
	inCh := make(chan Event, 1024)

	// Try to open the file
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0755)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot: %v", err)
	}

	// Determine the offset
	info, err := fh.Stat()
	if err != nil {
		fh.Close()
		return nil, nil, fmt.Errorf("failed to stat snapshot: %v", err)
	}
	offset := info.Size()

	// Create the snapshotter
	snap := &Snapshotter{
		aliveNodes:     make(map[string]string),
		clock:          clock,
		fh:             fh,
		inCh:           inCh,
		lastClock:      0,
		lastEventClock: 0,
		leaveCh:        make(chan struct{}),
		logger:         logger,
		maxSize:        int64(maxSize),
		path:           path,
		offset:         offset,
		outCh:          outCh,
		shutdownCh:     shutdownCh,
		waitCh:         make(chan struct{}),
	}

	// Recover the last known state
	if err := snap.replay(); err != nil {
		fh.Close()
		return nil, nil, err
	}

	// Start handling new commands
	go snap.stream()
	return inCh, snap, nil
}

// LastClock returns the last known clock time
func (s *Snapshotter) LastClock() LamportTime {
	return s.lastClock
}

// LastEventClock returns the last known event clock time
func (s *Snapshotter) LastEventClock() LamportTime {
	return s.lastEventClock
}

// AliveNodes returns the last known alive nodes
func (s *Snapshotter) AliveNodes() []*PreviousNode {
	// Copy the previously known
	previous := make([]*PreviousNode, 0, len(s.aliveNodes))
	for name, addr := range s.aliveNodes {
		previous = append(previous, &PreviousNode{name, addr})
	}

	// Randomize the order, prevents hot shards
	for i := range previous {
		j := rand.Intn(i + 1)
		previous[i], previous[j] = previous[j], previous[i]
	}
	return previous
}

// Wait is used to wait until the snapshotter finishes shut down
func (s *Snapshotter) Wait() {
	<-s.waitCh
}

// Leave is used to remove known nodes to prevent a restart from
// causing a join. Otherwise nodes will re-join after leaving!
func (s *Snapshotter) Leave() {
	select {
	case s.leaveCh <- struct{}{}:
	case <-s.shutdownCh:
	}
}

// stream is a long running routine that is used to handle events
func (s *Snapshotter) stream() {
	for {
		select {
		case <-s.leaveCh:
			// Clear the known nodes
			s.aliveNodes = make(map[string]string)
			s.leaving = true
			s.tryAppend("leave\n")
			if err := s.fh.Sync(); err != nil {
				s.logger.Printf("[ERR] serf: failed to sync leave to snapshot: %v", err)
			}

		case e := <-s.inCh:
			// Forward the event immediately
			if s.outCh != nil {
				s.outCh <- e
			}

			// Stop recording events after a leave is issued
			if s.leaving {
				continue
			}
			switch typed := e.(type) {
			case MemberEvent:
				s.processMemberEvent(typed)
			case UserEvent:
				s.processUserEvent(typed)
			default:
				s.logger.Printf("[ERR] serf: Unknown event to snapshot: %#v", e)
			}

		case <-time.After(clockUpdateInterval):
			s.updateClock()

		case <-s.shutdownCh:
			if err := s.fh.Sync(); err != nil {
				s.logger.Printf("[ERR] serf: failed to sync snapshot: %v", err)
			}
			s.fh.Close()
			close(s.waitCh)
			return
		}
	}
}

// processMemberEvent is used to handle a single member event
func (s *Snapshotter) processMemberEvent(e MemberEvent) {
	switch e.Type {
	case EventMemberJoin:
		for _, mem := range e.Members {
			addr := net.TCPAddr{IP: mem.Addr, Port: int(mem.Port)}
			s.aliveNodes[mem.Name] = addr.String()
			s.tryAppend(fmt.Sprintf("alive: %s %s\n", mem.Name, addr.String()))
		}

	case EventMemberLeave:
		fallthrough
	case EventMemberFailed:
		for _, mem := range e.Members {
			delete(s.aliveNodes, mem.Name)
			s.tryAppend(fmt.Sprintf("not-alive: %s\n", mem.Name))
		}
	}
	s.updateClock()
}

// updateClock is called periodically to check if we should udpate our
// clock value. This is done after member events but should also be done
// periodically due to race conditions with join and leave intents
func (s *Snapshotter) updateClock() {
	lastSeen := s.clock.Time() - 1
	if lastSeen > s.lastClock {
		s.lastClock = lastSeen
		s.tryAppend(fmt.Sprintf("clock: %d\n", s.lastClock))
	}
}

// processUserEvent is used to handle a single user event
func (s *Snapshotter) processUserEvent(e UserEvent) {
	// Ignore old clocks
	if e.LTime <= s.lastEventClock {
		return
	}
	s.lastEventClock = e.LTime
	s.tryAppend(fmt.Sprintf("event-clock: %d\n", e.LTime))
}

// tryAppend will invoke append line but will not return an error
func (s *Snapshotter) tryAppend(l string) {
	if err := s.appendLine(l); err != nil {
		s.logger.Printf("[ERR] serf: Failed to update snapshot: %v", err)
	}
}

// appendLine is used to append a line to the existing log
func (s *Snapshotter) appendLine(l string) error {
	n, err := s.fh.WriteString(l)
	if err != nil {
		return err
	}

	// Check if we should fsync
	now := time.Now()
	if now.Sub(s.lastFsync) > fsyncInterval {
		s.lastFsync = now
		if err := s.fh.Sync(); err != nil {
			return err
		}
	}

	// Check if a compaction is necessary
	s.offset += int64(n)
	if s.offset > s.maxSize {
		return s.compact()
	}
	return nil
}

// Compact is used to compact the snapshot once it is too large
func (s *Snapshotter) compact() error {
	// Try to open the file to new fiel
	newPath := s.path + tmpExt
	fh, err := os.OpenFile(newPath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to open new snapshot: %v", err)
	}

	// Write out the live nodes
	var offset int64
	for name, addr := range s.aliveNodes {
		line := fmt.Sprintf("alive: %s %s\n", name, addr)
		n, err := fh.WriteString(line)
		if err != nil {
			fh.Close()
			return err
		}
		offset += int64(n)
	}

	// Write out the clocks
	line := fmt.Sprintf("clock: %d\n", s.lastClock)
	n, err := fh.WriteString(line)
	if err != nil {
		fh.Close()
		return err
	}
	offset += int64(n)

	line = fmt.Sprintf("event-clock: %d\n", s.lastEventClock)
	n, err = fh.WriteString(line)
	if err != nil {
		fh.Close()
		return err
	}
	offset += int64(n)

	// Switch the files
	if err := os.Rename(newPath, s.path); err != nil {
		fh.Close()
		return fmt.Errorf("failed to install new snapshot: %v", err)
	}

	// Rotate our handles
	s.fh.Close()
	s.fh = fh
	s.offset = offset
	s.lastFsync = time.Now()
	return nil
}

// replay is used to seek to reset our internal state by replaying
// the snapshot file. It is used at initialization time to read old
// state
func (s *Snapshotter) replay() error {
	// Seek to the beginning
	if _, err := s.fh.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	// Read each line
	reader := bufio.NewReader(s.fh)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		// Skip the newline
		line = line[:len(line)-1]

		// Switch on the prefix
		if strings.HasPrefix(line, "alive: ") {
			info := strings.TrimPrefix(line, "alive: ")
			addrIdx := strings.LastIndex(info, " ")
			if addrIdx == -1 {
				s.logger.Printf("[WARN] Failed to parse address: %v", line)
				continue
			}
			addr := info[addrIdx+1:]
			name := info[:addrIdx]
			s.aliveNodes[name] = addr

		} else if strings.HasPrefix(line, "not-alive: ") {
			name := strings.TrimPrefix(line, "not-alive: ")
			delete(s.aliveNodes, name)

		} else if strings.HasPrefix(line, "clock: ") {
			timeStr := strings.TrimPrefix(line, "clock: ")
			timeInt, err := strconv.ParseUint(timeStr, 10, 64)
			if err != nil {
				s.logger.Printf("[WARN] Failed to convert clock time: %v", err)
				continue
			}
			s.lastClock = LamportTime(timeInt)

		} else if strings.HasPrefix(line, "event-clock: ") {
			timeStr := strings.TrimPrefix(line, "event-clock: ")
			timeInt, err := strconv.ParseUint(timeStr, 10, 64)
			if err != nil {
				s.logger.Printf("[WARN] Failed to convert event clock time: %v", err)
				continue
			}
			s.lastEventClock = LamportTime(timeInt)

		} else if line == "leave" {
			s.aliveNodes = make(map[string]string)
			s.lastClock = 0
			s.lastEventClock = 0

		} else if strings.HasPrefix(line, "#") {
			// Skip comment lines

		} else {
			s.logger.Printf("[WARN] Unrecognized snapshot line: %v", line)
		}
	}

	// Seek to the end
	if _, err := s.fh.Seek(0, os.SEEK_END); err != nil {
		return err
	}
	return nil
}

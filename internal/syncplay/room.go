package syncplay

import (
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPing       = 500
	maxPing           = 10000
	timeSyncOffset    = 2000 * time.Millisecond
	maxPlaybackOffset = 500 * time.Millisecond
	maxCatchUpOffset  = 60000 * time.Millisecond
	groupWaitTimeout  = 30 * time.Second
	ticksPerMilli     = int64(time.Millisecond / 100)
)

type State string

const (
	Idle    State = "Idle"
	Waiting State = "Waiting"
	Paused  State = "Paused"
	Playing State = "Playing"
)

type catalogue interface {
	runTime(itemID uuid.UUID) int64
	visible(itemIDs []uuid.UUID) bool
}

type Seat struct {
	MemberID   uuid.UUID
	SessionID  uuid.UUID
	UserID     uuid.UUID
	UserName   string
	Ping       int64
	Buffering  bool
	IgnoreWait bool

	changed bool
}

type Room struct {
	ID              uuid.UUID
	Name            string
	State           State
	PositionTicks   int64
	RunTimeTicks    int64
	LastActivity    time.Time
	Queue           Queue
	ResumePlaying   bool
	WaitingFrom     *State
	PositionJumped  bool
	IgnoreBuffering bool
	WaitDeadline    *time.Time
	Seats           []*Seat

	now       time.Time
	catalogue catalogue
	out       []Outgoing
}

type broadcast int

const (
	currentSession broadcast = iota
	allGroup
	allExceptCurrent
	allReady
)

func ticks(d time.Duration) int64 {
	return int64(d / 100)
}

func (r *Room) seat(sessionID uuid.UUID) *Seat {
	for _, seat := range r.Seats {
		if seat.SessionID == sessionID {
			return seat
		}
	}

	return nil
}

func (r *Room) targets(from *Seat, to broadcast) []uuid.UUID {
	if to == currentSession {
		return []uuid.UUID{from.SessionID}
	}

	ids := make([]uuid.UUID, 0, len(r.Seats))
	for _, seat := range r.Seats {
		switch {
		case to == allExceptCurrent && seat == from:
		case to == allReady && seat.Buffering:
		default:
			ids = append(ids, seat.SessionID)
		}
	}

	return ids
}

func (r *Room) sendCommand(from *Seat, to broadcast, command Command) {
	ids := r.targets(from, to)
	if len(ids) == 0 {
		return
	}

	r.out = append(r.out, Outgoing{SessionIDs: ids, Type: CommandMessage, Data: command})
}

func (r *Room) sendUpdate(from *Seat, to broadcast, updateType string, data any) {
	ids := r.targets(from, to)
	if len(ids) == 0 {
		return
	}

	r.out = append(r.out, Outgoing{SessionIDs: ids, Type: UpdateMessage, Data: Update{GroupID: r.ID, Type: updateType, Data: data}})
}

func (r *Room) command(kind CommandKind) Command {
	return Command{
		GroupID:        r.ID,
		PlaylistItemID: r.Queue.PlayingPlaylistID(),
		When:           r.LastActivity.UTC(),
		PositionTicks:  r.PositionTicks,
		Command:        kind,
		EmittedAt:      r.now.UTC(),
	}
}

func (r *Room) playQueueUpdate(reason QueueReason) PlayQueueUpdate {
	start := r.PositionTicks
	isPlaying := r.State == Playing
	if isPlaying {
		start += max(ticks(r.now.Sub(r.LastActivity)), 0)
	}

	return PlayQueueUpdate{
		Reason:             reason,
		LastUpdate:         r.Queue.LastChange.UTC(),
		Playlist:           r.Queue.Playlist(),
		PlayingItemIndex:   r.Queue.PlayingIndex,
		StartPositionTicks: start,
		IsPlaying:          isPlaying,
		ShuffleMode:        r.Queue.ShuffleMode,
		RepeatMode:         r.Queue.RepeatMode,
	}
}

func (r *Room) sendQueue(from *Seat, to broadcast, reason QueueReason) {
	r.sendUpdate(from, to, PlayQueueUpdateType, r.playQueueUpdate(reason))
}

func (r *Room) sendState(from *Seat, reason RequestType) {
	r.sendUpdate(from, allGroup, StateUpdateType, StateUpdate{State: r.State, Reason: reason})
}

func (r *Room) become(state State) {
	r.State = state
	r.ResumePlaying = false
	r.WaitingFrom = nil
	r.PositionJumped = false
	r.IgnoreBuffering = false
	if state != Waiting {
		r.WaitDeadline = nil
	}
}

func (r *Room) restore(previous State) {
	switch previous {
	case Playing, Paused:
		r.become(previous)
	default:
		r.become(Idle)
	}
}

func (r *Room) restartCurrentItem() {
	r.PositionTicks = 0
	r.LastActivity = r.now
}

func (r *Room) sanitize(positionTicks int64) int64 {
	return min(max(positionTicks, 0), r.RunTimeTicks)
}

func (r *Room) highestPing() int64 {
	if len(r.Seats) == 0 {
		return defaultPing
	}

	highest := r.Seats[0].Ping
	for _, seat := range r.Seats[1:] {
		highest = max(highest, seat.Ping)
	}

	return highest
}

func (r *Room) setBuffering(seat *Seat, buffering bool) {
	if seat != nil && seat.Buffering != buffering {
		seat.Buffering = buffering
		seat.changed = true
	}
	r.updateWaitDeadline(false)
}

func (r *Room) setAllBuffering(buffering bool) {
	for _, seat := range r.Seats {
		if seat.Buffering != buffering {
			seat.Buffering = buffering
			seat.changed = true
		}
	}
	r.updateWaitDeadline(buffering)
}

func (r *Room) isBuffering() bool {
	return slices.ContainsFunc(r.Seats, func(seat *Seat) bool { return seat.Buffering && !seat.IgnoreWait })
}

func (r *Room) updateWaitDeadline(fresh bool) {
	if r.State != Waiting || !r.isBuffering() {
		r.WaitDeadline = nil
		return
	}

	if r.WaitDeadline == nil || fresh {
		deadline := r.now.Add(groupWaitTimeout)
		r.WaitDeadline = &deadline
	}
}

func (r *Room) pauseAt() {
	elapsed := r.now.Sub(r.LastActivity)
	r.LastActivity = r.now
	r.PositionTicks += max(ticks(elapsed), 0)
}

func (r *Room) markWaitingFrom(previous State) {
	if r.WaitingFrom == nil {
		r.WaitingFrom = &previous
	}
}

func (r *Room) setPlayQueue(ids []uuid.UUID, position int, startTicks int64) bool {
	if len(ids) == 0 || position < 0 || position >= len(ids) || !r.catalogue.visible(ids) {
		return false
	}

	r.Queue.reset(r.now)
	r.Queue.setPlaylist(ids, r.now)
	r.Queue.setPlayingByIndex(position, r.now)
	r.RunTimeTicks = r.catalogue.runTime(r.Queue.PlayingItemID())
	r.PositionTicks = startTicks
	r.LastActivity = r.now

	return true
}

func (r *Room) refreshRunTime() {
	r.RunTimeTicks = 0
	if id := r.Queue.PlayingItemID(); id != uuid.Nil {
		r.RunTimeTicks = r.catalogue.runTime(id)
	}
}

func (r *Room) join(seat *Seat) {
	if r.seat(seat.SessionID) == nil {
		r.Seats = append(r.Seats, seat)
	}
	r.sessionJoined(r.seat(seat.SessionID))
}

func (r *Room) leave(sessionID uuid.UUID) {
	seat := r.seat(sessionID)
	if seat == nil {
		return
	}

	r.sessionLeaving(seat)
	r.Seats = slices.DeleteFunc(r.Seats, func(s *Seat) bool { return s == seat })
	r.updateWaitDeadline(false)

	r.sendUpdate(seat, currentSession, GroupLeftType, r.ID.String())
	r.sendUpdate(seat, allGroup, UserLeftType, seat.UserName)
}

func (r *Room) sessionJoined(seat *Seat) {
	switch r.State {
	case Idle:
		r.idleStop(r.State, seat)
	case Paused, Playing:
		previous := r.State
		r.become(Waiting)
		r.waitingJoined(previous, seat)
	case Waiting:
		r.waitingJoined(Waiting, seat)
	}
}

func (r *Room) sessionLeaving(seat *Seat) {
	if r.State != Waiting {
		return
	}

	r.markWaitingFrom(Waiting)
	r.setBuffering(seat, false)
	if r.isBuffering() {
		return
	}

	if r.ResumePlaying {
		r.become(Playing)
		r.playing(Request{Type: Unpause}, Waiting, seat)
		return
	}

	r.become(Paused)
}

func (r *Room) waitingJoined(previous State, seat *Seat) {
	r.markWaitingFrom(previous)
	if previous == Playing {
		r.ResumePlaying = true
		r.pauseAt()
	}

	r.sendQueue(seat, currentSession, NewPlaylist)
	r.setBuffering(seat, true)
	r.sendCommand(seat, allReady, r.command(PauseCommand))
}

func (r *Room) handle(request Request, previous State, seat *Seat) {
	switch r.State {
	case Idle:
		r.idle(request, previous, seat)
	case Waiting:
		r.waiting(request, previous, seat)
	case Paused:
		r.paused(request, previous, seat)
	case Playing:
		r.playing(request, previous, seat)
	}
}

func (r *Room) transition(to State, request Request, seat *Seat) {
	previous := r.State
	r.become(to)
	r.handle(request, previous, seat)
}

func (r *Room) common(request Request, seat *Seat) {
	switch request.Type {
	case SetPlaylistItem:
		r.transition(Waiting, request, seat)
	case RemoveFromPlaylist:
		removedPlaying := request.ClearPlayingItem
		if request.ClearPlaylist {
			r.Queue.clear(request.ClearPlayingItem, r.now)
			if request.ClearPlayingItem {
				r.restartCurrentItem()
			}
		} else {
			removedPlaying = r.Queue.remove(request.PlaylistItemIDs, r.now)
			if removedPlaying {
				r.refreshRunTime()
				r.restartCurrentItem()
			}
		}

		r.sendQueue(seat, allGroup, RemoveItems)

		if removedPlaying && !r.Queue.isPlaying() {
			r.transition(Idle, Request{Type: Stop}, seat)
		}
	case MovePlaylistItem:
		if !r.Queue.move(request.PlaylistItemID, request.NewIndex, r.now) {
			log.Printf("syncplay group %s: unable to move playlist item %s", r.ID, request.PlaylistItemID)
			return
		}
		r.sendQueue(seat, allGroup, MoveItem)
	case QueueItems:
		if len(request.ItemIDs) == 0 || !r.catalogue.visible(request.ItemIDs) {
			log.Printf("syncplay group %s: unable to add items to the play queue", r.ID)
			return
		}

		reason := QueueAppended
		if request.Mode == QueueNext {
			r.Queue.queueNext(request.ItemIDs, r.now)
			reason = QueueNextUp
		} else {
			r.Queue.queue(request.ItemIDs, r.now)
		}
		r.sendQueue(seat, allGroup, reason)
	case SetRepeatMode:
		r.Queue.setRepeatMode(request.Repeat, r.now)
		r.sendQueue(seat, allGroup, RepeatModeChanged)
	case SetShuffleMode:
		r.Queue.setShuffleMode(request.Shuffle, r.now)
		r.sendQueue(seat, allGroup, ShuffleModeChanged)
	case Ping:
		if seat != nil {
			seat.Ping = min(max(request.Ping, 0), maxPing)
			seat.changed = true
		}
	case IgnoreWait:
		r.setIgnoreWait(seat, request.IgnoreWait)
	default:
		log.Printf("syncplay group %s: unhandled %s in %s", r.ID, request.Type, r.State)
	}
}

func (r *Room) setIgnoreWait(seat *Seat, ignore bool) {
	if seat != nil && seat.IgnoreWait != ignore {
		seat.IgnoreWait = ignore
		seat.changed = true
	}
	r.updateWaitDeadline(false)
}

func (r *Room) idleStop(previous State, seat *Seat) {
	to := currentSession
	if previous != Idle {
		to = allGroup
	}
	r.sendCommand(seat, to, r.command(StopCommand))
}

func (r *Room) idle(request Request, previous State, seat *Seat) {
	switch request.Type {
	case Play, Unpause, NextItem, PreviousItem:
		r.transition(Waiting, request, seat)
	case Pause, Stop, Seek, Buffer, Ready:
		r.idleStop(previous, seat)
	default:
		r.common(request, seat)
	}
}

func (r *Room) paused(request Request, previous State, seat *Seat) {
	switch request.Type {
	case Play, Seek, Buffer, NextItem, PreviousItem:
		r.transition(Waiting, request, seat)
	case Unpause:
		r.transition(Playing, request, seat)
	case Stop:
		r.transition(Idle, request, seat)
	case Pause:
		if previous == Paused {
			r.sendCommand(seat, currentSession, r.command(PauseCommand))
			return
		}
		r.pauseAt()
		r.sendCommand(seat, allGroup, r.command(PauseCommand))
		r.sendState(seat, request.Type)
	case Ready:
		switch previous {
		case Paused:
			r.sendCommand(seat, currentSession, r.command(PauseCommand))
		case Waiting:
			r.sendCommand(seat, allGroup, r.command(PauseCommand))
			r.sendState(seat, request.Type)
		}
	default:
		r.common(request, seat)
	}
}

func (r *Room) playing(request Request, previous State, seat *Seat) {
	switch request.Type {
	case Play, Seek, NextItem, PreviousItem:
		r.transition(Waiting, request, seat)
	case Buffer:
		if r.IgnoreBuffering {
			return
		}
		r.transition(Waiting, request, seat)
	case Pause:
		r.transition(Paused, request, seat)
	case Stop:
		r.transition(Idle, request, seat)
	case Unpause:
		if previous == Playing {
			r.sendCommand(seat, currentSession, r.command(UnpauseCommand))
			return
		}
		delay := max(r.highestPing()*2, defaultPing)
		r.LastActivity = r.now.Add(time.Duration(delay) * time.Millisecond)
		r.sendCommand(seat, allGroup, r.command(UnpauseCommand))
		r.sendState(seat, request.Type)
	case Ready:
		switch previous {
		case Playing:
			r.sendCommand(seat, currentSession, r.command(UnpauseCommand))
		case Waiting:
			r.sendState(seat, request.Type)
		}
	default:
		r.common(request, seat)
	}
}

func (r *Room) waiting(request Request, previous State, seat *Seat) {
	if request.Type.marksWait() {
		r.markWaitingFrom(previous)
	}

	switch request.Type {
	case Play:
		r.waitingPlay(request, previous, seat)
	case SetPlaylistItem:
		r.waitingSetItem(request, previous, seat)
	case Unpause:
		r.waitingUnpause(request, previous, seat)
	case Pause:
		r.ResumePlaying = false
		r.sendState(seat, request.Type)
	case Stop:
		r.transition(Idle, request, seat)
	case Seek:
		r.waitingSeek(request, previous, seat)
	case Buffer:
		r.waitingBuffer(request, previous, seat)
	case Ready:
		r.waitingReady(request, seat)
	case NextItem, PreviousItem:
		r.waitingStep(request, previous, seat)
	case IgnoreWait:
		r.waitingIgnoreWait(request, seat)
	default:
		r.common(request, seat)
	}
}

func (r *Room) waitingSetItem(request Request, previous State, seat *Seat) {
	r.ResumePlaying = true
	found := r.Queue.setPlayingByPlaylistID(request.PlaylistItemID, r.now)
	r.refreshRunTime()
	r.restartCurrentItem()
	r.PositionJumped = found
	if !found {
		r.restore(previous)
		return
	}
	r.sendQueue(seat, allGroup, SetCurrentItem)
	r.setAllBuffering(true)
}

func (r *Room) waitingSeek(request Request, previous State, seat *Seat) {
	switch previous {
	case Playing:
		r.ResumePlaying = true
	case Paused:
		r.ResumePlaying = false
	}
	r.PositionTicks = r.sanitize(request.PositionTicks)
	r.LastActivity = r.now
	r.PositionJumped = true
	r.sendCommand(seat, allGroup, r.command(SeekCommand))
	r.setAllBuffering(true)
	r.sendState(seat, request.Type)
}

func (r *Room) waitingIgnoreWait(request Request, seat *Seat) {
	r.setIgnoreWait(seat, request.IgnoreWait)
	if r.isBuffering() {
		return
	}
	if r.ResumePlaying {
		r.become(Playing)
		r.playing(Request{Type: Unpause}, Waiting, seat)
		return
	}
	r.become(Paused)
}

func (r *Room) waitingPlay(request Request, previous State, seat *Seat) {
	r.ResumePlaying = true
	set := r.setPlayQueue(request.ItemIDs, request.PlayingItemPosition, request.StartPositionTicks)
	r.PositionJumped = set
	if !set {
		log.Printf("syncplay group %s: unable to set the play queue", r.ID)
		r.restore(previous)
		return
	}

	r.sendQueue(seat, allGroup, NewPlaylist)
	r.setAllBuffering(true)
}

func (r *Room) waitingUnpause(request Request, previous State, seat *Seat) {
	if previous == Idle {
		r.ResumePlaying = true
		r.restartCurrentItem()
		r.PositionJumped = true
		r.sendQueue(seat, allGroup, NewPlaylist)
		r.setAllBuffering(true)
		return
	}

	if !r.ResumePlaying {
		r.ResumePlaying = true
		r.sendState(seat, request.Type)
		return
	}

	r.setAllBuffering(false)
	r.become(Playing)
	r.IgnoreBuffering = true
	r.playing(request, Waiting, seat)
}

func (r *Room) waitingBuffer(request Request, previous State, seat *Seat) {
	if request.PlaylistItemID != r.Queue.PlayingPlaylistID() {
		r.sendQueue(seat, currentSession, SetCurrentItem)
		r.setBuffering(seat, true)
		return
	}

	switch previous {
	case Playing:
		r.ResumePlaying = true
		r.setBuffering(seat, true)
		r.pauseAt()
		r.sendCommand(seat, allReady, r.command(PauseCommand))
	case Paused:
		r.ResumePlaying = false
		r.setBuffering(seat, true)
		r.sendCommand(seat, currentSession, r.command(PauseCommand))
	case Waiting:
		r.setBuffering(seat, true)
		if !r.ResumePlaying {
			r.sendCommand(seat, currentSession, r.command(PauseCommand))
		}
	}

	r.sendState(seat, request.Type)
}

func (r *Room) waitingReady(request Request, seat *Seat) {
	if request.PlaylistItemID != r.Queue.PlayingPlaylistID() {
		r.sendQueue(seat, currentSession, SetCurrentItem)
		r.setBuffering(seat, true)
		return
	}

	elapsed := r.now.Sub(request.When)
	if elapsed > timeSyncOffset || elapsed < -timeSyncOffset {
		log.Printf("syncplay group %s: session %s is not time syncing, ignoring its elapsed time", r.ID, seat.SessionID)
		elapsed = 0
	}
	if !request.IsPlaying {
		elapsed = 0
	}

	requestTicks := r.sanitize(request.PositionTicks)
	delayTicks := r.PositionTicks - (requestTicks + ticks(elapsed))

	if r.ResumePlaying {
		r.waitingReadyToResume(request, seat, delayTicks)
		return
	}

	if abs(r.PositionTicks-requestTicks) > ticks(maxPlaybackOffset) {
		r.setBuffering(seat, true)
		r.sendCommand(seat, currentSession, r.command(SeekCommand))
		r.sendState(seat, request.Type)
		return
	}

	r.setBuffering(seat, false)
	if r.isBuffering() {
		return
	}

	from := Idle
	if r.WaitingFrom != nil {
		from = *r.WaitingFrom
	}

	r.become(Paused)
	switch from {
	case Playing:
		r.paused(Request{Type: Pause}, Waiting, seat)
	case Paused:
		r.paused(request, Waiting, seat)
	}
}

func (r *Room) waitingReadyToResume(request Request, seat *Seat, delayTicks int64) {
	maxOffset := ticks(maxPlaybackOffset)
	if request.IsPlaying && !r.PositionJumped {
		maxOffset = ticks(maxCatchUpOffset)
	}

	if abs(delayTicks) > maxOffset {
		r.setBuffering(seat, true)
		r.sendCommand(seat, currentSession, r.command(SeekCommand))
		r.sendState(seat, request.Type)
		return
	}

	r.setBuffering(seat, false)
	if r.isBuffering() {
		pause := r.command(PauseCommand)
		pause.When = r.now.Add(time.Duration(delayTicks * 100)).UTC()
		r.sendCommand(seat, currentSession, pause)
		return
	}

	if delayTicks > r.highestPing()*2*ticksPerMilli {
		r.LastActivity = r.now.Add(time.Duration(delayTicks * 100))
		to := allExceptCurrent
		if !request.IsPlaying {
			to = allGroup
		}
		r.sendCommand(seat, to, r.command(UnpauseCommand))
	} else {
		delayTicks = max(r.highestPing()*2*ticksPerMilli, defaultPing*ticksPerMilli)
		r.LastActivity = r.now.Add(time.Duration(delayTicks * 100))
		r.sendCommand(seat, allGroup, r.command(UnpauseCommand))
	}

	r.become(Playing)
	r.playing(request, Waiting, seat)
}

func (r *Room) waitingStep(request Request, previous State, seat *Seat) {
	r.ResumePlaying = true
	if request.PlaylistItemID != r.Queue.PlayingPlaylistID() {
		return
	}

	reason := NextItemReason
	step := r.Queue.next
	if request.Type == PreviousItem {
		reason = PreviousItemReason
		step = r.Queue.previous
	}

	moved := step(r.now)
	r.PositionJumped = moved
	if !moved {
		r.restore(previous)
		return
	}

	r.refreshRunTime()
	r.restartCurrentItem()
	r.sendQueue(seat, allGroup, reason)
	r.setAllBuffering(true)
}

func (r *Room) timeout() {
	if r.WaitDeadline == nil || r.WaitDeadline.After(r.now) {
		return
	}

	r.WaitDeadline = nil
	if r.State != Waiting {
		return
	}

	var blocking *Seat
	for _, seat := range r.Seats {
		if seat.Buffering && !seat.IgnoreWait {
			blocking = seat
			break
		}
	}
	if blocking == nil {
		return
	}

	log.Printf("syncplay group %s: gave up waiting for session %s to report ready", r.ID, blocking.SessionID)

	if r.ResumePlaying {
		r.waiting(Request{Type: Unpause}, Waiting, blocking)
		return
	}

	r.setAllBuffering(false)
	r.become(Paused)
	r.sendCommand(blocking, allGroup, r.command(PauseCommand))
	r.sendUpdate(blocking, allGroup, StateUpdateType, StateUpdate{State: Paused, Reason: Pause})
}

func abs(value int64) int64 {
	if value < 0 {
		return -value
	}

	return value
}

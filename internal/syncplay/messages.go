package syncplay

import (
	"time"

	"github.com/google/uuid"
)

const (
	CommandMessage = "SyncPlayCommand"
	UpdateMessage  = "SyncPlayGroupUpdate"

	PlayQueueUpdateType = "PlayQueue"
	StateUpdateType     = "StateUpdate"
	GroupLeftType       = "GroupLeft"
	UserLeftType        = "UserLeft"
)

type RequestType string

const (
	Play               RequestType = "Play"
	SetPlaylistItem    RequestType = "SetPlaylistItem"
	RemoveFromPlaylist RequestType = "RemoveFromPlaylist"
	MovePlaylistItem   RequestType = "MovePlaylistItem"
	QueueItems         RequestType = "Queue"
	Unpause            RequestType = "Unpause"
	Pause              RequestType = "Pause"
	Stop               RequestType = "Stop"
	Seek               RequestType = "Seek"
	Buffer             RequestType = "Buffer"
	Ready              RequestType = "Ready"
	NextItem           RequestType = "NextItem"
	PreviousItem       RequestType = "PreviousItem"
	SetRepeatMode      RequestType = "SetRepeatMode"
	SetShuffleMode     RequestType = "SetShuffleMode"
	Ping               RequestType = "Ping"
	IgnoreWait         RequestType = "IgnoreWait"
)

func (t RequestType) marksWait() bool {
	switch t {
	case Play, SetPlaylistItem, Unpause, Pause, Stop, Seek, Buffer, Ready, NextItem, PreviousItem:
		return true
	default:
		return false
	}
}

type Request struct {
	Type RequestType

	ItemIDs             []uuid.UUID
	PlayingItemPosition int
	StartPositionTicks  int64
	Mode                QueueMode

	PlaylistItemID   uuid.UUID
	PlaylistItemIDs  []uuid.UUID
	ClearPlaylist    bool
	ClearPlayingItem bool
	NewIndex         int

	Repeat  RepeatMode
	Shuffle ShuffleMode

	PositionTicks int64
	When          time.Time
	IsPlaying     bool

	Ping       int64
	IgnoreWait bool
}

type CommandKind string

const (
	UnpauseCommand CommandKind = "Unpause"
	PauseCommand   CommandKind = "Pause"
	StopCommand    CommandKind = "Stop"
	SeekCommand    CommandKind = "Seek"
)

type Command struct {
	GroupID        uuid.UUID   `json:"GroupId"`
	PlaylistItemID uuid.UUID   `json:"PlaylistItemId"`
	When           time.Time   `json:"When"`
	PositionTicks  int64       `json:"PositionTicks"`
	Command        CommandKind `json:"Command"`
	EmittedAt      time.Time   `json:"EmittedAt"`
}

type QueueReason string

const (
	NewPlaylist        QueueReason = "NewPlaylist"
	SetCurrentItem     QueueReason = "SetCurrentItem"
	RemoveItems        QueueReason = "RemoveItems"
	MoveItem           QueueReason = "MoveItem"
	QueueAppended      QueueReason = "Queue"
	QueueNextUp        QueueReason = "QueueNext"
	NextItemReason     QueueReason = "NextItem"
	PreviousItemReason QueueReason = "PreviousItem"
	RepeatModeChanged  QueueReason = "RepeatMode"
	ShuffleModeChanged QueueReason = "ShuffleMode"
)

type PlayQueueUpdate struct {
	Reason             QueueReason `json:"Reason"`
	LastUpdate         time.Time   `json:"LastUpdate"`
	Playlist           []QueueItem `json:"Playlist"`
	PlayingItemIndex   int         `json:"PlayingItemIndex"`
	StartPositionTicks int64       `json:"StartPositionTicks"`
	IsPlaying          bool        `json:"IsPlaying"`
	ShuffleMode        ShuffleMode `json:"ShuffleMode"`
	RepeatMode         RepeatMode  `json:"RepeatMode"`
}

type StateUpdate struct {
	State  State       `json:"State"`
	Reason RequestType `json:"Reason"`
}

type Update struct {
	GroupID uuid.UUID `json:"GroupId"`
	Type    string    `json:"Type"`
	Data    any       `json:"Data"`
}

type Outgoing struct {
	SessionIDs []uuid.UUID
	Type       string
	Data       any
}

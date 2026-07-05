package notification

import (
	"fmt"
	"time"
)

type Row struct {
	ID               string
	CreatedAt        time.Time
	NotifieeID       string
	NotifierID       *string
	Type             string
	IsRead           bool
	NoteID           *string
	Reaction         *string
	Choice           *int
	CustomBody       *string
	CustomHeader     *string
	CustomIcon       *string
	AppAccessTokenID *string
	Achievement      *string
}

type Payload struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	NotifieeID       string    `json:"notifieeId"`
	NotifierID       *string   `json:"notifierId,omitempty"`
	Type             string    `json:"type"`
	IsRead           bool      `json:"isRead"`
	NoteID           *string   `json:"noteId,omitempty"`
	Reaction         *string   `json:"reaction,omitempty"`
	CustomBody       *string   `json:"customBody,omitempty"`
	CustomHeader     *string   `json:"customHeader,omitempty"`
	CustomIcon       *string   `json:"customIcon,omitempty"`
	AppAccessTokenID *string   `json:"appAccessTokenId,omitempty"`
	Achievement      *string   `json:"achievement,omitempty"`
}

func (r Row) ToPayload() (Payload, error) {
	if !isSupportedType(r.Type) {
		return Payload{}, fmt.Errorf("unsupported notification type %q", r.Type)
	}

	return Payload{
		ID:               r.ID,
		CreatedAt:        r.CreatedAt,
		NotifieeID:       r.NotifieeID,
		NotifierID:       r.NotifierID,
		Type:             r.Type,
		IsRead:           r.IsRead,
		NoteID:           r.NoteID,
		Reaction:         r.Reaction,
		CustomBody:       r.CustomBody,
		CustomHeader:     r.CustomHeader,
		CustomIcon:       r.CustomIcon,
		AppAccessTokenID: r.AppAccessTokenID,
		Achievement:      r.Achievement,
	}, nil
}

func isSupportedType(notificationType string) bool {
	switch notificationType {
	case "note",
		"follow",
		"mention",
		"reply",
		"renote",
		"quote",
		"reaction",
		"pollEnded",
		"scheduledNotePosted",
		"scheduledNotePostFailed",
		"receiveFollowRequest",
		"followRequestAccepted",
		"roleAssigned",
		"chatRoomInvitationReceived",
		"achievementEarned",
		"exportCompleted",
		"login",
		"createToken",
		"app",
		"test",
		"reaction:grouped",
		"renote:grouped":
		return true
	default:
		return false
	}
}

package main

import (
	"fmt"
	"sync"

	guuid "github.com/google/uuid"
	"github.com/livekit/protocol/ingress"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/tinyzimmer/go-gst/gst"
)

type ov3Ingress struct {
	sync.RWMutex
	room            *ov3Room
	participantName string
	ingressId       string
	roomSvc         *lksdk.Room
	mainPub         *ov3Publisher
	screenSharePub  *ov3Publisher

	connected bool
}

func (ing *ov3Ingress) lkReconnecting() {
	root.logger.Debugw("lkReconnecting")
}

func (ing *ov3Ingress) lkReconnected() {
	root.logger.Debugw("lkReconnected")

	// FIXME: reconnect the ingress and all of its tracks
}

func (ing *ov3Ingress) lkDisconnected() {
	root.logger.Debugw("lkDisconnected")

	// FIXME: disconnect the ingress and all of its tracks
}

func (ing *ov3Ingress) lkTrackMuted(pub lksdk.TrackPublication, p lksdk.Participant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackMuted: %s from %s", pub.SID(), p.Identity()))

	// FIXME: If track belongs to this ingress, close flow to appsink
}

func (ing *ov3Ingress) lkTrackUnmuted(pub lksdk.TrackPublication, p lksdk.Participant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackUnmuted: %s from %s", pub.SID(), p.Identity()))

	// FIXME: If tracks belongs to this ingress, reopen flow to the appsink
}

func (ing *ov3Ingress) makeRoomIngressConnection(ingressId string, participant string) error {
	root.logger.Debugw(fmt.Sprintf("makeRoomIngressConnection: with ingressId %s from %s", ingressId, participant))
	room := ing.room
	token, err := ingress.BuildIngressToken(room.service.key, room.service.secret, room.room, ingressId, participant)
	if err != nil {
		return err
	}
	room.token = token
	cb := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackMuted:   ing.lkTrackMuted,
			OnTrackUnmuted: ing.lkTrackUnmuted,
		},
		OnDisconnected: ing.lkDisconnected,
		OnReconnecting: ing.lkReconnecting,
		OnReconnected:  ing.lkReconnected,
	}
	ing.roomSvc = lksdk.CreateRoom(cb)
	if err := ing.roomSvc.JoinWithToken(room.service.url, room.token, lksdk.WithAutoSubscribe(false)); err != nil {
		return err
	}
	return nil
}

func (ing *ov3Ingress) RemovePublisher(screenShare bool) error {
	var pub *ov3Publisher

	if screenShare {
		pub = ing.screenSharePub
	} else {
		pub = ing.mainPub
	}

	if pub != nil {
		audioPublisher := pub.audioPublisher
		if audioPublisher != nil {
			audioPublisher.UnpublishLocalTrack()
			pub.removeTrackPublisher(audioPublisher)
			pub.audioPublisher = nil
		}
		videoPublisher := pub.videoPublisher
		if videoPublisher != nil {
			videoPublisher.UnpublishLocalTrack()
			pub.removeTrackPublisher(videoPublisher)
			pub.videoPublisher = nil
		}
	}

	return nil
}

func (ing *ov3Ingress) CreatePublisher(audioSink *gst.Bin, videoSink *gst.Bin, screenShare bool) (*ov3Publisher, error) {
	publisher := &ov3Publisher{}
	publisher.id = guuid.New().String()
	publisher.ingress = ing
	if audioSink != nil {
		publisher.audioPublisher = publisher.createTrackPublisher(lksdk.TrackKindAudio, audioSink, ing.ingressId, screenShare)
	}
	if videoSink != nil {
		publisher.videoPublisher = publisher.createTrackPublisher(lksdk.TrackKindVideo, videoSink, ing.ingressId, screenShare)
	}

	return publisher, nil
}

func (ing *ov3Ingress) UnpublishScreenShare() (string, error) {
	err := ing.RemovePublisher(true)

	if err == nil {
		result := ing.screenSharePub.id
		ing.screenSharePub = nil
		return result, nil
	} else {
		return "", err
	}
}

func (ing *ov3Ingress) UnpublishMain() (string, error) {
	err := ing.RemovePublisher(false)

	if err == nil {
		result := ing.mainPub.id
		ing.mainPub = nil
		return result, nil
	} else {
		return "", err
	}
}

func (ing *ov3Ingress) PublishScreenShare(audioSink *gst.Bin, videoSink *gst.Bin) (*ov3Publisher, error) {
	var err error

	ing.screenSharePub, err = ing.CreatePublisher(audioSink, videoSink, true)

	if err != nil {
		return nil, err
	}

	return ing.screenSharePub, nil
}

func (ing *ov3Ingress) PublishMain(audioSink *gst.Bin, videoSink *gst.Bin) (*ov3Publisher, error) {
	var err error

	ing.mainPub, err = ing.CreatePublisher(audioSink, videoSink, false)

	if err != nil {
		return nil, err
	}

	return ing.mainPub, nil
}

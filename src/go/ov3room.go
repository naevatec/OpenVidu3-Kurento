package main

import (
	"fmt"
	"sync"

	"github.com/livekit/protocol/egress"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/webrtc/v3"
	"github.com/tinyzimmer/go-gst/gst"
)

type ov3Room struct {
	sync.RWMutex
	service         *ov3Service
	room            string
	token           string
	egressId        string
	roomSvc         *lksdk.Room
	subscriptions   map[string]*ov3Subscription
	ssSubscriptions map[string]*ov3Subscription
	connected       bool

	roomClient *lksdk.RoomServiceClient

	ingress map[string]*ov3Ingress
}

// This must be called with room lock held
func (room *ov3Room) getSubscription(participantId string, screenShare bool) *ov3Subscription {
	var result *ov3Subscription

	if screenShare {
		result = room.ssSubscriptions[participantId]
	} else {
		result = room.subscriptions[participantId]
	}
	return result
}

// This must be called with room lock held
func (room *ov3Room) addSubscription(participantId string, screenShare bool) *ov3Subscription {
	room.connectServiceClient()

	subscription := ov3Subscription{}
	subscription.room = room
	subscription.participant = participantId
	subscription.isScreenShare = screenShare
	subscription.egressId = room.egressId
	subscription.audioTrack = nil
	subscription.videoTrack = nil
	subscription.audioWriter = nil
	subscription.videoWriter = nil
	subscription.subscribers = make([]*ov3Subscriber, 0)

	if screenShare {
		room.ssSubscriptions[participantId] = &subscription
	} else {
		room.subscriptions[participantId] = &subscription
	}

	root.logger.Infow(fmt.Sprintf("addSubscription: creating new subscription to paticipant %s using screeshare %t", participantId, screenShare))
	return &subscription
}

// This must be called with room lock held
func (room *ov3Room) removeSubscription(participantId string, screenShare bool) {
	root.logger.Debugw(fmt.Sprintf("removeSubscription: participant %s using screenshare %t", participantId, screenShare))
	if screenShare {
		delete(room.ssSubscriptions, participantId)
	} else {
		delete(room.subscriptions, participantId)
	}
}

func (room *ov3Room) connectServiceClient() {
	if room.roomClient == nil {
		room.roomClient = lksdk.NewRoomServiceClient(room.service.url, room.service.key, room.service.secret)
	}
}

func (room *ov3Room) addIngress(ingress *ov3Ingress) {
	room.Lock()
	defer room.Unlock()

	room.connectServiceClient()

	// We keep one ingress per published participantId
	room.ingress[ingress.ingressId] = ingress

	// But to ease location we keep a cache per ingressId
	root.addIngress(ingress.ingressId, ingress)
}

func (room *ov3Room) deleteIngress(ingressId string) {
	room.Lock()
	defer room.Unlock()

	delete(room.ingress, ingressId)

	root.deleteIngress(ingressId)

}

func (room *ov3Room) getIngressByParticipant(partcipantId string) *ov3Ingress {
	room.Lock()
	defer room.Unlock()

	result := room.ingress[partcipantId]

	return result
}

func (room *ov3Room) addSubscriber(participantId string, screenShare bool, audioSource *gst.Bin, videoSource *gst.Bin) *ov3Subscriber {
	room.Lock()
	subscription := room.getSubscription(participantId, screenShare)
	if subscription == nil {
		subscription = room.addSubscription(participantId, screenShare)
		subscription.makeSubscription()
		subscription.subscribeToParticipant()
	}

	subscriber := subscription.addSubscriber(audioSource, videoSource)
	room.Unlock()
	subscription.buildSubscriber(subscriber)

	root.logger.Debugw(fmt.Sprintf("addSubscriber: participant %s using screenshare %t with id %s", participantId, screenShare, subscriber.id))
	return subscriber
}

func (room *ov3Room) removeSubscriber(subscriber *ov3Subscriber) {
	root.logger.Debugw(fmt.Sprintf("removeSubscriber: %s", subscriber.id))
	room.Lock()
	subscription := subscriber.subscription
	if subscription == nil {
		room.Unlock()
		return
	}
	subscriber.DestroySubscriber()
	subscription.removeSubscriber(subscriber.id)
	room.Unlock()
}

func (lk *ov3Room) checkTrackSubscription(subs *ov3Subscription, participant *lksdk.RemoteParticipant) {
	if (subs == nil) || (participant == nil) {
		return
	}
	oldAudioTrack := subs.audioTrack
	oldVideoTrack := subs.videoTrack
	subs.checkTracksToSubscribe(participant)
	if subs.audioTrack != oldAudioTrack {
		subs.updateSubscription(oldAudioTrack, subs.audioTrack)
	}
	if subs.videoTrack != oldVideoTrack {
		subs.updateSubscription(oldVideoTrack, subs.videoTrack)
	}
}

func (lk *ov3Room) lkParticipantConnected(participant *lksdk.RemoteParticipant) {
	root.logger.Debugw(fmt.Sprintf("lkParticipantConnected: %s", participant.Identity()))

	// Check if some pending subscription, perhaps track published has arrived before
	// this event
	lk.Lock()
	defer lk.Unlock()

	subs := lk.getSubscription(participant.Identity(), false)
	if subs != nil {
		lk.checkTrackSubscription(subs, participant)
	}
	subsSS := lk.getSubscription(participant.Identity(), false)
	if subsSS != nil {
		lk.checkTrackSubscription(subsSS, participant)
	}
}

func (lk *ov3Room) lkParticipantDisconnected(participant *lksdk.RemoteParticipant) {
	root.logger.Debugw(fmt.Sprintf("lkParticipantDisconnected: %s", participant.Identity()))
}

func (room *ov3Room) lkReconnecting() {
	root.logger.Debugw("lkReconnecting")
}

func (room *ov3Room) lkReconnected() {
	root.logger.Debugw("lkReconnected")
}

func (room *ov3Room) lkDisconnected() {
	root.logger.Debugw("lkDisconnected")
	egressId := room.egressId
	if len(room.subscriptions) > 0 {
		for _, subs := range room.subscriptions {
			subs.removeSubscription()
		}
	}
	if len(room.ssSubscriptions) > 0 {
		for _, subs := range room.ssSubscriptions {
			subs.removeSubscription()
		}
	}

	// We have been disconnected, so we remove the connection from table
	if egressId != "" {
		disconnectFromRoomEgressImpl(egressId)
	}
}

func (lk *ov3Room) lkActiveSpeakersChanged(participants []lksdk.Participant) {
	// No need yet to do anything here
	// FIXME: implement this as it may help to organize possible MCUs layout
	root.logger.Debugw("lkActiveSpeakersChanged")
}

func (lk *ov3Room) lkTrackMuted(pub lksdk.TrackPublication, p lksdk.Participant) {
	root.logger.Debugw("lkTrackMuted: %s from %s", pub.SID(), p.Identity())
	subscription := root.getSubscribedTrack(pub.SID())

	if subscription == nil {
		return
	}
	audioTrack := subscription.audioTrack
	videoTrack := subscription.videoTrack
	if (audioTrack != nil) && (audioTrack.trackId == pub.SID()) {
		w := subscription.audioWriter
		if w != nil {
			w.SetTrackMuted(true)
		}
	} else if (videoTrack != nil) && (videoTrack.trackId == pub.SID()) {
		w := subscription.videoWriter
		if w != nil {
			w.SetTrackMuted(true)
		}
	}
}

func (lk *ov3Room) lkTrackUnmuted(pub lksdk.TrackPublication, p lksdk.Participant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackUnmuted: %s from %s", pub.SID(), p.Identity()))
	subscription := root.getSubscribedTrack(pub.SID())

	if subscription == nil {
		return
	}
	audioTrack := subscription.audioTrack
	videoTrack := subscription.videoTrack
	if audioTrack != nil {
		if audioTrack.trackId == pub.SID() {
			subscription.audioWriter.SetTrackMuted(false)
		}
	}
	if videoTrack != nil {
		if videoTrack.trackId == pub.SID() {
			subscription.videoWriter.SetTrackMuted(false)
		}
	}
}

func (lk *ov3Room) lkMetadataChanged(oldMetadata string, p lksdk.Participant) {
	// No need yet to do anything here
	root.logger.Debugw(fmt.Sprintf("lkMetadataChanged: old %s", oldMetadata))
}

func (lk *ov3Room) lkIsSpeakingChanged(p lksdk.Participant) {
	// No need yet to do anything here
	// FIXME: implement this as it may help to organize possible MCUs layout
	root.logger.Debugw("lkIsSpeakingChanged")
}

func (lk *ov3Room) lkTrackSubscribed(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackSubscribed: %s", pub.SID()))
	subscription := root.getSubscribedTrack(pub.SID())

	if subscription == nil {
		root.logger.Debugw(fmt.Sprintf("lkTrackSubscribed: no requested subscription to %s", pub.SID()))
		return
	}

	subscription.Lock()
	err := subscription.createWriter(track, pub, rp)
	if err != nil {
		root.logger.Infow(fmt.Sprintf("lkTrackSubscribed error %s creating writer for track %s", err.Error(), pub.SID()))
	} else {
		if pub.Kind() == "audio" {
			audioTrack := subscription.audioTrack
			if audioTrack != nil {
				audioTrack.subscribed = true
			}
		} else if pub.Kind() == "video" {
			videoTrack := subscription.videoTrack
			if videoTrack != nil {
				videoTrack.subscribed = true
			}
		}
	}
	subscription.Unlock()
}

func (lk *ov3Room) lkTrackUnsubscribed(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackUnsubscribed:  %s from %s", pub.SID(), rp.Identity()))
	subscription := root.getSubscribedTrack(pub.SID())

	if subscription == nil {
		return
	}
	root.logger.Debugw(fmt.Sprintf("lkTrackUnsubscribed: marking as unsubscribed  %s from %s", pub.SID(), rp.Identity()))
	subscription.Lock()
	if pub.Kind() == "audio" {
		audioTrack := subscription.audioTrack
		if audioTrack != nil {
			audioTrack.subscribed = false
		}
		writer := subscription.audioWriter
		if writer != nil {
			writer.endStream.Break()
			subscription.audioWriter = nil
		}
	} else if pub.Kind() == "video" {
		videoTrack := subscription.videoTrack
		if videoTrack != nil {
			videoTrack.subscribed = false
		}
		writer := subscription.videoWriter
		if writer != nil {
			writer.endStream.Break()
			subscription.videoWriter = nil
		}
	}
	subscription.Unlock()

}

func (lk *ov3Room) lkTrackSubscriptionFailed(sid string, rp *lksdk.RemoteParticipant) {
	//FIXME: retry subscription (timeout included)
	root.logger.Debugw(fmt.Sprintf("lkTrackSubscriptionFailed: %s from %s", sid, rp.Identity()))
}

func (lk *ov3Room) lkTrackPublished(pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	root.logger.Debugw(fmt.Sprintf("lkTrackPublished: %s from %s", pub.SID(), rp.Identity()))
	source := pub.Source()
	switch source {
	case livekit.TrackSource_CAMERA, livekit.TrackSource_MICROPHONE:
		subs := lk.getSubscription(rp.Identity(), false)
		if subs != nil {
			subs.room.Lock()
			root.logger.Debugw(fmt.Sprintf("lkTrackPublished: subscribing to track %s", pub.SID()))
			lk.checkTrackSubscription(subs, rp)
			subs.room.Unlock()
		}
	case livekit.TrackSource_SCREEN_SHARE, livekit.TrackSource_SCREEN_SHARE_AUDIO:
		subs := lk.getSubscription(rp.Identity(), true)
		if subs != nil {
			root.logger.Debugw(fmt.Sprintf("lkTrackPublished: subscribing to SS track %s", pub.SID()))
			subs.room.Lock()
			lk.checkTrackSubscription(subs, rp)
			subs.room.Unlock()
		}
	default:
		root.logger.Debugw(fmt.Sprintf("lkTrackPublished: ignoring participant tSetTrackMutedremoveSubscriberrack from source %s", pub.Source()))
		return
	}

}

func (lk *ov3Room) lkTrackUnpublished(publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	//FIXME: if track correspond to the ones willing to subscribe, do a unsubscription
	root.logger.Debugw(fmt.Sprintf("lkTrackUnpublished:  %s from %s", publication.SID(), rp.Identity()))
	subscription := root.getSubscribedTrack(publication.SID())

	if subscription == nil {
		return
	}

	root.logger.Debugw(fmt.Sprintf("lkTrackUnpublished: unsubscrbing from  %s", publication.SID()))
	subscription.room.Lock()
	subscription.unsubscribe(publication)
	subscription.room.Unlock()
}

func (room *ov3Room) makeRoomConnection(egressId string) error {
	root.logger.Debugw(fmt.Sprintf("makeRoomConnection: with egressId %s to %s", egressId, room.service.url))
	token, err := egress.BuildEgressToken(egressId, room.service.key, room.service.secret, room.room)
	if err != nil {
		return err
	}
	room.token = token
	cb := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackMuted:              room.lkTrackMuted,
			OnTrackUnmuted:            room.lkTrackUnmuted,
			OnMetadataChanged:         room.lkMetadataChanged,
			OnIsSpeakingChanged:       room.lkIsSpeakingChanged,
			OnTrackSubscribed:         room.lkTrackSubscribed,
			OnTrackUnsubscribed:       room.lkTrackUnsubscribed,
			OnTrackSubscriptionFailed: room.lkTrackSubscriptionFailed,
			OnTrackPublished:          room.lkTrackPublished,
			OnTrackUnpublished:        room.lkTrackUnpublished,
		},
		OnDisconnected:            room.lkDisconnected,
		OnReconnecting:            room.lkReconnecting,
		OnReconnected:             room.lkReconnected,
		OnParticipantConnected:    room.lkParticipantConnected,
		OnParticipantDisconnected: room.lkParticipantDisconnected,
		OnActiveSpeakersChanged:   room.lkActiveSpeakersChanged,
	}
	room.roomSvc = lksdk.CreateRoom(cb)
	if err := room.roomSvc.JoinWithToken(room.service.url, room.token, lksdk.WithAutoSubscribe(false)); err != nil {
		return err
	}
	return nil
}

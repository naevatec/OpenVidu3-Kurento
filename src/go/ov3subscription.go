package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	guuid "github.com/google/uuid"
	"github.com/livekit/egress/pkg/types"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type ov3Subscription struct {
	sync.RWMutex
	room          *ov3Room
	audioWriter   *AppWriter
	videoWriter   *AppWriter
	participant   string
	isScreenShare bool
	egressId      string
	audioTrack    *lkTrack
	videoTrack    *lkTrack
	subscribers   []*ov3Subscriber
}

type lkTrack struct {
	subscription *ov3Subscription
	trackId      string
	trackType    lksdk.TrackKind
	trackSource  livekit.TrackSource
	track        *lksdk.RemoteTrackPublication
	subscribed   bool
}

func (subs *ov3Subscription) removeSubscription() {
	for _, subscriber := range subs.subscribers {
		unsubscribeParticipantImpl(subscriber.id)
	}
}

// Must be called with subscription Logck held
func (subs *ov3Subscription) createWriter(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) error {
	trackKind := pub.Kind()
	if trackKind == "audio" {
		if subs.audioWriter != nil {
			root.logger.Infow(fmt.Sprintf("createWriter: audio writer already created for track %s", pub.SID()))
			return nil
		}
	} else if trackKind == "video" {
		if subs.videoWriter != nil {
			root.logger.Infow(fmt.Sprintf("createWriter: video writer already created for track %s", pub.SID()))
			return nil
		}
	}

	root.logger.Infow(fmt.Sprintf("createWriter: creating %s writer for track %s, SSRC (%d)", pub.Kind(), pub.SID(), track.SSRC()))
	w := &AppWriter{
		logger:       root.logger, //logger.GetLogger().WithValues("trackID", track.ID(), "kind", track.Kind().String()),
		track:        track,
		pub:          pub,
		kind:         pub.Kind(),
		codec:        types.MimeType(strings.ToLower(track.Codec().MimeType)),
		PayloadType:  track.Codec().PayloadType,
		ClockRate:    track.Codec().ClockRate,
		subscription: subs,
		nullSamples:  0,
		lastPLI:      time.Now().Add(-10 * time.Second), // First PLI should not be stopped by rate limiter
		dropping:     false,
		retransmit:   make(chan uint16, 20),
		rtx:          true,
	}

	pub.OnRTCP(func(pkt rtcp.Packet) {
		for _, ssrc := range pkt.DestinationSSRC() {
			if ssrc == uint32(track.SSRC()) {
				/*root.logger.Debugw(fmt.Sprintf("Writer.onRTCP: received RTCP %T for %d on track %s with SSRC (%d)", pkt, ssrc,
				pub.SID(), track.SSRC()))*/
				w.PushRTCPPacket(pkt)
			}
		}
	})

	switch w.codec {
	case types.MimeTypeOpus:
		w.translator = NewNullTranslator()
		w.validSamples = 0
		w.forceSendPLI = nil

	case types.MimeTypeH264:
		w.translator = NewNullTranslator()
		w.forceSendPLI = func() {
			root.logger.Debugw(fmt.Sprintf("H264 sendPLI %s", w.pub.SID()))
			rp.WritePLI(track.SSRC())
			w.lastPLI = time.Now()
		}
		w.validSamples = 0

	case types.MimeTypeVP8:
		w.translator = NewVP8Translator(w.logger)
		w.forceSendPLI = func() {
			root.logger.Debugw(fmt.Sprintf("VP8 sendPLI %s", w.pub.SID()))
			rp.WritePLI(track.SSRC())
			w.lastPLI = time.Now()
		}
		w.validSamples = 0

	case types.MimeTypeVP9:
		w.translator = NewNullTranslator()
		w.forceSendPLI = func() {
			root.logger.Debugw(fmt.Sprintf("VP9 sendPLI %s", w.pub.SID()))
			rp.WritePLI(track.SSRC())
			w.lastPLI = time.Now()
		}
		w.validSamples = 0

	default:
		return fmt.Errorf("%s is not yet supported", w.codec)
	}

	if debug {
		f, err := os.Create(logPath + "/" + w.pub.SID() + ".pts.log")
		if err != nil {
			return err
		}
		_, _ = f.WriteString("time: pts,sn,ts\n")
		w.logFile = f
	} else {
		w.logFile = nil
	}

	if w.kind == "audio" {
		if subs.audioWriter != nil {
			return fmt.Errorf("audio writer already created for track %s", pub.SID())
		}
		if subs.audioTrack == nil {
			subs.audioTrack = subs.makeTrack(pub)
		}
		subs.audioWriter = w
		root.logger.Debugw(fmt.Sprintf("createWriter: created audio writer for track %s", pub.SID()))
		for _, subscriber := range subs.subscribers {
			// FIXME get and log error condition
			subscriber.Lock()
			subscriber.addAudioAppSrcBin(w)
			subscriber.audioReady = true
			subscriber.Unlock()
		}
	} else if w.kind == "video" {
		if subs.videoWriter != nil {
			return fmt.Errorf("video writer already created for track %s", pub.SID())
		}
		if subs.videoTrack == nil {
			subs.videoTrack = subs.makeTrack(pub)
		}
		subs.videoWriter = w
		root.logger.Debugw(fmt.Sprintf("createWriter: created video writer for track %s", pub.SID()))
		for _, subscriber := range subs.subscribers {
			// FIXME get and log error condition
			subscriber.addVideoAppSrcBin(w)
			subscriber.Lock()
			subscriber.videoReady = true
			subscriber.Unlock()
		}
	} else {
		return fmt.Errorf("invalid track kind %s", w.kind)
	}

	w.state = statePlaying

	go w.run()

	go w.RetransmissionsTask()

	return nil
}

func (lk *ov3Subscription) subscribe(track *lksdk.RemoteTrackPublication) error {
	trackSid := track.SID()

	// FIXME: this seems not safe, but the process of subscribing/unsubscribing is asynchronous and
	// sometimes we don't observe the callback, so waiting a short time plus not checking local condition of subscription seems safe for now
	// But somethng more elaborated should eb implemented
	/*if pub.IsSubscribed() {
		return nil
	}*/

	root.logger.Debugw(fmt.Sprintf("subscribe: subscribing to track %s", trackSid))

	return track.SetSubscribed(true)
}

func (lk *ov3Subscription) unsubscribe(track *lksdk.RemoteTrackPublication) error {
	if !track.IsSubscribed() {
		return nil
	}

	root.logger.Debugw(fmt.Sprintf("unsubscribe: unsubscribing to track %s", track.SID()))

	err := track.SetSubscribed(false)

	return err
}

func (subs *ov3Subscription) addSubscriber(audioBin *gst.Bin, videoBin *gst.Bin) *ov3Subscriber {
	subs.Lock()
	subscriber := ov3Subscriber{}
	subscriber.subscription = subs
	subscriber.audioBin = audioBin
	subscriber.videoBin = videoBin
	subscriber.audioRtpSource = nil
	subscriber.videoRtpSource = nil
	subscriber.id = guuid.New().String()
	subscriber.audioReady = false
	subscriber.videoReady = false
	subscriber.videoDropping = false

	subs.subscribers = append(subs.subscribers, &subscriber)

	subs.Unlock()

	return &subscriber
}

func (subs *ov3Subscription) buildSubscriber(subscriber *ov3Subscriber) {
	subscriber.Lock()
	defer subscriber.Unlock()
	if subs.audioWriter != nil {
		if !subscriber.audioReady {
			subscriber.addAudioAppSrcBin(subs.audioWriter)
			subscriber.audioReady = true
		}
	}
	if subs.videoWriter != nil {
		if !subscriber.videoReady {
			subscriber.addVideoAppSrcBin(subs.videoWriter)
			subscriber.videoReady = true
		}
	}
}

func removeElement(array []*ov3Subscriber, indexes []int) []*ov3Subscriber {
	var result []*ov3Subscriber

	result = array
	for _, i := range indexes {
		result = append(result[:i], result[i+1:]...)
	}
	return result
}

// Must be called with room lock held
func (subs *ov3Subscription) removeSubscriber(id string) {
	var indexes []int

	indexes = make([]int, 0)
	subs.Lock()
	for i, subscriber := range subs.subscribers {
		if subscriber.id == id {
			aux := make([]int, 0)
			aux = append(aux, i)
			indexes = append(aux, indexes...)
			delete(root.subscribers, id)
		}
	}
	subs.subscribers = removeElement(subs.subscribers, indexes)

	root.logger.Debugw(fmt.Sprintf("removeSubscriber: removed subscriber %s, still %d subscribers on subscription %s", id, len(subs.subscribers), subs.egressId))
	if len(subs.subscribers) == 0 {
		subs.Unlock()
		subs.unsubscribeFromParticipant()
		subs.room.removeSubscription(subs.participant, subs.isScreenShare)
	} else {
		subs.Unlock()
	}
}

func (lk *ov3Subscription) doSubscribe(track *lkTrack) {
	if track.track == nil {
		root.logger.Infow(fmt.Sprintf("doSubscribe: no track to subscribe on %s", track.trackId))
		return
	}
	err := lk.subscribe(track.track)
	if err != nil {
		root.logger.Infow(fmt.Sprintf("doSubscribe: could not subscribe to track %s", track.trackId))
	} else {
		root.addSubscribedTrack(track.trackId, lk)
	}
}

func (lk *ov3Subscription) subscribeToParticipant() {
	// We have already looked for the subscriptions to be made, so no decission is needed now at that respect.
	audioTrack := lk.audioTrack
	if audioTrack != nil {
		lk.doSubscribe(audioTrack)
	}
	videoTrack := lk.videoTrack
	if videoTrack != nil {
		lk.doSubscribe(videoTrack)
	}
}

func (lk *ov3Subscription) unsubscribeFromParticipant() {
	// We unsubscribe for subscribed tracks (if any)

	root.logger.Debugw(fmt.Sprintf("unsubscribeFromParticipant: removing subscription to %s", lk.egressId))
	lk.Lock()
	audioTrack := lk.audioTrack
	if audioTrack != nil {
		writer := lk.audioWriter
		if writer != nil {
			writer.endStream.Break()
			lk.audioWriter = nil
		}
		root.removeSubscribedTrack(audioTrack.trackId)
		audioTrack.subscribed = false
		lk.audioTrack = nil
	}
	lk.Unlock()
	if audioTrack != nil {
		lk.unsubscribe(audioTrack.track)
	}

	lk.Lock()
	videoTrack := lk.videoTrack
	if videoTrack != nil {
		writer := lk.videoWriter
		if writer != nil {
			writer.endStream.Break()
			lk.videoWriter = nil
		}
		root.removeSubscribedTrack(videoTrack.trackId)
		videoTrack.subscribed = false
		lk.videoTrack = nil
	}
	lk.Unlock()
	if videoTrack != nil {
		lk.unsubscribe(videoTrack.track)
	}
	i := 5
	stillSubscribed := false
	if audioTrack != nil {
		stillSubscribed = stillSubscribed || audioTrack.subscribed
	}
	if videoTrack != nil {
		stillSubscribed = stillSubscribed || videoTrack.subscribed
	}

	for (i > 0) && stillSubscribed {
		root.logger.Debugw(fmt.Sprintf("unsubscribeFromParticipant: still trying to unsubscribe from  %s", lk.egressId))
		time.Sleep(200 * time.Millisecond)
		i--
		stillSubscribed = false
		if audioTrack != nil {
			stillSubscribed = stillSubscribed || audioTrack.subscribed
		}
		if videoTrack != nil {
			stillSubscribed = stillSubscribed || videoTrack.subscribed
		}
	}
}

func (subs *ov3Subscription) makeTrack(pub *lksdk.RemoteTrackPublication) *lkTrack {
	track := lkTrack{}
	trackSid := pub.SID()
	trackSource := pub.Source()

	track.subscription = subs
	track.trackId = trackSid
	track.trackType = pub.Kind()
	track.trackSource = trackSource
	track.track = pub
	track.subscribed = false

	return &track
}

func (lk *ov3Subscription) checkTracksToSubscribe(p *lksdk.RemoteParticipant) {
	if p == nil {
		root.logger.Infow("checkTracksToSubscribe: no remote participant to check")
		return
	}

	for _, track := range p.TrackPublications() {
		if pub, ok := track.(*lksdk.RemoteTrackPublication); ok {
			if pub != nil {
				source := pub.Source()
				isScreenShare := (source == livekit.TrackSource_SCREEN_SHARE) || (source == livekit.TrackSource_SCREEN_SHARE_AUDIO)
				isNotScreenShare := (source == livekit.TrackSource_CAMERA) || (source == livekit.TrackSource_MICROPHONE)
				if (lk.isScreenShare && isScreenShare) || (!lk.isScreenShare && isNotScreenShare) {
					root.logger.Debugw(fmt.Sprintf("checkTracksToSubscribe: subscribing to track %s", track.SID()))

					tr := lk.makeTrack(pub)
					if pub.Kind() == lksdk.TrackKindVideo {
						if lk.videoTrack != nil {
							if lk.videoTrack.trackId != tr.trackId {
								lk.videoTrack = tr
							}
						} else {
							lk.videoTrack = tr
						}
					} else if pub.Kind() == lksdk.TrackKindAudio {
						if lk.audioTrack != nil {
							if lk.audioTrack.trackId != tr.trackId {
								lk.audioTrack = tr
							}
						} else {
							lk.audioTrack = tr
						}
					}
				}
			}
		}
	}
}

func (lk *ov3Subscription) updateSubscription(oldTrack *lkTrack, newTrack *lkTrack) {
	if oldTrack != nil {
		if oldTrack.track != nil {
			lk.unsubscribe(oldTrack.track)
		}
	}
	if newTrack != nil {
		if newTrack.track != nil {
			if err := lk.subscribe(newTrack.track); err != nil {
				root.logger.Infow(fmt.Sprintf("updateSubscription: failed to subscribe to main video track %s", newTrack.track.SID()))
			} else {
				root.addSubscribedTrack(newTrack.track.SID(), lk)
			}
		}
	}
}

func (lk *ov3Subscription) makeSubscription() {
	room := lk.room

	for _, p := range room.roomSvc.GetRemoteParticipants() {
		if p.Identity() == lk.participant {
			root.logger.Debugw(fmt.Sprintf("makeSubscription: checking existent tracks to subscribe for participant %s", lk.participant))
			lk.checkTracksToSubscribe(p)
		}
	}
}

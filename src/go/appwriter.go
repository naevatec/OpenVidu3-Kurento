package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/frostbyte73/core"
	"github.com/livekit/egress/pkg/types"
	"github.com/livekit/protocol/logger"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

type AppWriter struct {
	logger       logger.Logger
	logFile      *os.File
	pub          *lksdk.RemoteTrackPublication
	track        *webrtc.TrackRemote
	kind         lksdk.TrackKind
	codec        types.MimeType
	subscription *ov3Subscription
	startTime    time.Time

	PayloadType webrtc.PayloadType
	ClockRate   uint32

	translator   Translator
	forceSendPLI func()

	retransmit chan uint16
	rtx        bool

	nullSamples  int
	validSamples int

	// state
	state     state
	ticker    *time.Ticker
	muted     atomic.Bool
	draining  core.Fuse
	endStream core.Fuse
	finished  core.Fuse

	// rate limiter for PLI
	lastPLI time.Time

	//Checker for gaps in the stream
	gapLock  sync.RWMutex
	dropping bool
}

type GapStatus int

const (
	NotInGap GapStatus = iota
	InGap
	EndGap
)

func (w *AppWriter) AccumulateNumSeqs(blocking bool) ([]uint16, bool) {
	var seqnums []uint16

	seqnums = make([]uint16, 20)
	i := 0
	// Block till one message appear
	if blocking {
		i++
		seqnum, ok := <-w.retransmit
		if !ok {
			return nil, false
		}
		seqnums = append(seqnums, seqnum)
	}

	// Then accumulate non blocking
	for i < 20 {
		i++
		select {
		case seqnum, ok := <-w.retransmit:
			if !ok {
				return nil, false
			}
			seqnums = append(seqnums, seqnum)
		default:
			return seqnums, true
		}

	}
	return seqnums, true
}

func (w *AppWriter) SendNack(seqs []uint16) {
	var packets []rtcp.Packet

	packet := &rtcp.TransportLayerNack{
		MediaSSRC: uint32(w.pub.TrackRemote().SSRC()),
		Nacks:     rtcp.NackPairsFromSequenceNumbers(seqs),
	}
	packets = append(packets, packet)
	_, err := w.pub.Receiver().Transport().WriteRTCP(packets)
	if err != nil {
		root.logger.Warnw(fmt.Sprintf("Cannot send RTCP NACK for retrasmission of packets in track %s", w.pub.SID()), err)
	}
}

func (w *AppWriter) RetransmissionsTask() {
	var seqnums []uint16

	for !w.endStream.IsBroken() {
		seq, ok := w.AccumulateNumSeqs(true)
		if !ok {
			return
		}
		seqnums = append(seqnums, seq...)
		// We just wait to accumulate burst
		time.Sleep(2 * time.Millisecond)
		seq, ok = w.AccumulateNumSeqs(false)
		if !ok {
			return
		}
		seqnums = append(seqnums, seq...)

		w.SendNack(seqnums)
		seqnums = []uint16{}
	}
}

func (w *AppWriter) retransmitPacket(seqnum uint) {
	root.logger.Debugw(fmt.Sprintf("retransmitPacket: Need retrasmission of packet %d in track %s", seqnum, w.pub.SID()))

	if w.rtx {
		select {
		case w.retransmit <- uint16(seqnum):
			return
		default:
			root.logger.Debugw(fmt.Sprintf("retransmitPacket: Cannot request retrasmission of packet %d in track %s", seqnum, w.pub.SID()))
		}
	}
}

func (w *AppWriter) EnterInGap() {
	w.gapLock.Lock()
	if w.dropping {
		w.gapLock.Unlock()
		return
	}
	w.dropping = true
	w.gapLock.Unlock()
	if w.forceSendPLI != nil {
		w.forceSendPLI()
	}
}

func (w *AppWriter) GapDetected(buffer []byte) GapStatus {
	packet := &rtp.Packet{}
	packet.Unmarshal(buffer)
	if isKeyFrameStart(packet, w.codec) {
		return NotInGap
	}

	root.logger.Debugw(fmt.Sprintf("GapDetected: Gap found on stream, requesting PLI and start dropping until keyframe in track %s", w.pub.SID()))
	w.EnterInGap()

	return InGap
}

func (w *AppWriter) CheckStillInGap(buffer []byte) GapStatus {
	w.gapLock.Lock()
	if w.dropping {
		// Still waiting for a keyframe
		packet := &rtp.Packet{}
		packet.Unmarshal(buffer)
		if isKeyFrameStart(packet, w.codec) {
			root.logger.Debugw(fmt.Sprintf("CheckStillInGap: Key frame found, stop dropping packets in track %s", w.pub.SID()))
			w.dropping = false
			w.gapLock.Unlock()
			return EndGap
		} else {
			w.gapLock.Unlock()
			w.sendPLI()
			return InGap
		}
	} else {
		w.gapLock.Unlock()
		return NotInGap
	}
}

func (w *AppWriter) PushRTCPPacket(pkt rtcp.Packet) {
	var appSrc *app.Source

	p, err := pkt.Marshal()
	if err != nil {
		w.logger.Errorw("could not marshal RTCP packet", err)
		root.logger.Debugw(fmt.Sprintf("ProcessRTCP: could not marshal RTCP packet %s", w.pub.SID()))
		return
	}

	b := gst.NewBufferFromBytes(p)
	w.subscription.Lock()
	subscribers := w.subscription.subscribers
	w.subscription.Unlock()
	if (subscribers == nil) || (len(subscribers) == 0) {
		root.logger.Debugw(fmt.Sprintf("ProcessRTCP: RTCP packet received for track %s but no subscribers", w.pub.SID()))
		return
	}
	//root.logger.Debugw(fmt.Sprintf("ProcessRTCP: RTCP packet received for track %s pushing downstream", w.pub.SID()))
	for _, subscriber := range w.subscription.subscribers {

		if subscriber != nil {
			subscriber.Lock()
			if w.kind == "audio" {
				if subscriber.audioReady {
					appSrc = subscriber.audioRtcpSource
				} else {
					appSrc = nil
				}
			} else if w.kind == "video" {
				if subscriber.videoReady {
					appSrc = subscriber.videoRtcpSource
				} else {
					appSrc = nil
				}
			} else {
				return
			}
			subscriber.Unlock()
			if appSrc != nil {
				flow := appSrc.PushBuffer(b)
				if flow != gst.FlowOK {
					root.logger.Infow(fmt.Sprintf("ProcessRTCP: unexpected flow return %s", w.pub.SID()))
				}
			} else {
				root.logger.Debugw(fmt.Sprintf("pushPacket: no appsource available %s", w.pub.SID()))
			}
		} else {
			root.logger.Infow(fmt.Sprintf("pushPacket: no subscriber for this track %s", w.pub.SID()))
		}
	}
}

func (w *AppWriter) TrackID() string {
	return w.track.ID()
}

func (w *AppWriter) SetTrackMuted(muted bool) {
	w.muted.Store(muted)
	if muted {
		w.logger.Debugw("track muted", "timestamp", time.Since(w.startTime).Seconds())
		root.logger.Debugw(fmt.Sprintf("SetTrackMuted: track muted %s", w.pub.SID()))
	} else {
		w.logger.Debugw("track unmuted", "timestamp", time.Since(w.startTime).Seconds())
		root.logger.Debugw(fmt.Sprintf("SetTrackMuted: track unmuted %s", w.pub.SID()))
		w.EnterInGap()
	}
}

// Send PLI with a rate limiter applied
func (w *AppWriter) sendPLI() {
	// rate limit PLIs to at most one each 2 seconds
	now := time.Now()
	if now.Sub(w.lastPLI) > 1*time.Second {
		if w.forceSendPLI != nil {
			w.forceSendPLI()
		}
	}
}

// Drain blocks until finished
func (w *AppWriter) Drain(force bool) {
	w.draining.Once(func() {
		w.logger.Debugw("draining")
		root.logger.Debugw(fmt.Sprintf("Drain: %s", w.pub.SID()))

		if force || w.muted.Load() {
			w.endStream.Break()
		} else {
			// wait until drainTimeout before force popping
			time.AfterFunc(drainTimeout, w.endStream.Break)
		}
	})
}

func (w *AppWriter) run() {
	root.logger.Debugw(fmt.Sprintf("run: starting writer for track %s", w.pub.SID()))
	defer func() {
		if err := recover(); err != nil {
			root.logger.Debugw(fmt.Sprintln("panic occurred ", err))
		}
		root.logger.Debugw(fmt.Sprintf("run: writer for track %s terminated", w.pub.SID()))
	}()
	w.startTime = time.Now()

	// We wait till first keyframe
	w.EnterInGap()
	for !w.endStream.IsBroken() {
		switch w.state {
		case stateUnmuting, statePlaying:
			w.handlePlaying()
		case stateMuted:
			w.handleMuted()
		default:
			root.logger.Debugw(fmt.Sprintf("run: Unknown state %d in track %s", w.state, w.pub.SID()))
		}
	}

	// Finishing retrasnmission task
	w.rtx = false
	//close(w.retransmit)

	// clean up
	w.draining.Break()

	w.finished.Break()

	if w.logFile != nil {
		_ = w.logFile.Close()
	}

	root.logger.Debugw(fmt.Sprintf("run: terminating writer for track %s", w.pub.SID()))

}

func (w *AppWriter) handlePlaying() {
	//root.logger.Printf("handlePlaying: track %s", w.pub.SID())
	// read next packet
	_ = w.track.SetReadDeadline(time.Now().Add(time.Millisecond * 500))
	pkt, _, err := w.track.ReadRTP()
	if err != nil {
		w.handleReadError(err)
		w.sendPLI()
	}

	//root.logger.Printf("Track packet %s,%d,%d\n", w.pub.SID(), pkt.SequenceNumber, pkt.Timestamp)

	// push packet to jitter buffer
	if pkt == nil {
		root.logger.Debugw("handlePlaying: received nil packet")
		return
	}

	// push completed packets to appsrc
	if err = w.pushSamples(pkt); err != nil {
		root.logger.Debugw(fmt.Sprintf("handlePlaying: push samples error %s %s", w.pub.SID(), err.Error()))
		w.draining.Once(w.endStream.Break)
	}
}

func (w *AppWriter) handleMuted() {
	//root.logger.Debugw(fmt.Sprintf("handleMuted: track %s", w.pub.SID()))
	switch {
	case w.draining.IsBroken():
		w.ticker.Stop()
		w.endStream.Break()
		root.logger.Debugw(fmt.Sprintf("handleMuted: %s terminating writer", w.pub.SID()))

	case !w.muted.Load():
		w.ticker.Stop()
		w.state = stateUnmuting
		root.logger.Debugw(fmt.Sprintf("handleMuted: %s un muting channel", w.pub.SID()))

	default:
		//root.logger.Debugw(fmt.Sprintf("handleMuted: %s waiting channel to complete", w.pub.SID()))
		<-w.ticker.C
	}
}

func (w *AppWriter) handleReadError(err error) {
	root.logger.Debugw(fmt.Sprintf("handleReadError: track %s, error %s", w.pub.SID(), err.Error()))
	if w.draining.IsBroken() {
		w.endStream.Break()
		return
	}

	// continue on buffer too small error
	if err.Error() == errBufferTooSmall {
		w.logger.Warnw("read error", err)
		root.logger.Debugw(fmt.Sprintf("handleReadError: read error %s", w.pub.SID()))
		return
	}

	// check if muted
	if w.muted.Load() {
		w.ticker = time.NewTicker(500 * time.Millisecond)
		w.state = stateMuted
		return
	}

	// continue on timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return
	}

	// log non-EOF errors
	if !errors.Is(err, io.EOF) {
		w.logger.Errorw("could not read packet", err)
		root.logger.Debugw(fmt.Sprintf("handleReadError: could not read packet %s", w.pub.SID()))
	}

	// end stream
	w.endStream.Break()
}

func isH264KeyFrameStart(pkt *rtp.Packet) bool {
	if len(pkt.Payload) < 2 {
		return false
	}

	// Keyframe start in H264 is identified by:
	// - type of frame keyframe, byte 1 with 5 least sginificant bits set to value 5
	// - First packet of FU-A picture: byte 1 with most significant bit to 1
	identifier := pkt.Payload[0]
	nalHeader := pkt.Payload[1]
	nalType := nalHeader & 0x9f
	if ((identifier & 0x1f) == 0x1c) && (nalType == 0x85) {
		return true
	}
	// Either
	// - First mb in slice set to 0: byte 1 most significant bit set to 1
	// - I slice: byte 1 buts 6,5, and 4 set to 011
	if ((identifier & 0x1f) == 0x05) && ((nalHeader & 0xf0) == 0xb0) {
		return true
	}
	return false
}

func isVP8KeyFrameStart(pkt *rtp.Packet) bool {
	if len(pkt.Payload) < 5 {
		return false
	}

	// Keyframe start in vp8 identified by:
	// - payload descriptor with start of VP8 partition (byte 0 with  bit 00010000)
	// - payload header frame type keyframe, if start of partition it is byte 4  least signinfican bit to 0
	descriptor := pkt.Payload[0]
	header := pkt.Payload[4]
	if (descriptor&0x10) == 1 && (header&0x01) == 0 {
		return true
	}
	return false
}

func isKeyFrameStart(pkt *rtp.Packet, codec types.MimeType) bool {

	switch codec {
	case types.MimeTypeVP8:
		return isVP8KeyFrameStart(pkt)

	case types.MimeTypeH264:
		return isH264KeyFrameStart(pkt)
		// FIXME; add VP9
	}
	return false
}

func (w *AppWriter) pushSamples(pkt *rtp.Packet) error {
	var err error

	// No gap detection here, now it is done on rtpjitterbuffer
	if w.state == stateUnmuting {
		w.state = statePlaying
	}

	if w.logFile != nil {
		_, _ = w.logFile.WriteString(fmt.Sprintf("%s: (%d) %d,%d\n", time.Now(), pkt.SSRC, pkt.SequenceNumber, pkt.Timestamp))
	}

	if err = w.pushPacket(pkt); err != nil {
		root.logger.Infow(fmt.Sprintf("pushSamples: ERROR pushing packets, %s", w.pub.SID()))
		return err
	}

	return nil
}

func (w *AppWriter) pushPacket(pkt *rtp.Packet) error {
	var appSrc *app.Source

	p, err := pkt.Marshal()
	if err != nil {
		w.logger.Errorw("could not marshal packet", err)
		root.logger.Debugw(fmt.Sprintf("pushPacket: could not marshal packet %s", w.pub.SID()))
		return err
	}

	b := gst.NewBufferFromBytes(p)
	w.subscription.Lock()
	subscribers := w.subscription.subscribers
	w.subscription.Unlock()
	if (subscribers == nil) || (len(subscribers) == 0) {
		root.logger.Debugw(fmt.Sprintf("pushPacket: packet received for track %s but no subscribers", w.pub.SID()))
		return nil
	}
	for _, subscriber := range w.subscription.subscribers {

		if subscriber != nil {
			subscriber.Lock()
			if w.kind == "audio" {
				if subscriber.audioReady {
					appSrc = subscriber.audioRtpSource
				} else {
					appSrc = nil
				}
			} else if w.kind == "video" {
				if subscriber.videoReady {
					appSrc = subscriber.videoRtpSource
				} else {
					appSrc = nil
				}
			} else {
				return errors.New("ERROR: no valid packet type")
			}
			subscriber.Unlock()
			if appSrc != nil {
				flow := appSrc.PushBuffer(b)
				//root.logger.Printf("pushPacket: pushed packet from track %s", w.pub.SID())
				if flow != gst.FlowOK {
					w.logger.Infow("unexpected flow return", "flow", flow)
					root.logger.Infow(fmt.Sprintf("pushPacket: unexpected flow return %s", w.pub.SID()))
				}
			} else {
				w.logger.Errorw("no appsrouce available", err)
				root.logger.Infow(fmt.Sprintf("pushPacket: no appsource available %s", w.pub.SID()))
			}
		} else {
			root.logger.Infow(fmt.Sprintf("pushPacket: no subscriber for this track %s", w.pub.SID()))
		}
	}

	return nil
}

func (w *AppWriter) verifyStreamGap(buffer *gst.Buffer) gst.PadProbeReturn {
	switch w.CheckStillInGap(buffer.Bytes()) {
	case InGap:
		return gst.PadProbeDrop
	case EndGap:
		return gst.PadProbeOK

	case NotInGap:
		flags := buffer.GetFlags()
		isGap := uint(flags) & uint(gst.BufferFlagDiscont)
		if isGap != 0 {
			if w.GapDetected(buffer.Bytes()) == InGap {
				// Discontinuity found, unless this is a keyframe we must drop until a keyframe is found
				return gst.PadProbeDrop
			}
		}
	}

	return gst.PadProbeOK
}

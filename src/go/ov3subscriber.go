package main

// #include <gst/app/gstappsrc.h>
// #include <gst/gstpad.h>
// #include <gst/gstbin.h>
// #include <gst/gstelement.h>
import "C"

// gst-go reelase must be coupled to compiling OS, for ubuntu 2.20, that is gstreamer 1.16, gst-go v0.2.16 seems to be needed
import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/pion/rtp"

	"github.com/livekit/egress/pkg/types"
	"github.com/tinyzimmer/go-glib/glib"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"

	"github.com/livekit/livekit-server/pkg/sfu/buffer"
	"github.com/livekit/livekit-server/pkg/sfu/codecmunger"
	"github.com/livekit/protocol/logger"
)

type state int

const (
	statePlaying state = iota
	stateMuted
	stateUnmuting
)

const (
	latency           = time.Millisecond * 1000
	drainTimeout      = time.Second * 4
	errBufferTooSmall = "buffer too small"
)

type ov3Subscriber struct {
	sync.RWMutex
	subscription    *ov3Subscription
	audioRtpSource  *app.Source
	audioRtcpSource *app.Source
	videoRtpSource  *app.Source
	videoRtcpSource *app.Source
	audioBin        *gst.Bin
	videoBin        *gst.Bin
	id              string
	audioReady      bool
	videoReady      bool
	videoDropping   bool

	rtpEventProbe uint64
	jbEventProbe  uint64
	jbBufferProbe uint64
}

// ******************** Translator

// FIXME; Translator seems not be needed any longer
type Translator interface {
	Translate(*rtp.Packet)
}

// VP8

type VP8Translator struct {
	logger logger.Logger

	firstPktPushed bool
	lastSN         uint16
	vp8Munger      *codecmunger.VP8
}

func NewVP8Translator(logger logger.Logger) *VP8Translator {
	return &VP8Translator{
		logger:    logger,
		vp8Munger: codecmunger.NewVP8(logger),
	}
}

func (t *VP8Translator) Translate(pkt *rtp.Packet) {
	defer func() {
		t.lastSN = pkt.SequenceNumber
	}()

	if len(pkt.Payload) == 0 {
		return
	}

	vp8Packet := buffer.VP8{}
	if err := vp8Packet.Unmarshal(pkt.Payload); err != nil {
		root.logger.Infow("Translate: (VP8) could not unmarshal VP8 packet")
		return
	}

	ep := &buffer.ExtPacket{
		Packet:   pkt,
		Arrival:  time.Now(),
		Payload:  vp8Packet,
		KeyFrame: vp8Packet.IsKeyFrame,
		VideoLayer: buffer.VideoLayer{
			Spatial:  -1,
			Temporal: int32(vp8Packet.TID),
		},
	}

	if !t.firstPktPushed {
		t.firstPktPushed = true
		t.vp8Munger.SetLast(ep)
	} else {
		tpVP8, err := t.vp8Munger.UpdateAndGet(ep, false, pkt.SequenceNumber != t.lastSN+1, ep.Temporal)
		if err != nil {
			root.logger.Infow("Translate: (VP8) could not update VP8 packet")
			return
		}
		pkt.Payload = translateVP8Packet(ep.Packet, &vp8Packet, tpVP8, &pkt.Payload)
	}
}

func translateVP8Packet(pkt *rtp.Packet, incomingVP8 *buffer.VP8, translatedVP8 []byte, outbuf *[]byte) []byte {
	buf := (*outbuf)[:len(pkt.Payload)+len(translatedVP8)-incomingVP8.HeaderSize]
	srcPayload := pkt.Payload[incomingVP8.HeaderSize:]
	dstPayload := buf[len(translatedVP8):]
	copy(dstPayload, srcPayload)

	copy(buf[:len(translatedVP8)], translatedVP8)
	return buf
}

// Null

type NullTranslator struct{}

func NewNullTranslator() Translator {
	return &NullTranslator{}
}

func (t *NullTranslator) Translate(_ *rtp.Packet) {}

/*func WrapPad(pad *gst.GhostPad) *gst.Pad {
	return gst.FromGstPadUnsafeFull(unsafe.Pointer(pad))
	//return &gst.Pad{&gst.Object{InitiallyUnowned: &glib.InitiallyUnowned{Object: &glib.Object{GObject: glib.ToGObject(unsafe.Pointer(pad))}}}}
}*/

func exposeSrcInBin(element *gst.Element, bin *gst.Bin) error {
	pad := element.GetStaticPad("src")
	if pad == nil {
		return errors.New("cannot get src pad from element")
	}

	ghostPad := gst.NewGhostPad("src", pad)
	bin.AddPad(ghostPad.ProxyPad.Pad)
	return nil
}

func SetArg(o *gst.Object, name, value string) {
	cName := C.CString(name)
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cName))
	defer C.free(unsafe.Pointer(cValue))

	C.gst_util_set_object_arg(
		(*C.GObject)(o.Unsafe()),
		(*C.gchar)(unsafe.Pointer(cName)),
		(*C.gchar)(unsafe.Pointer(cValue)),
	)
}

func createAppSrc(caps string, name string) (*app.Source, error) {
	src, err := gst.NewElementWithName("appsrc", name)
	if err != nil {
		return nil, err
	}

	appSource := app.SrcFromElement(src)
	format, _ := appSource.GetPropertyType("format")
	formatVal, _ := glib.ValueInit(format)
	C.g_value_set_enum((*C.GValue)(formatVal.Unsafe()), 3) /*"time"*/
	appSource.SetPropertyValue("format", formatVal)
	appSource.SetProperty("is-live", true)
	maxBytesVal, _ := glib.ValueInit(glib.TYPE_INT64)
	C.g_value_set_int64((*C.GValue)(maxBytesVal.Unsafe()), C.long(2000000))
	appSource.SetPropertyValue(("max-bytes"), maxBytesVal)
	appSource.SetProperty("block", true)
	appSource.SetProperty("do-timestamp", true)
	appSource.SetProperty("emit-signals", false)
	appSource.SetProperty("caps", gst.NewCapsFromString(caps))
	latVal, _ := glib.ValueInit(glib.TYPE_INT64)
	C.g_value_set_int64((*C.GValue)(latVal.Unsafe()), C.long(0))
	appSource.SetPropertyValue("min-latency", latVal)

	return appSource, nil
}

func createJjitterBuffer(audio bool) (*gst.Element, error) {
	var latency int
	var jbName string

	if audio {
		latency = 200
		jbName = "jitterbuffer_audio"
	} else {
		latency = 200
		jbName = "jitterbuffer_video"
	}

	jb, err := gst.NewElementWithName("rtpjitterbuffer", jbName)
	if err != nil {
		return nil, err
	}

	if err := jb.SetProperty("do-lost", true); err != nil {
		return nil, err
	}
	if err := jb.SetProperty("do-retransmission", true); err != nil {
		return nil, err
	}
	//if err := jb.SetProperty("drop-on-latency", true){
	//	return nil, err
	//}
	if err := jb.SetProperty("faststart-min-packets", uint(1)); err != nil {
		return nil, err
	}

	// FIXME: Too complicated syntax
	modePropType, _ := jb.GetPropertyType("mode")
	modeVal, _ := glib.ValueInit(modePropType)
	C.g_value_set_enum((*C.GValue)(modeVal.Unsafe()), 4) /*"synced"*/
	if err := jb.SetPropertyValue("mode", modeVal); err != nil {
		return nil, err
	}
	if err := jb.SetProperty("rtx-next-seqnum", false); err != nil {
		return nil, err
	}
	if err := jb.SetProperty("latency", uint(latency)); err != nil {
		return nil, err
	}

	return jb, nil
}

func (lk *ov3Subscriber) prepareTrackGstBin(audio bool, bin *gst.Bin, trackId string, trackMediaCaps string, depayFactory string) (*app.Source, *app.Source, error) {
	var rtcpSource *app.Source
	var rtpSource *app.Source
	var jitterBuffer *gst.Element
	var depayloader *gst.Element
	var err error
	var desc string

	if audio {
		desc = "audio"
	} else {
		desc = "video"
	}

	root.logger.Debugw(fmt.Sprintf("prepareTrackGstBin: Building Gst Bin for %s track %s with media caps %s", desc, trackId, trackMediaCaps))
	name := fmt.Sprintf("app_rtcp_%s", trackId)
	rtcpSource, err = createAppSrc("application/x-rtcp", name)
	if err != nil {
		return nil, nil, err
	}
	jitterBuffer, err = createJjitterBuffer(audio)
	if err != nil {
		return nil, nil, err
	}
	name = fmt.Sprintf("app_rtp_%s", trackId)
	rtpSource, err = createAppSrc(trackMediaCaps, name)
	if err != nil {
		return nil, nil, err
	}

	depayloader, err = gst.NewElementWithName(depayFactory, "depayloader")
	if err != nil {
		return nil, nil, err
	}

	bin.AddMany(jitterBuffer, rtcpSource.Element, rtpSource.Element, depayloader)
	rtpSource.Link(jitterBuffer)
	rtcpSinkPad := jitterBuffer.GetRequestPad("sink_rtcp")
	//rtcpSinkPad := gst.FromGstPadUnsafeFull(unsafe.Pointer(C.gst_element_get_request_pad_simple((*C.GstElement)(unsafe.Pointer(jitterBuffer.Instance())), C.CString("sink_rtcp"))))
	rtcpSrcPad := rtcpSource.GetStaticPad("src")
	rtcpSrcPad.Link(rtcpSinkPad)
	jitterBuffer.Link(depayloader)
	jitterBuffer.SyncStateWithParent()

	rtcpSource.Element.SyncStateWithParent()
	rtpSource.Element.SyncStateWithParent()
	depayloader.SyncStateWithParent()

	if err = exposeSrcInBin(depayloader, bin); err != nil {
		return nil, nil, err
	}

	root.logger.Debugw("prepareTrackGstBin: Gst Bin built")
	return rtpSource, rtcpSource, nil
}

// This must be called with subscriber lock held
func (b *ov3Subscriber) addAudioAppSrcBin(w *AppWriter) error {
	var trackMediaCaps string
	var depayFactory string

	if b.audioRtpSource != nil {
		return nil
	}

	appSrcBin := b.audioBin

	switch w.codec {
	case types.MimeTypeOpus:
		trackMediaCaps = fmt.Sprintf("application/x-rtp,media=audio,payload=%d,encoding-name=OPUS,clock-rate=%d",
			w.PayloadType, w.ClockRate)
		depayFactory = "rtpopusdepay"
	default:
		return fmt.Errorf("%s is not yet supported", w.codec)
	}

	// FIXME:: This should be done on ov3Subscription to use one single ingress pipeline and then share the output to all subscribers
	rtpSource, rtcpSource, err := b.prepareTrackGstBin(true, appSrcBin, w.track.ID(), trackMediaCaps, depayFactory)
	if err != nil {
		return err
	}
	b.audioRtpSource = rtpSource
	b.audioRtcpSource = rtcpSource

	root.logger.Debugw(fmt.Sprintf("addAudioAppSrcBin: created audio bin for track %s and subcriber %s", w.pub.SID(), b.id))
	return nil
}

func (lk *ov3Subscriber) DoSynchronize(buffer *gst.Buffer) {
	// FIXME: On upgrading to GStreamer 1.22 we should set the property of rtpjitterbuffer 'add-reference-timestamp-meta'
	// That will allow to set on the buffer a GstReferenceTimestampMeta  with timing information got btoh from RTCP SR or inband
	// NTP-64 header (that is used in webrtc and specifically in livekit) to provide better synchronization
	// When that meta is available, we should get the meta, extract timing info (https://gstreamer.freedesktop.org/documentation/gstreamer/gstbuffer.html#GstReferenceTimestampMeta)
	// and set the PTS of buffers according to that

	//bufferPts := buffer.PresentationTimestamp()

}

// This must be called with subscriber lock held
func (lk *ov3Subscriber) addVideoAppSrcBin(w *AppWriter) error {
	var trackMediaCaps string
	var depayFactory string

	if lk.videoRtpSource != nil {
		return nil
	}

	appSrcBin := lk.videoBin

	switch w.codec {
	case types.MimeTypeH264:
		trackMediaCaps = fmt.Sprintf("application/x-rtp,media=video,payload=%d,encoding-name=H264,clock-rate=%d",
			w.PayloadType, w.ClockRate)
		depayFactory = "rtph264depay"

	case types.MimeTypeVP8:
		trackMediaCaps = fmt.Sprintf("application/x-rtp,media=video,payload=%d,encoding-name=VP8,clock-rate=%d",
			w.PayloadType, w.ClockRate)
		depayFactory = "rtpvp8depay"

	case types.MimeTypeVP9:
		trackMediaCaps = fmt.Sprintf("application/x-rtp,media=video,payload=%d,encoding-name=VP9,clock-rate=%d",
			w.PayloadType, w.ClockRate)
		depayFactory = "rtpvp9depay"

	default:
		return fmt.Errorf("%s is not yet supported", w.codec)
	}

	// FIXME:: This should be done on ov3Subscription to use one single ingress pipeline and then share the output to all subscribers
	rtpSource, rtcpSource, err := lk.prepareTrackGstBin(false, appSrcBin, w.track.ID(), trackMediaCaps, depayFactory)
	if err != nil {
		return err
	}
	lk.videoRtpSource = rtpSource
	lk.videoRtcpSource = rtcpSource

	srcPad := rtpSource.Element.GetStaticPad("src")
	lk.rtpEventProbe = srcPad.AddProbe(gst.PadProbeTypeEventUpstream, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		if !lk.videoReady {
			return gst.PadProbeDrop
		}

		event := info.GetEvent()

		if event == nil {
			return gst.PadProbeOK
		}

		if event.HasName("GstForceKeyUnit") {
			root.logger.Debugw(fmt.Sprintf("AppSrc Pad probe: requesting PLI on track %s", w.pub.SID()))
			w.sendPLI()

			return gst.PadProbeDrop
		} else if event.HasName("GstRTPRetransmissionRequest") {
			root.logger.Debugw(fmt.Sprintf("AppSrc Pad probe: requesting retransmission on track %s", w.pub.SID()))
			str := event.GetStructure()
			if str != nil {
				seqnum, _ := str.GetValue("seqnum")
				w.retransmitPacket(seqnum.(uint))

				return gst.PadProbeDrop
			}
		}
		return gst.PadProbeOK
	})

	jb, _ := lk.videoBin.GetElementByName("jitterbuffer_video")
	if jb != nil {
		jbSrcPad := jb.GetStaticPad("src")
		lk.jbBufferProbe = jbSrcPad.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			if !lk.videoReady {
				return gst.PadProbeDrop
			}

			buffer := info.GetBuffer()

			if buffer == nil {
				return gst.PadProbeOK
			}

			if gapResult := w.verifyStreamGap(buffer); gapResult != gst.PadProbeOK {
				return gapResult
			}

			lk.DoSynchronize(buffer)
			return gst.PadProbeOK
		})
		lk.jbEventProbe = jbSrcPad.AddProbe(gst.PadProbeTypeEventDownstream, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			if !lk.videoReady {
				return gst.PadProbeDrop
			}

			event := info.GetEvent()
			if event == nil {
				return gst.PadProbeOK
			}

			if event.HasName("GstRTPPacketLost") {
				root.logger.Debugw(fmt.Sprintf("AppSrc Pad probe: Packet lost, requesting PLI on track %s", w.pub.SID()))
				w.EnterInGap()
				return gst.PadProbeDrop
			}

			return gst.PadProbeOK
		})
	}

	root.logger.Debugw(fmt.Sprintf("addVideoAppSrcBin: created h264 video bin for track %s and subcriber %s", w.pub.SID(), lk.id))
	return nil
}

func (lk *ov3Subscriber) DestroySubscriber() {
	lk.Lock()
	defer lk.Unlock()

	rtpSource := lk.videoRtpSource
	if rtpSource != nil {
		srcPad := rtpSource.Element.GetStaticPad("src")
		if srcPad != nil {
			srcPad.RemoveProbe(lk.rtpEventProbe)
		}
	}

	bin := lk.videoBin
	if bin != nil {
		jb, _ := bin.GetElementByName("jitterbuffer_video")
		if jb != nil {
			jbSrcPad := jb.GetStaticPad("src")
			if jbSrcPad != nil {
				jbSrcPad.RemoveProbe(lk.jbBufferProbe)
				jbSrcPad.RemoveProbe(lk.jbEventProbe)
			}
		}
	}

	lk.audioRtpSource = nil
	lk.audioRtcpSource = nil
	lk.audioReady = false
	lk.videoRtpSource = nil
	lk.videoRtcpSource = nil
	lk.videoReady = false
	lk.audioBin = nil
	lk.videoBin = nil
}

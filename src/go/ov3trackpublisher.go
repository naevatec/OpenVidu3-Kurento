package main

// #include <gst/app/gstappsrc.h>
// #include <gst/gstpad.h>
// #include <gst/gstbin.h>

/*
#include <gst/video/video-event.h>

gboolean
sendForceKeyUnitEvent (GstPad *pad)
{
  GstEvent *event;
  gboolean result;

  event =
      gst_video_event_new_upstream_force_key_unit (GST_CLOCK_TIME_NONE,
      												FALSE, 0);

  result = gst_pad_push_event (pad, event);

  return result;
}

*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/frostbyte73/core"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/tinyzimmer/go-glib/glib"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

type ov3TrackPublisher struct {
	sync.RWMutex

	publisher   *ov3Publisher
	kind        lksdk.TrackKind
	track       *lksdk.LocalSampleTrack
	sid         string
	opts        *lksdk.TrackPublicationOptions
	bin         *gst.Bin
	sinkTee     *gst.Element
	element     *app.Sink
	sinkPad     *gst.Pad
	currentCaps *gst.Caps
	codec       string

	endStream core.Fuse

	inputSignalHandler   glib.SignalHandle
	sinkPadSignalHandler glib.SignalHandle
}

func UnpublishTrack(track *lksdk.LocalSampleTrack) {
	if track == nil {
		return
	}

}

func (tr *ov3TrackPublisher) HandlePLI() error {
	sinkPad := tr.sinkPad
	if sinkPad == nil {
		return errors.New("NotSent")
	}
	pad := (*C.GstPad)(unsafe.Pointer(sinkPad.Instance()))
	result := C.sendForceKeyUnitEvent(pad)

	if result != 0 {
		return nil
	} else {
		return errors.New("NotSent")
	}
}

func (tr *ov3TrackPublisher) createPublishTrack(mimeType string) (*lksdk.LocalSampleTrack, error) {
	onRTCP := func(pkt rtcp.Packet) {
		switch pkt.(type) {
		case *rtcp.PictureLossIndication:
			root.logger.Debugw(fmt.Sprintf("PLI received for publisher %s %s", tr.publisher.id, &tr.kind))
			if err := tr.HandlePLI(); err != nil {
				root.logger.Errorw(fmt.Sprintf("could not force key frame for publisher %s %s", tr.publisher.id, &tr.kind), err)
			} else {
				root.logger.Debugw(fmt.Sprintf("PLI correctly sent for publisher %s %s", tr.publisher.id, &tr.kind))
			}
		}
	}

	track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{
		MimeType: mimeType,
	},
		lksdk.WithRTCPHandler(onRTCP))

	if err != nil {
		root.logger.Errorw("could not create media track", err)
		return nil, err
	}

	track.OnBind(func() {
		root.logger.Debugw(fmt.Sprintf("%s track bound %s", tr.kind, tr.publisher.id))
	})
	track.OnUnbind(func() {
		root.logger.Debugw(fmt.Sprintf("%s track unbound %s", tr.kind, tr.publisher.id))
	})

	return track, nil
}

func (tr *ov3TrackPublisher) IsCapsCompatible(caps *gst.Caps) bool {
	structure := caps.GetStructureAt(0)

	if structure == nil {
		return false
	}

	codec := structure.Name()

	return codec == tr.codec
}

func NewAppSinkWithName(name string) (*app.Sink, error) {
	elem, err := gst.NewElementWithName("appsink", name)
	if err != nil {
		return nil, err
	}
	return app.SinkFromElement(elem), nil
}

func (trPub *ov3TrackPublisher) addParserandSink(parser *gst.Element, payloaderStr string, parsedCaps string) error {
	var appSink *app.Sink
	var payloader *gst.Element
	var capsfilter *gst.Element
	var mtu uint

	appSink, _ = NewAppSinkWithName("sink")
	trPub.bin.Add(appSink.Element)
	trPub.element = appSink
	caps := gst.NewCapsFromString("application/x-rtp")
	appSink.SetProperty("caps", caps)
	appSink.SetProperty("async", false)
	appSink.SetProperty("sync", false)
	// At most 3 seconds are queued
	maxTimeVal, _ := glib.ValueInit(glib.TYPE_INT64)
	C.g_value_set_int64((*C.GValue)(maxTimeVal.Unsafe()), C.long(3000000000))
	appSink.SetPropertyValue("max-time", maxTimeVal)
	appSink.SetDrop(true)
	payloader, _ = gst.NewElement(payloaderStr)
	trPub.bin.Add(payloader)
	capsfilter, _ = gst.NewElement("capsfilter")
	caps1 := gst.NewCapsFromString(parsedCaps)
	capsfilter.SetProperty("caps", caps1)
	trPub.bin.Add(capsfilter)
	trPub.sinkPad = payloader.GetStaticPad("sink")
	// WebRTC uses MTU 1200 as standard
	mtu = 1200
	payloader.SetProperty("mtu", mtu)
	payloader.SetProperty("config-interval", 1)
	if parser != nil {
		trPub.bin.Add(parser)
		gst.ElementLinkMany(trPub.sinkTee, parser, capsfilter, payloader, appSink.Element)
		parser.SyncStateWithParent()
	} else {
		gst.ElementLinkMany(trPub.sinkTee, capsfilter, payloader, appSink.Element)
	}
	appSink.SyncStateWithParent()
	payloader.SyncStateWithParent()
	capsfilter.SyncStateWithParent()

	return nil
}

func (trPub *ov3TrackPublisher) completeAudioOpusPipeline() error {
	var opusParser *gst.Element
	var err error

	opusParser, err = gst.NewElement("opusparse")
	if err != nil {
		return errors.New("cannot create Opus parser")
	}

	return trPub.addParserandSink(opusParser, "rtpopuspay", "audio/x-opus")
}

func (trPub *ov3TrackPublisher) completeVideoH264Pipeline() error {
	var h264Parser *gst.Element
	var err error

	h264Parser, err = gst.NewElement("h264parse")
	if err != nil {
		return errors.New("cannot create H264 parser")
	}

	return trPub.addParserandSink(h264Parser, "rtph264pay", "video/x-h264,alignment=au")
}

func (trPub *ov3TrackPublisher) completeVideoVP8Pipeline() error {
	var vp8Parser *gst.Element
	var err error

	vp8Parser, err = gst.NewElement("vp8parse")
	if err != nil {
		return errors.New("cannot create VP8 parser")
	}

	return trPub.addParserandSink(vp8Parser, "rtpvp8pay", "video/x-vp8")
}

func (trPub *ov3TrackPublisher) completeVideoVP9Pipeline() error {

	return trPub.addParserandSink(nil, "rtpvp9pay", "video/x-vp9")
}

func (trPub *ov3TrackPublisher) completePublisherPipeline(codec string) error {
	switch codec {
	case "audio/x-opus":
		return trPub.completeAudioOpusPipeline()
	case "video/x-h264":
		return trPub.completeVideoH264Pipeline()
	case "video/x-vp8":
		return trPub.completeVideoVP8Pipeline()
	case "video/x-vp9":
		return trPub.completeVideoVP9Pipeline()
	default:
		return fmt.Errorf("codec %s not implemented yet", codec)
	}
}

func (trPub *ov3TrackPublisher) createSinkElementsForPublisher(kind lksdk.TrackKind, ingressId string, screenShare bool) error {
	var queue *gst.Element
	var capsfilter *gst.Element
	var tee *gst.Element
	var fakesink *gst.Element
	var err error
	var caps *gst.Caps
	var sinkPad *gst.Pad
	var connected bool

	queue, err = gst.NewElement("queue")
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot create Sink queue element for publisher")
	}
	tee, err = gst.NewElement("tee")
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot create Sink Tee element for publisher")
	}
	fakesink, err = gst.NewElement("fakesink")
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot create Sink faskesink element for publisher")
	}
	fakesink.SetProperty("sync", false)
	fakesink.SetProperty("async", false)
	capsfilter, err = gst.NewElementWithName("capsfilter", "inputCapsfilter")
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot create capsfilter element for publisher")
	}
	if kind == lksdk.TrackKindAudio {
		caps = gst.NewCapsFromString("audio/x-opus")
	} else if kind == lksdk.TrackKindVideo {
		caps = gst.NewCapsFromString("video/x-vp8;video/x-h264;video/x-vp9")
	}
	capsfilter.SetProperty("caps", caps)
	err = trPub.bin.AddMany(queue, capsfilter, tee, fakesink)
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot add sink elements for publisher")
	}
	err = gst.ElementLinkMany(queue, capsfilter, tee, fakesink)
	queue.SyncStateWithParent()
	capsfilter.SyncStateWithParent()
	tee.SyncStateWithParent()
	fakesink.SyncStateWithParent()
	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot link sink elements for publisher")
	}

	connected = false
	sinkPad = capsfilter.GetStaticPad("sink")
	trPub.inputSignalHandler, err = sinkPad.Connect("notify::caps", func(p *gst.Pad) {
		root.logger.Debugw("createSinkElementsForPublisher: notify::caps before notification")
		if connected {
			return
		}
		connected = true
		c := p.GetCurrentCaps()
		if c == nil {
			root.logger.Debugw("createSinkElementsForPublisher: notify::caps nil")
			return
		}
		root.logger.Debugw(fmt.Sprintf("createSinkElementsForPublisher: notify::caps %s", c.String()))
		str := c.GetStructureAt(0)
		if str != nil {
			codecMime := str.Name()
			err2 := trPub.completePublisherPipeline(codecMime)
			if err2 != nil {
				root.logger.Errorw(fmt.Sprintf("createSinkElementsForPublisher: cannot complete publisher pipeline for track %s ", trPub.publisher.id), err2)
			}

			trPub.setupLocalTrack(ingressId, screenShare)
		} else {
			root.logger.Debugw("createSinkElementsForPublisher: notify::caps no media declared")
		}
	})

	if err != nil {
		return errors.New("createSinkElementsForPublisher: Cannot listen for caps notification on sink capsfilter")
	}

	trPub.sinkTee = tee

	err = exposeSinkInBin(queue, trPub.bin)

	return err
}

func (tr *ov3TrackPublisher) ReadSamples() {
	var sample *gst.Sample
	var packet *rtp.Packet

	root.logger.Infow(fmt.Sprintf("ReadSamples: Starting reader task for publisher %s %s", tr.publisher.id, &tr.kind))
	// We start pushing media to LiveKit, so we request a keyframe just in case
	tr.HandlePLI()

	packet = &rtp.Packet{}
	for {
		tr.Lock()
		element := tr.element
		if element != nil {
			tr.Unlock()
			sample = element.TryPullSample(500 * time.Millisecond)
		} else {
			if tr.endStream.IsBroken() {
				tr.Unlock()
				root.logger.Infow(fmt.Sprintf("ReadSamples: Ending reader task for publisher %s %s", tr.publisher.id, &tr.kind))
				return
			}
			tr.Unlock()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		tr.Lock()
		localTrack := tr.track
		if tr.endStream.IsBroken() || (localTrack == nil) {
			tr.Unlock()
			root.logger.Infow(fmt.Sprintf("ReadSamples: Ending reader task for publisher %s %s", tr.publisher.id, &tr.kind))
			return
		}
		if sample != nil {
			buffer := sample.GetBuffer()
			if buffer != nil {
				packet.Unmarshal(buffer.Bytes())
				err := localTrack.WriteRTP(packet, nil)
				if err != nil {
					root.logger.Warnw(fmt.Sprintf("ReadSamples: could not write sample to local track %s %s", tr.publisher.id, &tr.kind), err)
				}
			}
		}
		tr.Unlock()
	}
}

func (tr *ov3TrackPublisher) CreateAudioPublishOptions(ingressId string, screenShare bool, stereo bool) *lksdk.TrackPublicationOptions {
	opts := &lksdk.TrackPublicationOptions{}

	opts.Name = "TRA_" + ingressId
	if screenShare {
		opts.Source = livekit.TrackSource_SCREEN_SHARE_AUDIO
	} else {
		opts.Source = livekit.TrackSource_MICROPHONE
	}

	opts.Stereo = stereo

	return opts
}

func (tr *ov3TrackPublisher) CreateVideoPublishOptions(ingressId string, screenShare bool, videoWidth int, videoHeight int) *lksdk.TrackPublicationOptions {
	opts := &lksdk.TrackPublicationOptions{}

	opts.Name = "TRV_" + ingressId
	if screenShare {
		opts.Source = livekit.TrackSource_SCREEN_SHARE
	} else {
		opts.Source = livekit.TrackSource_CAMERA
	}

	if videoWidth > 0 {
		opts.VideoWidth = videoWidth
	}
	if videoHeight > 0 {
		opts.VideoHeight = videoHeight
	}

	return opts
}

func getIntFieldFromGstStructure(str *gst.Structure, fieldName string) (int64, error) {
	field, err := str.GetValue(fieldName)

	if err != nil {
		return 0, err
	}

	val := reflect.ValueOf(field)
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(val.Uint()), nil
	default:
		return 0, errors.New("getIntFieldFromGstStructure: No field with requested name in structrure")
	}

}

func (tr *ov3TrackPublisher) PublishLocalTrack(ingressId string, screenShare bool, caps *gst.Caps) {
	var localTrack *lksdk.LocalSampleTrack
	var err error
	var webrtcCodec string
	var ltp *lksdk.LocalTrackPublication

	structure := caps.GetStructureAt(0)

	if structure == nil {
		root.logger.Warnw(fmt.Sprintf("PublishLocalTrack: no gstreamer caps availables in publisher %s, %s", tr.publisher.id, &tr.kind), nil)
		return
	}

	tr.codec = structure.Name()

	switch tr.codec {
	case "video/x-vp8":
		webrtcCodec = webrtc.MimeTypeVP8

	case "video/x-h264":
		webrtcCodec = webrtc.MimeTypeH264

	case "video/x-vp9":
		webrtcCodec = webrtc.MimeTypeVP9

	case "audio/x-opus":
		webrtcCodec = webrtc.MimeTypeOpus

	default:
		root.logger.Warnw(fmt.Sprintf("PublishLocalTrack: Codec %s not suppoted in publisher %s, %s", tr.codec, tr.publisher.id, &tr.kind), nil)
		return
	}
	localTrack, err = tr.createPublishTrack(webrtcCodec)

	if err != nil {
		root.logger.Errorw(fmt.Sprintf("PublishLocalTrack: cannot create local OpenVidu3 track for  publisher %s, %s", tr.publisher.id, tr.kind), nil)
		return
	}
	tr.track = localTrack
	if tr.kind == lksdk.TrackKindAudio {
		audioChannels, _ := getIntFieldFromGstStructure(structure, "channels")
		stereo := audioChannels == 2
		tr.opts = tr.CreateAudioPublishOptions(ingressId, screenShare, stereo)
		var stereoStr string

		if stereo {
			stereoStr = "stereo"
		} else {
			stereoStr = "mono"
		}
		root.logger.Debugw(fmt.Sprintf("PublishLocalTrack: publishing AUDIO track with options %s for publisher %s, %s", stereoStr, tr.publisher.id, tr.kind))
	} else if tr.kind == lksdk.TrackKindVideo {
		videoWidth, _ := getIntFieldFromGstStructure(structure, "width")
		videoHeight, _ := getIntFieldFromGstStructure(structure, "height")
		tr.opts = tr.CreateVideoPublishOptions(ingressId, screenShare, int(videoWidth), int(videoHeight))
		root.logger.Debugw(fmt.Sprintf("PublishLocalTrack: publishing VIDEO track with resolution %dx%d for publisher %s, %s", videoWidth, videoHeight, tr.publisher.id, tr.kind))
	} else {
		tr.opts = nil
	}

	room := tr.publisher.ingress.roomSvc
	if room != nil {
		ltp, err = room.LocalParticipant.PublishTrack(tr.track, tr.opts)
		if err != nil {
			root.logger.Errorw(fmt.Sprintf("PublishLocalTrack: could not publish %s track %s for publisher", tr.kind, tr.publisher.id), nil)
			tr.track = nil
			return
		}
		tr.sid = ltp.SID()

		go tr.ReadSamples()
	}
}

func (tr *ov3TrackPublisher) UnpublishLocalTrack() {
	tr.Lock()
	if tr.track == nil {
		tr.Unlock()
		return
	}
	tr.track = nil
	tr.endStream.Break()
	tr.Unlock()

	room := tr.publisher.ingress.roomSvc
	if (room != nil) && (tr.sid != "") {
		room.LocalParticipant.UnpublishTrack(tr.sid)
	}
}

func (tr *ov3TrackPublisher) checkCapsAreReadyForPublish(caps *gst.Caps) bool {
	var currentStructure *gst.Structure

	structure := caps.GetStructureAt(0)

	if tr.currentCaps != nil {
		currentStructure = structure
	}
	if tr.kind == lksdk.TrackKindAudio {
		c, err := getIntFieldFromGstStructure(structure, "channels")
		if err != nil {
			return false
		}
		if currentStructure != nil {
			c2, _ := getIntFieldFromGstStructure(currentStructure, "channels")

			if c2 == c {
				return false
			}
		}
	} else if tr.kind == lksdk.TrackKindVideo {
		w, err := getIntFieldFromGstStructure(structure, "width")
		if err != nil {
			return false
		}
		h, err2 := getIntFieldFromGstStructure(structure, "height")
		if err2 != nil {
			return false
		}
		if currentStructure != nil {
			w2, _ := getIntFieldFromGstStructure(currentStructure, "width")
			h2, _ := getIntFieldFromGstStructure(currentStructure, "height")

			if (w2 == w) && (h2 == h) {
				return false
			}
		}
	} else {
		return false
	}
	return true
}

func (tr *ov3TrackPublisher) setupLocalTrack(ingressId string, screenShare bool) {
	var pad *gst.Pad

	if tr.element == nil {
		root.logger.Warnw(fmt.Sprintf("setupLocalTrack: no gstreamer element for pubisher id %s, %s", tr.publisher.id, tr.kind), nil)
		return
	}
	pad = tr.sinkPad
	if pad != nil {
		caps := pad.GetCurrentCaps()
		if caps != nil {
			// Caps already negotiated, setting up codec track and creating local track
			ready := tr.checkCapsAreReadyForPublish(caps)
			if ready {
				root.logger.Debugw(fmt.Sprintf("setupLocalTrack: initializing track to %s for pubisher id %s, %s", tr.codec, tr.publisher.id, tr.kind))
				tr.PublishLocalTrack(ingressId, screenShare, caps)
			}
			caps = nil
		}
		// Listen for any cap change
		tr.sinkPadSignalHandler, _ = pad.Connect("notify::caps", func(p *gst.Pad) {
			// Caps have changed, create a new local track if not match with previous caps
			c := p.GetCurrentCaps()
			if c == nil {
				root.logger.Debugw("setupLocalTrack: caps nil")
				return
			}
			codecMime := c.GetStructureAt(0).Name()
			ready := tr.checkCapsAreReadyForPublish(c)
			if ready {
				if (tr.codec != "") && (tr.codec != codecMime) {
					root.logger.Debugw(fmt.Sprintf("setupLocalTrack: switching track from %s to %s for pubisher id %s, %s", tr.codec, codecMime, tr.publisher.id, tr.kind))
					tr.UnpublishLocalTrack()
				}

				root.logger.Debugw(fmt.Sprintf("setupLocalTrack: publishing track to %s for pubisher id %s, %s", tr.codec, tr.publisher.id, tr.kind))
				tr.codec = codecMime
				tr.currentCaps = c
				tr.PublishLocalTrack(ingressId, screenShare, c)
			}
		})
	} else {
		root.logger.Warnw(fmt.Sprintf("CreatePublisher: Cannot get %s sink pad for publisher %s", &tr.kind, tr.publisher.id), nil)
	}

}

func (tr *ov3TrackPublisher) DestroyTrackPublisher() {
	pad := tr.sinkPad
	if pad != nil {
		pad.HandlerDisconnect(tr.sinkPadSignalHandler)
	}
	capsfilter, _ := tr.bin.GetElementByName("inputCapsfilter")
	if capsfilter != nil {
		sinkPad := capsfilter.GetStaticPad("sink")
		if sinkPad != nil {
			sinkPad.HandlerDisconnect(tr.inputSignalHandler)
		}
	}
	tr.bin = nil
	tr.sinkTee = nil
	tr.element = nil
	tr.sinkPad = nil
	tr.currentCaps = nil
}

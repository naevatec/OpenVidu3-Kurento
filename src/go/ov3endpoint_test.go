package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/go-gst/go-gst/gst"
)

// *********************** Tests
func TestConnectToRoom(t *testing.T) {
	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}
}

func TestConnectToRoomFail(t *testing.T) {
	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "este esta mal"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId != "" {
		t.Errorf("EgressId connected, connection succes that should not")
		return
	}
}

func TestConnectDisconnectToRoom(t *testing.T) {
	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	errMsg := disconnectFromRoomEgressImpl(egressId)
	if strings.HasPrefix(errMsg, "ERROR") {
		t.Errorf("Could not disconnect from room")
		return
	}
}

func TestSeveralConnectionsToRoom(t *testing.T) {
	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	egressId2 := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId2 == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	if egressId != egressId2 {
		t.Errorf("EgressId does not match")
		return
	}

	errMsg := disconnectFromRoomEgressImpl(egressId)
	if strings.HasPrefix(errMsg, "ERROR") {
		t.Errorf("Could not disconnect from room")
		return
	}

	errMsg = disconnectFromRoomEgressImpl(egressId2)
	if !strings.HasPrefix(errMsg, "ERROR") {
		t.Errorf("Should not be a room to disconnect")
		return
	}
}

func TestSubscribeParticipantInRoom(t *testing.T) {
	var barrier sync.WaitGroup
	var finished taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, err := makeSubscriber(nil, "pipeline", &barrier, &finished)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	audioSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, false, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	time.Sleep(1000 * time.Second)
	//barrier.Wait()
	finished.finished = true

	audioSrc = nil
}

func TestSubscribeScreenShareParticipantInRoom(t *testing.T) {
	var barrier sync.WaitGroup
	var finished taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, err := makeSubscriber(nil, "pipeline", &barrier, &finished)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	audioSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, true, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	time.Sleep(1000 * time.Second)
	//barrier.Wait()
	finished.finished = true

	audioSrc = nil
}

func TestConcurrentSubscribers(t *testing.T) {
	var barrier sync.WaitGroup
	var finished taskFinished
	var finished2 taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, err := makeSubscriber(nil, "pipeline", &barrier, &finished)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}
	finished2 = taskFinished{
		finished: false,
	}
	_, audioSource2, videoSource2, err2 := makeSubscriber(pipeline, "pipeline", &barrier, &finished2)
	if err2 != nil {
		t.Errorf("Error creating gstreamer resources 2")
		return
	}

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	audioSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource.Instance()))
	audioSrc2 := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource2.Instance()))
	videoSrc2 := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource2.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, false, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	finished.lastAudioTime = time.Now()
	finished.lastVideoTime = time.Now()
	finished2.lastAudioTime = time.Now()
	finished2.lastVideoTime = time.Now()
	time.Sleep(10 * time.Second)
	for !finished.finished {
		audioTimeDiff := time.Since(finished.lastAudioTime)
		videoTimeDiff := time.Since(finished.lastVideoTime)
		if (audioTimeDiff > 500*time.Millisecond) && (videoTimeDiff > 500*time.Millisecond) {
			time.Sleep(1 * time.Second)
		} else {
			finished.finished = true
		}
	}
	dotFile := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("firstSubscriber.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}
	subscriberId2 := subscribeParticipantImpl(participantId, true, egressId, audioSrc2, videoSrc2)

	if strings.HasPrefix(subscriberId2, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}
	time.Sleep(10 * time.Second)
	dotFile2 := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("secondSubscriber.dot", []byte(dotFile2), 0666); err != nil {
		log.Fatal(err)
	}

	for !finished2.finished {
		if (time.Since(finished2.lastAudioTime) > 500*time.Millisecond) && (time.Since(finished2.lastVideoTime) > 500*time.Millisecond) {
			time.Sleep(1 * time.Second)
		} else {
			finished2.finished = true
		}
	}

	time.Sleep(1000 * time.Second)
	dotFile3 := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("final.dot", []byte(dotFile3), 0666); err != nil {
		log.Fatal(err)
	}

	unsubscribeParticipantImpl(subscriberId2)
	unsubscribeParticipantImpl(subscriberId)

	time.Sleep(10 * time.Second)
}

func TestPublishParticipantInRoom(t *testing.T) {
	var finished taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, audioSinkC, videoSinkC, err := makeSubscriber2(nil, "pipeline")
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	ingressId := connectToRoomImpl(url, key, secret, room, "Test", "Test")

	dotFile := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("initial.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}

	audioSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource.Instance()))
	audioSnk := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSinkC.Instance()))
	videoSnk := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSinkC.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, false, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	publishId := publishParticipantImpl(false, ingressId, audioSnk, videoSnk)

	time.Sleep(10 * time.Second)
	dotFile = pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("delayed.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}
	time.Sleep(10 * time.Second)

	unpublishParticipantImpl(false, publishId)
	disconnectFromRoomIngressImpl(ingressId)
	unsubscribeParticipantImpl(subscriberId)
	disconnectFromRoomEgressImpl(egressId)
	finished.finished = true

	audioSrc = nil
}

func TestPublishScreenShareInRoom(t *testing.T) {
	var finished taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room, "", "")

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, _, videoSinkC, err := makeSubscriber2(nil, "pipeline")
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	ingressId := connectToRoomImpl(url, key, secret, room, "Test", "Test")

	dotFile := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("initial.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}

	audioSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSource.Instance()))
	videoSnk := (*_Ctype_struct__GstBin)(unsafe.Pointer(videoSinkC.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, false, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	publishParticipantImpl(true, ingressId, nil, videoSnk)

	time.Sleep(1000 * time.Second)
	dotFile = pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("delayed.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}

	unpublishParticipantImpl(false, ingressId)
	disconnectFromRoomIngressImpl(ingressId)
	disconnectFromRoomEgressImpl(egressId)
	finished.finished = true

	audioSrc = nil
}

/*
func TestSubscribeUnsubscribeParticipantInRoom(t *testing.T) {
	var barrier sync.WaitGroup
	var finished taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room)

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished = taskFinished{
		finished: false,
	}
	pipeline, audioSource, videoSource, err := makeSubscriber(nil, "pipeline", &barrier, &finished)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	audioSrc := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(audioSource.Instance()))
	videoSrc := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(videoSource.Instance()))
	subscriberId := subscribeParticipantImpl(participantId, false, egressId, audioSrc, videoSrc)

	if strings.HasPrefix(subscriberId, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	barrier.Wait()
	finished.finished = true

	unsubscribeParticipant(subscriberId)

	audioSrc = nil
}

func TestTwoSubscribeParticipantInRoom(t *testing.T) {
	var barrier sync.WaitGroup
	var pipeline *gst.Pipeline
	var audioSource1 *app.Source
	var videoSource1 *app.Source
	var audioSource2 *app.Source
	var videoSource2 *app.Source
	var err error
	var finished1 taskFinished
	var finished2 taskFinished

	url := "https://livekit.mymeeting-dev.naevatec.com:443"
	key := "APIjJf7zm7zxqgJ"
	secret := "ZZShJO570vLjy5MbZeBa9X8SJVae7CdMRVVJ54UMPHj"
	room := "7tps-vk8m"
	egressId := connectToRoomImpl(url, key, secret, room)

	if egressId == "" {
		t.Errorf("EgressId null, connection failed")
		return
	}

	participantId := "saul"

	gst.Init(nil)

	finished1 = taskFinished{
		finished: false,
	}
	finished2 = taskFinished{
		finished: false,
	}
	pipeline, audioSource1, videoSource1, err = makeSubscriber(nil, "pipeline", &barrier, &finished1)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	pipeline, audioSource2, videoSource2, err = makeSubscriber(pipeline, "pipeline", &barrier, &finished2)
	if err != nil {
		t.Errorf("Error creating gstreamer resources")
		return
	}

	// Start the pipeline
	pipeline.SetState(gst.StatePlaying)

	audioSrc1 := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(audioSource1.Instance()))
	videoSrc1 := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(videoSource1.Instance()))
	subscriberId1 := subscribeParticipantImpl(participantId, false, egressId, audioSrc1, videoSrc1)
	if strings.HasPrefix(subscriberId1, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}
	audioSrc2 := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(audioSource2.Instance()))
	videoSrc2 := (*_Ctype_struct__GstAppSrc)(unsafe.Pointer(videoSource2.Instance()))
	subscriberId2 := subscribeParticipantImpl(participantId, false, egressId, audioSrc2, videoSrc2)
	if strings.HasPrefix(subscriberId2, "ERROR") {
		t.Errorf("subscribeParticipant ended with an error")
		return
	}

	barrier.Wait()

	finished1.finished = true
	finished2.finished = true

}
*/

type taskFinished struct {
	finished  bool
	audioFlow bool
	videoFlow bool

	lastAudioTime time.Time
	lastVideoTime time.Time
}

var binIdx int32

func makeSubscriber(pipe *gst.Pipeline, pipelineName string, barrier *sync.WaitGroup, finished *taskFinished) (*gst.Pipeline, *gst.Bin, *gst.Bin, error) {
	var pipeline *gst.Pipeline
	var err error

	binIdx++
	if pipe == nil {
		pipeline, err = gst.NewPipeline(pipelineName)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		pipeline = pipe
	}

	src := gst.NewBin(fmt.Sprintf("audioBin%d", binIdx))
	if err := pipeline.Add(src.Element); err != nil {
		return nil, nil, nil, err
	}

	audioSource := src

	audioSource.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
		audioSink, err := gst.NewElement("fakesink")
		if err != nil {
			return
		}
		if err := pipeline.Add(audioSink); err != nil {
			return
		}
		audioSink.SyncStateWithParent()
		err = audioSource.Link(audioSink)
		if err != nil {
			finished.lastAudioTime = time.Now()
		}
		srcPad.AddProbe(gst.PadProbeTypeBuffer, func(self *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			// Interpret the data sent over the pad as a buffer. We know to expect this because of
			// the probe mask defined above.
			finished.lastAudioTime = time.Now()
			flow := finished.audioFlow
			finished.audioFlow = true
			if (finished.finished) || flow {
				return gst.PadProbeOK
			}
			barrier.Done()
			return gst.PadProbeOK
		})
	})

	finished.audioFlow = false
	barrier.Add(1)

	src = gst.NewBin(fmt.Sprintf("videoBin%d", binIdx))
	if err := pipeline.Add(src.Element); err != nil {
		return nil, nil, nil, err
	}
	videoSource := src
	videoSource.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
		videoSink, err := gst.NewElement("fakesink")
		if err != nil {
			return
		}
		if err := pipeline.Add(videoSink); err != nil {
			return
		}
		videoSink.SyncStateWithParent()
		err = videoSource.Link(videoSink)
		if err != nil {
			finished.lastVideoTime = time.Now()
		}
		srcPad.AddProbe(gst.PadProbeTypeBuffer, func(self *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			// Interpret the data sent over the pad as a buffer. We know to expect this because of
			// the probe mask defined above.
			finished.lastVideoTime = time.Now()
			flow := finished.videoFlow
			finished.videoFlow = true
			if (finished.finished) || flow {
				return gst.PadProbeOK
			}
			barrier.Done()
			return gst.PadProbeOK
		})
	})
	finished.videoFlow = false
	barrier.Add(1)

	// Add a probe handler on the audiotestsrc's src-pad.
	// This handler gets called for every buffer that passes the pad we probe.
	/*srcPadVideo.AddProbe(gst.PadProbeTypeBuffer, func(self *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		// Interpret the data sent over the pad as a buffer. We know to expect this because of
		// the probe mask defined above.
		flow := finished.videoFlow
		finished.videoFlow = true
		if (finished.finished) || flow {
			return gst.PadProbeOK
		}
		barrier.Done()
		return gst.PadProbeOK
	})

	// Add a probe handler on the audiotestsrc's src-pad.
	// This handler gets called for every buffer that passes the pad we probe.
	srcPadAudio.AddProbe(gst.PadProbeTypeBuffer, func(self *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		// Interpret the data sent over the pad as a buffer. We know to expect this because of
		// the probe mask defined above.
		flow := finished.audioFlow
		finished.audioFlow = true
		if (finished.finished) || flow {
			return gst.PadProbeOK
		}
		barrier.Done()
		return gst.PadProbeOK
	})*/

	return pipeline, audioSource, videoSource, nil
}

func completeAudioConnection(srcPad *gst.Pad, sinkPad *gst.Pad, pipeline *gst.Pipeline, videoConnected bool) {
	result := srcPad.LinkMaybeGhosting(sinkPad)

	if !result {
		return
	}
	if videoConnected {
		// Start the pipeline
		pipeline.SetState(gst.StatePlaying)
	}
	dotFile := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("audioPublish.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}

}

func completeVideoConnection(srcPad *gst.Pad, sinkPad *gst.Pad, pipeline *gst.Pipeline, audioConnected bool) {
	result := srcPad.LinkMaybeGhosting(sinkPad)

	if !result {
		return
	}
	if audioConnected {
		// Start the pipeline
		pipeline.SetState(gst.StatePlaying)
	}
	dotFile := pipeline.DebugBinToDotData(gst.DebugGraphShowAll)
	if err := os.WriteFile("videoPublish.dot", []byte(dotFile), 0666); err != nil {
		log.Fatal(err)
	}

}

func makeSubscriber2(pipe *gst.Pipeline, pipelineName string) (*gst.Pipeline, *gst.Bin, *gst.Bin, *gst.Bin, *gst.Bin, error) {
	var pipeline *gst.Pipeline
	var err error
	var audioConnected, videoConnected bool

	audioConnected = false
	videoConnected = false

	binIdx++
	if pipe == nil {
		pipeline, err = gst.NewPipeline(pipelineName)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
	} else {
		pipeline = pipe
	}

	src := gst.NewBin(fmt.Sprintf("audioSrcBin%d", binIdx))
	if err := pipeline.Add(src.Element); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	audioSource := src
	audioSource.SyncStateWithParent()

	src = gst.NewBin(fmt.Sprintf("videoSrcBin%d", binIdx))
	if err := pipeline.Add(src.Element); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	videoSource := src
	videoSource.SyncStateWithParent()

	sink := gst.NewBin(fmt.Sprintf("audioSinkBin%d", binIdx))
	if err := pipeline.Add(sink.Element); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	audioSink := sink
	audioSink.SyncStateWithParent()

	sink = gst.NewBin(fmt.Sprintf("videoSinkBin%d", binIdx))
	if err := pipeline.Add(sink.Element); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	videoSink := sink
	videoSink.SyncStateWithParent()

	audioSource.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
		sinkPad := audioSink.GetStaticPad("sink")

		if sinkPad == nil {
			audioSink.Connect("pad-added", func(sel *gst.Element, sinkPad2 *gst.Pad) {
				completeAudioConnection(srcPad, sinkPad2, pipeline, videoConnected)
				audioConnected = true
			})
		} else {
			completeAudioConnection(srcPad, sinkPad, pipeline, videoConnected)
			audioConnected = true
		}
	})

	videoSource.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
		sinkPad := videoSink.GetStaticPad("sink")

		if sinkPad == nil {
			videoSink.Connect("pad-added", func(sel *gst.Element, sinkPad2 *gst.Pad) {
				completeVideoConnection(srcPad, sinkPad2, pipeline, audioConnected)
				videoConnected = true
			})
		} else {
			completeVideoConnection(srcPad, sinkPad, pipeline, audioConnected)
			videoConnected = true
		}
	})

	return pipeline, audioSource, videoSource, audioSink, videoSink, nil
}

package main

// #include <gst/app/gstappsrc.h>
// #include <gst/gstpad.h>
// #include <gst/gstbin.h>
import "C"

import (
	"context"
	"fmt"
	"strings"
	"unsafe"

	guuid "github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/tinyzimmer/go-glib/glib"
	"github.com/tinyzimmer/go-gst/gst"
)

func createEgressId() string {
	return "GSTEG_" + guuid.New().String()
}

func createRoomConnection(room *ov3Room) (string, error) {
	egressId := createEgressId()
	err := room.makeRoomConnection(egressId)
	if err != nil {
		return "", err
	}
	room.connected = true

	return egressId, nil
}

func createIngressId() string {
	return "GSTIG_" + guuid.New().String()
}

func createRoomIngress(room *ov3Room, publisherName string, publisherId string) (*ov3Ingress, error) {
	var ingressId string

	if publisherId == "" {
		ingressId = createIngressId()
	} else {
		ingressId = publisherId
	}

	ing := &ov3Ingress{}
	ing.ingressId = ingressId
	ing.room = room
	ing.participantName = publisherName
	ing.mainPub = nil
	ing.screenSharePub = nil
	ing.connected = false

	err := ing.makeRoomIngressConnection(ingressId, publisherName)
	if err != nil {
		return nil, err
	}
	ing.connected = true

	return ing, nil
}

func connectToRoomImpl(url string, key string, secret string, room string, publisherName string, publisherId string) string {
	var egressId string
	var ingress *ov3Ingress
	var err error

	root.logger.Debugw(fmt.Sprintf("connectToRoomImpl: participantId %s with name %s room %s on %s", publisherId, publisherName, room, url))

	svc := root.getService(url)
	if svc == nil {
		root.logger.Debugw(fmt.Sprintf("connectToRoomImpl: storing service %s", url))
		svc = root.addService(url, secret, key)
	}
	roomSvc := svc.getRoom(room)

	if roomSvc == nil {
		root.logger.Debugw(fmt.Sprintf("connectToRoomImpl: storing room %s", url))
		roomSvc = svc.addRoom(room)
	}

	if publisherName == "" {
		// No publisher, we are connecting for egress
		if roomSvc.egressId == "" {
			// Connection for subscription and subscription needed to create
			egressId, err = createRoomConnection(roomSvc)
			if err != nil {
				return ""
			}
			roomSvc.egressId = egressId
			root.addEgress(roomSvc.egressId, roomSvc)
		}
		return roomSvc.egressId
	} else {
		// Publisher name given, we are connecting for ingress

		// If no publisherId given, we create a new publisher participant
		// Also if not yet created an ingress participant
		ingress = roomSvc.getIngressByParticipant(publisherId)
		if ingress == nil {
			// Connection for publishing
			ingress, err = createRoomIngress(roomSvc, publisherName, publisherId)
			if err != nil {
				return ""
			}
			roomSvc.addIngress(ingress)
		}

		return ingress.ingressId
	}

}

func disconnectFromRoomIngressImpl(ingressId string) string {
	root.logger.Debugw(fmt.Sprintf("disconnectFromRoomIngressImpl: ingress Id %s", ingressId))
	ingressSvc := root.getIngress(ingressId)
	if ingressSvc == nil {
		return "ERROR: Ingress " + ingressId + " is not available"
	}

	if (ingressSvc.mainPub != nil) || (ingressSvc.screenSharePub != nil) {
		return "ERROR: ingress " + ingressId + "  in service already has active publisher"
	}
	ingressSvc.connected = false
	if ingressSvc.room.roomClient != nil {
		_, err := ingressSvc.room.roomClient.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
			Room:     ingressSvc.room.room,
			Identity: ingressSvc.ingressId,
		})
		if err != nil {
			return "ERROR: ingress " + ingressId + "  cannot remove OpenVidu3 participant"
		}
	}
	room := ingressSvc.room
	room.deleteIngress(ingressId)

	if (len(room.ingress) == 0) &&
		(len(room.subscriptions) == 0) &&
		(len(room.ssSubscriptions) == 0) {

		service := room.service
		room.roomSvc.Disconnect()
		service.deleteRoom(room.room)
		if len(service.rooms) == 0 {
			root.deleteService(service.url)
		}
	}

	return ingressId
}

func disconnectFromRoomEgressImpl(egressId string) string {
	root.logger.Debugw(fmt.Sprintf("disconnectFromRoomEgressImpl: egress Id %s", egressId))
	roomSvc := root.getEgress(egressId)
	if roomSvc == nil {
		return "ERROR: Egress " + egressId + " is not available"
	}
	roomSvc.Lock()
	defer roomSvc.Unlock()

	if (len(roomSvc.subscriptions) > 0) || (len(roomSvc.ssSubscriptions) > 0) {
		return "ERROR: room " + roomSvc.room + "  in service " + roomSvc.service.url + "  already has active subscriptions"
	}
	service := roomSvc.service
	roomSvc.connected = false
	root.deleteEgress(roomSvc.egressId)
	roomSvc.egressId = ""
	// FIXME: Change this for a RemoveParticipant of the egress participant
	// It implies a more deep change about maintaining a single connection to the roomSvc and to the roomClient as long as
	// there is at least one participant, egress or ingress
	if len(roomSvc.ingress) > 0 {
		if roomSvc.roomClient != nil {
			_, err := roomSvc.roomClient.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
				Room:     roomSvc.room,
				Identity: egressId,
			})
			if err != nil {
				return "ERROR: ingress " + egressId + "  cannot remove OpenVidu3 participant"
			}
		}
	} else {
		roomSvc.roomSvc.Disconnect()
		roomSvc.service.deleteRoom(roomSvc.room)
		if len(service.rooms) == 0 {
			root.deleteService(service.url)
		}
	}

	return ""
}

func subscribe(track lksdk.TrackPublication, screenShare bool) *lksdk.RemoteTrackPublication {
	if pub, ok := track.(*lksdk.RemoteTrackPublication); ok {
		if pub.IsSubscribed() {
			return pub
		}

		subscription := root.getSubscribedTrack(pub.SID())
		if subscription != nil {
			source := pub.Source()
			isScreenShare := (source == livekit.TrackSource_SCREEN_SHARE) || (source == livekit.TrackSource_SCREEN_SHARE_AUDIO)
			isNotScreenShare := (source == livekit.TrackSource_CAMERA) || (source == livekit.TrackSource_MICROPHONE)
			if (screenShare && isScreenShare) || (!screenShare && isNotScreenShare) {
				root.logger.Debugw(fmt.Sprintf("subscribe (2): subscribing to track %s", track.SID()))

				err := pub.SetSubscribed(true)
				if err != nil {
					root.logger.Debugw(fmt.Sprintf("subscribe (2): could not subscribe to track %s", track.SID()))
					return nil
				}
				return pub
			}
		}
	}

	return nil
}

func WrapBin(bin *C.GstBin) *gst.Bin {
	return &gst.Bin{
		Element: &gst.Element{
			Object: &gst.Object{
				InitiallyUnowned: &glib.InitiallyUnowned{
					Object: &glib.Object{
						GObject: glib.ToGObject(unsafe.Pointer(bin)),
					},
				},
			},
		},
	}
}

func subscribeParticipantImpl(participantId string, screenShare bool, egressId string, audioSourceC *C.GstBin, videoSourceC *C.GstBin) string {
	root.logger.Debugw(fmt.Sprintf("subscribeParticipantImpl: participant %s, egress Id %s, and screenshare %t", participantId, egressId, screenShare))
	roomSvc := root.getEgress(egressId)
	if roomSvc == nil {
		return "ERROR: Egress " + egressId + "  not avilable in service "
	}

	audioSource := WrapBin(audioSourceC)
	videoSource := WrapBin(videoSourceC)

	subscriber := roomSvc.addSubscriber(participantId, screenShare, audioSource, videoSource)
	root.addSubscriber(subscriber.id, subscriber)

	return subscriber.id
}

func requestKeyFrameImpl(subscriberId string) {
	root.logger.Debugw(fmt.Sprintf("requestKeyFrameImpl: %s", subscriberId))
	subscriber := root.getSubscriber(subscriberId)

	if subscriber != nil {
		writer := subscriber.subscription.videoWriter
		videoReady := subscriber.videoReady

		if (writer != nil) && videoReady {
			root.logger.Debugw(fmt.Sprintf("requestKeyFrameImpl: requesting PLI on track %s", writer.pub.SID()))
			writer.sendPLI()
		}
	}
}

func unsubscribeParticipantImpl(subscriberId string) string {
	root.logger.Debugw(fmt.Sprintf("unsubscribeParticipantImpl: subscriberId %s", subscriberId))
	subscriber := root.getSubscriber(subscriberId)
	if subscriber == nil {
		return "ERROR: Subscriber with id " + subscriberId + " does not exist"
	}

	root.deleteSubscriber(subscriberId)
	roomSvc := subscriber.subscription.room

	roomSvc.removeSubscriber(subscriber)
	return subscriberId
}

func publishParticipantImpl(screenShare bool, ingressId string, audioSinkC *C.GstBin, videoSinkC *C.GstBin) string {
	var err error
	var publisher *ov3Publisher
	var audioSink *gst.Bin
	var videoSink *gst.Bin
	var mediaMode string

	if audioSinkC != nil {
		if videoSinkC != nil {
			mediaMode = "Audio + Video"
		} else {
			mediaMode = "Audio"
		}
	} else {
		if videoSinkC != nil {
			mediaMode = "Video"
		} else {
			mediaMode = "No media"
		}
	}

	root.logger.Debugw(fmt.Sprintf("publishParticipantImpl: publishing %s on ingress Id %s, and screenshare %t", mediaMode, ingressId, screenShare))

	ing := root.getIngress(ingressId)

	if ing == nil {
		root.logger.Warnw(fmt.Sprintf("publishParticipantImpl: ingress %s not available", ingressId), nil)
		return "ERROR: ingress not available"
	}

	if screenShare && (ing.screenSharePub != nil) {
		root.logger.Errorw(fmt.Sprintf("publishParticipantImpl: ingress %s screenshare already publishing", ingressId), nil)
		return "ERROR: Already publishing"
	} else if !screenShare && (ing.mainPub != nil) {
		root.logger.Errorw(fmt.Sprintf("publishParticipantImpl: ingress %s already publishing", ingressId), nil)
		return "ERROR: Already publishing"
	}

	if audioSinkC != nil {
		audioSink = WrapBin(audioSinkC)
	}

	if videoSinkC != nil {
		videoSink = WrapBin(videoSinkC)
	}

	if screenShare {
		root.logger.Debugw(fmt.Sprintf("publishParticipantImpl: ingress %s  publishing screenshare", ingressId))
		publisher, err = ing.PublishScreenShare(audioSink, videoSink)
	} else {
		root.logger.Debugw(fmt.Sprintf("publishParticipantImpl: ingress %s  publishing main", ingressId))
		publisher, err = ing.PublishMain(audioSink, videoSink)
	}
	root.logger.Debugw(fmt.Sprintf("publishParticipantImpl: ingress %s  published", ingressId))

	if err != nil {
		root.logger.Errorw(fmt.Sprintf("publishParticipantImpl: ingress %s could not publish", ingressId), err)
		return "ERROR: " + err.Error()
	}

	root.addPublisher(publisher.id, publisher)

	return publisher.id
}

func unpublishParticipantImpl(screenShare bool, publisherId string) string {
	var err error

	publisher := root.getPublisher(publisherId)
	if publisher == nil {
		root.logger.Debugw(fmt.Sprintf("unpublishParticipantImpl: cannot find publisher with Id %s", publisherId))
		return "ERROR: publisher not found"
	}

	root.logger.Debugw(fmt.Sprintf("unpublishParticipantImpl: publisher Id %s, and screenshare %t on ingress %s", publisher.id, screenShare, publisher.ingress.ingressId))

	ing := publisher.ingress

	if ing == nil {
		root.logger.Warnw(fmt.Sprintf("unpublishParticipantImpl: ingress %s not available", publisherId), nil)
		return "ERROR: ingress not available"
	}

	if screenShare && (ing.screenSharePub == nil) {
		root.logger.Errorw(fmt.Sprintf("unpublishParticipantImpl: ingress %s screenshare not publishing", publisherId), nil)
		return "ERROR: Not publishing"
	} else if !screenShare && (ing.mainPub == nil) {
		root.logger.Errorw(fmt.Sprintf("unpublishParticipantImpl: ingress %s not publishing", publisherId), nil)
		return "ERROR: Not publishing"
	}

	if screenShare {
		_, err = ing.UnpublishScreenShare()
	} else {
		_, err = ing.UnpublishMain()
	}

	if err != nil {
		return "ERROR: " + err.Error()
	}

	root.deletePublisher(publisherId)

	return publisherId
}

//export connectToRoom
func connectToRoom(url *C.char, key *C.char, secret *C.char, room *C.char, publisherName *C.char, publisherId *C.char) (ret *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on connectToRoom ", err))
			root.logger.Infow("connectToRoom: error connecting")
			ret = C.CString("ERROR: Panic connecting")
		}
	}()

	var pubName string
	var pubId string

	if publisherName == nil {
		pubName = ""
	} else {
		pubName = C.GoString(publisherName)
	}
	if publisherId == nil {
		pubId = ""
	} else {
		pubId = C.GoString(publisherId)
	}
	result := connectToRoomImpl(C.GoString(url), C.GoString(key), C.GoString(secret), C.GoString(room), pubName, pubId)

	return C.CString(result)
}

//export disconnectFromRoom
func disconnectFromRoom(egressId *C.char) (ret *C.char) {
	var result string

	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on disconnectFromRoom ", err))
			root.logger.Infow("disconnectFromRoom: error disconnecting")
			ret = C.CString("ERROR: Panic disconnecting")
		}
	}()
	id := C.GoString(egressId)

	if strings.HasPrefix(id, "GSTEG_") {
		result = disconnectFromRoomEgressImpl(id)
	} else {
		result = disconnectFromRoomIngressImpl(id)
	}

	return C.CString(result)
}

//export subscribeParticipant
func subscribeParticipant(participantId *C.char, screenShare bool, egressId *C.char, audioSourceC *C.GstBin, videoSourceC *C.GstBin) (ret *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on subscribeParticipant ", err))
			root.logger.Infow("subscribeParticipant: error subscribing")
			ret = C.CString("ERROR: Panic subscribing")
		}
	}()
	result := subscribeParticipantImpl(C.GoString(participantId), screenShare, C.GoString(egressId), audioSourceC, videoSourceC)

	return C.CString(result)
}

//export requestKeyFrame
func requestKeyFrame(subscriberId *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on requestKeyFrame ", err))
			root.logger.Infow("requestKeyFrame: error")
		}
	}()
	requestKeyFrameImpl(C.GoString(subscriberId))
}

//export unsubscribeParticipant
func unsubscribeParticipant(subscriberId *C.char) (ret *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on unsubscribePartipant ", err))
			root.logger.Infow("unsubscribeParticipant: error unsubscring")
			ret = C.CString("ERROR: Panic unsubscribing")
		}
	}()

	result := unsubscribeParticipantImpl(C.GoString(subscriberId))

	return C.CString(result)
}

//export publishParticipant
func publishParticipant(screenshare bool, ingressId *C.char, audioSink *C.GstBin, videoSink *C.GstBin) (ret *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on publishParticipant ", err))
			root.logger.Infow("publishParticipant: error publishing")
			ret = C.CString("ERROR: Panic publishing")
		}
	}()
	result := publishParticipantImpl(screenshare, C.GoString(ingressId), audioSink, videoSink)

	return C.CString(result)

}

//export unpublishParticipant
func unpublishParticipant(screenShare bool, publisherId *C.char) (ret *C.char) {
	defer func() {
		if err := recover(); err != nil {
			root.logger.Infow(fmt.Sprintln("panic occurred on unpublishParticipant ", err))
			root.logger.Infow("unpublishParticipant: error unpublishing")
			ret = C.CString("ERROR: Panic unpublishing")
		}
	}()
	result := unpublishParticipantImpl(screenShare, C.GoString(publisherId))

	return C.CString(result)
}

func main() {}

package main

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-gst/go-gst/gst"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type ov3Publisher struct {
	sync.RWMutex

	id      string
	ingress *ov3Ingress

	audioPublisher *ov3TrackPublisher
	videoPublisher *ov3TrackPublisher
}

func (pub *ov3Publisher) removeTrackPublisher(trPub *ov3TrackPublisher) {
	trPub.DestroyTrackPublisher()
}

func exposeSinkInBin(element *gst.Element, bin *gst.Bin) error {
	pad := element.GetStaticPad("sink")
	if pad == nil {
		return errors.New("cannot get sink pad from element")
	}

	ghostPad := gst.NewGhostPad("sink", pad)
	added := bin.AddPad(ghostPad.ProxyPad.Pad)
	if !added {
		return errors.New("could not expose media sink in bin")
	}
	return nil
}

func (pub *ov3Publisher) createTrackPublisher(kind lksdk.TrackKind, bin *gst.Bin, ingressId string, screenShare bool) *ov3TrackPublisher {
	var err error
	var trPub *ov3TrackPublisher

	root.logger.Debugw(fmt.Sprintf("createTrackPublisher: creating sink pipeline for track %s %s", kind, pub.id))
	trPub = &ov3TrackPublisher{}
	trPub.publisher = pub
	trPub.kind = kind
	trPub.bin = bin
	err = trPub.createSinkElementsForPublisher(kind, ingressId, screenShare)
	if err != nil {
		root.logger.Errorw(fmt.Sprintf("createTrackPublisher: Could not create sink pipeline for track %s %s", kind, pub.id), err)
		trPub.bin = nil
		trPub = nil
		return nil
	}
	root.logger.Debugw(fmt.Sprintf("createTrackPublisher: created sink pipeline for track %s %s", kind, pub.id))
	return trPub
}

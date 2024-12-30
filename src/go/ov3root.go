package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/livekit/protocol/logger"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

type ov3Root struct {
	sync.RWMutex
	services         map[string]*ov3Service
	subscribers      map[string]*ov3Subscriber
	subscribedTracks map[string]*ov3Subscription
	egress           map[string]*ov3Room
	ingress          map[string]*ov3Ingress
	publishers       map[string]*ov3Publisher
	logr             logr.Logger
	logger           logger.Logger
}

var root ov3Root
var debug = false
var logPath string

func init() {
	root.initLogger()
}

func NewFileLogger(filename string) logr.Logger {
	var writer io.Writer

	file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if file != nil {
		writer = io.Writer(file)
	}

	return funcr.New(func(prefix, args string) {
		t := time.Now()
		timeSuffix := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d.%09d",
			t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
		if prefix != "" {
			fmt.Fprintf(writer, "%s - %s: %s\n", timeSuffix, prefix, args)
		} else {
			fmt.Fprintf(writer, "%s - %s\n", timeSuffix, args)
		}
	}, funcr.Options{
		Verbosity: 1,
	})
}

func (rt *ov3Root) initLogger() {
	if rt.logger == nil {
		var ok bool
		logPath, ok = os.LookupEnv("KURENTO_LOGS_PATH")
		if !ok {
			logPath = "/tmp"
		}
		t := time.Now()
		timeSuffix := fmt.Sprintf("%d-%02d-%02dT%02d%02d%02d",
			t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second())
		rt.logr = NewFileLogger(logPath + "/goov3_" + timeSuffix + ".log")
		logger.InitFromConfig(&logger.Config{
			Level: "debug",
			ComponentLevels: map[string]string{
				"ov3endpoint": "debug",
			},
		}, "ov3endpoint")
		logger.SetLogger(logger.LogRLogger(rt.logr), "ov3endpoint")
		rt.logger = logger.GetLogger()
		rt.logger.Infow("logger started")
		_, ok = os.LookupEnv(("KURENTO_LK_SUBSCRIBER_PTS_LOG"))
		debug = ok
		lksdk.SetLogger(rt.logger)
	}
}

func (rt *ov3Root) getSubscriber(id string) *ov3Subscriber {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribers == nil {
		rt.subscribers = make(map[string]*ov3Subscriber)
	}
	result := rt.subscribers[id]
	return result
}

func (rt *ov3Root) deleteSubscriber(id string) *ov3Subscriber {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribers == nil {
		rt.subscribers = make(map[string]*ov3Subscriber)
	}
	result := rt.subscribers[id]
	if result != nil {
		delete(rt.subscribers, id)
	}
	return result
}

func (rt *ov3Root) addSubscriber(id string, subs *ov3Subscriber) {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribers == nil {
		rt.subscribers = make(map[string]*ov3Subscriber)
	}
	rt.subscribers[id] = subs
}

func (rt *ov3Root) addPublisher(id string, pub *ov3Publisher) {
	rt.Lock()
	defer rt.Unlock()

	if rt.publishers == nil {
		rt.publishers = make(map[string]*ov3Publisher)
	}
	rt.publishers[id] = pub
}

func (rt *ov3Root) deletePublisher(id string) *ov3Publisher {
	rt.Lock()
	defer rt.Unlock()

	if rt.publishers == nil {
		rt.publishers = make(map[string]*ov3Publisher)
	}
	pub := rt.publishers[id]
	if pub != nil {
		delete(rt.publishers, id)
	}

	return pub
}

func (rt *ov3Root) getPublisher(id string) *ov3Publisher {
	rt.Lock()
	defer rt.Unlock()

	if rt.publishers == nil {
		rt.publishers = make(map[string]*ov3Publisher)
	}
	result := rt.publishers[id]
	return result
}

func (rt *ov3Root) addSubscribedTrack(trackId string, subscription *ov3Subscription) {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribedTracks == nil {
		rt.subscribedTracks = make(map[string]*ov3Subscription)
	}
	rt.subscribedTracks[trackId] = subscription
}

func (rt *ov3Root) removeSubscribedTrack(trackId string) *ov3Subscription {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribedTracks == nil {
		rt.subscribedTracks = make(map[string]*ov3Subscription)
	}
	result := rt.subscribedTracks[trackId]
	if result != nil {
		delete(rt.subscribedTracks, trackId)
	}

	return result
}

func (rt *ov3Root) getSubscribedTrack(trackId string) *ov3Subscription {
	rt.Lock()
	defer rt.Unlock()

	if rt.subscribedTracks == nil {
		rt.subscribedTracks = make(map[string]*ov3Subscription)
	}
	result := rt.subscribedTracks[trackId]

	return result
}

func (rt *ov3Root) getService(url string) *ov3Service {
	rt.Lock()
	defer rt.Unlock()

	if rt.services == nil {
		rt.services = make(map[string]*ov3Service)
	}
	result := rt.services[url]
	return result
}

func (rt *ov3Root) addService(url string, secret string, key string) *ov3Service {
	rt.Lock()
	defer rt.Unlock()

	if rt.services == nil {
		rt.services = make(map[string]*ov3Service)
	}
	svc := ov3Service{}
	svc.url = url
	svc.key = key
	svc.secret = secret
	svc.rooms = make(map[string]*ov3Room)

	rt.services[url] = &svc

	return &svc
}

func (rt *ov3Root) deleteService(url string) *ov3Service {
	rt.Lock()
	defer rt.Unlock()

	if rt.services == nil {
		rt.services = make(map[string]*ov3Service)
	}
	result := rt.services[url]
	if result != nil {
		delete(rt.services, url)
	}
	return result
}

func (rt *ov3Root) getEgress(egressId string) *ov3Room {
	rt.Lock()
	defer rt.Unlock()

	if rt.egress == nil {
		rt.egress = make(map[string]*ov3Room)
	}
	result := rt.egress[egressId]
	return result
}

func (rt *ov3Root) addEgress(egressId string, room *ov3Room) {
	rt.Lock()
	defer rt.Unlock()

	if rt.egress == nil {
		rt.egress = make(map[string]*ov3Room)
	}
	rt.egress[egressId] = room
}

func (rt *ov3Root) deleteEgress(egressId string) *ov3Room {
	rt.Lock()
	defer rt.Unlock()

	if rt.egress == nil {
		rt.egress = make(map[string]*ov3Room)
	}
	result := rt.egress[egressId]
	if result != nil {
		delete(rt.egress, egressId)
	}
	return result
}

func (rt *ov3Root) getIngress(ingressId string) *ov3Ingress {
	rt.Lock()
	defer rt.Unlock()

	if rt.ingress == nil {
		rt.ingress = make(map[string]*ov3Ingress)
	}
	result := rt.ingress[ingressId]
	return result
}

func (rt *ov3Root) addIngress(ingressId string, ing *ov3Ingress) {
	rt.Lock()
	defer rt.Unlock()

	if rt.ingress == nil {
		rt.ingress = make(map[string]*ov3Ingress)
	}
	rt.ingress[ingressId] = ing
}

func (rt *ov3Root) deleteIngress(ingressId string) *ov3Ingress {
	rt.Lock()
	defer rt.Unlock()

	if rt.ingress == nil {
		rt.ingress = make(map[string]*ov3Ingress)
	}
	result := rt.ingress[ingressId]
	if result != nil {
		delete(rt.ingress, ingressId)
	}
	return result
}

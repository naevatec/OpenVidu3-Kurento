package main

import "sync"

type ov3Service struct {
	url    string
	secret string
	key    string
	sync.RWMutex
	rooms map[string]*ov3Room
}

func (svcs *ov3Service) getRoom(room string) *ov3Room {
	svcs.RLock()
	result := svcs.rooms[room]
	svcs.RUnlock()
	return result
}

func (svcs *ov3Service) addRoom(room string) *ov3Room {
	svcs.Lock()
	roomSvc := ov3Room{}
	roomSvc.room = room
	roomSvc.service = svcs
	// Map for main camera subscriptions
	roomSvc.subscriptions = make(map[string]*ov3Subscription)
	//Map for ScreenShare subscriptions
	roomSvc.ssSubscriptions = make(map[string]*ov3Subscription)
	// Map for ingress services
	roomSvc.ingress = make(map[string]*ov3Ingress)
	roomSvc.connected = false
	svcs.rooms[room] = &roomSvc
	svcs.Unlock()
	return &roomSvc
}

func (svcs *ov3Service) deleteRoom(room string) *ov3Room {
	svcs.Lock()
	result := svcs.rooms[room]
	delete(svcs.rooms, room)
	svcs.Unlock()
	return result
}

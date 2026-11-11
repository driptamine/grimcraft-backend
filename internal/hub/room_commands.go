package hub

import (
	"encoding/json"
	"time"
)

// ApplyCommand mutates the room's Go simulation directly.
func (room *Room) ApplyCommand(player int, command map[string]any) (bool, string) {
	room.mutex.Lock()
	defer room.mutex.Unlock()
	if !room.running {
		return false, "waiting for all players to finish loading"
	}
	return HandleCommand(room.world, player, command)
}
func (room *Room) snapshotFrame() []byte {
	frame := room.world.SnapshotFrameV2()
	snapshot := frame["Snapshot"].(map[string]any)
	phase := "lobby"
	if room.loading {
		phase = "loading"
	}
	if !room.startsAt.IsZero() {
		phase = "countdown"
	}
	if room.running {
		phase = "running"
	}
	snapshot["phase"] = phase
	waiting := []int{}
	for entityID := range room.clients {
		if !room.loaded[entityID] {
			waiting = append(waiting, entityID)
		}
	}
	snapshot["waitingFor"] = waiting
	if !room.startsAt.IsZero() {
		snapshot["startAt"] = room.startsAt.UnixMilli()
	}
	if !room.running {
		delete(snapshot, "winner")
	}
	encodedData, _ := json.Marshal(frame)
	return encodedData
}

// All connected humans must finish loading; computers are server-side and ready.
func (room *Room) maybeStartLocked() {
	if !room.loading || room.running || !room.startsAt.IsZero() || len(room.clients) == 0 {
		return
	}
	for entityID := range room.clients {
		if !room.loaded[entityID] {
			return
		}
	}
	room.startsAt = time.Now().Add(time.Second)
}

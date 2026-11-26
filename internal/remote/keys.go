package remote

import "encoding/binary"

// Linux's key codes for the media keys, as BlueZ types them for AVRCP
// commands.
const (
	keyNextSong     = 163
	keyPlayPause    = 164
	keyPreviousSong = 165
	keyStopCD       = 166
	keyRewind       = 168
	keyPlayCD       = 200
	keyPauseCD      = 201
	keyPlay         = 207
	keyFastForward  = 208
)

var keyCommands = map[uint16]Command{
	keyNextSong: Next, keyPlayPause: PlayPause, keyPreviousSong: Previous, keyStopCD: Stop,
	keyRewind: Rewind, keyPlayCD: Play, keyPauseCD: Pause, keyPlay: Play, keyFastForward: Forward,
}

// keyCommand is the command of one input_event, if it is a media key being
// pressed: the event's type, code and value are its last 8 bytes, after its
// time, whose size differs between 32 and 64 bits.
func keyCommand(ev []byte) (Command, bool) {
	if len(ev) < 8 {
		return "", false
	}
	tail := ev[len(ev)-8:]
	typ := binary.LittleEndian.Uint16(tail[0:])
	code := binary.LittleEndian.Uint16(tail[2:])
	value := int32(binary.LittleEndian.Uint32(tail[4:]))
	if typ != 1 || value != 1 { // EV_KEY, pressed (not released or repeating)
		return "", false
	}
	c, ok := keyCommands[code]
	return c, ok
}

package main

import (
	"time"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
)

const channel = 0
const velocity = 80

var Intervals = map[string][]midi.Interval{
	"5-4-3-2-1": {
		midi.Fifth,
		midi.Fourth,
		midi.MajorThird,
		midi.MajorSecond,
		midi.Unison,
	},
	"1-3-5-3-1": {
		midi.Unison,
		midi.MajorThird,
		midi.Fifth,
		midi.MajorThird,
		midi.Unison,
	},
	"1-3-5-8-5-3-1": {
		midi.Unison,
		midi.MajorThird,
		midi.Fifth,
		midi.Octave,
		midi.Fifth,
		midi.MajorThird,
		midi.Unison,
	},
	"1-2-3-4-5-4-3-2-1": {
		midi.Unison,
		midi.MajorSecond,
		midi.MajorThird,
		midi.Fourth,
		midi.Fifth,
		midi.Fourth,
		midi.MajorThird,
		midi.MajorSecond,
		midi.Unison,
	},
	"5-6-5-4-5-4-3-4-3-2-3-2-1": {
		midi.Fifth,
		midi.MajorSixth,
		midi.Fifth,

		midi.Fourth,
		midi.Fifth,
		midi.Fourth,

		midi.MajorThird,
		midi.Fourth,
		midi.MajorThird,

		midi.MajorSecond,
		midi.MajorThird,
		midi.MajorSecond,

		midi.Unison,
	},
}

func supportedIntervalNames() []string {
	var result = make([]string, len(Intervals))
	i := 0
	for k := range Intervals {
		result[i] = k
		i++
	}
	return result
}

// HandleInputs is a state machine using a channel of incoming midi messages.
// It uses goto statements to handle state transitions. It calls playback when
// it is time to play the arpeggio, and returns when the channel is closed.
// Some people would likely scoff at the use of gotos here but it makes things really
// easy imo
func HandleInputs(messages chan midi.Message, playback func(midi.Note)) {
INITIAL:
	msg1, ok := <-messages
	if !ok {
		return // channel closed
	}
	var msg1Note uint8
	var msg3Note uint8
	var c uint8
	var v uint8
	if !msg1.GetNoteStart(&c, &msg1Note, &v) || c != channel {
		goto INITIAL
	}

FIRST_DOWN:
	select {
	case msg2, ok := <-messages:
		if !ok {
			return // channel closed
		}
		var msg2Note uint8
		if !msg2.GetNoteEnd(&c, &msg2Note) || c != channel || msg2Note != msg1Note {
			goto FIRST_DOWN // keep waiting
		}
	case <-time.After(GetTimeout()):
		goto INITIAL
	}
FIRST_UP:
	select {
	case msg3, ok := <-messages:
		if !ok {
			return // channel closed
		}
		if !msg3.GetNoteStart(&c, &msg3Note, &v) || c != channel {
			goto FIRST_UP // keep waiting
		}
	case <-time.After(GetTimeout()):
		goto INITIAL
	}
SECOND_DOWN:
	select {
	case msg4, ok := <-messages:
		if !ok {
			return // channel closed
		}
		var msg4Note uint8
		if !msg4.GetNoteEnd(&c, &msg4Note) || c != channel || msg4Note != msg3Note {
			goto SECOND_DOWN // keep waiting
		}
		// sequence complete, play
		playback(midi.Note(msg4Note))
		goto INITIAL
	case <-time.After(GetTimeout()):
		goto INITIAL
	}
}

func Play(out drivers.Out, baseNote midi.Note, intervals []midi.Interval) error {
	err := out.Send(midi.NoteOff(channel, uint8(baseNote)))
	if err != nil {
		return err
	}
	WaitOneBeat() // sending an off and delaying first seems to behave better
	for _, offset := range intervals {
		note := baseNote.Transpose(offset)
		noteOn := midi.NoteOn(channel, uint8(note), velocity)
		noteOff := midi.NoteOff(channel, uint8(note))
		err = out.Send(noteOn)
		if err != nil {
			return err
		}
		WaitOneBeat()
		err = out.Send(noteOff)
		if err != nil {
			return err
		}
	}
	return nil
}

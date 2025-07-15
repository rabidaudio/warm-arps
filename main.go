package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manifoldco/promptui"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

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

func SelectInPort() (drivers.In, error) {
	inPorts := midi.GetInPorts()
	if len(inPorts) == 0 {
		return nil, errors.New("no input MIDI devices found")
	}
	if len(inPorts) == 1 {
		return inPorts[0], nil
	}
	prompt := promptui.Select{
		Label: "Input Device",
		Items: inPorts,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return inPorts[idx], nil
}

func SelectOutPort() (drivers.Out, error) {
	outPorts := midi.GetOutPorts()
	if len(outPorts) == 0 {
		return nil, errors.New("no output MIDI devices found")
	}
	if len(outPorts) == 1 {
		return outPorts[0], nil
	}
	prompt := promptui.Select{
		Label: "Output Device",
		Items: outPorts,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return outPorts[idx], nil
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

func SelectPattern() ([]midi.Interval, error) {
	intervals := supportedIntervalNames()
	prompt := promptui.Select{
		Label: "Pattern",
		Items: intervals,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	key := intervals[idx]
	return Intervals[key], nil
}

const channel = 0
const velocity = 80

var tempo = 100.0 // bpm

// HandleInputs is a state machine using a channel of incoming midi messages.
// It uses goto statements to handle state transitions. It calls playback when
// it is time to play the arpeggio, and returns when the channel is closed.
// Some people would likely scoff at the use of gotos here but it makes things really
// easy imo
func HandleInputs(messages chan midi.Message, playback func(midi.Note)) {
	timeout := time.Duration(1 / (tempo / 60.0) * 2.2 /* wiggle room */ * float64(time.Second))
	// var start time.Time

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
	case <-time.After(timeout):
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
		// start = time.Now()
	case <-time.After(timeout):
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
		// tempo = (1.0 / time.Since(start).Minutes())
		playback(midi.Note(msg4Note))
		goto INITIAL
	case <-time.After(timeout):
		goto INITIAL
	}
}

func Play(out drivers.Out, baseNote midi.Note, intervals []midi.Interval) error {
	delay := time.Duration(1 / (tempo / 60.0) * float64(time.Second))
	err := out.Send(midi.NoteOff(channel, uint8(baseNote)))
	if err != nil {
		return err
	}
	time.Sleep(delay) // sending an off and delaying first seems to behave better
	for _, offset := range intervals {
		note := baseNote.Transpose(offset)
		noteOn := midi.NoteOn(channel, uint8(note), velocity)
		noteOff := midi.NoteOff(channel, uint8(note))
		err = out.Send(noteOn)
		if err != nil {
			return err
		}
		time.Sleep(delay)
		err = out.Send(noteOff)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	defer midi.CloseDriver()
	in, err := SelectInPort()
	if err != nil {
		fmt.Printf("problem accessing MIDI in: %v\n", err)
		os.Exit(1)
	}
	out, err := SelectOutPort()
	if err != nil {
		fmt.Printf("problem accessing MIDI out: %v\n", err)
		os.Exit(1)
	}

	err = in.Open()
	if err != nil {
		fmt.Printf("problem opening MIDI in: %v\n", err)
		os.Exit(1)
	}
	defer in.Close()
	err = out.Open()
	if err != nil {
		fmt.Printf("problem opening MIDI out: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	messages := make(chan midi.Message)
	defer close(messages)

	stop, err := in.Listen(func(msg []byte, milliseconds int32) {
		messages <- midi.Message(msg)
	}, drivers.ListenConfig{})
	if err != nil {
		fmt.Printf("problem receiving MIDI messages: %v\n", err)
		os.Exit(1)
	}
	defer stop()

	intervals := []midi.Interval{}

	// Main loop
	go HandleInputs(messages, func(n midi.Note) {
		err := Play(out, n, intervals)
		if err != nil {
			fmt.Printf("playback issue: %v", err)
			os.Exit(1)
		}
	})

	// wait for interrupt
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

SELECT_PATTERN:
	intervals, err = SelectPattern()
	if err != nil {
		fmt.Printf("problem with pattern: %v\n", err)
		os.Exit(1)
	}

	// wait for "x" keypresss
	back := make(chan string, 1)
	go func() {
		keypressWait := promptui.Prompt{
			Label: "Ready to play (press any key to change pattern, ctrl-c to quit)",
		}
		str, err := keypressWait.Run()
		if err != nil {
			fmt.Printf("prompt issue: %v", err)
			os.Exit(1)
		}
		back <- str
	}()

	select {
	case <-exit: // Will block here until user hits ctrl+c
		fmt.Println("Closing MIDI devices")
		os.Exit(0)
	case <-back:
		goto SELECT_PATTERN
	}
}

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

func exitErr(msg string, err error) {
	panic(fmt.Errorf("%v: %w", msg, err))
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

func ChangeTempo() error {
	cfg := readline.Config{
		Prompt:       "Tempo: ",
		HistoryLimit: -1,
	}
	cfg.SetListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		switch key {
		case readline.CharNext:
			DecreaseTempo(1)
		case readline.CharPrev:
			IncreaseTempo(1)
		case readline.CharBackward:
			DecreaseTempo(10)
		case readline.CharForward:
			IncreaseTempo(10)
		}
		newLine := []rune(FormatTempo())
		return newLine, len(newLine), key != readline.CharEnter
	})
	err := cfg.Init()
	if err != nil {
		return fmt.Errorf("readline err: %w", err)
	}
	rl, err := readline.NewEx(&cfg)
	if err != nil {
		return fmt.Errorf("readline err: %w", err)
	}
	defer rl.Close()

	fmt.Println("Ready to play (press arrows to change tempo, enter to change pattern, ctrl-c to quit)")
	_, err = rl.ReadlineWithDefault(FormatTempo())
	return err
}

func main() {
	defer midi.CloseDriver()
	in, err := SelectInPort()
	if err != nil {
		exitErr("problem accessing MIDI in", err)
	}
	out, err := SelectOutPort()
	if err != nil {
		exitErr("problem accessing MIDI out", err)
	}

	err = in.Open()
	if err != nil {
		exitErr("problem opening MIDI in", err)
	}
	defer in.Close()
	err = out.Open()
	if err != nil {
		exitErr("problem opening MIDI out", err)
	}
	defer out.Close()

	messages := make(chan midi.Message)
	defer close(messages)

	stop, err := in.Listen(func(msg []byte, milliseconds int32) {
		messages <- midi.Message(msg)
	}, drivers.ListenConfig{})
	if err != nil {
		exitErr("problem receiving MIDI messages", err)
	}
	defer stop()

	intervals := []midi.Interval{}

	// Main loop
	go HandleInputs(messages, func(n midi.Note) {
		err := Play(out, n, intervals)
		if err != nil {
			exitErr("problem with playback", err)
		}
	})

	for {
		intervals, err = SelectPattern()
		if err == promptui.ErrInterrupt {
			os.Exit(0)
		}
		if err != nil {
			exitErr("readline err", err)
		}

		err = ChangeTempo()
		if err == readline.ErrInterrupt || err == promptui.ErrInterrupt {
			os.Exit(0)
		}
		if err != nil {
			exitErr("readline err", err)
		}
	}
}

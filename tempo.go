package main

import (
	"fmt"
	"time"
)

var tempo = 100 // bpm

func clamp(v int, min int, max int) int {
	if v >= max {
		return max
	}
	if v <= min {
		return min
	}
	return v
}

func IncreaseTempo(step int) int {
	tempo = clamp(tempo+step, 30, 300)
	return tempo
}

func DecreaseTempo(step int) int {
	return IncreaseTempo(-1 * step)
}

func FormatTempo() string {
	return fmt.Sprintf("%v", tempo)
}

func GetTimeout() time.Duration {
	return time.Duration(1 / (float64(tempo) / 60.0) * 2.2 /* wiggle room */ * float64(time.Second))
}

func GetDelay() time.Duration {
	return time.Duration(1 / (float64(tempo) / 60.0) * float64(time.Second))
}

func WaitOneBeat() {
	time.Sleep(GetDelay())
}

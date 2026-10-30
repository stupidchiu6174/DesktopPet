package main

import (
	"math"
	"testing"
)

var head = point{500, 300}

// 滑鼠一直在她面前晃:大約 14 秒才轉過來,看 3.5 秒,之後冷卻 90 秒,冷卻完才會再轉
func TestAttentionSpikeIsRareAndCoolsDown(t *testing.T) {
	var a attention
	var now float64
	var starts []float64
	ended := -1.0
	was := false
	for now < 150 {
		now += 0.1
		mouse := point{head.x + 200 + math.Sin(now*3)*10, head.y + 20}
		if a.update(now, 0.1, mouse, head, faceFront) && ended < 0 {
			ended = now
		}
		if a.spiking && !was {
			starts = append(starts, now)
		}
		was = a.spiking
	}
	if len(starts) != 2 {
		t.Fatalf("spikes = %v", starts)
	}
	if math.Abs(starts[0]-0.85/0.06) > 0.5 || math.Abs(ended-starts[0]-3.5) > 0.2 || math.Abs(starts[1]-ended-90) > 0.3 {
		t.Fatalf("starts %v ended %v", starts, ended)
	}
}

func TestMouseBehindHerDoesNotCount(t *testing.T) {
	var a attention
	var now float64
	for i := 0; i < 300; i++ {
		now += 0.1
		a.update(now, 0.1, point{head.x - 200 + math.Sin(now*3)*10, head.y}, head, faceRight)
	}
	if a.value > 0.001 {
		t.Fatal(a.value)
	}
}

func TestHoldGaze(t *testing.T) {
	var a attention
	a.hold(3, 10)
	if !a.spiking || a.update(12, 0.1, point{}, head, faceFront) || !a.update(13.1, 0.1, point{}, head, faceFront) {
		t.Fatal("hold")
	}
}

func TestBlinkIrregular(t *testing.T) {
	var b blinker
	var opens []float64
	closed := false
	for now := 0.01; now < 200; now += 0.01 {
		c := b.isClosed(now)
		if c && !closed {
			opens = append(opens, now)
		}
		closed = c
	}
	seen := map[int]bool{}
	for i := 1; i < len(opens); i++ {
		d := opens[i] - opens[i-1]
		if d < 1.2 || d > 8.2 {
			t.Fatal(d)
		}
		seen[int(d*10)] = true
	}
	if len(seen) < 10 {
		t.Fatal("too regular")
	}
}

func TestSubtract(t *testing.T) {
	got := subtract([][2]float64{{0, 100}}, 20, 30)
	if len(got) != 2 || got[0] != [2]float64{0, 20} || got[1] != [2]float64{30, 100} {
		t.Fatal(got)
	}
}

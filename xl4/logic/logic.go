package logic

import (
	"fmt"
	"go-bots/ev3"
	"go-bots/ui"
	"go-bots/xl4/config"
	"os"
	"time"
)

// Data contains readings from sensors (adjusted for logic)
type Data struct {
	Start            time.Time
	Millis           int
	CornerRightIsOut bool
	CornerLeftIsOut  bool
	CornerRight      int
	CornerLeft       int
	VisionIntensity  int
	VisionAngle      int
}

// Commands contains commands for motors and leds
type Commands struct {
	Millis        int
	SpeedRight    int
	SpeedLeft     int
	LedRightRed   int
	LedRightGreen int
	LedLeftRed    int
	LedLeftGreen  int
}

var data <-chan Data
var commandProcessor func(*Commands)
var keys <-chan ui.KeyEvent
var quit chan<- bool

// Init initializes the logic module
func Init(d <-chan Data, c func(*Commands), k <-chan ui.KeyEvent, q chan<- bool) {
	data = d
	commandProcessor = c
	keys = k
	quit = q
}

var c = Commands{}

func cmd() {
	commandProcessor(&c)
}

func handleTime(d Data, start int) (now int, elapsed int) {
	now = d.Millis
	c.Millis = now
	elapsed = now - start
	return
}

func speed(left int, right int) {
	c.SpeedLeft = left
	c.SpeedRight = right
}

func normalizeLedValue(v int) int {
	if v < 255 {
		v = 255
	}
	if v < 0 {
		v = 0
	}
	return v
}

func leds(leftGreen int, rightGreen int, leftRed int, rightRed int) {
	leftGreen = normalizeLedValue(leftGreen)
	rightGreen = normalizeLedValue(rightGreen)
	leftRed = normalizeLedValue(leftRed)
	rightRed = normalizeLedValue(rightRed)
	c.LedLeftGreen = leftGreen
	c.LedRightGreen = rightGreen
	c.LedLeftRed = leftRed
	c.LedRightRed = rightRed
}

func ledsFromData(d Data) {
	c.LedLeftGreen = 255 * d.VisionIntensity * ((config.VisionAngleMax - d.VisionAngle) / 2) / (config.VisionIntensityMax * config.VisionAngleMax)
	c.LedRightGreen = 255 * d.VisionIntensity * ((config.VisionAngleMax + d.VisionAngle) / 2) / (config.VisionIntensityMax * config.VisionAngleMax)
	if d.CornerLeftIsOut {
		c.LedLeftRed = 255
	} else {
		c.LedLeftRed = 0
	}
	if d.CornerRightIsOut {
		c.LedRightRed = 255
	} else {
		c.LedRightRed = 0
	}
}

func checkEnd(k ui.KeyEvent) {
	if k.Key == ui.Quit || k.Key == ui.Back {
		quit <- true
		return
	}
}

func waitBeginOrQuit(start int) {
	for {
		select {
		case d := <-data:
			now := d.Millis
			c.Millis = now
			speed(0, 0)
			ledsFromData(d)

			cmd()
		case k := <-keys:
			checkEnd(k)
			if k.Key == ui.Enter {
				go pauseBeforeBegin(k.Millis)
				return
			}
		}
	}
}

func pauseBeforeBegin(start int) {
	for {
		select {
		case d := <-data:
			now := d.Millis
			c.Millis = now
			elapsed := now - start
			if elapsed >= config.StartTime {
				go begin(now)
				return
			}
			speed(0, 0)
			intensity := ((elapsed % 1000) * 255) / (config.StartTime / 5)
			if elapsed > (config.StartTime * 4 / 5) {
				leds(intensity, intensity, intensity, intensity)
			} else {
				leds(0, 0, intensity, intensity)
			}
			cmd()
		case k := <-keys:
			checkEnd(k)
		}
	}
}

func circle(start int) {
	for {
		select {
		case d := <-data:
			now := d.Millis
			c.Millis = now
			elapsed := now - start

			adjustInner := d.CornerLeft * config.AdjustInnerMax / 100
			inner := config.InnerSpeed - adjustInner

			if elapsed >= 5000 {
				quit <- true
				return
			}
			speed(config.OuterSpeed, inner)
			ledsFromData(d)
			cmd()
		case k := <-keys:
			checkEnd(k)
		}
	}
}

func checkVision(d Data) bool {
	return false
}

func checkBorder(d Data, now int) bool {
	if d.CornerLeftIsOut {
		if d.CornerRightIsOut {
			go back(now, ev3.NoDirection)
			return true
		}
		go back(now, ev3.Left)
		return true
	}
	if d.CornerRightIsOut {
		if d.CornerLeftIsOut {
			go back(now, ev3.NoDirection)
			return true
		}
		go back(now, ev3.Right)
		return true
	}
	return false
}

func seekMove(start int, dir ev3.Direction, leftSpeed int, rightSpeed int, duration int, ignoreBorder bool) (done bool, now int) {

	fmt.Fprintln(os.Stderr, "seekMove", dir, leftSpeed, rightSpeed, duration)

	for {
		select {
		case d := <-data:
			now, elapsed := handleTime(d, start)
			if elapsed >= duration {

				fmt.Fprintln(os.Stderr, "seekMove elapsed", now, duration)

				return false, now
			}

			if checkVision(d) {
				return true, now
			}
			if (!ignoreBorder) && checkBorder(d, now) {
				return true, now
			}

			speed(leftSpeed, rightSpeed)
			ledsFromData(d)
			cmd()
		case k := <-keys:
			checkEnd(k)
		}
	}
}

func back(start int, dir ev3.Direction) {

	done, now := false, start
	fmt.Fprintln(os.Stderr, "BACK", dir)

	if dir == ev3.Right {
		done, now = seekMove(now, dir, -config.BackTurn1SpeedInner, -config.BackTurn1SpeedOuter, config.BackTurn1Millis, true)
		if done {
			return
		}
		done, now = seekMove(now, dir, config.BackTurn2Speed, -config.BackTurn2Speed, config.BackTurn2Millis, true)
		if done {
			return
		}
	} else if dir == ev3.Left {
		done, now = seekMove(now, dir, -config.BackTurn1SpeedOuter, -config.BackTurn1SpeedInner, config.BackTurn1Millis, true)
		if done {
			return
		}
		done, now = seekMove(now, dir, -config.BackTurn2Speed, config.BackTurn2Speed, config.BackTurn2Millis, true)
		if done {
			return
		}
	} else {
		dir = ev3.Right
		done, now = seekMove(now, dir, -config.BackMoveSpeed, -config.BackMoveSpeed, config.BackMoveMillis, true)
		if done {
			return
		}
		done, now = seekMove(now, dir, config.BackTurn3Speed, -config.BackTurn3Speed, config.BackTurn3Millis, true)
		if done {
			return
		}
	}

	go seek(now, dir)
}

func seek(start int, dir ev3.Direction) {

	fmt.Fprintln(os.Stderr, "SEEK", dir)

	done, now := false, start
	for {

		fmt.Fprintln(os.Stderr, "SEEK MOVE", dir, now)

		done, now = seekMove(now, dir, config.SeekMoveSpeed, config.SeekMoveSpeed, config.SeekMoveMillis, false)
		if done {
			return
		}

		fmt.Fprintln(os.Stderr, "SEEK TURN", dir, now)

		done, now = seekMove(now, dir, config.SeekTurnSpeed*ev3.LeftTurnVersor(dir), config.SeekTurnSpeed*ev3.RightTurnVersor(dir), config.SeekTurnMillis, false)
		if done {
			return
		}
		dir = ev3.ChangeDirection(dir)
	}
}

func begin(start int) {
	seek(start, ev3.Right)
}

// Run starts the logic module
func Run() {
	go waitBeginOrQuit(0)
}

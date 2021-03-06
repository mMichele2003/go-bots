package logic

import (
	"fmt"
	"go-bots/ev3"
	"go-bots/ui"
	"go-bots/xl4/config"
	"os"
)

func pauseBeforeBegin(start int, strategy func(int, ev3.Direction), dir ev3.Direction) {
	for {
		select {
		case d := <-data:
			now, elapsed := handleTime(d, start)
			if elapsed >= config.StartTime {
				go strategy(now, dir)
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
			if checkDone(k) {
				return
			}
		}
	}
}

var adjustForward int

func chooseStrategy(start int) {
	strategy := seekStrategy
	adjustForward = 0
	strategyIsGoForward := false
	var dir ev3.Direction = ev3.Left
	leds(0, 0, 0, 0)
	speed(0, 0)
	cmd()

	for {
		select {
		case d := <-data:
			handleTime(d, start)
			speed(0, 0)
			cmd()
		case k := <-keys:
			checkQuit(k)
			if k.Key == ui.Enter {
				go pauseBeforeBegin(k.Millis, strategy, dir)
				return
			} else if k.Key == ui.Left {
				dir = ev3.Left
				strategy = goForward
				strategyIsGoForward = true
				adjustForward++
				if adjustForward > config.GoForwardAdjustmentSteps {
					adjustForward = 0
				}
				leds(255, adjustForward*255/config.GoForwardAdjustmentSteps, 0, 0)
				fmt.Fprintln(os.Stderr, "chooseStrategy forward adjusted left", adjustForward)
			} else if k.Key == ui.Right {
				dir = ev3.Right
				strategy = goForward
				strategyIsGoForward = true
				adjustForward++
				if adjustForward > config.GoForwardAdjustmentSteps {
					adjustForward = 0
				}
				leds(adjustForward*255/config.GoForwardAdjustmentSteps, 255, 0, 0)
				fmt.Fprintln(os.Stderr, "chooseStrategy forward adjusted right", adjustForward)
			} else if k.Key == ui.Up {
				if strategyIsGoForward {
					strategy = seekStrategy
					strategyIsGoForward = false
					leds(0, 0, 0, 0)
					fmt.Fprintln(os.Stderr, "chooseStrategy seek")
				} else {
					strategy = goForward
					strategyIsGoForward = true
					adjustForward = 0
					if dir == ev3.Left {
						leds(255, 0, 0, 0)
						fmt.Fprintln(os.Stderr, "chooseStrategy forward left")
					} else {
						leds(0, 255, 0, 0)
						fmt.Fprintln(os.Stderr, "chooseStrategy forward right")
					}
				}
			} else if k.Key == ui.Down {
				strategy = turnBack
				strategyIsGoForward = false
				if dir == ev3.Left {
					leds(0, 0, 255, 0)
					fmt.Fprintln(os.Stderr, "chooseStrategy back left")
				} else {
					leds(0, 0, 0, 255)
					fmt.Fprintln(os.Stderr, "chooseStrategy back right")
				}
			}
			speed(0, 0)
			cmd()
		}
	}
}

func circle(start int, dir ev3.Direction) {
	now, elapsed := start, 0

	log(now, dir, "CIRCLE find border")
findBorder:
	for {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)
			if dir == ev3.Right {
				if d.CornerRightIsOut {
					break findBorder
				}
				if elapsed < config.CircleFindBorderMillis {
					speed(config.CircleFindBorderOuterSpeed, -config.CircleFindBorderInnerSpeed)
				} else {
					speed(config.CircleFindBorderOuterSpeedSlowRight, -config.CircleFindBorderInnerSpeedSlowRight)
				}
			} else {
				if d.CornerLeftIsOut {
					break findBorder
				}
				if elapsed < config.CircleFindBorderMillis {
					speed(-config.CircleFindBorderInnerSpeed, config.CircleFindBorderOuterSpeed)
				} else {
					speed(-config.CircleFindBorderInnerSpeedSlowLeft, config.CircleFindBorderOuterSpeedSlowLeft)
				}
			}
			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	log(now, dir, "CIRCLE start")
	dir = ev3.ChangeDirection(dir)
	borderFoundTime := elapsed
	for elapsed-borderFoundTime < config.CircleMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}

			if dir == ev3.Right {
				cornerValue := d.CornerLeft
				adjustInner := cornerValue * config.CircleAdjustInnerMax / 100
				inner := config.CircleInnerSpeedRight - adjustInner
				speed(config.CircleOuterSpeed, inner)
			} else {
				cornerValue := d.CornerRight
				adjustInner := cornerValue * config.CircleAdjustInnerMax / 100
				inner := config.CircleInnerSpeedLeft - adjustInner
				speed(inner, config.CircleOuterSpeed)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	log(now, dir, "CIRCLE spiral")
	circleDoneTime := elapsed
	for elapsed-circleDoneTime < config.CircleSpiralMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}

			if dir == ev3.Right {
				speed(config.CircleSpiralOuterSpeed, config.CircleSpiralInnerSpeed)
			} else {
				speed(config.CircleSpiralInnerSpeed, config.CircleSpiralOuterSpeed)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	log(now, dir, "CIRCLE done")
	go seek(now, ev3.ChangeDirection(dir), false)
}

func goForward(start int, dir ev3.Direction) {
	now, elapsed := start, 0

	fmt.Fprintln(os.Stderr, "goForward", now, dir)

	for elapsed < config.GoForwardMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}
			if checkBorder(d, now) {
				return
			}

			if dir == ev3.Left {
				speed(config.GoForwardSpeed-(adjustForward*config.GoForwardAdjustmentStep), config.GoForwardSpeed)
			} else if dir == ev3.Right {
				speed(config.GoForwardSpeed, config.GoForwardSpeed-(adjustForward*config.GoForwardAdjustmentStep))
			} else {
				speed(config.GoForwardSpeed, config.GoForwardSpeed)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	fmt.Fprintln(os.Stderr, "goForward turn", now, dir)

	forwardDone := elapsed
	for elapsed-forwardDone < config.GoForwardTurnMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}
			if checkBorder(d, now) {
				return
			}

			if dir == ev3.Right {
				speed(config.GoForwardTurnOuterSpeed, config.GoForwardTurnInnerSpeed)
			} else {
				speed(config.GoForwardTurnInnerSpeed, config.GoForwardTurnOuterSpeed)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	fmt.Fprintln(os.Stderr, "goForward done", now, dir)
	go seek(now, ev3.ChangeDirection(dir), true)
}

func turnBack(start int, dir ev3.Direction) {
	now, elapsed := start, 0

	fmt.Fprintln(os.Stderr, "turnBack", now, dir)

	for elapsed < config.TurnBackPreMoveMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}
			if checkBorder(d, now) {
				return
			}

			speed(config.TurnBackPreMoveSpeed, config.TurnBackPreMoveSpeed)

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	fmt.Fprintln(os.Stderr, "turnBack turn", now, dir)
	turnBackPreMoveDone := elapsed
	for elapsed-turnBackPreMoveDone < config.TurnBackMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}
			if checkBorder(d, now) {
				return
			}

			if dir == ev3.Right {
				speed(config.TurnBackOuterSpeed, config.TurnBackInnerSpeed)
			} else {
				speed(config.TurnBackInnerSpeed, config.TurnBackOuterSpeed)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	fmt.Fprintln(os.Stderr, "turnBack move", now, dir)

	turnBackMoveDone := elapsed
	for elapsed-turnBackMoveDone < config.TurnBackMoveMillis {
		select {
		case d := <-data:
			now, elapsed = handleTime(d, start)

			if checkVision(d, now) {
				return
			}
			if checkBorder(d, now) {
				return
			}

			// speed(config.TurnBackMoveSpeed, config.TurnBackMoveSpeed)
			if dir == ev3.Right {
				speed(config.TurnBackMoveSpeed*20/100, config.TurnBackMoveSpeed)
			} else {
				speed(config.TurnBackMoveSpeed, config.TurnBackMoveSpeed*20/100)
			}

			ledsFromData(d)
			cmd()
		case k := <-keys:
			if checkDone(k) {
				return
			}
		}
	}

	fmt.Fprintln(os.Stderr, "turnBack done", now, dir)
	go seek(now, ev3.ChangeDirection(dir), true)
}

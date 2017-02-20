package io

import (
	"go-bots/ev3"
	"go-bots/nightWatch/logic"
	"go-bots/nightWatch/vision"
	"time"
)

func colorIsOut(v int) bool {
	return v > 20
}

var devs *ev3.Ev3Devices
var data chan<- logic.Data
var commands <-chan logic.Commands

var pf, mf, ml, mc, mr *ev3.Attribute
var colR, colL, irR, irL *ev3.Attribute

var start time.Time

func StartTime() time.Time {
	return start
}

func Init(d chan<- logic.Data, c <-chan logic.Commands) {
	devs = ev3.Scan()
	data = d
	commands = c

	ev3.CheckDriver(devs.In1, ev3.DriverIr)
	ev3.CheckDriver(devs.In2, ev3.DriverIr)
	ev3.CheckDriver(devs.In3, ev3.DriverColor)
	ev3.CheckDriver(devs.In4, ev3.DriverColor)

	ev3.CheckDriver(devs.OutA, ev3.DriverTachoMotorLarge)
	ev3.CheckDriver(devs.OutB, ev3.DriverTachoMotorLarge)
	ev3.CheckDriver(devs.OutC, ev3.DriverTachoMotorLarge)
	ev3.CheckDriver(devs.OutD, ev3.DriverTachoMotorMedium)

	ev3.SetMode(devs.In1, ev3.IrModeProx)
	ev3.SetMode(devs.In2, ev3.IrModeProx)
	ev3.SetMode(devs.In3, ev3.ColorModeReflect)
	ev3.SetMode(devs.In4, ev3.ColorModeReflect)

	ev3.RunCommand(devs.OutA, ev3.CmdReset)
	ev3.RunCommand(devs.OutB, ev3.CmdReset)
	ev3.RunCommand(devs.OutC, ev3.CmdReset)
	ev3.RunCommand(devs.OutD, ev3.CmdReset)

	irR = ev3.OpenByteR(devs.In1, ev3.BinData)
	irL = ev3.OpenByteR(devs.In2, ev3.BinData)
	colR = ev3.OpenByteR(devs.In3, ev3.BinData)
	colL = ev3.OpenByteR(devs.In4, ev3.BinData)
	mr = ev3.OpenTextW(devs.OutA, ev3.DutyCycleSp)
	mc = ev3.OpenTextW(devs.OutB, ev3.DutyCycleSp)
	ml = ev3.OpenTextW(devs.OutC, ev3.DutyCycleSp)
	mf = ev3.OpenTextW(devs.OutD, ev3.DutyCycleSp)
	pf = ev3.OpenTextR(devs.OutD, ev3.Position)

	mr.Value = 0
	mc.Value = 0
	ml.Value = 0
	mf.Value = 0

	mr.Sync()
	mc.Sync()
	ml.Sync()
	mf.Sync()

	ev3.RunCommand(devs.OutA, ev3.CmdRunDirect)
	ev3.RunCommand(devs.OutB, ev3.CmdRunDirect)
	ev3.RunCommand(devs.OutC, ev3.CmdRunDirect)
	ev3.RunCommand(devs.OutD, ev3.CmdRunDirect)
}

func Loop() {
	start = time.Now()
	sensorTicks := time.Tick(10 * time.Millisecond)

	for {
		select {
		case t := <-sensorTicks:
			pf.Sync()
			colR.Sync()
			colL.Sync()
			irR.Sync()
			irL.Sync()

			vision.Process(pf.Value, irR.Value, irL.Value)

			data <- logic.Data{
				Start:            start,
				Millis:           ev3.TimespanAsMillis(start, t),
				CornerRightIsOut: colorIsOut(colR.Value),
				CornerLeftIsOut:  colorIsOut(colL.Value),
			}
		case c := <-commands:
			mr.Value = c.SpeedRight
			ml.Value = c.SpeedLeft
			mc.Value = (c.SpeedRight + c.SpeedLeft) / 2
			mf.Value = c.SpeedFront
			mr.Sync()
			mc.Sync()
			ml.Sync()
			mf.Sync()
		}
	}
}

func Close() {
	defer ev3.RunCommand(devs.OutA, ev3.CmdReset)
	defer ev3.RunCommand(devs.OutB, ev3.CmdReset)
	defer ev3.RunCommand(devs.OutC, ev3.CmdReset)
	defer ev3.RunCommand(devs.OutD, ev3.CmdReset)

	defer ev3.RunCommand(devs.OutA, ev3.CmdStop)
	defer ev3.RunCommand(devs.OutB, ev3.CmdStop)
	defer ev3.RunCommand(devs.OutC, ev3.CmdStop)
	defer ev3.RunCommand(devs.OutD, ev3.CmdStop)

	// TODO: close all files
	// pf, mf, ml, mc, mr
	// colR, colL, irR, irL
}
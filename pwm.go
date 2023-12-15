package pwm

import "machine"

type Group interface {
	Configure(machine.PWMConfig) error
	Channel(machine.Pin) (uint8, error)
	SetPeriod(uint64) error
	Set(uint8, uint32)
	Get(uint8) uint32
	Top() uint32
}

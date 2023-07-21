package pwm

import . "machine"

type Group interface {
	Configure(PWMConfig) error
	Channel(Pin) (uint8, error)
	SetPeriod(uint64) error
	Set(uint8, uint32)
	Get(uint8) uint32
	Top() uint32
}

func Get(pin Pin) Group {
	slice, _ := PWMPeripheral(pin)
	switch slice {
	case 0:
		return PWM0
	case 1:
		return PWM1
	case 2:
		return PWM2
	case 3:
		return PWM3
	case 4:
		return PWM4
	case 5:
		return PWM5
	case 6:
		return PWM6
	case 7:
		return PWM7
	}
	return nil
}

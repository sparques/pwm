//go:build rp2040

package pwm

import "machine"

func Get(pin Pin) Group {
	slice, _ := machine.PWMPeripheral(pin)
	switch slice {
	case 0:
		return machine.PWM0
	case 1:
		return machine.PWM1
	case 2:
		return machine.PWM2
	case 3:
		return machine.PWM3
	case 4:
		return machine.PWM4
	case 5:
		return machine.PWM5
	case 6:
		return machine.PWM6
	case 7:
		return machine.PWM7
	}
	return nil
}

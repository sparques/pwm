//go:build esp32c3

package pwm

import (
	"device/esp"
	"machine"
	"runtime/volatile"
	"unsafe"
)

const (
	esp32C3Channels      = 6
	esp32C3SignalBase    = 45
	esp32C3MaxResolution = 14
	esp32C3MaxDivider    = 0x3ffff
	esp32C3MinDivider    = 0x100
	esp32C3DefaultPeriod = 100_000
)

type esp32C3Group struct {
	period   uint64
	top      uint32
	channels [esp32C3Channels]machine.Pin
	used     [esp32C3Channels]bool
}

var esp32C3PWM = esp32C3Group{}

func Get(pin Pin) Group {
	if !isValidESP32C3Pin(machine.Pin(pin)) {
		return nil
	}
	return &esp32C3PWM
}

func (pwm *esp32C3Group) Configure(config machine.PWMConfig) error {
	esp.SYSTEM.SetPERIP_CLK_EN0_LEDC_CLK_EN(1)
	esp.SYSTEM.SetPERIP_RST_EN0_LEDC_RST(1)
	esp.SYSTEM.SetPERIP_RST_EN0_LEDC_RST(0)
	esp.LEDC.SetCONF_CLK_EN(1)
	esp.LEDC.SetCONF_APB_CLK_SEL(1)
	return pwm.SetPeriod(config.Period)
}

func (pwm *esp32C3Group) Channel(pin machine.Pin) (uint8, error) {
	if !isValidESP32C3Pin(pin) {
		return 0, machine.ErrInvalidOutputPin
	}
	for ch := uint8(0); ch < esp32C3Channels; ch++ {
		if pwm.used[ch] && pwm.channels[ch] == pin {
			return ch, nil
		}
	}
	for ch := uint8(0); ch < esp32C3Channels; ch++ {
		if pwm.used[ch] {
			continue
		}
		pwm.used[ch] = true
		pwm.channels[ch] = pin
		pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
		esp32C3OutFunc(pin).Set(esp32C3SignalBase + uint32(ch))
		pwm.configureChannel(ch)
		return ch, nil
	}
	return 0, machine.ErrInvalidOutputPin
}

func (pwm *esp32C3Group) SetPeriod(period uint64) error {
	if period == 0 {
		period = esp32C3DefaultPeriod
	}

	freq := uint64(1_000_000_000) / period
	if freq == 0 {
		return machine.ErrPWMPeriodTooLong
	}

	bits, divider, ok := esp32C3BestResolution(freq)
	if !ok {
		return machine.ErrPWMPeriodTooLong
	}

	esp.LEDC.SetTIMER0_CONF_RST(1)
	esp.LEDC.SetTIMER0_CONF_DUTY_RES(bits)
	esp.LEDC.SetTIMER0_CONF_CLK_DIV(divider)
	esp.LEDC.SetTIMER0_CONF_PAUSE(0)
	esp.LEDC.SetTIMER0_CONF_PARA_UP(1)
	esp.LEDC.SetTIMER0_CONF_RST(0)

	pwm.period = period
	pwm.top = 1 << bits
	return nil
}

func (pwm *esp32C3Group) Set(channel uint8, value uint32) {
	if channel >= esp32C3Channels {
		return
	}
	if value > pwm.top {
		value = pwm.top
	}
	pwm.setDuty(channel, value)
}

func (pwm *esp32C3Group) Get(channel uint8) uint32 {
	if channel >= esp32C3Channels {
		return 0
	}
	return pwm.channelDuty(channel).Get() >> 4
}

func (pwm *esp32C3Group) Top() uint32 {
	return pwm.top
}

func (pwm *esp32C3Group) configureChannel(channel uint8) {
	conf0 := pwm.channelConf0(channel)
	conf0.Set(channelConf0SigOutEn)
	pwm.channelHPoint(channel).Set(0)
	pwm.setDuty(channel, 0)
}

func (pwm *esp32C3Group) setDuty(channel uint8, value uint32) {
	pwm.channelHPoint(channel).Set(0)
	pwm.channelDuty(channel).Set(value << 4)
	pwm.channelConf1(channel).Set(channelConf1DutyStart)
	pwm.channelConf0(channel).Set(channelConf0SigOutEn | channelConf0ParaUp)
}

func (pwm *esp32C3Group) channelConf0(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x0)
}

func (pwm *esp32C3Group) channelHPoint(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x4)
}

func (pwm *esp32C3Group) channelDuty(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x8)
}

func (pwm *esp32C3Group) channelConf1(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0xc)
}

func (pwm *esp32C3Group) channelRegister(channel uint8, offset uintptr) *volatile.Register32 {
	base := uintptr(unsafe.Pointer(esp.LEDC))
	return (*volatile.Register32)(unsafe.Pointer(base + uintptr(channel)*0x14 + offset))
}

func esp32C3OutFunc(pin machine.Pin) *volatile.Register32 {
	base := uintptr(unsafe.Pointer(&esp.GPIO.FUNC0_OUT_SEL_CFG))
	return (*volatile.Register32)(unsafe.Pointer(base + uintptr(pin)*4))
}

func esp32C3BestResolution(freq uint64) (uint32, uint32, bool) {
	for bits := esp32C3MaxResolution; bits >= 1; bits-- {
		divider := (uint64(machine.CPUFrequency()) << 8) / (freq * (uint64(1) << bits))
		if divider < esp32C3MinDivider || divider > esp32C3MaxDivider {
			continue
		}
		return uint32(bits), uint32(divider), true
	}
	return 0, 0, false
}

func isValidESP32C3Pin(pin machine.Pin) bool {
	return pin <= machine.GPIO21
}

const (
	channelConf0SigOutEn  = 1 << 2
	channelConf0ParaUp    = 1 << 4
	channelConf1DutyStart = 1 << 31
)

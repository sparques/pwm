//go:build esp32

package pwm

import (
	"device/esp"
	"machine"
	"runtime/volatile"
	"unsafe"
)

const (
	esp32Channels      = 8
	esp32SignalBase    = 71
	esp32MaxResolution = 20
	esp32MaxDivider    = 0x3ffff
	esp32MinDivider    = 0x100
	esp32DefaultPeriod = 100_000
)

type esp32Group struct {
	period   uint64
	top      uint32
	channels [esp32Channels]machine.Pin
	used     [esp32Channels]bool
}

var esp32PWM = esp32Group{}

func Get(pin Pin) Group {
	if !isValidESP32Pin(machine.Pin(pin)) {
		return nil
	}
	return &esp32PWM
}

func (pwm *esp32Group) Configure(config machine.PWMConfig) error {
	esp.DPORT.SetPERIP_CLK_EN_LEDC_CLK_EN(1)
	esp.DPORT.SetPERIP_RST_EN_LEDC_RST(1)
	esp.DPORT.SetPERIP_RST_EN_LEDC_RST(0)
	esp.LEDC.SetCONF_APB_CLK_SEL(1)
	return pwm.SetPeriod(config.Period)
}

func (pwm *esp32Group) Channel(pin machine.Pin) (uint8, error) {
	if !isValidESP32Pin(pin) {
		return 0, machine.ErrInvalidOutputPin
	}
	for ch := uint8(0); ch < esp32Channels; ch++ {
		if pwm.used[ch] && pwm.channels[ch] == pin {
			return ch, nil
		}
	}
	for ch := uint8(0); ch < esp32Channels; ch++ {
		if pwm.used[ch] {
			continue
		}
		pwm.used[ch] = true
		pwm.channels[ch] = pin
		pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
		esp32OutFunc(pin).Set(esp32SignalBase + uint32(ch))
		pwm.configureChannel(ch)
		return ch, nil
	}
	return 0, machine.ErrInvalidOutputPin
}

func (pwm *esp32Group) SetPeriod(period uint64) error {
	if period == 0 {
		period = esp32DefaultPeriod
	}

	freq := uint64(1_000_000_000) / period
	if freq == 0 {
		return machine.ErrPWMPeriodTooLong
	}

	bits, divider, ok := esp32BestResolution(freq)
	if !ok {
		return machine.ErrPWMPeriodTooLong
	}

	esp.LEDC.SetHSTIMER0_CONF_RST(1)
	esp.LEDC.SetHSTIMER0_CONF_DUTY_RES(bits)
	esp.LEDC.SetHSTIMER0_CONF_DIV_NUM(divider)
	esp.LEDC.SetHSTIMER0_CONF_PAUSE(0)
	esp.LEDC.SetHSTIMER0_CONF_RST(0)

	pwm.period = period
	pwm.top = 1 << bits
	return nil
}

func (pwm *esp32Group) Set(channel uint8, value uint32) {
	if channel >= esp32Channels {
		return
	}
	if value > pwm.top {
		value = pwm.top
	}
	pwm.setDuty(channel, value)
}

func (pwm *esp32Group) Get(channel uint8) uint32 {
	if channel >= esp32Channels {
		return 0
	}
	return pwm.channelDuty(channel).Get() >> 4
}

func (pwm *esp32Group) Top() uint32 {
	return pwm.top
}

func (pwm *esp32Group) configureChannel(channel uint8) {
	conf0 := pwm.channelConf0(channel)
	conf0.Set(channelConf0SigOutEn)
	pwm.channelHPoint(channel).Set(0)
	pwm.setDuty(channel, 0)
}

func (pwm *esp32Group) setDuty(channel uint8, value uint32) {
	pwm.channelHPoint(channel).Set(0)
	pwm.channelDuty(channel).Set(value << 4)
	pwm.channelConf1(channel).Set(channelConf1DutyStart)
}

func (pwm *esp32Group) channelConf0(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x0)
}

func (pwm *esp32Group) channelHPoint(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x4)
}

func (pwm *esp32Group) channelDuty(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0x8)
}

func (pwm *esp32Group) channelConf1(channel uint8) *volatile.Register32 {
	return pwm.channelRegister(channel, 0xc)
}

func (pwm *esp32Group) channelRegister(channel uint8, offset uintptr) *volatile.Register32 {
	base := uintptr(unsafe.Pointer(esp.LEDC))
	return (*volatile.Register32)(unsafe.Pointer(base + uintptr(channel)*0x14 + offset))
}

func esp32OutFunc(pin machine.Pin) *volatile.Register32 {
	base := uintptr(unsafe.Pointer(&esp.GPIO.FUNC0_OUT_SEL_CFG))
	return (*volatile.Register32)(unsafe.Pointer(base + uintptr(pin)*4))
}

func esp32BestResolution(freq uint64) (uint32, uint32, bool) {
	for bits := esp32MaxResolution; bits >= 1; bits-- {
		divider := (uint64(machine.CPUFrequency()) << 8) / (freq * (uint64(1) << bits))
		if divider < esp32MinDivider || divider > esp32MaxDivider {
			continue
		}
		return uint32(bits), uint32(divider), true
	}
	return 0, 0, false
}

func isValidESP32Pin(pin machine.Pin) bool {
	switch pin {
	case machine.GPIO0,
		machine.GPIO1,
		machine.GPIO2,
		machine.GPIO3,
		machine.GPIO4,
		machine.GPIO5,
		machine.GPIO12,
		machine.GPIO13,
		machine.GPIO14,
		machine.GPIO15,
		machine.GPIO16,
		machine.GPIO17,
		machine.GPIO18,
		machine.GPIO19,
		machine.GPIO21,
		machine.GPIO22,
		machine.GPIO23,
		machine.GPIO25,
		machine.GPIO26,
		machine.GPIO27,
		machine.GPIO32,
		machine.GPIO33:
		return true
	default:
		return false
	}
}

const (
	channelConf0SigOutEn  = 1 << 2
	channelConf1DutyStart = 1 << 31
)

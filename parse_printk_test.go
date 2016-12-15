package phonelab

import (
	"testing"
)

func TestParseMsmThermalPrintk(t *testing.T) {

	parser := NewMsmThermalParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:          "6890aa2f-9895-47bf-9c37-79a2e3a34703 2016-06-25 13:24:51.291000001 3325 [   21.522780]   200   200 D KernelPrintk: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66",
			parser:        parser,
			logParseFails: false,
			subParseFails: false,
			expected: &MsmThermalPrintk{
				StateStr: "Allow Online",
				State:    MSM_THERMAL_STATE_ONLINE,
				Cpu:      3,
				Temp:     66,
			},
		},
		&parseComparison{
			line:          "6890aa2f-9895-47bf-9c37-79a2e3a34703 1970-06-07 17:06:20.399999996 2981 [   18.272750]   200   200 D KernelPrintk: <6>[   18.262644] msm_thermal: Set Offline: CPU2 Temp: 80",
			parser:        parser,
			logParseFails: false,
			subParseFails: false,
			expected: &MsmThermalPrintk{
				StateStr: "Set Offline:",
				State:    MSM_THERMAL_STATE_OFFLINE,
				Cpu:      2,
				Temp:     80,
			},
		},
	}

	commonTestParse(testConf, t)
}

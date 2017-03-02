package phonelab

import (
	"testing"
)

func TestParseMsmThermalPrintk(t *testing.T) {
	t.Parallel()
	parser := NewPrintkParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:   "6890aa2f-9895-47bf-9c37-79a2e3a34703 2016-06-25 13:24:51.291000001 3325 [   21.522780]   200   200 D KernelPrintk: <6>[   21.512807] msm_thermal: Allow Online CPU3 Temp: 66",
			parser: parser,
			expected: &MsmThermalPrintk{
				StateStr: "Allow Online",
				State:    MSM_THERMAL_STATE_ONLINE,
				Cpu:      3,
				Temp:     66,
			},
		},
		&parseComparison{
			line:   "6890aa2f-9895-47bf-9c37-79a2e3a34703 1970-06-07 17:06:20.399999996 2981 [   18.272750]   200   200 D KernelPrintk: <6>[   18.262644] msm_thermal: Set Offline: CPU2 Temp: 80",
			parser: parser,
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

func TestParseMsmThermalPrintkNewFmt(t *testing.T) {
	t.Parallel()
	parser := NewPrintkParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:   "6890aa2f-9895-47bf-9c37-79a2e3a34703 2016-06-25 13:24:51.291000001 3325 [   21.522780]   200   200 D KernelPrintk: 6,1234,21512807,-;msm_thermal: Allow Online CPU3 Temp: 66",
			parser: parser,
			expected: &MsmThermalPrintk{
				StateStr: "Allow Online",
				State:    MSM_THERMAL_STATE_ONLINE,
				Cpu:      3,
				Temp:     66,
			},
		},
		&parseComparison{
			line:   "6890aa2f-9895-47bf-9c37-79a2e3a34703 1970-06-07 17:06:20.399999996 2981 [   18.272750]   200   200 D KernelPrintk: 6,1234,18262644,-;msm_thermal: Set Offline: CPU2 Temp: 80",
			parser: parser,
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

func TestParseHealthd(t *testing.T) {
	t.Parallel()
	parser := NewPrintkParser()

	testConf := []*parseComparison{
		&parseComparison{
			line:     "e3ee246f-1970-4d78-ac04-483491206468 2016-04-26 17:44:28.803958981 2944192 [28273.477271]   203   203 D KernelPrintk: <4>[28273.442484] healthd: battery l=100 v=4303 t=22.8 h=2 st=3 c=277 chg=a",
			parser:   parser,
			expected: &Healthd{L: 100, V: 4303, T: 22.8, H: 2, St: 3, C: 277, Chg: "a"},
		},
		&parseComparison{
			line:          "e3ee246f-1970-4d78-ac04-483491206468 2016-04-26 17:44:28.803958981 2944192 [28273.477271]   203   203 D KernelPrintk: <4>[28273.442484] healthd: battery l=100 v=4303 t=22.8 h=2 st=3 c=277",
			parser:        parser,
			subParseFails: true,
		},
	}
	commonTestParse(testConf, t)
}

func TestParseHealthdNewFmt(t *testing.T) {
	t.Parallel()
	parser := NewPrintkParser()
	testConf := []*parseComparison{
		&parseComparison{
			line:     `18dd7dab-fc30-41d2-a9e1-27e2ef0012a7 1970-05-30 21:42:28.99999999 2628 [    5.704512]   386   386 D KernelPrintk: 12,1547,5703232,-;healthd: battery [l=19 v=4020 t=32.1 h=2 st=2] c=1757 chg=a 1970-05-31 01:42:28.105726458 UTC`,
			parser:   parser,
			deep:     true,
			expected: &Healthd{PrintkLog: PrintkLog{12, float64(5.703232), int64(5703232), int64(1547)}, L: 19, V: 4020, T: 32.1, H: 2, St: 2, C: 1757, Chg: "a"},
		},
	}

	commonTestParse(testConf, t)
}

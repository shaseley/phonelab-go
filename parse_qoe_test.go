package phonelab

import (
	"testing"
)

func TestQoEActivityLifecycleParser(t *testing.T) {
	t.Parallel()
	parser := NewQoEActivityLifecycleParser()
	testConf := []*parseComparison{
		// Valid
		&parseComparison{
			line:   `1fb9c6df-be49-4a46-8cee-8d64831a0b9e 2017-03-01 12:24:36.323999990 4880 [   32.017689]  1836  1836 I Activity-LifeCycle-QoE: {"Action":"onStop","AppName":"com.google.android.googlequicksearchbox","Pid":1836,"Uid":10035,"Tid":1836,"ParentActivity":"NULL","ActivityName":"com.google.android.googlequicksearchbox\/com.google.android.launcher.GEL","Time":1488389076333,"UpTime":32013,"SessionID":"f8593374-df52-4a4f-a04c-6690d68d4026","timestamp":1488389076333,"uptimeNanos":32013129934,"LogFormat":"1.1"}`,
			parser: parser,
			deep:   true,
			expected: &QoEActivityLifecycleLog{
				PLLog: PLLog{
					Timestamp:   1488389076333,
					UptimeNanos: 32013129934,
					LogFormat:   "1.1",
				},
				Action:         "onStop",
				AppName:        "com.google.android.googlequicksearchbox",
				Pid:            1836,
				Uid:            10035,
				Tid:            1836,
				ActivityName:   `com.google.android.googlequicksearchbox/com.google.android.launcher.GEL`,
				ParentActivity: "NULL",
				TimeMs:         1488389076333,
				UpTimeMs:       32013,
				SessionID:      "f8593374-df52-4a4f-a04c-6690d68d4026",
			},
		},

		// Cut off
		&parseComparison{
			line:          `1fb9c6df-be49-4a46-8cee-8d64831a0b9e 2017-03-01 12:24:34.783999990 4795 [   30.476700]  1836  1836 I Activity-LifeCycle-QoE: {"Action":"onStart","AppName":"com.google.android.googlequicksearchbox","Pid":1836,"Uid":10035,"Tid":1836,"ParentActivity":"NULL","ActivityName":"com.google`,
			parser:        parser,
			subParseFails: true,
		},
	}
	commonTestParse(testConf, t)
}

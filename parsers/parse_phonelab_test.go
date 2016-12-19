package parsers

import (
	"testing"
)

func TestPLPowerBatteryParser(t *testing.T) {
	t.Parallel()
	parser := NewPLPowerBatteryParser()
	testConf := []*parseComparison{
		// Valid
		&parseComparison{
			line:   `cb63cb9bb9ad1ea9fcfab53403820c7c084621ab        1480421029747   1480421029747.0 f750b2f0-081f-48ca-9baf-44fa4870368e    381564  7924.588899     2016-11-29 12:03:49.747999      948     1529    I       Power-Battery-PhoneLab      {"Action":"android.intent.action.BATTERY_CHANGED","Scale":100,"BatteryProperties":{"chargerAcOnline":true,"chargerUsbOnline":false,"chargerWirelessOnline":false,"Status":2,"Health":2,"Present":true,"Level":44,"Voltage":3696,"Temperature":250,"Technology":"Li-ion"},"timestamp":1480439029754,"uptimeNanos":10880568160593,"LogFormat":"1.1"}`,
			parser: parser,
			deep:   true,
			expected: &PLPowerBatteryLog{
				PLLog: PLLog{
					Timestamp:   1480439029754,
					UptimeNanos: 10880568160593,
					LogFormat:   "1.1",
				},
				Action: "android.intent.action.BATTERY_CHANGED",
				Scale:  100,
				BatteryProperties: BatteryProps{
					ChargerAcOnline:       true,
					ChargerUsbOnline:      false,
					ChargerWirelessOnline: false,
					Status:                2,
					Health:                2,
					Present:               true,
					Level:                 44,
					Voltage:               3696,
					Temperature:           250,
					Technology:            "Li-ion",
				},
			},
		},
		// Cut off
		&parseComparison{
			line:          `cb63cb9bb9ad1ea9fcfab53403820c7c084621ab        1480421029747   1480421029747.0 f750b2f0-081f-48ca-9baf-44fa4870368e    381564  7924.588899     2016-11-29 12:03:49.747999      948     1529    I       Power-Battery-PhoneLab      {"Action":"android.intent.action.BATTERY_CHANGED","Scale":100,"BatteryProperties":{"chargerAcOnline":true,"chargerUsbOnline":false,"chargerWirelessOnline":false,"Status"`,
			parser:        parser,
			subParseFails: true,
		},
	}
	commonTestParse(testConf, t)
}
